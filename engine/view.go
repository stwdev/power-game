package engine

import (
	"encoding/json"
	"errors"
	"fmt"
)

// View is what a client may see: the game without the hidden deck order
// or RNG state, plus values the UI needs for the player to move.
type View struct {
	Game       *Game             `json:"game"`
	Current    int               `json:"current"`
	Actual     []int             `json:"actual"`
	Future     []int             `json:"future"`
	Prices     [NumResources]int `json:"prices"`
	CanPass    bool              `json:"canPass"`
	BuildCosts []int             `json:"buildCosts,omitempty"` // -1 when not buildable
	Suggest    *Powering         `json:"suggest,omitempty"`    // best powering, in the power phase
	Restock    RestockTable      `json:"restock"`              // units added per round, by step
	Custom     bool              `json:"customRestock"`
	Step2At    int               `json:"step2At"`
	EndAt      int               `json:"endAt"`
	MaxPlants  int               `json:"maxPlants"`
}

// View builds the redacted client view.
func (g *Game) View() View {
	cp := *g
	cp.Deck = nil
	cp.RNG = RNG{}
	if cp.Market == nil {
		cp.Market = []int{}
	}
	if cp.Log == nil {
		cp.Log = []string{}
	}
	cur := g.CurrentPlayer()
	v := View{
		Game:      &cp,
		Current:   cur,
		Actual:    g.ActualMarket(),
		Future:    g.FutureMarket(),
		CanPass:   cur >= 0 && g.CanPass(),
		Step2At:   g.Step2Trigger(),
		EndAt:     g.EndTrigger(),
		MaxPlants: g.MaxPlants(),
		Restock:   g.Restock(),
		Custom:    g.CustomRestock != nil,
	}
	for r := Resource(0); r < NumResources; r++ {
		v.Prices[r] = UnitPrice(r, g.ResMarket[r])
	}
	if g.Phase == PhasePower && cur >= 0 {
		p := &g.Players[cur]
		best := BestPowering(p.Plants, p.Res, len(p.Cities))
		v.Suggest = &best
	}
	if g.Phase == PhaseBuild && v.Current >= 0 {
		v.BuildCosts = g.BuildCosts(v.Current)
	}
	return v
}

// Load parses and validates a saved game. Saves are treated as untrusted
// input: anything inconsistent is rejected rather than trusted.
func Load(data []byte) (*Game, error) {
	if len(data) > 1<<20 {
		return nil, errors.New("save too large")
	}
	var g Game
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("bad save: %w", err)
	}
	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("bad save: %w", err)
	}
	g.computeActive()
	return &g, nil
}

// Validate checks that the state is internally consistent.
func (g *Game) Validate() error {
	n := len(g.Players)
	if n < MinPlayers || n > MaxPlayers {
		return errors.New("player count")
	}
	b, err := BuildBoard(g.Map)
	if err != nil {
		return err
	}
	if g.Map.ID == "" {
		g.Map.ID = "classic"
	}
	g.board = b
	if len(g.Order) != n || len(g.Bought) != n || len(g.Passed) != n {
		return errors.New("table size")
	}
	seen := make([]bool, n)
	for _, p := range g.Order {
		if p < 0 || p >= n || seen[p] {
			return errors.New("turn order")
		}
		seen[p] = true
	}
	if g.Step < 1 || g.Step > 3 || g.Round < 1 || g.Round > 1000 {
		return errors.New("step or round")
	}
	switch g.Phase {
	case PhaseAuction, PhaseResources, PhaseBuild, PhasePower, PhaseOver:
	default:
		return errors.New("phase")
	}
	if g.Turn < 0 || g.Turn > n {
		return errors.New("turn")
	}
	if len(g.Owners) != len(g.Board().Cities) {
		return errors.New("owners")
	}
	if len(g.Regions) < 1 || len(g.Regions) > len(g.Board().RegionNames) {
		return errors.New("regions")
	}
	for _, r := range g.Regions {
		if r < 0 || r >= len(g.Board().RegionNames) {
			return errors.New("region")
		}
	}
	plantSeen := map[int]bool{}
	checkPlant := func(num int) error {
		if num == Step3Card {
			if plantSeen[num] {
				return errors.New("duplicate step 3 card")
			}
			plantSeen[num] = true
			return nil
		}
		if _, ok := PlantByNum(num); !ok || plantSeen[num] {
			return fmt.Errorf("plant %d", num)
		}
		plantSeen[num] = true
		return nil
	}
	for _, num := range g.Market {
		if num == Step3Card {
			return errors.New("step 3 card in market")
		}
		if err := checkPlant(num); err != nil {
			return err
		}
	}
	for _, num := range g.Deck {
		if err := checkPlant(num); err != nil {
			return err
		}
	}
	for _, num := range g.Removed {
		if num == Step3Card {
			return errors.New("step 3 card removed")
		}
		if err := checkPlant(num); err != nil {
			return err
		}
	}
	var held [NumResources]int
	for i := range g.Players {
		p := &g.Players[i]
		p.Name = CleanName(p.Name, fmt.Sprintf("Player %d", i+1))
		if p.AI < Human || p.AI > Hard || p.Money < 0 || p.Money > 100000 {
			return errors.New("player")
		}
		if len(p.Plants) > g.MaxPlants()+1 {
			return errors.New("too many plants")
		}
		for _, num := range p.Plants {
			if num == Step3Card {
				return errors.New("step 3 card held")
			}
			if err := checkPlant(num); err != nil {
				return err
			}
		}
		for r := range p.Res {
			if p.Res[r] < 0 {
				return errors.New("resources")
			}
			held[r] += p.Res[r]
		}
		if !StorageOf(p.Plants).Fits(p.Res) {
			return errors.New("storage")
		}
		citySeen := map[int]bool{}
		for _, c := range p.Cities {
			if c < 0 || c >= len(g.Board().Cities) || citySeen[c] || !contains(g.Owners[c], i) {
				return errors.New("cities")
			}
			citySeen[c] = true
		}
	}
	for c, owners := range g.Owners {
		if len(owners) > 3 {
			return errors.New("city slots")
		}
		for _, o := range owners {
			if o < 0 || o >= n || !contains(g.Players[o].Cities, c) {
				return errors.New("city owner")
			}
		}
	}
	for r := Resource(0); r < NumResources; r++ {
		if g.ResMarket[r] < 0 || g.ResMarket[r] > marketCap[r] || g.Supply[r] < 0 ||
			g.ResMarket[r]+g.Supply[r]+held[r] != totalPieces[r] {
			return errors.New("resource totals")
		}
	}
	if g.CustomRestock != nil {
		if err := g.CustomRestock.Check(); err != nil {
			return err
		}
	}
	if g.DiscardFor < -1 || g.DiscardFor >= n {
		return errors.New("discard")
	}
	if g.Auc != nil {
		a := g.Auc
		if len(a.Bidders) < 2 || a.Next < 0 || a.Next >= len(a.Bidders) ||
			a.HighBidder < 0 || a.HighBidder >= n || !contains(g.Market, a.Plant) {
			return errors.New("auction")
		}
		for _, b := range a.Bidders {
			if b < 0 || b >= n {
				return errors.New("auction bidder")
			}
		}
	}
	if g.Phase == PhaseAuction && g.DiscardFor < 0 && g.Auc == nil && g.chooser() < 0 {
		return errors.New("stalled auction")
	}
	if g.Phase == PhaseOver {
		if len(g.Ranking) != n {
			return errors.New("ranking")
		}
		for _, r := range g.Ranking {
			if r < 0 || r >= n {
				return errors.New("ranking")
			}
		}
	}
	if len(g.Log) > maxLog {
		g.Log = g.Log[len(g.Log)-maxLog:]
	}
	for i, l := range g.Log {
		if len(l) > 200 {
			g.Log[i] = l[:200]
		}
	}
	g.DeckSize = len(g.Deck)
	return nil
}
