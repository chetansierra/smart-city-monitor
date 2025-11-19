#!/bin/bash

# Script to test the complete data pipeline
# Simulator → Kafka → Data Ingestion → PostgreSQL + Redis

echo "=== Testing Complete Data Pipeline ==="
echo ""

# Start sensor simulator in background
echo "Starting Sensor Simulator..."
cd backend
go run cmd/sensor-simulator/main.go > /tmp/simulator.log 2>&1 &
SIMULATOR_PID=$!
echo "  PID: $SIMULATOR_PID"

# Wait a bit for simulator to start
sleep 3

# Start data ingestion in background
echo "Starting Data Ingestion Service..."
go run cmd/data-ingestion/main.go > /tmp/ingestion.log 2>&1 &
INGESTION_PID=$!
echo "  PID: $INGESTION_PID"

# Wait for data to flow
echo ""
echo "Letting data flow for 10 seconds..."
sleep 10

# Check results
echo ""
echo "=== Checking Results ==="

# Check PostgreSQL
echo ""
echo "PostgreSQL sensor_readings count:"
docker exec postgres psql -U admin -d smart_city -c "SELECT COUNT(*) FROM sensor_readings;" -t

# Check Redis
echo ""
echo "Redis keys count:"
docker exec redis redis-cli DBSIZE

# Show some sample data
echo ""
echo "Sample sensor readings (last 5):"
docker exec postgres psql -U admin -d smart_city -c "SELECT sensor_id, sensor_type, value, unit, timestamp FROM sensor_readings ORDER BY timestamp DESC LIMIT 5;"

# Cleanup
echo ""
echo "Stopping services..."
kill $SIMULATOR_PID 2>/dev/null
kill $INGESTION_PID 2>/dev/null

echo ""
echo "✓ Test complete!"
echo ""
echo "Logs available at:"
echo "  - /tmp/simulator.log"
echo "  - /tmp/ingestion.log"
