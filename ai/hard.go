package ai

import (
	"powergame/engine"
)

// Rollouts per candidate move. Every candidate is scored on the same set
// of simulated futures, which keeps comparisons fair with few samples.
const rollouts = 12

// rolloutRounds caps how far a simulation runs before it is scored.
const rolloutRounds = 30

// hard decides for a Hard bot (b uses Medium heuristics as a baseline).
func (b *bot) hard() engine.Action {
	g := b.g
	switch g.Phase {
	case engine.PhaseAuction:
		if g.DiscardFor >= 0 {
			return b.discard()
		}
		if g.Auc != nil {
			return b.hardBid()
		}
		return b.hardChoose()
	case engine.PhaseResources:
		return b.buyResources()
	case engine.PhaseBuild:
		return b.hardBuild()
	}
	return engine.Action{Type: engine.ActDone}
}

func (b *bot) hardChoose() engine.Action {
	var cands [][]engine.Action
	if b.g.CanPass() {
		cands = append(cands, []engine.Action{{Type: engine.ActPass}})
	}
	for _, n := range b.affordablePlants() {
		cands = append(cands, []engine.Action{{Type: engine.ActChoose, Plant: n, Bid: n}})
	}
	if len(cands) <= 1 {
		return b.choosePlant()
	}
	return b.bestOf(cands)[0]
}

func (b *bot) hardBid() engine.Action {
	next := b.g.Auc.High + 1
	if next > b.me.Money {
		return engine.Action{Type: engine.ActPass}
	}
	cands := [][]engine.Action{
		{{Type: engine.ActPass}},
		{{Type: engine.ActBid, Bid: next}},
	}
	return b.bestOf(cands)[0]
}

// hardBuild compares stopping now with building 1..4 more cities (greedy
// picks), then performs the first step of the winning plan.
func (b *bot) hardBuild() engine.Action {
	done := engine.Action{Type: engine.ActDone}
	sim := clone(b.g)
	plans := [][]engine.Action{{done}}
	var path []engine.Action
	for k := 0; k < 4; k++ {
		sb := newBot(sim, b.p, engine.Medium)
		opts := sb.cheapestCities()
		if len(opts) == 0 {
			break
		}
		pick := sb.pickCity(opts, 0.3)
		if pick.cost > sb.me.Money {
			break
		}
		a := engine.Action{Type: engine.ActBuild, Player: b.p, City: pick.city}
		if sim.Apply(a) != nil {
			break
		}
		path = append(path, a)
		plan := append(append([]engine.Action(nil), path...), done)
		plans = append(plans, plan)
	}
	if len(plans) == 1 {
		return done
	}
	return b.bestOf(plans)[0]
}

// bestOf plays each plan followed by simulated futures and returns the
// plan with the highest average score for this bot.
func (b *bot) bestOf(plans [][]engine.Action) []engine.Action {
	scores := make([]float64, len(plans))
	for r := 0; r < rollouts; r++ {
		world := b.determinize(uint64(r))
		for i, plan := range plans {
			sim := clone(world)
			ok := true
			for _, a := range plan {
				a.Player = b.p
				if sim.CurrentPlayer() != b.p || sim.Apply(a) != nil {
					ok = false
					break
				}
			}
			if !ok {
				scores[i] -= 1000
				continue
			}
			scores[i] += b.playout(sim, uint64(r))
		}
	}
	best := 0
	for i := range scores {
		if scores[i] > scores[best] {
			best = i
		}
	}
	return plans[best]
}

// determinize copies the game and rebuilds the part of the deck Hard
// cannot see from public knowledge only: every plant not in the market,
// not owned and not publicly removed may be in it, in any order. Plants
// placed under the step 3 card were seen, so they keep their places.
func (b *bot) determinize(salt uint64) *engine.Game {
	w := clone(b.g)
	rng := engine.RNG{State: seed(b.g, b.p) ^ (salt+1)*0xd1b54a32d192ed03}
	cut := len(w.Deck)
	for i, c := range w.Deck {
		if c == engine.Step3Card {
			cut = i
			break
		}
	}
	seen := map[int]bool{}
	for _, n := range w.Market {
		seen[n] = true
	}
	for _, n := range w.Removed {
		seen[n] = true
	}
	for _, p := range w.Players {
		for _, n := range p.Plants {
			seen[n] = true
		}
	}
	for _, n := range w.Deck[cut:] {
		seen[n] = true
	}
	var pool []int // ascending, so the result never depends on hidden order
	for _, pl := range engine.AllPlants() {
		if !seen[pl.Num] {
			pool = append(pool, pl.Num)
		}
	}
	engine.Shuffle(&rng, pool)
	// Plant 13 is known to start on top of the deck until it is drawn.
	if w.Round == 1 && !seen[13] {
		for i, n := range pool {
			if n == 13 {
				pool[0], pool[i] = pool[i], pool[0]
			}
		}
	}
	top := pool[:min(cut, len(pool))]
	w.Deck = append(append([]int(nil), top...), w.Deck[cut:]...)
	w.DeckSize = len(w.Deck)
	w.RNG = engine.RNG{State: rng.Next()}
	return w
}

// playout finishes the game and scores the result for this bot. Easy
// bots are modelled as Easy; everyone else (humans, Hard bots and this
// bot itself) plays the Medium heuristics, which keeps rollouts cheap.
func (b *bot) playout(sim *engine.Game, salt uint64) float64 {
	limit := sim.Round + rolloutRounds
	for sim.Phase != engine.PhaseOver && sim.Round < limit {
		p := sim.CurrentPlayer()
		lvl := engine.Medium
		if sim.Players[p].AI == engine.Easy {
			lvl = engine.Easy
		}
		sb := newBot(sim, p, lvl)
		sb.rng.State ^= salt
		a := sb.decide()
		a.Player = p
		if sim.Apply(a) != nil {
			if sim.Apply(fallback(sim, p)) != nil {
				break
			}
		}
	}
	return b.score(sim)
}

// score is 1 for a win plus a small margin term that separates close
// outcomes; unfinished games are judged on the same terms.
func (b *bot) score(sim *engine.Game) float64 {
	me := &sim.Players[b.p]
	mine := float64(me.LastPowered)*1000 + float64(me.Money)
	if sim.Phase != engine.PhaseOver {
		mine = float64(len(me.Cities))*1000 + float64(me.Money)
	}
	best := -1.0
	for i := range sim.Players {
		if i == b.p {
			continue
		}
		o := &sim.Players[i]
		v := float64(o.LastPowered)*1000 + float64(o.Money)
		if sim.Phase != engine.PhaseOver {
			v = float64(len(o.Cities))*1000 + float64(o.Money)
		}
		best = max(best, v)
	}
	s := 0.0
	if mine > best {
		s = 1
	}
	margin := (mine - best) / 5000
	return s + max(-0.3, min(0.3, margin))
}
