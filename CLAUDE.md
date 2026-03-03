# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Real-time IoT sensor simulation platform. Simulates 50+ environmental sensors (temperature, pollution, humidity, noise) across a virtual smart city, streaming live data to a React frontend via SSE.

## Architecture

```
Sensor Simulator → Kafka (sensor-readings topic) → Data Ingestion → PostgreSQL + Redis
                                                                           ↓
Frontend (React + Google Maps) ← SSE ← API Gateway ← Redis Pub/Sub ←────┘
```

Three independently running Go services share `backend/internal/` packages:

- **`cmd/sensor-simulator`** – Generates readings for all sensors every `GENERATION_INTERVAL_MS` ms. Listens on `simulation:commands` Redis Pub/Sub channel for runtime control (add/delete sensors, change scenarios). Syncs active sensors from DB every 5s.
- **`cmd/data-ingestion`** – Consumes `sensor-readings` Kafka topic; batches writes (100 msgs or 1s) to PostgreSQL; immediately updates Redis cache (`sensor:latest:{id}`, `sensor:stream:{id}`, `pollution:leaderboard`) and publishes to `sensor:updates` for SSE.
- **`cmd/api-gateway`** – Fiber HTTP server on `:8080`. Bridges REST + SSE. Admin endpoints publish commands to Redis `simulation:commands`, which the simulator picks up. Background goroutines: Redis Pub/Sub listener, session inactivity monitor, nerd-stats aggregator.

### Frontend

Single-page React 19 app (`frontend/src/App.tsx` is the monolithic component). Uses Google Maps API for sensor map. Connects to SSE stream at `/stream`. Key constants in `frontend/src/constants/app.ts`, types in `frontend/src/types/app.ts`.

### Key design decisions

- **Session-scoped sensors**: Each browser session gets a UUID (`SESSION_KEY` in localStorage). Sensors created via the UI are tagged with `session_id`; the inactivity monitor auto-shuts them down after idling.
- **Redis as control plane**: The simulator polls `simulation:config` and `scenario:active` Redis keys every 5s for runtime changes pushed by the admin API.
- **Batch PostgreSQL writes**: Ingestion buffers up to 100 readings before flushing, or flushes every 1s — whichever comes first.
- **SSE broadcaster pattern**: `internal/sse` maintains a set of connected clients; the API gateway's Redis Pub/Sub listener fans out messages to all.

### Simulation scenarios

Defined in `backend/internal/scenario/scenario.go`: `normal`, `rush_hour`, `heatwave`, `industrial_incident`. The active scenario is stored in Redis (`scenario:active`) and read by the simulator every 5s.

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

- `/health` — health check (no prefix)
- `/stream` — SSE stream endpoint (no prefix)
- `/api/v1/sensors` — sensor CRUD + control
- `/api/v1/readings` — sensor readings
- `/api/v1/analytics/*` — city stats, top polluted, zone analytics, hourly aggregations
- `/api/v1/metrics/*` — Kafka, Redis, system health, nerd stats
- `/api/v1/pipeline/*` — Kafka topic/message browser, Redis key browser, SSE connection info
- `/api/v1/admin/*` — simulation control (rate, scenario, chaos mode, thresholds)
- `/api/v1/session/*` — per-session sensor config

Rate limiting is applied via Redis to all `/api/v1` routes (`internal/middleware/ratelimit.go`).

## Data Models

PostgreSQL schema is in `migrations/` (auto-applied by Docker on first start):
- `sensors` — sensor registry with UUID PK
- `sensor_readings` — partitioned by month on `timestamp`; partition management in `migrations/003_create_partitions.sql` (add new partitions for upcoming months manually)
- `sensor_aggregates` — hourly/daily pre-computed aggregates

Redis key patterns (see `internal/redis/operations.go`):
- `sensor:latest:{uuid}` — hash of latest reading (short TTL)
- `sensor:stream:{uuid}` — Redis stream, capped at 1000 entries
- `pollution:leaderboard` — sorted set by PM2.5 value
- `simulation:config` — JSON of `SimulationConfig` (polled by simulator)
- `scenario:active` — JSON of active `Scenario` (polled by simulator)
- `simulation:commands` — Pub/Sub channel for immediate sensor control

## Troubleshooting

- **Kafka not ready**: Wait 15–20s after `docker compose up` before starting Go services.
- **Partition errors**: Add partitions for the current/next month in `migrations/003_create_partitions.sql` and re-apply, or run the SQL manually via Adminer.
- **Simulator not responding to admin commands**: Check that `simulation:commands` Pub/Sub channel is working; Redis Pub/Sub listener runs in the simulator's `listenForCommands` goroutine.
