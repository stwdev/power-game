package ai

import (
	"os"
	"testing"
	"time"

	"powergame/engine"
)

func TestH2H(t *testing.T) {
	if os.Getenv("H2H") == "" {
		t.Skip("set H2H=1 to run the strength benchmark")
	}
	for _, setup := range [][]engine.Level{{2, 3}, {2, 2, 3}, {1, 2, 3}} {
		wins := map[engine.Level]int{}
		start := time.Now()
		n := 30
		for s := 0; s < n; s++ {
			lv := append([]engine.Level(nil), setup...)
			k := s % len(lv)
			lv = append(lv[k:], lv[:k]...)
			g := playOut(t, lv, uint64(5000+s))
			wins[g.Players[g.Ranking[0]].AI]++
		}
		t.Logf("%v: easy=%d med=%d hard=%d (%.2fs/game)", setup, wins[1], wins[2], wins[3], time.Since(start).Seconds()/float64(n))
	}
}
