package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func newTestGame(t *testing.T, n int) *Game {
	t.Helper()
	cfg := Config{Seed: 7}
	for i := 0; i < n; i++ {
		cfg.Players = append(cfg.Players, PlayerConfig{Name: "P"})
	}
	g, err := NewGame(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestBoardIsConnected(t *testing.T) {
	b := classicBoard()
	all := make([]bool, len(b.Cities))
	for i := range all {
		all[i] = true
	}
	for c, d := range b.Distances([]int{0}, all) {
		if d < 0 || d > 1000 {
			t.Fatalf("%s unreachable", b.Cities[c].Name)
		}
	}
	for _, e := range b.Edges {
		if e.Cost < 3 || e.Cost > 30 {
			t.Errorf("edge %v cost %d out of range", e, e.Cost)
		}
	}
}

func TestUnitPrice(t *testing.T) {
	cases := []struct {
		r          Resource
		avail, out int
	}{
		{Coal, 24, 1}, {Coal, 22, 1}, {Coal, 21, 2}, {Coal, 1, 8}, {Coal, 0, 0},
		{Uranium, 12, 1}, {Uranium, 5, 8}, {Uranium, 4, 10}, {Uranium, 1, 16},
	}
	for _, c := range cases {
		if got := UnitPrice(c.r, c.avail); got != c.out {
			t.Errorf("UnitPrice(%v,%d)=%d want %d", c.r, c.avail, got, c.out)
		}
	}
}

func TestStorageWithHybrid(t *testing.T) {
	c := StorageOf([]int{4, 5}) // coal 2 in, hybrid 2 in
	if !c.Fits([NumResources]int{8, 0, 0, 0}) || !c.Fits([NumResources]int{4, 4, 0, 0}) {
		t.Error("should fit")
	}
	if c.Fits([NumResources]int{0, 5, 0, 0}) || c.Fits([NumResources]int{5, 4, 0, 0}) {
		t.Error("should not fit")
	}
	kept, removed := c.Trim([NumResources]int{6, 6, 1, 0})
	if !c.Fits(kept) || removed[Garbage] != 1 || kept[Coal]+kept[Oil] != 8 {
		t.Errorf("trim gave %v / %v", kept, removed)
	}
}

func TestBestPowering(t *testing.T) {
	// Plants 5 (hybrid 2->1), 13 (eco ->1), 15 (coal 2->3).
	pw := BestPowering([]int{5, 13, 15}, [NumResources]int{2, 1, 0, 0}, 10)
	if pw.Powered != 4 { // 15 + 13; not enough fuel for 5 as well
		t.Fatalf("powered %d, want 4 (%+v)", pw.Powered, pw)
	}
	pw = BestPowering([]int{5, 13, 15}, [NumResources]int{3, 1, 0, 0}, 5)
	if pw.Powered != 5 || pw.Use != [NumResources]int{3, 1, 0, 0} {
		t.Fatalf("hybrid should mix coal and oil: %+v", pw)
	}
	pw = BestPowering([]int{5, 13, 15}, [NumResources]int{3, 1, 0, 0}, 1)
	if pw.Powered != 1 || pw.Use != [NumResources]int{} {
		t.Fatalf("with one city only the eco plant should run: %+v", pw)
	}
}

func TestTurnEnforcement(t *testing.T) {
	g := newTestGame(t, 3)
	cur := g.CurrentPlayer()
	other := (cur + 1) % 3
	if err := g.Apply(Action{Type: ActChoose, Player: other, Plant: 3, Bid: 3}); err == nil {
		t.Fatal("out-of-turn action accepted")
	}
	if err := g.Apply(Action{Type: ActPass, Player: cur}); err == nil {
		t.Fatal("pass in round 1 accepted")
	}
	if err := g.Apply(Action{Type: ActChoose, Player: cur, Plant: 7, Bid: 7}); err == nil {
		t.Fatal("future market plant accepted")
	}
	if err := g.Apply(Action{Type: ActChoose, Player: cur, Plant: 3, Bid: 2}); err == nil {
		t.Fatal("bid under minimum accepted")
	}
	if err := g.Apply(Action{Type: ActChoose, Player: cur, Plant: 3, Bid: 999}); err == nil {
		t.Fatal("unaffordable bid accepted")
	}
}

func TestAuctionRound(t *testing.T) {
	g := newTestGame(t, 3)
	first := g.CurrentPlayer()
	must := func(a Action) {
		t.Helper()
		a.Player = g.CurrentPlayer()
		if err := g.Apply(a); err != nil {
			t.Fatalf("%+v: %v", a, err)
		}
	}
	must(Action{Type: ActChoose, Plant: 4, Bid: 4})
	must(Action{Type: ActBid, Bid: 5})
	must(Action{Type: ActPass})
	if g.Auc == nil {
		t.Fatal("auction ended early")
	}
	must(Action{Type: ActPass}) // first player drops; second wins at 5
	if g.Auc != nil || g.CurrentPlayer() != first {
		t.Fatalf("chooser should choose again, got %d", g.CurrentPlayer())
	}
	must(Action{Type: ActChoose, Plant: 3, Bid: 3})
	must(Action{Type: ActPass})
	if len(g.Players[first].Plants) != 1 || g.Players[first].Money != StartMoney-3 {
		t.Fatalf("first player should own plant 3: %+v", g.Players[first])
	}
	// Last player buys alone at the minimum price.
	must(Action{Type: ActChoose, Plant: g.ActualMarket()[0], Bid: g.ActualMarket()[0]})
	if g.Phase != PhaseResources {
		t.Fatalf("phase %s", g.Phase)
	}
	// Turn order is set by plant size after round 1.
	for i := 1; i < len(g.Order); i++ {
		if highestPlant(&g.Players[g.Order[i-1]]) < highestPlant(&g.Players[g.Order[i]]) {
			t.Fatal("order not by highest plant")
		}
	}
}

func TestBuildCostsMatchBuildCost(t *testing.T) {
	g := newTestGame(t, 4)
	g.Phase, g.Turn = PhaseBuild, 0
	p := g.CurrentPlayer()
	var first int
	for c := range g.Board().Cities {
		if g.IsActive(c) {
			first = c
			break
		}
	}
	if err := g.Apply(Action{Type: ActBuild, Player: p, City: first}); err != nil {
		t.Fatal(err)
	}
	costs := g.BuildCosts(p)
	for c := range g.Board().Cities {
		want, err := g.BuildCost(p, c)
		if err != nil {
			want = -1
		}
		if costs[c] != want {
			t.Fatalf("city %d: %d vs %d", c, costs[c], want)
		}
	}
	if err := g.Apply(Action{Type: ActBuild, Player: p, City: first}); err == nil {
		t.Fatal("double build accepted")
	}
}

func TestViewHidesSecrets(t *testing.T) {
	g := newTestGame(t, 3)
	data, err := json.Marshal(g.View())
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, `"deck":`) || !strings.Contains(s, `"rng":{"state":0}`) {
		t.Fatalf("view leaks hidden state: %s", s)
	}
}

func TestLoadRejectsTampering(t *testing.T) {
	g := newTestGame(t, 3)
	good, _ := json.Marshal(g)
	if _, err := Load(good); err != nil {
		t.Fatalf("valid save rejected: %v", err)
	}
	tamper := []func(*Game){
		func(g *Game) { g.Players[0].Money = -5 },
		func(g *Game) { g.Players[0].Res[Coal] = 3 }, // no storage, breaks totals
		func(g *Game) { g.Market = append(g.Market, 3) },
		func(g *Game) { g.Order[0] = g.Order[1] },
		func(g *Game) { g.Players[1].Cities = []int{999} },
		func(g *Game) { g.Phase = "hacked" },
		func(g *Game) { g.Players = g.Players[:1] },
		func(g *Game) { g.ResMarket[Uranium] = 50 },
	}
	for i, f := range tamper {
		var c Game
		_ = json.Unmarshal(good, &c)
		f(&c)
		data, _ := json.Marshal(&c)
		if _, err := Load(data); err == nil {
			t.Errorf("tamper %d accepted", i)
		}
	}
	if _, err := Load([]byte(`{"players":`)); err == nil {
		t.Error("truncated save accepted")
	}
	if _, err := Load(make([]byte, 2<<20)); err == nil {
		t.Error("huge save accepted")
	}
}

func TestCleanName(t *testing.T) {
	if got := CleanName("  Ann\x00‮<b>  ", "x"); got != "Ann<b>" {
		t.Errorf("got %q", got)
	}
	if got := CleanName("\n\t", "fallback"); got != "fallback" {
		t.Errorf("got %q", got)
	}
	if got := CleanName(strings.Repeat("é", 40), "x"); len([]rune(got)) != MaxNameRunes {
		t.Errorf("not truncated: %q", got)
	}
}

func TestRegionsAreConnected(t *testing.T) {
	adj := classicBoard().RegionAdjacency()
	for seed := uint64(0); seed < 50; seed++ {
		for n := 2; n <= 6; n++ {
			cfg := Config{Seed: seed}
			for i := 0; i < n; i++ {
				cfg.Players = append(cfg.Players, PlayerConfig{})
			}
			g, _ := NewGame(cfg)
			if len(g.Regions) != regionsFor(n) {
				t.Fatalf("%d players got %d regions", n, len(g.Regions))
			}
			reached := map[int]bool{g.Regions[0]: true}
			for changed := true; changed; {
				changed = false
				for _, a := range g.Regions {
					for _, b := range g.Regions {
						if reached[a] && !reached[b] && adj[a][b] {
							reached[b], changed = true, true
						}
					}
				}
			}
			if len(reached) != len(g.Regions) {
				t.Fatalf("regions %v not connected", g.Regions)
			}
		}
	}
}

func TestViewNeverSendsNullLists(t *testing.T) {
	g := newTestGame(t, 3)
	g.Market = g.Market[:0]
	g.Step = 3
	data, _ := json.Marshal(g.View())
	for _, k := range []string{`"actual":null`, `"future":null`, `"market":null`} {
		if strings.Contains(string(data), k) {
			t.Errorf("view contains %s", k)
		}
	}
}

// powerPhase sets up player 0 with plants 5 (hybrid 2->1), 13 (eco 1),
// 15 (coal 2->3), 3 coal, 2 oil and 4 cities, and makes it their turn to
// power.
func powerPhase(t *testing.T) *Game {
	t.Helper()
	g := newTestGame(t, 3)
	p := &g.Players[0]
	p.Plants = []int{5, 13, 15}
	for _, n := range p.Plants {
		g.Market = removeInt(g.Market, n)
		g.Deck = removeInt(g.Deck, n)
	}
	p.Res = [NumResources]int{3, 2, 0, 0}
	g.ResMarket[Coal] -= 3
	g.ResMarket[Oil] -= 2
	p.Cities = []int{0, 1, 2, 3}
	for _, c := range p.Cities {
		g.Owners[c] = []int{0}
	}
	g.Phase, g.Turn, g.Order = PhasePower, 0, []int{0, 1, 2}
	return g
}

func TestPowerChoice(t *testing.T) {
	g := powerPhase(t)
	if v := g.View(); v.Suggest == nil || v.Suggest.Powered != 4 {
		t.Fatalf("suggestion %+v", g.View().Suggest)
	}
	// Run only the eco plant and the coal plant: 4 cities, 2 coal.
	if err := g.Apply(Action{Type: ActPower, Player: 0, Fire: []int{13, 15}}); err != nil {
		t.Fatal(err)
	}
	p := g.Players[0]
	if p.LastPowered != 4 || p.Res != [NumResources]int{1, 2, 0, 0} || p.LastIncome != Income(4) {
		t.Fatalf("got %+v", p)
	}
	if g.CurrentPlayer() != 1 || g.Phase != PhasePower {
		t.Fatal("turn should pass to the next player")
	}
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPowerNothingSavesFuel(t *testing.T) {
	g := powerPhase(t)
	money := g.Players[0].Money
	if err := g.Apply(Action{Type: ActPass, Player: 0}); err != nil {
		t.Fatal(err)
	}
	p := g.Players[0]
	if p.LastPowered != 0 || p.Money != money+Income(0) || p.Res != [NumResources]int{3, 2, 0, 0} {
		t.Fatalf("got %+v", p)
	}
}

func TestPowerHybridSplit(t *testing.T) {
	g := powerPhase(t)
	// Hybrid plant 5 burns 2: choose 1 coal + 1 oil.
	if err := g.Apply(Action{Type: ActPower, Player: 0, Fire: []int{5}, HybridCoal: 1}); err != nil {
		t.Fatal(err)
	}
	if got := g.Players[0].Res; got != [NumResources]int{2, 1, 0, 0} {
		t.Fatalf("res %v", got)
	}
}

func TestPowerRejectsBadChoices(t *testing.T) {
	bad := []Action{
		{Type: ActPower, Fire: []int{20}},                   // not owned
		{Type: ActPower, Fire: []int{15, 15}},               // repeated
		{Type: ActPower, Fire: []int{5}, HybridCoal: 3},     // hybrid only burns 2
		{Type: ActPower, Fire: []int{5, 15}, HybridCoal: 2}, // needs 4 coal, has 3
		{Type: ActBuild, City: 5},
	}
	for i, a := range bad {
		g := powerPhase(t)
		before := g.Players[0]
		if err := g.Apply(a); err == nil {
			t.Errorf("bad power choice %d accepted", i)
		}
		if g.Players[0].Res != before.Res || g.Turn != 0 {
			t.Errorf("rejected action %d changed state", i)
		}
	}
}

func TestPowerLastPlayerEndsRound(t *testing.T) {
	g := powerPhase(t)
	round := g.Round
	for i := 0; i < 3; i++ {
		if err := g.Apply(Action{Type: ActPass, Player: g.CurrentPlayer()}); err != nil {
			t.Fatal(err)
		}
	}
	if g.Round != round+1 || g.Phase != PhaseAuction {
		t.Fatalf("round %d phase %s", g.Round, g.Phase)
	}
}

func TestBuyAllIsAtomic(t *testing.T) {
	g := newTestGame(t, 3)
	g.Players[0].Plants = []int{4, 7} // coal 2 (stores 4), oil 3 (stores 6)
	g.Market = removeInt(removeInt(g.Market, 4), 7)
	g.Phase, g.Turn, g.Order = PhaseResources, 0, []int{1, 2, 0} // last in order buys first
	money, coal := g.Players[0].Money, g.ResMarket[Coal]

	// Too much coal for storage: nothing may change.
	if err := g.Apply(Action{Type: ActBuyAll, Player: 0, Amounts: [NumResources]int{5, 1, 0, 0}}); err == nil {
		t.Fatal("over-storage order accepted")
	}
	if g.Players[0].Money != money || g.ResMarket[Coal] != coal || g.Turn != 0 {
		t.Fatal("rejected order changed state")
	}
	for _, bad := range [][NumResources]int{{-1, 0, 0, 0}, {0, 0, 0, 99}, {0, 0, 1, 0}} {
		if err := g.Apply(Action{Type: ActBuyAll, Player: 0, Amounts: bad}); err == nil {
			t.Fatalf("bad order %v accepted", bad)
		}
	}

	want, err := g.OrderCost(0, [NumResources]int{4, 3, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Apply(Action{Type: ActBuyAll, Player: 0, Amounts: [NumResources]int{4, 3, 0, 0}}); err != nil {
		t.Fatal(err)
	}
	p := g.Players[0]
	if p.Res != [NumResources]int{4, 3, 0, 0} || p.Money != money-want || want != 1+1+1+2+3+3+3 {
		t.Fatalf("got %+v, cost %d", p, want)
	}
	if g.Turn != 1 || g.CurrentPlayer() != 2 {
		t.Fatal("confirming the order should end the turn")
	}
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestViewShowsRestock(t *testing.T) {
	g := newTestGame(t, 4)
	if v := g.View(); v.Restock[1] != [NumResources]int{6, 4, 3, 2} {
		t.Fatalf("restock %v", v.Restock)
	}
}
