package engine

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func checkBoard(t *testing.T, b *Board) {
	t.Helper()
	if len(b.RegionNames) != 6 {
		t.Fatalf("%s: %d regions", b.Name, len(b.RegionNames))
	}
	count := make([]int, 6)
	names := map[string]bool{}
	for i, c := range b.Cities {
		count[c.Region]++
		if names[c.Name] {
			t.Errorf("%s: duplicate city name %s", b.Name, c.Name)
		}
		names[c.Name] = true
		if c.X < 0 || c.X > b.Width || c.Y < 0 || c.Y > b.Height {
			t.Errorf("%s: city %d off the board", b.Name, i)
		}
		for j := 0; j < i; j++ {
			if math.Hypot(c.X-b.Cities[j].X, c.Y-b.Cities[j].Y) < 60 {
				t.Errorf("%s: cities %d and %d overlap", b.Name, i, j)
			}
		}
	}
	for r, n := range count {
		if n < 7 {
			t.Errorf("%s: region %d has only %d cities", b.Name, r, n)
		}
	}
	if !regionsConnected(b) {
		t.Errorf("%s: regions not connected", b.Name)
	}
	all := make([]bool, len(b.Cities))
	for i := range all {
		all[i] = true
	}
	for c, d := range b.Distances([]int{0}, all) {
		if d == math.MaxInt {
			t.Fatalf("%s: city %d unreachable", b.Name, c)
		}
	}
}

func TestBuiltinMaps(t *testing.T) {
	for _, m := range Maps() {
		spec := MapSpec{ID: m.ID, Seed: 12345}
		b, err := BuildBoard(spec)
		if err != nil {
			t.Fatalf("%s: %v", m.ID, err)
		}
		checkBoard(t, b)
		again, _ := BuildBoard(spec)
		if !reflect.DeepEqual(b.Cities, again.Cities) || !reflect.DeepEqual(b.Edges, again.Edges) {
			t.Errorf("%s is not deterministic", m.ID)
		}
	}
	if b, _ := BuildBoard(MapSpec{ID: "classic"}); len(b.Cities) != 42 || b.Cities[0].Name != "Aldmoor" {
		t.Error("classic map changed")
	}
}

func TestRandomMaps(t *testing.T) {
	shapes := map[int]bool{}
	for seed := uint64(0); seed < 300; seed++ {
		b, err := BuildBoard(MapSpec{ID: "random", Seed: seed * 7919})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		checkBoard(t, b)
		shapes[len(b.Cities)] = true
	}
	if len(shapes) < 3 {
		t.Error("random maps all look alike")
	}
	if _, err := BuildBoard(MapSpec{ID: "random", Seed: MaxMapSeed}); err == nil {
		t.Error("oversized seed accepted")
	}
	if _, err := BuildBoard(MapSpec{ID: "nope"}); err == nil {
		t.Error("unknown map accepted")
	}
}

func TestMapCodes(t *testing.T) {
	for _, seed := range []uint64{0, 1, 31, 32, 123456789, MaxMapSeed - 1} {
		code := MapCode(seed)
		got, err := ParseMapCode(code)
		if err != nil || got != seed {
			t.Fatalf("%d -> %s -> %d, %v", seed, code, got, err)
		}
	}
	a, _ := ParseMapCode("k3f9 2qxa")
	b, _ := ParseMapCode("K3F9-2QXA")
	if a != b {
		t.Error("codes should ignore case and separators")
	}
	if x, _ := ParseMapCode("O0IL-0000"); x != 0b00000_00000_00001_00001<<20 {
		t.Errorf("look-alike letters: %d", x)
	}
	for _, bad := range []string{"", "K3F9", "K3F9-2QXA-1", "K3F9-2QX!", "UUUU-UUUU"} {
		if _, err := ParseMapCode(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestChosenRegions(t *testing.T) {
	cfg := Config{Map: MapSpec{ID: "classic"}, Players: []PlayerConfig{{}, {}, {}}}
	b := classicBoard()
	adj := b.RegionAdjacency()
	// Find three connected regions and three that are not.
	var good, bad []int
	for x := 0; x < 6 && good == nil; x++ {
		for y := 0; y < 6 && good == nil; y++ {
			for z := 0; z < 6 && good == nil; z++ {
				if x < y && y < z && adj[x][y] && (adj[y][z] || adj[x][z]) {
					good = []int{z, x, y}
				}
			}
		}
	}
	for x := 0; x < 6 && bad == nil; x++ {
		for y := x + 1; y < 6 && bad == nil; y++ {
			for z := y + 1; z < 6 && bad == nil; z++ {
				if !adj[x][y] && !adj[x][z] && !adj[y][z] {
					bad = []int{x, y, z}
				}
			}
		}
	}
	cfg.Regions = good
	g, err := NewGame(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if g.Regions[0] > g.Regions[1] || len(g.Regions) != 3 {
		t.Fatalf("regions %v", g.Regions)
	}
	for _, rs := range [][]int{{0, 1}, {0, 0, 1}, {0, 1, 9}, bad} {
		if rs == nil {
			continue
		}
		cfg.Regions = rs
		if _, err := NewGame(cfg); err == nil {
			t.Errorf("regions %v accepted", rs)
		}
	}
}

func TestRandomMapGamePlaysAndSaves(t *testing.T) {
	cfg := Config{Seed: 3, Map: MapSpec{ID: "random", Seed: 987654}, Players: []PlayerConfig{{}, {}, {}, {}}}
	g, err := NewGame(cfg)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(g)
	g2, err := Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g.Board().Cities, g2.Board().Cities) {
		t.Fatal("reloaded game has a different map")
	}
	// A save cannot switch to a bogus map.
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	raw["map"] = map[string]any{"id": "random", "seed": float64(MaxMapSeed)}
	bad, _ := json.Marshal(raw)
	if _, err := Load(bad); err == nil {
		t.Error("save with bad map accepted")
	}
}

func TestRegionsMustHoldEnoughCities(t *testing.T) {
	b := classicBoard()
	// Drop most of region 0's cities from a copy: 3 regions now hold < 21.
	small := *b
	small.Cities = nil
	for _, c := range b.Cities {
		if c.Region != 0 || len(small.Cities) < 2 {
			small.Cities = append(small.Cities, c)
		}
	}
	small.connect()
	adj := small.RegionAdjacency()
	for x := 1; x < 6; x++ {
		for y := x + 1; y < 6; y++ {
			if adj[0][x] && adj[x][y] {
				if err := CheckRegions(&small, []int{0, x, y}, 2); err == nil {
					t.Fatal("too few cities accepted for 2 players")
				}
				return
			}
		}
	}
	t.Skip("no connected triple found")
}
