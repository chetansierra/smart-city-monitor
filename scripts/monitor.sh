#!/bin/bash

# Monitoring script for Smart City Monitor
# Shows real-time statistics about the system

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

clear

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   Smart City Monitor - Live Stats${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Function to run PostgreSQL queries
run_psql() {
    docker exec postgres psql -U admin -d smart_city -c "$1" -t 2>/dev/null
}

# Function to run Redis commands
run_redis() {
    docker exec redis redis-cli $1 2>/dev/null
}

# Function to run Kafka commands
run_kafka() {
    docker exec kafka kafka-topics --bootstrap-server localhost:9093 $1 2>/dev/null
}

# Loop to continuously display stats
while true; do
    # Move cursor to top
    tput cup 4 0

    # ===================
    # PostgreSQL Stats
    # ===================
    echo -e "${GREEN}PostgreSQL Statistics:${NC}"
    echo "─────────────────────────────────────"

    # Total readings
    total_readings=$(run_psql "SELECT COUNT(*) FROM sensor_readings;")
    echo -e "Total Readings:        ${YELLOW}$total_readings${NC}"

    # Readings by type
    echo ""
    echo "Readings by Sensor Type:"
    run_psql "SELECT
        RPAD(sensor_type, 15) as type,
        LPAD(COUNT(*)::text, 10) as count,
        LPAD(ROUND(AVG(value), 2)::text, 10) as avg_value
    FROM sensor_readings
    GROUP BY sensor_type
    ORDER BY sensor_type;"

    # Recent activity (last minute)
    echo ""
    recent_count=$(run_psql "SELECT COUNT(*) FROM sensor_readings WHERE timestamp > NOW() - INTERVAL '1 minute';")
    echo -e "Last 1 minute:         ${YELLOW}$recent_count${NC} readings"

    # Latest reading
    echo ""
    echo "Latest Reading:"
    run_psql "SELECT
        sensor_type,
        ROUND(value, 2) as value,
        unit,
        TO_CHAR(timestamp, 'HH24:MI:SS') as time
    FROM sensor_readings
    ORDER BY timestamp DESC
    LIMIT 1;"

    echo ""
    echo ""

    # ===================
    # Redis Stats
    # ===================
    echo -e "${GREEN}Redis Statistics:${NC}"
    echo "─────────────────────────────────────"

    # Total keys
    total_keys=$(run_redis "DBSIZE")
    echo -e "Total Keys:            ${YELLOW}$total_keys${NC}"

    # Keys by pattern
    latest_count=$(run_redis "KEYS sensor:latest:*" | wc -l)
    stream_count=$(run_redis "KEYS sensor:stream:*" | wc -l)
    echo -e "Latest Readings:       ${YELLOW}$latest_count${NC}"
    echo -e "Time-series Streams:   ${YELLOW}$stream_count${NC}"

    # Memory usage
    memory=$(run_redis "INFO memory" | grep "used_memory_human" | cut -d: -f2 | tr -d '\r')
    echo -e "Memory Used:           ${YELLOW}$memory${NC}"

    # Top polluted sensors
    echo ""
    echo "Top 3 Most Polluted Areas:"
    run_redis "ZREVRANGE pollution:leaderboard 0 2 WITHSCORES" | paste - - | head -3

    echo ""
    echo ""

    # ===================
    # Kafka Stats
    # ===================
    echo -e "${GREEN}Kafka Statistics:${NC}"
    echo "─────────────────────────────────────"

    # List topics
    echo "Topics:"
    run_kafka "--list" | grep -v "^__" | while read topic; do
        echo "  - $topic"
    done

    # Topic details for sensor-readings
    echo ""
    echo "sensor-readings topic details:"
    docker exec kafka kafka-run-class kafka.tools.GetOffsetShell \
        --broker-list localhost:9093 \
        --topic sensor-readings \
        2>/dev/null | awk -F: '{sum+=$3} END {print "  Total messages: " sum}'

    echo ""
    echo ""

    # ===================
    # Docker Containers
    # ===================
    echo -e "${GREEN}Docker Containers:${NC}"
    echo "─────────────────────────────────────"
    docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "(NAME|kafka|postgres|redis|zookeeper)" | head -7

    echo ""
    echo ""
    echo -e "${BLUE}Refreshing every 3 seconds... (Ctrl+C to exit)${NC}"

    # Wait before next refresh
    sleep 3
done
