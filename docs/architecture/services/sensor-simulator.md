# Sensor Simulator Architecture

## Stack

- Go producer + Redis subscriber

## Responsibilities

- Generate synthetic sensor readings by sensor type.
- Produce readings to Kafka topic `sensor-readings`.
- Listen on Redis channel `simulation:commands` for control commands.

## Control Commands

- `create_sensor`
- `delete_sensor`
- `shutdown_session`
