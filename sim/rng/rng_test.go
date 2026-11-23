package rng_test

import (
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/rng"
)

func TestStreamIsDeterministic(t *testing.T) {
	a, b := rng.For(42, "ledger", 7), rng.For(42, "ledger", 7)
	for i := 0; i < 100; i++ {
		if x, y := a.Uint64(), b.Uint64(); x != y {
			t.Fatalf("draw %d differs: %d vs %d", i, x, y)
		}
	}
}

// TestDeterministicGoldenValues pins the generator output so any change to the
// algorithm or seeding is caught on every architecture (ADR-0006).
func TestDeterministicRNGGoldenValues(t *testing.T) {
	r := rng.For(42, "ledger", 1)
	got := []uint64{r.Uint64(), r.Uint64(), r.Uint64()}
	want := goldenUint64
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("golden draw %d = %d, want %d (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestStreamsAreIndependent(t *testing.T) {
	base := rng.For(1, "fuel", 10).Uint64()
	others := map[string]uint64{
		"seed":     rng.For(2, "fuel", 10).Uint64(),
		"name":     rng.For(1, "demand", 10).Uint64(),
		"tick":     rng.For(1, "fuel", 11).Uint64(),
		"extra":    rng.For(1, "fuel", 10, "A1").Uint64(),
		"extra2":   rng.For(1, "fuel", 10, "A2").Uint64(),
		"negative": rng.For(1, "fuel", -10).Uint64(),
	}
	for k, v := range others {
		if v == base {
			t.Errorf("changing %s did not change the stream", k)
		}
	}
	if rng.For(1, "x", 0, "ab", "c").Uint64() == rng.For(1, "x", 0, "a", "bc").Uint64() {
		t.Error(`extra keys ("ab","c") and ("a","bc") collide`)
	}
	if rng.For(1, "x", 0, "a").Uint64() == rng.For(1, "x", 0, "a", "").Uint64() {
		t.Error(`extra keys ("a") and ("a","") collide`)
	}
}

func TestFloat64Range(t *testing.T) {
	r := rng.For(7, "test", 0)
	sum := 0.0
	const n = 20000
	for i := 0; i < n; i++ {
		f := r.Float64()
		if f < 0 || f >= 1 {
			t.Fatalf("Float64 = %v outside [0,1)", f)
		}
		sum += f
	}
	if mean := sum / n; mean < 0.48 || mean > 0.52 {
		t.Errorf("Float64 mean = %v, want ≈ 0.5", mean)
	}
}

func TestIntn(t *testing.T) {
	r := rng.For(9, "test", 0)
	counts := make([]int, 6)
	for i := 0; i < 60000; i++ {
		v := r.Intn(6)
		if v < 0 || v >= 6 {
			t.Fatalf("Intn(6) = %d", v)
		}
		counts[v]++
	}
	for i, c := range counts {
		if c < 9000 || c > 11000 {
			t.Errorf("Intn(6) bucket %d = %d, want ≈ 10000", i, c)
		}
	}
	if v := r.Intn(1); v != 0 {
		t.Errorf("Intn(1) = %d", v)
	}
	// A bound just above 2^62 rejects about a quarter of draws, exercising the loop.
	big := 1<<62 + 1
	for i := 0; i < 100; i++ {
		if v := r.Intn(big); v < 0 || v >= big {
			t.Fatalf("Intn(2^62+1) = %d", v)
		}
	}
}

func TestIntnPanicsOnNonPositive(t *testing.T) {
	for _, n := range []int{0, -3} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Intn(%d) did not panic", n)
				}
			}()
			rng.For(1, "x", 0).Intn(n)
		}()
	}
}
