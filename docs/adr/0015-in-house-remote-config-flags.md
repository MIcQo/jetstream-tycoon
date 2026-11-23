# ADR-0015: In-house remote config and feature flags

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
We need remote tuning, feature flags, A/B tests and a forced-update gate, offline-safe and compatible with determinism.

## Decision
- Postgres tables `config_params` and `feature_flags`, served by `/config` with ETag, CDN and Redis caching.
- Defaults are bundled in the app.
- Bucket assignment is computed **on the client** (`hash(flagKey+userId) % 100`), so the response is shared and cacheable.
- Flags that change game rules apply only at a new game or day boundary, and are snapshotted into the save.
- `flag_exposure` analytics events feed A/B analysis.

## Consequences
- No vendor cost, full control, deterministic.
- We build a small admin UI ourselves later.

## Alternatives considered
LaunchDarkly or Firebase Remote Config (cost or vendor lock-in; harder to tie into simulation determinism).
