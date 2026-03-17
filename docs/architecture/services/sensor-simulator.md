# Smart City Monitor — Sensor Simulator Service

## Overview

The sensor simulator is a Go service that generates synthetic environmental sensor readings and publishes them to Kafka. It is the sole data producer in the system.

**Entry point:** `backend/cmd/sensor-simulator/main.go`

## Responsibilities

- Generate realistic sensor readings for all active sensors every `GENERATION_INTERVAL_MS` milliseconds (default: 1000ms).
- Produce readings to the `sensor-readings` Kafka topic.
- Listen on the Redis `simulation:commands` Pub/Sub channel for runtime control commands.
- Sync active sensors from PostgreSQL every 5 seconds.
- Auto-create required Kafka topics on startup.
- Apply scenario-specific data generation profiles (normal, rush_hour, heatwave, industrial_incident).

## Sensor Types

| Type | Unit | Typical Range |
|------|------|---------------|
| temperature | celsius | 15–35 |
| pollution | µg/m³ (PM2.5) | 10–100 |
| humidity | % | 30–80 |
| noise | dB | 40–85 |

## Control Commands

The simulator listens on the Redis `simulation:commands` Pub/Sub channel for these commands:

| Command | Effect |
|---------|--------|
| `create_sensor` | Add a new sensor to the simulation loop |
| `delete_sensor` | Remove a sensor from the simulation loop |
| `shutdown_session` | Stop all sensors belonging to a session |

## Runtime Configuration

The simulator polls two Redis keys every 5 seconds:

- `simulation:config` — JSON of `SimulationConfig` (generation rate, thresholds, behavior overrides).
- `scenario:active` — JSON of the active scenario (changes data generation profiles).

## Circuit Breaker on Kafka Producer

The Kafka producer is wrapped with a circuit breaker from `internal/resilience`:

- **Closed state:** Normal operation. Failures increment a counter.
- **Open state (after threshold failures):** Kafka writes are skipped. Messages are buffered in-memory (up to 1000 messages). Prevents blocking the generation loop.
- **Half-open (after timeout):** A single message is attempted. Success closes the breaker; failure re-opens it.

Configuration: `CB_THRESHOLD` (default 5), `CB_TIMEOUT_SECONDS` (default 30).

## Message Format

Messages published to `sensor-readings` Kafka topic:

```json
{
  "sensor_id": "uuid",
  "sensor_type": "temperature",
  "value": 23.5,
  "unit": "celsius",
  "location": {
    "latitude": 40.7128,
    "longitude": -74.0060
  },
  "timestamp": "2025-01-15T10:30:00Z",
  "zone": "downtown",
  "session_id": "uuid-or-empty"
}
```

## Dependencies

| Dependency | Usage |
|------------|-------|
| Kafka | Produce sensor readings |
| PostgreSQL | Read active sensor registry |
| Redis | Subscribe to commands, read config/scenario |

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `SENSOR_COUNT` | 50 | Number of sensors to seed on first run |
| `GENERATION_INTERVAL_MS` | 1000 | Milliseconds between generation cycles |
| `SIMULATOR_START_EMPTY` | false | If true, start with no sensors (wait for UI creation) |
| `KAFKA_TOPIC_SENSOR_READINGS` | sensor-readings | Target Kafka topic |
| `CB_THRESHOLD` | 5 | Circuit breaker failure threshold |
| `CB_TIMEOUT_SECONDS` | 30 | Circuit breaker recovery timeout |
