---
name: determinism-check
description: Verify the simulation is deterministic across native and WASM builds and diagnose divergence. Use after any change in sim/.
---

# Determinism check

1. Run `make determinism`. It runs every script in `sim/testdata/scripts/` natively (amd64; arm64 in CI) and as WASM under Node, then compares state hashes per game day.
2. If the hashes differ, find the **first diverging day**:
   ```bash
   go run ./tools/simdiff --script <s> --dump-day N
   ```
   Diff the JSON dumps of the two builds.
3. Usual causes:
   - Ranging over a map (`grep -rn "range " sim/ | grep -i map`).
   - Unsorted slices built from maps.
   - Shared RNG streams.
   - `time.Now`.
   - Goroutines.
   - Float accumulation order or fused multiply-add. Add explicit `float64()` conversions and round to cents once.
   - Remote config read outside `ConfigSnapshot`.
4. Fix the cause, re-run, and add a regression script reproducing the case.
5. Update golden hashes **only** for intentional behaviour changes, with `make golden-update`. Explain why in the PR.
