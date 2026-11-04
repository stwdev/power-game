package ai

import (
	"os"
	"sort"
	"testing"

	"powergame/engine"
)

// TestEndgameBenchmark looks at how bots finish games: how much cash they
// are left holding, how many cities they power in the final round, and how
// often a loser ends the game with enough unspent cash that a final push
// could have beaten the winner. Run with ENDGAME=1 go test ./ai -run Endgame -v
func TestEndgameBenchmark(t *testing.T) {
	if os.Getenv("ENDGAME") == "" {
		t.Skip("set ENDGAME=1 to run the end-game benchmark")
	}
	type stat struct {
		n, wins, cash, powered, idle int
		missed                       int // losses with cash to have built past the winner
	}
	st := map[engine.Level]*stat{1: {}, 2: {}, 3: {}}
	games := 30
	for s := 0; s < games; s++ {
		var lv []engine.Level
		switch s % 3 {
		case 0:
			lv = []engine.Level{engine.Medium, engine.Hard, engine.Medium, engine.Hard}
		case 1:
			lv = []engine.Level{engine.Medium, engine.Hard, engine.Medium}
		default:
			lv = []engine.Level{engine.Easy, engine.Medium, engine.Hard}
		}
		k := s % len(lv)
		lv = append(lv[k:], lv[:k]...)
		cfg := engine.Config{Seed: uint64(9100 + s)}
		for _, l := range lv {
			cfg.Players = append(cfg.Players, engine.PlayerConfig{AI: l})
		}
		g, _ := engine.NewGame(cfg)
		for steps := 0; g.Phase != engine.PhaseOver && steps < 40000; steps++ {
			if err := g.Apply(Decide(g)); err != nil {
				t.Fatal(err)
			}
		}
		if g.Phase != engine.PhaseOver {
			t.Fatalf("game %d did not finish (round %d)", s, g.Round)
		}
		win := g.Ranking[0]
		best := g.Players[win].LastPowered
		for i, p := range g.Players {
			x := st[p.AI]
			x.n++
			// Cash after the last build phase: the final payout arrives only
			// once building is over, so it could never fund a final push.
			spare := p.Money - p.LastIncome
			x.cash += spare
			x.powered += p.LastPowered
			x.idle += capacity(p.Plants) - p.LastPowered
			if i == win {
				x.wins++
				continue
			}
			// Could the cash left have bought the cities (at ~$20 each,
			// with plant capacity to spare) to beat the winner?
			short := best + 1 - p.LastPowered
			if short > 0 && capacity(p.Plants)-p.LastPowered >= short && spare >= short*25 {
				x.missed++
			}
		}
	}
	lv := []engine.Level{1, 2, 3}
	sort.Slice(lv, func(i, j int) bool { return lv[i] < lv[j] })
	for _, l := range lv {
		x := st[l]
		if x.n == 0 {
			continue
		}
		f := float64(x.n)
		t.Logf("level %d: wins %d/%d seats, cash left after building $%.0f, powered %.1f, idle capacity %.1f, losses with cash to have won %d",
			l, x.wins, x.n, float64(x.cash)/f, float64(x.powered)/f, float64(x.idle)/f, x.missed)
	}
}
