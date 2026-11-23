package detmath_test

import (
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/detmath"
)

func TestRoundCents(t *testing.T) {
	cases := []struct {
		in   float64
		want int64
	}{
		{0, 0},
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
