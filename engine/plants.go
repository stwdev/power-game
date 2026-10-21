package engine

// Resource is a fuel type traded on the resource market.
type Resource int

const (
	Coal Resource = iota
	Oil
	Garbage
	Uranium
	NumResources
)

var resourceNames = [NumResources]string{"coal", "oil", "garbage", "uranium"}

func (r Resource) String() string {
	if r < 0 || r >= NumResources {
		return "?"
	}
	return resourceNames[r]
}

// Kind is what a power plant burns.
type Kind int

const (
	KindCoal Kind = iota
	KindOil
	KindGarbage
	KindUranium
	KindHybrid // burns coal and/or oil
	KindEco    // needs no fuel
)

var kindNames = []string{"coal", "oil", "garbage", "uranium", "hybrid", "eco"}

func (k Kind) String() string {
	if k < 0 || int(k) >= len(kindNames) {
		return "?"
	}
	return kindNames[k]
}

// Plant is a power plant card. Num is also its minimum bid.
type Plant struct {
	Num    int  `json:"num"`
	Kind   Kind `json:"kind"`
	Input  int  `json:"input"`  // fuel units burned per activation
	Cities int  `json:"cities"` // cities powered per activation
}

// Step3Card is the deck marker that starts step 3 when drawn.
const Step3Card = -1

var allPlants = []Plant{
	{3, KindOil, 2, 1}, {4, KindCoal, 2, 1}, {5, KindHybrid, 2, 1}, {6, KindGarbage, 1, 1},
	{7, KindOil, 3, 2}, {8, KindCoal, 3, 2}, {9, KindOil, 1, 1}, {10, KindCoal, 2, 2},
	{11, KindUranium, 1, 2}, {12, KindHybrid, 2, 2}, {13, KindEco, 0, 1}, {14, KindGarbage, 2, 2},
	{15, KindCoal, 2, 3}, {16, KindOil, 2, 3}, {17, KindUranium, 1, 2}, {18, KindEco, 0, 2},
	{19, KindGarbage, 2, 3}, {20, KindCoal, 3, 5}, {21, KindHybrid, 2, 4}, {22, KindEco, 0, 2},
	{23, KindUranium, 1, 3}, {24, KindGarbage, 2, 4}, {25, KindCoal, 2, 5}, {26, KindOil, 2, 5},
	{27, KindEco, 0, 3}, {28, KindUranium, 1, 4}, {29, KindHybrid, 1, 4}, {30, KindGarbage, 3, 6},
	{31, KindCoal, 3, 6}, {32, KindOil, 3, 6}, {33, KindEco, 0, 4}, {34, KindUranium, 1, 5},
	{35, KindOil, 1, 5}, {36, KindCoal, 3, 7}, {37, KindEco, 0, 4}, {38, KindGarbage, 3, 7},
	{39, KindUranium, 1, 6}, {40, KindOil, 2, 6}, {42, KindCoal, 2, 6}, {44, KindEco, 0, 5},
	{46, KindHybrid, 3, 7}, {50, KindEco, 0, 6},
}

var plantIndex = func() map[int]Plant {
	m := make(map[int]Plant, len(allPlants))
	for _, p := range allPlants {
		m[p.Num] = p
	}
	return m
}()

// PlantByNum looks up a plant card.
func PlantByNum(n int) (Plant, bool) {
	p, ok := plantIndex[n]
	return p, ok
}

// AllPlants returns a copy of every plant card.
func AllPlants() []Plant {
	return append([]Plant(nil), allPlants...)
}

// Payment is the income for powering n cities.
var Payment = []int{10, 22, 33, 44, 54, 64, 73, 82, 90, 98, 105, 112, 118, 124, 129, 134, 138, 142, 145, 148, 150}

// Income returns the payout for powering n cities.
func Income(n int) int {
	if n < 0 {
		n = 0
	}
	if n >= len(Payment) {
		n = len(Payment) - 1
	}
	return Payment[n]
}

// Market capacity per resource and total pieces in the game.
var (
	marketCap   = [NumResources]int{24, 24, 24, 12}
	totalPieces = [NumResources]int{24, 24, 24, 12}
	startMarket = [NumResources]int{24, 18, 6, 2}
	uraniumCost = []int{1, 2, 3, 4, 5, 6, 7, 8, 10, 12, 14, 16}
)

// resupply[players][step-1][resource]
var resupply = map[int][3][NumResources]int{
	2: {{3, 2, 1, 1}, {4, 2, 2, 1}, {3, 4, 3, 1}},
	3: {{4, 2, 1, 1}, {5, 3, 2, 1}, {3, 4, 3, 1}},
	4: {{5, 3, 2, 1}, {6, 4, 3, 2}, {4, 5, 4, 2}},
	5: {{5, 4, 3, 2}, {7, 5, 3, 3}, {5, 6, 5, 2}},
	6: {{7, 5, 3, 2}, {9, 6, 5, 3}, {6, 7, 6, 3}},
}

// UnitPrice is the price of the cheapest unit when avail units are in the market.
// It returns 0 when the market is empty.
func UnitPrice(r Resource, avail int) int {
	if avail <= 0 || r < 0 || r >= NumResources {
		return 0
	}
	idx := marketCap[r] - avail
	if r == Uranium {
		return uraniumCost[idx]
	}
	return idx/3 + 1
}

// MarketCap is the most units of r the market can hold.
func MarketCap(r Resource) int { return marketCap[r] }

// TotalPieces is the number of units of r in the game.
func TotalPieces(r Resource) int { return totalPieces[r] }

// Capacity is how much fuel a set of plants can store.
type Capacity struct {
	Coal, Oil, Hybrid, Garbage, Uranium int
}

// StorageOf sums the storage (twice the input) of the given plants.
func StorageOf(plants []int) Capacity {
	var c Capacity
	for _, n := range plants {
		p, ok := PlantByNum(n)
		if !ok {
			continue
		}
		s := 2 * p.Input
		switch p.Kind {
		case KindCoal:
			c.Coal += s
		case KindOil:
			c.Oil += s
		case KindHybrid:
			c.Hybrid += s
		case KindGarbage:
			c.Garbage += s
		case KindUranium:
			c.Uranium += s
		}
	}
	return c
}

// Fits reports whether res can be stored in this capacity.
func (c Capacity) Fits(res [NumResources]int) bool {
	return res[Coal] <= c.Coal+c.Hybrid &&
		res[Oil] <= c.Oil+c.Hybrid &&
		res[Coal]+res[Oil] <= c.Coal+c.Oil+c.Hybrid &&
		res[Garbage] <= c.Garbage &&
		res[Uranium] <= c.Uranium
}

// Trim returns res reduced until it fits, and how much was removed.
func (c Capacity) Trim(res [NumResources]int) (kept, removed [NumResources]int) {
	kept = res
	clamp := func(r Resource, max int) {
		if kept[r] > max {
			removed[r] += kept[r] - max
			kept[r] = max
		}
	}
	clamp(Coal, c.Coal+c.Hybrid)
	clamp(Oil, c.Oil+c.Hybrid)
	clamp(Garbage, c.Garbage)
	clamp(Uranium, c.Uranium)
	for kept[Coal]+kept[Oil] > c.Coal+c.Oil+c.Hybrid {
		if kept[Coal]-c.Coal >= kept[Oil]-c.Oil {
			kept[Coal]--
			removed[Coal]++
		} else {
			kept[Oil]--
			removed[Oil]++
		}
	}
	return kept, removed
}

// Powering is a choice of plants to fire in bureaucracy.
type Powering struct {
	Powered int               `json:"powered"`
	Fired   []int             `json:"fired"`
	Use     [NumResources]int `json:"use"`
}

// BestPowering picks the plants that power the most cities (capped at
// cities), preferring the option that burns the least fuel.
func BestPowering(plants []int, res [NumResources]int, cities int) Powering {
	best := Powering{Powered: -1}
	bestFuel := 0
	n := len(plants)
	for mask := 0; mask < 1<<n; mask++ {
		var need [NumResources]int
		hybrid, capSum := 0, 0
		var fired []int
		for i := 0; i < n; i++ {
			if mask&(1<<i) == 0 {
				continue
			}
			p, ok := PlantByNum(plants[i])
			if !ok {
				continue
			}
			fired = append(fired, p.Num)
			capSum += p.Cities
			switch p.Kind {
			case KindCoal:
				need[Coal] += p.Input
			case KindOil:
				need[Oil] += p.Input
			case KindGarbage:
				need[Garbage] += p.Input
			case KindUranium:
				need[Uranium] += p.Input
			case KindHybrid:
				hybrid += p.Input
			}
		}
		ok := true
		for r := Resource(0); r < NumResources; r++ {
			if need[r] > res[r] {
				ok = false
			}
		}
		if !ok {
			continue
		}
		spareCoal, spareOil := res[Coal]-need[Coal], res[Oil]-need[Oil]
		if spareCoal+spareOil < hybrid {
			continue
		}
		// Burn hybrid fuel from whichever pile is larger.
		for h := hybrid; h > 0; h-- {
			if spareCoal >= spareOil {
				spareCoal--
				need[Coal]++
			} else {
				spareOil--
				need[Oil]++
			}
		}
		powered := capSum
		if powered > cities {
			powered = cities
		}
		fuel := need[Coal] + need[Oil] + need[Garbage] + need[Uranium]
		if powered > best.Powered || (powered == best.Powered && fuel < bestFuel) {
			best = Powering{Powered: powered, Fired: fired, Use: need}
			bestFuel = fuel
		}
	}
	if best.Powered < 0 {
		best.Powered = 0
	}
	return best
}
