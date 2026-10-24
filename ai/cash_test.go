package ai

import (
	"os"
	"testing"

	"powergame/engine"
)

// TestCashBenchmark looks for early plant purchases that leave a bot too
// poor to fuel the plant and build the cities to use it, and checks how
// those bots do afterwards. Run with CASH=1 go test ./ai -run Cash -v
func TestCashBenchmark(t *testing.T) {
	if os.Getenv("CASH") == "" {
		t.Skip("set CASH=1 to run the cash-starvation benchmark")
	}
	type stat struct {
		buys, overreach, overreachWins, wins, games   int
		rank4Income, overreachRank4Income             int
		r4, r4over                                    int
		r6cities, r6income, finalPowered, finalCities int
	}
	st := map[engine.Level]*stat{2: {}, 3: {}}
	games := 30
	for s := 0; s < games; s++ {
		lv := []engine.Level{engine.Medium, engine.Hard, engine.Medium, engine.Hard}
		k := s % 4
		lv = append(lv[k:], lv[:k]...)
		cfg := engine.Config{Seed: uint64(7000 + s)}
		for _, l := range lv {
			cfg.Players = append(cfg.Players, engine.PlayerConfig{AI: l})
		}
		g, _ := engine.NewGame(cfg)
		over := make([]bool, len(lv))
		snap := false
		r4 := make([]int, len(lv))
		for steps := 0; g.Phase != engine.PhaseOver && steps < 30000; steps++ {
			before := make([]int, len(lv))
			for i := range g.Players {
				before[i] = len(g.Players[i].Plants)
			}
			money := make([]int, len(lv))
			for i := range g.Players {
				money[i] = g.Players[i].Money
			}
			round := g.Round
			if err := g.Apply(Decide(g)); err != nil {
				t.Fatal(err)
			}
			for i := range g.Players {
				p := &g.Players[i]
				if len(p.Plants) > before[i] || (g.JustBought >= 0 && g.DiscardFor == i) {
					if round <= 4 {
						st[p.AI].buys++
						// Money left must buy fuel for every plant and
						// at least two more cities to keep growing.
						b := newBot(g, i, engine.Medium)
						fuel := 0.0
						for _, n := range p.Plants {
							pl, _ := engine.PlantByNum(n)
							fuel += float64(pl.Input) * b.price(pl.Kind)
						}
						if float64(p.Money) < fuel+30 && money[i]-p.Money > 0 {
							st[p.AI].overreach++
							over[i] = true
						}
					}
				}
			}
			if g.Round == 7 && !snap {
				snap = true
				for _, q := range g.Players {
					st[q.AI].r6cities += len(q.Cities)
					st[q.AI].r6income += q.LastIncome
				}
			}
			if g.Round == 5 && g.Phase == engine.PhaseAuction {
				for i := range g.Players {
					if r4[i] == 0 {
						r4[i] = g.Players[i].LastIncome + 1
					}
				}
			}
		}
		w := g.Ranking[0]
		for i, l := range lv {
			s := st[l]
			s.games++
			if over[i] {
				s.r4over++
				s.overreachRank4Income += r4[i] - 1
				if i == w {
					s.overreachWins++
				}
			} else {
				s.r4++
				s.rank4Income += r4[i] - 1
			}
			if i == w {
				s.wins++
			}
			s.finalPowered += g.Players[i].LastPowered
			s.finalCities += len(g.Players[i].Cities)
		}
	}
	for _, l := range []engine.Level{engine.Medium, engine.Hard} {
		s := st[l]
		t.Logf("level %d: %d of %d early buys left it cash-starved; round-4 income %.0f when starved vs %.0f otherwise; win rate %d/%d starved vs %d/%d otherwise",
			l, s.overreach, s.buys,
			float64(s.overreachRank4Income)/float64(max(1, s.r4over)), float64(s.rank4Income)/float64(max(1, s.r4)),
			s.overreachWins, s.r4over, s.wins-s.overreachWins, s.r4)
		t.Logf("level %d: after round 6: %.1f cities, income $%.0f; at the end: %.1f cities, %.1f powered; wins %d/%d",
			l, float64(s.r6cities)/float64(s.games), float64(s.r6income)/float64(s.games),
			float64(s.finalCities)/float64(s.games), float64(s.finalPowered)/float64(s.games), s.wins, games)
	}
}
