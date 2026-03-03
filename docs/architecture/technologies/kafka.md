# Kafka Architecture Notes

## Role

Kafka is the event backbone between simulator and ingestion.

## Primary Topics

- `sensor-readings`
- `admin-commands`

## Service Usage

- Sensor simulator: producer to `sensor-readings`
- Data ingestion: consumer of `sensor-readings`
- API gateway: produces admin commands
