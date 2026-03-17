# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Real-time IoT sensor simulation platform. Simulates 50+ environmental sensors (temperature, pollution, humidity, noise) across a virtual smart city, streaming live data to a React frontend via SSE. Production-hardened with circuit breakers, retry logic, dead letter queues, anomaly detection, and pattern detection.

## Architecture

```
Sensor Simulator → Kafka ─┬→ Data Ingestion → PostgreSQL (persistence)
  ↑ circuit breaker        │   ↑ retry + DLQ     → Redis (caching only)
  ↑ message buffering      │   ↑ anomaly detect  → Kafka anomaly/event topics
                           │   ↑ pattern detect
                           │   ↑ aggregation worker
                           └→ API Gateway (own consumer group) → SSE → Frontend
                              ↑ 3-topic subscribe (readings/anomalies/events)
                              ↑ pre-computed analytics fast path
```

Three independently running Go services share `backend/internal/` packages:

- **`cmd/sensor-simulator`** – Generates readings for all sensors every `GENERATION_INTERVAL_MS` ms. Listens on `simulation:commands` Redis Pub/Sub channel for runtime control (add/delete sensors, change scenarios). Syncs active sensors from DB every 5s. Circuit breaker on Kafka producer with in-memory message buffering (up to 1000 msgs) when circuit opens. Auto-creates Kafka topics on startup.
- **`cmd/data-ingestion`** – Consumes `sensor-readings` Kafka topic with retry (3 retries, exponential backoff) and DLQ on exhaustion. Does NOT persist raw readings to PostgreSQL — only caches in Redis and accumulates in-memory hourly aggregates that flush to `sensor_aggregates` every 5 minutes. Redis operations wrapped with circuit breaker for caching. Publishes anomalies to `sensor-anomalies` and detected events to `sensor-events` Kafka topics (no Redis Pub/Sub). Runs inline anomaly detection (Welford's algorithm, z-score > 2.5) and zone-level pattern detection (industrial_incident, heatwave, rush_hour). Starts background aggregation worker (city/zone stats caching, 5-min interval) and Kafka consumer lag monitor (30s interval). Maintains Redis counters: `stats:total_readings` (INCR), `stats:distinct_sensors` (HyperLogLog), `session:reading_stats:{session_id}` (per-session reading stats).
- **`cmd/api-gateway`** – Fiber HTTP server on `:8080`. Bridges REST + SSE. Admin endpoints publish commands to Redis `simulation:commands`, which the simulator picks up. Background goroutines: Kafka consumer (3 topics: `sensor-readings`, `sensor-anomalies`, `sensor-events` via own consumer group `gateway-group`), session inactivity monitor, nerd-stats aggregator. Health endpoint checks Kafka connectivity alongside PostgreSQL and Redis. Analytics endpoints use pre-computed `sensor_aggregates` and cached city/zone stats (no raw readings fallback). Session/global reading stats sourced from Redis counters.

### Frontend

Single-page React 19 app (`frontend/src/App.tsx` is the monolithic component). Uses Google Maps API for sensor map. Connects to SSE stream at `/stream`. Key constants in `frontend/src/constants/app.ts`, types in `frontend/src/types/app.ts`. Displays anomaly feed panel (scrolling list, max 20 events) in sidebar and scenario banners (auto-dismissing) at top of page.

### Key design decisions

- **Session-scoped sensors**: Each browser session gets a UUID (`SESSION_KEY` in localStorage). Sensors created via the UI are tagged with `session_id`; the inactivity monitor auto-shuts them down after idling.
- **Redis as control plane**: The simulator polls `simulation:config` and `scenario:active` Redis keys every 5s for runtime changes pushed by the admin API.
- **No raw reading persistence**: Raw sensor readings are NOT written to PostgreSQL. Instead, data-ingestion accumulates readings in-memory and flushes hourly aggregates to `sensor_aggregates` every 5 minutes. Per-session and global reading stats are maintained via Redis counters.
- **SSE broadcaster pattern**: `internal/sse` maintains a set of connected clients; the API gateway's Kafka consumer fans out messages to all. Supports three event types: sensor updates, anomalies, and detected events.
- **Circuit breakers**: Wrap Kafka producer (simulator), Redis operations (data-ingestion). States: closed → open (after threshold failures) → half-open (after timeout) → closed (after success threshold). Prevents cascading failures.
- **Dead Letter Queue**: Failed messages after retry exhaustion go to `sensor-readings-dlq` topic with error headers. DLQ messages are browsable via API.
- **Pre-computed analytics**: Data-ingestion accumulates in-memory hourly buckets and flushes to `sensor_aggregates` every 5 min. Aggregation worker caches city stats and zone stats in Redis with 5-min TTL. API endpoints use pre-computed data only (no raw query fallback).
- **Anomaly detection**: Welford's online algorithm (O(1) per reading). Per-sensor running mean/variance. Z-score > 2.5 after 30 samples triggers anomaly. Results stored in `anomaly_events` table and broadcast via SSE.
- **Pattern detection**: Zone-level sliding windows (2-min window, 5-min debounce). Detects industrial_incident, heatwave, rush_hour based on multi-sensor thresholds. Results stored in `detected_events` table and broadcast via SSE.

### Simulation scenarios

Defined in `backend/internal/scenario/scenario.go`: `normal`, `rush_hour`, `heatwave`, `industrial_incident`. The active scenario is stored in Redis (`scenario:active`) and read by the simulator every 5s.

### Internal packages

- **`internal/resilience`** – Generic circuit breaker (`circuitbreaker.go`) and exponential backoff retry with jitter (`retry.go`). Used by producer, consumer, data-ingestion, and simulator.
- **`internal/kafka`** – Producer (idempotent, LZ4 compression, circuit breaker), consumer (retry + DLQ), DLQ producer (`dlq.go`), topic admin (`topics.go`), consumer lag monitor (`metrics.go`), zone-based partitioner (`partitioner.go`).
- **`internal/anomaly`** – Welford's algorithm anomaly detector (`detector.go`), zone-level pattern detector (`patterns.go`).
- **`internal/aggregation`** – Background worker (`worker.go`) that populates `sensor_aggregates` and caches city/zone stats in Redis.
- **`internal/zones`** – Shared zone boundary constants and `GetZone(lat, lng)` helper. Used by aggregation worker, pattern detector, and analytics handlers.
- **`internal/models`** – Data models including `events.go` (AnomalyEvent, DetectedEvent).

## Development Commands

### Infrastructure (Docker)

```bash
# Start all infrastructure + backend services
./scripts/start-all.sh

# Or manually
docker compose up -d      # Start Zookeeper, Kafka, PostgreSQL, Redis, Adminer, and pre-built service containers
docker compose down       # Stop all
docker compose logs -f api-gateway   # Tail logs for a service
```

PostgreSQL is on port **5433** (avoids conflict with local installs). Adminer UI at http://localhost:8083.

### Backend (Go)

All commands run from `backend/`:

```bash
# Run individual services (reads .env from project root)
go run cmd/sensor-simulator/main.go
go run cmd/data-ingestion/main.go
go run cmd/api-gateway/main.go

# Run all unit tests
go test ./internal/... -v

# Run tests for a specific package
go test ./internal/postgres/... -v
go test ./internal/redis/... -v
go test ./internal/kafka/... -v
go test ./internal/sse/... -v
go test ./internal/resilience/... -v

# Run integration tests (requires running API gateway)
go test ./tests/integration/... -v

# Run performance tests
go test ./tests/performance/... -v
```

Configuration is loaded from `.env` in the project root (or `../.env` relative to the working directory). Each service calls `cfg.Validate("service-name")` on startup, which will fail fast with missing required vars.

### Frontend

```bash
cd frontend
npm install
npm run dev       # Dev server at http://localhost:5173
npm run build     # TypeScript check + Vite build
npm run lint      # ESLint
```

Frontend env vars: `VITE_API_URL` (default: `http://localhost:8080/api/v1`), `VITE_GOOGLE_MAPS_API_KEY`.

### Kubernetes

```bash
kubectl apply -f k8s/ -n smart-city
kubectl get pods -n smart-city
kubectl scale deployment data-ingestion --replicas=5
kubectl port-forward svc/api-gateway 8080:8080
```

## API Routes

Base: `http://localhost:8080/api/v1` — all routes are defined in `backend/cmd/api-gateway/routes.go`.

- `/health` — health check with PostgreSQL, Redis, and Kafka connectivity (no prefix)
- `/stream` — SSE stream endpoint, delivers `sensor_update`, `anomaly`, and `scenario_detected` events (no prefix)
- `/api/v1/sensors` — sensor CRUD + control
- `/api/v1/readings` — sensor readings
- `/api/v1/analytics/*` — city stats, top polluted, zone analytics, hourly aggregations, anomalies, detected events
  - `/api/v1/analytics/anomalies?limit=50&since=...` — recent anomaly events
  - `/api/v1/analytics/events?limit=20&since=...` — recent detected events (patterns)
- `/api/v1/metrics/*` — Kafka (with consumer lag), Redis, system health, nerd stats
- `/api/v1/pipeline/*` — Kafka topic/message browser, Redis key browser, SSE connection info, DLQ browser
  - `/api/v1/pipeline/dlq?limit=10` — browse dead letter queue messages
- `/api/v1/admin/*` — simulation control (rate, scenario, chaos mode, thresholds)
- `/api/v1/session/*` — per-session sensor config

Rate limiting is applied via Redis to all `/api/v1` routes (`internal/middleware/ratelimit.go`).

## Data Models

PostgreSQL schema is in `migrations/` (auto-applied by Docker on first start):
- `sensors` — sensor registry with UUID PK
- `sensor_readings` — (LEGACY, no longer written to) partitioned by month on `timestamp`. Raw readings are NOT persisted; stats come from Redis counters and `sensor_aggregates`.
- `sensor_aggregates` — hourly/daily pre-computed aggregates (populated by aggregation worker every 5 min)
- `anomaly_events` — z-score anomaly detections with expected mean/stddev (`migrations/010_stream_processing.sql`)
- `detected_events` — zone-level pattern detections with sensor_ids UUID array (`migrations/010_stream_processing.sql`)

Kafka topics:
- `sensor-readings` — main sensor data pipeline (6 partitions), consumed by both data-ingestion and api-gateway
- `sensor-readings-dlq` — dead letter queue for failed messages (1 partition)
- `sensor-anomalies` — anomaly events published by data-ingestion (3 partitions), consumed by api-gateway
- `sensor-events` — detected pattern events published by data-ingestion (3 partitions), consumed by api-gateway

Redis key patterns (see `internal/redis/operations.go`):
- `sensor:latest:{uuid}` — hash of latest reading (short TTL)
- `sensor:stream:{uuid}` — Redis stream, capped at 1000 entries
- `pollution:leaderboard` — sorted set by PM2.5 value
- `simulation:config` — JSON of `SimulationConfig` (polled by simulator)
- `scenario:active` — JSON of active `Scenario` (polled by simulator)
- `simulation:commands` — Pub/Sub channel for immediate sensor control
- `analytics:city-stats:computed` — pre-computed city stats JSON (5-min TTL, written by aggregation worker)
- `analytics:zone:{zone}:{type}` — pre-computed zone stats (5-min TTL, written by aggregation worker)
- `kafka:consumer-lag:{groupID}` — consumer lag JSON (written by lag monitor every 30s)
- `stats:total_readings` — global counter of all readings processed (INCR by data-ingestion)
- `stats:distinct_sensors` — HyperLogLog of distinct sensor IDs that have produced readings
- `session:reading_stats:{session_id}` — hash with `total_readings`, `value_sum`, `last_reading_at` (per-session stats)
- `sensor:anomalies` — (legacy, no longer used for real-time — API gateway consumes Kafka directly)
- `sensor:events` — (legacy, no longer used for real-time — API gateway consumes Kafka directly)

## Environment Variables

In addition to the standard Kafka/Redis/PostgreSQL vars, the following resilience/intelligence vars are used (see `.env`):

| Variable | Default | Used by |
|---|---|---|
| `KAFKA_TOPIC_DLQ` | `sensor-readings-dlq` | data-ingestion, api-gateway |
| `KAFKA_TOPIC_ANOMALIES` | `sensor-anomalies` | data-ingestion, api-gateway |
| `KAFKA_TOPIC_EVENTS` | `sensor-events` | data-ingestion, api-gateway |
| `KAFKA_CONSUMER_GROUP_GATEWAY` | `gateway-group` | api-gateway — own consumer group for real-time SSE |
| `CB_THRESHOLD` | `5` | all services — circuit breaker failure threshold |
| `CB_TIMEOUT_SECONDS` | `30` | all services — circuit breaker open-to-half-open timeout |
| `MAX_RETRIES` | `3` | data-ingestion — Kafka consumer retry count |
| `RETRY_INITIAL_BACKOFF_MS` | `100` | data-ingestion — initial retry backoff |

## Troubleshooting

- **Kafka not ready**: Wait 15–20s after `docker compose up` before starting Go services. Topics are auto-created by services on startup.
- **Partition errors**: Add partitions for the current/next month in `migrations/003_create_partitions.sql` and re-apply, or run the SQL manually via Adminer.
- **Simulator not responding to admin commands**: Check that `simulation:commands` Pub/Sub channel is working; Redis Pub/Sub listener runs in the simulator's `listenForCommands` goroutine.
- **DLQ messages accumulating**: Browse via `GET /api/v1/pipeline/dlq?limit=10`. Check the `error` and `original-topic` headers for root cause.
- **No anomalies detected**: Anomaly detection requires 30+ readings per sensor before z-scores are computed. Wait a few minutes after starting.
- **Missing aggregates**: The aggregation worker runs every 5 minutes. Check `SELECT COUNT(*) FROM sensor_aggregates` after waiting.
- **Circuit breaker open**: Check logs for `circuit breaker opened` messages. The breaker auto-recovers after `CB_TIMEOUT_SECONDS`. During open state, data-ingestion skips Redis updates but continues PostgreSQL writes; simulator buffers messages in memory.
- **Failed batch files**: If PostgreSQL batch writes fail after all retries, data is written to `/tmp/smart-city-failed-batch-*.json` inside the data-ingestion container.
