---
name: add-migration
description: Create a Postgres or ClickHouse schema migration with golang-migrate.
---

# Add a migration

1. Create the files:
   ```bash
   migrate create -ext sql -dir migrations/postgres -seq <name>
   ```
   Use `migrations/clickhouse` for ClickHouse.
2. Write both `up` and `down`. Prefer **additive** changes. For renames or removals, use expand → migrate → contract across releases.
3. Postgres:
   - `NOT NULL` + defaults; foreign keys with explicit `ON DELETE`.
   - Index foreign keys used in queries.
   - Use `CREATE INDEX CONCURRENTLY` in its own migration for large tables.
4. ClickHouse:
   - LowCardinality for enums; codecs; no Nullable.
   - `ORDER BY` from low to high cardinality; TTL.
   - For a new rollup: the target table first, then the MV. Backfill with `INSERT ... SELECT` in a separate migration if needed.
   - Load the `clickhouse-best-practices` skill to review the schema.
5. Run `make migrate-up`, then the down migration, then up again locally.
6. Update the schema diagrams in `docs/DIAGRAMS.md` (§6 or §7).
