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
