# ADR-0012: RabbitMQ between the API and a separate ClickHouse ingestor

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
An in-memory batcher in the API loses events on a crash or deploy, and fills up when ClickHouse is unavailable. Clients delete local events once the server accepts them, so acceptance must mean the events are stored safely.

## Decision
- The API publishes telemetry to the durable quorum queue `telemetry.events`, using publisher confirms.
- The API replies **202 only after the confirm**; if RabbitMQ is unavailable it replies 503 and the client retries.
- `cmd/ingestor` consumes (prefetch about 10k), batch-inserts into ClickHouse with a dedup token, and **acks only after a successful insert**.
- Failures are requeued with backoff. Malformed messages go to `telemetry.dlq`.
- Delivery is at-least-once, with duplicates removed by `event_id`.

## Consequences
- No data loss when ClickHouse is down or during deploys, and the API and ingestor scale independently.
- Adds RabbitMQ to operate and monitor (queue depth, consumers, DLQ alerts).

## Alternatives considered
ClickHouse `async_insert` (simpler, but no buffering during an outage); Kafka/Redpanda (more throughput than needed, heavier to operate); Redis Streams (weaker durability guarantees).
