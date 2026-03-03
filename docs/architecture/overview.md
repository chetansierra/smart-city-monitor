# System Overview

Smart City Monitor is an event-driven system for sensor simulation, ingestion, storage, and live visualization.

## Core Flow

`Sensor Simulator -> Kafka(sensor-readings) -> Data Ingestion -> PostgreSQL + Redis -> API Gateway -> SSE -> Frontend`

## Runtime Services

- Frontend (Vite/React)
- API Gateway (Go/Fiber + SSE)
- Data Ingestion (Go Kafka consumer)
- Sensor Simulator (Go producer)
- PostgreSQL
- Redis
- Kafka + Zookeeper

## Architecture Principles

- SSE is the only real-time client transport.
- Redis is bounded by TTL + maxmemory policy to prevent unbounded growth.
- Session-scoped behavior (origin/radius/event history) is stored in Redis with expiry.
- Historical source of truth is PostgreSQL; Redis is for hot cache and coordination.
