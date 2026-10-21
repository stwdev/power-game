package engine

// RNG is a small splitmix64 generator. Its state lives in the game so
// that saved games and future network play replay identically.
type RNG struct {
	State uint64 `json:"state"`
}

// Next returns the next 64 random bits.
func (r *RNG) Next() uint64 {
	r.State += 0x9e3779b97f4a7c15
	z := r.State
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// Intn returns a value in [0, n). n must be > 0.
func (r *RNG) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	// Rejection sampling avoids modulo bias.
	limit := ^uint64(0) - (^uint64(0) % uint64(n))
	for {
		v := r.Next()
		if v < limit {
			return int(v % uint64(n))
		}
	}
}

// Shuffle permutes s in place.
func Shuffle[T any](r *RNG, s []T) {
	for i := len(s) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}
