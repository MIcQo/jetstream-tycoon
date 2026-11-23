---
name: add-sim-system
description: Add a new simulation System (a new game mechanic that runs every tick or day) to sim/. Use when a feature needs its own per-tick or per-day logic.
---

# Add a simulation system

1. **Check that you need it.** If the mechanic is just a bonus or penalty, use Modifiers or Actions (ADR-0005). If it's content, use YAML (ADR-0004).
2. Create `sim/systems/<name>/<name>.go` implementing:
   ```go
   type System interface { Name() string; OnTick(c *TickCtx); OnDay(c *TickCtx) }
   ```
3. **Randomness**: use only `c.RNG("<name>")`, the per-system stream `hash(seed, "<name>", tick)`. For per-airline randomness, use `c.RNG("<name>", airlineID)`.
4. **Iteration**: loop over `c.State.SortedAirlineIDs()` and the sorted keys of every map. **Never `range` over a map.**
5. **Changes**:
   - Mutate state only through the helpers in `sim/state` or by emitting commands.
   - Read stats through `effects.Apply`.
   - Money is int64 cents.
   - Notify with `c.Notify(key, params)` and report analytics with `c.Track(event)`.
6. **Registration**: add the system to the ordered list in `sim/systems/registry.go`. Its position matters, so document why it runs after its dependencies.
7. **State**: if new fields are needed, bump `schemaVersion` and add `sim/state/migrate/vN_to_vN+1.go` with a fixture test.
8. **Config**: put tunable numbers in `content/balance.yaml`, not in constants.
9. **Tests**:
   - Table-driven unit tests.
   - Extend one golden script in `sim/testdata/scripts/`.
   - Run `make sim-test` and `make determinism`.
10. **Docs**: update the system order in `docs/TECH_DESIGN.md` §3 and diagram 2 in `docs/DIAGRAMS.md`.
