# 📋 Week 1 Task Tracker - Phase 1: Foundation

> **Project**: Smart City Environmental Monitor  
> **Phase**: 1 - Foundation  
> **Duration**: November 19 - November 25, 2024  
> **Goal**: Establish infrastructure and data pipeline (Simulator → Kafka → Storage)

---

## 🎯 Week 1 Objectives

By end of Week 1, you should have:
- ✅ Complete local development environment
- ✅ Data flowing: Sensor Simulator → Kafka → PostgreSQL + Redis
- ✅ Ability to view stored sensor data
- ✅ Foundation for Week 2 API development

---

## 📅 Daily Breakdown

### **Day 1 (Nov 19) - Environment Setup** ✅ COMPLETE

#### Tasks
- [x] **1.1** Create project directory structure
  ```
  smart-city-monitor/
  ├── backend/
  │   ├── cmd/
  │   │   ├── sensor-simulator/
  │   │   ├── data-ingestion/
  │   │   ├── analytics/
  │   │   └── api-gateway/
  │   ├── internal/
  │   │   ├── models/
  │   │   ├── kafka/
  │   │   ├── redis/
  │   │   └── postgres/
  │   └── pkg/
  ├── frontend/
  ├── k8s/
  ├── docker-compose.yml
  └── README.md
  ```

- [x] **1.2** Initialize Go modules
  ```bash
  cd backend
  go mod init github.com/yourusername/smart-city-monitor
  ```

- [x] **1.3** Create `docker-compose.yml` with:
  - [x] Zookeeper
  - [x] Kafka broker
  - [x] Kafka UI
  - [x] PostgreSQL
  - [x] Redis
  - [x] Redis Commander (optional)

- [x] **1.4** Create `.env` file with all environment variables

- [x] **1.5** Test infrastructure startup
  ```bash
  docker-compose up -d
  docker-compose ps  # Verify all services running
  ```

**Deliverable**: All containers running successfully

**Time Estimate**: 3-4 hours

---

### **Day 2 (Nov 20) - Database Setup** ✅ COMPLETE

#### Tasks
- [x] **2.1** Create PostgreSQL initialization scripts
  - [x] `migrations/001_create_sensors_table.sql`
  - [x] `migrations/002_create_sensor_readings_table.sql`
  - [x] `migrations/003_create_partitions.sql`
  - [x] `migrations/004_create_aggregates_table.sql`
  - [x] `migrations/005_create_alerts_table.sql`
  - [x] `migrations/006_create_indexes.sql`

- [x] **2.2** Add migration volume to docker-compose
  ```yaml
  postgres:
    volumes:
      - ./migrations:/docker-entrypoint-initdb.d
  ```

- [x] **2.3** Create initial sensor seed data (50 sensors)
  - [x] Generate sensor locations across virtual city grid
  - [x] Mix of sensor types (temperature, pollution, humidity, noise)
  - [x] Seed script: `migrations/007_seed_sensors.sql`

- [x] **2.4** Verify database schema
  ```bash
  docker exec -it postgres psql -U admin -d smart_city
  \dt  # List tables
  SELECT COUNT(*) FROM sensors;  # Should return 50
  ```

- [x] **2.5** Create Go database package
  - [x] `internal/postgres/connection.go`
  - [x] `internal/postgres/queries.go`
  - [x] Test connection from Go code

**Deliverable**: Database schema created and seeded with 50 sensors

**Time Estimate**: 4-5 hours

---

### **Day 3 (Nov 21) - Kafka Setup & Sensor Simulator** ✅ COMPLETE

#### Tasks
- [x] **3.1** Create Kafka topics
  ```bash
  # Via Kafka UI or CLI
  - sensor-readings (6 partitions, replication: 1)
  - admin-commands (3 partitions)
  - alerts (3 partitions)
  ```

- [x] **3.2** Create Kafka package
  - [x] `internal/kafka/producer.go`
  - [x] `internal/kafka/consumer.go`
  - [x] Configuration and connection helpers

- [x] **3.3** Build Sensor Simulator service
  - [x] `cmd/sensor-simulator/main.go`
  - [x] Load sensors from PostgreSQL on startup
  - [x] Generate realistic sensor data:
    - Temperature: 15-35°C (varies by time of day)
    - PM2.5: 10-150 µg/m³ (higher during rush hours)
    - Humidity: 30-80%
    - Noise: 40-90 dB
  - [x] Add randomness and patterns (rush hour spikes, etc.)
  - [x] Publish to Kafka topic `sensor-readings`

- [x] **3.4** Configuration
  - [x] Read from environment variables
  - [x] Configurable sensor count and generation interval
  - [x] Graceful shutdown handling

- [x] **3.5** Test simulator
  ```bash
  cd backend/cmd/sensor-simulator
  go run main.go
  ```
  - [x] Verify messages in Kafka UI
  - [x] Check message format matches schema

**Deliverable**: Sensor simulator producing 50 messages/second to Kafka

**Time Estimate**: 5-6 hours

---

### **Day 4 (Nov 22) - Data Ingestion Service** ✅ COMPLETE

#### Tasks
- [x] **4.1** Create Redis package
  - [x] `internal/redis/connection.go`
  - [x] `internal/redis/operations.go`
  - [x] Helper functions for sensor data storage

- [x] **4.2** Build Data Ingestion service
  - [x] `cmd/data-ingestion/main.go`
  - [x] Kafka consumer setup (consumer group: `data-ingestion-group`)
  - [x] Read from `sensor-readings` topic

- [x] **4.3** Implement data storage logic
  - [x] Parse Kafka message
  - [x] Insert into `sensor_readings` table (PostgreSQL)
  - [x] Update Redis:
    - `HSET sensor:latest:{id}` (latest reading)
    - `GEOADD sensors:geo` (geospatial index)
    - `XADD sensor:stream:{id}` (time-series)
  - [x] Handle errors and retries

- [x] **4.4** Add batch processing
  - [x] Batch database inserts (e.g., every 100 messages or 1 second)
  - [x] Optimize for throughput

- [x] **4.5** Test end-to-end flow
  ```bash
  # Terminal 1: Simulator
  cd backend/cmd/sensor-simulator && go run main.go

  # Terminal 2: Ingestion
  cd backend/cmd/data-ingestion && go run main.go
  ```
  - [x] Verify data in PostgreSQL
  - [x] Verify data in Redis (use Redis Commander)
  - [x] Check Kafka consumer lag (should be near zero)

**Deliverable**: Complete data pipeline working (Simulator → Kafka → PostgreSQL + Redis)

**Time Estimate**: 5-6 hours

---

### **Day 5 (Nov 23) - Testing & Monitoring** ✅ COMPLETE

#### Tasks
- [x] **5.1** Add structured logging
  - [x] Use `zerolog` library
  - [x] Log levels: DEBUG, INFO, WARN, ERROR
  - [x] Structured fields (sensor_id, sensor_type, etc.)
  - [x] Pretty console output for development

- [x] **5.2** Create monitoring queries
  - [x] Create `scripts/monitoring-queries.sql` with:
    - System health checks (database size, connections)
    - Data statistics (readings by type, trends)
    - Sensor health monitoring
    - Anomaly detection queries
    - Performance metrics
    - Environmental insights
  - [x] Create `scripts/monitor.sh` for live monitoring

- [x] **5.3** Write basic unit tests
  - [x] PostgreSQL tests (`internal/postgres/queries_test.go`)
  - [x] Redis tests (`internal/redis/operations_test.go`)
  - [x] Kafka tests (`internal/kafka/producer_test.go`)
  - [x] All tests passing

- [x] **5.4** Helper scripts
  - [x] `scripts/start-all.sh` - Start all services
  - [x] `scripts/stop-all.sh` - Stop all services
  - [x] `scripts/monitor.sh` - Real-time monitoring
  - [x] `scripts/load-test.sh` - Load testing script

**Deliverable**: Comprehensive monitoring, testing, and operational tooling

**Time Estimate**: 4-5 hours

---

### **Day 6 (Nov 24) - Documentation & Polish** ✅ COMPLETE

#### Tasks
- [x] **6.1** Update README.md
  - [x] Mark Phase 1 tasks as complete
  - [x] Add troubleshooting section
  - [x] Add screenshots (optional)

- [x] **6.2** Create DEVELOPMENT.md
  - [x] Local setup instructions
  - [x] Common commands
  - [x] Debugging tips
  - [x] FAQ

- [x] **6.3** Code cleanup
  - [x] Remove debug code
  - [x] Add comments to complex logic
  - [x] Format code (`gofmt`)
  - [x] Run linters (`golangci-lint`)

- [x] **6.4** Create .gitignore
  ```
  .env
  *.log
  /vendor
  /bin
  /data
  .DS_Store
  ```

- [x] **6.5** Git commit
  ```bash
  git add .
  git commit -m "Phase 1 complete: Foundation infrastructure"
  git tag v0.1.0-phase1
  ```

**Deliverable**: Clean, documented codebase ready for Phase 2 ✅

**Time Estimate**: 2-3 hours

---

### **Day 7 (Nov 25) - Buffer Day / Week 2 Prep**

#### Tasks
- [ ] **7.1** Fix any remaining issues from Week 1
- [ ] **7.2** Performance optimization if needed
- [ ] **7.3** Review Week 2 requirements (API & Real-Time)
- [ ] **7.4** Design API endpoint structure
- [ ] **7.5** Plan WebSocket implementation

**Deliverable**: Ready to start Week 2

**Time Estimate**: 2-4 hours (or less if all complete)

---

## 🔍 Testing Checklist

After completing Week 1, verify:

### Infrastructure
- [ ] Docker Compose brings up all services without errors
- [ ] PostgreSQL accessible on port 5432
- [ ] Kafka accessible on port 9092
- [ ] Kafka UI accessible at http://localhost:8081
- [ ] Redis accessible on port 6379

### Data Pipeline
- [ ] Sensor simulator starts without errors
- [ ] Messages appear in Kafka topic `sensor-readings`
- [ ] Data ingestion service consumes messages
- [ ] Sensor readings appear in PostgreSQL
- [ ] Latest readings cached in Redis
- [ ] No consumer lag in Kafka

### Data Verification
- [ ] PostgreSQL has 50 sensors in `sensors` table
- [ ] `sensor_readings` table receives new rows every second
- [ ] Redis has latest values for all sensors
- [ ] Geospatial data in Redis is correct

### Performance
- [ ] System handles 50 messages/second smoothly
- [ ] CPU usage < 50% (on local machine)
- [ ] No memory leaks after 10+ minutes
- [ ] Kafka consumer lag < 100 messages

---

## 🚧 Common Issues & Solutions

### Issue: Kafka broker not starting
**Solution**: 
```bash
# Check if port 9092 is in use
lsof -i :9092
# Or increase broker memory in docker-compose
KAFKA_HEAP_OPTS: "-Xmx512M -Xms256M"
```

### Issue: PostgreSQL connection refused
**Solution**:
```bash
# Wait for PostgreSQL to fully initialize
docker-compose logs postgres
# Check if migrations ran
docker exec -it postgres psql -U admin -d smart_city -c "\dt"
```

### Issue: Go dependencies not resolving
**Solution**:
```bash
go mod tidy
go mod download
```

### Issue: Sensor data not appearing in PostgreSQL
**Solution**:
- Check Kafka consumer is running
- Verify Kafka topic has messages (Kafka UI)
- Check ingestion service logs for errors
- Test database connection manually

---

## 📊 Success Metrics

**Week 1 is complete when**:
- ✅ All 5 Docker services running
- ✅ 50 sensors seeded in PostgreSQL
- ✅ Sensor simulator producing 50 msgs/sec to Kafka
- ✅ Data ingestion writing to PostgreSQL + Redis
- ✅ Can query recent sensor readings from database
- ✅ Can retrieve latest sensor values from Redis
- ✅ Zero consumer lag in Kafka

---

## 📚 Key Files Created This Week

```
backend/
├── cmd/
│   ├── sensor-simulator/main.go
│   └── data-ingestion/main.go
├── internal/
│   ├── models/sensor.go
│   ├── kafka/producer.go
│   ├── kafka/consumer.go
│   ├── postgres/connection.go
│   ├── postgres/queries.go
│   ├── redis/connection.go
│   └── redis/operations.go
├── go.mod
└── go.sum

migrations/
├── 001_create_sensors_table.sql
├── 002_create_sensor_readings_table.sql
├── 003_create_partitions.sql
├── 004_create_aggregates_table.sql
├── 005_create_alerts_table.sql
├── 006_create_indexes.sql
└── 007_seed_sensors.sql

scripts/
├── start-all.sh
├── stop-all.sh
├── view-logs.sh
└── reset-db.sh

docker-compose.yml
.env
.gitignore
DEVELOPMENT.md
```

---

## 🎯 Week 2 Preview

Next week you'll build:
- REST API endpoints (Fiber or Gin framework)
- WebSocket server for real-time updates
- Basic analytics service
- API testing with Postman/cURL

**Preparation**: Review Go HTTP frameworks and WebSocket libraries

---

**Last Updated**: November 19, 2025
**Status**: Week 1 - Complete ✅
**Next Review**: November 25, 2025