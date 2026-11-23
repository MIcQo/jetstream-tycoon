# Architecture Decision Records

Decisions are recorded in Nygard format (see `template.md` and ADR-0001). The current design is in `../TECH_DESIGN.md`.

| ADR | Title | Status |
|---|---|---|
| [0001](0001-record-architecture-decisions.md) | Record architecture decisions | Accepted |
| [0002](0002-client-side-deterministic-simulation-wasm.md) | Run the game simulation on the device as deterministic Go compiled to WebAssembly | Accepted |
| [0003](0003-single-codebase-capacitor.md) | One TypeScript/Preact codebase for web, iOS and Android via Capacitor | Accepted |
| [0004](0004-data-driven-content.md) | Data-driven content with one shared schema for events and investments | Accepted |
| [0005](0005-effects-modifier-system.md) | A single effects/modifier system for all bonuses and penalties | Accepted |
| [0006](0006-determinism-rules.md) | Strict determinism in the simulation | Accepted |
| [0007](0007-postgres-golang-migrate.md) | Postgres for accounts, save metadata and config; golang-migrate for schemas | Accepted |
| [0008](0008-saves-object-storage-optimistic-sync.md) | Save files in object storage, synced with optimistic locking and presigned URLs | Accepted |
| [0009](0009-binary-save-format.md) | MessagePack + zstd as the save format | Accepted |
| [0010](0010-guest-first-auth.md) | Guest-first accounts with Apple/Google/email linking; JWT + rotating refresh tokens | Accepted |
| [0011](0011-clickhouse-analytics.md) | ClickHouse for gameplay and product analytics | Accepted |
| [0012](0012-rabbitmq-telemetry-queue.md) | RabbitMQ between the API and a separate ClickHouse ingestor | Accepted |
| [0013](0013-prometheus-grafana.md) | Prometheus + Grafana for technical metrics and alerting | Accepted |
| [0014](0014-sentry-crash-reporting.md) | Sentry for crash and error reporting | Accepted |
| [0015](0015-in-house-remote-config-flags.md) | In-house remote config and feature flags | Accepted |
| [0016](0016-key-based-localization.md) | Key-based localization with ICU MessageFormat | Accepted |
| [0017](0017-map-svg-canvas-layers.md) | World map rendered as SVG base + Canvas overlay + HTML labels | Accepted |
| [0018](0018-ads-monetization.md) | Free-to-play monetization with AdMob rewarded ads; rewards as simulation commands | Accepted |
| [0019](0019-redis-cdn.md) | Redis for rate limiting and caching; CDN for static and cacheable endpoints | Accepted |
| [0020](0020-ai-airlines.md) | AI airlines as full simulation entities driven by utility AI through the player's command path | Accepted |
| [0021](0021-deterministic-math-and-modifier-stacking.md) | Deterministic math library, rounding and modifier stacking order | Accepted |
| [0022](0022-save-keys-account-merge-deletion.md) | Per-save object keys, account merge on sign-in, and outbox-based account deletion | Accepted |
| [0023](0023-save-retention-staging-upload-limits.md) | Server-side save revision retention, staged uploads and upload abuse limits | Accepted |
