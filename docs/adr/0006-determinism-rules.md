# ADR-0006: Strict determinism in the simulation

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
Determinism enables golden tests, the balance bot, crash reproduction from a seed plus a command log, and future server-side replay verification (leaderboards). Go has several sources of non-determinism, most notably randomised map iteration order.

## Decision
In `sim/`:
1. No `range` over maps (a custom vet analyzer enforces this); iterate sorted keys.
2. No `time.Now`, goroutines or global `math/rand`.
3. A separate seeded PCG stream per system: `hash(seed, systemID, tick)`.
4. **Money is int64 cents.** Floats are used only for factors, rounded once at a defined point, with explicit conversions to prevent fused multiply-add.
5. Remote config that affects the simulation is snapshotted into the save.
6. CI runs the same seed and command script natively (amd64, arm64) and as WASM in Node, and the final state hashes must match.

## Consequences
- Bugs are reproducible, and saves and replays stay stable as systems are added.
- Developers must follow the rules, and the linter plus the CI cross-check enforce them.

## Alternatives considered
Best-effort determinism (silently breaks replays); fixed-point math everywhere (safer but much less readable).
