---
name: add-analytics-event
description: Add a gameplay or product analytics event end to end: tracking call, event catalog, ClickHouse columns or rollup, privacy check.
---

# Add an analytics event

1. **Privacy**: no personal data (emails, names, IPs, free text). IDs are pseudonymous only.
2. Name it `object_verb` in snake_case (e.g. `hub_designated`).
3. Emit it:
   - From the simulation: `c.Track(analytics.Event{Name:..., RouteID:..., RefID:..., AmountCents:..., Value:..., Props:..., Metrics:...})`.
   - From the UI: `telemetry.track(name, {...})`.
4. Use the **typed fields** (`route_id`, `ref_id`, `amount_cents`, `value`) for values you will filter or aggregate on. Everything else goes in `props`/`metrics`.
5. High-frequency data (e.g. per flight): **aggregate before sending** (e.g. `route_daily`) or sample through remote config.
6. Add it to the event catalog in `docs/TECH_DESIGN.md` §12.
7. If a dashboard needs it, add a rollup table + materialized view through a ClickHouse migration (skill `add-migration`). Dashboards read only rollups.
8. Test it: an ingestor integration test inserts it, and the rollup query returns the expected row.
