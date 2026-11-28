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
7. **Deterministic math only (`sim/detmath`)**:
   - `+ − × ÷` and `math.Sqrt` are IEEE-754 correctly rounded, so they are allowed directly.
   - **Every other function** (`Pow`, `Exp`, `Log`, `Tanh`, trig) comes from `sim/detmath`: pure-Go ports with every product wrapped in `float64()`, so no fused multiply-add on arm64, and no assembly versions.
   - `math.*` beyond `Sqrt`, `Floor`, `Abs`, `Min` and `Max` is banned in `sim/` by the vet analyzer. No calls into host JS math.
   - Integer powers (loan annuities) use repeated multiplication; `x^1.5` = `x·Sqrt(x)`.
   - Fixed exponents on static data (e.g. `distKm^0.8`, `pop^0.5`) are **precomputed in datagen** and stored in content.
8. **Normal random numbers** use Irwin–Hall (`Σ12 uniforms − 6`), with no `Log` or `Cos`.
9. **Rounding**: `detmath.RoundCents(x float64) int64` rounds half away from zero (`Floor(|x|+0.5)` with the sign restored). It is the only float → cents conversion.
10. **NaN/Inf guard**: `detmath.Finite(x, ctx)` is applied to every formula result. In tests and debug builds it panics. In release it clamps to 0 and reports to Sentry with the seed and tick.
11. **`expr-lang/expr`** is restricted to an allow-listed function set backed by `detmath`, and results that are money go through `RoundCents`.
12. **M0 gate**: CI compares `detmath` test vectors **bit for bit** on amd64, arm64 and wasm before any game formula is written.
**TickCtx** holds state, content, RNG, effects, `Notify()` and `Track(analyticsEvent)`.

### WASM bridge: serialization boundary
- **Commands and queries are JSON strings.** They are small, rare and easy to debug.
- **State traffic is MessagePack bytes.** Go copies the bytes into a JS `Uint8Array` with `js.CopyBytesToJS`, and the Worker **transfers** its `ArrayBuffer` to the UI thread without copying.
```
sim.newGame(json{airlineName, homeCountry, difficulty, seed}) → Uint8Array   // gzip(MessagePack) snapshot
sim.load(Uint8Array snapshot) → json{ok, error?}                              // on success, the next update is kind "init"
sim.save() → Uint8Array                                                       // gzip(MessagePack) snapshot → SaveStore
sim.command(json{type, ...}) → json{ok, errorKey?}
sim.advance(ticks) → Uint8Array                                               // MessagePack {delta, notifications, prompts, analytics[]}
sim.query(json{type: "routeForecast", routeId, priceEco, priceBiz}) → json{result}
```
Worker → UI messages carry an **uncompressed** MessagePack buffer, always sent with `transfer: [buf]`. The UI never needs a decompressor; gzip is used only between the simulation and `SaveStore` (ADR-0024).
- **`{kind: "init", buf}`** is sent after `newGame` or a successful `load`, and after any detected sequence gap. It holds the full UI **state view**: every entity the UI renders, without simulation-internal caches.
- **`{kind: "update", buf}`** is a delta envelope:
```
{ seq, tick,
  changed: { airlines?: {...}, countries?: {...}, airports?: {...}, planes?: {...}, routes?: {...},
             loans?: {...}, offers?: {...}, modifiers?: {...}, pending?: {...}, missions?: {...},
             markets?: {...}, landingRights?: {...} },              // ID → full entity (not field diffs)
  deleted: { planes?: ID[], routes?: ID[], loans?: ID[], offers?: ID[], modifiers?: ID[],
             pending?: ID[], missions?: ID[], landingRights?: ID[], airlines?: ID[] },  // tombstones
  flights:  [{planeId, from, to, departTick, arriveTick}],            // for client interpolation
  notifications: [...], prompts: [...], analytics: [...] }
```
  - Changed entities are sent whole, which keeps the UI merge trivial.
  - `seq` increases by 1 per message. If the UI sees a gap, it asks for a fresh `init`.
- **Request priority in the Worker**:
  - Commands and queries are handled **between ticks**, before the next tick runs.
  - `routeForecast` queries are **coalesced**: a newer query for the same route replaces the queued one, so only the latest slider value is evaluated.
  - **Autosave is deferred**: at a day boundary the Worker first sends the tick delta, then answers pending queries, and only then encodes and writes the save (in a later macrotask). A slider never waits behind save encoding.
- Build: standard Go `GOOS=js GOARCH=wasm` with `-trimpath -ldflags="-s -w"`, then `wasm-opt -Oz`. Served brotli-compressed from the CDN.
  - **Realistic budget**: ≤ 10 MB raw and **≤ 2.5 MB brotli** including `expr`, measured in M0 and enforced in CI.
  - Compile and instantiate in ≤ 1.5 s on a low-end Android WebView, also measured in M0.
  - Loaded with `WebAssembly.instantiateStreaming` behind a branded loading screen; cached by the service worker on web and bundled inside the mobile apps.
  - Fallback if size is still a problem: TinyGo, which needs checking against `expr`.
- **Worker → UI traffic**: the simulation can run at 4x, but the UI gets at most **~4 updates per second**.
  - Each update is a delta envelope with changed entities and tombstones (see above), sent as a transferable `ArrayBuffer` (MessagePack).
  - Plane positions are **not** streamed. The UI receives `{planeId, depart, arrive}` once per flight and interpolates locally.
- Content is compiled to JSON, so no YAML parser is needed in WASM.
- **GC and allocations**: Go WASM has a stop-the-world GC, so the hot paths are allocation-free:
  - Flights, demand allocation and AI candidate scoring use preallocated, reused slices in `TickCtx` and `DecideCtx`, and avoid maps inside the hot loops.
  - Benchmarks assert `allocs/op = 0` for those paths.
  - `debug.SetGCPercent` is tuned after profiling.
  - Target: day-boundary work (Demand + AI + Economy) ≤ 16 ms p95 on a mid-range phone, so no visible stutter.

## 4. State model
The **player is one airline among several**. Every system loops over all airlines, sorted by ID. Ownership lives on the world entities.
```go
type State struct {
  Version int; Seed uint64; Tick int64; Speed int
  Difficulty Difficulty                       // easy | normal | hard (AI count, skill)
  Airlines  map[AirlineID]*Airline            // player = "P", AI = "A1".."A6"
  Countries map[string]*Country               // ISO2 → ownerID ("" = unowned), purchasedTick
  Airports  map[string]*Airport               // IATA → baseLevel (real-world, from datagen), level, hubOf; owner = Countries[iso].ownerID
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
**Implementation notes (M2)**: the code is in `sim/state/state.go`.
- Entity IDs come from a sequential counter (`State.NewID("pl")` → `pl1`, `pl2`, …), so the same commands always produce the same IDs.
- `Airports` holds only airports whose state differs from content; an absent airport runs at its base level. `Countries` holds only owned countries.
- `state.SortedKeys` is the one way to walk a map in game logic.
- A new game (`sim.NewGame`) validates the airline name (1–40 characters), the home country (it must have an airport) and the difficulty. It then gives the player the home country, its **main airport as hub** (largest city served, then longest runway), one starter plane (the cheapest lowest-tier model), and `StartCash` from §7.0.

**Airport levels and access**:
- Every airport has a real-world `baseLevel` from datagen: OurAirports `large_airport` = 3, `medium_airport` = 2.
- An airport in an **unowned** country, or one leased by non-owners, runs at `level = baseLevel`.
- **Only the country owner can upgrade above `baseLevel`.** The level never drops below `baseLevel`, even after a buyout or restructuring.
- **Lessees use the airport's current level** (capacity, max plane tier), but are limited by their `departuresCap`.
- When passenger capacity is short, the owner's passengers are allocated first, then lessees pro rata by scheduled seats.

## 5. Content (data-driven)
- `planes.yaml`, `airport_levels.yaml`, `balance.yaml`
- `events/*.yaml`: one schema for incidents and investments. Each event has choices; each choice has a cost and outcomes; each outcome has a chance, effects and an optional delayed reveal.
- Conditions and formulas use `expr-lang/expr`.
- **Pipeline** (M1): YAML in `content/` (`planes.yaml`, `airport_levels.yaml`, `balance.yaml`, `seasons.yaml`, `holidays.yaml`) is loaded **strictly** by `tools/contentc` (unknown or misspelled fields are errors) and validated by `sim/content.Validate` against the world data (country and region references) and `content/i18n/en.json` (name keys). `sim/content` has the Go types and a strict JSON `Decode`; it has no YAML dependency, because the simulation only reads compiled JSON. `make content-check` runs `datagen check` and `contentc verify`.
- **Bundle** (M1.8): `make content-build` compiles the content and the simulation's subset of `content/world` (countries with stats, airports, cities, candidates; no names or map paths) into `sim/content/bundle/content.json.gz` (≈ 330 KB gzip, 1 MB JSON). It is **embedded** with `go:embed` in `sim.wasm` and the server, and **committed** so builds stay hermetic; `contentc verify` fails CI if it is stale. `contentVersion` = `"c"` + 12 hex digits of the SHA-256 of the content and world JSON, so it changes exactly when content changes. gzip output carries no timestamp, so builds are byte-identical. The bridge returns `contentVersion` in its view, and the server reports it at `GET /content/latest`.
- Each content build has a `contentVersion`, which is stored in saves and analytics.
- Later: the server can serve newer content bundles, so new events ship without an app release.

## 6. Effects / modifiers
- A **Modifier** is `{id, source, scope, target, stat, op(add|mul|set), value, priority, expiresTick}`. Scopes: `global`, `country`, `airport`, `route`, `plane`, `airline`.
- An **Action** is an instant effect: cash, reputation, damage, give plane.

### Stacking order (normative)
For a stat on a target, collect the active modifiers whose scope matches the target, either directly or via its parents (plane → route → airport → country → airline → global).
1. **Sort** by `(scopeRank, priority, expiresTick, id)`, with scopeRank `global < airline < country < airport < route < plane`. Float addition and multiplication don't commute exactly, so this fixed order is required for determinism.
2. **Same-source rule**: only one modifier per `(source, stat, target)` is active. Re-applying refreshes `expiresTick` and value; it doesn't stack.
3. **Evaluate**: `v = (base + Σ add) × Π mul`, in the sorted order.
4. **Clamp**: `v = clamp(v, stat.min, stat.max)`, using per-stat bounds from the stat registry (`sim/effects/stats.go`, e.g. demand multiplier ∈ [0.2, 3.0]). This prevents runaway compounding.
5. **Set overrides**: if any `set` modifiers exist, the **last one in sorted order** wins and replaces `v`. It's used for `airportClosed`, `routeSuspended` and similar flags.

Modifiers are pruned at the first tick ≥ `expiresTick`.

## 7. Formulas
These are starting values; all coefficients live in `content/balance.yaml`. Money is in cents (`RoundCents` once per result). `^` uses the rules in §3 determinism, rule 7.

### 7.0 Base economy (restored from rev 1, updated)
| Thing | Formula / rule |
|---|---|
| Country price | `k_country · pop^0.5 · gdpPc^0.7`, with the exponents precomputed in datagen as `countryPriceBase` |
| Country upkeep | daily `0.0005 · countryPrice` (owning idle countries costs money) |
| Start cash | `startBase · (maxCountryPrice / homePrice)^0.3`, clamped to `[startMin, startMax]`; `^0.3` via `detmath.Pow` once at new game |
| O–D base demand / day | `D_OD = d0 · popFactor_A · popFactor_B / distFactor_AB · wealthAvg · airportSizeFactor · mods`, where `popFactor = cityPop^0.5` and `distFactor = distKm^0.8` (both from datagen; for pairs outside the precomputed set, `distFactor` comes from `detmath.Pow` and is cached in `Markets`) |
| Wealth | `wealth = gdpPc / worldMedianGdpPc`, clamped to [0.3, 3]; `wealthAvg = (wealth_A + wealth_B)/2` |
| Reference price (economy) | `ref_eco = (25 + 0.09 · km) · wealthAvg` dollars → cents |
| Price factor | `x = ref/price`; economy `clamp(x·√x, 0.1, 2.0)` (elasticity 1.5); business `clamp(detmath.Pow(x, 0.8), 0.1, 2.0)` |
| Service factor | `0.7 + 0.3·avgCondition + 0.05·avgPlaneLevel + 0.03·airportLevel` (business: the non-constant part ×2) |
| Reputation factor | `0.5 + reputation/100` (reputation 0–100, starts at 50) |
| Attractiveness | `A = priceF · serviceF · repF · mods` |
| Flight time | `ceil((km / speedKmh) · 60)` minutes, rounded up to whole ticks; turnaround 1 tick; a round trip is 2 legs |
| Fuel cost / leg | `km · burnPerKm · (1 − effBonus(level)) · fuelPrice` |
| Airport fees / leg | `feePerPax(level) · pax + feePerLanding(level)`; at leased airports ×1.5; the owner gets `ownerFeeShare` (30%) of **lessees'** fees, only if it has ≥ 1 own departure from that airport in the last 7 days |
| Flight outcome | `pSuccess = relBase(model) · (0.6 + 0.4·condition) · mods`. On failure: 70% delay (revenue ×0.5 for compensation, reputation −0.2); 30% cancel (no revenue, reputation −1, see §7.4) |
| Reputation drift | daily: `+0.02` per on-time-rate point above 90%, decaying toward 50 by 0.5% per day |
| Condition decay | per flight hour `−0.0002 · (1 + ageDays/maxAgeDays)`; overhaul restores to 1.0 (cost `0.08·price`, 5 days downtime) |
| Maintenance / day | `upkeepDay · (1 + ageDays/maxAgeDays) · (2 − condition)` |
| Resale value | `price · (1 + 0.1·level) · depFactor · condition`, where `depFactor` is multiplied **daily** by the constant `dailyDep = 0.88^(1/365)` (stored as a literal in content) |
| Fuel price (daily) | `p' = p + θ(μ − p) + σ·μ·Z + shock`, θ = 0.05, σ = 0.02, `Z` = Irwin–Hall normal; shock chance 1%/day, size ±U(20%, 60%)·μ, decaying linearly over 20 days; `p` clamped to `[0.4μ, 3μ]` |
| Loan rate (annual) | `baseRate + spread(creditScore)`: ≥ 800 → +1%; 650–799 → +3%; 500–649 → +6%; 350–499 → +10%; < 350 → no new loans |
| Loan instalment (weekly) | annuity `P·r / (1 − (1+r)^−n)`, `r = annual/52`, `(1+r)^n` by repeated multiplication |
| Overdraft | negative cash accrues 25%/year, charged daily |
| Credit score (0–1000) | `100 + 350·equityRatio + 250·min(coverage, 3)/3 + max(0, 200 − 60·missed365) + 100·cashTrend`, with guarded denominators: `equityRatio = assets ≤ 0 ? 0 : clamp((assets − debt)/assets, 0, 1)`; `coverage = debtService90d == 0 ? (opProfit90d ≥ 0 ? 3 : 0) : opProfit90d / debtService90d`; `cashTrend = clamp(Δcash30d / max(|cash|, dailyRevenue30d, 100_00) + 0.5, 0, 1)` (all values in cents) |
| Net worth | `cash + Σ resale(planes) + Σ countryPrice(owned)·0.8 + Σ airport upgrade costs·0.5 − debt` |

### 7.1 New mechanics (formulas)
| Thing | Formula / rule |
|---|---|
| Season factor | `season[country][month] × Π holiday[country][date]` (generated as modifiers by the Season system from `seasons.yaml` and `holidays.yaml`) |
| Class demand | `D_biz = D · bizShare(gdp, businessProfile) · seasonBiz`; `D_eco = D − D_biz` |
| Class reference price | `ref_eco` as before; `ref_biz = 3.5 · ref_eco` |
| Class elasticity | economy 1.5, business 0.8; business service weight ×2 |
| Market share (per class) | `A_you / (A_you + Σ A_comp_i)`; with no competitors: `min(1, A_you)` |
| Competitor price | daily: `p += 0.1(ref·(1+noise) − p)`; if your share > 60% for 7 days: undercut 5–10% (chance) |
| Direct pax per day | `min(D_class · share, seats_class_per_day)` (see §7.4 for how these are spread over flights) |
| Connecting pax | for each hub H, pairs of legs (O→H, H→D) with no direct O–D by that airline: `D_OD · 0.4 · min(share_leg1, share_leg2)`, limited to the leftover seats on both legs |
| Connecting fare | `(fare_leg1 + fare_leg2) · 0.85`, split between legs by distance ratio for revenue accounting |
| Landing rights | upfront `0.05–0.15 · countryPrice · sizeFactor`, yearly lease `0.3 · upfront`, fees ×1.5, departures/day cap by airport size |
| Seat config | `eco + 2·biz ≤ model.seatUnits`; reconfiguring costs money and takes 48 ticks of downtime |

### 7.4 From daily demand to hourly flights
Demand is calculated per **day**, while flights depart per **tick**. The two are linked like this:
1. **Day start** (Demand system `OnDay`): for every market, class, airline and **direction**, compute today's **direct pool**: `min(D_class · share, scheduledSeats_class_today)`.
   - Each route has **two directional pools per day** (`from → to` and `to → from`). Planes alternate direction after each leg plus 1 turnaround tick, and the O–D demand splits 50/50 between the two directions.
   - The day is defined by the **departure tick**.
2. **Connecting itineraries are booked at day start**, after the direct pools.
   - Leftover per leg and class = scheduled seats − direct pool.
   - **Contention at a hub**: many connecting pairs `(S_i → H → S_j)` share the same leg capacity. All candidate pairs of an airline at a hub are processed in one pass:
     1. Sort by **connecting fare descending**, then `(originIATA, destIATA)`. This is realistic yield management and fully deterministic.
     2. Reserve greedily: `conn = min(connDemand, leftoverLeg1, leftoverLeg2)`, then subtract it from both legs.
     3. **Fairness cap**: no single pair may take more than 50% of a leg's leftover in the first pass. A second pass (same order) fills whatever remains.
   - The seats are **reserved on both legs**. The second leg's reservation belongs to the first departure of the second leg that is at least 1 tick after the first leg arrives, on that same day or the next.
3. **Each departure** (Flights `OnTick`) does **all of its accounting at the departure tick**:
   - The reliability roll (ok / delay / cancel), boarding, revenue, fuel and fees, and the `route_daily` counters are all booked on the **departure day**. A long-haul flight leaving at 20:00 on day *d* counts fully for day *d*.
   - The plane is then locked in `flying` until `departTick + flightTicks + delayTicks`, plus 1 turnaround tick. The UI animates it across those ticks from the `flights` entries.
   - Boarding from the direction's remaining pool:
   - `boarded = min(seatsLeft, ceil(poolRemaining / departuresRemainingToday))`. This spreads passengers evenly over the day's departures.
   - Direct passengers board first, then the reserved connecting passengers for that leg.
   - Pools are integers, and leftovers go to the last departure of the day.
4. **A cancelled flight** loses its passengers. Up to 50% are rebooked onto later departures of the same airline and route that day, if seats are free. Rebooked connecting passengers keep their second-leg reservation; otherwise both legs' reservations are released.
5. **Day end**: unused pools expire. Nothing carries over to the next day.
6. Load factor, revenue and `route_daily` analytics are summed per day from the actual boardings.

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
- **Timing**: the AI system runs **before Offers** in the tick. Its commands carry `issuedTick` and are applied at the next tick boundary.
  - The Offers system accepts a response if `issuedTick ≤ expiresTick`, so the 1-tick latency can never make an AI miss an offer.
  - Every offer lasts at least 24 ticks.
  - Urgent cash triggers act 1 tick later, which is acceptable because liquidation needs 14 days.
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
- Each country has at most **one owner**. Other airlines reach its airports only through landing rights, which the owner can't block.
- The owner earns a share of **lessees'** landing fees, but **only at airports where it has run its own departures in the last 7 days**.
- Every owned country costs a **daily upkeep** (§7.0).
- Together, these stop the "toll booth" strategy of buying many cheap micro-states just for fee income.
- **Buyout offers** (any airline → owner): `offer ≥ countryValue × (1 + premium)` is accepted if:
  - `premium` depends on how important the country is to the owner: home = never sold; hub country ×1.5; otherwise 0.2–0.5, scaled by personality.
  - Financial distress lowers the premium: DISTRESS −50%, LIQUIDATION accepts any offer ≥ 0.8 × value.
  - The AI answers immediately. The player gets a prompt (an `Offer` that expires after 7 days).
- On transfer, the airports and their levels go to the buyer. The seller's existing routes there become **landing rights for 1 year** (grace period).

### Takeovers (both ways)
- **Airline valuation** = net worth + goodwill, where goodwill = 3 × average monthly route profit (last 90 days) × (reputation / 50).
- An offer is accepted when `price ≥ valuation × (1 + takeoverPremium)`. The premium is lower when the target is in DISTRESS or LIQUIDATION, or is losing money.
- **The player acquires an AI**: all of its assets, routes and debt merge into the player's airline; its brand is retired. A monopoly check blocks **any** takeover (player or AI, of any target) where the combined airline would hold **> 50% of world capacity**. World capacity is the seats per day of all real airlines; background carriers are excluded.
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
- Snapshot = **gzip(MessagePack state)** behind a `JTSV` header (ADR-0009, ADR-0024; zstd was dropped for WASM size). A JSON export exists for debugging and bug reports.
  - Each snapshot also has metadata `{saveId, revision, tick, schemaVersion, contentVersion, deviceId, updatedAt, summary{cash, netWorth, day}}`.
- Autosave: every game day, on pause, and when the app goes to the background. Keep the last 3 rolling copies (corruption safety).
- Old saves are upgraded on load by versioned Go migrator funcs inside the simulation.

## 10. Accounts and auth (server)
- **Guest**: on first launch the device generates `deviceId` and `localUserId`, and the game is fully playable offline. The first time it goes online, it registers `POST /auth/guest` → anonymous user + tokens.
- **Link**: Sign in with Apple (required on iOS if Google is offered), Google, or email + magic link.
  - **Identity is new**: it is attached to the current guest user.
  - **Identity already belongs to another user** (guest U1 on device B signs in with an Apple ID owned by U2): **merge U1 into U2** in one transaction:
    - Move U1's `saves`, `devices` and `refresh_tokens` to U2. Save IDs are UUIDv7, so they never collide, and blob keys are per save (§11), so no objects move.
    - Set `users.merged_into = U2` and `deleted_at` on U1. Revoke U1's tokens and issue tokens for U2.
    - Insert `user_aliases(alias=U1, canonical=U2)` and emit `account_merged{from, to}`. ClickHouse analytics resolve aliases through a dictionary.
    - The device then lists U2's saves. Both airlines appear, and the player picks which to continue. No save is overwritten.
  - Signing in on a new device pulls the cloud saves.
- **Tokens**: JWT access token (15 min) + rotating refresh token (stored hashed). Native tokens go in Keychain/Keystore via Capacitor Secure Storage.
- **Account deletion** (`DELETE /account`, an App Store requirement) creates a `deletion_jobs` row (an outbox) and returns 202. A worker then completes these steps, retrying until all succeed:
  1. **Object storage**: delete every object under `saves/{save_id}/` for each save of the user **and its merged aliases**.
  2. **ClickHouse**: `DELETE FROM events WHERE user_id IN (user, aliases)` (lightweight delete).
  3. **Sentry**: delete the user's data through the Sentry data-deletion API.
  4. **Postgres**: delete the user rows (cascading to identities, devices, tokens, saves) and the alias rows.
  - The job is complete only when every step has succeeded. Completion is tracked per step.

```sql
-- migrations/postgres/0001_init.up.sql
CREATE TABLE users (id UUID PRIMARY KEY, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), deleted_at TIMESTAMPTZ,
  merged_into UUID REFERENCES users(id));
CREATE TABLE user_aliases (alias UUID PRIMARY KEY, canonical UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  merged_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE deletion_jobs (id UUID PRIMARY KEY, user_id UUID NOT NULL, requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  storage_done BOOLEAN NOT NULL DEFAULT false, clickhouse_done BOOLEAN NOT NULL DEFAULT false,
  sentry_done BOOLEAN NOT NULL DEFAULT false, postgres_done BOOLEAN NOT NULL DEFAULT false,
  attempts INT NOT NULL DEFAULT 0, last_error TEXT);
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
  blob_key TEXT NOT NULL,                              -- object storage key: saves/{save_id}/{revision}
  blob_size INT NOT NULL, blob_sha256 BYTEA NOT NULL,  -- integrity + "unchanged" detection
  device_id UUID, updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE INDEX ON saves (user_id);
```
Save files live in **object storage**, not Postgres, which keeps the `saves` rows small and avoids TOAST bloat from frequent rewrites. **Retention of old revisions is done by the server, not by S3**:
- S3/MinIO lifecycle rules work by object age or noncurrent versions only. They can't keep "the highest 3 revisions" under a prefix.
- After a commit of revision *R*, the server enqueues deletion of `saves/{save_id}/{R−3}` (and any lower revision still present) in a `blob_gc` table processed by a background worker. So the last **3 committed revisions** are kept.
- **Uploads that are never committed** (upload-url issued and blob uploaded, but no `PUT /saves/{id}`) go to a staging prefix `staging/{save_id}/{uuid}`. A **lifecycle rule expires `staging/` after 24 h**. On commit, the server copies the staged object to `saves/{save_id}/{R}` (a server-side copy) and deletes the staged one.
```sql
CREATE TABLE blob_gc (key TEXT PRIMARY KEY, enqueued_at TIMESTAMPTZ NOT NULL DEFAULT now(), attempts INT NOT NULL DEFAULT 0);
CREATE TABLE save_quotas (user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE, max_saves INT NOT NULL DEFAULT 10);
```
```sql
```

## 11. Cloud save sync
Single player, so the protocol is simple and conflicts are explicit:
- **Skip when unchanged**: the client compares the snapshot's sha256 with the last synced hash and uploads nothing if they match.
- Upload in **3 steps**, so save bytes never pass through the API:
  1. `POST /saves/{id}/upload-url {baseRevision, sha256, size}` returns a presigned PUT URL for `staging/{save_id}/{uuid}`. It is rejected early with 409 if `baseRevision` is stale, 413 if `size > maxSaveSizeBytes`, or 429 if the user is over quota. Details below.
  2. The client PUTs the blob directly to object storage, with headers `Content-Length: size` and `x-amz-checksum-sha256: base64(sha256)`.
  3. `PUT /saves/{id}` commits `{baseRevision, uploadId, sha256, summary}`. This is a conditional update (`WHERE revision = baseRevision`). The server checks the staged object (HEAD: size and checksum) and copies it to `saves/{save_id}/{R}`.
  - If the revision matches: `revision+1`, and the new revision is returned.
  - Otherwise: return **409** with the server's metadata. The client shows "This device: Day 210, $45M / Cloud: Day 180, $30M — keep which?"
- **Abuse protection**:
  - **Size cap**: `size ≤ maxSaveSizeBytes` (remote config, default **4 MB**; typical saves are 50–500 KB).
  - **Signed headers**: the presigned `PutObject` **signs `Content-Length` and `ChecksumSHA256`** (`x-amz-checksum-sha256`), so the storage layer itself rejects uploads with a different size or hash. A client can't claim 50 KB and upload gigabytes.
  - **Short-lived URLs**: they expire after 5 minutes.
  - **Rate limit**: `upload-url` is limited per user in Redis (e.g. 30 per hour).
  - **Quotas**:
    - At most **10 saves per user** (`save_quotas.max_saves`, adjustable). Creating an 11th returns 429 `quota.saves_exceeded`, and the UI offers to delete an old save.
    - At most 3 revisions are retained per save (above).
    - Storage per user is therefore bounded at roughly 10 × 3 × 4 MB.
- **First upload of a new save** (`baseRevision = 0`):
  - Save IDs are client-generated **UUIDv7**.
  - `upload-url` checks that no row exists for the ID, or that one exists for the same user with the same sha256 (a retry).
  - The commit is an `INSERT ... ON CONFLICT (id) DO NOTHING RETURNING revision`, which creates revision 1. If no row is returned:
    - Existing row owned by the same user with the same sha256 and revision 1 → **200** (idempotent retry).
    - Owned by the same user but different → **409**.
    - Owned by another user → **403**.
- Every save operation checks `saves.user_id = caller`, and returns 403 otherwise.
- `GET /saves` lists metadata. `GET /saves/{id}` returns metadata + a presigned download URL.
- Sync triggers: app start, every 5 min of play while online, app background, manual "Sync now".
- Later (anti-cheat, for leaderboards): upload the command log; the server replays it in the same Go simulation to verify.

## 12. Analytics → ClickHouse
### Flow
1. The simulation emits gameplay events via `Track()`; the UI emits product events.
2. Both go into a durable **TelemetryQueue** on the device (IndexedDB / file), so nothing is lost offline.
3. When online, the client sends batches: `POST /telemetry` (≤500 events, gzip). Each event has a client-generated `event_id`.
4. **Idempotent retries**:
   - Each client batch has a stable `batch_id` (UUIDv7). The batch's composition is persisted in the local queue, so a retry sends the identical batch.
   - The API runs `SET NX tlm:{user_id}:{batch_id}` in Redis (TTL 48 h). If the key already exists, it returns **202 without publishing**. If the publish fails, it deletes the key.
   - The ingestor also skips messages whose `batch_id` is in `ing:{batch_id}` (set after a successful insert, TTL 48 h). This covers requeues that rebuild ClickHouse batches with a different composition (and so a different dedup token).
   - Both guards matter because materialized views count rows **at insert time**, before ReplacingMergeTree merges, so `sum()` rollups would double-count duplicates.
   - Remaining risk: a crash between the ClickHouse insert and setting the Redis key. It's rare and accepted; exact figures use `uniqExact(event_id)` on raw events.
5. The API checks the Redis rate limit, then validates and enriches each event (user_id, geo country from IP, received_at, then drops the IP).
6. The API publishes the batch to **RabbitMQ** (`telemetry.events`, a durable **quorum queue**, persistent messages, **publisher confirms**). It returns **202 only after the confirm**, so the client deletes its local copy only when the events are safe.
7. The **ingestor** service (`cmd/ingestor`) consumes with prefetch about 10k:
   - It builds one ClickHouse batch per ~5 s or 10k events (clickhouse-go v2 native batch), using `insert_deduplication_token` = hash of the message IDs.
   - It acks the messages (`multiple=true`) **only after** ClickHouse accepts the insert.
   - On a failed insert: nack + requeue, with exponential backoff.
   - Malformed messages go to the dead-letter exchange → `telemetry.dlq`, where they can be inspected and replayed.
8. Delivery is **at-least-once**. Duplicates are removed by `event_id` (ReplacingMergeTree) plus the insert dedup token (`non_replicated_deduplication_window` on single-node ClickHouse).
9. If ClickHouse is down, events wait in RabbitMQ, which has queue length and disk limits with alerting. If RabbitMQ is down, the API returns 503 and clients keep events in their local queue and retry later. **No data loss in either case.**
10. The API and ingestor scale separately. Publishing sits behind a `Sink` interface, which has an in-memory implementation for tests and local dev.

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
- Account deletion → `DELETE FROM events WHERE user_id IN (user, aliases)` (lightweight delete), run by the deletion job (§10).
- **Account merges**: a `user_aliases` ReplacingMergeTree table, synced from Postgres, backs a dictionary. Retention and funnel queries use `dictGetOrDefault('user_alias', 'canonical', user_id, user_id)`.

## 13. Server API
```
POST /auth/guest | /auth/apple | /auth/google | /auth/email/start | /auth/email/verify
POST /auth/refresh            DELETE /account
GET  /saves                   GET /saves/{id} (meta + presigned download)   DELETE /saves/{id}
POST /saves/{id}/upload-url   (size cap, signed Content-Length + SHA-256, 5 min, quota)   PUT /saves/{id} (commit revision)
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
  1. **SVG base layer**: country shapes **pre-projected by `datagen`** (Natural Earth I projection, 1000-unit-wide viewBox) into compact SVG path strings with relative coordinates rounded to 0.1 unit (`content/world/map/world-110m.json` ≈ 27 KB gzip, `world-50m.json` ≈ 134 KB gzip). Because paths are pre-projected, TopoJSON and runtime projection are not needed. Two detail levels: world zoom and regional zoom. The layer also has ownership fill plus a **pattern or outline** for owned/leased (colour-blind safe), airport dots. Changes rarely; used for click hit-testing.
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
