// Package ai provides computer opponents at three skill levels.
//
//   - Easy plays loosely: random affordable plants, minimum bids, skips
//     fuel now and then and builds without much thought.
//   - Medium values plants by income against fuel cost, buys fuel for the
//     cities it can supply and grows its network to match.
//   - Hard looks ahead: for each important decision (which plant to
//     auction, whether to raise a bid, how far to build) it plays the rest
//     of the game out many times and picks the move that wins most often.
//
// Bots only use public information. Hard's simulations shuffle the unseen
// deck instead of reading its real order.
package ai

import (
	"math"
	"sort"

	"powergame/engine"
)

// Decide returns the action for the current player, who must be a bot.
// The result is always one that the engine accepts.
func Decide(g *engine.Game) engine.Action {
	p := g.CurrentPlayer()
	if p < 0 {
		return engine.Action{Type: engine.ActDone}
	}
	lvl := g.Players[p].AI
	if lvl < engine.Easy || lvl > engine.Hard {
		lvl = engine.Medium
	}
	var a engine.Action
	if lvl == engine.Hard {
		a = newBot(g, p, engine.Medium).hard()
	} else {
		a = newBot(g, p, lvl).decide()
	}
	a.Player = p
	if test := clone(g); test.Apply(a) != nil {
		return fallback(g, p)
	}
	return a
}

func newBot(g *engine.Game, p int, lvl engine.Level) *bot {
	return &bot{g: g, p: p, me: &g.Players[p], lvl: lvl, rng: engine.RNG{State: seed(g, p)}}
}

// fallback is a move that is always legal.
func fallback(g *engine.Game, p int) engine.Action {
	if g.Phase == engine.PhaseAuction {
		if g.DiscardFor >= 0 {
			for _, n := range g.Players[p].Plants {
				if n != g.JustBought {
					return engine.Action{Type: engine.ActDiscard, Player: p, Plant: n}
				}
			}
		}
		if g.Auc == nil && !g.CanPass() {
			m := g.ActualMarket()
			return engine.Action{Type: engine.ActChoose, Player: p, Plant: m[0], Bid: m[0]}
		}
		return engine.Action{Type: engine.ActPass, Player: p}
	}
	if g.Phase == engine.PhasePower {
		return powerAction(g, p)
	}
	return engine.Action{Type: engine.ActDone, Player: p}
}

// powerAction runs the plants that power the most cities on the least fuel.
func powerAction(g *engine.Game, p int) engine.Action {
	me := &g.Players[p]
	best := engine.BestPowering(me.Plants, me.Res, len(me.Cities))
	return engine.Action{Type: engine.ActPower, Player: p, Fire: best.Fired, HybridCoal: best.HybridCoal}
}

// clone deep-copies everything Apply mutates, with logging switched off.
func clone(g *engine.Game) *engine.Game {
	c := *g
	c.Quiet = true
	c.Players = make([]engine.Player, len(g.Players))
	for i, p := range g.Players {
		p.Plants = append([]int(nil), p.Plants...)
		p.Cities = append([]int(nil), p.Cities...)
		c.Players[i] = p
	}
	c.Order = append([]int(nil), g.Order...)
	c.Market = append([]int(nil), g.Market...)
	c.Deck = append([]int(nil), g.Deck...)
	c.Bought = append([]bool(nil), g.Bought...)
	c.Passed = append([]bool(nil), g.Passed...)
	c.Log = nil
	c.Owners = make([][]int, len(g.Owners))
	for i, o := range g.Owners {
		c.Owners[i] = append([]int(nil), o...)
	}
	if g.Auc != nil {
		a := *g.Auc
		a.Bidders = append([]int(nil), a.Bidders...)
		c.Auc = &a
	}
	return &c
}

func seed(g *engine.Game, p int) uint64 {
	s := uint64(g.Round)*0x9e3779b97f4a7c15 ^ uint64(p+1)*0xbf58476d1ce4e5b9 ^
		uint64(g.Turn+1)*0x94d049bb133111eb ^ uint64(g.Players[p].Money)<<20 ^
		uint64(len(g.Players[p].Cities))<<40
	for _, n := range g.Market {
		s = s*31 + uint64(n)
	}
	for _, o := range g.Players {
		s = s*131 + uint64(o.Money)
	}
	return s
}

type bot struct {
	g   *engine.Game
	p   int
	me  *engine.Player
	lvl engine.Level
	rng engine.RNG
}

func (b *bot) chance(pct int) bool { return b.rng.Intn(100) < pct }

func (b *bot) decide() engine.Action {
	g := b.g
	switch g.Phase {
	case engine.PhaseAuction:
		if g.DiscardFor >= 0 {
			return b.discard()
		}
		if g.Auc != nil {
			return b.respondBid()
		}
		return b.choosePlant()
	case engine.PhaseResources:
		return b.buyResources()
	case engine.PhaseBuild:
		return b.build()
	case engine.PhasePower:
		return powerAction(g, b.p)
	}
	return engine.Action{Type: engine.ActDone}
}

// price estimates the per-unit fuel cost of a plant kind right now.
func (b *bot) price(k engine.Kind) float64 {
	unit := func(r engine.Resource) float64 {
		pr := engine.UnitPrice(r, b.g.ResMarket[r])
		if pr == 0 {
			return 12 // empty market: fuel is scarce
		}
		if b.g.ResMarket[r] < 4 {
			return float64(pr) + 2 // scarce fuel climbs as others buy
		}
		return float64(pr)
	}
	switch k {
	case engine.KindCoal:
		return unit(engine.Coal)
	case engine.KindOil:
		return unit(engine.Oil)
	case engine.KindGarbage:
		return unit(engine.Garbage)
	case engine.KindUranium:
		return unit(engine.Uranium)
	case engine.KindHybrid:
		return math.Min(unit(engine.Coal), unit(engine.Oil))
	}
	return 0
}

// worth is a plant's rough value per round of operation.
func (b *bot) worth(num int) float64 {
	pl, ok := engine.PlantByNum(num)
	if !ok {
		return 0
	}
	w := float64(pl.Cities)*10 - float64(pl.Input)*b.price(pl.Kind)*1.2
	if pl.Kind == engine.KindEco {
		w += 5
	}
	return w
}

func capacity(plants []int) int {
	c := 0
	for _, n := range plants {
		if pl, ok := engine.PlantByNum(n); ok {
			c += pl.Cities
		}
	}
	return c
}

// bestSet is the plants kept after adding num, dropping the weakest if
// over the limit.
func (b *bot) bestSet(add int) []int {
	set := append(append([]int(nil), b.me.Plants...), add)
	if len(set) > b.g.MaxPlants() {
		worst := -1
		for i, n := range set {
			if n == add {
				continue
			}
			if worst < 0 || b.worth(n) < b.worth(set[worst]) {
				worst = i
			}
		}
		set = append(set[:worst], set[worst+1:]...)
	}
	return set
}

func (b *bot) setWorth(set []int) float64 {
	w := 0.0
	for _, n := range set {
		w += b.worth(n)
	}
	return w
}

// growth is how many cities the bot expects to add next round.
func (b *bot) growth() int {
	switch {
	case b.g.Round <= 2:
		return 2
	case b.g.Step == 1:
		return 3
	default:
		return 4
	}
}

// plantValue is how much the bot gains from owning plant num.
func (b *bot) plantValue(num int) float64 {
	gain := b.setWorth(b.bestSet(num)) - b.setWorth(b.me.Plants)
	have := capacity(b.me.Plants)
	need := len(b.me.Cities) + b.growth()
	if have >= need+2 {
		gain *= 0.35
	} else if have >= need {
		gain *= 0.7
	}
	return gain
}

func (b *bot) maxBid(num int) int {
	if b.lvl == engine.Easy {
		return min(num+2, b.me.Money)
	}
	bid := num + int(math.Max(0, b.plantValue(num)*0.3))
	reserve := 10
	if b.g.Round == 1 {
		reserve = 8
	}
	return min(bid, b.me.Money-reserve)
}

func (b *bot) affordablePlants() []int {
	var out []int
	for _, n := range b.g.ActualMarket() {
		if n <= b.me.Money {
			out = append(out, n)
		}
	}
	return out
}

func (b *bot) choosePlant() engine.Action {
	affordable := b.affordablePlants()
	mustBuy := !b.g.CanPass()
	if len(affordable) == 0 {
		return engine.Action{Type: engine.ActPass}
	}
	if b.lvl == engine.Easy {
		if !mustBuy && (capacity(b.me.Plants) > len(b.me.Cities) || b.chance(30)) {
			return engine.Action{Type: engine.ActPass}
		}
		n := affordable[b.rng.Intn(len(affordable))]
		return engine.Action{Type: engine.ActChoose, Plant: n, Bid: n}
	}
	best, bestScore := -1, math.Inf(-1)
	for _, n := range affordable {
		s := b.plantValue(n) - float64(n)*0.3
		if s > bestScore {
			best, bestScore = n, s
		}
	}
	if !mustBuy && (bestScore < 6 || b.maxBid(best) < best) {
		return engine.Action{Type: engine.ActPass}
	}
	return engine.Action{Type: engine.ActChoose, Plant: best, Bid: best}
}

func (b *bot) respondBid() engine.Action {
	auc := b.g.Auc
	next := auc.High + 1
	if next > b.me.Money {
		return engine.Action{Type: engine.ActPass}
	}
	if b.lvl == engine.Easy {
		if next <= auc.Plant+2 && b.chance(35) {
			return engine.Action{Type: engine.ActBid, Bid: next}
		}
		return engine.Action{Type: engine.ActPass}
	}
	if next <= b.maxBid(auc.Plant) {
		return engine.Action{Type: engine.ActBid, Bid: next}
	}
	return engine.Action{Type: engine.ActPass}
}

func (b *bot) discard() engine.Action {
	worst := -1
	for _, n := range b.me.Plants {
		if n == b.g.JustBought {
			continue
		}
		if worst < 0 || b.worth(n) < b.worth(worst) {
			worst = n
		}
	}
	return engine.Action{Type: engine.ActDiscard, Plant: worst}
}

// fuelTarget is how much of each resource the bot wants: enough to run
// its most efficient plants for the cities it expects to have.
func (b *bot) fuelTarget(goal int) [engine.NumResources]int {
	var want [engine.NumResources]int
	plants := append([]int(nil), b.me.Plants...)
	sort.Slice(plants, func(i, j int) bool { return b.worth(plants[i]) > b.worth(plants[j]) })
	powered := 0
	for _, n := range plants {
		if powered >= goal {
			break
		}
		pl, _ := engine.PlantByNum(n)
		powered += pl.Cities
		switch pl.Kind {
		case engine.KindCoal:
			want[engine.Coal] += pl.Input
		case engine.KindOil:
			want[engine.Oil] += pl.Input
		case engine.KindGarbage:
			want[engine.Garbage] += pl.Input
		case engine.KindUranium:
			want[engine.Uranium] += pl.Input
		case engine.KindHybrid:
			if b.price(engine.KindOil) < b.price(engine.KindCoal) {
				want[engine.Oil] += pl.Input
			} else {
				want[engine.Coal] += pl.Input
			}
		}
	}
	return want
}

func (b *bot) buyResources() engine.Action {
	goal := len(b.me.Cities) + b.growth()
	if b.lvl == engine.Easy {
		if b.chance(10) {
			return engine.Action{Type: engine.ActDone}
		}
		goal = 99
	}
	if a, ok := b.buyToward(b.fuelTarget(goal), 99); ok {
		return a
	}
	return engine.Action{Type: engine.ActDone}
}

// buyToward buys as much as it can of the first resource below want,
// never paying more than maxUnit for a unit.
func (b *bot) buyToward(want [engine.NumResources]int, maxUnit int) (engine.Action, bool) {
	g := b.g
	for r := engine.Resource(0); r < engine.NumResources; r++ {
		need := want[r] - b.me.Res[r]
		for k := min(need, g.ResMarket[r]); k > 0; k-- {
			if engine.UnitPrice(r, g.ResMarket[r]-k+1) > maxUnit {
				continue
			}
			cost, ok := g.ResourceCost(r, k)
			if !ok || cost > b.me.Money || !g.CanStore(b.p, r, k) {
				continue
			}
			return engine.Action{Type: engine.ActBuy, Resource: r, Amount: k}, true
		}
	}
	return engine.Action{}, false
}

// usable is how many cities the bot can power with fuel it holds.
func (b *bot) usable() int {
	return engine.BestPowering(b.me.Plants, b.me.Res, 1000).Powered
}

type cityCost struct{ city, cost int }

func (b *bot) cheapestCities() []cityCost {
	var out []cityCost
	for c, cost := range b.g.BuildCosts(b.p) {
		if cost >= 0 {
			out = append(out, cityCost{c, cost})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].cost != out[j].cost {
			return out[i].cost < out[j].cost
		}
		return out[i].city < out[j].city
	})
	return out
}

// roomWeight is how many dollars one unit of room is worth when choosing
// where to build.
const roomWeight = 8.0

// pickCity chooses where to build, trading cost now against room to grow:
// a cheap city in a corner or behind an opponent's wall is often a trap.
func (b *bot) pickCity(opts []cityCost) cityCost {
	if len(opts) == 1 {
		return opts[0]
	}
	r := b.newRoomer()
	best, bestScore := opts[0], math.Inf(1)
	for i, o := range opts {
		if i >= 10 || o.cost > b.me.Money || o.cost > opts[0].cost+25 {
			break
		}
		s := float64(o.cost) - roomWeight*r.room(o.city)
		if s < bestScore {
			best, bestScore = o, s
		}
	}
	return best
}

func contains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func (b *bot) build() engine.Action {
	done := engine.Action{Type: engine.ActDone}
	opts := b.cheapestCities()
	if len(opts) == 0 {
		return done
	}
	have := len(b.me.Cities)
	if b.lvl == engine.Easy {
		limit := capacity(b.me.Plants)
		if b.me.Money > 150 || b.g.Round > 18 {
			limit += 2
		}
		if have >= limit || b.chance(20) {
			return done
		}
		pick := opts[b.rng.Intn(min(3, len(opts)))]
		if pick.cost > b.me.Money {
			pick = opts[0]
		}
		if pick.cost > b.me.Money {
			return done
		}
		return engine.Action{Type: engine.ActBuild, City: pick.city}
	}
	target := max(b.usable(), 1)
	if b.g.Step >= 2 && b.me.Money > 60 {
		target++
	}
	if b.me.Money-opts[0].cost > 40 {
		target = max(target, capacity(b.me.Plants))
	}
	if b.g.Round > 18 {
		target = 99 // the game has stalled: race to the finish
	}
	if have >= target || opts[0].cost > b.me.Money-5 {
		return done
	}
	return engine.Action{Type: engine.ActBuild, City: b.pickCity(opts).city}
}
