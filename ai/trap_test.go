package ai

import (
	"os"
	"testing"

	"powergame/engine"
)

// TestTrapBenchmark measures how hemmed-in each bot level gets: the road
// cost it pays per house and how often it wants to build but cannot.
// Run with TRAP=1 go test ./ai -run Trap -v
func TestTrapBenchmark(t *testing.T) {
	if os.Getenv("TRAP") == "" {
		t.Skip("set TRAP=1 to run the blockade benchmark")
	}
	type stat struct{ roads, houses, stuck, boxed, turns, wins, games, cities int }
	st := map[engine.Level]*stat{1: {}, 2: {}, 3: {}}
	games := 24
	for s := 0; s < games; s++ {
		lv := []engine.Level{engine.Medium, engine.Hard, engine.Medium, engine.Hard}
		k := s % 4
		lv = append(lv[k:], lv[:k]...)
		maps := []engine.MapSpec{{ID: "classic"}, {ID: "ridge"}, {ID: "random", Seed: uint64(s) * 7777}}
		cfg := engine.Config{Seed: uint64(4000 + s), Map: maps[s%3]}
		for _, l := range lv {
			cfg.Players = append(cfg.Players, engine.PlayerConfig{AI: l})
		}
		g, _ := engine.NewGame(cfg)
		for steps := 0; g.Phase != engine.PhaseOver && steps < 30000; steps++ {
			p := g.CurrentPlayer()
			me := &g.Players[p]
			a := Decide(g)
			if g.Phase == engine.PhaseBuild {
				if a.Type == engine.ActBuild {
					cost, _ := g.BuildCost(p, a.City)
					st[me.AI].roads += cost - engine.SlotCost[len(g.Owners[a.City])]
					st[me.AI].houses++
				} else if len(me.Cities) > 0 {
					cheapest := 1 << 30
					for _, c := range g.BuildCosts(p) {
						if c >= 0 && c < cheapest {
							cheapest = c
						}
					}
					if cheapest > 35 && g.Step < 3 {
						st[me.AI].boxed++
					}
					// Finished building: was it by choice or because nothing
					// near enough was affordable?
					usable := engine.BestPowering(me.Plants, me.Res, 1000).Powered
					st[me.AI].turns++
					if usable > len(me.Cities) && me.Money >= 30 {
						cheapest := 1 << 30
						for _, c := range g.BuildCosts(p) {
							if c >= 0 && c < cheapest {
								cheapest = c
							}
						}
						if cheapest > me.Money {
							st[me.AI].stuck++
						}
					}
				}
			}
			if err := g.Apply(a); err != nil {
				t.Fatal(err)
			}
		}
		for i := range st {
			_ = i
		}
		st[g.Players[g.Ranking[0]].AI].wins++
		for i, l := range lv {
			st[l].games++
			st[l].cities += len(g.Players[i].Cities)
		}
	}
	for _, l := range []engine.Level{engine.Medium, engine.Hard} {
		s := st[l]
		t.Logf("level %d: road $/house %.1f, boxed in %d and stuck %d of %d build turns, final cities %.1f, wins %d/%d",
			l, float64(s.roads)/float64(max(1, s.houses)), s.boxed, s.stuck, s.turns,
			float64(s.cities)/float64(max(1, s.games)), s.wins, games)
	}
}
