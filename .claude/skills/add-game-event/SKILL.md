---
name: add-game-event
description: Add a random event, incident, news item or investment offer (accept/decline with random outcomes) as content YAML.
---

# Add a game event or investment

1. Create `content/events/<id>.yaml` using the shared schema (ADR-0004):
```yaml
id: <snake_case_id>          # never rename once shipped
kind: offer                  # offer | incident | news
weight: 5                    # relative chance
cooldownDays: 30
expiresDays: 3               # offers only
conditions: [{expr: "airline.routes >= 1"}]
choices:
  - id: accept
    cost: {formula: "airline.revenue30d * 0.1", min: 200000}   # cents
    revealAfterDays: 5
    outcomes:
      - {chance: 0.6, messageKey: events.<id>.success, effects: [{scope: global, stat: demand, op: mul, value: 1.2, days: 30}]}
      - {chance: 0.4, messageKey: events.<id>.fail, effects: []}
  - id: decline
```
2. Chances **sum to 1**. Use **formulas** for costs so the event scales with the airline's size. Use existing `stat` names (see `sim/effects/stats.go`).
3. Add i18n keys to `content/i18n/en.json`: `events.<id>.title`, `.body`, choice labels, and outcome messages (ICU).
4. AI handling: AI airlines choose by expected value × risk tolerance automatically. Make sure the effects are computable (no free-text outcomes).
5. Run `make content-check`. For high-impact events, run `make balance` (skill `run-balance`).
6. If the event needs a new condition or effect *kind*, stop and ask `sim-engineer`.
