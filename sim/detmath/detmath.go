// Package detmath provides cross-architecture deterministic math primitives (ADR-0021).
//
// Only IEEE-754 correctly rounded operations (+ − × ÷, Sqrt) and exact operations
// (Floor, Abs, bit manipulation) are used. Every product that is added to another
// value is wrapped in an explicit float64() conversion, which the Go specification
// guarantees prevents fused multiply-add on architectures such as arm64. Results are
// therefore bit-identical on amd64, arm64 and wasm.
package detmath

import "math"

const (
	ln2Hi = 6.93147180369123816490e-01
	ln2Lo = 1.90821492927058770002e-10
	log2e = 1.44269504088896338700e+00

	expOverflow  = 7.09782712893383973096e+02
	expUnderflow = -7.45133219101941108420e+02

	sqrt2Half = 7.07106781186547524401e-01

	mask  = 0x7FF
	shift = 64 - 11 - 1
	bias  = 1023
)

// RoundCents converts a float64 amount in cents to int64 cents using
// round-half-away-from-zero (ADR-0021). NaN converts to 0.
func RoundCents(x float64) int64 {
	if x != x { // NaN
		return 0
	}
	r := int64(math.Floor(float64(math.Abs(x) + 0.5)))
	if x < 0 {
		return -r
	}
	return r
}

// Sqrt wraps the IEEE-754 correctly rounded square root.
func Sqrt(x float64) float64 {
	return math.Sqrt(x)
}

// Clamp limits x to [lo, hi]. NaN is returned as lo.
func Clamp(x, lo, hi float64) float64 {
	if x != x { // NaN
		return lo
	}
	return math.Max(lo, math.Min(x, hi))
}

// frexp breaks a finite, non-zero f into a normalized fraction in [0.5, 1) and an
// integral power of two.
func frexp(f float64) (frac float64, exp int) {
	if (math.Float64bits(f)>>shift)&mask == 0 { // subnormal: scale into the normal range
		f *= 1 << 52
		exp = -52
	}
	x := math.Float64bits(f)
	exp += int((x>>shift)&mask) - bias + 1
	x &^= mask << shift
	x |= (-1 + bias) << shift
	return math.Float64frombits(x), exp
}

// pow2 returns 2^n for n in [-1022, 1023].
func pow2(n int) float64 {
	return math.Float64frombits(uint64(n+bias) << shift)
}

// ldexp returns frac × 2^exp for |exp| ≤ 2044, as two multiplications by powers of
// two. Each is exact for normal results; only the last can round (subnormal result),
// and IEEE defines that rounding, so the result is deterministic.
func ldexp(frac float64, exp int) float64 {
	a := exp / 2
	return float64(frac*pow2(a)) * pow2(exp-a)
}

// Exp returns e^x. It is a port of the FreeBSD/Go portable algorithm with all
// multiply-add sequences made explicit.
func Exp(x float64) float64 {
	switch {
	case x != x: // NaN
		return x
	case x > expOverflow: // includes +Inf
		return math.Inf(1)
	case x < expUnderflow: // includes -Inf
		return 0
	}
	k := int(math.Floor(float64(log2e*x) + 0.5))
	hi := x - float64(float64(k)*ln2Hi)
	lo := float64(float64(k) * ln2Lo)
	return expmulti(hi, lo, k)
}

func expmulti(hi, lo float64, k int) float64 {
	const (
		p1 = 1.66666666666666657415e-01
		p2 = -2.77777777770155933842e-03
		p3 = 6.61375632143793436117e-05
		p4 = -1.65339022054652515390e-06
		p5 = 4.13813679705723846039e-08
	)
	r := hi - lo
	t := float64(r * r)
	poly := p4 + float64(t*p5)
	poly = p3 + float64(t*poly)
	poly = p2 + float64(t*poly)
	poly = p1 + float64(t*poly)
	c := r - float64(t*poly)
	y := 1 - ((lo - float64(float64(r*c)/(2-c))) - hi)
	return ldexp(y, k)
}

// Log returns the natural logarithm of x. Log(0) = -Inf, Log(x<0) = NaN.
func Log(x float64) float64 {
	const (
		l1 = 6.666666666666735130e-01
		l2 = 3.999999999940941908e-01
		l3 = 2.857142874366239149e-01
		l4 = 2.222219843214978396e-01
		l5 = 1.818357216161805012e-01
		l6 = 1.531383769920937332e-01
		l7 = 1.479819860511658591e-01
	)
	switch {
	case x != x || x > math.MaxFloat64: // NaN or +Inf
		return x
	case x < 0:
		return math.NaN()
	case x == 0:
		return math.Inf(-1)
	}
	f1, ki := frexp(x)
	if f1 < sqrt2Half {
		f1 *= 2
		ki--
	}
	f := f1 - 1
	k := float64(ki)

	s := f / (2 + f)
	s2 := float64(s * s)
	s4 := float64(s2 * s2)
	odd := l5 + float64(s4*l7)
	odd = l3 + float64(s4*odd)
	odd = l1 + float64(s4*odd)
	t1 := float64(s2 * odd)
	even := l4 + float64(s4*l6)
	even = l2 + float64(s4*even)
	t2 := float64(s4 * even)
	r := t1 + t2
	hfsq := float64(float64(0.5*f) * f)
	inner := float64(s*(hfsq+r)) + float64(k*ln2Lo)
	return float64(k*ln2Hi) - ((hfsq - inner) - f)
}

// PowInt returns x^n for an integer exponent using exponentiation by squaring.
// The multiplication order is fixed, so the result is deterministic.
func PowInt(x float64, n int) float64 {
	if n < 0 {
		return 1 / PowInt(x, -n)
	}
	result := 1.0
	base := x
	for n > 0 {
		if n&1 == 1 {
			result *= base
		}
		base *= base
		n >>= 1
	}
	return result
}

// Pow returns x^y. Integer exponents with |y| ≤ 2^20 use PowInt; otherwise
// x^y = Exp(y·Log(x)). The special cases follow from Log and Exp: x = 0 gives 0 for
// y > 0 and +Inf for y < 0; x = +Inf the reverse; negative x with a non-integer y,
// or any NaN, gives NaN.
func Pow(x, y float64) float64 {
	if yi := math.Floor(y); yi == y && math.Abs(y) <= 1<<20 {
		return PowInt(x, int(yi))
	}
	return Exp(float64(y * Log(x)))
}

// Normal returns an approximately standard normal value from 12 uniform samples in
// [0, 1) (Irwin–Hall). It needs no transcendental functions, and the summation
// order is fixed.
func Normal(uniform func() float64) float64 {
	sum := 0.0
	for i := 0; i < 12; i++ {
		sum += uniform()
	}
	return sum - 6
}

// strict makes Finite panic; it is false only in release builds.
var strict = !releaseBuild

// NonFiniteHook is called in release builds when Finite meets NaN or ±Inf, so the
// host can report it (e.g. to Sentry). It must not affect simulation state.
var NonFiniteHook func(context string, x float64)

// Finite guards a formula result (ADR-0021). In debug and test builds a NaN or
// ±Inf panics. In release builds (build tag "release") it returns 0 and calls
// NonFiniteHook.
func Finite(x float64, context string) float64 {
	if !math.IsNaN(x) && !math.IsInf(x, 0) {
		return x
	}
	if strict {
		panic("detmath: non-finite value in " + context)
	}
	if NonFiniteHook != nil {
		NonFiniteHook(context, x)
	}
	return 0
}
