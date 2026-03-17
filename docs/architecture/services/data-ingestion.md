# Smart City Monitor — Data Ingestion Service

## Overview

The data ingestion service is a Go service that consumes raw sensor readings from Kafka, performs real-time anomaly detection and zone-level pattern detection, and writes pre-computed aggregates to PostgreSQL. It does NOT persist raw readings.

**Entry point:** `backend/cmd/data-ingestion/main.go`

## Responsibilities

- Consume the `sensor-readings` Kafka topic with retry logic (3 retries, exponential backoff).
- Route failed messages to the `sensor-readings-dlq` dead letter queue after retry exhaustion.
- Run inline anomaly detection using Welford's online algorithm (z-score > 2.5 after 30 samples).
- Run zone-level pattern detection (industrial_incident, heatwave, rush_hour) on sliding windows.
- Publish detected anomalies to `sensor-anomalies` Kafka topic.
- Publish detected events to `sensor-events` Kafka topic.
- Cache latest readings in Redis (`sensor:latest:*`, `sensor:stream:*`).
- Maintain Redis counters: `stats:total_readings` (INCR), `stats:distinct_sensors` (HyperLogLog), `session:reading_stats:{session_id}`.
- Accumulate in-memory hourly aggregates and flush to `sensor_aggregates` PostgreSQL table every 5 minutes.
- Run background aggregation worker that caches city stats and zone stats in Redis (5-min TTL).
- Run Kafka consumer lag monitor (30-second interval).

## What It Does NOT Do

- It does NOT write raw sensor readings to PostgreSQL. The `sensor_readings` table is legacy and not written to.
- It does NOT use Redis Pub/Sub for real-time broadcasting. Anomalies and events go through Kafka topics.

## Processing Pipeline

```
Kafka (sensor-readings)
  → Deserialize message
  → Retry wrapper (3 retries, exponential backoff with jitter)
    → On exhaustion: send to DLQ (sensor-readings-dlq)
  → Cache in Redis (latest reading hash + bounded stream)
  → Increment Redis counters (total readings, HyperLogLog, session stats)
  → Anomaly detection (Welford's algorithm, per-sensor running stats)
    → If z-score > 2.5 and samples >= 30: publish to sensor-anomalies topic + store in anomaly_events table
  → Pattern detection (zone-level sliding windows)
    → If multi-sensor thresholds met (2-min window, 5-min debounce): publish to sensor-events topic + store in detected_events table
  → Accumulate in-memory hourly bucket
    → Every 5 minutes: flush to sensor_aggregates table
```

## Anomaly Detection

Uses Welford's online algorithm for O(1) per-reading computation:

- Maintains per-sensor running mean and variance.
- After 30 samples, computes z-score for each new reading.
- Z-score > 2.5 triggers an anomaly event.
- Anomaly events are stored in `anomaly_events` PostgreSQL table and published to `sensor-anomalies` Kafka topic.

Implementation: `backend/internal/anomaly/detector.go`

## Pattern Detection

Detects zone-level patterns using sliding windows:

| Pattern | Trigger Condition |
|---------|-------------------|
| `industrial_incident` | Multiple pollution + noise sensors in industrial zone exceed thresholds |
| `heatwave` | Multiple temperature sensors across zones exceed threshold |
| `rush_hour` | Multiple noise + pollution sensors in downtown/highway zones exceed thresholds |

- Window size: 2 minutes.
- Debounce: 5 minutes between detections of the same pattern.
- Detected events stored in `detected_events` PostgreSQL table and published to `sensor-events` Kafka topic.

Implementation: `backend/internal/anomaly/patterns.go`

## Circuit Breaker on Redis

Redis cache operations are wrapped with a circuit breaker:

- When Redis is unavailable, the circuit opens and Redis updates are skipped.
- Kafka consumption and anomaly detection continue uninterrupted.
- The circuit auto-recovers after `CB_TIMEOUT_SECONDS`.

## Aggregation Worker

A background goroutine runs every 5 minutes:

1. Flushes in-memory hourly reading buckets to `sensor_aggregates` table.
2. Computes and caches city-wide stats in Redis (`analytics:city-stats:computed`, 5-min TTL).
3. Computes and caches per-zone stats in Redis (`analytics:zone:{zone}:{type}`, 5-min TTL).

Implementation: `backend/internal/aggregation/worker.go`

## Dependencies

| Dependency | Usage |
|------------|-------|
| Kafka | Consume readings, produce anomalies/events, DLQ |
| PostgreSQL | Write aggregates, anomaly events, detected events |
| Redis | Cache latest readings, counters, analytics cache |

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `KAFKA_TOPIC_SENSOR_READINGS` | sensor-readings | Source topic |
| `KAFKA_TOPIC_DLQ` | sensor-readings-dlq | Dead letter queue topic |
| `KAFKA_TOPIC_ANOMALIES` | sensor-anomalies | Anomaly output topic |
| `KAFKA_TOPIC_EVENTS` | sensor-events | Pattern event output topic |
| `KAFKA_CONSUMER_GROUP_INGESTION` | data-ingestion-group | Consumer group ID |
| `MAX_RETRIES` | 3 | Retry count before DLQ |
| `RETRY_INITIAL_BACKOFF_MS` | 100 | Initial retry backoff |
| `CB_THRESHOLD` | 5 | Circuit breaker failure threshold |
| `CB_TIMEOUT_SECONDS` | 30 | Circuit breaker recovery timeout |
