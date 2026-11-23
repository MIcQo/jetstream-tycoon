---
name: add-ai-personality
description: Add or tune an AI rival airline personality (weights for pricing, risk, expansion, aggression).
---

# Add an AI personality

1. Edit `content/ai_personalities.yaml`:
```yaml
- id: eco_long_haul
  nameKey: ai.personality.eco_long_haul
  riskTolerance: 0.5     # 0..1
  priceBias: 0.85        # x reference price
  bizShareBias: 0.6
  expansionRate: 0.7
  debtLimit: 0.6         # debt / assets
  regionFocus: 0.2       # 1 = stays near home
  reserveDays: 20
  aggression: 0.3        # undercutting, buyout attempts
  planeAgePref: new      # new | mixed | used
  routeLengthPref: long
```
2. Add airline name parts to `content/airline_names.yaml` if needed, and the i18n keys.
3. Run `make balance` (skill `run-balance`). The guardrails:
   - Bankruptcy rate 10–25% on Normal.
   - The personality wins no more than 35% of the time.
4. If the behaviour can't be expressed with weights, describe the needed generator change for `game-ai-engineer`.
