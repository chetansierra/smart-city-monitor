# Smart City Monitor — Architecture Documentation

This directory contains the architecture documentation for the Smart City Monitor platform. Each document is self-contained and can be read independently.

## Document Index

### System Overview

- [overview.md](overview.md) — High-level system architecture, data flow, and design principles.

### Service Documentation

- [services/sensor-simulator.md](services/sensor-simulator.md) — Sensor data generation, Kafka production, circuit breaker, message buffering.
- [services/data-ingestion.md](services/data-ingestion.md) — Kafka consumption, retry/DLQ, anomaly detection, pattern detection, aggregation.
- [services/api-gateway.md](services/api-gateway.md) — REST API, SSE streaming, Kafka multi-topic consumer, session management.
- [services/frontend.md](services/frontend.md) — React 19 SPA, Google Maps, SSE client, anomaly feed.

### Technology Documentation

- [technologies/kafka.md](technologies/kafka.md) — KRaft mode, topics, partitioning, consumer groups, DLQ.
- [technologies/redis.md](technologies/redis.md) — Caching, keyspace design, TTL policy, Pub/Sub channels.
- [technologies/postgresql.md](technologies/postgresql.md) — Schema, tables, aggregates, no raw reading persistence.
- [technologies/sse.md](technologies/sse.md) — SSE broadcaster, event types, connection lifecycle.

### Cross-Cutting Concerns

- [resilience.md](resilience.md) — Circuit breakers, retry with backoff, dead letter queue.
- [intelligence.md](intelligence.md) — Anomaly detection (Welford's algorithm), zone-level pattern detection.
- [deployment.md](deployment.md) — Docker Compose, production deployment, Kubernetes.

## Maintenance Rule

When any service behavior, data model, or technology configuration changes, update the corresponding document in this directory as part of the same PR or commit.
