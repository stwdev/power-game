// Package engine implements the rules of an electricity-network building
// game (an unofficial take on the classic "power grid" board game) as a
// deterministic state machine. Every change goes through Apply, which
// validates the action against the current state, so the same engine can
// later run on a server as the single source of truth for online play.
package engine

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

// Level is who controls a seat.
type Level int

const (
	Human Level = iota
	Easy
	Medium
	Hard
)

// Phase is the current part of a round.
type Phase string

const (
	PhaseAuction   Phase = "auction"
	PhaseResources Phase = "resources"
	PhaseBuild     Phase = "build"
	PhasePower     Phase = "power"
	PhaseOver      Phase = "over"
)

const (
	MinPlayers   = 2
	MaxPlayers   = 6
	StartMoney   = 50
	MaxNameRunes = 16
	maxLog       = 300
)

// SlotCost is the price of the 1st, 2nd and 3rd house in a city.
var SlotCost = [3]int{10, 15, 20}

// Player is one seat at the table.
type Player struct {
	Name        string            `json:"name"`
	AI          Level             `json:"ai"`
	Money       int               `json:"money"`
	Plants      []int             `json:"plants"`
	Res         [NumResources]int `json:"res"`
	Cities      []int             `json:"cities"`
	LastPowered int               `json:"lastPowered"`
	LastIncome  int               `json:"lastIncome"`
}

// Auction is a running auction for one plant.
type Auction struct {
	Plant      int   `json:"plant"`
	High       int   `json:"high"`
	HighBidder int   `json:"highBidder"`
	Bidders    []int `json:"bidders"` // still bidding, in turn order
	Next       int   `json:"next"`    // index into Bidders of who acts
}

// Game is the complete, serialisable game state.
type Game struct {
	Players      []Player          `json:"players"`
	Order        []int             `json:"order"` // turn order, first player first
	Round        int               `json:"round"`
	Step         int               `json:"step"`
	Phase        Phase             `json:"phase"`
	Market       []int             `json:"market"`  // sorted plant numbers
	Removed      []int             `json:"removed"` // plants publicly out of the game
	Deck         []int             `json:"deck,omitempty"`
	DeckSize     int               `json:"deckSize"`
	ResMarket    [NumResources]int `json:"resMarket"`
	Supply       [NumResources]int `json:"supply"`
	Owners       [][]int           `json:"owners"` // per city, players with a house there
	Regions      []int             `json:"regions"`
	Bought       []bool            `json:"bought"`
	Passed       []bool            `json:"passed"`
	JustBought   int               `json:"justBought"`
	Turn         int               `json:"turn"`
	Auc          *Auction          `json:"auction,omitempty"`
	DiscardFor   int               `json:"discardFor"`
	Step3Pending bool              `json:"step3Pending"`
	EndTriggered bool              `json:"endTriggered"`
	Ranking      []int             `json:"ranking,omitempty"`
	Log          []string          `json:"log"`
	RNG          RNG               `json:"rng"`

	// Quiet skips logging; AI simulations use it for speed.
	Quiet  bool `json:"-"`
	active []bool
}

// PlayerConfig describes a seat when starting a game.
type PlayerConfig struct {
	Name string `json:"name"`
	AI   Level  `json:"ai"`
}

// Config starts a new game.
type Config struct {
	Players []PlayerConfig `json:"players"`
	Seed    uint64         `json:"seed"`
}

// CleanName strips control characters and limits length. Names are shown
// to other players, so they are treated as untrusted text.
func CleanName(s string, fallback string) string {
	var b strings.Builder
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if n >= MaxNameRunes {
			break
		}
		if unicode.IsControl(r) || !unicode.IsPrint(r) {
			continue
		}
		b.WriteRune(r)
		n++
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return fallback
	}
	return out
}

// NewGame sets up a new game.
func NewGame(cfg Config) (*Game, error) {
	n := len(cfg.Players)
	if n < MinPlayers || n > MaxPlayers {
		return nil, fmt.Errorf("need %d-%d players", MinPlayers, MaxPlayers)
	}
	g := &Game{RNG: RNG{State: cfg.Seed}, Round: 1, Step: 1, DiscardFor: -1, JustBought: -1}
	for i, pc := range cfg.Players {
		if pc.AI < Human || pc.AI > Hard {
			return nil, errors.New("bad AI level")
		}
		g.Players = append(g.Players, Player{
			Name:   CleanName(pc.Name, fmt.Sprintf("Player %d", i+1)),
			AI:     pc.AI,
			Money:  StartMoney,
			Plants: []int{},
			Cities: []int{},
		})
	}
	g.Regions = g.pickRegions(regionsFor(n))
	g.computeActive()

	// Plants 3-10 start in the market; 13 goes on top of the deck and the
	// step 3 card at the bottom.
	var deck []int
	for _, p := range allPlants {
		switch {
		case p.Num <= 10:
			g.Market = append(g.Market, p.Num)
		case p.Num != 13:
			deck = append(deck, p.Num)
		}
	}
	Shuffle(&g.RNG, deck)
	remove := 0
	switch {
	case n <= 3:
		remove = 8
	case n == 4:
		remove = 4
	}
	deck = deck[remove:]
	g.Deck = append([]int{13}, deck...)
	g.Deck = append(g.Deck, Step3Card)
	g.DeckSize = len(g.Deck)

	g.ResMarket = startMarket
	for r := Resource(0); r < NumResources; r++ {
		g.Supply[r] = totalPieces[r] - startMarket[r]
	}
	g.Removed = []int{}
	g.Owners = make([][]int, len(board.Cities))
	for i := range g.Owners {
		g.Owners[i] = []int{}
	}
	g.Order = make([]int, n)
	for i := range g.Order {
		g.Order[i] = i
	}
	Shuffle(&g.RNG, g.Order)
	g.logf("Round 1 begins. Regions in play: %s.", g.regionList())
	g.beginAuction()
	return g, nil
}

func regionsFor(n int) int {
	switch {
	case n <= 3:
		return 3
	case n == 4:
		return 4
	default:
		return 5
	}
}

// pickRegions grows a random connected set of k regions.
func (g *Game) pickRegions(k int) []int {
	adj := board.RegionAdjacency()
	total := len(board.RegionNames)
	in := make([]bool, total)
	start := g.RNG.Intn(total)
	in[start] = true
	picked := []int{start}
	for len(picked) < k {
		var cand []int
		for r := 0; r < total; r++ {
			if in[r] {
				continue
			}
			for _, q := range picked {
				if adj[q][r] {
					cand = append(cand, r)
					break
				}
			}
		}
		if len(cand) == 0 {
			break
		}
		r := cand[g.RNG.Intn(len(cand))]
		in[r] = true
		picked = append(picked, r)
	}
	sort.Ints(picked)
	return picked
}

func (g *Game) computeActive() {
	g.active = make([]bool, len(board.Cities))
	for i, c := range board.Cities {
		for _, r := range g.Regions {
			if c.Region == r {
				g.active[i] = true
			}
		}
	}
}

// IsActive reports whether a city is in play.
func (g *Game) IsActive(city int) bool {
	if g.active == nil {
		g.computeActive()
	}
	return city >= 0 && city < len(g.active) && g.active[city]
}

func (g *Game) regionList() string {
	var names []string
	for _, r := range g.Regions {
		names = append(names, board.RegionNames[r])
	}
	return strings.Join(names, ", ")
}

func (g *Game) logf(format string, args ...any) {
	if g.Quiet {
		return
	}
	g.Log = append(g.Log, fmt.Sprintf(format, args...))
	if len(g.Log) > maxLog {
		g.Log = append([]string(nil), g.Log[len(g.Log)-maxLog:]...)
	}
}

// MaxPlants is how many plants a player may own.
func (g *Game) MaxPlants() int {
	if len(g.Players) == 2 {
		return 4
	}
	return 3
}

// Step2Trigger is the city count that starts step 2.
func (g *Game) Step2Trigger() int {
	switch len(g.Players) {
	case 2:
		return 10
	case 6:
		return 6
	default:
		return 7
	}
}

// EndTrigger is the city count that ends the game.
func (g *Game) EndTrigger() int {
	switch len(g.Players) {
	case 2:
		return 21
	case 3, 4:
		return 17
	case 5:
		return 15
	default:
		return 14
	}
}

// ActualMarket are the plants that may be auctioned now.
func (g *Game) ActualMarket() []int {
	if g.Step >= 3 {
		return append([]int{}, g.Market...)
	}
	k := min(4, len(g.Market))
	return append([]int{}, g.Market[:k]...)
}

// FutureMarket are the plants visible but not yet for sale.
func (g *Game) FutureMarket() []int {
	if g.Step >= 3 || len(g.Market) <= 4 {
		return []int{}
	}
	return append([]int{}, g.Market[4:]...)
}

func (g *Game) marketTarget() int {
	if g.Step >= 3 {
		return 6
	}
	return 8
}

func (g *Game) refill() {
	for len(g.Market) < g.marketTarget() && len(g.Deck) > 0 && !g.Step3Pending {
		c := g.Deck[0]
		g.Deck = g.Deck[1:]
		if c == Step3Card {
			g.Step3Pending = true
			g.logf("The Step 3 card was drawn.")
			continue
		}
		g.Market = append(g.Market, c)
		sort.Ints(g.Market)
	}
	g.DeckSize = len(g.Deck)
}

func (g *Game) removeLowestPlant() {
	if len(g.Market) == 0 {
		return
	}
	g.logf("Plant %d leaves the game.", g.Market[0])
	g.Removed = append(g.Removed, g.Market[0])
	g.Market = g.Market[1:]
	g.refill()
}

// cleanMarket drops plants that are no bigger than the largest network.
func (g *Game) cleanMarket() {
	for len(g.Market) > 0 && g.Market[0] <= g.MaxCities() {
		g.removeLowestPlant()
	}
}

func (g *Game) applyStep3() {
	if !g.Step3Pending {
		return
	}
	g.Step3Pending = false
	g.Step = 3
	g.logf("Step 3 begins: every plant in the market is for sale.")
	if len(g.Market) > 0 {
		g.Removed = append(g.Removed, g.Market[0])
		g.Market = g.Market[1:]
	}
	Shuffle(&g.RNG, g.Deck)
	for len(g.Market) > 6 {
		g.Deck = append(g.Deck, g.Market[len(g.Market)-1])
		g.Market = g.Market[:len(g.Market)-1]
	}
	g.refill()
}

// MaxCities is the largest network size at the table.
func (g *Game) MaxCities() int {
	m := 0
	for _, p := range g.Players {
		m = max(m, len(p.Cities))
	}
	return m
}

func highestPlant(p *Player) int {
	h := 0
	for _, n := range p.Plants {
		h = max(h, n)
	}
	return h
}

func (g *Game) determineOrder() {
	prev := make([]int, len(g.Players))
	for i, p := range g.Order {
		prev[p] = i
	}
	sort.SliceStable(g.Order, func(i, j int) bool {
		a, b := &g.Players[g.Order[i]], &g.Players[g.Order[j]]
		if len(a.Cities) != len(b.Cities) {
			return len(a.Cities) > len(b.Cities)
		}
		if ha, hb := highestPlant(a), highestPlant(b); ha != hb {
			return ha > hb
		}
		return prev[g.Order[i]] < prev[g.Order[j]]
	})
}

// CurrentPlayer is who must act next, or -1 when the game is over.
func (g *Game) CurrentPlayer() int {
	switch g.Phase {
	case PhaseAuction:
		if g.DiscardFor >= 0 {
			return g.DiscardFor
		}
		if g.Auc != nil {
			return g.Auc.Bidders[g.Auc.Next]
		}
		return g.chooser()
	case PhaseResources, PhaseBuild:
		if g.Turn >= 0 && g.Turn < len(g.Order) {
			return g.Order[len(g.Order)-1-g.Turn]
		}
	case PhasePower:
		if g.Turn >= 0 && g.Turn < len(g.Order) {
			return g.Order[g.Turn]
		}
	}
	return -1
}

func (g *Game) chooser() int {
	for _, p := range g.Order {
		if !g.Bought[p] && !g.Passed[p] {
			return p
		}
	}
	return -1
}

// CanPass reports whether the current player may pass.
func (g *Game) CanPass() bool {
	switch g.Phase {
	case PhaseAuction:
		if g.DiscardFor >= 0 {
			return false
		}
		if g.Auc != nil {
			return true
		}
		if g.Round > 1 {
			return true
		}
		p := g.CurrentPlayer()
		for _, n := range g.ActualMarket() {
			if n <= g.Players[p].Money {
				return false
			}
		}
		return true
	case PhaseResources, PhaseBuild, PhasePower:
		return true
	}
	return false
}

func (g *Game) beginAuction() {
	g.Phase = PhaseAuction
	n := len(g.Players)
	g.Bought = make([]bool, n)
	g.Passed = make([]bool, n)
	g.Auc = nil
	g.DiscardFor, g.JustBought = -1, -1
	g.logf("Round %d: plant auction.", g.Round)
	g.afterAuctionStep()
}

// afterAuctionStep moves on once nobody owes a discard or bid.
func (g *Game) afterAuctionStep() {
	if g.chooser() >= 0 {
		return
	}
	if g.Round == 1 {
		g.determineOrder()
	}
	anyBought := false
	for _, b := range g.Bought {
		anyBought = anyBought || b
	}
	if !anyBought {
		g.removeLowestPlant()
	}
	g.applyStep3()
	g.Phase = PhaseResources
	g.Turn = 0
	g.logf("Buying resources (last player first).")
}

// Action types.
const (
	ActChoose  = "choose"  // Plant, Bid: open an auction
	ActBid     = "bid"     // Bid: raise the current auction
	ActPass    = "pass"    // pass on choosing or bidding
	ActDiscard = "discard" // Plant: drop a plant when over the limit
	ActBuy     = "buy"     // Resource, Amount
	ActBuild   = "build"   // City
	ActDone    = "done"    // end resource buying or building
	ActPower   = "power"   // Fire, HybridCoal: run these plants (pass runs none)
	ActBuyAll  = "buyAll"  // Amounts: buy all of these at once, then finish
)

// Action is one player decision.
type Action struct {
	Type     string   `json:"type"`
	Player   int      `json:"player"`
	Plant    int      `json:"plant,omitempty"`
	Bid      int      `json:"bid,omitempty"`
	Resource Resource `json:"resource"`
	Amount   int      `json:"amount,omitempty"`
	City     int      `json:"city"`
	// Fire lists the plants to run; HybridCoal is how much of the hybrid
	// plants' fuel is coal (the rest is oil), or -1 to choose automatically.
	Fire       []int `json:"fire,omitempty"`
	HybridCoal int   `json:"hybridCoal,omitempty"`
	// Amounts is a whole fuel order for ActBuyAll, per resource.
	Amounts [NumResources]int `json:"amounts"`
}

// Apply validates and performs an action. On error the state is unchanged.
func (g *Game) Apply(a Action) error {
	cur := g.CurrentPlayer()
	if cur < 0 {
		return errors.New("game is over")
	}
	if a.Player != cur {
		return errors.New("not your turn")
	}
	p := &g.Players[cur]
	switch g.Phase {
	case PhaseAuction:
		return g.applyAuction(cur, p, a)
	case PhaseResources:
		return g.applyResources(cur, p, a)
	case PhaseBuild:
		return g.applyBuild(cur, p, a)
	case PhasePower:
		return g.applyPower(cur, p, a)
	}
	return errors.New("no action possible")
}

func contains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func (g *Game) applyAuction(cur int, p *Player, a Action) error {
	if g.DiscardFor >= 0 {
		if a.Type != ActDiscard {
			return errors.New("you must discard a plant")
		}
		if !contains(p.Plants, a.Plant) || a.Plant == g.JustBought {
			return errors.New("choose one of your older plants to discard")
		}
		p.Plants = removeInt(p.Plants, a.Plant)
		g.Removed = append(g.Removed, a.Plant)
		kept, removed := StorageOf(p.Plants).Trim(p.Res)
		p.Res = kept
		for r := range removed {
			g.Supply[r] += removed[r]
		}
		g.logf("%s scraps plant %d.", p.Name, a.Plant)
		g.DiscardFor, g.JustBought = -1, -1
		g.afterAuctionStep()
		return nil
	}

	if g.Auc != nil {
		auc := g.Auc
		switch a.Type {
		case ActBid:
			if a.Bid <= auc.High {
				return fmt.Errorf("bid more than %d", auc.High)
			}
			if a.Bid > p.Money {
				return errors.New("you cannot afford that bid")
			}
			auc.High, auc.HighBidder = a.Bid, cur
			auc.Next = (auc.Next + 1) % len(auc.Bidders)
			g.logf("%s bids %d.", p.Name, a.Bid)
		case ActPass:
			auc.Bidders = append(auc.Bidders[:auc.Next:auc.Next], auc.Bidders[auc.Next+1:]...)
			if auc.Next >= len(auc.Bidders) {
				auc.Next = 0
			}
			g.logf("%s drops out.", p.Name)
		default:
			return errors.New("bid or pass")
		}
		if len(auc.Bidders) == 1 {
			g.resolveAuction()
		}
		return nil
	}

	switch a.Type {
	case ActPass:
		if !g.CanPass() {
			return errors.New("everyone must buy a plant in round 1")
		}
		g.Passed[cur] = true
		g.logf("%s passes on buying a plant.", p.Name)
		g.afterAuctionStep()
		return nil
	case ActChoose:
		if !contains(g.ActualMarket(), a.Plant) {
			return errors.New("that plant is not for sale")
		}
		if a.Bid < a.Plant {
			return fmt.Errorf("minimum bid is %d", a.Plant)
		}
		if a.Bid > p.Money {
			return errors.New("you cannot afford that bid")
		}
		auc := &Auction{Plant: a.Plant, High: a.Bid, HighBidder: cur}
		for _, q := range g.Order {
			if !g.Bought[q] && !g.Passed[q] {
				auc.Bidders = append(auc.Bidders, q)
			}
		}
		for i, q := range auc.Bidders {
			if q == cur {
				auc.Next = (i + 1) % len(auc.Bidders)
			}
		}
		g.Auc = auc
		g.logf("%s offers %d for plant %d.", p.Name, a.Bid, a.Plant)
		if len(auc.Bidders) == 1 {
			g.resolveAuction()
		}
		return nil
	}
	return errors.New("choose a plant or pass")
}

func (g *Game) resolveAuction() {
	auc := g.Auc
	g.Auc = nil
	w := auc.HighBidder
	p := &g.Players[w]
	p.Money -= auc.High
	p.Plants = append(p.Plants, auc.Plant)
	sort.Ints(p.Plants)
	g.Bought[w] = true
	g.Market = removeInt(g.Market, auc.Plant)
	g.logf("%s buys plant %d for %d.", p.Name, auc.Plant, auc.High)
	g.refill()
	if len(p.Plants) > g.MaxPlants() {
		g.DiscardFor, g.JustBought = w, auc.Plant
		return
	}
	g.afterAuctionStep()
}

func removeInt(s []int, v int) []int {
	out := make([]int, 0, len(s))
	done := false
	for _, x := range s {
		if x == v && !done {
			done = true
			continue
		}
		out = append(out, x)
	}
	return out
}

// ResourceCost is the price of buying k units of r now; ok is false if the
// market does not hold that many.
func (g *Game) ResourceCost(r Resource, k int) (cost int, ok bool) {
	if r < 0 || r >= NumResources || k < 0 || k > g.ResMarket[r] {
		return 0, false
	}
	avail := g.ResMarket[r]
	for i := 0; i < k; i++ {
		cost += UnitPrice(r, avail)
		avail--
	}
	return cost, true
}

// CanStore reports whether player p could hold k more units of r.
func (g *Game) CanStore(p int, r Resource, k int) bool {
	if r < 0 || r >= NumResources || p < 0 || p >= len(g.Players) {
		return false
	}
	res := g.Players[p].Res
	res[r] += k
	return StorageOf(g.Players[p].Plants).Fits(res)
}

func (g *Game) applyResources(cur int, p *Player, a Action) error {
	switch a.Type {
	case ActBuy:
		if a.Resource < 0 || a.Resource >= NumResources {
			return errors.New("unknown resource")
		}
		if a.Amount < 1 || a.Amount > MarketCap(a.Resource) {
			return errors.New("bad amount")
		}
		cost, ok := g.ResourceCost(a.Resource, a.Amount)
		if !ok {
			return errors.New("not enough in the market")
		}
		if cost > p.Money {
			return errors.New("you cannot afford that")
		}
		if !g.CanStore(cur, a.Resource, a.Amount) {
			return errors.New("your plants cannot store that much")
		}
		p.Money -= cost
		p.Res[a.Resource] += a.Amount
		g.ResMarket[a.Resource] -= a.Amount
		g.logf("%s buys %d %s for %d.", p.Name, a.Amount, a.Resource, cost)
		return nil
	case ActBuyAll:
		cost, err := g.OrderCost(cur, a.Amounts)
		if err != nil {
			return err
		}
		p.Money -= cost
		var bought []string
		for r := Resource(0); r < NumResources; r++ {
			if k := a.Amounts[r]; k > 0 {
				p.Res[r] += k
				g.ResMarket[r] -= k
				bought = append(bought, fmt.Sprintf("%d %s", k, r))
			}
		}
		if len(bought) > 0 {
			g.logf("%s buys %s for %d.", p.Name, strings.Join(bought, ", "), cost)
		}
		g.endResourceTurn()
		return nil
	case ActDone, ActPass:
		g.endResourceTurn()
		return nil
	}
	return errors.New("buy resources or finish")
}

// OrderCost checks a whole fuel order for player p (market stock, storage
// and money) and returns its price. Nothing is changed.
func (g *Game) OrderCost(p int, amounts [NumResources]int) (int, error) {
	if p < 0 || p >= len(g.Players) {
		return 0, errors.New("no such player")
	}
	pl := &g.Players[p]
	res := pl.Res
	total := 0
	for r := Resource(0); r < NumResources; r++ {
		k := amounts[r]
		if k < 0 || k > MarketCap(r) {
			return 0, errors.New("bad amount")
		}
		cost, ok := g.ResourceCost(r, k)
		if !ok {
			return 0, fmt.Errorf("not enough %s in the market", r)
		}
		total += cost
		res[r] += k
	}
	if !StorageOf(pl.Plants).Fits(res) {
		return 0, errors.New("your plants cannot store that much")
	}
	if total > pl.Money {
		return 0, errors.New("you cannot afford that")
	}
	return total, nil
}

func (g *Game) endResourceTurn() {
	g.Turn++
	if g.Turn >= len(g.Order) {
		g.Phase = PhaseBuild
		g.Turn = 0
		g.logf("Building (last player first).")
	}
}

// BuildCost is what player p pays to build in city, or an error saying
// why they cannot.
func (g *Game) BuildCost(p, city int) (int, error) {
	if p < 0 || p >= len(g.Players) {
		return 0, errors.New("no such player")
	}
	if city < 0 || city >= len(board.Cities) || !g.IsActive(city) {
		return 0, errors.New("that city is not in play")
	}
	if contains(g.Owners[city], p) {
		return 0, errors.New("you are already there")
	}
	used := len(g.Owners[city])
	if used >= g.Step || used >= len(SlotCost) {
		return 0, errors.New("that city is full for this step")
	}
	cost := SlotCost[used]
	if pl := &g.Players[p]; len(pl.Cities) > 0 {
		d := board.Distances(pl.Cities, g.active)[city]
		if d == math.MaxInt {
			return 0, errors.New("no road to that city")
		}
		cost += d
	}
	return cost, nil
}

// BuildCosts is BuildCost for every city at once (-1 where p cannot
// build), computing the road network distances only once.
func (g *Game) BuildCosts(p int) []int {
	out := make([]int, len(board.Cities))
	if p < 0 || p >= len(g.Players) {
		for i := range out {
			out[i] = -1
		}
		return out
	}
	if g.active == nil {
		g.computeActive()
	}
	var dist []int
	if pl := &g.Players[p]; len(pl.Cities) > 0 {
		dist = board.Distances(pl.Cities, g.active)
	}
	for c := range out {
		used := len(g.Owners[c])
		if !g.active[c] || contains(g.Owners[c], p) || used >= g.Step || used >= len(SlotCost) {
			out[c] = -1
			continue
		}
		cost := SlotCost[used]
		if dist != nil {
			if dist[c] == math.MaxInt {
				out[c] = -1
				continue
			}
			cost += dist[c]
		}
		out[c] = cost
	}
	return out
}

func (g *Game) applyBuild(cur int, p *Player, a Action) error {
	switch a.Type {
	case ActBuild:
		cost, err := g.BuildCost(cur, a.City)
		if err != nil {
			return err
		}
		if cost > p.Money {
			return errors.New("you cannot afford that")
		}
		p.Money -= cost
		p.Cities = append(p.Cities, a.City)
		g.Owners[a.City] = append(g.Owners[a.City], cur)
		g.logf("%s builds in %s for %d.", p.Name, board.Cities[a.City].Name, cost)
		g.cleanMarket()
		return nil
	case ActDone, ActPass:
		g.Turn++
		if g.Turn >= len(g.Order) {
			g.endBuild()
		}
		return nil
	}
	return errors.New("build or finish")
}

func (g *Game) endBuild() {
	if g.Step == 1 && g.MaxCities() >= g.Step2Trigger() {
		g.Step = 2
		g.logf("Step 2 begins: cities now hold two houses.")
		g.removeLowestPlant()
	}
	g.cleanMarket()
	if g.MaxCities() >= g.EndTrigger() {
		g.EndTriggered = true
	}
	g.applyStep3()
	g.bureaucracy()
}

// bureaucracy starts the power phase: each player, in turn order, picks
// which plants to run (bots decide instantly).
func (g *Game) bureaucracy() {
	g.Phase = PhasePower
	g.Turn = 0
	g.logf("Powering cities.")
}

func (g *Game) applyPower(cur int, p *Player, a Action) error {
	var pw Powering
	switch a.Type {
	case ActPower:
		var err error
		pw, err = PlanPowering(p.Plants, p.Res, len(p.Cities), a.Fire, a.HybridCoal)
		if err != nil {
			return err
		}
	case ActPass:
		pw = Powering{Fired: []int{}}
	default:
		return errors.New("choose which plants to run")
	}
	for r := range pw.Use {
		p.Res[r] -= pw.Use[r]
		g.Supply[r] += pw.Use[r]
	}
	p.LastPowered = pw.Powered
	p.LastIncome = Income(pw.Powered)
	p.Money += p.LastIncome
	g.logf("%s powers %d cities and earns %d.", p.Name, pw.Powered, p.LastIncome)
	g.Turn++
	if g.Turn >= len(g.Order) {
		g.endRound()
	}
	return nil
}

// endRound finishes bureaucracy once everyone has been paid.
func (g *Game) endRound() {
	if g.EndTriggered {
		g.finish()
		return
	}
	table := resupply[len(g.Players)][g.Step-1]
	for r := Resource(0); r < NumResources; r++ {
		add := min(table[r], g.Supply[r], marketCap[r]-g.ResMarket[r])
		g.ResMarket[r] += add
		g.Supply[r] -= add
	}
	if len(g.Market) > 0 {
		if g.Step < 3 {
			top := g.Market[len(g.Market)-1]
			g.Market = g.Market[:len(g.Market)-1]
			g.Deck = append(g.Deck, top)
			g.refill()
		} else {
			g.removeLowestPlant()
		}
	}
	g.applyStep3()
	g.Round++
	g.determineOrder()
	g.beginAuction()
}

func (g *Game) finish() {
	g.Phase = PhaseOver
	g.Auc = nil
	g.Ranking = make([]int, len(g.Players))
	for i := range g.Ranking {
		g.Ranking[i] = i
	}
	sort.SliceStable(g.Ranking, func(i, j int) bool {
		a, b := &g.Players[g.Ranking[i]], &g.Players[g.Ranking[j]]
		if a.LastPowered != b.LastPowered {
			return a.LastPowered > b.LastPowered
		}
		if a.Money != b.Money {
			return a.Money > b.Money
		}
		return len(a.Cities) > len(b.Cities)
	})
	g.logf("Game over! %s wins.", g.Players[g.Ranking[0]].Name)
}
