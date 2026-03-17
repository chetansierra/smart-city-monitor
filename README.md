# Smart City Environmental Monitor

Real-time IoT sensor simulation platform built with Go, Kafka, Redis, PostgreSQL, and React.

## What It Does

Simulates 50+ environmental sensors across a virtual smart city, streaming live data to a React frontend via Server-Sent Events. Includes production-grade resilience (circuit breakers, retry logic, dead letter queues), real-time anomaly detection, and zone-level pattern detection.

**Sensor types:** Temperature, Pollution (PM2.5), Humidity, Noise

**Key features:**
- Interactive Google Maps with live sensor markers
- Real-time anomaly detection (Welford's algorithm, z-score > 2.5)
- Zone-level pattern detection (heatwave, rush_hour, industrial_incident)
- Circuit breakers, retry with exponential backoff, dead letter queue
- Pre-computed analytics with hourly aggregates
- Admin panel with scenario simulation controls
- Data pipeline visualization (Kafka, Redis, SSE browser)
- Session-scoped sensors with auto-cleanup

## Architecture

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

Three Go backend services share `backend/internal/` packages:

| Service | Role |
|---------|------|
| **Sensor Simulator** | Generates readings, produces to Kafka, circuit breaker on producer |
| **Data Ingestion** | Consumes Kafka, anomaly/pattern detection, writes aggregates to PostgreSQL |
| **API Gateway** | REST API + SSE, consumes 3 Kafka topics, session management |

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Go, Fiber framework |
| Frontend | React 19, TypeScript, Vite, Google Maps API |
| Messaging | Apache Kafka (KRaft mode, no Zookeeper) |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| Real-time | Server-Sent Events (SSE) |
| Containers | Docker, Docker Compose |
| Orchestration | Kubernetes (manifests in `k8s/`) |

## Quick Start

```bash
# Start all infrastructure + backend services
./scripts/start-all.sh

# Start frontend (separate terminal)
cd frontend
npm install
npm run dev
```

Frontend: http://localhost:5173
API: http://localhost:8080
Adminer (DB UI): http://localhost:8083

### Manual Setup

```bash
docker compose up -d          # Start Kafka, PostgreSQL, Redis, backend services
# Wait ~15 seconds for Kafka to be ready

cd frontend
npm install
npm run dev                   # Dev server at http://localhost:5173
```

## Development

### Backend (from `backend/`)

```bash
go run cmd/sensor-simulator/main.go
go run cmd/data-ingestion/main.go
go run cmd/api-gateway/main.go

go test ./internal/... -v     # Unit tests
go test ./tests/integration/... -v  # Integration tests (requires running API)
```

### Frontend (from `frontend/`)

```bash
npm run dev       # Dev server
npm run build     # Production build
npm run lint      # ESLint
```

### Environment

Configuration in `.env` at project root. Key settings:

| Variable | Default |
|----------|---------|
| `KAFKA_BROKERS` | localhost:9092 |
| `REDIS_ADDR` | localhost:6379 |
| `POSTGRES_PORT` | 5433 |
| `SENSOR_COUNT` | 50 |
| `GENERATION_INTERVAL_MS` | 1000 |

## Documentation

- [Architecture Overview](docs/architecture/overview.md)
- [API Reference](docs/API.md)
- [Service Docs](docs/architecture/services/)
- [Technology Docs](docs/architecture/technologies/)
- [Resilience Patterns](docs/architecture/resilience.md)
- [Anomaly & Pattern Detection](docs/architecture/intelligence.md)
- [Deployment](docs/architecture/deployment.md)
- [Postman Guide](docs/POSTMAN_GUIDE.md)
- [Testing Guide](backend/tests/README.md)

## API Routes

Base: `http://localhost:8080`

| Group | Prefix | Description |
|-------|--------|-------------|
| Health | `/health` | Service health with dependency checks |
| SSE | `/stream` | Real-time event stream |
| Sensors | `/api/v1/sensors` | Sensor CRUD and readings |
| Session | `/api/v1/session` | Per-session configuration |
| Readings | `/api/v1/readings` | Bulk reading queries |
| Analytics | `/api/v1/analytics` | City stats, zones, anomalies, events |
| Metrics | `/api/v1/metrics` | Kafka, Redis, system metrics |
| Pipeline | `/api/v1/pipeline` | Kafka/Redis/DLQ browser |
| Admin | `/api/v1/admin` | Simulation control, scenarios |

Full route details: [API Reference](docs/API.md)

## Kafka Topics

| Topic | Partitions | Purpose |
|-------|------------|---------|
| `sensor-readings` | 6 | Main sensor data pipeline |
| `sensor-anomalies` | 3 | Anomaly detection events |
| `sensor-events` | 3 | Zone-level pattern detections |
| `sensor-readings-dlq` | 1 | Dead letter queue |

## PostgreSQL Tables

| Table | Status | Purpose |
|-------|--------|---------|
| `sensors` | Active | Sensor registry |
| `sensor_aggregates` | Active | Pre-computed hourly/daily aggregates |
| `anomaly_events` | Active | Z-score anomaly detections |
| `detected_events` | Active | Zone-level pattern detections |
| `sensor_readings` | Legacy | NOT written to; raw readings are not persisted |

## Troubleshooting

- **Kafka not ready:** Wait 15–20s after `docker compose up`.
- **No anomalies:** Anomaly detection requires 30+ readings per sensor. Wait a few minutes.
- **Missing aggregates:** Aggregation worker runs every 5 minutes.
- **DLQ messages:** Browse via `GET /api/v1/pipeline/dlq?limit=10`.
- **Circuit breaker open:** Check logs for `circuit breaker opened`. Auto-recovers after 30s.
