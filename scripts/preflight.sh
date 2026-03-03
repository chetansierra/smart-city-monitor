#!/bin/bash

# Preflight checks for Smart City Monitor runtime configuration.
# Validates required env keys and Kafka topic naming before services consume/produce.

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"

if [ ! -f "$ENV_FILE" ]; then
  echo "Preflight failed: missing .env file at $ENV_FILE"
  exit 1
fi

required_keys=(
  KAFKA_BROKERS
  KAFKA_TOPIC_SENSOR_READINGS
  KAFKA_TOPIC_ADMIN_COMMANDS
  KAFKA_TOPIC_ALERTS
  REDIS_ADDR
  POSTGRES_HOST
  POSTGRES_PORT
  POSTGRES_DB
  POSTGRES_USER
  POSTGRES_PASSWORD
  LOG_LEVEL
  ENVIRONMENT
)

missing=()
for key in "${required_keys[@]}"; do
  line="$(grep -E "^${key}=" "$ENV_FILE" || true)"
  if [ -z "$line" ] || [ -z "${line#*=}" ]; then
    missing+=("$key")
  fi
done

if [ "${#missing[@]}" -gt 0 ]; then
  echo "Preflight failed: missing required .env keys: ${missing[*]}"
  exit 1
fi

topics=(
  "$(grep -E '^KAFKA_TOPIC_SENSOR_READINGS=' "$ENV_FILE" | cut -d= -f2-)"
  "$(grep -E '^KAFKA_TOPIC_ADMIN_COMMANDS=' "$ENV_FILE" | cut -d= -f2-)"
  "$(grep -E '^KAFKA_TOPIC_ALERTS=' "$ENV_FILE" | cut -d= -f2-)"
)

for topic in "${topics[@]}"; do
  if ! [[ "$topic" =~ ^[a-zA-Z0-9._-]+$ ]]; then
    echo "Preflight failed: invalid Kafka topic name '$topic' (allowed: letters, numbers, ., _, -)"
    exit 1
  fi
done

if ! docker ps --format "{{.Names}}" | grep -q "^kafka$"; then
  echo "Preflight failed: kafka container is not running."
  exit 1
fi

echo "Preflight: ensuring Kafka topics exist..."
docker exec kafka kafka-topics --bootstrap-server localhost:9093 --create --topic "${topics[0]}" --partitions 6 --replication-factor 1 --if-not-exists >/dev/null 2>&1
docker exec kafka kafka-topics --bootstrap-server localhost:9093 --create --topic "${topics[1]}" --partitions 3 --replication-factor 1 --if-not-exists >/dev/null 2>&1
docker exec kafka kafka-topics --bootstrap-server localhost:9093 --create --topic "${topics[2]}" --partitions 3 --replication-factor 1 --if-not-exists >/dev/null 2>&1

echo "Preflight checks passed."
