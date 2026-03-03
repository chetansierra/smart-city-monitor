# Architecture Docs

This folder is the source of truth for system architecture.

## Structure

- `overview.md`: High-level architecture and core data flow.
- `services/`: Service-specific architecture and responsibilities.
- `technologies/`: Technology-specific operational notes and configuration.

## Service Docs

- `services/frontend.md`
- `services/api-gateway.md`
- `services/data-ingestion.md`
- `services/sensor-simulator.md`

## Technology Docs

- `technologies/redis.md`
- `technologies/kafka.md`
- `technologies/zookeeper.md`
- `technologies/postgresql.md`
- `technologies/sse.md`

## Documentation Rule

When a service/technology behavior changes, update the corresponding file here in the same PR.
