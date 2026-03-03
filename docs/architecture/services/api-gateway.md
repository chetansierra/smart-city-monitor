# API Gateway Architecture

## Stack

- Go + Fiber
- SSE broadcaster (`internal/sse`)
- Redis pub/sub listener for live fanout

## Responsibilities

- REST APIs for sensors, readings, analytics, admin, metrics.
- Session handling (via `X-Session-ID`).
- SSE streaming endpoint: `/stream`.
- Aggregated platform metrics endpoint: `/api/v1/metrics/nerds`.
- Nerd stats cache + broadcaster loop.

## Real-time Path

- Receives Redis pub/sub messages from `sensor:updates`.
- Broadcasts to SSE clients as `sensor_update`.
- Broadcasts internal metrics snapshots as `stats_update`.
