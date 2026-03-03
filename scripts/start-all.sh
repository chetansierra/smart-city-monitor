#!/bin/bash

# Script to start all Smart City Monitor services
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "$PROJECT_ROOT"

echo "========================================="
echo " Starting Smart City Monitor"
echo "========================================="
echo ""

# Start Docker infrastructure
echo "1. Starting Docker infrastructure..."
docker compose up -d

# Wait for services to be healthy
echo ""
echo "2. Waiting for services to be ready..."
sleep 15

# Check if services are running
echo ""
echo "3. Checking service health..."
docker ps --format "table {{.Names}}\t{{.Status}}"

echo ""
echo "4. Running preflight checks..."
if ! ./scripts/preflight.sh; then
    echo "   ⚠️  Preflight checks failed. Fix configuration and retry."
    exit 1
fi

echo ""
echo "5. Waiting for API Gateway to be ready..."
MAX_RETRIES=30
RETRY_COUNT=0
until curl -f http://localhost:8080/health >/dev/null 2>&1; do
    RETRY_COUNT=$((RETRY_COUNT+1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        echo "   ⚠️  API Gateway failed to start. Check logs with: docker logs api-gateway"
        break
    fi
    echo "   Waiting for API Gateway... ($RETRY_COUNT/$MAX_RETRIES)"
    sleep 2
done

if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
    echo "   ✓ API Gateway is ready"
fi

echo ""
echo "6. Verifying application service containers..."
for service in sensor-simulator data-ingestion alert-monitor api-gateway; do
    if docker ps --format "{{.Names}}" | grep -q "^${service}$"; then
        echo "   ✓ ${service} container is running"
    else
        echo "   ⚠️  ${service} container is not running"
    fi
done

echo ""
echo "========================================="
echo " ✓ All services started!"
echo "========================================="
echo ""
echo "Docker Services:"
echo "  ✓ PostgreSQL"
echo "  ✓ Redis"
echo "  ✓ Kafka + Zookeeper"
echo "  ✓ API Gateway"
echo "  ✓ Alert Monitor"
echo ""
echo "Logs:"
echo "  API Gateway:    docker logs -f api-gateway"
echo "  Alert Monitor:  docker logs -f alert-monitor"
echo "  Simulator:      docker logs -f sensor-simulator"
echo "  Ingestion:      docker logs -f data-ingestion"
echo ""
echo "Web UIs:"
echo "  API Gateway:      http://localhost:8080/health"
echo "  Kafka UI:         http://localhost:8081 (if enabled in docker-compose)"
echo "  Redis Commander:  http://localhost:8082 (if enabled in docker-compose)"
echo ""
echo "API Documentation:"
echo "  See docs/API.md or docs/POSTMAN_GUIDE.md"
echo ""
echo "To monitor: ./scripts/monitor.sh"
echo "To stop:    ./scripts/stop-all.sh"
echo ""
