# 🏙️ Smart City Environmental Monitor

> Real-time IoT sensor simulation platform showcasing Full-Stack + Golang + Kafka + Redis + Kubernetes

**Status**: 🟡 In Development  
**Started**: November 19, 2024  
**Target**: 5 weeks

---

## 📖 Quick Links

- [What It Does](#what-it-does)
- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
- [Getting Started](#getting-started)
- [Development Phases](#development-phases)
- [API Documentation](#api-documentation)
- [Progress](#progress)

---

## 🎯 What It Does

Simulates 50+ environmental sensors across a virtual smart city, displaying real-time data on:
- 🌡️ Temperature
- 🌫️ Air Quality (PM2.5)
- 💧 Humidity  
- 🔊 Noise Levels

**User Features**:
- Interactive map with live sensor markers
- Real-time charts and analytics
- Alert system for threshold breaches
- Admin panel to control simulation scenarios (rush hour, heatwave, etc.)

**Why This Project**:
- Demonstrates microservices architecture
- Showcases event-driven design with Kafka
- Proves real-time data handling at scale
- Full-stack + DevOps in one project

---

## 🛠️ Tech Stack

### Backend
- **Language**: Go 1.21+
- **Framework**: Fiber/Gin
- **Message Queue**: Apache Kafka
- **Cache**: Redis
- **Database**: PostgreSQL

### Frontend
- **Framework**: React 18
- **Maps**: Leaflet
- **Charts**: Recharts
- **Real-time**: WebSockets

### Infrastructure
- **Containers**: Docker
- **Orchestration**: Kubernetes
- **Development**: Docker Compose

---

## 🏗️ Architecture
```
Frontend (React + WebSocket)
    ↓
API Gateway (Go)
    ↓
├─ Sensor Simulator → Kafka → Data Ingestion → PostgreSQL + Redis
├─ Analytics Service → Aggregations
└─ Admin Service → Control Commands
    ↓
Real-time updates via Redis Pub/Sub → WebSocket → Frontend
```

### Services

1. **Sensor Simulator** - Generates realistic sensor data → Kafka
2. **Data Ingestion** - Consumes Kafka → Stores in PostgreSQL + Redis
3. **Analytics Service** - Computes aggregations and detects alerts
4. **API Gateway** - REST API + WebSocket server
5. **Admin Service** - Controls simulation via Kafka commands

### Data Flow
```
Sensor Data → Kafka Topics → Consumer Groups → Storage (PostgreSQL + Redis) → WebSocket → UI
```

---

## 🚀 Getting Started

### Prerequisites
```bash
- Docker Desktop (required)
- Go 1.21+ (required for backend)
- Node.js 18+ (for frontend - Week 3)
```

### Quick Start (Automated)

The easiest way to get started:

```bash
# 1. Start all services
./scripts/start-all.sh

# 2. Monitor the system
./scripts/monitor.sh

# 3. Stop all services
./scripts/stop-all.sh
```

### Manual Setup

If you prefer manual control:

```bash
# 1. Start Docker infrastructure
docker compose up -d

# Wait ~15 seconds for services to be ready

# 2. Create Kafka topics
docker exec kafka kafka-topics --bootstrap-server localhost:9093 \
  --create --topic sensor-readings --partitions 6 --replication-factor 1 --if-not-exists

docker exec kafka kafka-topics --bootstrap-server localhost:9093 \
  --create --topic admin-commands --partitions 3 --replication-factor 1 --if-not-exists

docker exec kafka kafka-topics --bootstrap-server localhost:9093 \
  --create --topic alerts --partitions 3 --replication-factor 1 --if-not-exists

# 3. Run backend services (in separate terminals)

# Terminal 1: Sensor Simulator
cd backend
go run cmd/sensor-simulator/main.go

# Terminal 2: Data Ingestion
cd backend
go run cmd/data-ingestion/main.go

# 4. Access Web UIs
# Kafka UI:        http://localhost:8081
# Redis Commander: http://localhost:8082
```

### Environment Variables

The `.env` file is already configured with defaults. Key settings:

```env
# Kafka
KAFKA_BROKERS=localhost:9092

# Redis
REDIS_ADDR=localhost:6379

# PostgreSQL (Note: port 5433 to avoid conflicts)
POSTGRES_HOST=127.0.0.1
POSTGRES_PORT=5433
POSTGRES_DB=smart_city
POSTGRES_USER=admin
POSTGRES_PASSWORD=password

# Simulator
SENSOR_COUNT=50
GENERATION_INTERVAL_MS=1000

# Application
LOG_LEVEL=info
ENVIRONMENT=development
```

---

## 📅 Development Phases

### Phase 1: Foundation (Week 1) - ✅ COMPLETE
- [x] Project structure setup
- [x] Docker Compose environment (Kafka, Redis, PostgreSQL, Zookeeper)
- [x] PostgreSQL schema with monthly partitioning
- [x] Sensor simulator (Kafka producer) - 50 sensors, realistic data patterns
- [x] Data ingestion (Kafka consumer → PostgreSQL + Redis batch processing)
- [x] Structured logging with zerolog
- [x] Monitoring queries and live dashboard
- [x] Unit tests for all components
- [x] Helper scripts (start-all, stop-all, monitor, load-test)
- [x] Data persistence with Docker volumes

**Goal**: Data flows from simulator → Kafka → storage ✅ **ACHIEVED**

**Performance**:
- 50 sensors @ 1 reading/sec
- ~24 readings/sec throughput with batch processing
- Zero data loss
- Full persistence across restarts

---

### Phase 2: API & Real-Time (Week 2)
- [ ] REST API endpoints
- [ ] WebSocket server
- [ ] Redis Pub/Sub integration
- [ ] Basic analytics service

**Goal**: API + WebSocket streaming live data

---

### Phase 3: Frontend (Week 3)
- [ ] React app with Leaflet map
- [ ] WebSocket client integration
- [ ] Real-time sensor markers
- [ ] City stats cards
- [ ] Sensor detail panel
- [ ] Alerts panel

**Goal**: Functional web app with live map

---

### Phase 4: Analytics & Admin (Week 4)
- [ ] Analytics dashboard
- [ ] Historical trends
- [ ] Alert detection system
- [ ] Admin panel UI
- [ ] Scenario implementations (rush hour, heatwave, etc.)

**Goal**: Complete all features

---

### Phase 5: Kubernetes & Polish (Week 5)
- [ ] Dockerize all services
- [ ] Kubernetes manifests
- [ ] Horizontal Pod Autoscaling
- [ ] Health checks
- [ ] UI/UX polish
- [ ] Documentation

**Goal**: Production-ready K8s deployment

---

## 🗄️ Data Models

### PostgreSQL Schema
```sql
-- Sensors
CREATE TABLE sensors (
    id UUID PRIMARY KEY,
    name VARCHAR(100),
    type VARCHAR(50), -- temperature, pollution, humidity, noise
    location GEOGRAPHY(POINT, 4326),
    status VARCHAR(20),
    config JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- Sensor Readings (partitioned by month)
CREATE TABLE sensor_readings (
    id BIGSERIAL,
    sensor_id UUID REFERENCES sensors(id),
    sensor_type VARCHAR(50),
    value DECIMAL(10, 2),
    unit VARCHAR(20),
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    timestamp TIMESTAMP NOT NULL,
    PRIMARY KEY (timestamp, sensor_id)
) PARTITION BY RANGE (timestamp);

-- Aggregates
CREATE TABLE sensor_aggregates (
    id BIGSERIAL PRIMARY KEY,
    sensor_id UUID,
    sensor_type VARCHAR(50),
    aggregation_type VARCHAR(20), -- hourly, daily
    avg_value DECIMAL(10, 2),
    min_value DECIMAL(10, 2),
    max_value DECIMAL(10, 2),
    period_start TIMESTAMP,
    period_end TIMESTAMP
);

-- Alerts
CREATE TABLE alerts (
    id BIGSERIAL PRIMARY KEY,
    sensor_id UUID REFERENCES sensors(id),
    alert_type VARCHAR(50),
    severity VARCHAR(20),
    message TEXT,
    value DECIMAL(10, 2),
    threshold DECIMAL(10, 2),
    timestamp TIMESTAMP,
    acknowledged BOOLEAN DEFAULT FALSE
);
```

### Redis Structures
```redis
# Latest readings
HSET sensor:latest:{id} value 23.5 timestamp "..." unit "celsius"

# Geospatial index
GEOADD sensors:geo -74.0060 40.7128 {sensor_id}

# Time-series stream
XADD sensor:stream:{id} MAXLEN ~ 1000 * value 23.5 timestamp "..."

# City stats (cached 5min)
SET city:stats "{...}" EX 300

# Leaderboard
ZADD pollution:leaderboard 85.5 {sensor_id}

# Pub/Sub
PUBLISH sensor:updates "{...}"
```

### Kafka Message Schema
```json
{
  "sensor_id": "uuid",
  "sensor_type": "temperature",
  "value": 23.5,
  "unit": "celsius",
  "location": {
    "latitude": 40.7128,
    "longitude": -74.0060
  },
  "timestamp": "2024-11-19T10:30:00Z"
}
```

---

## 🌐 API Documentation

### Base URL: `http://localhost:8080/api`

#### Sensors
```http
GET /api/sensors
GET /api/sensors/:id
GET /api/sensors/:id/readings?from=<timestamp>&to=<timestamp>
```

#### Analytics
```http
GET /api/analytics/city-stats
GET /api/analytics/zones/compare?zones=downtown,industrial
GET /api/analytics/top-polluted?limit=5
```

#### Alerts
```http
GET /api/alerts?active=true
POST /api/alerts/:id/acknowledge
```

#### Admin
```http
POST /api/admin/sensors (create)
POST /api/admin/sensors/:id/control (start/stop)
POST /api/admin/scenario (change scenario)
```

#### WebSocket
```javascript
// Connect
ws://localhost:8080/ws

// Subscribe
{ "action": "subscribe", "sensor_id": "uuid" }

// Receive updates
{ "type": "sensor_update", "data": {...} }
```

---

## 🐳 Deployment

### Docker Compose
```bash
docker-compose up -d          # Start
docker-compose logs -f        # View logs
docker-compose down           # Stop
```

### Kubernetes
```bash
kubectl apply -f k8s/ -n smart-city
kubectl get pods -n smart-city
kubectl scale deployment data-ingestion --replicas=5
kubectl port-forward svc/api-gateway 8080:8080
```

---

## 📊 Progress

### Current Phase: **Phase 2 - API & Real-Time** (Starting Week 2)

**Week 1 Completed** (Days 1-5):
- [x] Project structure and configuration
- [x] Docker Compose with Kafka, Redis, PostgreSQL, Zookeeper
- [x] PostgreSQL schema with partitioning (50 sensors seeded)
- [x] Sensor simulator with realistic data generation
- [x] Data ingestion service with batch processing
- [x] Structured logging (zerolog)
- [x] Comprehensive monitoring queries
- [x] Unit tests (9 tests passing)
- [x] Helper scripts (start-all, stop-all, monitor, load-test)
- [x] Load testing and performance documentation
- [x] Full data persistence

**Week 1 Achievements**:
- ✅ Complete data pipeline working (Simulator → Kafka → PostgreSQL + Redis)
- ✅ 3,500+ sensor readings successfully processed
- ✅ Zero data loss with Docker volume persistence
- ✅ Real-time Redis cache with geospatial indexing
- ✅ Monitoring dashboard and SQL queries

**Next (Week 2)**:
- [ ] REST API with Fiber framework
- [ ] WebSocket server for real-time updates
- [ ] Analytics service
- [ ] Redis Pub/Sub integration

**Blockers**: None

**Documentation**:
- See [docs/load-test-results.md](docs/load-test-results.md) for performance metrics
- See [docs/optimizations.md](docs/optimizations.md) for future improvements
- See [week1-tasks.md](week1-tasks.md) for detailed task breakdown

---

## 📚 Resources

- [Kafka Docs](https://kafka.apache.org/documentation/)
- [Redis Docs](https://redis.io/docs/)
- [PostgreSQL Docs](https://www.postgresql.org/docs/)
- [Kubernetes Docs](https://kubernetes.io/docs/)

---

## 🔧 Available Scripts

Located in `scripts/` directory:

- **`start-all.sh`** - Start all Docker services and backend applications
- **`stop-all.sh`** - Gracefully stop all services
- **`monitor.sh`** - Real-time monitoring dashboard (refreshes every 3s)
- **`load-test.sh`** - Run load tests with 50/100/200/500 sensors
- **`test-pipeline.sh`** - Quick test of complete data pipeline
- **`monitoring-queries.sql`** - Comprehensive SQL queries for system monitoring

### Testing

```bash
# Run all unit tests
cd backend
go test ./internal/... -v

# Run specific package tests
go test ./internal/postgres -v
go test ./internal/redis -v
go test ./internal/kafka -v

# Test complete pipeline
./scripts/test-pipeline.sh
```

---

## 📝 Notes

### Current Limitations
- PostgreSQL on port 5433 (to avoid conflicts with local postgres on 5432)
- 50 sensors seeded in database
- Batch processing causes ~50% throughput (see [optimizations.md](docs/optimizations.md))

### Troubleshooting
- **Kafka connection refused**: Wait 15-20 seconds after `docker compose up` for Kafka to be ready
- **PostgreSQL partition error**: Check that partitions exist for current month (see `migrations/003_create_partitions.sql`)
- **Redis keys not updating**: Verify data ingestion service is running

### Useful Commands

```bash
# Check Docker services
docker ps

# View Kafka topics
docker exec kafka kafka-topics --bootstrap-server localhost:9093 --list

# Check PostgreSQL data
docker exec postgres psql -U admin -d smart_city -c "SELECT COUNT(*) FROM sensor_readings;"

# Check Redis keys
docker exec redis redis-cli DBSIZE

# View logs
docker logs kafka -f
docker logs postgres -f
docker logs redis -f
```

### Web UIs
- **Kafka UI**: http://localhost:8081 - View topics, messages, consumer groups
- **Redis Commander**: http://localhost:8082 - Browse Redis keys and values

---

**Last Updated**: November 19, 2025
**Version**: 1.1
**Status**: Week 1 Complete ✅