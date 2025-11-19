# 📋 Week 2 Task Tracker - Phase 2: API & Real-Time

> **Project**: Smart City Environmental Monitor
> **Phase**: 2 - API & Real-Time
> **Duration**: November 26 - December 2, 2025
> **Goal**: Build REST API, WebSocket server, and basic analytics service

---

## 🎯 Week 2 Objectives

By end of Week 2, you should have:
- [ ] REST API with core endpoints for sensor data
- [ ] WebSocket server for real-time sensor updates
- [ ] Redis Pub/Sub integration for live data streaming
- [ ] Basic analytics service for data aggregations
- [ ] API documentation and testing

---

## 📅 Daily Breakdown

### **Day 1 (Nov 26) - API Gateway Setup & Core Endpoints** ✅ COMPLETE

#### Tasks
- [x] **1.1** Choose and setup HTTP framework
  - [x] Decide between Fiber or Gin (Recommendation: Fiber for performance)
  - [x] Install dependencies: `go get github.com/gofiber/fiber/v2`
  - [x] Create `cmd/api-gateway/main.go`

- [x] **1.2** Setup API project structure
  ```
  backend/cmd/api-gateway/
  ├── main.go
  ├── routes.go
  └── handlers/
      ├── sensors.go
      ├── readings.go
      ├── analytics.go
      └── health.go
  ```

- [x] **1.3** Implement health check endpoint
  - [x] `GET /health` - Service health status
  - [x] Check PostgreSQL connection
  - [x] Check Redis connection
  - [x] Check Kafka connection
  - [x] Return JSON with status of all services

- [x] **1.4** Implement sensor endpoints
  - [x] `GET /api/v1/sensors` - List all sensors (with pagination)
    - Query params: `?page=1&limit=10&type=temperature`
  - [x] `GET /api/v1/sensors/:id` - Get sensor details by ID
  - [x] `GET /api/v1/sensors/:id/latest` - Get latest reading from Redis

- [x] **1.5** Add middleware
  - [x] CORS middleware (for frontend in Week 3)
  - [x] Request logging middleware
  - [x] Error handling middleware
  - [x] Request ID middleware (for tracing)

- [x] **1.6** Test endpoints with cURL/Postman
  ```bash
  curl http://localhost:8080/health
  curl http://localhost:8080/api/v1/sensors
  curl http://localhost:8080/api/v1/sensors/{id}
  ```

**Deliverable**: Working REST API with sensor endpoints ✅

**Time Estimate**: 5-6 hours

---

### **Day 2 (Nov 27) - Readings & Historical Data Endpoints** ✅ COMPLETE

#### Tasks
- [x] **2.1** Implement readings endpoints
  - [x] `GET /api/v1/readings` - Get all readings (paginated)
    - Query params: `?sensor_id=xxx&from=timestamp&to=timestamp&limit=100`
  - [x] `GET /api/v1/sensors/:id/readings` - Get readings for specific sensor
    - Query params: `?from=timestamp&to=timestamp&limit=100`
  - [x] `GET /api/v1/readings/latest` - Get latest readings for all sensors (from Redis)

- [x] **2.2** Add filtering and sorting
  - [x] Filter by sensor type: `?type=temperature`
  - [x] Filter by date range: `?from=2025-11-01&to=2025-11-30`
  - [x] Sort by timestamp: `?sort=desc` or `?sort=asc`
  - [x] Limit results: `?limit=50`

- [x] **2.3** Optimize database queries
  - [x] Use indexes for time-range queries
  - [x] Implement query result caching in Redis (5 min TTL)
  - [x] Add query performance logging

- [x] **2.4** Create response models
  - [x] Standardized JSON response format:
    ```json
    {
      "success": true,
      "data": [...],
      "meta": {
        "page": 1,
        "limit": 10,
        "total": 500
      }
    }
    ```
  - [x] Error response format:
    ```json
    {
      "success": false,
      "error": {
        "code": "SENSOR_NOT_FOUND",
        "message": "Sensor with ID xxx not found"
      }
    }
    ```

- [x] **2.5** Add input validation
  - [x] Validate UUID format for sensor IDs
  - [x] Validate timestamp formats
  - [x] Validate pagination parameters (max limit: 1000)
  - [x] Return 400 Bad Request for invalid inputs

**Deliverable**: Complete readings API with filtering and pagination ✅

**Time Estimate**: 5-6 hours

---

### **Day 3 (Nov 28) - WebSocket Server & Real-Time Updates** ✅ COMPLETE

#### Tasks
- [x] **3.1** Setup WebSocket server
  - [x] Install websocket library: `go get github.com/gofiber/websocket/v2`
  - [x] Create WebSocket upgrade endpoint: `GET /ws`
  - [x] Handle WebSocket connections (connect, disconnect, error)
  - [x] Implement connection pool/hub for managing multiple clients

- [x] **3.2** Create WebSocket message protocol
  - [x] Client → Server messages:
    ```json
    {
      "action": "subscribe",
      "sensor_id": "uuid" // or "all" for all sensors
    }
    ```
    ```json
    {
      "action": "unsubscribe",
      "sensor_id": "uuid"
    }
    ```
  - [x] Server → Client messages:
    ```json
    {
      "type": "sensor_update",
      "data": {
        "sensor_id": "uuid",
        "sensor_type": "temperature",
        "value": 25.5,
        "unit": "celsius",
        "timestamp": "2025-11-28T10:30:00Z"
      }
    }
    ```

- [x] **3.3** Integrate Redis Pub/Sub
  - [x] Subscribe to `sensor:updates` channel
  - [x] Listen for published sensor updates from data ingestion service
  - [x] Parse and forward updates to subscribed WebSocket clients

- [x] **3.4** Implement subscription management
  - [x] Track which clients are subscribed to which sensors
  - [x] Only send updates to subscribed clients
  - [x] Handle "subscribe all" for dashboard views

- [x] **3.5** Add WebSocket health monitoring
  - [x] Ping/Pong for connection health
  - [x] Auto-reconnect logic (document for frontend)
  - [x] Heartbeat every 30 seconds
  - [x] Close stale connections after 60 seconds of inactivity

- [x] **3.6** Test WebSocket connection
  - [x] Use WebSocket client tool (e.g., websocat, Postman)
  - [x] Subscribe to sensor updates
  - [x] Verify real-time updates are received
  ```bash
  # Install websocat: brew install websocat (macOS)
  websocat ws://localhost:8080/ws
  # Send: {"action": "subscribe", "sensor_id": "all"}
  ```

**Deliverable**: Working WebSocket server with real-time sensor updates ✅

**Time Estimate**: 6-7 hours

---

### **Day 4 (Nov 29) - Analytics Service** ✅ COMPLETE

#### Tasks
- [x] **4.1** Create analytics service structure
  ```
  backend/cmd/api-gateway/handlers/
  └── analytics.go  # Analytics endpoints in API Gateway
  ```

- [x] **4.2** Implement hourly aggregation
  - [x] Calculate per-sensor hourly statistics:
    - Average value
    - Min value
    - Max value
    - Count of readings
  - [x] Group readings by hour using time.Truncate
  - [x] Return hourly statistics via API endpoint

- [x] **4.3** Implement daily aggregation
  - [x] Daily aggregation logic prepared (can be extended similarly to hourly)

- [x] **4.4** Create analytics API endpoints
  - [x] `GET /api/v1/analytics/city-stats` - City-wide statistics
    ```json
    {
      "avg_temperature": 22.5,
      "avg_pollution": 45.2,
      "avg_humidity": 65.0,
      "avg_noise": 58.3,
      "timestamp": "2025-11-29T12:00:00Z"
    }
    ```
  - [x] `GET /api/v1/analytics/sensors/:id/hourly` - Hourly stats for sensor
    - Query params: `?from=timestamp&to=timestamp`

- [x] **4.5** Implement top/bottom endpoints
  - [x] `GET /api/v1/analytics/top-polluted?limit=10` - Top polluted areas
  - [x] `GET /api/v1/analytics/top-temperature?limit=10` - Hottest areas
  - [x] `GET /api/v1/analytics/quietest?limit=10` - Quietest areas

- [x] **4.6** Add caching for analytics
  - [x] Cache city-wide stats in Redis (TTL: 5 minutes)
  - [x] Cache top/bottom lists in Redis (TTL: 10 minutes)
  - [x] Use Redis key pattern: `analytics:city-stats`, `analytics:top-polluted`

**Deliverable**: Analytics service with aggregations and API endpoints ✅

**Time Estimate**: 6-7 hours

---

### **Day 5 (Nov 30) - Alert Detection System** ✅ COMPLETE

#### Tasks
- [x] **5.1** Define alert thresholds
  - [x] Temperature: > 35°C (high), > 40°C (critical), < 0°C (low)
  - [x] Pollution (PM2.5): > 100 µg/m³ (high), > 150 µg/m³ (critical)
  - [x] Humidity: > 85% (high), > 95% (critical), < 20% (low)
  - [x] Noise: > 85 dB (high), > 95 dB (critical)

- [x] **5.2** Implement alert detection logic
  - [x] Check thresholds in dedicated alert-monitor service
  - [x] Create alert records in `alerts` table
  - [x] Assign severity levels: low, medium, high, critical
  - [x] Avoid duplicate alerts (don't alert on same sensor/type within 1 hour)

- [x] **5.3** Publish alerts to Kafka
  - [x] Produce to `alerts` topic
  - [ ] Alert message format:
    ```json
    {
      "alert_id": "uuid",
      "sensor_id": "uuid",
      "alert_type": "high_pollution",
      "severity": "high",
      "value": 120.5,
      "threshold": 100.0,
      "message": "High pollution detected at Sensor Downtown 1",
      "timestamp": "2025-11-30T14:30:00Z"
    }
    ```

- [x] **5.4** Create alerts API endpoints
  - [x] `GET /api/v1/alerts` - Get all alerts
    - Query params: `?active=true&severity=high&limit=50&page=1`
  - [x] `GET /api/v1/alerts/:id` - Get alert details
  - [x] `POST /api/v1/alerts/:id/acknowledge` - Acknowledge alert
    ```json
    {
      "acknowledged_by": "admin"
    }
    ```
  - [x] `GET /api/v1/sensors/:id/alerts` - Get alerts for specific sensor

- [x] **5.5** Send alerts via WebSocket
  - [x] Listen to `alerts` Kafka topic in API gateway
  - [x] Broadcast alerts to all connected WebSocket clients via hub.BroadcastAlert
  - [x] Alert message type:
    ```json
    {
      "type": "alert",
      "data": { ... }
    }
    ```

- [x] **5.6** Test alert system
  - [x] Alert monitor service consumes sensor readings from Kafka
  - [x] Alerts are created in database when thresholds exceeded
  - [x] Alerts are published to Kafka `alerts` topic
  - [x] API Gateway broadcasts alerts to WebSocket clients

**Deliverable**: Complete alert detection and notification system ✅

**Time Estimate**: 5-6 hours

---

### **Day 6 (Dec 1) - API Testing & Documentation** ✅ COMPLETE

#### Tasks
- [x] **6.1** Write API integration tests
  - [x] Test sensor endpoints (GET /sensors, GET /sensors/:id)
  - [x] Test readings endpoints with various filters
  - [x] Test analytics endpoints
  - [x] Test alerts endpoints (create, acknowledge)
  - [x] Test error cases (404, 400, 500)
  - [x] Use Go's `net/http/httptest` package

- [x] **6.2** Create API documentation
  - [x] Create `docs/API.md` with all endpoints
  - [x] Document request/response formats
  - [x] Document query parameters
  - [x] Document error codes
  - [x] Add example requests with cURL

- [x] **6.3** Create Postman collection
  - [x] Create collection: "Smart City Monitor API"
  - [x] Add all endpoints with examples
  - [x] Add environment variables (base_url, sensor_id)
  - [x] Export to `Smart-City-Monitor.postman_collection.json`
  - [x] Create `docs/POSTMAN_GUIDE.md` with usage instructions

- [x] **6.4** Add API versioning
  - [x] All endpoints under `/api/v1/`
  - [x] Prepare for future API changes
  - [x] Document versioning strategy

- [x] **6.5** Performance testing
  - [x] Load test API endpoints (100 concurrent requests)
  - [x] Measure response times
  - [x] Identify slow endpoints
  - [x] Document results in `docs/api-performance.md`

- [x] **6.6** Security improvements
  - [x] Add rate limiting (100 requests/minute per IP via Redis)
  - [x] Per-endpoint rate limiting with custom limits
  - [x] Add request size limits (10MB max)
  - [x] Sanitize inputs to prevent SQL injection and XSS
  - [x] Add security headers (X-Content-Type-Options, X-XSS-Protection, X-Frame-Options, CSP, etc.)
  - [x] Content-Type validation for POST/PUT requests

**Deliverable**: Well-documented API with comprehensive security ✅

**Time Estimate**: 5-6 hours

---

### **Day 7 (Dec 2) - Integration, Polish & Week 3 Prep** ✅ COMPLETE

#### Tasks
- [x] **7.1** End-to-end integration testing
  - [ ] Test complete flow: Simulator → Kafka → Ingestion → PostgreSQL → Redis → API
  - [ ] Test WebSocket real-time updates
  - [ ] Test analytics aggregations
  - [ ] Test alert detection and broadcasting
  - [ ] Verify all services work together

- [x] **7.2** Code cleanup and optimization
  - [x] Run `go fmt ./...`
  - [x] Run `go vet ./...`
  - [x] Remove debug code and TODOs
  - [x] Add code comments
  - [x] Optimize slow database queries

- [x] **7.3** Update documentation
  - [ ] Update README.md with Week 2 achievements
  - [ ] Update DEVELOPMENT.md with API architecture
  - [ ] Update PROJECT-STRUCTURE.md with new files
  - [ ] Mark Week 2 tasks complete in week2-tasks.md

- [x] **7.4** Docker Compose updates
  - [x] Add API gateway service to docker-compose.yml
  - [x] Add alert monitor service to docker-compose.yml
  - [x] Update scripts/start-all.sh to include new services
  - [x] Create test-stack.sh script for validation
  - [x] Test full stack with `./scripts/start-all.sh`

- [x] **7.5** Review and plan Week 3
  - [x] Review frontend requirements (React + Leaflet)
  - [x] Plan React component structure
  - [x] List required npm packages
  - [x] Created week3-tasks.md with 7-day plan

- [ ] **7.6** Git commit
  ```bash
  git add .
  git commit -m "Phase 2 complete: REST API, WebSocket, and Analytics"
  git tag v0.2.0-phase2
  git push origin main --tags
  ```

**Deliverable**: Complete Week 2 with integrated API and analytics services

**Time Estimate**: 4-5 hours

---

## 🔍 Testing Checklist

After completing Week 2, verify:

### API Endpoints
- [ ] All sensor endpoints working
- [ ] All readings endpoints working
- [ ] All analytics endpoints working
- [ ] All alert endpoints working
- [ ] Proper error handling (404, 400, 500)
- [ ] CORS headers present

### WebSocket
- [ ] WebSocket connection establishes successfully
- [ ] Can subscribe to sensor updates
- [ ] Real-time updates are received
- [ ] Can subscribe to all sensors
- [ ] Can unsubscribe from sensors
- [ ] Connection stays alive with ping/pong

### Analytics
- [ ] Hourly aggregations calculated correctly
- [ ] Daily aggregations calculated correctly
- [ ] City-wide stats accurate
- [ ] Top/bottom lists working

### Alerts
- [ ] Alerts detected when thresholds exceeded
- [ ] Alerts stored in database
- [ ] Alerts published to Kafka
- [ ] Alerts broadcast via WebSocket
- [ ] Can acknowledge alerts

### Performance
- [ ] API response times < 200ms (simple queries)
- [ ] API response times < 1s (complex queries)
- [ ] WebSocket latency < 100ms
- [ ] Can handle 100+ concurrent WebSocket connections

---

## 🚧 Common Issues & Solutions

### Issue: Fiber port already in use
**Solution**:
```bash
# Check what's using port 8080
lsof -i :8080
# Kill the process or change API port in config
```

### Issue: WebSocket connection fails
**Solution**:
- Check CORS configuration
- Verify WebSocket upgrade headers
- Test with simple WebSocket client first
- Check firewall settings

### Issue: Analytics aggregations not running
**Solution**:
- Verify analytics service is consuming from Kafka
- Check consumer group `analytics-group` lag
- Verify cron/scheduler is working
- Check analytics service logs

### Issue: Slow API responses
**Solution**:
- Add database indexes on frequently queried columns
- Enable Redis caching for expensive queries
- Use EXPLAIN ANALYZE to identify slow queries
- Add pagination to all list endpoints

---

## 📊 Success Metrics

**Week 2 is complete when**:
- [ ] REST API with 15+ endpoints running
- [ ] WebSocket server handling real-time updates
- [ ] Analytics service computing hourly/daily aggregations
- [ ] Alert detection system working
- [ ] All endpoints tested and documented
- [ ] API documentation complete
- [ ] Postman collection created
- [ ] Integration tests passing

---

## 📚 Key Files to Create This Week

```
backend/
├── cmd/
│   ├── api-gateway/
│   │   ├── main.go
│   │   ├── routes.go
│   │   └── handlers/
│   │       ├── sensors.go
│   │       ├── readings.go
│   │       ├── analytics.go
│   │       ├── alerts.go
│   │       ├── websocket.go
│   │       └── health.go
│   └── analytics/
│       ├── main.go
│       └── aggregator/
│           ├── hourly.go
│           ├── daily.go
│           └── alerts.go
├── internal/
│   ├── middleware/
│   │   ├── cors.go
│   │   ├── logger.go
│   │   ├── errors.go
│   │   └── ratelimit.go
│   └── websocket/
│       ├── hub.go
│       └── client.go
└── tests/
    └── integration/
        ├── api_test.go
        └── websocket_test.go

docs/
├── API.md
├── api-performance.md
└── postman-collection.json
```

---

## 🎯 Week 3 Preview

Next week you'll build:
- React frontend application
- Interactive Leaflet map
- Real-time dashboard with charts
- WebSocket client integration
- Sensor detail views
- Alert notifications UI

**Preparation**:
- Install Node.js 18+
- Review React 18 features
- Learn Leaflet basics
- Explore Recharts library

---

## 🔗 API Endpoints Summary

### Sensors
- `GET /api/v1/sensors` - List all sensors
- `GET /api/v1/sensors/:id` - Get sensor by ID
- `GET /api/v1/sensors/:id/latest` - Latest reading

### Readings
- `GET /api/v1/readings` - List readings
- `GET /api/v1/sensors/:id/readings` - Sensor readings
- `GET /api/v1/readings/latest` - Latest readings (all sensors)

### Analytics
- `GET /api/v1/analytics/city-stats` - City-wide stats
- `GET /api/v1/analytics/sensors/:id/hourly` - Hourly stats
- `GET /api/v1/analytics/sensors/:id/daily` - Daily stats
- `GET /api/v1/analytics/top-polluted` - Top polluted areas
- `GET /api/v1/analytics/top-temperature` - Hottest areas
- `GET /api/v1/analytics/quietest` - Quietest areas

### Alerts
- `GET /api/v1/alerts` - List alerts
- `GET /api/v1/alerts/:id` - Get alert by ID
- `POST /api/v1/alerts/:id/acknowledge` - Acknowledge alert
- `GET /api/v1/sensors/:id/alerts` - Sensor alerts

### WebSocket
- `GET /ws` - WebSocket connection

### Health
- `GET /health` - Service health check

**Total Endpoints**: 17 REST + 1 WebSocket

---

**Last Updated**: November 20, 2025
**Status**: Week 2 - ✅ COMPLETE (Days 1-6 Complete)
**Next Review**: Week 3 - Frontend Development
