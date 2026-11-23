# ADR-0019: Redis for rate limiting and caching; CDN for static and cacheable endpoints

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
Public endpoints (`/telemetry`, `/auth`, upload URLs) need abuse protection. Content, config and the app are read far more often than they change.

## Decision
- Redis token buckets per user, device and IP, plus a short-TTL cache for config.
- A CDN serves the web app, `sim.wasm` (brotli), immutable `/content/{version}` bundles and `/config`, cached for 60 s and varying by platform and app version.

## Consequences
- Protects the backend cheaply and cuts most read traffic.
- One more component (Redis); the cache must be invalidated when config changes.

## Alternatives considered
Rate limiting in process memory (inconsistent with several API instances).
