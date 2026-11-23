# ADR-0011: ClickHouse for gameplay and product analytics

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
Balancing and product decisions need event-level analytics: retention, funnels, economy curves and event outcomes, at potentially high volume and with cheap aggregation.

## Decision
- A single `events` table (ReplacingMergeTree on `event_id`) partitioned by month and ordered by `(event_name, event_date, user_id, event_id)`.
- Frequently filtered fields are typed columns, with codecs on every column.
- Raw events are kept **90 days**. Materialized-view rollups are kept 2 years, and **Grafana reads only rollups**.
- No personal data in ClickHouse; `user_id` is pseudonymous.
- `client_perf` events are sampled.

## Consequences
- Fast and cheap analytics.
- Duplicates are removed eventually (at merge time), so exact counts use `uniqExact(event_id)` or rollups.
- Schema changes need care (golang-migrate, additive).

## Alternatives considered
Postgres for analytics (doesn't scale for event data); third-party product analytics (cost, data ownership, weaker game-economy queries).
