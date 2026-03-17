# Smart City Monitor — Anomaly Detection and Pattern Detection

## Overview

Smart City Monitor runs two levels of real-time intelligence in the data-ingestion service:

1. **Per-sensor anomaly detection** using Welford's online algorithm.
2. **Zone-level pattern detection** using sliding windows with multi-sensor correlation.

Both run inline during Kafka message processing with O(1) per-reading overhead.

## Anomaly Detection

Implementation: `backend/internal/anomaly/detector.go`

### Algorithm: Welford's Online Method

Welford's algorithm computes running mean and variance in a single pass with O(1) time and O(1) space per reading. No historical data needs to be stored or queried.

For each new reading value `x` for sensor `s`:

1. Increment sample count: `n += 1`
2. Compute delta: `delta = x - mean`
3. Update mean: `mean += delta / n`
4. Compute delta2: `delta2 = x - mean`
5. Update M2: `M2 += delta * delta2`
6. Variance: `variance = M2 / (n - 1)` (when n > 1)
7. Stddev: `stddev = sqrt(variance)`
8. Z-score: `z = abs(x - mean) / stddev`

### Triggering Conditions

| Condition | Threshold |
|-----------|-----------|
| Minimum samples before detection | 30 |
| Z-score threshold for anomaly | > 2.5 |

An anomaly is triggered when both conditions are met simultaneously.

### Anomaly Event Output

When an anomaly is detected:

1. An `AnomalyEvent` record is inserted into the `anomaly_events` PostgreSQL table.
2. The event is published to the `sensor-anomalies` Kafka topic.
3. The API Gateway consumes this topic and broadcasts it to SSE clients as an `anomaly` event type.

### Anomaly Event Schema

```json
{
  "sensor_id": "uuid",
  "sensor_type": "temperature",
  "value": 45.2,
  "z_score": 3.1,
  "expected_mean": 22.5,
  "expected_stddev": 7.3,
  "zone": "downtown",
  "detected_at": "2025-01-15T10:30:00Z"
}
```

### Querying Anomalies

```
GET /api/v1/analytics/anomalies?limit=50&since=2025-01-15T00:00:00Z
```

## Pattern Detection

Implementation: `backend/internal/anomaly/patterns.go`

### Concept

Pattern detection correlates multiple sensors within a geographic zone to detect city-level events. Unlike anomaly detection (per-sensor), pattern detection operates at the zone level.

### Supported Patterns

| Pattern | Zone(s) | Trigger Condition |
|---------|---------|-------------------|
| `industrial_incident` | Industrial zones | Multiple pollution AND noise sensors exceed thresholds simultaneously |
| `heatwave` | All zones | Multiple temperature sensors across different zones exceed threshold |
| `rush_hour` | Downtown, Highway | Multiple noise AND pollution sensors exceed thresholds simultaneously |

### Sliding Window Configuration

| Parameter | Value |
|-----------|-------|
| Window size | 2 minutes |
| Debounce interval | 5 minutes (per pattern type) |

The sliding window collects recent readings per zone. When enough sensors within a zone exceed thresholds within the window, a pattern is detected. The debounce prevents repeated firing of the same pattern.

### Detected Event Output

When a pattern is detected:

1. A `DetectedEvent` record is inserted into the `detected_events` PostgreSQL table.
2. The event is published to the `sensor-events` Kafka topic.
3. The API Gateway consumes this topic and broadcasts it to SSE clients as a `scenario_detected` event type.

### Detected Event Schema

```json
{
  "event_type": "industrial_incident",
  "zone": "industrial",
  "severity": "high",
  "description": "Elevated pollution and noise levels detected in industrial zone",
  "sensor_ids": ["uuid1", "uuid2", "uuid3"],
  "detected_at": "2025-01-15T10:30:00Z"
}
```

### Querying Detected Events

```
GET /api/v1/analytics/events?limit=20&since=2025-01-15T00:00:00Z
```

## Zone Definitions

Zones are defined in `backend/internal/zones/`. The `GetZone(lat, lng)` helper maps a geographic coordinate to a named zone. Zones are used by:

- Pattern detector (zone-level correlation)
- Aggregation worker (zone-level stats)
- Analytics API (zone-level queries)
- Kafka partitioner (zone-based routing)

## Relationship to Scenarios

Scenarios (rush_hour, heatwave, industrial_incident) alter the sensor simulator's data generation profiles, making it more likely that the corresponding patterns will be detected. However, pattern detection is independent of scenarios — it can detect patterns even under the `normal` scenario if sensor values naturally correlate.
