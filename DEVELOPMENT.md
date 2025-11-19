# Development Guide - Smart City Environmental Monitor

This document provides detailed information about the system architecture, data flow, and development practices.

---

## Table of Contents

- [System Architecture](#system-architecture)
- [Data Flow](#data-flow)
- [Component Details](#component-details)
- [Database Schema](#database-schema)
- [Redis Data Structures](#redis-data-structures)
- [Kafka Topics](#kafka-topics)
- [Development Workflow](#development-workflow)
- [Testing Strategy](#testing-strategy)
- [Performance Considerations](#performance-considerations)

---

## System Architecture

### High-Level Overview

```
┌─────────────────┐
│  Sensor         │
│  Simulator      │
│  (Go)           │
└────────┬────────┘
         │ Produces
         ↓
┌─────────────────┐
│   Apache        │
│   Kafka         │◄───────────────┐
│  (6 partitions) │                │
└────────┬────────┘                │
         │ Consumes               │
         ↓                         │
┌─────────────────┐                │
│  Data           │                │
│  Ingestion      │                │
│  (Go)           │                │
└────────┬────────┘                │
         │                         │
         ├──────────┬──────────────┤
         │          │              │
         ↓          ↓              ↓
┌─────────────┐ ┌────────┐ ┌──────────┐
│ PostgreSQL  │ │ Redis  │ │ Redis    │
│ (Partitioned│ │ Cache  │ │ Pub/Sub  │
│  by month)  │ │        │ │          │
└─────────────┘ └────────┘ └──────────┘
         │          │              │
         └──────────┴──────────────┘
                    │
         (Future: API Gateway + WebSocket)
                    │
                    ↓
            ┌──────────────┐
            │  Frontend    │
            │  (React)     │
            └──────────────┘
```

### Component Interaction

1. **Sensor Simulator** generates realistic sensor data
2. **Kafka** acts as message queue and decouples components
3. **Data Ingestion** processes messages in batches
4. **PostgreSQL** stores historical data (partitioned for performance)
5. **Redis** provides fast access to latest readings and caching
6. **Redis Pub/Sub** will enable real-time WebSocket updates (Week 2)

---

## Data Flow

### Detailed Message Flow

```
1. Sensor Simulator
   ├─ Loads 50 sensors from PostgreSQL
   ├─ Generates reading every 1 second per sensor
   ├─ Applies realistic patterns (time-of-day, rush hour)
   └─ Produces to Kafka topic: sensor-readings

2. Kafka
   ├─ Receives messages on 6 partitions
   ├─ Persists to disk (with volume persistence)
   └─ Makes available to consumer groups

3. Data Ingestion Consumer
   ├─ Consumes from sensor-readings topic
   ├─ Accumulates messages in batch (100 or 1 second)
   ├─ Writes batch to PostgreSQL (INSERT batch)
   └─ Updates Redis immediately:
       ├─ HSET sensor:latest:{id} (current reading)
       ├─ XADD sensor:stream:{id} (time-series)
       ├─ ZADD pollution:leaderboard (if pollution sensor)
       └─ PUBLISH sensor:updates (for future WebSocket)

4. Storage
   ├─ PostgreSQL: Long-term storage, queryable
   └─ Redis: Fast access, caching, real-time
```

### Message Format

Kafka messages use JSON format:

```json
{
  "sensor_id": "084002f2-d543-476f-8709-5e87a0c00f3f",
  "sensor_type": "temperature",
  "value": 25.34,
  "unit": "celsius",
  "location": {
    "latitude": 40.7589,
    "longitude": -73.9851
  },
  "timestamp": "2025-11-19T12:30:45Z"
}
```

---

## Component Details

### 1. Sensor Simulator

**Location**: `backend/cmd/sensor-simulator/main.go`

**Responsibilities**:
- Load sensors from PostgreSQL on startup
- Generate realistic sensor readings based on time of day
- Send messages to Kafka at configured interval

**Data Generation Logic**:

```go
// Temperature: 15-35°C with daily variation peaking at 2 PM
baseTemp := 20.0
dailyVariation := 8.0 * sin(2π * (hour - 6) / 24)
value = baseTemp + dailyVariation + randomNoise

// Pollution: Higher during rush hours (7-9 AM, 5-7 PM)
basePollution := 40.0
if rushHour {
    rushHourFactor = 40.0
}
value = basePollution + rushHourFactor + randomNoise

// Humidity: 30-80% with morning peak
baseHumidity := 55.0
dailyVariation := 15.0 * sin(2π * (hour - 3) / 24)

// Noise: 40-90 dB, higher during day and rush hours
baseNoise := 55.0
if daytime { dayFactor = 15.0 }
if rushHour { rushFactor = 10.0 }
```

**Configuration**:
- `SENSOR_COUNT`: Number of sensors to simulate
- `GENERATION_INTERVAL_MS`: Milliseconds between readings
- `LOG_LEVEL`: Logging verbosity

### 2. Data Ingestion Service

**Location**: `backend/cmd/data-ingestion/main.go`

**Responsibilities**:
- Consume messages from Kafka
- Batch messages for efficient database writes
- Update Redis with latest readings
- Handle errors gracefully

**Batch Processing**:

```go
type IngestionService struct {
    db          *postgres.DB
    redis       *redis.Client
    batchSize   int           // 100 messages
    batchBuffer []SensorReading
    mu          *sync.Mutex
}

// Flush conditions:
// 1. Buffer reaches batchSize (100 messages)
// 2. Ticker fires (every 1 second)
```

**Redis Operations per Message**:
1. `HSET sensor:latest:{id}` - Latest reading (1 hour TTL)
2. `XADD sensor:stream:{id}` - Time-series stream (max 1000 entries)
3. `ZADD pollution:leaderboard` - Sorted set (if pollution sensor)
4. `PUBLISH sensor:updates` - Pub/Sub for real-time clients

**Performance Optimization**:
- Batch PostgreSQL writes reduce transaction overhead
- Redis operations are non-blocking
- Graceful shutdown flushes remaining messages

### 3. PostgreSQL Database

**Connection Pooling**:
```go
MaxConnections:     25
MaxIdleConnections: 5
```

**Partitioning Strategy**:
- Table `sensor_readings` partitioned by `timestamp` (monthly)
- Partitions created: 2024-11, 2024-12, 2025-01, 2025-02, 2025-11, 2025-12
- Future: Automatic partition creation via cron job

**Indexes**:
```sql
-- Primary key on partitioned table
PRIMARY KEY (timestamp, sensor_id)

-- Index on sensor_type for filtered queries
CREATE INDEX idx_sensor_readings_type ON sensor_readings(sensor_type);

-- Index on timestamp for time-range queries
CREATE INDEX idx_sensor_readings_timestamp ON sensor_readings(timestamp DESC);
```

### 4. Redis Cache

**Data Structures Used**:

| Structure | Key Pattern | Purpose | TTL |
|-----------|------------|---------|-----|
| Hash | `sensor:latest:{id}` | Latest reading | 1 hour |
| Stream | `sensor:stream:{id}` | Time-series data | MaxLen 1000 |
| Sorted Set | `pollution:leaderboard` | Top polluted areas | None |
| GeoSpatial | `sensors:geo` | Location-based queries | None |
| String | `city:stats` | Aggregated city stats | 5 min |
| Pub/Sub | `sensor:updates` | Real-time notifications | N/A |

**Persistence**:
- AOF (Append-Only File) enabled
- Docker volume: `redis-data:/data`

### 5. Apache Kafka

**Topics**:

| Topic | Partitions | Replication | Purpose |
|-------|-----------|-------------|---------|
| sensor-readings | 6 | 1 | Sensor data from simulator |
| admin-commands | 3 | 1 | Control commands (future) |
| alerts | 3 | 1 | Alert notifications (future) |

**Consumer Groups**:
- `data-ingestion-group` - Processes sensor readings
- `analytics-group` - Future analytics processing

**Configuration**:
```yaml
KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
KAFKA_HEAP_OPTS: "-Xmx512M -Xms256M"
```

---

## Database Schema

### Sensors Table

```sql
CREATE TABLE sensors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('temperature', 'pollution', 'humidity', 'noise')),
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

**Sample Data**:
- 50 sensors seeded
- Distributed across NYC-like grid (40.70-40.80 lat, -74.02 to -73.92 lon)
- Mix: 12 temperature, 14 pollution, 12 humidity, 12 noise

### Sensor Readings Table (Partitioned)

```sql
CREATE TABLE sensor_readings (
    id BIGSERIAL,
    sensor_id UUID NOT NULL,
    sensor_type VARCHAR(50) NOT NULL,
    value DECIMAL(10, 2) NOT NULL,
    unit VARCHAR(20) NOT NULL,
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (timestamp, sensor_id)
) PARTITION BY RANGE (timestamp);
```

### Aggregates Table (Future)

```sql
CREATE TABLE sensor_aggregates (
    id BIGSERIAL PRIMARY KEY,
    sensor_id UUID NOT NULL,
    sensor_type VARCHAR(50) NOT NULL,
    aggregation_type VARCHAR(20) NOT NULL, -- 'hourly', 'daily'
    avg_value DECIMAL(10, 2),
    min_value DECIMAL(10, 2),
    max_value DECIMAL(10, 2),
    stddev_value DECIMAL(10, 2),
    period_start TIMESTAMP NOT NULL,
    period_end TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### Alerts Table (Future)

```sql
CREATE TABLE alerts (
    id BIGSERIAL PRIMARY KEY,
    sensor_id UUID NOT NULL REFERENCES sensors(id),
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL, -- 'low', 'medium', 'high', 'critical'
    message TEXT NOT NULL,
    value DECIMAL(10, 2),
    threshold DECIMAL(10, 2),
    timestamp TIMESTAMP NOT NULL,
    acknowledged BOOLEAN DEFAULT FALSE,
    acknowledged_at TIMESTAMP,
    acknowledged_by VARCHAR(100)
);
```

---

## Redis Data Structures

### Latest Reading (Hash)

```redis
HSET sensor:latest:084002f2-d543-476f-8709-5e87a0c00f3f
    sensor_id "084002f2-d543-476f-8709-5e87a0c00f3f"
    sensor_type "temperature"
    value "25.34"
    unit "celsius"
    latitude "40.7589"
    longitude "-73.9851"
    timestamp "1700395845"

EXPIRE sensor:latest:084002f2-d543-476f-8709-5e87a0c00f3f 3600
```

### Time-Series Stream

```redis
XADD sensor:stream:084002f2-d543-476f-8709-5e87a0c00f3f MAXLEN ~ 1000 *
    value "25.34"
    timestamp "1700395845"
```

### Geospatial Index

```redis
GEOADD sensors:geo -73.9851 40.7589 084002f2-d543-476f-8709-5e87a0c00f3f

# Query nearby sensors
GEORADIUS sensors:geo -74.0 40.75 5 km
```

### Pollution Leaderboard

```redis
ZADD pollution:leaderboard 85.5 sensor-id-1
ZADD pollution:leaderboard 92.3 sensor-id-2

# Get top 10 most polluted
ZREVRANGE pollution:leaderboard 0 9 WITHSCORES
```

---

## Kafka Topics

### sensor-readings

**Purpose**: Sensor data from simulator
**Partitions**: 6 (allows parallel processing)
**Format**: JSON

### admin-commands (Future)

**Purpose**: Control commands (start/stop sensors, change scenarios)
**Partitions**: 3
**Format**: JSON

```json
{
  "command": "change_scenario",
  "scenario": "rush_hour",
  "timestamp": "2025-11-19T12:00:00Z"
}
```

### alerts (Future)

**Purpose**: Alert notifications
**Partitions**: 3
**Format**: JSON

```json
{
  "alert_id": "uuid",
  "sensor_id": "uuid",
  "alert_type": "high_pollution",
  "severity": "high",
  "value": 150.5,
  "threshold": 100.0,
  "timestamp": "2025-11-19T12:00:00Z"
}
```

---

## Development Workflow

### Setting Up Development Environment

```bash
# 1. Clone and setup
git clone <repo-url>
cd smart-city-monitor

# 2. Install dependencies
cd backend
go mod download

# 3. Start infrastructure
docker compose up -d

# 4. Create Kafka topics
./scripts/create-topics.sh

# 5. Run services
go run cmd/sensor-simulator/main.go
go run cmd/data-ingestion/main.go
```

### Running Tests

```bash
# All tests
go test ./internal/... -v

# Specific package
go test ./internal/postgres -v -run TestBatchReadingsValidation

# With coverage
go test ./internal/... -cover

# Integration tests (requires running services)
go test ./internal/... -v -tags=integration
```

### Code Formatting

```bash
# Format all Go code
go fmt ./...

# Vet code
go vet ./...

# Run linter (if installed)
golangci-lint run
```

### Adding a New Service

1. Create directory: `backend/cmd/new-service/`
2. Create `main.go` with logger setup
3. Add dependencies to `go.mod`
4. Create tests in `backend/cmd/new-service/main_test.go`
5. Update docker-compose.yml if needed
6. Document in README.md

---

## Testing Strategy

### Unit Tests

**Coverage**: Core business logic, data structures, utilities

```go
// Example: backend/internal/postgres/queries_test.go
func TestBatchReadingsValidation(t *testing.T) {
    readings := make([]models.SensorReading, 100)
    // ... test logic
}
```

### Integration Tests

**Coverage**: Database operations, Redis operations, Kafka message flow

```go
// Example: Requires running PostgreSQL
func TestInsertSensorReading(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    // ... test with real database
}
```

### Load Tests

**Script**: `scripts/load-test.sh`

Tests system with:
- 50 sensors (baseline)
- 100 sensors
- 200 sensors
- 500 sensors

Measures:
- Throughput (readings/sec)
- Success rate
- Database write latency
- Redis operation latency

---

## Performance Considerations

### Current Performance

- **Throughput**: ~24 readings/sec (with 50 sensors @ 1/sec)
- **Batch Size**: 100 messages or 1 second
- **PostgreSQL Writes**: Batched for efficiency
- **Redis Operations**: < 10ms average

### Bottlenecks Identified

1. **Batch Processing Delay**: Conservative flush interval (1 sec)
2. **Synchronous PostgreSQL Writes**: Blocks consumer
3. **Single Consumer Instance**: No horizontal scaling yet

### Optimization Roadmap

See [docs/optimizations.md](docs/optimizations.md) for detailed optimization plan.

**Quick Wins**:
1. Increase batch size to 200-500
2. Reduce flush interval to 500ms
3. Add Redis pipelining for multiple operations

**Medium-term**:
1. Async PostgreSQL writes with goroutine pool
2. Horizontal scaling with multiple consumer instances
3. Add Prometheus metrics

---

## Logging

### Structured Logging with Zerolog

All services use structured logging:

```go
log.Info().
    Int("sensor_count", 50).
    Int("interval_ms", 1000).
    Msg("Starting simulation")

log.Error().
    Err(err).
    Str("sensor_id", id).
    Msg("Failed to send message to Kafka")
```

**Log Levels**:
- `DEBUG`: Detailed information for debugging
- `INFO`: General informational messages
- `WARN`: Warning messages (recoverable errors)
- `ERROR`: Error messages (requires attention)

**Configuration**:
- `LOG_LEVEL` in `.env` (default: "info")
- `ENVIRONMENT=development` enables pretty console output

---

## Monitoring

### Available Monitoring Tools

1. **Real-time Dashboard**: `./scripts/monitor.sh`
2. **SQL Queries**: `scripts/monitoring-queries.sql`
3. **Kafka UI**: http://localhost:8081
4. **Redis Commander**: http://localhost:8082

### Key Metrics to Monitor

- Kafka consumer lag
- PostgreSQL write throughput
- Redis operation latency
- Error rates by component
- Batch flush frequency

See [scripts/monitoring-queries.sql](scripts/monitoring-queries.sql) for comprehensive monitoring queries.

---

## Contributing

When contributing to this project:

1. **Follow Go conventions**: Use `go fmt`, `go vet`
2. **Write tests**: Aim for >70% coverage
3. **Update documentation**: Keep README and this file in sync
4. **Use structured logging**: Always use zerolog, not fmt.Println
5. **Handle errors properly**: Never silently ignore errors
6. **Add comments**: Explain why, not what

---

**Last Updated**: November 19, 2025
**Version**: 1.0
