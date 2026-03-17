# Smart City Monitor — Deployment

## Overview

Smart City Monitor supports three deployment modes: local Docker Compose for development, production deployment on EC2 with Caddy HTTPS, and Kubernetes for orchestrated scaling.

## Local Development (Docker Compose)

### Start Everything

```bash
./scripts/start-all.sh
```

Or manually:

```bash
docker compose up -d          # Infrastructure + backend services
cd frontend && npm run dev    # Frontend dev server at http://localhost:5173
```

### Stop Everything

```bash
docker compose down
```

### Service Ports

| Service | Port | URL |
|---------|------|-----|
| API Gateway | 8080 | http://localhost:8080 |
| Frontend (dev) | 5173 | http://localhost:5173 |
| PostgreSQL | 5433 | — |
| Redis | 6379 | — |
| Kafka | 9092, 9093 | — |
| Adminer | 8083 | http://localhost:8083 |

### Docker Compose Services

The `docker-compose.yml` includes:

- **kafka** — Confluent Kafka 7.5.0 in KRaft mode (no Zookeeper)
- **postgres** — PostgreSQL 16 with auto-applied migrations
- **redis** — Redis 7 with AOF persistence and 256MB memory cap
- **adminer** — PostgreSQL web UI
- **api-gateway** — Pre-built Go service container
- **data-ingestion** — Pre-built Go service container
- **sensor-simulator** — Pre-built Go service container

### Volumes

| Volume | Purpose |
|--------|---------|
| `postgres-data` | PostgreSQL data persistence |
| `redis-data` | Redis AOF persistence |
| `kafka-data` | Kafka log segments |

## Production Deployment

### Architecture

- **Frontend:** Deployed on Vercel (static build)
- **Backend:** Three Go services running in Docker on EC2 (t2.micro)
- **Database:** Neon PostgreSQL (managed, external)
- **Reverse Proxy:** Caddy with automatic HTTPS
- **Images:** Pre-built and pushed to Docker Hub

### EC2 Setup Scripts

Located in `scripts/`:

- Caddy configuration for HTTPS termination and reverse proxying to port 8080
- Docker Compose production override for EC2 environment
- Health check and monitoring scripts

### Docker Hub Images

Backend services are built as multi-stage Docker images and pushed to Docker Hub for EC2 deployment. Each service has its own Dockerfile:

- `backend/Dockerfile.api-gateway`
- `backend/Dockerfile.data-ingestion`
- `backend/Dockerfile.sensor-simulator`

## Kubernetes Deployment

### Apply Manifests

```bash
kubectl apply -f k8s/ -n smart-city
kubectl get pods -n smart-city
```

### Scaling

```bash
kubectl scale deployment data-ingestion --replicas=5
```

### Port Forward for Local Access

```bash
kubectl port-forward svc/api-gateway 8080:8080
```

## Environment Configuration

Configuration is loaded from `.env` in the project root. Each service validates required variables on startup via `cfg.Validate("service-name")`.

### Key Variables

| Category | Variables |
|----------|-----------|
| PostgreSQL | `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_SSLMODE` |
| Redis | `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB` |
| Kafka | `KAFKA_BROKERS`, `KAFKA_TOPIC_SENSOR_READINGS`, `KAFKA_TOPIC_DLQ`, `KAFKA_TOPIC_ANOMALIES`, `KAFKA_TOPIC_EVENTS` |
| Resilience | `CB_THRESHOLD`, `CB_TIMEOUT_SECONDS`, `MAX_RETRIES`, `RETRY_INITIAL_BACKOFF_MS` |
| Simulator | `SENSOR_COUNT`, `GENERATION_INTERVAL_MS`, `SIMULATOR_START_EMPTY` |
| Frontend | `VITE_API_URL`, `VITE_GOOGLE_MAPS_API_KEY` |

## Health Checks

The API Gateway exposes a health endpoint that checks all dependencies:

```
GET /health
```

Response includes connectivity status for PostgreSQL, Redis, and Kafka. Used by Docker Compose healthcheck and Kubernetes liveness probes.
