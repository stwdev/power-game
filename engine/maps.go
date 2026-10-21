package engine

import (
	"errors"
	"math"
	"strings"
)

// MapSpec says which board a game is played on. Only the spec is saved;
// the board itself is always rebuilt from it, so a save file cannot carry
// a hand-crafted map.
type MapSpec struct {
	ID   string `json:"id"`             // see Maps()
	Seed uint64 `json:"seed,omitempty"` // random maps only, below MaxMapSeed
}

// MaxMapSeed bounds random-map seeds: 40 bits fit an 8-character map code
// and stay exact as JavaScript numbers.
const MaxMapSeed = 1 << 40

// MapInfo describes a selectable map.
type MapInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Maps lists the selectable maps.
func Maps() []MapInfo {
	return []MapInfo{
		{"classic", "Classic", "Six regions in a two-by-three grid."},
		{"lakeside", "Lakeside", "Six regions ringed around a great lake."},
		{"ridge", "Ridge", "A long zig-zag of six regions: expensive to cross."},
		{"random", "Random", "A brand-new map. Share its code to replay it."},
	}
}

// canvas size for generated maps (portrait, like the classic board).
const (
	mapW, mapH = 640.0, 940.0
	mapMargin  = 40.0
)

// layouts are region centres for generated maps.
var layouts = map[string][][2]float64{
	"grid":  {{170, 170}, {470, 170}, {170, 470}, {470, 470}, {170, 770}, {470, 770}},
	"ring":  ring(),
	"ridge": {{190, 110}, {450, 255}, {190, 400}, {450, 545}, {190, 690}, {450, 835}},
}

func ring() [][2]float64 {
	var c [][2]float64
	for i := 0; i < 6; i++ {
		a := -math.Pi/2 + float64(i)*math.Pi/3
		c = append(c, [2]float64{320 + 205*math.Cos(a), 470 + 335*math.Sin(a)})
	}
	return c
}

var (
	lakesideRegions = []string{"North Shore", "Reedbank", "Eastmere", "South Shore", "Willowfen", "Westmere"}
	ridgeRegions    = []string{"Summit", "Crag", "Highpass", "Scree", "Foothill", "Valley"}
	randomRegions   = []string{"Amberlands", "Brightwater", "Coldmarch", "Deepwood", "Emberfall", "Farreach",
		"Greystone", "Hollowmere", "Ironvale", "Juniperlands", "Kestrelmoor", "Lowfen", "Mistral", "Northmarch",
		"Oakshire", "Pinefold", "Quietdale", "Rivermark", "Saltmarsh", "Thornwold"}
	namePrefixes = []string{"Ash", "Bram", "Cold", "Dun", "Elm", "Fern", "Glen", "Hart", "Iron", "Kings", "Lark",
		"Mill", "North", "Oak", "Pine", "Raven", "Salt", "Thorn", "Wester", "Wyn", "Bright", "Stone", "Black",
		"Red", "Fox", "Marsh", "Clay", "Hazel", "Wolf", "Swan", "Rook", "Briar", "Copper", "Frost", "Gold", "Moss"}
	nameSuffixes = []string{"ford", "bury", "wick", "dale", "moor", "ton", "field", "haven", "port", "stead",
		"gate", "mouth", "holm", "by", "crest", "combe", "well", "ridge", "mere", "ley", "brook", "hollow"}
)

// BuildBoard creates the board for a spec, or explains why the spec is bad.
func BuildBoard(spec MapSpec) (*Board, error) {
	switch spec.ID {
	case "", "classic":
		return classicBoard(), nil
	case "lakeside":
		return generate("lakeside", "Lakeside", layouts["ring"], lakesideRegions, 0x1a4e51de)
	case "ridge":
		return generate("ridge", "Ridge", layouts["ridge"], ridgeRegions, 0x41d6e)
	case "random":
		if spec.Seed >= MaxMapSeed {
			return nil, errors.New("bad map code")
		}
		r := RNG{State: spec.Seed ^ 0x6d61702d73656564}
		var centres [][2]float64
		switch r.Intn(4) {
		case 0:
			centres = layouts["grid"]
		case 1:
			centres = layouts["ring"]
		case 2:
			centres = layouts["ridge"]
		default:
			centres = scatter(&r)
		}
		// Nudge the centres so even the stock layouts come out different.
		moved := make([][2]float64, len(centres))
		for i, c := range centres {
			moved[i] = [2]float64{c[0] + float64(r.Intn(61)-30), c[1] + float64(r.Intn(61)-30)}
		}
		names := append([]string(nil), randomRegions...)
		Shuffle(&r, names)
		b, err := generate("random", "Random", moved, names[:6], r.Next())
		if err != nil {
			return nil, err
		}
		b.Seed, b.Code = spec.Seed, MapCode(spec.Seed)
		return b, nil
	}
	return nil, errors.New("unknown map")
}

// scatter places six region centres at random, well apart.
func scatter(r *RNG) [][2]float64 {
	for attempt := 0; attempt < 200; attempt++ {
		var c [][2]float64
		for tries := 0; len(c) < 6 && tries < 400; tries++ {
			p := [2]float64{130 + float64(r.Intn(381)), 130 + float64(r.Intn(681))}
			ok := true
			for _, q := range c {
				if math.Hypot(p[0]-q[0], p[1]-q[1]) < 250 {
					ok = false
					break
				}
			}
			if ok {
				c = append(c, p)
			}
		}
		if len(c) == 6 {
			return c
		}
	}
	return layouts["grid"]
}

// generate scatters 7-8 cities around each region centre, assigns every
// city to its nearest centre (so regions are contiguous), names them and
// lays roads. It retries until every region is big enough and the regions
// form one connected land.
func generate(id, name string, centres [][2]float64, regions []string, seed uint64) (*Board, error) {
	r := RNG{State: seed}
	for attempt := 0; attempt < 100; attempt++ {
		var pts [][2]float64
		for _, c := range centres {
			want := 7 + r.Intn(2)
			got := 0
			for tries := 0; got < want && tries < 600; tries++ {
				ang := float64(r.Intn(3600)) * math.Pi / 1800
				rad := 150 * math.Sqrt(float64(r.Intn(1001))/1000)
				p := [2]float64{c[0] + rad*math.Cos(ang), c[1] + rad*math.Sin(ang)}
				if p[0] < mapMargin || p[0] > mapW-mapMargin || p[1] < mapMargin || p[1] > mapH-mapMargin {
					continue
				}
				ok := true
				for _, q := range pts {
					if math.Hypot(p[0]-q[0], p[1]-q[1]) < 74 {
						ok = false
						break
					}
				}
				if ok {
					pts = append(pts, p)
					got++
				}
			}
		}
		b := &Board{ID: id, Name: name, RegionNames: append([]string(nil), regions...), Width: mapW, Height: mapH}
		count := make([]int, len(centres))
		used := map[string]bool{}
		for _, p := range pts {
			best, bestD := 0, math.Inf(1)
			for i, c := range centres {
				if d := math.Hypot(p[0]-c[0], p[1]-c[1]); d < bestD {
					best, bestD = i, d
				}
			}
			count[best]++
			b.Cities = append(b.Cities, City{Name: cityName(&r, used), Region: best, X: math.Round(p[0]), Y: math.Round(p[1])})
		}
		// Every region needs 7 cities so any 3 regions reach the 21 cities
		// a two-player game ends at.
		ok := true
		for _, n := range count {
			ok = ok && n >= 7
		}
		if !ok {
			continue
		}
		b.connect()
		if regionsConnected(b) {
			return b, nil
		}
	}
	return nil, errors.New("could not build that map")
}

func cityName(r *RNG, used map[string]bool) string {
	for {
		n := namePrefixes[r.Intn(len(namePrefixes))] + nameSuffixes[r.Intn(len(nameSuffixes))]
		if !used[n] {
			used[n] = true
			return n
		}
	}
}

func regionsConnected(b *Board) bool {
	adj := b.RegionAdjacency()
	seen := make([]bool, len(adj))
	stack := []int{0}
	seen[0] = true
	for len(stack) > 0 {
		r := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for q, ok := range adj[r] {
			if ok && !seen[q] {
				seen[q] = true
				stack = append(stack, q)
			}
		}
	}
	for _, s := range seen {
		if !s {
			return false
		}
	}
	return true
}

// Map codes use Crockford's base32 (no I, L, O or U to misread).
const codeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// MapCode turns a random-map seed into a shareable code like "K3F9-2QXA".
func MapCode(seed uint64) string {
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = codeAlphabet[seed&31]
		seed >>= 5
	}
	return string(b[:4]) + "-" + string(b[4:])
}

// ParseMapCode reads a map code, forgiving case, spaces, dashes and the
// look-alike letters I, L and O.
func ParseMapCode(code string) (uint64, error) {
	var seed uint64
	n := 0
	for _, ch := range strings.ToUpper(code) {
		switch ch {
		case '-', ' ':
			continue
		case 'I', 'L':
			ch = '1'
		case 'O':
			ch = '0'
		}
		i := strings.IndexRune(codeAlphabet, ch)
		if i < 0 || n >= 8 {
			return 0, errors.New("map codes are 8 letters and digits, like K3F9-2QXA")
		}
		seed = seed<<5 | uint64(i)
		n++
	}
	if n != 8 {
		return 0, errors.New("map codes are 8 letters and digits, like K3F9-2QXA")
	}
	return seed, nil
}
