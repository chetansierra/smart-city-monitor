# Smart City Monitor — API Reference

**Base URL:** `http://localhost:8080`
**Last Updated:** March 2026

## Overview

The API Gateway exposes REST endpoints for sensor management, analytics, system metrics, pipeline visualization, and admin controls. All real-time data is delivered via SSE (see `/stream`). Rate limiting is applied to all `/api/v1` routes via Redis.

## Authentication

No authentication required (development mode). Session tracking via `X-Session-ID` header.

## Response Format

### Success

```json
{
  "success": true,
  "data": { ... },
  "meta": { "page": 1, "limit": 10, "total": 100 }
}
```

### Error

```json
{
  "success": false,
  "error": { "code": "ERROR_CODE", "message": "Human-readable message" }
}
```

---

## Health Check

### `GET /health`

Returns health status of PostgreSQL, Redis, and Kafka.

```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "services": {
      "postgres": "healthy",
      "redis": "healthy",
      "kafka": "healthy"
    }
  }
}
```

---

## SSE Streaming

### `GET /stream?session_id=<uuid>`

Server-Sent Events endpoint. Delivers three event types:

| Event Type | Description |
|------------|-------------|
| `sensor_update` | Live sensor reading |
| `anomaly` | Z-score anomaly detection |
| `scenario_detected` | Zone-level pattern detection |

### `GET /stream/stats`

Returns SSE connection count and statistics.

### `GET /api/v1/stream/history`

Returns recent SSE events for the current session.

---

## Sensors — `/api/v1/sensors`

### `GET /api/v1/sensors`

List all sensors. Supports filtering by `type` and pagination (`page`, `limit`).

### `GET /api/v1/sensors/footprint`

Get geographic footprint of all sensors.

### `GET /api/v1/sensors/:id`

Get a specific sensor by UUID.

### `GET /api/v1/sensors/:id/latest`

Get the latest reading for a sensor (from Redis cache).

### `GET /api/v1/sensors/:id/readings`

Get historical readings. Query params: `from`, `to`, `limit`.

### `POST /api/v1/sensors/start`

Create and start a new sensor. Body: `{ "name", "type", "latitude", "longitude" }`. Sensor is tagged with the session ID.

### `DELETE /api/v1/sensors/:id`

Delete a sensor.

---

## Session — `/api/v1/session`

### `GET /api/v1/session/config`

Get session sensor configuration.

### `PUT /api/v1/session/config`

Update session configuration.

### `DELETE /api/v1/session/sensors`

Teardown all sensors for the current session.

### `POST /api/v1/session/teardown`

Teardown endpoint for `sendBeacon` (browser close). Same as DELETE but accepts POST for beacon compatibility.

---

## Readings — `/api/v1/readings`

### `GET /api/v1/readings`

Get readings with filtering (`sensor_id`, `sensor_type`, `from`, `to`) and pagination (`page`, `limit`, `sort`).

### `GET /api/v1/readings/latest`

Get latest readings from all sensors, grouped by type.

---

## Analytics — `/api/v1/analytics`

All analytics endpoints serve pre-computed data from `sensor_aggregates` and Redis cache. No raw reading queries.

### `GET /api/v1/analytics/city-stats`

City-wide averages for all sensor types. Cached 5 minutes.

### `GET /api/v1/analytics/sensors/:id/hourly`

Hourly aggregated stats for a sensor. Query params: `from`, `to` (required).

### `GET /api/v1/analytics/top-polluted?limit=N`

Most polluted sensors by PM2.5 value.

### `GET /api/v1/analytics/top-temperature?limit=N`

Hottest sensors by temperature.

### `GET /api/v1/analytics/quietest?limit=N`

Quietest sensors by noise level.

### `GET /api/v1/analytics/hourly`

Hourly aggregations across sensors.

### `GET /api/v1/analytics/compare`

Sensor comparison data.

### `GET /api/v1/analytics/zones`

Zone-level analytics.

### `GET /api/v1/analytics/anomalies?limit=50&since=<timestamp>`

Recent anomaly events from `anomaly_events` table.

### `GET /api/v1/analytics/events?limit=20&since=<timestamp>`

Recent detected pattern events from `detected_events` table.

---

## Metrics — `/api/v1/metrics`

### `GET /api/v1/metrics/kafka`

Kafka metrics including consumer lag per group.

### `GET /api/v1/metrics/redis`

Redis memory usage, key count, and connection info.

### `GET /api/v1/metrics/system`

System health overview.

### `GET /api/v1/metrics/nerds`

Aggregated nerd stats (global reading counts, throughput, uptime).

### `GET /api/v1/metrics/nerds/global`

Alias for `/nerds`.

### `GET /api/v1/metrics/nerds/session`

Per-session reading stats.

---

## Pipeline — `/api/v1/pipeline`

### `GET /api/v1/pipeline/flow/stats`

Data flow statistics across the pipeline.

### `GET /api/v1/pipeline/kafka/topics`

List all Kafka topics.

### `GET /api/v1/pipeline/kafka/topics/:topic`

Details for a specific Kafka topic.

### `GET /api/v1/pipeline/kafka/topics/:topic/messages`

Browse messages in a Kafka topic.

### `GET /api/v1/pipeline/redis/keys`

Browse Redis keys.

### `GET /api/v1/pipeline/redis/keys/:key`

Get detail for a specific Redis key.

### `GET /api/v1/pipeline/dlq?limit=10`

Browse dead letter queue messages. Each message includes `error`, `original-topic`, and `retry-count` headers.

### `GET /api/v1/pipeline/stream/connections`

SSE connection info.

---

## Admin — `/api/v1/admin`

### Sensor Control

| Method | Path | Description |
|--------|------|-------------|
| POST | `/sensors/control` | Control individual sensors |
| POST | `/sensors/start-all` | Start all sensors |
| POST | `/sensors/stop-all` | Stop all sensors |

### Simulation Control

| Method | Path | Description |
|--------|------|-------------|
| POST | `/simulation/rate` | Set generation rate (ms) |
| GET | `/simulation/status` | Get simulation status |
| GET | `/simulation/config` | Get full simulation config |
| POST | `/simulation/threshold` | Set sensor threshold |
| POST | `/simulation/behavior` | Set sensor behavior pattern |
| POST | `/simulation/time-compression` | Set time compression factor |
| POST | `/simulation/chaos-mode` | Toggle chaos mode |
| POST | `/simulation/reset-config` | Reset config to defaults |

### Scenarios

| Method | Path | Description |
|--------|------|-------------|
| GET | `/scenarios` | List available scenarios (normal, rush_hour, heatwave, industrial_incident) |
| POST | `/scenarios/activate` | Activate a scenario. Body: `{ "scenario": "rush_hour" }` |
| GET | `/scenarios/current` | Get currently active scenario |

### System

| Method | Path | Description |
|--------|------|-------------|
| POST | `/system/clear-cache` | Clear Redis cache |
| POST | `/system/reset` | Reset entire simulation |
