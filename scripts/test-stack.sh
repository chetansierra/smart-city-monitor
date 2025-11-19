#!/bin/bash

# Script to test the full Smart City Monitor stack

echo "========================================="
echo " Testing Smart City Monitor Stack"
echo "========================================="
echo ""

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

TESTS_PASSED=0
TESTS_FAILED=0

# Function to test service
test_service() {
    local service_name=$1
    local test_command=$2

    echo -n "Testing $service_name... "

    if eval $test_command > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASS${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

echo "1. Testing Docker Services"
echo "----------------------------"

test_service "PostgreSQL" "docker exec postgres pg_isready -U admin"
test_service "Redis" "docker exec redis redis-cli ping"
test_service "Kafka" "docker exec kafka kafka-broker-api-versions --bootstrap-server localhost:9093"
test_service "API Gateway Container" "docker ps | grep -q api-gateway"
test_service "Alert Monitor Container" "docker ps | grep -q alert-monitor"

echo ""
echo "2. Testing API Endpoints"
echo "----------------------------"

# Wait for API to be fully ready
sleep 2

test_service "Health Endpoint" "curl -f http://localhost:8080/health"
test_service "Sensors Endpoint" "curl -f http://localhost:8080/api/v1/sensors"
test_service "Readings Endpoint" "curl -f http://localhost:8080/api/v1/readings"
test_service "Analytics Endpoint" "curl -f http://localhost:8080/api/v1/analytics/city-stats"
test_service "Alerts Endpoint" "curl -f http://localhost:8080/api/v1/alerts"

echo ""
echo "3. Testing Web UIs"
echo "----------------------------"

test_service "Kafka UI" "curl -f http://localhost:8081"
test_service "Redis Commander" "curl -f http://localhost:8082"

echo ""
echo "4. Testing Kafka Topics"
echo "----------------------------"

test_service "sensor-readings topic" "docker exec kafka kafka-topics --bootstrap-server localhost:9093 --list | grep -q sensor-readings"
test_service "alerts topic" "docker exec kafka kafka-topics --bootstrap-server localhost:9093 --list | grep -q alerts"

echo ""
echo "5. Checking Application Services"
echo "----------------------------"

if pgrep -f "cmd/sensor-simulator/main.go" > /dev/null; then
    echo -e "Sensor Simulator... ${GREEN}✓ RUNNING${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "Sensor Simulator... ${YELLOW}⚠ NOT RUNNING${NC}"
    echo "  (Start with: cd backend && go run cmd/sensor-simulator/main.go)"
fi

if pgrep -f "cmd/data-ingestion/main.go" > /dev/null; then
    echo -e "Data Ingestion... ${GREEN}✓ RUNNING${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "Data Ingestion... ${YELLOW}⚠ NOT RUNNING${NC}"
    echo "  (Start with: cd backend && go run cmd/data-ingestion/main.go)"
fi

echo ""
echo "========================================="
echo " Test Results"
echo "========================================="
echo ""
echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All critical services are running!${NC}"
    echo ""
    echo "Next steps:"
    echo "  - View API docs: docs/API.md"
    echo "  - Import Postman: Smart-City-Monitor.postman_collection.json"
    echo "  - Run integration tests: cd backend && go test -v ./tests/integration/..."
    echo "  - Run performance tests: cd backend && go run tests/performance/load_test_main.go"
    exit 0
else
    echo -e "${RED}⚠ Some services failed to start${NC}"
    echo ""
    echo "Troubleshooting:"
    echo "  - Check Docker logs: docker logs <service-name>"
    echo "  - Restart stack: ./scripts/stop-all.sh && ./scripts/start-all.sh"
    exit 1
fi
