package engine

import (
	"container/heap"
	"math"
)

// City is a node on the board.
type City struct {
	Name   string  `json:"name"`
	Region int     `json:"region"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

// Edge is a connection between two cities.
type Edge struct {
	A    int `json:"a"`
	B    int `json:"b"`
	Cost int `json:"cost"`
}

// Board is a map: cities grouped into regions, joined by roads. Boards are
// never loaded from outside; they are rebuilt from a MapSpec.
type Board struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Seed        uint64   `json:"seed,omitempty"` // random maps only
	Code        string   `json:"code,omitempty"` // shareable form of Seed
	Cities      []City   `json:"cities"`
	Edges       []Edge   `json:"edges"`
	RegionNames []string `json:"regionNames"`
	Width       float64  `json:"width"`
	Height      float64  `json:"height"`
	adj         [][]Edge
}

// Cities are authored on a 900x600 landscape grid (regions 3 wide, 2 tall)
// and transposed so the board fits a portrait phone screen.
var cityData = []City{
	{"Aldmoor", 0, 60, 50}, {"Brindle", 0, 170, 40}, {"Corrow", 0, 250, 110}, {"Dunhallow", 0, 90, 140},
	{"Eskvale", 0, 190, 170}, {"Fernby", 0, 50, 250}, {"Glimmerford", 0, 150, 260},
	{"Harrowgate", 1, 350, 60}, {"Ironwick", 1, 460, 45}, {"Jessamy", 1, 560, 90}, {"Kestrel Bay", 1, 390, 160},
	{"Larkspur", 1, 510, 180}, {"Millbrook", 1, 330, 260}, {"Northcote", 1, 450, 265},
	{"Oakhaven", 2, 650, 40}, {"Pellmar", 2, 760, 70}, {"Quillon", 2, 860, 40}, {"Ravensport", 2, 680, 160},
	{"Saltmere", 2, 830, 160}, {"Thornby", 2, 720, 250}, {"Ulvenmoor", 2, 860, 270},
	{"Valecrest", 3, 70, 340}, {"Wyndale", 3, 200, 330}, {"Yarrowfield", 3, 130, 420}, {"Zephyr Point", 3, 40, 500},
	{"Ashcombe", 3, 250, 430}, {"Bramblecourt", 3, 120, 560}, {"Cinderholt", 3, 230, 550},
	{"Dawnridge", 4, 360, 360}, {"Emberly", 4, 480, 340}, {"Foxhollow", 4, 570, 400}, {"Greywater", 4, 420, 450},
	{"Hollin", 4, 330, 530}, {"Ivymere", 4, 480, 540}, {"Juniper Falls", 4, 570, 520},
	{"Kingsferry", 5, 660, 350}, {"Lowmarsh", 5, 780, 340}, {"Marrow Cove", 5, 870, 400}, {"Nettlefield", 5, 700, 450},
	{"Orchard Glen", 5, 830, 480}, {"Pinecrest", 5, 650, 560}, {"Quarry Hill", 5, 790, 570},
}

var regionNames = []string{"Frostmark", "Highreach", "Stormcoast", "Redlands", "Goldplain", "Sunreach"}

func classicBoard() *Board {
	b := &Board{ID: "classic", Name: "Classic", RegionNames: regionNames, Width: 640, Height: 940}
	for _, c := range cityData {
		b.Cities = append(b.Cities, City{Name: c.Name, Region: c.Region, X: c.Y + 20, Y: c.X + 20})
	}
	b.connect()
	return b
}

// connect builds the roads between the board's cities.
func (b *Board) connect() {
	// Gabriel graph: connect a and b when no other city lies inside the
	// circle whose diameter is ab. It is planar and connected, so it gives
	// a clean road network without hand-authoring edges.
	b.Edges = nil
	n := len(b.Cities)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			ci, cj := b.Cities[i], b.Cities[j]
			mx, my := (ci.X+cj.X)/2, (ci.Y+cj.Y)/2
			r2 := sq(ci.X-cj.X)/4 + sq(ci.Y-cj.Y)/4
			blocked := false
			for k := 0; k < n && !blocked; k++ {
				if k != i && k != j && sq(b.Cities[k].X-mx)+sq(b.Cities[k].Y-my) < r2 {
					blocked = true
				}
			}
			if blocked {
				continue
			}
			d := math.Hypot(ci.X-cj.X, ci.Y-cj.Y)
			cost := int(math.Round(d / 8))
			if cost < 3 {
				cost = 3
			}
			b.Edges = append(b.Edges, Edge{A: i, B: j, Cost: cost})
		}
	}
	b.adj = make([][]Edge, n)
	for _, e := range b.Edges {
		b.adj[e.A] = append(b.adj[e.A], e)
		b.adj[e.B] = append(b.adj[e.B], Edge{A: e.B, B: e.A, Cost: e.Cost})
	}
}

func sq(x float64) float64 { return x * x }

// RegionAdjacency reports which regions share a road.
func (b *Board) RegionAdjacency() [][]bool {
	r := len(b.RegionNames)
	adj := make([][]bool, r)
	for i := range adj {
		adj[i] = make([]bool, r)
	}
	for _, e := range b.Edges {
		ra, rb := b.Cities[e.A].Region, b.Cities[e.B].Region
		adj[ra][rb], adj[rb][ra] = true, true
	}
	return adj
}

// Distances returns the cheapest connection cost from any source city to
// every city, travelling only through cities where allowed is true.
// Unreachable cities get math.MaxInt.
func (b *Board) Distances(sources []int, allowed []bool) []int {
	dist := make([]int, len(b.Cities))
	for i := range dist {
		dist[i] = math.MaxInt
	}
	pq := &distHeap{}
	for _, s := range sources {
		if s >= 0 && s < len(dist) && dist[s] != 0 {
			dist[s] = 0
			heap.Push(pq, distItem{s, 0})
		}
	}
	for pq.Len() > 0 {
		it := heap.Pop(pq).(distItem)
		if it.d > dist[it.city] {
			continue
		}
		for _, e := range b.adj[it.city] {
			if !allowed[e.B] {
				continue
			}
			nd := it.d + e.Cost
			if nd < dist[e.B] {
				dist[e.B] = nd
				heap.Push(pq, distItem{e.B, nd})
			}
		}
	}
	return dist
}

type distItem struct{ city, d int }
type distHeap []distItem

func (h distHeap) Len() int           { return len(h) }
func (h distHeap) Less(i, j int) bool { return h[i].d < h[j].d }
func (h distHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *distHeap) Push(x any)        { *h = append(*h, x.(distItem)) }
func (h *distHeap) Pop() any {
	old := *h
	it := old[len(old)-1]
	*h = old[:len(old)-1]
	return it
}
