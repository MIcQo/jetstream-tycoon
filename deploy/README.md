# Local infrastructure

`make infra-up` starts everything below with health checks. `make dev` also applies migrations and runs the API server.
Credentials are for local development only.

| Service | URL / port | Login |
|---|---|---|
| Postgres | `localhost:5432` | `jetstream` / `jetstream`, db `jetstream` |
| ClickHouse | HTTP `localhost:8123`, native `localhost:9000` | `default` / `jetstream` |
| RabbitMQ | AMQP `localhost:5672`, UI http://localhost:15672 | `jetstream` / `jetstream` |
| Redis | `localhost:6379` | — |
| MinIO (S3) | API `localhost:9002`, console http://localhost:9001 | `jetstream` / `jetstream-dev-secret`, bucket `jetstream-saves` |
| Prometheus | http://localhost:9090 | — |
| Alertmanager | http://localhost:9093 | — |
| Grafana | http://localhost:3000 | `admin` / `jetstream` |

- `minio-init` creates the `jetstream-saves` bucket and a lifecycle rule that expires `staging/` after 1 day (ADR-0023).
- Grafana has Prometheus and ClickHouse data sources and the dashboards in `grafana/dashboards/` provisioned.
- Prometheus scrapes the exporters, RabbitMQ (`:15692`) and ClickHouse (`:9363`). The `api_server` and `ingestor` targets show as down until those services run in containers (planned with their Dockerfiles). The `TargetDown` alert is expected for them locally.
- `make infra-down` stops everything. `make infra-reset` also **deletes all local data volumes**.
