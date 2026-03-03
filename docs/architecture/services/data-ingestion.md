# Data Ingestion Architecture

## Stack

- Go + Kafka consumer group
- PostgreSQL writer
- Redis cache updater

## Responsibilities

- Consume `sensor-readings` topic.
- Persist readings to PostgreSQL.
- Update Redis hot keys (`sensor:latest:*`, `sensor:stream:*`).
- Publish live update notifications to Redis pub/sub (`sensor:updates`).

## Notes

- PostgreSQL is the durable source.
- Redis is the live cache and transport trigger layer.
