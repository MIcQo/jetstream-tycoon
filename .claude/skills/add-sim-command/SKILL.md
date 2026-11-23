---
name: add-sim-command
description: Add a new player/AI command (an action that changes game state, e.g. buy, upgrade, set price) end to end: sim validation and apply, WASM bridge, UI, AI generator, analytics.
---

# Add a simulation command

1. Define it in `sim/command/<name>.go`:
   ```go
   type BuyX struct { Airline AirlineID; ... }   // JSON/MessagePack tags, stable names
   func (c BuyX) Validate(s *State, ct *Content) error
   func (c BuyX) Apply(s *State, ctx *ApplyCtx)
   ```
   - `Validate`: ownership, cash in cents, capacity, airport level, range, and any status restrictions (e.g. LIQUIDATION).
   - Return typed errors with **i18n keys** (`errors.cmd.insufficient_cash`).
   - `Apply` must be deterministic and may only touch state.
2. Register it in `sim/command/registry.go` (type name → decoder). **Never rename a shipped type name.**
3. Analytics: emit a `Track` event in `Apply` (skill `add-analytics-event`).
4. **AI awareness**: if AI airlines should use it, add or extend a candidate generator in `sim/ai/` (ask `game-ai-engineer`).
5. UI: add a typed helper in `web/src/worker/commands.ts` and the panel or control. Show validation errors through i18n.
6. Tests:
   - Validate rejects each invalid case.
   - Apply changes the expected state.
   - Extend a golden script.
   - Run `make sim-test` and `make determinism`.
