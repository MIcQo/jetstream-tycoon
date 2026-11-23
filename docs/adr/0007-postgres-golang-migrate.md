# ADR-0007: Postgres for accounts, save metadata and config; golang-migrate for schemas

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
We need relational, transactional storage for users, identities, devices, tokens, save metadata (optimistic locking) and feature flags. The preferred stack includes Postgres and Go Migrate.

## Decision
- Postgres 16 with `pgx`.
- Schemas are versioned SQL in `migrations/postgres/`, applied with golang-migrate. ClickHouse uses the same tool in `migrations/clickhouse/`.
- Postgres does **not** store save files (ADR-0008) or analytics (ADR-0011).

## Consequences
- A familiar, reliable store with strong consistency for the sync revision check.
- Rows stay small, so vacuum and backups stay cheap.

## Alternatives considered
MySQL/MariaDB (also viable; Postgres chosen for JSONB and its ecosystem); a document DB (weaker transactional guarantees).
