// Package rng provides deterministic, independent random streams for the
// simulation (ADR-0006). A stream is derived from (seed, stream name, tick, extra
// keys), so adding a new system or airline never changes the numbers drawn by
// existing ones. The generator is xoshiro256** seeded through SplitMix64.
package rng

// Stream is a deterministic pseudo-random number generator.
type Stream struct {
	s [4]uint64
}

// For returns the stream for a named system (and optional extra keys such as an
// airline ID) at a given tick.
func For(seed uint64, name string, tick int64, extra ...string) *Stream {
	h := fnv1a(name)
	for _, e := range extra {
		h = fnv1aContinue(h^0xff, e) // 0xff separates keys so ("ab","c") != ("a","bc")
	}
	x := seed ^ h ^ (uint64(tick) * 0x9E3779B97F4A7C15)
	var st Stream
	for i := range st.s {
		x, st.s[i] = splitmix64(x)
	}
	return &st
}

const (
	fnvOffset = 14695981039346656037
	fnvPrime  = 1099511628211
)

func fnv1a(s string) uint64 { return fnv1aContinue(fnvOffset, s) }

func fnv1aContinue(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime
	}
	return h
}

// splitmix64 advances the state and returns (next state, output).
func splitmix64(x uint64) (uint64, uint64) {
	x += 0x9E3779B97F4A7C15
	z := x
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return x, z ^ (z >> 31)
}

func rotl(x uint64, k uint) uint64 { return (x << k) | (x >> (64 - k)) }

// Uint64 returns the next 64 random bits (xoshiro256**).
func (r *Stream) Uint64() uint64 {
	s := &r.s
	result := rotl(s[1]*5, 7) * 9
	t := s[1] << 17
	s[2] ^= s[0]
	s[3] ^= s[1]
	s[1] ^= s[2]
	s[0] ^= s[3]
	s[2] ^= t
	s[3] = rotl(s[3], 45)
	return result
}

// Float64 returns a uniform value in [0, 1) with 53 random bits.
func (r *Stream) Float64() float64 {
	return float64(r.Uint64()>>11) * (1.0 / (1 << 53))
}

// Intn returns a uniform integer in [0, n). It panics if n <= 0. Rejection
// sampling removes modulo bias.
func (r *Stream) Intn(n int) int {
	if n <= 0 {
		panic("rng: Intn with n <= 0")
	}
	bound := uint64(n)
	threshold := -bound % bound // (2^64 - bound) mod bound
	for {
		v := r.Uint64()
		if v >= threshold {
			return int(v % bound)
		}
	}
}
