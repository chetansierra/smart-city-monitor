# Redis Configuration and Architecture

This document is the Redis source of truth for Smart City Monitor.

## Runtime Role

Redis is used for:

- Hot cache of latest sensor values
- Short time-series buffers for chart continuity
- Session heartbeat/config/event history
- Visitor/session counters for metrics
- Pub/Sub fanout trigger for SSE updates
- Simulation command channel for simulator control

Redis is not the long-term source of truth. PostgreSQL is.

## Deployment Configuration

From `docker-compose.yml`:

- Image: `redis:7-alpine`
- Persistence: `--appendonly yes`
- Memory cap: `--maxmemory 256mb`
- Eviction policy: `--maxmemory-policy volatile-ttl`

Implication:

- Only keys with TTL are eligible for eviction.
- Cache keys must always have TTL when growth is possible.

## Environment Variables

Configured via:

- `REDIS_ADDR`
- `REDIS_PASSWORD`
- `REDIS_DB`

Loaded in: `backend/internal/config/config.go`.

## Keyspace Design

### Sensor cache

- `sensor:latest:<sensor_id>` (hash)
- `sensor:stream:<sensor_id>` (stream, bounded)

### Session state

- `session:heartbeat:<session_id>`
- `session:config:<session_id>`
- `session:events:<session_id>`

### Counters

- `stats:visitors:total`
- `stats:visitors:seen_sessions`

### Channels

- `sensor:updates` (Pub/Sub)
- `simulation:commands` (Pub/Sub)

## TTL and Bounded Retention Policy

Defined in `backend/internal/redis/operations.go`:

- `latestReadingTTL = 30m`
- `sensorStreamTTL = 30m`
- `sensorStreamMaxLen = 300`
- `sessionHeartbeatTTL = 24h`
- `sessionConfigTTL = 24h`
- `sessionStreamTTL = 90s`
- `sessionStreamMaxLen = 180`

Operational intent:

- Preserve smooth post-refresh UX for recent data.
- Prevent unbounded per-user and per-sensor growth.
- Keep memory predictable under high concurrency.

## Write Path Usage

Data ingestion service writes:

1. Latest reading hash
2. Bounded sensor stream entry
3. Pub/Sub notification (`sensor:updates`)

API gateway writes/reads:

- Session heartbeat/config/events
- Visitor counters
- SSE history retrieval (`session:events:*`)

Simulator uses:

- Subscription to `simulation:commands`

## Operational Commands

Useful checks:

```bash
# memory policy
redis-cli CONFIG GET maxmemory maxmemory-policy

# key count
redis-cli DBSIZE

# top memory consumers (best effort)
redis-cli --bigkeys

# pubsub activity
redis-cli PUBSUB CHANNELS
```

## Guardrails and Practices

- Add TTL to every new high-cardinality key.
- Bound lists/streams with length caps.
- Keep payload format stable (`timestamp` uses RFC3339Nano strings).
- Do not store large historical datasets in Redis.
- For new counters/sets, review memory impact at 10x expected sessions.

## Troubleshooting

### Memory grows unexpectedly

- Check keys missing TTL: `redis-cli --scan | head`
- Verify eviction policy is `volatile-ttl`.
- Inspect new key patterns introduced by recent changes.

### Live UI missing updates

- Verify `sensor:updates` pub/sub activity.
- Verify API gateway SSE listener is running.
- Verify session ID is sent to `/stream`.
