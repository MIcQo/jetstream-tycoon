# Jetstream Tycoon

A single-player, real-time airline tycoon for web, iOS and Android, with AI rival airlines. It works fully offline, with cloud sync when online.

## Read first
- `docs/VISION.md`: game design (what and why)
- `docs/TECH_DESIGN.md`: current technical design (how)
- `docs/DIAGRAMS.md`: architecture, flows, schemas (Mermaid)
- `docs/adr/`: architecture decisions. **Do not contradict an accepted ADR.** To change one, write a new ADR (skill `add-adr`).

## Stack
- Go: simulation as WASM, API server, ingestor
- TypeScript + Vite + Preact: UI; Capacitor for mobile
- Postgres + golang-migrate · ClickHouse · RabbitMQ · Redis · S3/MinIO
- Prometheus + Grafana · Sentry · AdMob

## Layout
```
sim/            pure Go game logic (state, systems, effects, command, content, ai, analytics)
cmd/simwasm/    WASM bridge          cmd/server/  API          cmd/ingestor/  RabbitMQ → ClickHouse
server/         auth, sync, telemetry, config, stores
content/        YAML game content (+ content/i18n)    tools/datagen  tools/balancebot
migrations/postgres  migrations/clickhouse            web/  mobile/  deploy/
```

## Golden rules (non-negotiable)
1. **Determinism in `sim/`** (ADR-0006):
   - No `range` over maps; iterate sorted keys.
   - No `time.Now`, goroutines, `select` or global `math/rand`.
   - Use the RNG stream for the system or AI airline (`rng.For(seed, "<system>", tick)`).
2. **Money is `int64` cents.** Floats only for factors; round once.
3. **All state changes go through commands.** Player, AI and ad rewards all use `sim/command`, with the same validation (ADR-0020).
4. **Content over code** (ADR-0004): new planes, events, missions and personalities are YAML. Code changes only for new *kinds* of condition, effect or system.
5. **Bonuses and penalties are Modifiers or Actions** (ADR-0005). No ad-hoc flags.
6. **No hardcoded user-facing strings.** Use i18n keys; the simulation emits `{key, params}` (ADR-0016).
7. **No personal data** in ClickHouse, Prometheus labels or Sentry. No user IDs or raw paths as metric labels.
8. **Schema changes are migrations** (golang-migrate, additive, with `down` files).
9. **Simulation-affecting remote config** applies only at a day boundary and is snapshotted into the save.

## Commands (Makefile, created in M0)
```
make dev            # docker-compose infra + server + vite
make test           # all unit tests
make sim-test       # sim unit + golden tests
make determinism    # same seed/script: native amd64, arm64, wasm(node) → hashes must match
make content-check  # validate content YAML + i18n keys
make wasm           # build sim.wasm (size-optimised)
make balance        # headless AI-only balance runs → report
make migrate-up     # postgres + clickhouse migrations
make lint           # golangci-lint + custom sim analyzers + eslint
```

## Agents (`.claude/agents/`)
| Agent | Owns |
|---|---|
| `sim-engineer` | `sim/` systems, commands, effects, formulas, WASM bridge |
| `game-ai-engineer` | `sim/ai/`: utility AI, personalities, difficulty |
| `content-designer` | `content/` YAML, events, missions, balance values, i18n text |
| `frontend-engineer` | `web/`: Preact UI, SVG/canvas map, worker, platform adapters |
| `mobile-engineer` | `mobile/`, Capacitor plugins, AdMob, consent, store builds |
| `backend-engineer` | `server/`, `cmd/server`: auth, sync, config, telemetry API, Postgres, Redis, S3 |
| `data-engineer` | ClickHouse schema and rollups, ingestor, RabbitMQ, analytics catalog |
| `devops-engineer` | `deploy/`, CI/CD, Prometheus/alerts, Grafana, Sentry releases, CDN |
| `qa-engineer` | tests: golden, determinism, integration, Playwright |
| `balance-analyst` | runs the balance bot and proposes `balance.yaml` changes |
| `architecture-reviewer` | read-only review against the ADRs and golden rules |

## Skills (`.claude/skills/`)
`add-sim-system`, `add-sim-command`, `add-game-event`, `add-plane-model`, `add-mission-template`, `add-ai-personality`, `add-analytics-event`, `add-feature-flag`, `add-migration`, `add-metric-alert`, `add-i18n-strings`, `add-adr`, `determinism-check`, `run-balance`, `release-mobile`.

## Conventions
- Go: standard layout, `errors.Is/As`, table-driven tests, `golangci-lint` clean. Packages in `sim/` import nothing from `server/`.
- TS: strict mode, no `any`, components in `web/src/ui`, platform code only via `web/src/platform`.
- Commits: Conventional Commits (`feat(sim): ...`). One milestone task per PR, with tests.
- After changing a design: update `docs/TECH_DESIGN.md` and `docs/DIAGRAMS.md` in the same PR.
