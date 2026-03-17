# Smart City Monitor — API Gateway Service

## Overview

The API Gateway is a Go service built on the Fiber framework. It serves REST APIs, manages SSE streaming connections, and consumes three Kafka topics to deliver real-time data to connected clients.

**Entry point:** `backend/cmd/api-gateway/main.go`
**Routes:** `backend/cmd/api-gateway/routes.go`

## Responsibilities

- Serve REST API endpoints for sensors, readings, analytics, metrics, admin, pipeline, and session management.
- Run SSE streaming endpoint (`/stream`) that delivers sensor updates, anomalies, and detected events to connected browsers.
- Run background Kafka consumer (`gateway-group`) subscribing to three topics: `sensor-readings`, `sensor-anomalies`, `sensor-events`.
- Fan out Kafka messages to all connected SSE clients via the broadcaster pattern (`internal/sse`).
- Manage session lifecycle (creation, heartbeat, inactivity timeout, teardown).
- Aggregate and cache nerd stats for the metrics dashboard.
- Publish admin commands to Redis `simulation:commands` Pub/Sub channel.
- Apply per-endpoint rate limiting via Redis.

## Real-Time Data Path

```
Kafka (3 topics) → API Gateway Consumer → SSE Broadcaster → Connected Clients
```

The API Gateway does NOT use Redis Pub/Sub for real-time sensor data. It consumes Kafka directly with its own consumer group (`gateway-group`), independent of the data-ingestion consumer group.

### SSE Event Types

| Event Type | Source Topic | Payload |
|------------|-------------|---------|
| `sensor_update` | sensor-readings | Sensor reading data |
| `anomaly` | sensor-anomalies | Anomaly detection with z-score, expected mean/stddev |
| `scenario_detected` | sensor-events | Zone-level pattern detection |

## API Route Groups

All routes are defined in `backend/cmd/api-gateway/routes.go`. Base URL: `http://localhost:8080`.

### Top-Level (no prefix)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/health` | healthHandler.Check | Health check with PostgreSQL, Redis, Kafka connectivity |
| GET | `/stream` | sseHandler.HandleStream | SSE streaming endpoint |
| GET | `/stream/stats` | sseHandler.GetStats | SSE connection statistics |

### Sensors (`/api/v1/sensors`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | List all sensors with filtering and pagination |
| GET | `/footprint` | Get sensor geographic footprint |
| POST | `/start` | Create and start a new sensor |
| DELETE | `/:id` | Delete a sensor |
| GET | `/:id` | Get sensor by ID |
| GET | `/:id/latest` | Get latest reading from Redis cache |
| GET | `/:id/readings` | Get historical readings |

### Session (`/api/v1/session`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/config` | Get session sensor configuration |
| PUT | `/config` | Update session configuration |
| DELETE | `/sensors` | Teardown session sensors |
| POST | `/teardown` | Teardown for sendBeacon (browser close) |

### Readings (`/api/v1/readings`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Get readings with filtering and pagination |
| GET | `/latest` | Get latest readings from all sensors |

### Analytics (`/api/v1/analytics`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/city-stats` | City-wide statistics (from pre-computed cache) |
| GET | `/sensors/:id/hourly` | Hourly aggregated stats for a sensor |
| GET | `/top-polluted` | Most polluted sensors |
| GET | `/top-temperature` | Hottest sensors |
| GET | `/quietest` | Quietest sensors |
| GET | `/hourly` | Hourly aggregations |
| GET | `/compare` | Sensor comparison data |
| GET | `/zones` | Zone-level analytics |
| GET | `/anomalies` | Recent anomaly events (from `anomaly_events` table) |
| GET | `/events` | Recent detected events (from `detected_events` table) |

### Metrics (`/api/v1/metrics`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/kafka` | Kafka metrics including consumer lag |
| GET | `/redis` | Redis memory and key metrics |
| GET | `/system` | System health overview |
| GET | `/nerds` | Aggregated nerd stats (global) |
| GET | `/nerds/global` | Global nerd stats (alias) |
| GET | `/nerds/session` | Per-session nerd stats |

### Pipeline (`/api/v1/pipeline`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/flow/stats` | Data flow statistics |
| GET | `/kafka/topics` | List Kafka topics |
| GET | `/kafka/topics/:topic` | Topic detail |
| GET | `/kafka/topics/:topic/messages` | Browse topic messages |
| GET | `/redis/keys` | Browse Redis keys |
| GET | `/redis/keys/:key` | Redis key detail |
| GET | `/dlq` | Browse dead letter queue messages |
| GET | `/stream/connections` | SSE connection info |

### Admin (`/api/v1/admin`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/sensors/control` | Control individual sensors (start/stop) |
| POST | `/sensors/start-all` | Start all sensors |
| POST | `/sensors/stop-all` | Stop all sensors |
| POST | `/simulation/rate` | Set generation rate |
| GET | `/simulation/status` | Get simulation status |
| POST | `/system/clear-cache` | Clear Redis cache |
| POST | `/system/reset` | Reset entire simulation |
| GET | `/scenarios` | List available scenarios |
| POST | `/scenarios/activate` | Activate a scenario |
| GET | `/scenarios/current` | Get current active scenario |
| GET | `/simulation/config` | Get full simulation config |
| POST | `/simulation/threshold` | Set sensor threshold |
| POST | `/simulation/behavior` | Set sensor behavior pattern |
| POST | `/simulation/time-compression` | Set time compression factor |
| POST | `/simulation/chaos-mode` | Toggle chaos mode |
| POST | `/simulation/reset-config` | Reset simulation config to defaults |

## Session Management

- Each browser session is identified by a UUID in the `X-Session-ID` header.
- Session sensors are tagged with `session_id` in PostgreSQL.
- A background inactivity monitor checks session heartbeats and auto-tears down idle sessions.
- Session teardown sends a `shutdown_session` command via Redis Pub/Sub to the simulator.

## Rate Limiting

All `/api/v1` routes are rate-limited via Redis. Implementation: `backend/internal/middleware/ratelimit.go`.

## Dependencies

| Dependency | Usage |
|------------|-------|
| Kafka | Consume 3 topics for real-time data |
| PostgreSQL | Read sensors, aggregates, anomalies, events |
| Redis | Session state, rate limiting, caching, Pub/Sub for admin commands |
