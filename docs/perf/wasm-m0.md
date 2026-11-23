# sim.wasm measurements — M0 (issue #49)

Measured on 2026-11-23 with Go 1.27.0, `GOOS=js GOARCH=wasm -trimpath -ldflags="-s -w"`, **without** `wasm-opt` (not installed in the measuring environment; CI runs it when available).
Build under test: the M0.6 bridge (`sim/bridge`, engine, rng, ledger stub, embedded determinism scripts).

## Size

| Build | Raw | gzip -9 | Budget |
|---|---|---|---|
| `sim.wasm` (M0.6 bridge) | 4.88 MB | 1.34 MB | 10 MB raw / 2.5 MB brotli |
| `expr-lang/expr` added (measured as the delta between two minimal programs, one compiling and running an expression) | +3.65 MB | +0.80 MB | — |
| **Projected `sim.wasm` with `expr`** | **≈ 8.5 MB** | **≈ 2.1 MB** | within budget |

Brotli was not available locally; brotli `-q 11` is typically 10–20% smaller than gzip -9, so the projected brotli size is about 1.7–1.9 MB.

**Risk**: with `expr` the raw size uses about 85% of the 10 MB budget before any real game logic exists. Mitigations, in order:
1. Run `wasm-opt -Oz` in every build (typically −10–20%).
2. Keep `expr` usage to the allow-listed `detmath` functions (ADR-0021) and avoid pulling `reflect`-heavy helpers.
3. If M1–M3 growth exceeds the budget: compile content expressions to a small custom bytecode in datagen instead of shipping `expr`, or try TinyGo (re-validate determinism first).

## Startup and per-day cost (headless Chromium, median of 3)

| CPU throttling | Compile | Instantiate + Go start | `newGame` | One game day (24 ticks) |
|---|---|---|---|---|
| 1× (dev machine) | 11 ms | 18 ms | 7 ms | 0.3 ms |
| 4× | 24 ms | 77 ms | 29 ms | 1.2 ms |
| 6× (≈ low-end Android) | 34 ms | 124 ms | 64 ms | 2.1 ms |

- Budget: compile + instantiate ≤ 1.5 s on a low-end Android WebView. At 6× throttling it is about **0.16 s**, so there is a large margin.
- Budget: day-boundary work ≤ 16 ms p95. Currently **2.1 ms** at 6×, but M0 has only the stub system; re-measure from M3.
- CPU throttling is a stand-in; a real device check on a low-end Android phone is planned for M4, when the UI exists.

## CI budgets
- `make wasm-size-check` (CI job "WASM Size Budget") enforces ≤ 10 MB raw and ≤ 2.5 MB brotli (gzip -9 when brotli is unavailable).
- Startup time is not measured in CI, because shared runners are too noisy. Re-run `tools/wasmbench` at the end of each milestone and update this file.
