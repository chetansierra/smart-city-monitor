# Testing Guide

This directory contains integration tests and performance tests for the Smart City Monitor API.

## Integration Tests

Location: `tests/integration/api_test.go`

### Running Integration Tests

The integration tests require the API gateway to be running on `localhost:8080`.

1. Start all services:
```bash
# From backend directory
go run cmd/api-gateway/main.go
```

2. Run the tests:
```bash
# From backend directory
go test -v ./tests/integration/...
```

### Test Coverage

The integration tests cover:

- **Health Endpoint** - Service health check
- **Sensor Endpoints**
  - List all sensors
  - Get sensor by ID
  - Get latest sensor reading
  - Error cases (404, invalid UUID)
- **Readings Endpoints**
  - Get all readings with pagination
  - Filter by date range, sensor type, sort order
  - Get latest readings
- **Analytics Endpoints**
  - City-wide statistics
  - Top polluted areas
  - Hottest areas
  - Quietest areas
- **Error Handling**
  - Invalid pagination parameters
  - Invalid date formats
- **Security & CORS**
  - CORS headers
  - Security headers

Total: **14 test cases** covering core API endpoints

## Performance Tests

Location: `tests/performance/load_test_main.go`

### Running Performance Tests

The performance tests simulate 100 concurrent clients making requests to the API.

1. Ensure API gateway is running on `localhost:8080`

2. Run the performance test:
```bash
# From backend directory
go run tests/performance/load_test_main.go
```

### Test Configuration

- **Concurrent Clients**: 100
- **Requests per Client**: 10
- **Total Requests**: 1000 (100 clients × 10 requests)

### Endpoints Tested

1. `GET /api/v1/sensors`
2. `GET /api/v1/readings?limit=50`
3. `GET /api/v1/readings/latest`
4. `GET /api/v1/analytics/city-stats`
5. `GET /api/v1/analytics/top-polluted?limit=10`
6. `GET /health`

### Performance Metrics

The test measures:
- **Success Rate** - Percentage of successful requests
- **Average Latency** - Mean response time
- **Min/Max Latency** - Fastest and slowest response times
- **P95/P99** - 95th and 99th percentile latencies
- **Throughput** - Requests per second

### Output

The test generates:
1. **Console output** - Real-time performance summary
2. **Markdown report** - Saved to `docs/api-performance.md`

The report includes:
- Overall test summary
- Per-endpoint performance breakdown
- Performance assessment and recommendations

## Performance Criteria

### Response Time Goals

- ✅ **Fast** - < 200ms (acceptable)
- ⚠️ **Moderate** - 200ms - 1s (needs optimization)
- ❌ **Slow** - > 1s (requires immediate attention)

### Reliability Goals

- ✅ **Excellent** - > 99% success rate
- ⚠️ **Good** - > 95% success rate
- ❌ **Poor** - < 95% success rate

## Running All Tests

To run both integration and performance tests:

```bash
# Terminal 1 - Start API Gateway
cd backend
go run cmd/api-gateway/main.go

# Terminal 2 - Run Integration Tests
cd backend
go test -v ./tests/integration/...

# Terminal 3 - Run Performance Tests
cd backend
go run tests/performance/load_test_main.go
```

## CI/CD Integration

To integrate with CI/CD pipelines:

```bash
# Integration tests (with timeout)
go test -v -timeout 30s ./tests/integration/...

# Skip tests if API is not available (useful for unit test runs)
go test -v -short ./tests/integration/...
```

## Notes

- Integration tests will skip if the API is not running (graceful degradation)
- Performance tests require a warm API (run after services have been running for a bit)
- For best performance test results, ensure no other load on the system
- The performance test includes a 500ms pause between endpoint tests
