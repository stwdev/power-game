package ai

import (
	"encoding/json"
	"reflect"
	"testing"

	"powergame/engine"
)

func playOut(t *testing.T, levels []engine.Level, seed uint64) *engine.Game {
	t.Helper()
	cfg := engine.Config{Seed: seed}
	for _, l := range levels {
		cfg.Players = append(cfg.Players, engine.PlayerConfig{Name: "bot", AI: l})
	}
	g, err := engine.NewGame(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for steps := 0; g.Phase != engine.PhaseOver; steps++ {
		if steps > 20000 || g.Round > 80 {
			t.Fatalf("seed %d: game did not finish (round %d, phase %s)", seed, g.Round, g.Phase)
		}
		a := Decide(g)
		if err := g.Apply(a); err != nil {
			t.Fatalf("seed %d: bot action %+v rejected: %v", seed, a, err)
		}
		if steps%25 == 0 {
			if err := g.Validate(); err != nil {
				t.Fatalf("seed %d round %d: invalid state: %v", seed, g.Round, err)
			}
		}
	}
	return g
}

func checkSave(t *testing.T, g *engine.Game) {
	t.Helper()
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Load(data); err != nil {
		t.Fatalf("reload: %v", err)
	}
}

func TestBotsFinishGames(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for seed := uint64(1); seed <= 12; seed++ {
			levels := make([]engine.Level, n)
			for i := range levels {
				levels[i] = engine.Level(1 + (i+int(seed))%2)
			}
			g := playOut(t, levels, seed*uint64(n))
			if len(g.Ranking) != n {
				t.Fatal("no ranking")
			}
			checkSave(t, g)
		}
	}
}

func TestHardFinishesEveryTableSize(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	for n := 2; n <= 6; n++ {
		levels := make([]engine.Level, n)
		for i := range levels {
			levels[i] = engine.Level(1 + i%3)
		}
		checkSave(t, playOut(t, levels, uint64(77+n)))
	}
}

func TestMediumBeatsEasy(t *testing.T) {
	wins := 0
	const games = 40
	for s := 0; s < games; s++ {
		levels := []engine.Level{engine.Easy, engine.Medium}
		if s%2 == 1 {
			levels[0], levels[1] = levels[1], levels[0]
		}
		g := playOut(t, levels, uint64(300+s))
		if g.Players[g.Ranking[0]].AI == engine.Medium {
			wins++
		}
	}
	if wins < games*3/4 {
		t.Errorf("medium won only %d of %d games against easy", wins, games)
	}
}

func TestHardBeatsMedium(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	wins := 0
	const games = 10
	for s := 0; s < games; s++ {
		levels := []engine.Level{engine.Medium, engine.Hard}
		if s%2 == 1 {
			levels[0], levels[1] = levels[1], levels[0]
		}
		g := playOut(t, levels, uint64(900+s))
		if g.Players[g.Ranking[0]].AI == engine.Hard {
			wins++
		}
	}
	if wins < games*6/10 {
		t.Errorf("hard won only %d of %d games against medium", wins, games)
	}
}

func TestBotsIgnoreHiddenDeck(t *testing.T) {
	// Two games that differ only in the hidden deck order must get the
	// same first decision from every bot level.
	for _, lvl := range []engine.Level{engine.Easy, engine.Medium, engine.Hard} {
		cfg := engine.Config{Seed: 42, Players: []engine.PlayerConfig{{AI: lvl}, {AI: lvl}, {AI: lvl}}}
		g1, _ := engine.NewGame(cfg)
		g2, _ := engine.NewGame(cfg)
		for i, j := 1, len(g2.Deck)-2; i < j; i, j = i+1, j-1 {
			g2.Deck[i], g2.Deck[j] = g2.Deck[j], g2.Deck[i]
		}
		a1, a2 := Decide(g1), Decide(g2)
		if !reflect.DeepEqual(a1, a2) {
			t.Errorf("level %d: decision depends on hidden deck: %+v vs %+v", lvl, a1, a2)
		}
	}
}

func TestBotsFinishOnEveryMap(t *testing.T) {
	maps := []engine.MapSpec{{ID: "lakeside"}, {ID: "ridge"}}
	for s := uint64(1); s <= 8; s++ {
		maps = append(maps, engine.MapSpec{ID: "random", Seed: s * 104729})
	}
	for i, m := range maps {
		n := 2 + i%5
		cfg := engine.Config{Seed: uint64(i + 1), Map: m}
		for k := 0; k < n; k++ {
			cfg.Players = append(cfg.Players, engine.PlayerConfig{AI: engine.Level(1 + k%2)})
		}
		g, err := engine.NewGame(cfg)
		if err != nil {
			t.Fatal(err)
		}
		for steps := 0; g.Phase != engine.PhaseOver; steps++ {
			if steps > 20000 {
				t.Fatalf("%v: game did not finish", m)
			}
			if err := g.Apply(Decide(g)); err != nil {
				t.Fatalf("%v: %v", m, err)
			}
		}
		checkSave(t, g)
	}
}

func TestBotsFinishWithoutRestock(t *testing.T) {
	for n := 2; n <= 6; n += 2 {
		zero := engine.RestockTable{}
		cfg := engine.Config{Seed: uint64(n), Restock: &zero}
		for k := 0; k < n; k++ {
			cfg.Players = append(cfg.Players, engine.PlayerConfig{AI: engine.Level(1 + k%2)})
		}
		g, err := engine.NewGame(cfg)
		if err != nil {
			t.Fatal(err)
		}
		for steps := 0; g.Phase != engine.PhaseOver; steps++ {
			if steps > 30000 {
				t.Fatalf("%d players: game did not finish (round %d)", n, g.Round)
			}
			if err := g.Apply(Decide(g)); err != nil {
				t.Fatal(err)
			}
		}
		checkSave(t, g)
	}
}

// endgameSeat describes one player in a hand-made end-game position.
type endgameSeat struct {
	cities, money int
	plants        []int // eco plants only, so no fuel is needed
}

// endgameTable sets up a 3-player step 2 game in the build phase. Player 1
// is the bot under test and builds first; player 0 builds last.
func endgameTable(t *testing.T, lvl engine.Level, opp, bot endgameSeat) *engine.Game {
	t.Helper()
	g, err := engine.NewGame(engine.Config{Seed: 5, Players: []engine.PlayerConfig{{AI: engine.Medium}, {AI: lvl}, {AI: engine.Medium}}})
	if err != nil {
		t.Fatal(err)
	}
	g.Step = 2
	for p, seat := range map[int]endgameSeat{0: opp, 1: bot} {
		for len(g.Players[p].Cities) < seat.cities {
			best, cost := -1, 1<<30
			for c, k := range g.BuildCosts(p) {
				if k >= 0 && k < cost {
					best, cost = c, k
				}
			}
			if best < 0 {
				t.Fatal("no room to place cities")
			}
			g.Players[p].Cities = append(g.Players[p].Cities, best)
			g.Owners[best] = append(g.Owners[best], p)
		}
		for _, n := range seat.plants {
			g.Market = removePlant(g.Market, n)
			g.Deck = removePlant(g.Deck, n)
		}
		g.Players[p].Plants = seat.plants
		g.Players[p].Money = seat.money
	}
	g.Order = []int{0, 2, 1} // building runs from the last seat: the bot first
	g.Phase, g.Turn = engine.PhaseBuild, 0
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
	return g
}

func removePlant(s []int, n int) []int {
	for i, m := range s {
		if m == n {
			return append(s[:i:i], s[i+1:]...)
		}
	}
	return s
}

var (
	ecoNine     = []int{18, 27, 33} // powers 2 + 3 + 4
	ecoEleven   = []int{22, 27, 50} // powers 2 + 3 + 6
	ecoThirteen = []int{33, 37, 44} // powers 4 + 4 + 5
	ecoFifteen  = []int{37, 44, 50} // powers 4 + 5 + 6
)

func TestFinalRoundDetection(t *testing.T) {
	g := endgameTable(t, engine.Medium, endgameSeat{16, 100, ecoEleven}, endgameSeat{6, 200, ecoThirteen})
	if !newBot(g, 1, engine.Medium).finalRound() {
		t.Error("opponent one city from the end with $100: should be the final round")
	}
	g = endgameTable(t, engine.Medium, endgameSeat{8, 100, ecoEleven}, endgameSeat{6, 200, ecoThirteen})
	if newBot(g, 1, engine.Medium).finalRound() {
		t.Error("nobody near the end: should not be the final round")
	}
}

// buildTurn plays the current player's whole build turn and returns how
// many cities it built.
func buildTurn(t *testing.T, g *engine.Game) int {
	t.Helper()
	p := g.CurrentPlayer()
	before := len(g.Players[p].Cities)
	for g.CurrentPlayer() == p && g.Phase == engine.PhaseBuild {
		if err := g.Apply(Decide(g)); err != nil {
			t.Fatal(err)
		}
	}
	return len(g.Players[p].Cities) - before
}

func TestFinalPushSpendsTheCash(t *testing.T) {
	// The opponent can end the game this round. The bot has $200 and plants
	// for 13 cities: holding the cash would waste it, so it must build up
	// to what its plants can run.
	g := endgameTable(t, engine.Medium, endgameSeat{16, 100, ecoEleven}, endgameSeat{6, 200, ecoThirteen})
	built := buildTurn(t, g)
	if got := 6 + built; got < 12 {
		t.Errorf("built %d cities in the final round, ending with %d and $%d", built, got, g.Players[1].Money)
	}
}

func TestClosesOutWhenAhead(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	// The bot can afford to reach the 17-city end trigger and would power
	// 15, while nobody else can power more than 9: it should end the game
	// now instead of waiting.
	for _, lvl := range []engine.Level{engine.Medium, engine.Hard} {
		g := endgameTable(t, lvl, endgameSeat{16, 100, ecoNine}, endgameSeat{12, 400, ecoFifteen})
		built := buildTurn(t, g)
		if got := 12 + built; got < g.EndTrigger() {
			t.Errorf("level %d stopped at %d cities with $%d instead of ending the game it was winning",
				lvl, got, g.Players[1].Money)
		}
	}
}
