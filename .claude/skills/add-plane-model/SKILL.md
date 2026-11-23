---
name: add-plane-model
description: Add or change an aircraft model (seats, range, speed, price, levels) in content.
---

# Add a plane model

1. Edit `content/planes.yaml`:
```yaml
- id: narrow_m            # stable
  tier: 3
  nameKey: planes.narrow_m
  seatUnits: 180          # eco + 2*biz <= seatUnits
  rangeKm: 5500
  speedKmh: 830
  price: 9000000000       # cents
  upkeepDay: 450000
  burnPerKm: 3.1
  maxAgeDays: 9125
  minAirportLevel: 3
  levels: [{cost: 150000000, seatUnits: +6, efficiency: 0.04, reliability: 0.01}]
```
2. Sanity checks:
   - Cost per seat-km should sit between the neighbouring tiers.
   - Range must allow some real airport pairs for its tier.
   - `minAirportLevel` must exist in `airport_levels.yaml`.
3. Add the i18n name. Use fictional model names only, never real manufacturer brands.
4. Run `make content-check` and `make balance`. Check that AI airlines don't ignore the model, or use only that one.
