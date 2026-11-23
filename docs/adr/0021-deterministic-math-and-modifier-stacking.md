# ADR-0021: Deterministic math library, rounding and modifier stacking order

- **Status**: Accepted
- **Date**: 2026-11-23
- **Refines**: ADR-0005, ADR-0006

## Context
A review found two gaps.
1. Explicit `float64()` casts prevent fused multiply-add, but transcendental functions (`math.Pow`, `Exp`, `Log`) and `expr` float evaluation can differ by 1 ULP across amd64, arm64 and wasm, because some architectures have assembly versions. NaN and Inf propagation can also diverge.
2. ADR-0005 required a defined stacking order for modifiers but didn't specify one. Float operations don't commute exactly, so the order itself affects determinism.

## Decision
- **`sim/detmath`**: pure-Go ports of the needed functions (`Pow`, `Exp`, `Log`, ...), with every product wrapped in `float64()` and no assembly versions.
  - Only `+ − × ÷`, `Sqrt`, `Floor`, `Abs`, `Min` and `Max` may be used directly. All other `math.*` calls are banned in `sim/` by the vet analyzer.
  - Integer powers use repeated multiplication. Fixed exponents on static data are precomputed in datagen.
  - Normal random numbers use Irwin–Hall (12 uniforms).
- **Rounding**: `RoundCents` (half away from zero) is the only float → cents conversion.
- **NaN/Inf**: `Finite` guards every formula result. It panics in tests and debug builds; in release it clamps to 0 and reports to Sentry.
- **`expr`** may call only `detmath`-backed functions.
- **M0 gate**: CI compares `detmath` test vectors bit for bit across amd64, arm64 and wasm.
- **Modifier stacking**:
  1. Sort by `(scopeRank, priority, expiresTick, id)`.
  2. Only one modifier per `(source, stat, target)` is active.
  3. Evaluate `(base + Σadd) × Πmul` in the sorted order.
  4. Clamp to the per-stat `[min, max]`.
  5. The last `set` in sorted order overrides the result.

## Consequences
- Bit-identical results on all targets, so replays, golden tests and leaderboards stay stable.
- A small math library to maintain and test, plus a datagen step for precomputed factors.
- Designers can't use arbitrary math functions in content formulas, only the allow-listed set.
- Clamps bound balance exploits from stacked bonuses.

## Alternatives considered
- Fixed-point arithmetic everywhere: fully safe, but much harder to read and write for game formulas.
- Trusting `math.*` and testing only: divergence would appear later, with new Go versions or architectures.
- Additive multipliers `1 + Σ(mul−1)`: these avoid compounding but make large single bonuses weaker. Clamping covers the runaway risk instead.
