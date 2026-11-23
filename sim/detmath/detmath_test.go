package detmath_test

import (
	"math"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/detmath"
)

func TestRoundCents(t *testing.T) {
	cases := []struct {
		in   float64
		want int64
	}{
		{0, 0},
		{math.NaN(), 0},
		{0.49, 0},
		{0.5, 1},
		{-0.49, 0},
		{-0.5, -1},
		{100.4, 100},
		{100.5, 101},
		{-100.4, -100},
		{-100.5, -101},
	}
	for _, tc := range cases {
		if got := detmath.RoundCents(tc.in); got != tc.want {
			t.Errorf("RoundCents(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSqrt(t *testing.T) {
	if got := detmath.Sqrt(9); got != 3 {
		t.Fatalf("Sqrt(9) = %v, want 3", got)
	}
}

func FuzzRoundCents(f *testing.F) {
	f.Add(0.0)
	f.Add(0.5)
	f.Add(-0.5)
	f.Add(12345.67)
	f.Add(-98765.43)

	f.Fuzz(func(t *testing.T, x float64) {
		if math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) > 1e15 {
			return
		}
		got := detmath.RoundCents(x)
		neg := detmath.RoundCents(-x)
		if got != -neg {
			t.Fatalf("symmetry violated for %v: RoundCents(x)=%d, RoundCents(-x)=%d", x, got, neg)
		}
	})
}
