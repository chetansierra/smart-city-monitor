#!/bin/bash

# Script to stop all Smart City Monitor services

echo "========================================="
echo " Stopping Smart City Monitor"
echo "========================================="
echo ""

# Stop application services
echo "1. Stopping application services..."

if [ -f /tmp/smart-city-pids.txt ]; then
    # Read PIDs from file
    pids=($(grep -E "^[0-9]+$" /tmp/smart-city-pids.txt))

    for pid in "${pids[@]}"; do
        if ps -p $pid > /dev/null 2>&1; then
            echo "   Stopping process $pid..."
            kill $pid 2>/dev/null
        fi
    done

    # Wait for graceful shutdown
    sleep 2

    # Force kill if still running
    for pid in "${pids[@]}"; do
        if ps -p $pid > /dev/null 2>&1; then
            echo "   Force stopping process $pid..."
            kill -9 $pid 2>/dev/null
        fi
    done

    rm /tmp/smart-city-pids.txt
    echo "   ✓ Application services stopped"
else
    echo "   No PID file found, manually stopping Go processes..."
    pkill -f "cmd/sensor-simulator/main.go" 2>/dev/null
    pkill -f "cmd/data-ingestion/main.go" 2>/dev/null
fi

echo ""
echo "2. Stopping Docker infrastructure..."
docker compose down

echo ""
echo "========================================="
echo " ✓ All services stopped!"
echo "========================================="
echo ""
echo "Data is persisted in Docker volumes."
echo "To start again: ./scripts/start-all.sh"
echo ""
