# 📁 Smart City Monitor - Project Structure

> Complete file tree with purpose and responsibility of each component

**Last Updated**: November 19, 2025
**Version**: 1.0 (Week 1 Complete)

---

## 📂 Root Directory

```
smart-city-monitor/
├── backend/                    # Go backend services
├── frontend/                   # React frontend (Week 3+)
├── k8s/                       # Kubernetes manifests (Week 5)
├── migrations/                # PostgreSQL database migrations
├── scripts/                   # Helper scripts for operations
├── docs/                      # Documentation files
├── docker-compose.yml         # Local development infrastructure
├── .env                       # Environment variables
├── .gitignore                 # Git ignore patterns
├── README.md                  # Project overview and quick start
├── DEVELOPMENT.md             # Detailed technical documentation
├── PROJECT-STRUCTURE.md       # This file - complete project tree
└── week1-tasks.md            # Week 1 task tracker
```

---

## 🔧 Backend (`/backend`)

### Directory Structure
```
backend/
├── cmd/                       # Application entry points (executables)
│   ├── sensor-simulator/
│   ├── data-ingestion/
│   ├── analytics/            # Week 2+
│   ├── api-gateway/          # Week 2+
│   └── test-db/
├── internal/                  # Internal packages (not importable externally)
│   ├── config/
│   ├── models/
│   ├── postgres/
│   ├── redis/
│   ├── kafka/
│   └── logger/
├── pkg/                       # Public packages (importable externally)
├── go.mod                     # Go module definition
└── go.sum                     # Go module checksums
```

---

### `/backend/cmd` - Application Entry Points

Each subdirectory contains a `main.go` file that serves as an executable service.

#### `cmd/sensor-simulator/main.go`
**Purpose**: Simulate 50 environmental sensors across a virtual smart city
**Responsibilities**:
- Load sensor configurations from PostgreSQL on startup
- Generate realistic sensor readings based on:
  - Time of day (temperature peaks at 2 PM)
  - Rush hour patterns (pollution/noise spikes at 7-9 AM, 5-7 PM)
  - Randomness for realistic variation
- Publish sensor readings to Kafka topic `sensor-readings`
- Run continuously at configurable interval (default: 1 second per sensor)
- Graceful shutdown on SIGINT/SIGTERM

**Key Features**:
- Structured logging with zerolog
- Environment-based configuration
- Connection pooling for PostgreSQL
- Kafka producer with error handling

**Produces to Kafka**: `sensor-readings` topic (6 partitions)

---

#### `cmd/data-ingestion/main.go`
**Purpose**: Consume sensor data from Kafka and store in PostgreSQL + Redis
**Responsibilities**:
- Consume messages from Kafka topic `sensor-readings`
- Batch processing for efficient database writes:
  - Accumulate up to 100 messages OR 1 second (whichever comes first)
  - Batch insert into PostgreSQL
- Update Redis cache with latest readings:
  - `HSET sensor:latest:{id}` - Latest reading with 1-hour TTL
  - `XADD sensor:stream:{id}` - Time-series stream (max 1000 entries)
  - `ZADD pollution:leaderboard` - Sorted set of pollution levels
  - `GEOADD sensors:geo` - Geospatial index for location queries
  - `PUBLISH sensor:updates` - Pub/Sub for real-time clients
- Handle errors with retry logic
- Graceful shutdown (flush remaining messages)

**Key Features**:
- Consumer group: `data-ingestion-group`
- Batch processing with mutex-protected buffer
- Structured logging with context
- Metrics tracking (batch size, flush frequency)

**Consumes from Kafka**: `sensor-readings` topic
**Writes to**: PostgreSQL `sensor_readings` table, Redis (multiple data structures)

---

#### `cmd/analytics/main.go` *(Week 2+)*
**Purpose**: Real-time analytics and aggregations
**Planned Responsibilities**:
- Compute hourly/daily aggregations (avg, min, max, stddev)
- Detect anomalies and threshold breaches
- Generate alerts for abnormal readings
- Publish alerts to Kafka topic `alerts`

---

#### `cmd/api-gateway/main.go` *(Week 2+)*
**Purpose**: REST API and WebSocket server
**Planned Responsibilities**:
- REST endpoints for sensor data queries
- WebSocket server for real-time updates
- Subscribe to Redis Pub/Sub for sensor updates
- Broadcast to connected WebSocket clients
- Rate limiting and authentication

---

#### `cmd/test-db/main.go`
**Purpose**: Database connection testing utility
**Responsibilities**:
- Load configuration from environment
- Test PostgreSQL connection
- Verify connection pool settings
- Validate database accessibility
- Exit with success/failure code

**Usage**: `go run cmd/test-db/main.go`

---

### `/backend/internal` - Internal Packages

Packages in `internal/` are only importable by code within the `backend/` module.

#### `internal/config/config.go`
**Purpose**: Centralized configuration management
**Responsibilities**:
- Load environment variables from `.env` file
- Parse and validate configuration values
- Provide typed configuration structs:
  - `AppConfig` - Application settings (log level, environment)
  - `PostgresConfig` - Database connection settings
  - `RedisConfig` - Redis connection settings
  - `KafkaConfig` - Kafka broker and topic configuration
- Default values for missing environment variables
- Configuration validation on startup

**Key Structs**:
```go
type Config struct {
    App      AppConfig
    Postgres PostgresConfig
    Redis    RedisConfig
    Kafka    KafkaConfig
}
```

---

#### `internal/models/sensor.go`
**Purpose**: Core data models and types
**Responsibilities**:
- Define sensor types (temperature, pollution, humidity, noise)
- Sensor metadata struct (ID, name, type, location, status)
- Sensor reading struct (value, unit, timestamp, location)
- Kafka message format (JSON serialization)
- Validation methods

**Key Types**:
```go
type SensorType string
type Sensor struct { ID, Name, Type, Latitude, Longitude, ... }
type SensorReading struct { SensorID, Type, Value, Unit, Timestamp, ... }
type KafkaMessage struct { ... }
```

---

#### `internal/postgres/connection.go`
**Purpose**: PostgreSQL connection management
**Responsibilities**:
- Establish database connection using pgx driver
- Configure connection pool (max connections, idle connections)
- Health check and ping functionality
- Connection retry logic
- Graceful connection closure

**Key Functions**:
- `Connect(cfg Config) (*DB, error)` - Create connection pool
- `TestConnection(cfg Config) error` - Test connectivity
- `Close()` - Close connection pool

---

#### `internal/postgres/queries.go`
**Purpose**: PostgreSQL database operations
**Responsibilities**:
- `LoadSensors()` - Load all active sensors
- `InsertSensorReading()` - Insert single reading
- `BatchInsertReadings()` - Batch insert for performance
- Query helpers for analytics (future)
- Transaction management

**Key Operations**:
- Uses prepared statements for performance
- Parameterized queries to prevent SQL injection
- Batch inserts with `COPY` protocol (future optimization)

---

#### `internal/postgres/queries_test.go`
**Purpose**: Unit tests for PostgreSQL operations
**Tests**:
- Batch readings validation (100 readings)
- Data structure integrity
- UUID generation
- Sensor type validation
- Timestamp handling

**Run**: `go test ./internal/postgres -v`

---

#### `internal/redis/connection.go`
**Purpose**: Redis connection management
**Responsibilities**:
- Create Redis client using go-redis library
- Configure connection settings (address, password, DB)
- Health check (PING command)
- Connection retry logic
- Graceful disconnection

**Key Functions**:
- `Connect(cfg Config) (*redis.Client, error)`
- `TestConnection(cfg Config) error`
- `Close()`

---

#### `internal/redis/operations.go`
**Purpose**: Redis data operations
**Responsibilities**:
- `UpdateLatestReading()` - Store latest sensor value (HSET + EXPIRE)
- `AddToStream()` - Add to time-series stream (XADD with MAXLEN)
- `UpdateGeoIndex()` - Update geospatial index (GEOADD)
- `UpdatePollutionLeaderboard()` - Update sorted set (ZADD)
- `PublishUpdate()` - Publish to Pub/Sub channel (PUBLISH)
- Error handling and retry logic

**Redis Data Structures Used**:
- Hash: `sensor:latest:{id}` (TTL: 1 hour)
- Stream: `sensor:stream:{id}` (MaxLen: 1000)
- GeoSpatial: `sensors:geo`
- Sorted Set: `pollution:leaderboard`
- Pub/Sub: `sensor:updates` channel

---

#### `internal/redis/operations_test.go`
**Purpose**: Unit tests for Redis operations
**Tests**:
- Redis key format validation
- Data structure types
- TTL settings
- Stream max length
- Pub/Sub channel names

**Run**: `go test ./internal/redis -v`

---

#### `internal/kafka/producer.go`
**Purpose**: Kafka message producer
**Responsibilities**:
- Create Kafka producer with configuration
- Serialize messages to JSON
- Send messages to specified topic
- Handle delivery reports
- Error handling and retries
- Graceful shutdown (flush pending messages)

**Key Functions**:
- `NewProducer(brokers []string) (*Producer, error)`
- `SendMessage(topic string, message interface{}) error`
- `Close()`

---

#### `internal/kafka/consumer.go`
**Purpose**: Kafka message consumer
**Responsibilities**:
- Create Kafka consumer with consumer group
- Subscribe to topics
- Poll for messages
- Deserialize JSON messages
- Commit offsets (manual or automatic)
- Handle rebalancing
- Graceful shutdown

**Key Functions**:
- `NewConsumer(brokers []string, groupID string) (*Consumer, error)`
- `Subscribe(topics []string) error`
- `ReadMessage(ctx context.Context) (*Message, error)`
- `Close()`

---

#### `internal/kafka/producer_test.go`
**Purpose**: Unit tests for Kafka operations
**Tests**:
- Message serialization to JSON
- Message deserialization from JSON
- Schema validation
- Error handling

**Run**: `go test ./internal/kafka -v`

---

#### `internal/logger/logger.go`
**Purpose**: Centralized structured logging
**Responsibilities**:
- Configure zerolog global logger
- Set log level from configuration (DEBUG, INFO, WARN, ERROR)
- Pretty console output for development
- JSON output for production
- Add service name to all log entries
- RFC3339 timestamp format

**Key Functions**:
- `Setup(cfg Config)` - Initialize logger

**Usage**:
```go
log.Info().Str("sensor_id", id).Msg("Processing sensor")
log.Error().Err(err).Msg("Failed to connect")
```

---

### `/backend/pkg` - Public Packages

Currently empty. Reserved for packages that could be imported by external projects.

---

## 🗄️ Database Migrations (`/migrations`)

SQL scripts run on PostgreSQL container startup (via Docker volume mount).

### Migration Files

#### `001_create_sensors_table.sql`
**Purpose**: Create sensors metadata table
**Schema**:
- `id` UUID PRIMARY KEY
- `name` VARCHAR(100) - Sensor name
- `type` VARCHAR(50) - Sensor type (temperature, pollution, humidity, noise)
- `latitude`, `longitude` DECIMAL - Location coordinates
- `status` VARCHAR(20) - Active/inactive status
- `config` JSONB - Additional configuration
- `created_at`, `updated_at` TIMESTAMP

**Indexes**: None initially (add later if needed)

---

#### `002_create_sensor_readings_table.sql`
**Purpose**: Create partitioned table for time-series sensor data
**Schema**:
- `id` BIGSERIAL
- `sensor_id` UUID REFERENCES sensors(id)
- `sensor_type` VARCHAR(50)
- `value` DECIMAL(10, 2)
- `unit` VARCHAR(20)
- `latitude`, `longitude` DECIMAL - Reading location
- `timestamp` TIMESTAMP NOT NULL
- PRIMARY KEY (timestamp, sensor_id)

**Partitioning**: RANGE partitioning by `timestamp` (monthly partitions)

---

#### `003_create_partitions.sql`
**Purpose**: Create monthly partitions for sensor_readings
**Partitions Created**:
- `sensor_readings_2024_11` (Nov 2024)
- `sensor_readings_2024_12` (Dec 2024)
- `sensor_readings_2025_01` (Jan 2025)
- `sensor_readings_2025_02` (Feb 2025)
- `sensor_readings_2025_11` (Nov 2025) - Current
- `sensor_readings_2025_12` (Dec 2025)

**Future**: Automate partition creation with cron job or pg_cron

---

#### `004_create_aggregates_table.sql`
**Purpose**: Store pre-computed aggregations (hourly, daily)
**Schema**:
- `id` BIGSERIAL PRIMARY KEY
- `sensor_id` UUID
- `sensor_type` VARCHAR(50)
- `aggregation_type` VARCHAR(20) - 'hourly' or 'daily'
- `avg_value`, `min_value`, `max_value`, `stddev_value` DECIMAL
- `period_start`, `period_end` TIMESTAMP
- `created_at` TIMESTAMP

**Usage**: Analytics service will populate this table (Week 2+)

---

#### `005_create_alerts_table.sql`
**Purpose**: Store alert events for threshold breaches
**Schema**:
- `id` BIGSERIAL PRIMARY KEY
- `sensor_id` UUID REFERENCES sensors(id)
- `alert_type` VARCHAR(50) - Type of alert
- `severity` VARCHAR(20) - low, medium, high, critical
- `message` TEXT - Alert description
- `value`, `threshold` DECIMAL - Actual vs threshold value
- `timestamp` TIMESTAMP
- `acknowledged` BOOLEAN - Acknowledgment status
- `acknowledged_at` TIMESTAMP
- `acknowledged_by` VARCHAR(100)

**Usage**: Analytics service will write alerts here (Week 2+)

---

#### `006_create_indexes.sql`
**Purpose**: Create database indexes for query performance
**Indexes**:
- `idx_sensor_readings_type` ON sensor_readings(sensor_type)
- `idx_sensor_readings_timestamp` ON sensor_readings(timestamp DESC)
- `idx_sensors_type` ON sensors(type)
- `idx_sensors_status` ON sensors(status)
- Future: Add composite indexes as needed

---

#### `007_seed_sensors.sql`
**Purpose**: Seed database with 50 initial sensors
**Seed Data**:
- 12 temperature sensors
- 14 pollution sensors (PM2.5)
- 12 humidity sensors
- 12 noise sensors
- Distributed across NYC-like grid (40.70-40.80 lat, -74.02 to -73.92 lon)
- All sensors initially active
- Realistic sensor names (e.g., "Temp Sensor Downtown 1")

---

## 📜 Scripts (`/scripts`)

Operational helper scripts for development and monitoring.

### `start-all.sh`
**Purpose**: Automated startup of entire system
**Actions**:
1. Start Docker Compose services (PostgreSQL, Redis, Kafka, Zookeeper, UIs)
2. Wait 15 seconds for services to initialize
3. Create Kafka topics if they don't exist:
   - `sensor-readings` (6 partitions)
   - `admin-commands` (3 partitions)
   - `alerts` (3 partitions)
4. Start sensor simulator in background
5. Start data ingestion service in background
6. Save PIDs to `/tmp/smart-city-pids.txt`
7. Display status and access URLs

**Usage**: `./scripts/start-all.sh`

---

### `stop-all.sh`
**Purpose**: Graceful shutdown of entire system
**Actions**:
1. Kill Go processes (simulator, ingestion) using saved PIDs
2. Stop Docker Compose services
3. Clean up PID file
4. Display shutdown confirmation

**Usage**: `./scripts/stop-all.sh`

**Note**: Does NOT delete Docker volumes (data persists)

---

### `monitor.sh`
**Purpose**: Real-time system monitoring dashboard
**Displays**:
- PostgreSQL statistics:
  - Total sensor readings
  - Readings by sensor type
  - Readings in last minute/hour
  - Recent sensor activity
- Redis statistics:
  - Total keys
  - Latest readings count
  - Stream entries
  - Geospatial index size
- Kafka statistics:
  - Topic message count
  - Consumer group lag
- Docker container status
- Timestamp of last refresh

**Refresh**: Every 3 seconds
**Usage**: `./scripts/monitor.sh`
**Exit**: Ctrl+C

---

### `load-test.sh`
**Purpose**: Automated load testing with varying sensor counts
**Test Scenarios**:
1. 50 sensors (baseline)
2. 100 sensors
3. 200 sensors
4. 500 sensors

**Metrics Collected**:
- Test duration
- Readings created
- Throughput (readings/sec)
- Success rate
- Database write latency
- Redis operation latency

**Output**: Logs to `/tmp/load-test-{sensor-count}.log`
**Usage**: `./scripts/load-test.sh`

---

### `test-pipeline.sh`
**Purpose**: Quick end-to-end pipeline test
**Actions**:
1. Check all Docker services are running
2. Start sensor simulator for 10 seconds
3. Start data ingestion service
4. Verify data in PostgreSQL
5. Verify data in Redis
6. Check Kafka consumer lag
7. Display test results

**Usage**: `./scripts/test-pipeline.sh`

---

### `monitoring-queries.sql`
**Purpose**: Comprehensive SQL queries for system analysis
**Query Categories**:
1. **System Health** (Database size, connections, long-running queries)
2. **Data Statistics** (Readings by type, hourly/daily trends)
3. **Sensor Health** (Active/inactive sensors, last seen timestamps)
4. **Anomaly Detection** (Out-of-range values, sudden spikes, flatlines)
5. **Performance Metrics** (Ingestion rate, partition distribution, index usage)
6. **Environmental Insights** (Pollution hotspots, temperature trends, noise patterns)

**Total**: 20+ queries
**Usage**: Copy-paste into `psql` or run via script

---

## 📚 Documentation (`/docs`)

### `load-test-results.md`
**Purpose**: Document load testing results and performance metrics
**Contents**:
- Test configurations (sensor count, duration)
- Throughput measurements
- Success rates
- Identified bottlenecks
- Strengths and areas for optimization
- Comparison across different loads

**Updated**: After each load test run

---

### `optimizations.md`
**Purpose**: Track future performance optimizations and enhancements
**Contents**:
- Performance optimizations (batch size, async writes, Redis pipelining)
- Scalability enhancements (horizontal scaling, partitioning)
- Monitoring & observability (Prometheus, Grafana, distributed tracing)
- Code quality & testing improvements
- Infrastructure enhancements (HA setup, resource limits)
- Feature additions (anomaly detection, GraphQL API)
- Priority matrix (High/Medium/Low)
- Estimated impact for each optimization

**Updated**: As new optimization ideas emerge

---

## 🐳 Infrastructure

### `docker-compose.yml`
**Purpose**: Local development infrastructure orchestration
**Services**:

1. **zookeeper** - Kafka coordination service
   - Image: confluentinc/cp-zookeeper:7.4.0
   - Port: 2181
   - Volume: `zookeeper-data`, `zookeeper-logs`

2. **kafka** - Message broker
   - Image: confluentinc/cp-kafka:7.4.0
   - Ports: 9092 (internal), 9093 (external)
   - Volume: `kafka-data`
   - Depends on: zookeeper

3. **kafka-ui** - Web UI for Kafka
   - Image: provectuslabs/kafka-ui:latest
   - Port: 8081
   - URL: http://localhost:8081

4. **postgres** - PostgreSQL database
   - Image: postgres:16
   - Port: 5433 (to avoid conflicts with local postgres)
   - Volume: `postgres-data`, `./migrations` (init scripts)
   - Database: smart_city
   - User: admin / password

5. **redis** - Cache and data structures
   - Image: redis:7-alpine
   - Port: 6379
   - Volume: `redis-data`
   - AOF persistence enabled

6. **redis-commander** - Web UI for Redis
   - Image: rediscommander/redis-commander:latest
   - Port: 8082
   - URL: http://localhost:8082

**Volumes**:
- `postgres-data` - PostgreSQL data persistence
- `redis-data` - Redis data persistence
- `kafka-data` - Kafka message persistence
- `zookeeper-data` - Zookeeper data persistence
- `zookeeper-logs` - Zookeeper logs persistence

---

### `.env`
**Purpose**: Environment configuration for all services
**Sections**:

1. **PostgreSQL Configuration**
   - Host, port, database name, credentials
   - Connection pool settings (max connections, idle connections)

2. **Redis Configuration**
   - Address, port, password, database

3. **Kafka Configuration**
   - Broker addresses
   - Topic names (sensor-readings, admin-commands, alerts)
   - Consumer group IDs

4. **Sensor Simulator Configuration**
   - Sensor count (default: 50)
   - Generation interval (default: 1000ms)
   - Feature flags (rush hour, weather patterns)

5. **Application Settings**
   - Log level (debug, info, warn, error)
   - Environment (development, production)

**Note**: Not committed to git (in .gitignore)

---

### `.gitignore`
**Purpose**: Exclude files from version control
**Patterns**:
- Environment variables (.env files)
- Logs (*.log, logs/, tmp/*.log)
- Go artifacts (vendor, bin, *.exe, *.out, coverage files)
- IDE files (.vscode, .idea, *.swp)
- OS files (.DS_Store, Thumbs.db)
- Docker volumes (postgres-data, redis-data, kafka-data)
- Temporary files (PIDs, load test logs)
- Build artifacts (dist, build)
- Frontend (node_modules, build)
- Kubernetes secrets

---

## 📖 Documentation Files

### `README.md`
**Purpose**: Project overview and quick start guide
**Contents**:
- What the project does
- Tech stack
- Architecture diagram
- Quick start (automated)
- Manual setup instructions
- Development phases with completion status
- Data models (PostgreSQL, Redis, Kafka)
- API documentation (Week 2+)
- Deployment instructions
- Progress tracking
- Available scripts
- Troubleshooting guide

**Audience**: New developers, recruiters, project overview

---

### `DEVELOPMENT.md`
**Purpose**: Detailed technical documentation for developers
**Contents**:
- System architecture with ASCII diagrams
- Detailed data flow explanation
- Component responsibilities
- Database schema with indexes
- Redis data structures with examples
- Kafka topics and message formats
- Development workflow
- Testing strategy
- Performance considerations
- Logging conventions
- Monitoring tools
- Contributing guidelines

**Audience**: Active developers working on the project

---

### `PROJECT-STRUCTURE.md` (This File)
**Purpose**: Complete file tree with purpose of each file/directory
**Contents**:
- Full directory structure
- Purpose of each file
- Responsibilities and key features
- Usage instructions
- Cross-references to other files

**Audience**: Developers navigating the codebase

---

### `week1-tasks.md`
**Purpose**: Week 1 task tracker and daily breakdown
**Contents**:
- Daily task lists (Days 1-7)
- Task completion checkboxes
- Deliverables for each day
- Time estimates
- Testing checklist
- Common issues & solutions
- Success metrics
- Key files created

**Audience**: Project manager, developers tracking progress

---

## 🎨 Frontend (`/frontend`) - Week 3+

**Status**: Not yet created
**Planned Structure**:
```
frontend/
├── public/
├── src/
│   ├── components/
│   │   ├── Map/
│   │   ├── Dashboard/
│   │   ├── SensorDetail/
│   │   └── Alerts/
│   ├── hooks/
│   ├── services/
│   │   ├── api.js
│   │   └── websocket.js
│   ├── App.js
│   └── index.js
├── package.json
└── .env.local
```

---

## ☸️ Kubernetes (`/k8s`) - Week 5

**Status**: Not yet created
**Planned Structure**:
```
k8s/
├── namespaces/
│   └── smart-city.yaml
├── deployments/
│   ├── sensor-simulator.yaml
│   ├── data-ingestion.yaml
│   ├── analytics.yaml
│   └── api-gateway.yaml
├── services/
│   └── api-gateway-service.yaml
├── configmaps/
│   └── app-config.yaml
├── secrets/                    # Not in git
│   └── db-credentials.yaml
├── persistent-volumes/
│   ├── postgres-pv.yaml
│   └── redis-pv.yaml
└── hpa/                       # Horizontal Pod Autoscaling
    └── data-ingestion-hpa.yaml
```

---

## 📊 File Count Summary

### Week 1 (Current)
- **Go Files**: 15+ files
- **SQL Migrations**: 7 files
- **Scripts**: 5 shell scripts + 1 SQL file
- **Documentation**: 5 markdown files
- **Configuration**: 3 files (docker-compose.yml, .env, .gitignore)

### Total Lines of Code (Estimated)
- **Go Code**: ~2,000 lines
- **SQL**: ~500 lines
- **Shell Scripts**: ~300 lines
- **Documentation**: ~2,000 lines
- **Configuration**: ~150 lines

**Total**: ~5,000 lines

---

## 🔄 File Lifecycle

### Created in Week 1
- All backend Go code
- All database migrations
- All helper scripts
- Core documentation
- Docker infrastructure

### Planned for Week 2
- `cmd/api-gateway/main.go`
- `cmd/analytics/main.go`
- API endpoint handlers
- WebSocket server code
- Additional tests

### Planned for Week 3
- Complete `/frontend` directory
- React components
- WebSocket client
- API service layer

### Planned for Week 5
- Complete `/k8s` directory
- Deployment manifests
- Service definitions
- ConfigMaps and Secrets

---

## 🎯 Key Files for Each Persona

### **Backend Developer**
- `backend/internal/models/`
- `backend/cmd/*/main.go`
- `DEVELOPMENT.md`

### **DevOps Engineer**
- `docker-compose.yml`
- `scripts/`
- `.env`
- `k8s/` (Week 5)

### **Frontend Developer** (Week 3+)
- `frontend/src/`
- `README.md` (API documentation section)

### **Database Administrator**
- `migrations/`
- `scripts/monitoring-queries.sql`

### **Project Manager**
- `README.md`
- `week1-tasks.md`
- `docs/load-test-results.md`

---

## 📝 Naming Conventions

### Go Files
- Package names: lowercase, single word (e.g., `postgres`, `kafka`)
- File names: lowercase with underscores (e.g., `sensor_reading.go`)
- Test files: `*_test.go`

### SQL Files
- Migration files: numbered prefix (e.g., `001_create_sensors_table.sql`)
- Query files: descriptive names (e.g., `monitoring-queries.sql`)

### Scripts
- Shell scripts: kebab-case with `.sh` extension (e.g., `start-all.sh`)
- Executable permissions required (`chmod +x`)

### Documentation
- Markdown files: UPPERCASE for root-level (e.g., `README.md`)
- Markdown files: lowercase for docs folder (e.g., `load-test-results.md`)

---

## 🔗 File Dependencies

### High-Level Dependency Graph
```
.env
  └─> docker-compose.yml
        └─> migrations/*.sql → PostgreSQL
        └─> Docker services (Kafka, Redis, etc.)

internal/config
  └─> All cmd/*/main.go files

internal/models
  └─> internal/postgres
  └─> internal/redis
  └─> internal/kafka

cmd/sensor-simulator
  └─> internal/postgres (load sensors)
  └─> internal/kafka (produce messages)

cmd/data-ingestion
  └─> internal/kafka (consume messages)
  └─> internal/postgres (write readings)
  └─> internal/redis (cache updates)
```

---

## 🚀 Quick Navigation

### To Start Development
1. Read: `README.md`
2. Run: `./scripts/start-all.sh`
3. Monitor: `./scripts/monitor.sh`

### To Understand Architecture
1. Read: `DEVELOPMENT.md`
2. Review: `docker-compose.yml`
3. Explore: `backend/internal/models/`

### To Add a New Service
1. Create: `backend/cmd/new-service/main.go`
2. Update: `docker-compose.yml` (if containerizing)
3. Update: `scripts/start-all.sh`
4. Document: `DEVELOPMENT.md`

### To Debug Issues
1. Check: `./scripts/monitor.sh`
2. Query: `scripts/monitoring-queries.sql`
3. Logs: `docker logs <service-name>`

---

**Maintained By**: Development Team
**Review Frequency**: Weekly or after major changes
**Version**: Updated with each development phase

