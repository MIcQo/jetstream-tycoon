# Jetstream Tycoon — Diagrams

## 1. System architecture
Every device runs the game simulation itself. The server only handles accounts, save sync, analytics and config.

```mermaid
flowchart LR
  subgraph Device["Device: Web / iOS / Android (Capacitor)"]
    UI["UI thread<br/>Preact panels, SVG map, Canvas planes"]
    W["Web Worker<br/>sim.wasm (Go)"]
    SS[("SaveStore<br/>gzip MessagePack")]
    TQ[("TelemetryQueue<br/>durable, offline-safe")]
    CC[("Config cache<br/>bundled defaults")]
    ADS["AdsService<br/>AdMob plugin"]
    SEN["Sentry SDK"]
    UI -- commands --> W
    W -- "deltas max 4/s" --> UI
    W -- snapshots --> SS
    W -- gameplay events --> TQ
    UI -- "product + perf events" --> TQ
    CC -- "UI flags" --> UI
    CC -- "setConfig cmd" --> W
    ADS -- "adReward cmd" --> W
  end

  CDN["CDN<br/>web app, sim.wasm, content, config"]

  subgraph Server["Go API server (stateless)"]
    AUTH["auth"]
    SAVES["saves metadata + presign"]
    TEL["telemetry endpoint"]
    CFG["config + content"]
    MET["metrics endpoint"]
  end

  CC <-- "ETag fetch" --> CDN
  CDN --> CFG
  UI <-- "sign-in, tokens" --> AUTH
  SS <-- "revision check, commit" --> SAVES
  SS <-- "presigned PUT / GET" --> OBJ[("Object storage<br/>save files")]
  TQ -- "gzip batches" --> TEL

  AUTH --> PG[("Postgres<br/>accounts, save meta, flags")]
  SAVES --> PG
  CFG --> PG
  AUTH --> RD[("Redis<br/>rate limits, config cache")]
  TEL --> RD
  CFG --> RD

  TEL -- "publish + confirm" --> MQ[["RabbitMQ<br/>telemetry.events"]]
  MQ -- "prefetch 10k" --> ING["Ingestor (Go)"]
  ING -- "batch insert, then ack" --> CH[("ClickHouse<br/>raw 90d, rollups 2y")]
  MQ -. "bad messages" .-> DLQ[["telemetry.dlq"]]

  PROM["Prometheus"] -- scrape --> MET
  PROM -- scrape --> ING
  PROM -- scrape --> EXP["Exporters<br/>postgres, redis, rabbitmq, node, ClickHouse"]
  PROM --> AM["Alertmanager"]
  GRAF["Grafana"] --> PROM
  GRAF -- "rollups only" --> CH

  SEN --> SENTRY["Sentry"]
  MET -.-> SENTRY
  AUTH -.-> IDP["Apple / Google / Email"]
  ADS -.-> ADMOB["AdMob"]
```

## 2. Simulation tick pipeline
Commands from the player and the AI airlines are applied at the start of a tick, and both go through the same validation. Then the systems run in a fixed order. Each system has its own RNG stream, and each AI airline has its own too.

```mermaid
flowchart LR
  CMD["Command queue<br/>player + AI"] --> T{{"Tick = 1 game hour"}}
  T --> S1[Season] --> S2[Fuel] --> AI["AI controllers<br/>utility AI, staggered"]
  AI --> S3[Competition] --> S4[Demand]
  S4 --> S5[Flights] --> S6[Aging] --> S7[Economy] --> S8[Leases]
  S8 --> S9[Loans] --> OF[Offers] --> S10[Events] --> S11[Missions] --> S12[Tutorial]
  S12 --> S13[Milestones] --> S14[Analytics]
  AI -. "commands for next tick" .-> CMD
  FX[("Effects engine<br/>modifiers")] -.-> S4
  FX -.-> S5
  FX -.-> S7
  S14 --> OUT["delta, notifications, prompts, analytics"]
```

## 3. Cloud save sync
Revisions use optimistic locking. Save files go straight to object storage through presigned URLs, which sign the size and checksum, into a staging prefix that expires after 24 h. The server keeps the last 3 committed revisions. Nothing is uploaded if the save hasn't changed.

```mermaid
sequenceDiagram
  participant D as Device
  participant API as API server
  participant PG as Postgres
  participant OBJ as Object storage
  D->>D: sha256(snapshot) equals last synced? then skip
  Note over D,PG: First upload: baseRevision=0, commit = INSERT ON CONFLICT DO NOTHING (identical retry 200, else 409/403)
  D->>API: POST /saves/{id}/upload-url (baseRevision=7, sha256, size)
  API->>PG: SELECT revision WHERE id=? (+ quota, size cap, rate limit)
  alt revision is 7
    API-->>D: presigned PUT to staging/{save_id}/{uuid}, signs Content-Length + SHA-256, 5 min
    D->>OBJ: PUT blob with signed headers (mismatch rejected by storage)
    D->>API: PUT /saves/{id} commit (baseRevision=7, uploadId, sha256, summary)
    API->>OBJ: HEAD staged object, copy to saves/{save_id}/8
    API->>PG: UPDATE saves ... WHERE id=? AND revision=7
    alt still 7
      API->>PG: enqueue blob_gc saves/{save_id}/5
      API-->>D: 200 revision=8
    else changed meanwhile
      API-->>D: 409 server summary
    end
  else another device saved first
    API-->>D: 409 server summary (day, cash)
    D->>D: Ask player: keep this device or cloud
    D->>API: GET /saves/{id} for presigned download, or retry with server revision
  end
```

## 4. Telemetry pipeline
```mermaid
sequenceDiagram
  participant S as sim.wasm / UI
  participant Q as TelemetryQueue
  participant API as POST /telemetry
  participant MQ as RabbitMQ
  participant I as Ingestor
  participant CH as ClickHouse
  S->>Q: Track(event with event_id)
  Note over Q: stored locally, survives offline
  Q->>API: batch of up to 500, gzip
  API->>API: Redis SET NX batch_id (duplicate: 202, skip)
  API->>API: rate limit (Redis), validate, enrich, drop IP
  API->>MQ: publish persistent message
  MQ-->>API: publisher confirm
  API-->>Q: 202 accepted, delete sent events
  Note over API,Q: RabbitMQ down means 503, client keeps events and retries
  MQ->>I: deliver (prefetch 10k)
  I->>I: skip batch_id already inserted (Redis)
  I->>CH: INSERT batch every 5 s or 10k (dedup token)
  alt insert ok
    CH-->>I: ok
    I->>I: mark batch_ids inserted (Redis)
    I->>MQ: ack multiple
  else ClickHouse down
    I->>MQ: nack + requeue, backoff
  end
  Note over CH: event_id dedup via ReplacingMergeTree
```

## 5. Airline financial status
```mermaid
stateDiagram-v2
  [*] --> OK
  OK --> WARNING: cash below 0
  WARNING --> OK: cash recovered
  WARNING --> DISTRESS: missed loan payment
  DISTRESS --> OK: debts current
  DISTRESS --> LIQUIDATION: 14 days negative
  LIQUIDATION --> OK: cash recovered after sales
  LIQUIDATION --> RESTRUCTURING: nothing left, restructuring available
  LIQUIDATION --> BANKRUPT: nothing left, no restructuring left
  RESTRUCTURING --> OK: accept
  RESTRUCTURING --> BANKRUPT: decline
  BANKRUPT --> [*]
```

## 6. Postgres schema
```mermaid
erDiagram
  users ||--o{ identities : "signs in with"
  users ||--o{ devices : owns
  users ||--o{ refresh_tokens : has
  devices ||--o{ refresh_tokens : "issued to"
  users ||--o{ saves : owns
  users ||--o{ user_aliases : "merged from"
  users ||--o{ deletion_jobs : requests
  users ||--o| save_quotas : "limited by"
  devices ||--o{ saves : "last written by"

  users {
    uuid id PK
    timestamptz created_at
    timestamptz deleted_at
    uuid merged_into FK
  }
  user_aliases {
    uuid alias PK
    uuid canonical FK
    timestamptz merged_at
  }
  deletion_jobs {
    uuid id PK
    uuid user_id
    timestamptz requested_at
    boolean storage_done
    boolean clickhouse_done
    boolean sentry_done
    boolean postgres_done
    int attempts
    text last_error
  }
  identities {
    text provider PK
    text subject PK
    uuid user_id FK
    text email
  }
  devices {
    uuid id PK
    uuid user_id FK
    text platform
    text app_version
    timestamptz last_seen
  }
  refresh_tokens {
    bytea token_hash PK
    uuid user_id FK
    uuid device_id FK
    timestamptz expires_at
    timestamptz revoked_at
  }
  saves {
    uuid id PK
    uuid user_id FK
    text name
    bigint revision "optimistic lock"
    bigint tick
    int schema_version
    text content_version
    jsonb summary "day, cash, net worth"
    text blob_key "saves/{save_id}/{revision}"
    int blob_size
    bytea blob_sha256
    uuid device_id FK
    timestamptz updated_at
  }
  save_quotas {
    uuid user_id PK
    int max_saves "default 10"
  }
  blob_gc {
    text key PK "old revision to delete"
    timestamptz enqueued_at
    int attempts
  }
  feature_flags {
    text key PK
    boolean enabled
    jsonb variants
    jsonb rules "targeting"
    boolean sim_affecting
    timestamptz updated_at
  }
  config_params {
    text key PK
    jsonb value
    text description
    timestamptz updated_at
  }
```

## 7. ClickHouse schema
`events` uses `ReplacingMergeTree(received_at)`, `PARTITION BY toYYYYMM(event_date)`, `ORDER BY (event_name, event_date, user_id, event_id)` and `TTL 90 days` with codecs (Delta/ZSTD, T64, Gorilla). The rollup tables are kept 2 years, and Grafana reads only those.

```mermaid
erDiagram
  events ||--o{ dau_mv : "session_start"
  dau_mv ||--|| dau : "writes to"
  events ||--o{ route_daily_mv : "route_daily"
  route_daily_mv ||--|| route_daily_agg : "writes to"
  events ||--o{ event_outcomes_mv : "event_outcome"
  event_outcomes_mv ||--|| event_outcomes_agg : "writes to"

  events {
    UUID event_id "sort key 4"
    LowCardinality_String event_name "sort key 1"
    DateTime64_3 ts "client time"
    DateTime64_3 received_at "version column"
    Date event_date "MATERIALIZED, sort key 2"
    UUID user_id "sort key 3"
    UUID device_id
    UUID session_id
    UUID save_id
    LowCardinality_String platform
    LowCardinality_String app_version
    LowCardinality_String content_version
    LowCardinality_String geo_country
    UInt32 game_day
    String route_id "typed hot field"
    LowCardinality_String ref_id "typed hot field"
    Int64 amount_cents "typed hot field"
    Float64 value "typed hot field"
    Map_String_String props
    Map_String_Float64 metrics
  }
  dau_mv {
    Date day
    LowCardinality_String platform
    uniqState users
  }
  dau {
    Date day "sort key 1"
    LowCardinality_String platform "sort key 2"
    AggregateFunction_uniq_UUID users
  }
  route_daily_mv {
    Date day
    String platform
    String content_version
  }
  route_daily_agg {
    Date day "sort key 1"
    LowCardinality_String platform "sort key 2"
    LowCardinality_String content_version "sort key 3"
    AggregateFunction_uniq routes
    SimpleAggregateFunction_sum revenue
    AggregateFunction_quantiles load_factor
  }
  event_outcomes_mv {
    Date day
    String ref_id
  }
  event_outcomes_agg {
    Date day "sort key 1"
    LowCardinality_String ref_id "event id, sort key 2"
    LowCardinality_String choice "sort key 3"
    LowCardinality_String outcome "sort key 4"
    SimpleAggregateFunction_sum n
  }
```

## 8. Save state model
The game state is stored as one gzip-compressed MessagePack snapshot in object storage. The Postgres row in `saves` points to it. Plane models come from content files, not from the save.

```mermaid
erDiagram
  STATE ||--|{ AIRLINE : "player + AI"
  STATE ||--|| FUEL_MARKET : has
  STATE ||--|| USED_MARKET : has
  STATE ||--o{ MODIFIER : active
  STATE ||--o{ PENDING_EVENT : "player prompts"
  STATE ||--o{ MISSION : "player mission board"
  STATE ||--o{ COUNTRY : world
  STATE ||--o{ MARKET : "airport pairs"
  AIRLINE |o--o{ COUNTRY : "owns (exclusive)"
  AIRLINE ||--o{ LANDING_RIGHT : leases
  AIRLINE ||--o{ PLANE : owns
  AIRLINE ||--o{ ROUTE : operates
  AIRLINE ||--o{ LOAN : owes
  AIRLINE ||--o{ OFFER : "makes / receives"
  AIRLINE }o--o| AI_PERSONALITY : "AI only"
  COUNTRY ||--o{ AIRPORT : contains
  LANDING_RIGHT }o--|| AIRPORT : "grants access to"
  ROUTE }o--|| AIRPORT : "departs from"
  ROUTE }o--|| AIRPORT : "arrives at"
  ROUTE }o--|| MARKET : "competes in"
  MARKET ||--o{ BACKGROUND_CARRIER : includes
  ROUTE ||--o{ PLANE : assigned
  PLANE }o--|| PLANE_MODEL : "instance of"
  USED_MARKET ||--o{ PLANE_OFFER : lists

  STATE {
    int version
    uint64 seed
    int64 tick
    int speed
    enum difficulty
    json configSnapshot "sim-affecting flags"
  }
  AIRLINE {
    string id PK "P or A1..A6"
    string name
    string color
    enum controller "human, ai"
    string personality FK "AI only"
    string homeCountry
    int64 cashCents
    float reputation
    int creditScore
    enum status
    int restructuringsLeft
    stringList hubs
  }
  AI_PERSONALITY {
    string id PK "from content"
    float riskTolerance
    float priceBias
    float expansionRate
    float aggression
  }
  COUNTRY {
    string iso2 PK
    string ownerId FK "empty if unowned"
    int64 purchasedTick
  }
  AIRPORT {
    string iata PK
    string iso2 FK
    int baseLevel "real-world, from datagen"
    int level "above base only by owner"
  }
  LANDING_RIGHT {
    string airlineId PK
    string iata PK
    int64 yearlyLeaseCents
    int64 expiresTick
    int departuresCap
  }
  MARKET {
    string pairKey PK "IATA-IATA"
    json lastShares "per airline"
  }
  BACKGROUND_CARRIER {
    string id PK
    string name
    int64 priceEcoCents
    int64 priceBizCents
    float quality
    int seatsPerDay
  }
  ROUTE {
    id id PK
    string airlineId FK
    string fromIata FK
    string toIata FK
    int64 priceEcoCents
    int64 priceBizCents
  }
  PLANE {
    id id PK
    string airlineId FK
    string modelId FK
    int level
    int ageDays
    float condition
    int seatsEco
    int seatsBiz
    id routeId FK
    enum state "flying, idle, overhaul, reconfig"
  }
  PLANE_MODEL {
    string id PK "from content"
    int tier
    int seatUnits
    int rangeKm
    int speedKmh
    int64 priceCents
  }
  LOAN {
    id id PK
    string airlineId FK
    int64 principalCents
    float rate
    int weeksLeft
    int64 instalmentCents
    int missedPayments
  }
  OFFER {
    id id PK
    enum kind "countryBuyout, takeover"
    string fromAirline FK
    string toAirline FK
    string subject "ISO2 or airline id"
    int64 priceCents
    int64 expiresTick
    enum status
  }
  MODIFIER {
    string id PK
    string source
    enum scope
    string target
    string stat
    enum op
    float value
    int64 expiresTick
  }
  PENDING_EVENT {
    id id PK
    string eventId
    int64 expiresTick
    string chosen
    int64 revealTick
  }
  MISSION {
    id id PK
    string templateId
    json params
    int64 deadlineTick
    enum status
  }
  FUEL_MARKET {
    float price
    float mean
    floatList history90d
  }
  USED_MARKET {
    int64 refreshTick
  }
  PLANE_OFFER {
    id id PK
    string modelId
    int ageDays
    float condition
    int64 priceCents
  }
```

## 9. AI airline decision loop
```mermaid
flowchart TB
  TRG{{"Daily slot (staggered per AI)<br/>or urgent trigger: cash below 0, offer, price war"}} --> GEN
  subgraph GEN["Candidate generators (capped, e.g. 50 each)"]
    G1[Expansion<br/>countries, landing rights, buyouts]
    G2[Network<br/>top-K precomputed pairs, hubs]
    G3[Fleet<br/>buy, assign, upgrade, sell]
    G4[Pricing<br/>hill-climb on forecast]
    G5[Finance<br/>loans, reserve]
    G6[Corporate<br/>offers, takeovers, event choices]
  end
  GEN --> SC["Score = expected profit x personality weights - risk<br/>forecast noise by difficulty"]
  SC --> PICK["Pick best within budget and max actions per day<br/>mistake chance by difficulty"]
  PICK --> CMDS["Commands, same validation as player"]
  CMDS --> Q[("Command queue")]
```
