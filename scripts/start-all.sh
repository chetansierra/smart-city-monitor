#!/bin/bash

# Script to start all Smart City Monitor services

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
echo "4. Creating Kafka topics (if needed)..."
docker exec kafka kafka-topics --bootstrap-server localhost:9093 --create --topic sensor-readings --partitions 6 --replication-factor 1 --if-not-exists 2>/dev/null
docker exec kafka kafka-topics --bootstrap-server localhost:9093 --create --topic admin-commands --partitions 3 --replication-factor 1 --if-not-exists 2>/dev/null
docker exec kafka kafka-topics --bootstrap-server localhost:9093 --create --topic alerts --partitions 3 --replication-factor 1 --if-not-exists 2>/dev/null

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
echo "6. Starting additional application services..."

# Start sensor simulator in background
echo "   Starting Sensor Simulator..."
cd backend
go run cmd/sensor-simulator/main.go > /tmp/sensor-simulator.log 2>&1 &
SIMULATOR_PID=$!
echo "sensor-simulator" > /tmp/smart-city-pids.txt
echo $SIMULATOR_PID >> /tmp/smart-city-pids.txt

# Wait a bit
sleep 3

# Start data ingestion in background
echo "   Starting Data Ingestion Service..."
go run cmd/data-ingestion/main.go > /tmp/data-ingestion.log 2>&1 &
INGESTION_PID=$!
echo "data-ingestion" >> /tmp/smart-city-pids.txt
echo $INGESTION_PID >> /tmp/smart-city-pids.txt

cd ..

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
echo "Application Service PIDs:"
echo "  Sensor Simulator:   $SIMULATOR_PID"
echo "  Data Ingestion:     $INGESTION_PID"
echo ""
echo "Logs:"
echo "  API Gateway:    docker logs -f api-gateway"
echo "  Alert Monitor:  docker logs -f alert-monitor"
echo "  Simulator:      /tmp/sensor-simulator.log"
echo "  Ingestion:      /tmp/data-ingestion.log"
echo ""
echo "Web UIs:"
echo "  API Gateway:      http://localhost:8080/health"
echo "  Kafka UI:         http://localhost:8081"
echo "  Redis Commander:  http://localhost:8082"
echo ""
echo "API Documentation:"
echo "  See docs/API.md or docs/POSTMAN_GUIDE.md"
echo ""
echo "To monitor: ./scripts/monitor.sh"
echo "To stop:    ./scripts/stop-all.sh"
echo ""
