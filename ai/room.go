package ai

import "math"

// roomer estimates how much of the map a network can still grow into.
//
// Every city that still has a free house slot is worth something to us if
// we can reach it more cheaply than any opponent: it is "our side" of the
// map. Cities an opponent reaches first are mostly lost (only a shared
// slot in a later step is left), and far-away cities count for less. A
// corner start, or a network walled in by opponents, scores low.
type roomer struct {
	b    *bot
	mine []int // cheapest road cost from our network (MaxInt if none)
	opp  []int // cheapest road cost from any opponent's network
}

func (b *bot) newRoomer() *roomer {
	g := b.g
	roads := g.RoadCosts()
	n := len(roads)
	r := &roomer{b: b, mine: make([]int, n), opp: make([]int, n)}
	for x := range r.mine {
		r.mine[x], r.opp[x] = math.MaxInt, math.MaxInt
	}
	for i := range g.Players {
		dist := r.opp
		if i == b.p {
			dist = r.mine
		}
		for _, c := range g.Players[i].Cities {
			for x, d := range roads[c] {
				if d < dist[x] {
					dist[x] = d
				}
			}
		}
	}
	return r
}

// room is how much space our network would have after adding city.
func (r *roomer) room(city int) float64 {
	g := r.b.g
	roads := g.RoadCosts()
	total := 0.0
	for x, owners := range g.Owners {
		if !g.IsActive(x) || x == city || len(owners) >= 3 || contains(owners, r.b.p) {
			continue
		}
		d := min(r.mine[x], roads[city][x])
		if d == math.MaxInt {
			continue
		}
		// Open now, or only once a later step adds a slot.
		v := 1.0
		if len(owners) >= g.Step {
			v = 0.4
		}
		switch o := r.opp[x]; {
		case d < o:
		case d == o:
			v *= 0.5
		default:
			v *= 0.15
		}
		total += v * 20 / float64(20+d)
	}
	return total
}
