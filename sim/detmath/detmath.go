// Package detmath provides cross-architecture deterministic math primitives (ADR-0021).
package detmath

import "math"

// RoundCents converts a float64 amount in cents to int64 cents using
// round-half-away-from-zero (ADR-0021).
func RoundCents(x float64) int64 {
	if math.IsNaN(x) {
		return 0
	}
	if x < 0 {
		return -int64(math.Floor(float64(-x + 0.5)))
	}
	return int64(math.Floor(float64(x + 0.5)))
}

// Sqrt wraps IEEE-754 correctly rounded square root.
func Sqrt(x float64) float64 {
	return math.Sqrt(x)
}
