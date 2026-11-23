---
name: add-mission-template
description: Add a mission/contract template (goal, deadline, reward, penalty) for the mission board.
---

# Add a mission template

1. Create `content/missions/<id>.yaml`:
```yaml
id: connect_pair
weight: 4
requires: {expr: "airline.airports >= 2"}
params:                         # filled when the mission is generated
  from: {pick: airport_in_network}
  to:   {pick: airport_near_network, maxKm: 3000}
goal: {expr: "hasRoute(params.from, params.to)"}
deadlineDays: 30
reward:  [{action: cash, formula: "max(2000000_00, distanceKm(params.from, params.to) * 50000)"}]
penalty: [{action: reputation, value: -2}]
textKey: missions.connect_pair
```
2. Pickers must choose from **real airports near the player's network**, and must be deterministic (sorted candidates + RNG stream).
3. Add ICU i18n text with params (`{from}`, `{to}`, `{days}`).
4. Rival missions can reference AI airlines (`pick: rival_on_route`).
5. Run `make content-check`, and add a sim test generating the mission with a fixed seed.
