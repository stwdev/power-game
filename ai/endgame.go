package ai

import (
	"math"

	"powergame/engine"
)

// The game ends after any build phase in which someone reaches the end
// trigger, and the winner is whoever powers the most cities that round.
// Cash left over only breaks ties. So once anyone can end the game this
// round, money in the bank is nearly worthless: the right play is a final
// push that turns it into fuel and cities.

// stallRound is when a game counts as stalled and every bot races to end it.
const stallRound = 18

// buildCosts plays out player p building its cheapest cities one at a time
// (each new city brings its neighbours closer) and returns the running
// total after each city, stopping when money runs out or after limit
// cities.
func buildCosts(g *engine.Game, p, money, limit int) []int {
	roads := g.RoadCosts()
	pl := &g.Players[p]
	// The cities we could still build in, with their house price and the
	// cheapest road to them from our network so far.
	type cand struct{ city, slot, road int }
	var cands []cand
	for x, owners := range g.Owners {
		used := len(owners)
		if !g.IsActive(x) || used >= g.Step || used >= len(engine.SlotCost) || contains(owners, p) {
			continue
		}
		road := math.MaxInt
		for _, c := range pl.Cities {
			road = min(road, roads[c][x])
		}
		if len(pl.Cities) == 0 {
			road = 0 // the first city needs no road; later ones are reset below
		}
		cands = append(cands, cand{x, engine.SlotCost[used], road})
	}
	var out []int
	total := 0
	for len(out) < limit && len(cands) > 0 {
		best := 0
		for i, c := range cands {
			if c.road == math.MaxInt {
				continue
			}
			b := cands[best]
			if b.road == math.MaxInt || c.slot+c.road < b.slot+b.road ||
				(c.slot+c.road == b.slot+b.road && c.city < b.city) {
				best = i
			}
		}
		c := cands[best]
		if c.road == math.MaxInt || total+c.slot+c.road > money {
			break
		}
		total += c.slot + c.road
		out = append(out, total)
		cands[best] = cands[len(cands)-1]
		cands = cands[:len(cands)-1]
		for i := range cands {
			r := roads[c.city][cands[i].city]
			if len(out) == 1 && len(pl.Cities) == 0 {
				cands[i].road = r // our first city: roads now start from it
			} else {
				cands[i].road = min(cands[i].road, r)
			}
		}
	}
	return out
}

// stillBuilds reports whether player p has a build turn left this round.
func stillBuilds(g *engine.Game, p int) bool {
	switch g.Phase {
	case engine.PhaseAuction, engine.PhaseResources:
		return true
	case engine.PhaseBuild:
		for pos, q := range g.Order {
			if q == p {
				// Building runs from the last seat to the first.
				return pos <= len(g.Order)-1-g.Turn
			}
		}
	}
	return false
}

// buildMoney is roughly what player p can still spend on cities this round.
// Before the fuel phase some of it will go on fuel.
func buildMoney(g *engine.Game, p int) int {
	m := g.Players[p].Money
	if g.Phase == engine.PhaseAuction || g.Phase == engine.PhaseResources {
		m = m * 4 / 5
	}
	return m
}

// reach is how many cities player p could own after this round's building.
func reach(g *engine.Game, p int) int {
	pl := &g.Players[p]
	if !stillBuilds(g, p) {
		return len(pl.Cities)
	}
	end := g.EndTrigger()
	money := buildMoney(g, p)
	if len(pl.Cities)+money/engine.SlotCost[0] < end {
		// Even $10 houses could not get there: skip the detailed count.
		// Callers only compare reach with the end trigger.
		return len(pl.Cities) + money/engine.SlotCost[0]
	}
	return len(pl.Cities) + len(buildCosts(g, p, money, max(0, end-len(pl.Cities))))
}

// finalRound reports, once per decision, whether the game can end after
// this round's build phase: some player (us included) can afford to reach
// the end trigger, or already has.
func (b *bot) finalRound() bool {
	if !b.endDone {
		b.endDone = true
		g := b.g
		b.final = g.EndTriggered
		for i := range g.Players {
			if !b.final && reach(g, i) >= g.EndTrigger() {
				b.final = true
			}
		}
	}
	return b.final
}

// potential is the most cities player p could power at the end of this
// round: limited by plants, by fuel once p has had its fuel turn, and by
// the cities p owns or can still afford to build.
func potential(g *engine.Game, p int) int {
	pl := &g.Players[p]
	powerable := capacity(pl.Plants)
	if !buysLater(g, p) {
		powerable = engine.BestPowering(pl.Plants, pl.Res, 1000).Powered
	}
	return min(powerable, reach(g, p))
}

// buysLater reports whether player p still has a fuel turn this round.
func buysLater(g *engine.Game, p int) bool {
	switch g.Phase {
	case engine.PhaseAuction:
		return true
	case engine.PhaseResources:
		for pos, q := range g.Order {
			if q == p {
				return pos <= len(g.Order)-1-g.Turn // fuel runs from the last seat
			}
		}
	}
	return false
}

// winsByEnding reports whether we can end the game this round and expect
// to win: we can afford to reach the end trigger, and the cities we can
// power beat what anyone else could power by the end of the round. Then
// building to the trigger is right even if the last cities go unpowered.
func (b *bot) winsByEnding() bool {
	g := b.g
	end := g.EndTrigger()
	if reach(g, b.p) < end {
		return false
	}
	mine := min(b.pushTarget(), max(end, len(b.me.Cities)))
	// Ties go to the most cash after building, so count what it costs us
	// to reach the trigger.
	cash := b.me.Money
	if need := end - len(b.me.Cities); need > 0 {
		cash -= buildCosts(g, b.p, b.me.Money, need)[need-1]
	}
	for i := range g.Players {
		if i == b.p {
			continue
		}
		if theirs := potential(g, i); theirs > mine || (theirs == mine && g.Players[i].Money >= cash) {
			return false
		}
	}
	return true
}

// heldFuelValue is what the fuel we already hold is worth at market prices.
func (b *bot) heldFuelValue() float64 {
	v := 0.0
	for r := engine.Resource(0); r < engine.NumResources; r++ {
		v += float64(b.me.Res[r]) * float64(max(1, engine.UnitPrice(r, b.g.ResMarket[r])))
	}
	return v
}

// pushTarget is the most cities we could power this round by spending
// everything on fuel and cities, given plants `set` and `money`.
func (b *bot) pushTargetWith(set []int, money int) int {
	cities := len(b.me.Cities)
	capa := capacity(set)
	if b.g.Phase == engine.PhaseBuild {
		// Fuel is bought: build up to what it can run.
		fuelled := engine.BestPowering(set, b.me.Res, 1000).Powered
		return min(fuelled, cities+len(buildCosts(b.g, b.p, money, max(0, fuelled-cities))))
	}
	plan := b.fuelPlan(set)
	held := b.heldFuelValue()
	builds := buildCosts(b.g, b.p, money, max(0, capa-cities))
	best := 0
	for k := 0; k <= len(builds); k++ {
		spent := max(0, plan.cost(cities+k)-held)
		if k > 0 {
			spent += float64(builds[k-1])
		}
		if spent <= float64(money) {
			best = min(capa, cities+k)
		}
	}
	// With no money to build, we can still power what we have.
	if best == 0 {
		for p := min(capa, cities); p > 0; p-- {
			if plan.cost(p)-held <= float64(money) {
				return p
			}
		}
	}
	return best
}

func (b *bot) pushTarget() int { return b.pushTargetWith(b.me.Plants, b.me.Money) }
