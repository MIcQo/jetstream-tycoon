package detmath_test

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/detmath"
)

var update = flag.Bool("update", false, "rewrite testdata/vectors.txt from the current build")

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

func TestClamp(t *testing.T) {
	cases := []struct{ x, lo, hi, want float64 }{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{11, 0, 10, 10},
		{0, 0, 10, 0},
		{10, 0, 10, 10},
		{math.NaN(), 1, 2, 1},
	}
	for _, c := range cases {
		if got := detmath.Clamp(c.x, c.lo, c.hi); got != c.want {
			t.Errorf("Clamp(%v,%v,%v) = %v, want %v", c.x, c.lo, c.hi, got, c.want)
		}
	}
}

// ulpDiff returns the distance in units of last place between two finite floats
// of the same sign.
func ulpDiff(a, b float64) uint64 {
	ua, ub := math.Float64bits(a), math.Float64bits(b)
	if ua > ub {
		return ua - ub
	}
	return ub - ua
}

func TestExpAccuracy(t *testing.T) {
	for x := -700.0; x <= 700; x += 0.731 {
		got, want := detmath.Exp(x), math.Exp(x)
		if d := ulpDiff(got, want); d > 1 {
			t.Fatalf("Exp(%v) = %v, want %v (%d ulp)", x, got, want, d)
		}
	}
	for _, x := range []float64{1e-9, -1e-9, 0.5, -0.5, 1, -1, 2.5, 10, -10, -745} {
		got, want := detmath.Exp(x), math.Exp(x)
		if d := ulpDiff(got, want); d > 1 {
			t.Errorf("Exp(%v) = %v, want %v (%d ulp)", x, got, want, d)
		}
	}
}

func TestExpSpecialCases(t *testing.T) {
	if got := detmath.Exp(0); got != 1 {
		t.Errorf("Exp(0) = %v", got)
	}
	if got := detmath.Exp(math.Inf(1)); !math.IsInf(got, 1) {
		t.Errorf("Exp(+Inf) = %v", got)
	}
	if got := detmath.Exp(math.Inf(-1)); got != 0 {
		t.Errorf("Exp(-Inf) = %v", got)
	}
	if got := detmath.Exp(math.NaN()); !math.IsNaN(got) {
		t.Errorf("Exp(NaN) = %v", got)
	}
	if got := detmath.Exp(710); !math.IsInf(got, 1) {
		t.Errorf("Exp(710) = %v, want +Inf", got)
	}
	// Go's amd64 assembly math.Exp returns +Inf here, although e^709.7 ≈ 1.6549840276802644e308
	// is representable. detmath must return the finite value on every architecture.
	if got := detmath.Exp(709.7); got != 1.6549840276802644e+308 {
		t.Errorf("Exp(709.7) = %v, want 1.6549840276802644e308", got)
	}
	if got := detmath.Exp(-746); got != 0 {
		t.Errorf("Exp(-746) = %v, want 0", got)
	}
	tiny := 1.0 / (1 << 30)
	if got := detmath.Exp(tiny); got != 1+tiny {
		t.Errorf("Exp(2^-30) = %v, want %v", got, 1+tiny)
	}
	// Subnormal results go through ldexp's scaling loop.
	if got, want := detmath.Exp(-740), math.Exp(-740); got != want {
		t.Errorf("Exp(-740) = %v, want %v", got, want)
	}
}

func TestLogAccuracy(t *testing.T) {
	for x := 1e-300; x < 1e300; x *= 3.7 {
		got, want := detmath.Log(x), math.Log(x)
		if d := ulpDiff(got, want); d > 1 {
			t.Fatalf("Log(%v) = %v, want %v (%d ulp)", x, got, want, d)
		}
	}
	for _, x := range []float64{0.5, 0.7, 0.75, 1.5, 2, 10} {
		got, want := detmath.Log(x), math.Log(x)
		if d := ulpDiff(got, want); d > 1 {
			t.Errorf("Log(%v) = %v, want %v (%d ulp)", x, got, want, d)
		}
	}
	// Subnormal inputs: Go's amd64 assembly math.Log is wrong here (it returns about
	// -709.09 for 4e-320), so the reference is computed from a scaled normal value.
	for _, x := range []float64{4e-320, 1e-310, 5e-324} {
		got := detmath.Log(x)
		want := math.Log(x*(1<<52)) - 52*math.Ln2
		if math.Abs(got-want) > 1e-12 {
			t.Errorf("Log(%v) = %v, want %v", x, got, want)
		}
	}
}

func TestLogSpecialCases(t *testing.T) {
	if got := detmath.Log(1); got != 0 {
		t.Errorf("Log(1) = %v", got)
	}
	if got := detmath.Log(0); !math.IsInf(got, -1) {
		t.Errorf("Log(0) = %v", got)
	}
	if got := detmath.Log(-1); !math.IsNaN(got) {
		t.Errorf("Log(-1) = %v", got)
	}
	if got, want := detmath.Log(math.MaxFloat64), math.Log(math.MaxFloat64); ulpDiff(got, want) > 1 {
		t.Errorf("Log(MaxFloat64) = %v, want %v", got, want)
	}
	if got := detmath.Log(math.Inf(1)); !math.IsInf(got, 1) {
		t.Errorf("Log(+Inf) = %v", got)
	}
	if got := detmath.Log(math.NaN()); !math.IsNaN(got) {
		t.Errorf("Log(NaN) = %v", got)
	}
}

func TestPowInt(t *testing.T) {
	cases := []struct {
		x    float64
		n    int
		want float64
	}{
		{2, 0, 1},
		{2, 1, 2},
		{2, 10, 1024},
		{3, 5, 243},
		{2, -2, 0.25},
		{-2, 3, -8},
		{1.5, 2, 2.25},
	}
	for _, c := range cases {
		if got := detmath.PowInt(c.x, c.n); got != c.want {
			t.Errorf("PowInt(%v,%d) = %v, want %v", c.x, c.n, got, c.want)
		}
	}
}

func TestPow(t *testing.T) {
	// Non-integer exponents: relative error bound from Exp(y·Log(x)).
	for _, x := range []float64{0.01, 0.5, 1.7, 2, 9.5, 1000, 123456} {
		for _, y := range []float64{0.3, 0.5, 0.8, 1.5, 2.25, -0.7} {
			got, want := detmath.Pow(x, y), math.Pow(x, y)
			if rel := math.Abs(got-want) / want; rel > 1e-13 {
				t.Errorf("Pow(%v,%v) = %v, want %v (rel %g)", x, y, got, want, rel)
			}
		}
	}
	special := []struct{ x, y, want float64 }{
		{5, 0, 1},
		{1, 123.4, 1},
		{2, 3, 8},
		{4, -1, 0.25},
		{0, 0.5, 0},
		{math.Inf(1), 0.5, math.Inf(1)},
		{math.Inf(1), -0.5, 0},
	}
	for _, c := range special {
		if got := detmath.Pow(c.x, c.y); got != c.want {
			t.Errorf("Pow(%v,%v) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
	if got := detmath.Pow(0, -0.5); !math.IsInf(got, 1) {
		t.Errorf("Pow(0,-0.5) = %v, want +Inf", got)
	}
	for _, c := range [][2]float64{{-2, 0.5}, {math.NaN(), 2}, {2, math.NaN()}} {
		if got := detmath.Pow(c[0], c[1]); !math.IsNaN(got) {
			t.Errorf("Pow(%v,%v) = %v, want NaN", c[0], c[1], got)
		}
	}
	// Very large integer exponents use Exp/Log rather than PowInt.
	if got := detmath.Pow(1.0000001, 1<<21); math.Abs(got-math.Pow(1.0000001, 1<<21)) > 1e-9 {
		t.Errorf("Pow(1.0000001, 2^21) = %v", got)
	}
}

func TestNormal(t *testing.T) {
	half := func() float64 { return 0.5 }
	if got := detmath.Normal(half); got != 0 {
		t.Errorf("Normal(0.5…) = %v, want 0", got)
	}
	zero := func() float64 { return 0 }
	if got := detmath.Normal(zero); got != -6 {
		t.Errorf("Normal(0…) = %v, want -6", got)
	}
	calls := 0
	counting := func() float64 { calls++; return 0.25 }
	if got := detmath.Normal(counting); got != -3 || calls != 12 {
		t.Errorf("Normal(0.25…) = %v after %d calls, want -3 after 12", got, calls)
	}
}

func TestFinite(t *testing.T) {
	if got := detmath.Finite(1.5, "ok"); got != 1.5 {
		t.Errorf("Finite(1.5) = %v", got)
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		func() {
			defer func() {
				r := recover()
				if r == nil || !strings.Contains(fmt.Sprint(r), "price") {
					t.Errorf("Finite(%v) did not panic with context, got %v", bad, r)
				}
			}()
			detmath.Finite(bad, "price")
		}()
	}
}

// vectorInputs is the fixed input set for the cross-architecture bit-exact check.
func vectorInputs() []float64 {
	var xs []float64
	for x := -50.0; x <= 50; x += 0.37 {
		xs = append(xs, x)
	}
	xs = append(xs, 1e-300, 4e-320, 0.1, 0.2, 0.3, 1e10, 709.5, -744)
	return xs
}

func vectorLines() []string {
	var lines []string
	for _, x := range vectorInputs() {
		ax := math.Abs(x) + 1e-3
		lines = append(lines, fmt.Sprintf("%016x exp=%016x log=%016x pow15=%016x pow08=%016x",
			math.Float64bits(x),
			math.Float64bits(detmath.Exp(x)),
			math.Float64bits(detmath.Log(ax)),
			math.Float64bits(detmath.Pow(ax, 1.5)),
			math.Float64bits(detmath.Pow(ax, 0.8)),
		))
	}
	return lines
}

// TestDeterministicVectors compares results bit for bit with testdata/vectors.txt,
// which was generated on amd64. CI runs this on amd64, arm64 and wasm (ADR-0021).
func TestDeterministicVectors(t *testing.T) {
	const path = "testdata/vectors.txt"
	got := vectorLines()
	if *update {
		if err := os.WriteFile(path, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	i := 0
	for sc.Scan() {
		if i >= len(got) {
			t.Fatalf("vectors file has more lines than inputs (%d)", len(got))
		}
		if sc.Text() != got[i] {
			bits, _ := strconv.ParseUint(strings.Fields(got[i])[0], 16, 64)
			t.Errorf("vector %d (x=%v) differs:\n got  %s\n want %s", i, math.Float64frombits(bits), got[i], sc.Text())
		}
		i++
	}
	if i != len(got) {
		t.Fatalf("vectors file has %d lines, want %d", i, len(got))
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

func TestExpBoundaries(t *testing.T) {
	const overflow = 7.09782712893383973096e+02
	const underflow = -7.45133219101941108420e+02
	if got := detmath.Exp(overflow); math.IsInf(got, 0) || got < 1.79e308 {
		t.Errorf("Exp(overflow limit) = %v, want finite ≈ MaxFloat64", got)
	}
	if got := detmath.Exp(underflow); got == 0 {
		t.Errorf("Exp(underflow limit) = 0, want the smallest subnormal")
	}
	if got := detmath.Exp(math.Nextafter(overflow, math.Inf(1))); !math.IsInf(got, 1) {
		t.Errorf("Exp(just above overflow) = %v, want +Inf", got)
	}
}

func TestLogNearSqrtHalf(t *testing.T) {
	// The reduction switches at √2/2; values on both sides must stay accurate.
	for _, x := range []float64{0.7071067811865476, 0.7071067811865475, 1.4142135623730951} {
		got, want := detmath.Log(x), math.Log(x)
		if d := ulpDiff(got, want); d > 1 {
			t.Errorf("Log(%v) = %v, want %v (%d ulp)", x, got, want, d)
		}
	}
}

func TestPowIntegerExponentLimit(t *testing.T) {
	// Up to 2^20 integer exponents use exact repeated squaring.
	x := 1.0000001
	if got, want := detmath.Pow(x, 1<<20), detmath.PowInt(x, 1<<20); got != want {
		t.Errorf("Pow(x, 2^20) = %v, want PowInt result %v", got, want)
	}
	if got := detmath.PowInt(2, 0); got != 1 {
		t.Errorf("PowInt(2, 0) = %v, want 1", got)
	}
}

func TestFiniteReleaseMode(t *testing.T) {
	restore := detmath.SetStrict(false)
	defer restore()

	var gotCtx string
	var gotX float64
	detmath.NonFiniteHook = func(ctx string, x float64) { gotCtx, gotX = ctx, x }
	defer func() { detmath.NonFiniteHook = nil }()

	if got := detmath.Finite(math.Inf(1), "fuel"); got != 0 {
		t.Errorf("release Finite(+Inf) = %v, want 0", got)
	}
	if gotCtx != "fuel" || !math.IsInf(gotX, 1) {
		t.Errorf("hook got (%q, %v), want (\"fuel\", +Inf)", gotCtx, gotX)
	}
	if got := detmath.Finite(2.5, "ok"); got != 2.5 {
		t.Errorf("release Finite(2.5) = %v", got)
	}

	detmath.NonFiniteHook = nil
	if got := detmath.Finite(math.NaN(), "no hook"); got != 0 {
		t.Errorf("release Finite(NaN) without hook = %v, want 0", got)
	}
}
