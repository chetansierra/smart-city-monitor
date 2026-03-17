# Smart City Monitor — System Architecture Overview

## What This System Does

Smart City Monitor is a real-time IoT sensor simulation platform. It simulates 50+ environmental sensors (temperature, pollution, humidity, noise) across a virtual smart city, streaming live data to a web frontend via Server-Sent Events (SSE). The system includes production-grade resilience (circuit breakers, retry logic, dead letter queues), real-time anomaly detection, and zone-level pattern detection.

## Core Data Flow

```
Sensor Simulator → Kafka ─┬→ Data Ingestion → PostgreSQL (aggregates only)
  ↑ circuit breaker        │   ↑ retry + DLQ     → Redis (caching)
  ↑ message buffering      │   ↑ anomaly detect  → Kafka anomaly/event topics
                           │   ↑ pattern detect
                           │   ↑ aggregation worker
                           └→ API Gateway (own consumer group) → SSE → Frontend
                              ↑ 3-topic subscribe (readings/anomalies/events)
                              ↑ pre-computed analytics fast path
```

## Runtime Services

The platform consists of three independently running Go backend services that share `backend/internal/` packages, plus a React frontend:

| Service | Language | Port | Role |
|---------|----------|------|------|
| Sensor Simulator | Go | — | Generates sensor readings, produces to Kafka |
| Data Ingestion | Go | — | Consumes Kafka, runs anomaly/pattern detection, writes aggregates |
| API Gateway | Go (Fiber) | 8080 | REST API + SSE streaming, consumes 3 Kafka topics |
| Frontend | React 19 + TypeScript | 5173 (dev) | SPA with Google Maps, live sensor visualization |

## Infrastructure Dependencies

| Component | Image | Port | Purpose |
|-----------|-------|------|---------|
| Kafka | confluentinc/cp-kafka:7.5.0 | 9092, 9093 | Event backbone (KRaft mode, no Zookeeper) |
| PostgreSQL | postgres:16-alpine | 5433 | Durable storage for sensors, aggregates, anomaly/event records |
| Redis | redis:7-alpine | 6379 | Hot cache, coordination, Pub/Sub, counters |
| Adminer | adminer:latest | 8083 | PostgreSQL web UI |

## Key Architecture Principles

1. **SSE is the only real-time client transport.** The frontend receives all live updates (sensor readings, anomalies, detected events) through a single SSE connection.

2. **Redis is cache and coordination only.** PostgreSQL is the durable source of truth. Redis stores hot data with TTLs and bounded structures to prevent unbounded growth.

3. **No raw reading persistence.** Raw sensor readings are NOT written to PostgreSQL. Data ingestion accumulates readings in-memory and flushes hourly aggregates to `sensor_aggregates` every 5 minutes. Per-session and global reading stats use Redis counters.

4. **API Gateway consumes Kafka directly.** The API Gateway runs its own Kafka consumer group (`gateway-group`) subscribing to three topics: `sensor-readings`, `sensor-anomalies`, `sensor-events`. It does NOT rely on Redis Pub/Sub for real-time data.

5. **Session-scoped sensors.** Each browser session gets a UUID stored in localStorage. Sensors created via the UI are tagged with `session_id`. An inactivity monitor auto-shuts them down after idling.

6. **Pre-computed analytics only.** Analytics endpoints serve data from `sensor_aggregates` and Redis-cached city/zone stats. There is no fallback to raw reading queries.

7. **Circuit breakers prevent cascading failures.** Kafka producer (simulator), Redis operations (data-ingestion), and other external calls are wrapped with circuit breakers that open after threshold failures and auto-recover.

## Simulation Scenarios

The system supports four predefined scenarios that alter sensor behavior:

| Scenario | Effect |
|----------|--------|
| `normal` | Default sensor behavior |
| `rush_hour` | Elevated noise and pollution in downtown/highway zones |
| `heatwave` | Elevated temperatures across all zones |
| `industrial_incident` | Spike in pollution and noise in industrial zones |

Scenarios are stored in Redis (`scenario:active`) and polled by the simulator every 5 seconds. They can be activated via the admin API.
