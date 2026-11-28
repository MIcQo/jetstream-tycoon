# Jetstream Tycoon — Milestones

Each milestone has a goal, its scope (issues), exit criteria and the owning agents.

**Issue policy**:
- M0–M3 have detailed issues, each about one PR (1–3 days).
- Later milestones start as one **epic** issue with a checklist, split into issues when the milestone starts.

**Labels**:
- Area: `area:sim`, `area:ai`, `area:content`, `area:web`, `area:mobile`, `area:server`, `area:data`, `area:infra`, `area:docs`
- Type: `type:feature`, `type:chore`, `type:test`, `type:epic`
- Size: `size:S` / `size:M` / `size:L` (≤1 / 1–3 / 3–5 days)

---

## M0 — Foundations
**Goal**: an empty but working monorepo where every later piece has a home and CI enforces the golden rules from day one.

| # | Issue | Area | Agent | Size |
|---|---|---|---|---|
| 0.1 | Monorepo scaffold: Go module, folder layout, Makefile with all targets as stubs, editorconfig | infra | devops | S |
| 0.2 | `deploy/docker-compose.yml`: Postgres, ClickHouse, RabbitMQ, Redis, MinIO, Prometheus, Alertmanager, Grafana (provisioned data sources) | infra | devops | M |
| 0.3 | golang-migrate setup for Postgres + ClickHouse with `0001` empty migrations; `make migrate-up/down` | infra | backend | S |
| 0.4 | `sim/` skeleton: State, tick loop, System registry, RNG streams (`rng.For`), command queue, a stub system | sim | sim | M |
| 0.5 | Custom vet analyzer: forbid `range` over maps, `time.Now`, `math/rand`, `go` statements in `sim/` | sim | sim | M |
| 0.6 | `cmd/simwasm` bridge (newGame/command/advance/save/load) + size-optimised build (`-s -w`, wasm-opt) | sim | sim | M |
| 0.7 | Determinism harness: script runner, state hash per day, native vs WASM (Node) compare; `make determinism` | sim | qa | M |
| 0.8 | `web/` Vite + Preact + TS strict, Web Worker loading sim.wasm, "tick counter" demo screen | web | frontend | M |
| 0.9 | i18n skeleton: i18next + ICU, `en.json`, en-XA pseudo-locale, missing-key check | web | frontend | S |
| 0.10 | Capacitor shell (iOS/Android) running the web build; platform adapter interfaces + web stubs | mobile | mobile | M |
| 0.11 | `cmd/server` skeleton: chi router, `/healthz`, `/metrics` (internal port), config, Sentry, graceful shutdown | server | backend | S |
| 0.12 | Sentry wiring: web/Capacitor SDK + WASM panic recovery hook + server; release naming | infra | devops | S |
| 0.14 | `sim/detmath` (Pow/Exp/Log ports, RoundCents, Finite, Irwin–Hall normal) + bit-exact test vectors across amd64/arm64/wasm (ADR-0021) | sim | sim | M |
| 0.15 | Measure the real WASM size (with `expr`) and the instantiate time on a low-end Android WebView; set CI budgets | sim | sim | S |
| 0.13 | CI pipeline: lint → unit → content-check → determinism (amd64, arm64, wasm) → wasm size budget → build | infra | devops | M |

**Exit criteria**:
- `make dev` starts the infrastructure, the server and the web app.
- The browser shows ticks advancing from sim.wasm.
- CI is green, the determinism job runs, and a map-range in `sim/` fails lint.
- `detmath` vectors are bit-identical on all 3 targets, and the WASM size and instantiate budgets are measured and enforced.

## M1 — World data
**Goal**: real countries and airports as validated, versioned content.

| # | Issue | Area | Agent | Size |
|---|---|---|---|---|
| 1.1 | `tools/datagen`: import Natural Earth countries → simplified TopoJSON + pre-projected SVG paths (2 detail levels) | content | sim | L |
| 1.2 | datagen: OurAirports large/medium (~1,000) with IATA, city, lat/lon, base level from size | content | sim | M |
| 1.3 | datagen: country stats (population, GDP per capita) + business/tourism profile, hemisphere/climate; attribution file | content | content | M |
| 1.4 | datagen: localized country/city names (Natural Earth / OurAirports fields) → `content/i18n` | content | content | S |
| 1.5 | datagen: precomputed great-circle distances + top-30 candidate destinations per airport | content | sim | M |
| 1.6 | Content schemas + loader + validator (`make content-check`): planes, airport levels, balance, seasons, holidays | content | sim | M |
| 1.7 | Initial content: 3 plane tiers, airport levels 1–5, `balance.yaml`, seasons and holidays tables | content | content | M |
| 1.8 | Content build → `content.json` with `contentVersion`; embedded in WASM/server | content | sim | S |

**Exit criteria**: `make content-check` passes; content.json is under 1.5 MB brotli; the map paths render in a test page.

## M2 — Simulation core and local saves
**Goal**: start, run, save and load a game in the browser, deterministically.

| # | Issue | Area | Agent | Size |
|---|---|---|---|---|
| 2.1 | Full state model (multi-airline from day one, per ADR-0020) + sorted-ID helpers | sim | sim | M |
| 2.2 | Command framework: Validate/Apply, typed errors with i18n keys, registry, `setSpeed`, `newGame` | sim | sim | M |
| 2.3 | Effects engine: Modifier/Action, `effects.Apply`, expiry, stacking order + tests | sim | sim | M |
| 2.4 | Save format: MessagePack + gzip (ADR-0024), schemaVersion, migrator framework + JSON debug export | sim | sim | M |
| 2.5 | Bridge deltas: changed-entity tracking, MessagePack transferable buffers, 4/s UI throttle | sim | sim | M |
| 2.6 | `SaveStore` (IndexedDB web, Filesystem Capacitor), autosave per day/pause/background, 3 rolling copies | web | frontend | M |
| 2.7 | New-game flow UI: airline name, home country picker, difficulty (stored for M8.5) | web | frontend | S |
| 2.8 | Golden test scripts + save migration fixture tests | sim | qa | S |

**Exit criteria**: new game → run 30 days at 4x → save → reload → identical hash; determinism green.

## M3 — Core economy loop
**Goal**: buy, fly, earn. The first profitable route.

| # | Issue | Area | Agent | Size |
|---|---|---|---|---|
| 3.1 | Countries: purchase price formula, exclusive ownership, buy command | sim | sim | S |
| 3.2 | Landing rights: lease, yearly fee, departures cap, expiry | sim | sim | M |
| 3.3 | Airports: build (activate), upgrade, levels → capacity/slots/max tier | sim | sim | S |
| 3.4 | Planes: buy, assign to route, upkeep; range and airport-level validation | sim | sim | M |
| 3.5 | Routes: open/close, single price (classes come in M5), demand formula on existing routes only | sim | sim | M |
| 3.6 | Flights system: schedule legs, flight time, pax, revenue, fuel and fees, success/delay/cancel | sim | sim | L |
| 3.7 | Fuel market: mean-reverting random walk + 90-day history | sim | sim | S |
| 3.8 | Economy system: daily upkeep, cash, net worth; ledger summary for finance charts | sim | sim | M |
| 3.9 | Golden scripts: "first route profit" and "overextended airline" | sim | qa | S |

**Exit criteria**: a headless script reaches positive daily profit by day 3 with the starter kit; formulas unit-tested.

---

## Later milestones (epics, split when started)

### M4 — Map UI and panels
- SVG countries/airports and canvas routes/planes layers (shared projection, pan/zoom/pinch).
- Top bar, side panel / bottom sheet, tab bar.
- Country/airport/route/plane panels; buy and assign flows.
- Colour-blind patterns, dark mode, en-XA check.
- **Exit**: the full M3 loop is playable by touch and mouse at ≥ 30 fps on a mid-range phone.

### M5 — Pricing depth
- Seat classes + seat config/reconfigure.
- Per-class prices and elasticity, reference prices.
- Background carriers and market share.
- `routeForecast` query + price slider preview.
- Seasons/holidays modifiers + 12-month outlook.
- **Exit**: price changes visibly shift load factor and profit; forecast matches the simulation within 5%.

### M6 — Network strategy
- Hubs (level ≥ 3, limits).
- Connecting passengers through hubs (seat allocation, fares).
- Aging, condition, overhaul.
- Used-plane market + selling.
- **Exit**: a hub network beats point-to-point in a golden scenario.

### M7 — Events, investments, missions
- Event scheduler (weights, cooldowns, conditions), prompts with expiry, delayed reveals.
- 8 events + 3 investments.
- Mission board + 5 templates.
- News feed.
- **Exit**: events and missions appear, resolve and are replayable.

### M8 — Finance and onboarding
- Loans, credit score, overdraft, status state machine, liquidation.
- Restructuring and bankruptcy.
- Milestones/achievements, finance charts.
- Data-driven tutorial.
- **Exit**: a new player finishes the tutorial in ≤ 10 min; the full status chain is covered by golden tests.

### M8.5 — AI airlines
- Controller interface; utility AI generators + scoring.
- 5 personalities, difficulty table.
- Multi-airline markets, country buyouts, takeovers, AI bankruptcy.
- Rivals panel, map colours.
- AI-only balance bot in CI.
- **Exit**: Normal-difficulty guardrails hold (see `run-balance`); AI ≤ 5 ms per airline-day in WASM. **Offline MVP complete.**

### M9 — Backend: accounts, sync, config
- Auth: guest, Apple, Google, email magic link, refresh rotation, account deletion.
- Saves: object storage + presigned URLs, commit, 409 conflict UI.
- Remote config + flags (client bucketing, setConfig).
- Redis rate limits; CDN headers.
- **Exit**: start on web, continue on phone; a conflict prompt appears when expected.

### M10 — Analytics and observability
- Client telemetry queue.
- `/telemetry` → RabbitMQ (confirms) → ingestor → ClickHouse (typed columns, codecs, TTL) + rollups.
- Event catalog v1.
- Prometheus metrics/exporters/alerts + runbooks.
- Grafana dashboards.
- **Exit**: dashboards show real sessions; killing ClickHouse for 10 min loses no events.

### M11 — Mobile launch
- AdMob rewarded/interstitial with remote pacing.
- UMP + ATT consent.
- Local notifications; store assets.
- Sentry native + release health.
- TestFlight / Play internal beta, staged rollout plan.
- **Exit**: the beta is installed by testers with a crash-free rate ≥ 99.5%.
