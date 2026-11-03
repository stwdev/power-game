package ai

import (
	"math"
	"testing"

	"powergame/engine"
)

// buildCosts must match a simple reference that rescans the whole map after
// every city, at every build turn of a bot game.
func TestBuildCostsMatchesReference(t *testing.T) {
	g, _ := engine.NewGame(engine.Config{Seed: 3, Players: []engine.PlayerConfig{{}, {}, {}}})
	for step := 0; g.Phase != engine.PhaseOver && step < 3000; step++ {
		if g.Phase == engine.PhaseBuild {
			for p := range g.Players {
				for _, money := range []int{0, 30, 90, 250} {
					got := buildCosts(g, p, money, 20)
					want := slowBuildCosts(g, p, money, 20)
					if len(got) != len(want) {
						t.Fatalf("p%d $%d: %v vs %v", p, money, got, want)
					}
					for i := range got {
						if got[i] != want[i] {
							t.Fatalf("p%d $%d: %v vs %v", p, money, got, want)
						}
					}
				}
			}
		}
		g.Apply(Decide(g))
	}
}

func slowBuildCosts(g *engine.Game, p, money, limit int) []int {
	roads := g.RoadCosts()
	n := len(roads)
	pl := &g.Players[p]
	dist := make([]int, n)
	mine := make([]bool, n)
	for x := range dist {
		dist[x] = math.MaxInt
	}
	for _, c := range pl.Cities {
		mine[c] = true
		for x, d := range roads[c] {
			dist[x] = min(dist[x], d)
		}
	}
	have := len(pl.Cities)
	var out []int
	total := 0
	for len(out) < limit {
		best, bestCost := -1, math.MaxInt
		for x := 0; x < n; x++ {
			used := len(g.Owners[x])
			if mine[x] || !g.IsActive(x) || used >= g.Step || used >= len(engine.SlotCost) {
				continue
			}
			c := engine.SlotCost[used]
			if have > 0 {
				if dist[x] == math.MaxInt {
					continue
				}
				c += dist[x]
			}
			if c < bestCost {
				best, bestCost = x, c
			}
		}
		if best < 0 || total+bestCost > money {
			break
		}
		total += bestCost
		out = append(out, total)
		mine[best] = true
		have++
		for x, d := range roads[best] {
			dist[x] = min(dist[x], d)
		}
	}
	return out
}
