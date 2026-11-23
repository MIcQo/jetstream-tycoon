# ADR-0013: Prometheus + Grafana for technical metrics and alerting

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
We need system health monitoring and alerting, separate from player analytics.

## Decision
- Services expose `/metrics` (`client_golang`) on an internal port. Exporters cover Postgres, Redis, RabbitMQ, ClickHouse, nodes and containers.
- Alertmanager handles alerts.
- Grafana shows Prometheus and the ClickHouse rollups, with dashboards provisioned as code.
- Labels never contain user or save IDs or raw paths.
- Client performance arrives through telemetry and is also aggregated into Prometheus histograms.

## Consequences
- Standard, cheap and self-hostable.
- Cardinality discipline is required.

## Alternatives considered
Hosted APM only (cost); putting technical metrics into ClickHouse (weaker alerting).
