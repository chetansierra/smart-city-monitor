#!/bin/bash

# Load testing script for Smart City Monitor
# Tests with increasing sensor counts to measure throughput

echo "========================================="
echo " Smart City Monitor - Load Testing"
echo "========================================="
echo ""

# Test configurations
TEST_DURATION=30  # seconds for each test
TEST_COUNTS=(50 100 200 500)

# Results file
RESULTS_FILE="/tmp/load-test-results.txt"
echo "Load Test Results - $(date)" > $RESULTS_FILE
echo "=========================================" >> $RESULTS_FILE
echo "" >> $RESULTS_FILE

# Function to run PostgreSQL query
run_psql() {
    docker exec postgres psql -U admin -d smart_city -c "$1" -t 2>/dev/null | tr -d ' '
}

# Function to perform load test
run_load_test() {
    local sensor_count=$1

    echo ""
    echo "========================================="
    echo " Testing with $sensor_count sensors"
    echo "========================================="

    # Get initial count
    initial_count=$(run_psql "SELECT COUNT(*) FROM sensor_readings;")
    echo "Initial readings: $initial_count"

    # Create test .env with higher sensor count
    cp .env .env.backup
    sed -i.tmp "s/SENSOR_COUNT=.*/SENSOR_COUNT=$sensor_count/" .env
    rm -f .env.tmp

    # Start simulator
    echo "Starting simulator with $sensor_count sensors..."
    cd backend
    go run cmd/sensor-simulator/main.go > /tmp/load-test-simulator.log 2>&1 &
    SIM_PID=$!

    # Start ingestion
    echo "Starting data ingestion..."
    go run cmd/data-ingestion/main.go > /tmp/load-test-ingestion.log 2>&1 &
    ING_PID=$!

    cd ..

    # Wait for test duration
    echo "Running test for $TEST_DURATION seconds..."
    sleep $TEST_DURATION

    # Stop services
    echo "Stopping services..."
    kill $SIM_PID 2>/dev/null
    kill $ING_PID 2>/dev/null
    sleep 2

    # Get final count
    final_count=$(run_psql "SELECT COUNT(*) FROM sensor_readings;")

    # Calculate metrics
    readings_created=$((final_count - initial_count))
    readings_per_second=$((readings_created / TEST_DURATION))
    expected_readings=$((sensor_count * TEST_DURATION))
    success_rate=$(awk "BEGIN {printf \"%.2f\", ($readings_created / $expected_readings) * 100}")

    # Display results
    echo ""
    echo "Results:"
    echo "  Expected readings: $expected_readings"
    echo "  Actual readings:   $readings_created"
    echo "  Success rate:      $success_rate%"
    echo "  Throughput:        $readings_per_second readings/sec"

    # Save to results file
    echo "Sensor Count: $sensor_count" >> $RESULTS_FILE
    echo "  Test Duration:     ${TEST_DURATION}s" >> $RESULTS_FILE
    echo "  Expected Readings: $expected_readings" >> $RESULTS_FILE
    echo "  Actual Readings:   $readings_created" >> $RESULTS_FILE
    echo "  Success Rate:      $success_rate%" >> $RESULTS_FILE
    echo "  Throughput:        $readings_per_second readings/sec" >> $RESULTS_FILE
    echo "" >> $RESULTS_FILE

    # Check PostgreSQL stats
    echo ""
    echo "Database stats:"
    docker exec postgres psql -U admin -d smart_city -c "
        SELECT
            sensor_type,
            COUNT(*) as count
        FROM sensor_readings
        WHERE timestamp > NOW() - INTERVAL '${TEST_DURATION} seconds'
        GROUP BY sensor_type
        ORDER BY sensor_type;
    "

    # Restore original .env
    mv .env.backup .env
}

# Ensure Docker services are running
echo "Checking Docker services..."
if ! docker ps | grep -q postgres; then
    echo "Error: PostgreSQL container is not running"
    echo "Please start Docker services with: docker compose up -d"
    exit 1
fi

# Run tests
for count in "${TEST_COUNTS[@]}"; do
    run_load_test $count

    # Cool down between tests
    if [ $count != ${TEST_COUNTS[-1]} ]; then
        echo ""
        echo "Cooling down for 10 seconds..."
        sleep 10
    fi
done

echo ""
echo "========================================="
echo " Load Testing Complete!"
echo "========================================="
echo ""
echo "Results saved to: $RESULTS_FILE"
echo ""
cat $RESULTS_FILE
echo ""
echo "Logs available at:"
echo "  - /tmp/load-test-simulator.log"
echo "  - /tmp/load-test-ingestion.log"
echo ""
