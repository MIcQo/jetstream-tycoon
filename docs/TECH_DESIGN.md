# Jetstream Tycoon — Technical Design (Phase 2, rev 2)

Architecture decisions and their reasons are recorded in [`adr/`](adr/README.md).

## 0. Key decisions
| Topic | Decision |
|---|---|
| Platforms | Web, iOS, Android from **one TypeScript/Preact codebase**; mobile via **Capacitor** |
| Offline | **Required.** The game simulation runs **on the device** |
| Simulation | Pure Go, compiled to **WebAssembly**, running in a Web Worker (web + mobile) |
| Server | Go service for accounts, cloud save sync and analytics ingest. It never runs the game loop |
| Accounts | Play instantly as guest; link Apple / Google / email to sync across devices |
| Storage | Postgres (accounts, save metadata, config) · **S3-compatible object storage** (save files; MinIO locally) · ClickHouse (gameplay + product analytics) |
| Queue | **RabbitMQ** quorum queue between the API and a separate **ingestor** service → ClickHouse |
| Cache / limits | **Redis**: rate limiting (`/telemetry`, `/auth`) + config cache |
| CDN | In front of `/content` bundles, `/config`, the web app and `sim.wasm` |
| Migrations | golang-migrate for both (`migrations/postgres`, `migrations/clickhouse`) |
| Dashboards | Grafana with the ClickHouse data source |
| Ads | AdMob via Capacitor plugin (rewarded + interstitial); none on web for MVP |
| Remote config | Own Go service plus Postgres; bundled defaults; feature flags and A/B splits |
| i18n | Key-based strings, ICU MessageFormat (`i18next` + ICU plugin), `Intl` formatting |
| Crashes | Sentry: `@sentry/capacitor` (JS + native), `sentry-go` (server) |
| Tech metrics | **Prometheus + Grafana** (+ Alertmanager), see §21 |

## 1. High-level architecture
```
┌──────────── Device (Web / iOS / Android via Capacitor) ────────────┐
│  UI thread: Preact + SVG map                                       │
│      ▲ deltas / notifications          │ commands                  │
│      │                                 ▼                           │
│  Web Worker:  sim.wasm (Go)  ── tick loop, rules, RNG, effects     │
│      │ snapshots                         │ analytics events        │
│      ▼                                   ▼                         │
│  SaveStore (local)                 TelemetryQueue (local, durable) │
└──────┬───────────────────────────────────┬─────────────────────────┘
       │ sync when online                  │ flush when online
       ▼                                   ▼
┌──────────────────────── Go API server (stateless) ─────────────────┐
│  /auth  /saves (sync, conflict detection)  /telemetry  /content    │
└──────┬─────────────────────────────────────┬───────────────────────┘
       ▼              ▼            ▼             ▼
   Postgres     Object storage   Redis       RabbitMQ ──► ingestor ──► ClickHouse ──► Grafana
 (metadata)     (save files)   (limits)     (durable)    (batches)
```
A CDN serves the web app, `sim.wasm`, content bundles and `/config`. Saves are uploaded and downloaded straight to/from object storage through presigned URLs.

## 2. Repository layout (monorepo)
```
sim/                  pure Go game logic (no I/O, no time.Now, no global rand)
  state/ systems/ effects/ command/ content/ analytics/
cmd/simwasm/          WASM entry: JS bridge (syscall/js) around sim
cmd/server/           API server main
cmd/ingestor/         RabbitMQ consumer → ClickHouse batch writer
server/               auth, sync, telemetry, content endpoints, stores
tools/datagen/        builds world data (Natural Earth, OurAirports, World Bank)
tools/balancebot/     headless runs of the simulation for balancing
content/              YAML sources → compiled to content.json at build time
migrations/postgres/  migrations/clickhouse/
web/                  Vite + Preact app (shared by all platforms)
  src/worker/ src/ui/ src/map/ src/platform/ (web vs Capacitor adapters)
mobile/               Capacitor config, ios/, android/ projects
deploy/               docker-compose (postgres, clickhouse, grafana, server)
```

## 3. Simulation core (runs on device)
- **Pure and deterministic.** Same seed and same command log give the same state. Enables golden tests, the balance bot, and optional server-side replay validation later.
- Tick = 1 game hour. 1 day = 60 s at 1x (2.5 s per tick). Speeds: pause, 1x, 2x, 4x. The tick timer lives in the Worker.
- Commands are queued and applied at the tick boundary.
- **The app pauses while closed or backgrounded** (Capacitor `appStateChange` → pause + autosave). No idle progress.

```go
type System interface {
    Name() string
    OnTick(c *TickCtx)
    OnDay(c *TickCtx)
}
// Registered in order: Season → Fuel → AI (emits commands for next tick) → Competition → Demand → Flights
//   → Aging → Economy → Leases → Loans → Offers → Events → Missions → Tutorial → Milestones → Analytics
```

### Determinism rules (enforced in CI)
1. **No Go map iteration in game logic.** Use sorted slices of IDs (`slices.Sorted(maps.Keys(m))`) or ordered containers. A lint rule (custom `go vet` analyzer) flags `range` over maps in `sim/`.
2. **No `time.Now`, goroutines, `select`, or global `math/rand`** in `sim/`. All time comes from `Tick`.
3. **Separate RNG streams per system**: `rng = PCG(hash(seed, systemID, tick))`. Adding a new system doesn't change the random numbers of existing ones, which keeps old saves and replays stable.
4. **Money in int64 cents.** Floats only for factors; results rounded to cents at a single point, with explicit `float64()` conversions to avoid fused multiply-add differences between CPU architectures.
5. **Simulation-affecting remote config is snapshotted into the save** (see §18), so a replay uses the same values.
6. **CI cross-check**: the same seed + command script runs natively (amd64), as WASM in Node, and on arm64. The final state hashes must match.
**TickCtx** holds state, content, RNG, effects, `Notify()` and `Track(analyticsEvent)`.

### WASM bridge (JSON in / out)
```
sim.newGame({airlineName, homeCountry, seed}) → snapshot
sim.load(snapshot)          sim.save() → snapshot
sim.command({type, ...})    → {ok, error?}
sim.advance(ticks)          → {delta, notifications, prompts, analytics[]}
sim.query({type: "routeForecast", routeId, price}) → result
```
- Build: standard Go `GOOS=js GOARCH=wasm` with `-trimpath -ldflags="-s -w"`, then `wasm-opt -Oz`. Served brotli-compressed from the CDN (target ≤ 2 MB).
  - Loaded with `WebAssembly.instantiateStreaming` behind a branded loading screen; cached by the service worker on web and bundled inside the mobile apps.
  - Fallback if size is still a problem: TinyGo, which needs checking against `expr`.
- **Worker → UI traffic**: the simulation can run at 4x, but the UI gets at most **~4 updates per second**.
  - Each update is a compact delta of only the changed entities, sent as a transferable `ArrayBuffer` (MessagePack).
  - Plane positions are **not** streamed. The UI receives `{planeId, depart, arrive}` once per flight and interpolates locally.
- Content is compiled to JSON, so no YAML parser is needed in WASM.
- TinyGo is an option later if size matters.

## 4. State model
The **player is one airline among several**. Every system loops over all airlines, sorted by ID. Ownership lives on the world entities.
```go
type State struct {
  Version int; Seed uint64; Tick int64; Speed int
  Difficulty Difficulty                       // easy | normal | hard (AI count, skill)
  Airlines  map[AirlineID]*Airline            // player = "P", AI = "A1".."A6"
  Countries map[string]*Country               // ISO2 → ownerID ("" = unowned), purchasedTick
  Airports  map[string]*Airport               // IATA → level, ownerID (via country), hubOf
  LandingRights map[LRKey]*LandingRight       // (airlineID, IATA) → lease, expiry, departures cap
  Planes    map[ID]*Plane                     // airlineID, model, level, ageDays, condition, seatsEco/Biz, routeID, state
  Routes    map[ID]*Route                     // airlineID, from, to, priceEco, priceBiz, planeIDs
  Markets   map[PairKey]*Market               // airport pair → background carriers, last shares per airline
  Loans     map[ID]*Loan                      // airlineID, ...
  Offers    []Offer                           // country buyouts, airline takeovers (between any airlines)
  Missions  []Mission                         // player only
  ConfigSnapshot map[string]any               // sim-affecting remote config (determinism)
  Modifiers []effects.Modifier                // scope may include airlineID
  Pending   []PendingEvent                    // player prompts; AI resolves its own events immediately
  UsedMarket UsedMarket; Fuel FuelMarket      // shared by all airlines
  Milestones map[string]int64
}
type Airline struct {
  ID AirlineID; Name, Color string; Controller Controller // human | ai
  Personality string                          // AI only, content ID
  Home string; CashCents int64; Reputation float64; CreditScore int
  Status Status; RestructuringsLeft int; Hubs []string
}
// Maps are for lookup only. Iteration always goes over sorted keys (see the determinism rules).
// Full diagram: docs/DIAGRAMS.md §8
```

## 5. Content (data-driven)
These are unchanged from rev 1:
- `planes.yaml`, `airport_levels.yaml`, `balance.yaml`
- `events/*.yaml`: one schema for incidents and investments. Each event has choices; each choice has a cost and outcomes; each outcome has a chance, effects and an optional delayed reveal.
- Conditions and formulas use `expr-lang/expr`.
- Each content build has a `contentVersion`, which is stored in saves and analytics.
- Later: the server can serve newer content bundles, so new events ship without an app release.

## 6. Effects / modifiers
Unchanged from rev 1:
- A modifier is `{scope, target, stat, op(mul|add|set), value, expiresTick}`.
- Instant effects (cash, reputation, damage, give plane) are separate `Action`s.

## 7. Formulas
Unchanged from rev 1, see that table: demand, reference price, price factor `clamp((ref/price)^1.5, 0.1, 2)`, attractiveness, fuel random walk, aging, resale, loans, credit score.

### 7.1 New mechanics (formulas)
| Thing | Formula / rule |
|---|---|
| Season factor | `season[country][month] × Π holiday[country][date]` (generated as modifiers by the Season system from `seasons.yaml` and `holidays.yaml`) |
| Class demand | `D_biz = D · bizShare(gdp, businessProfile) · seasonBiz`; `D_eco = D − D_biz` |
| Class reference price | `ref_eco` as before; `ref_biz = 3.5 · ref_eco` |
| Class elasticity | economy 1.5, business 0.8; business service weight ×2 |
| Market share (per class) | `A_you / (A_you + Σ A_comp_i)`; with no competitors: `min(1, A_you)` |
| Competitor price | daily: `p += 0.1(ref·(1+noise) − p)`; if your share > 60% for 7 days: undercut 5–10% (chance) |
| Direct pax per day | `min(D_class · share, seats_class_per_day)` |
| Connecting pax | for each hub H, pairs of legs (O→H, H→D) with no direct O–D: `D_OD · 0.4 · min(share_leg1, share_leg2)`, filling leftover seats on both legs |
| Connecting fare | `(fare_leg1 + fare_leg2) · 0.85` |
| Landing rights | upfront `0.05–0.15 · countryPrice · sizeFactor`, yearly lease `0.3 · upfront`, fees ×1.5, departures/day cap by airport size |
| Seat config | `eco + 2·biz ≤ model.seatUnits`; reconfiguring costs money and takes 48 ticks of downtime |

Demand is calculated **only for routes that exist** (and connecting pairs through hubs), cached per day, and recalculated on demand for previews. It is never computed for all ~500k airport pairs.

### 7.2 Content files added
`seasons.yaml`, `holidays.yaml` (by game-calendar dates), `competitors.yaml` (background carriers: name parts per region, behaviour profiles), `ai_personalities.yaml`, `difficulty.yaml`, `airline_names.yaml`, `missions/*.yaml` (templates: goal expression, parameter pickers, deadline, reward, penalty), `tutorial.yaml` (steps: textKey, highlight UI ID, `completeWhen` expression), `ads.yaml` (rewarded reward definitions as Effects/Actions).

## 7.3 AI airlines
### Principles
- **Same rules as the player.** AI airlines are full `Airline` entities. They act **only by issuing the same commands** as the player, through the same validation. There are no hidden bonuses, except an optional small cash multiplier on Hard (`difficulty.yaml`).
- **Deterministic.** The AI runs inside the simulation (WASM) with its own RNG stream per airline (`hash(seed, "ai", airlineID, tick)`), so AI behaviour replays exactly.
- **Pluggable controllers.** `Controller` is an interface (`human`, `ai`, later `remote` for multiplayer):
```go
type Controller interface { Decide(c *DecideCtx, a AirlineID) []command.Command }
```

### Decision model: utility AI
- Each AI decides **once per game day**, at a staggered hour (A1 at 02:00, A2 at 05:00, ...) to spread CPU load. It also reacts at once to urgent triggers: cash < 0, an offer received, a price war.
- **Candidate generators** propose actions:

| Generator | Examples |
|---|---|
| Expansion | buy an unowned country, lease landing rights, make a country buyout offer |
| Network | open a route from **precomputed top-K candidate pairs** (datagen keeps the 30 best destinations per airport by demand/distance), designate a hub |
| Fleet | buy new or used planes, assign, upgrade, overhaul, reconfigure seats, sell old planes |
| Pricing | adjust each route's price by ±2–10% using the route forecast (hill-climbing on profit) |
| Finance | take, repay or refinance loans; keep a cash reserve |
| Corporate | accept or decline offers, make takeover offers, choose event outcomes by expected value × risk tolerance |

- Each candidate gets a score: **score = expected profit over horizon × personality weights − risk penalty**.
  - The AI picks the best actions within its budget, up to `maxActionsPerDay`.
  - Forecasts use the same `routeForecast` as the player's UI, with **difficulty noise** added (Easy ±30%, Normal ±12%, Hard ±4%).

### Personalities (`ai_personalities.yaml`)
| Personality | Behaviour |
|---|---|
| Budget | low prices (0.75–0.9 × reference), economy-only, high load factor, older or used planes |
| Premium | high prices, more business seats, new planes, high service |
| Regional | expands within a few nearby countries first, many short routes, strong hub |
| Aggressive | fast expansion on loans, undercuts the leader, starts price wars, bids on countries |
| Conservative | low debt, high cash reserve, profitable routes only |

Each personality is a set of weights: `riskTolerance, priceBias, bizShareBias, expansionRate, debtLimit, regionFocus, reserveDays, aggression, planeAgePref`. New personalities need no code.

### Difficulty (`difficulty.yaml`)
| | Easy | Normal | Hard |
|---|---|---|---|
| AI airlines | 2 | 4 | 6 |
| Forecast noise | ±30% | ±12% | ±4% |
| Mistake chance (random suboptimal pick) | 20% | 8% | 2% |
| Max actions per day | 2 | 4 | 6 |
| Reaction delay to your price changes | 7 days | 3 days | 1 day |
| AI start cash multiplier | 0.8 | 1.0 | 1.2 |

AI home countries are picked by seed, avoiding the player's home and each other's regions.

### Market share with real competitors
- On an airport pair, the airlines competing are **every airline flying that pair** (player and AI) plus **background carriers** (the virtual competitors from rev 3, now standing in for the airlines we don't model).
- `share_i = A_i / (Σ A_airlines + Σ A_background)`, per class. Connecting passengers through a hub compete on the O–D pair against direct flights by other airlines, with a connection penalty (×0.7 attractiveness).
- Background carriers shrink on a pair when real airlines add capacity there, so the market doesn't saturate.

### Country ownership and buyouts (exclusive)
- Each country has at most **one owner**. Other airlines reach its airports only through landing rights, which the owner can't block. The owner earns a share of the landing fees, which gives owning a country extra value.
- **Buyout offers** (any airline → owner): `offer ≥ countryValue × (1 + premium)` is accepted if:
  - `premium` depends on how important the country is to the owner: home = never sold; hub country ×1.5; otherwise 0.2–0.5, scaled by personality.
  - Financial distress lowers the premium: DISTRESS −50%, LIQUIDATION accepts any offer ≥ 0.8 × value.
  - The AI answers immediately. The player gets a prompt (an `Offer` that expires after 7 days).
- On transfer, the airports and their levels go to the buyer. The seller's existing routes there become **landing rights for 1 year** (grace period).

### Takeovers (both ways)
- **Airline valuation** = net worth + goodwill, where goodwill = 3 × average monthly route profit (last 90 days) × (reputation / 50).
- An offer is accepted when `price ≥ valuation × (1 + takeoverPremium)`. The premium is lower when the target is in DISTRESS or LIQUIDATION, or is losing money.
- **The player acquires an AI**: all of its assets, routes and debt merge into the player's airline; its brand is retired. A monopoly check blocks taking over the market leader if the result would hold > 50% of world capacity.
- **An AI offers to buy the player** only when the player is in DISTRESS or LIQUIDATION. Accepting ends the game with a **"Sold"** ending and a final score, as an alternative to restructuring.
- **AI bankruptcy**: its planes go to the used market, its countries become unowned (purchasable), and its routes close. A news event follows, plus a "Market opportunity" mission.

### Player-facing features
- **Rivals panel**: ranking by net worth, countries, routes, passengers, reputation. Per rival: personality hint, home, recent moves.
- **Map**: each airline has its own colour and pattern for owned countries; rival routes can be toggled on and off.
- **News feed**: "SkyMerid opened 4 routes in Brazil", "Price war on LHR–JFK".
- **Missions** involving rivals: "Beat Aurora Air's share on route X", "Acquire a rival".
- **Milestones**: "First takeover", "#1 airline by net worth".

### Performance budget
AI decisions take ≤ 5 ms per airline per day in WASM on a mid-range phone, with a hard cap on candidates per generator (e.g. 50). The `Markets` index means demand is only computed for pairs that some airline actually flies.

### Bonus: the AI is the balance bot
`tools/balancebot` runs headless AI-only games (all personalities × difficulties × seeds) for 10 game years and reports bankruptcy rates, net-worth curves and dominant strategies in CI.

## 8. Bankruptcy and restructuring
```
OK → WARNING (cash<0) → DISTRESS (missed payment) → LIQUIDATION (14 days negative, auto-sell)
   → if restructuringsLeft > 0: RESTRUCTURING offer (accept = continue, decline = bankrupt)
   → else BANKRUPT (read-only save)
```
Restructuring: all debt becomes one loan (10 years, high rate); the lowest-value 50% of planes are sold; non-hub airports lose one level; reputation −30; credit score set to 250; home country and hubs are kept.

## 9. Local saves
- `SaveStore` interface with one implementation per platform:
  - **Web**: IndexedDB.
  - **iOS/Android**: Capacitor Filesystem (the app data dir; avoids iOS WebView storage eviction).
- Snapshot = **zstd(MessagePack state)**, about 3–5x smaller and faster than gzip JSON. A JSON export exists for debugging and bug reports.
  - Each snapshot also has metadata `{saveId, revision, tick, schemaVersion, contentVersion, deviceId, updatedAt, summary{cash, netWorth, day}}`.
- Autosave: every game day, on pause, and when the app goes to the background. Keep the last 3 rolling copies (corruption safety).
- Old saves are upgraded on load by versioned Go migrator funcs inside the simulation.

## 10. Accounts and auth (server)
- **Guest**: on first launch the device generates `deviceId` and `localUserId`, and the game is fully playable offline. The first time it goes online, it registers `POST /auth/guest` → anonymous user + tokens.
- **Link**: Sign in with Apple (required on iOS if Google is offered), Google, or email + magic link. Linking attaches the guest to that identity; signing in on a new device pulls the cloud saves.
- **Tokens**: JWT access token (15 min) + rotating refresh token (stored hashed). Native tokens go in Keychain/Keystore via Capacitor Secure Storage.
- **Account deletion** endpoint (an App Store requirement) removes the Postgres rows and deletes the user's rows in ClickHouse.

```sql
-- migrations/postgres/0001_init.up.sql
CREATE TABLE users (id UUID PRIMARY KEY, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), deleted_at TIMESTAMPTZ);
CREATE TABLE identities (user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL, subject TEXT NOT NULL, email TEXT, PRIMARY KEY (provider, subject));
CREATE TABLE devices (id UUID PRIMARY KEY, user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  platform TEXT NOT NULL, app_version TEXT, last_seen TIMESTAMPTZ);
CREATE TABLE refresh_tokens (token_hash BYTEA PRIMARY KEY, user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id UUID, expires_at TIMESTAMPTZ NOT NULL, revoked_at TIMESTAMPTZ);
CREATE TABLE saves (id UUID PRIMARY KEY, user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL, revision BIGINT NOT NULL, tick BIGINT NOT NULL,
  schema_version INT NOT NULL, content_version TEXT NOT NULL,
  summary JSONB NOT NULL,                              -- day, cash, net worth (for list + conflict UI)
  blob_key TEXT NOT NULL,                              -- object storage key: saves/{user}/{save}/{revision}
  blob_size INT NOT NULL, blob_sha256 BYTEA NOT NULL,  -- integrity + "unchanged" detection
  device_id UUID, updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE INDEX ON saves (user_id);
```
Save files live in **object storage**, not Postgres, which keeps the `saves` rows small and avoids TOAST bloat from frequent rewrites. A bucket lifecycle rule keeps the last 3 revisions per save; older ones expire.
```sql
```

## 11. Cloud save sync
Single player, so the protocol is simple and conflicts are explicit:
- **Skip when unchanged**: the client compares the snapshot's sha256 with the last synced hash and uploads nothing if they match.
- Upload in **3 steps**, so save bytes never pass through the API:
  1. `POST /saves/{id}/upload-url {baseRevision, sha256, size}` → presigned PUT URL (rejected early with 409 if `baseRevision` is stale).
  2. The client PUTs the blob directly to object storage.
  3. `PUT /saves/{id}` commits `{baseRevision, blobKey, sha256, summary}`. This is a conditional update (`WHERE revision = baseRevision`), and the server checks that the object exists with the right size and hash.
  - If the revision matches: `revision+1`, and the new revision is returned.
  - Otherwise: return **409** with the server's metadata. The client shows "This device: Day 210, $45M / Cloud: Day 180, $30M — keep which?"
- `GET /saves` lists metadata. `GET /saves/{id}` returns metadata + a presigned download URL.
- Sync triggers: app start, every 5 min of play while online, app background, manual "Sync now".
- Later (anti-cheat, for leaderboards): upload the command log; the server replays it in the same Go simulation to verify.

## 12. Analytics → ClickHouse
### Flow
1. The simulation emits gameplay events via `Track()`; the UI emits product events.
2. Both go into a durable **TelemetryQueue** on the device (IndexedDB / file), so nothing is lost offline.
3. When online, the client sends batches: `POST /telemetry` (≤500 events, gzip). Each event has a client-generated `event_id`.
4. The API checks the Redis rate limit, then validates and enriches each event (user_id, geo country from IP, received_at, then drops the IP).
5. The API publishes the batch to **RabbitMQ** (`telemetry.events`, a durable **quorum queue**, persistent messages, **publisher confirms**). It returns **202 only after the confirm**, so the client deletes its local copy only when the events are safe.
6. The **ingestor** service (`cmd/ingestor`) consumes with prefetch about 10k:
   - It builds one ClickHouse batch per ~5 s or 10k events (clickhouse-go v2 native batch), using `insert_deduplication_token` = hash of the message IDs.
   - It acks the messages (`multiple=true`) **only after** ClickHouse accepts the insert.
   - On a failed insert: nack + requeue, with exponential backoff.
   - Malformed messages go to the dead-letter exchange → `telemetry.dlq`, where they can be inspected and replayed.
7. Delivery is **at-least-once**. Duplicates are removed by `event_id` (ReplacingMergeTree) plus the insert dedup token (`non_replicated_deduplication_window` on single-node ClickHouse).
8. If ClickHouse is down, events wait in RabbitMQ, which has queue length and disk limits with alerting. If RabbitMQ is down, the API returns 503 and clients keep events in their local queue and retry later. **No data loss in either case.**
9. The API and ingestor scale separately. Publishing sits behind a `Sink` interface, which has an in-memory implementation for tests and local dev.

### Event catalog (v1)
- **Product**: `app_open`, `session_start`, `session_end{duration}`, `screen_view{screen}`, `tutorial_step{step}`, `account_linked{provider}`, `sync_conflict{resolution}`, `consent_changed`.
- **Gameplay**: `game_started{home}`, `country_bought{iso, price}`, `airport_built/upgraded{iata, level}`, `plane_bought/sold/upgraded/overhauled{model, level, age, value, used}`, `route_opened{from, to, km, price}`, `price_changed{route, old, new, refPrice}`, `route_daily{route, flights, pax, loadFactor, revenue, cost}` (aggregated daily, **not per flight**), `event_presented/choice/outcome{eventId, choice, outcome}`, `loan_taken/repaid/missed{amount, rate}`, `status_changed{from, to}`, `bankrupt{day, netWorthPeak}`, `milestone_reached{id}`, `daily_snapshot{cash, netWorth, debt, routes, planes, reputation, fuel}`.

### Schema
- Frequently filtered fields are **typed columns**, not Map lookups. Rare fields stay in the Maps.
- **Codecs** keep storage small.
- **Raw events are kept 90 days**; dashboards read **rollups** kept 2 years.
```sql
-- migrations/clickhouse/0001_events.up.sql
CREATE TABLE events (
  event_id        UUID                     CODEC(ZSTD(1)),
  event_name      LowCardinality(String),
  ts              DateTime64(3, 'UTC')     CODEC(Delta, ZSTD(1)),   -- client time
  received_at     DateTime64(3, 'UTC')     CODEC(Delta, ZSTD(1)),
  event_date      Date MATERIALIZED toDate(ts),
  user_id         UUID,
  device_id       UUID                     CODEC(ZSTD(1)),
  session_id      UUID                     CODEC(ZSTD(1)),
  save_id         UUID                     CODEC(ZSTD(1)),
  platform        LowCardinality(String),        -- web | ios | android
  app_version     LowCardinality(String),
  content_version LowCardinality(String),
  geo_country     LowCardinality(String),
  game_day        UInt32                   CODEC(Delta, ZSTD(1)),
  -- hot gameplay fields as typed columns (0 / '' when not applicable, no Nullable)
  route_id        String                   CODEC(ZSTD(1)),
  ref_id          LowCardinality(String),  -- eventId / missionId / modelId / milestone / screen
  amount_cents    Int64                    CODEC(T64, ZSTD(1)),     -- price, revenue, cost, loan
  value           Float64                  CODEC(Gorilla, ZSTD(1)), -- load factor, fps, ms
  props           Map(LowCardinality(String), String)  CODEC(ZSTD(3)),
  metrics         Map(LowCardinality(String), Float64) CODEC(ZSTD(3))
) ENGINE = ReplacingMergeTree(received_at)          -- removes duplicates from retries
PARTITION BY toYYYYMM(event_date)
ORDER BY (event_name, event_date, user_id, event_id)
TTL event_date + INTERVAL 90 DAY;

-- daily active users (pre-aggregated)
CREATE TABLE dau (day Date, platform LowCardinality(String), users AggregateFunction(uniq, UUID))
ENGINE = AggregatingMergeTree ORDER BY (day, platform);
CREATE MATERIALIZED VIEW dau_mv TO dau AS
SELECT toDate(ts) AS day, platform, uniqState(user_id) AS users FROM events
WHERE event_name = 'session_start' GROUP BY day, platform;

-- route economics per day (balance dashboards)
CREATE TABLE route_daily_agg (day Date, platform LowCardinality(String), content_version LowCardinality(String),
  routes AggregateFunction(uniq, String), revenue SimpleAggregateFunction(sum, Int64),
  load_factor AggregateFunction(quantiles(0.5, 0.9), Float64))
ENGINE = AggregatingMergeTree ORDER BY (day, platform, content_version) TTL day + INTERVAL 2 YEAR;
CREATE MATERIALIZED VIEW route_daily_mv TO route_daily_agg AS
SELECT toDate(ts) AS day, platform, content_version, uniqState(route_id) AS routes,
  sum(amount_cents) AS revenue, quantilesState(0.5, 0.9)(value) AS load_factor
FROM events WHERE event_name = 'route_daily' GROUP BY day, platform, content_version;

-- event / investment outcomes
CREATE TABLE event_outcomes_agg (day Date, ref_id LowCardinality(String), choice LowCardinality(String),
  outcome LowCardinality(String), n SimpleAggregateFunction(sum, UInt64))
ENGINE = AggregatingMergeTree ORDER BY (day, ref_id, choice, outcome) TTL day + INTERVAL 2 YEAR;
CREATE MATERIALIZED VIEW event_outcomes_mv TO event_outcomes_agg AS
SELECT toDate(ts) AS day, ref_id, props['choice'] AS choice, props['outcome'] AS outcome, count() AS n
FROM events WHERE event_name = 'event_outcome' GROUP BY day, ref_id, choice, outcome;
```
- More rollups follow the same pattern: economy progression per game day (net-worth percentiles), retention cohorts, client performance by app version, ad funnel.
- **Rule: Grafana dashboards read only rollup tables**, never raw `events`. Raw events are for ad-hoc analysis within 90 days.
- **Sampling**: `client_perf` events are sent by 10% of sessions (the rate is a remote config value) and are already aggregated on the client. All gameplay and product events are 100%.

### Dashboards (Grafana)
- **Product**: DAU/MAU, D1/D7/D30 retention, session length, platform split, tutorial funnel.
- **Balance**: time to first route, net-worth curve by game day, bankruptcy rate and day, loan usage, chosen price vs. reference price, event accept rates and outcomes, most profitable models/routes.

Added events: `ai_action{airline, personality, action}` (sampled), `rival_ranked{rank}` (daily), `buyout_offer{from, to, iso, price, accepted}`, `takeover{acquirer, target, price}`, `game_ended{ending: bankrupt|sold}`, `ad_offered/shown/rewarded/failed{placement, reward}`, `interstitial_shown`, `mission_offered/accepted/completed/failed{template}`, `tutorial_step/skipped`, `hub_designated`, `landing_rights_leased/expired`, `seat_config_changed`, `competitor_price_war`, `restructured`, `flag_exposure{flag, variant}` (for A/B analysis).

### Privacy
- Consent on first launch uses **Google UMP** (the GDPR/CMP consent form AdMob requires) together with analytics consent.
- iOS shows the **App Tracking Transparency** prompt before personalised ads; if declined, ads are non-personalised.
- No email, name or IP in ClickHouse; `user_id` is pseudonymous.
- Account deletion → `DELETE FROM events WHERE user_id = ?` (lightweight delete).

## 13. Server API
```
POST /auth/guest | /auth/apple | /auth/google | /auth/email/start | /auth/email/verify
POST /auth/refresh            DELETE /account
GET  /saves                   GET /saves/{id} (meta + presigned download)   DELETE /saves/{id}
POST /saves/{id}/upload-url   PUT /saves/{id} (commit revision)
POST /telemetry               (batch, idempotent by event_id)
GET  /content/latest          (version check; bundle download later)
GET  /config?platform=&appVersion=&locale=   (remote config + flags, ETag-cached)
POST /ads/reward/verify       (AdMob server-side verification callback, later)
GET  /healthz
```
The server is stateless, so it scales horizontally.
- **Redis** handles rate limits: a token bucket per user/device/IP for `/telemetry`, `/auth/*` and `upload-url`. It also caches `/config`, with a 30 s TTL plus invalidation on update.
- **CDN caching**:
  - `/content/{version}` is immutable (`Cache-Control: max-age=31536000, immutable`).
  - `/config` is cached on the CDN for 60 s and varies by platform and app version. Per-user flag buckets are computed **on the client** from `hash(flagKey+userId)`, so the response can be shared and cached.

## 14. Frontend and mobile
- **Map rendering in layers**, all sharing one `d3-geo` projection and pan/zoom transform:
  1. **SVG base layer**: country shapes from **TopoJSON** (about 80% smaller than GeoJSON). Shapes are pre-projected and simplified by `datagen` into SVG path strings, so there's no projection work at runtime. Two detail levels: world zoom and regional zoom. The layer also has ownership fill plus a **pattern or outline** for owned/leased (colour-blind safe), airport dots. Changes rarely; used for click hit-testing.
  2. **Canvas overlay**: route arcs (great-circle) and **moving planes**, redrawn with `requestAnimationFrame`.
     - DPR-aware sizing.
     - Viewport culling.
     - Capped at 30 fps on mobile and in low-power mode; stops when paused and nothing is moving.
     - Planes are hit-tested through a quadtree.
  3. **HTML layer**: labels, tooltips and selection popups.
- **Layout**: responsive. Desktop has a side panel; mobile has a bottom sheet plus a tab bar (Map · Fleet · Routes · Finance · Offers). Touch targets ≥ 44 px.
- **Platform adapter** (`src/platform/`): storage, secure tokens, sign-in, haptics, lifecycle. The web implementation and Capacitor plugins share one interface.
- **Capacitor plugins**: App, Filesystem, Preferences, Haptics, Local Notifications (e.g. "Your loan payment is due" when you return), Sign in with Apple, Google Auth, Secure Storage.
- **Performance budget**: ≤ 300 map paths visible, `requestAnimationFrame` plane animation, simulation off the main thread.

## 15. Testing
- Simulation: formula unit tests, golden tests (seed + commands → state hash), and the balance bot over 5 game years.
- Content validator in CI: IDs, references, chances sum to 1, expressions compile.
- Server: integration tests against real Postgres/ClickHouse (testcontainers).
- Mobile: Playwright on the web build; occasional device testing.

## 16. Implementation plan
| # | Milestone | Done when |
|---|---|---|
| M0 | Monorepo scaffold, wasm build, Vite, Capacitor shell, docker-compose (PG, CH, Prometheus, Grafana), Sentry wiring, i18n skeleton, CI with determinism check | `make dev` runs the web app with a stub simulation |
| M1 | datagen: countries, about 1,000 airports, stats, localized names, seasons/holidays → content.json | content validates |
| M2 | Simulation core + WASM bridge + Worker + local saves | new game / tick / save / load in browser |
| M3 | Countries, landing rights, airports, planes, routes, flights, fuel | profit loop works |
| M4 | Map UI (SVG + canvas layers) + responsive panels | playable on web and phone browser |
| M5 | Seat classes, pricing per class, competitors + market share, forecast, seasons | price decisions matter |
| M6 | Hubs + connecting passengers, aging, overhaul, used market | network strategy |
| M7 | Effects engine, 8 events + 3 investments, missions | |
| M8 | Loans, credit, restructuring/bankruptcy, milestones, finance charts, tutorial | single-player loop complete |
| M8.5 | **AI airlines**: controller interface, utility AI, 5 personalities, difficulty, multi-airline markets, country buyouts, takeovers, rivals panel, news feed, AI-driven balance bot | **offline MVP complete** |
| M9 | Server: auth, cloud sync (object storage + presigned URLs), remote config + flags, Redis rate limits, CDN | cross-device play, flags live |
| M10 | Telemetry queue, RabbitMQ + ingestor, ClickHouse + rollups, Prometheus metrics + alerts, Grafana dashboards | metrics + alerts live |
| M11 | AdMob rewarded/interstitial, UMP/ATT consent, iOS/Android builds, beta | apps in testers' hands |

## 17. Ads
- `AdsService` interface in `src/platform/`: `isAvailable(placement)`, `showRewarded(placement) → result`, `showInterstitial()`.
  - Capacitor implementation: `@capacitor-community/admob`.
  - Web / offline implementation: a no-op stub (buttons hidden).
- **Rewards are applied through the simulation as commands** (`{type: "adReward", rewardId}`), so they become normal Effects/Actions and stay deterministic and replayable.
- Pacing rules come from remote config (`ads.interstitial.minIntervalSec`, `ads.rewarded.dailyCap`, placement on/off).
- Never during the tutorial, a decision prompt or the first session.
- Later: server-side verification of rewarded ads (`/ads/reward/verify`) if cheating matters (e.g. leaderboards).

## 18. Remote config and feature flags
- **Model** (Postgres):
```sql
CREATE TABLE config_params (key TEXT PRIMARY KEY, value JSONB NOT NULL, description TEXT, updated_at TIMESTAMPTZ DEFAULT now());
CREATE TABLE feature_flags (key TEXT PRIMARY KEY, enabled BOOLEAN NOT NULL DEFAULT false,
  variants JSONB,            -- e.g. [{"name":"A","weight":50},{"name":"B","weight":50}]
  rules JSONB,               -- targeting: platform, minAppVersion, locale, geo_country, user percentage
  sim_affecting BOOLEAN NOT NULL DEFAULT false, updated_at TIMESTAMPTZ DEFAULT now());
```
- **Assignment** is stable: `bucket = hash(flagKey + userId) % 100`, the same on every device.
- **Client**:
  - Bundled `defaults.json`, so the app works offline on first launch.
  - Fetches `/config` with an ETag on start and every 30 min; the cache is stored locally.
  - UI flags apply immediately.
  - **Sim-affecting** values (balance overrides, new mechanics) are applied only at a new game or day boundary, through a `setConfig` command, and are **recorded in the save** to keep determinism.
- Every exposure emits `flag_exposure`, so A/B results can be analysed in ClickHouse.
- **Admin**: SQL or a small protected admin API first; an admin UI later. A Redis cache can be added in front when traffic needs it.
- Also carries `minAppVersion` (force-update prompt) and `contentVersion` (download a new content bundle).

## 19. Localization
- **No hardcoded strings.** UI and content reference keys (`nameKey`, `textKey`).
  - The simulation emits notifications as `{key, params}`, never as finished text.
- Files: `web/src/i18n/{locale}.json` (UI) and `content/i18n/{locale}.json` (events, missions, tutorial). Both use ICU MessageFormat (plurals, gender, select).
- Numbers, currency (the in-game $), dates and the game calendar are formatted with `Intl.*` for the active locale.
- **Country and city names** use localized fields from Natural Earth and OurAirports, with English as the fallback.
- CI: missing-key check and an **en-XA pseudo-locale** build (longer strings, accents) to catch layout overflow.
- Layout uses logical CSS properties, so RTL (Arabic, Hebrew) is possible later.
- Launch locales: English first. More are added through a translation platform (e.g. Crowdin / Weblate) without code changes.

## 20. Crash and error reporting (Sentry)
- **App**: `@sentry/capacitor` covers JS errors, native crashes (iOS/Android), ANRs and performance traces (sampled).
- **Simulation (WASM)**: the bridge recovers Go panics and reports them to Sentry with context: `seed`, `tick`, `schemaVersion`, `contentVersion`, active flags, and the **last 200 commands**. Because the simulation is deterministic, the bug can be reproduced exactly from the last save plus those commands.
- **Server**: `sentry-go` middleware, request tracing, with panics captured.
- **Releases** are tagged `appVersion+contentVersion`. CI uploads source maps and dSYM/ProGuard mappings.
- **Privacy**: user ID only (pseudonymous), no email or IP; respects the consent choice; `beforeSend` scrubbing.
- Alerts on new issues and crash-free rate drops; the release health dashboard is used to gate store rollouts (staged rollout %).

## 21. Technical metrics (Prometheus + Grafana)
**Split of responsibilities**:
- Prometheus: system health. Low-cardinality time series, alerting.
- ClickHouse: per-user and per-event analytics.
- Sentry: errors and crashes.
- Grafana shows all three: Prometheus and ClickHouse as data sources, with links out to Sentry.

### Server instrumentation (`prometheus/client_golang`, `/metrics` on a separate internal port)
| Metric | Type | Labels |
|---|---|---|
| `http_requests_total` | counter | `route` (template, not raw path), `method`, `status` |
| `http_request_duration_seconds` | histogram | `route`, `method` |
| `auth_logins_total` | counter | `provider`, `result` |
| `save_sync_total` | counter | `result` (ok, conflict, error) |
| `save_size_bytes` | histogram | — |
| `telemetry_events_received_total` | counter | `platform` |
| `telemetry_events_dropped_total` | counter | `reason` |
| `telemetry_publish_duration_seconds` | histogram | — (includes publisher confirm) |
| `telemetry_publish_errors_total` | counter | `reason` |
| `ingestor_batch_size` | histogram | — |
| `ingestor_messages_total` | counter | `result` (inserted, requeued, dead_lettered) |
| `ingestor_lag_seconds` | gauge | — (now − oldest unacked received_at) |
| `ratelimit_rejected_total` | counter | `endpoint` |
| `save_upload_url_total` | counter | `result` |
| `clickhouse_insert_duration_seconds` | histogram | `table` |
| `clickhouse_insert_errors_total` | counter | `table` |
| `config_requests_total` | counter | `cache` (hit, 304, miss) |
| `db_pool_*` | gauges | via `pgx` pool stats collector |
| Go runtime | standard | `go_*`, `process_*` |

Rule: **no user IDs, save IDs or raw paths as labels**, to keep cardinality bounded.

### Infrastructure exporters
- RabbitMQ's built-in `rabbitmq_prometheus` plugin (port 15692): queue depth, unacked messages, publish/deliver rates, consumers
- `redis_exporter`
- MinIO/S3 metrics endpoint
- `postgres_exporter`
- ClickHouse's built-in Prometheus endpoint (`<prometheus>` config, port 9363)
- `node_exporter` / cAdvisor for host and containers
- Grafana's own metrics

### Client-side technical metrics
- Prometheus can't scrape phones. Clients send technical metrics in the same telemetry batch as analytics, as aggregated `client_perf` events: `fps_p50/p95`, `tick_duration_ms_p95`, `wasm_load_ms`, `save_duration_ms`, `memory_mb`.
  - These land in ClickHouse.
  - The server also aggregates them into Prometheus histograms labelled by `platform` and `app_version`, for live dashboards.
- Sentry Performance covers detailed traces.

### Dashboards (provisioned as code in `deploy/grafana/`)
1. **API overview**: RPS, error rate, p50/p95/p99 latency per route.
2. **Sync and auth**: sync conflicts, failures, login success by provider.
3. **Telemetry pipeline**: received → published → queue depth → inserted / requeued / DLQ, ingestor lag, ClickHouse insert latency.
4. **Databases**: Postgres connections, slow queries, replication; ClickHouse parts, merges, query time.
5. **Client performance**: fps, tick time and WASM load time by platform and app version.
6. **Infrastructure**: CPU, memory, disk, containers.

### Alerts (Alertmanager → e.g. email, Slack or Telegram)
| Alert | Condition |
|---|---|
| API error rate | 5xx > 2% for 5 min |
| API latency | p95 > 500 ms for 10 min |
| Telemetry backlog | `telemetry.events` depth growing for 15 min, or `ingestor_lag_seconds` > 300 |
| Ingestor down | no consumers on `telemetry.events` for 2 min |
| Dead letters | `telemetry.dlq` depth > 0 |
| RabbitMQ resources | memory or disk alarm raised |
| Publish failures | `telemetry_publish_errors_total` rate > 0 for 5 min |
| ClickHouse inserts failing | `clickhouse_insert_errors_total` rate > 0 for 5 min |
| Sync conflicts spike | conflict ratio > 10% (likely a client bug) |
| Postgres | connections > 80% of max, or exporter down |
| Target down | any `up == 0` for 2 min |
| Disk | < 15% free |

### Deployment
`deploy/docker-compose.yml` adds `rabbitmq`, `redis`, `minio`, `ingestor`, `prometheus`, `alertmanager`, `grafana`, `postgres_exporter`, `redis_exporter` and `node_exporter`. The Prometheus scrape config, alert rules and Grafana dashboards/data sources are all versioned in the repo.
