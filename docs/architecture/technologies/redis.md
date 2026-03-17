# Smart City Monitor — Redis Architecture

## Role

Redis serves as the hot cache, coordination layer, and control plane for Smart City Monitor. It is NOT the durable source of truth — PostgreSQL is. All Redis keys use TTLs or bounded structures to prevent unbounded memory growth.

## Deployment

- **Image:** `redis:7-alpine`
- **Port:** 6379
- **Persistence:** AOF (`--appendonly yes`)
- **Memory cap:** 256MB (`--maxmemory 256mb`)
- **Eviction policy:** `volatile-ttl` (only keys with TTL are evicted)
- **Volume:** `redis-data`

## Keyspace Design

### Sensor Cache

| Key Pattern | Type | TTL | Purpose |
|-------------|------|-----|---------|
| `sensor:latest:{sensor_id}` | Hash | 30m | Latest reading for a sensor |
| `sensor:stream:{sensor_id}` | Stream (max 300) | 30m | Recent readings for chart continuity |
| `pollution:leaderboard` | Sorted Set | — | PM2.5 leaderboard by value |

### Session State

| Key Pattern | Type | TTL | Purpose |
|-------------|------|-----|---------|
| `session:heartbeat:{session_id}` | String | 24h | Session liveness tracking |
| `session:config:{session_id}` | Hash | 24h | Per-session sensor configuration |
| `session:events:{session_id}` | Stream (max 180) | 90s | SSE replay buffer for reconnects |
| `session:reading_stats:{session_id}` | Hash | — | Per-session stats (total_readings, value_sum, last_reading_at) |

### Global Counters

| Key Pattern | Type | Purpose |
|-------------|------|---------|
| `stats:total_readings` | Counter (INCR) | Total readings processed across all time |
| `stats:distinct_sensors` | HyperLogLog | Approximate count of distinct sensor IDs that have produced data |
| `stats:visitors:total` | Counter | Total visitor count |
| `stats:visitors:seen_sessions` | Set | Unique session tracking |

### Pre-Computed Analytics Cache

| Key Pattern | Type | TTL | Purpose |
|-------------|------|-----|---------|
| `analytics:city-stats:computed` | String (JSON) | 5m | City-wide aggregated stats |
| `analytics:zone:{zone}:{type}` | String (JSON) | 5m | Per-zone per-type aggregated stats |

### Infrastructure Monitoring

| Key Pattern | Type | TTL | Purpose |
|-------------|------|-----|---------|
| `kafka:consumer-lag:{groupID}` | String (JSON) | — | Consumer lag snapshot (updated every 30s) |

### Control Plane

| Key Pattern | Type | Purpose |
|-------------|------|---------|
| `simulation:config` | String (JSON) | Runtime simulation configuration (polled by simulator every 5s) |
| `scenario:active` | String (JSON) | Active scenario (polled by simulator every 5s) |

### Pub/Sub Channels

| Channel | Publisher | Subscriber | Purpose |
|---------|-----------|------------|---------|
| `simulation:commands` | API Gateway (admin endpoints) | Sensor Simulator | Runtime sensor control (create, delete, shutdown) |

**Note:** `sensor:updates` and `sensor:anomalies` Pub/Sub channels are legacy. The API Gateway now consumes Kafka directly for real-time data.

## Write Path by Service

### Data Ingestion Writes

1. `sensor:latest:{id}` — Latest reading hash (HSET)
2. `sensor:stream:{id}` — Bounded stream entry (XADD MAXLEN)
3. `pollution:leaderboard` — Sorted set update (ZADD) for pollution sensors
4. `stats:total_readings` — Increment (INCR)
5. `stats:distinct_sensors` — HyperLogLog add (PFADD)
6. `session:reading_stats:{session_id}` — Per-session counters (HINCRBY)
7. `analytics:city-stats:computed` — Pre-computed city stats (SET with TTL, via aggregation worker)
8. `analytics:zone:{zone}:{type}` — Pre-computed zone stats (SET with TTL, via aggregation worker)

### API Gateway Writes/Reads

1. `session:heartbeat:{session_id}` — Heartbeat updates
2. `session:config:{session_id}` — Session configuration CRUD
3. `session:events:{session_id}` — SSE event history
4. `stats:visitors:*` — Visitor counters
5. `simulation:commands` — Pub/Sub publish for admin commands
6. Rate limiting keys — Per-endpoint rate limit counters

### Simulator Reads

1. `simulation:config` — Poll every 5s
2. `scenario:active` — Poll every 5s
3. `simulation:commands` — Subscribe (Pub/Sub)

## Circuit Breaker

Data ingestion wraps Redis operations with a circuit breaker. When Redis is unavailable:
- Redis cache updates are skipped silently.
- Kafka consumption and anomaly detection continue uninterrupted.
- The breaker auto-recovers after `CB_TIMEOUT_SECONDS`.

## TTL and Bounded Retention Policy

Defined in `backend/internal/redis/operations.go`:

| Constant | Value | Applied To |
|----------|-------|------------|
| `latestReadingTTL` | 30 minutes | `sensor:latest:*` |
| `sensorStreamTTL` | 30 minutes | `sensor:stream:*` |
| `sensorStreamMaxLen` | 300 | `sensor:stream:*` (MAXLEN) |
| `sessionHeartbeatTTL` | 24 hours | `session:heartbeat:*` |
| `sessionConfigTTL` | 24 hours | `session:config:*` |
| `sessionStreamTTL` | 90 seconds | `session:events:*` |
| `sessionStreamMaxLen` | 180 | `session:events:*` (MAXLEN) |

## Guardrails

- Every new high-cardinality key MUST have a TTL.
- Lists and streams MUST be bounded with MAXLEN.
- Payload timestamps use RFC3339Nano strings.
- Do not store large historical datasets in Redis.
- For new counters/sets, review memory impact at 10x expected sessions.

## Troubleshooting

- **Memory grows unexpectedly:** Check for keys missing TTL (`redis-cli --scan | head`). Verify eviction policy is `volatile-ttl`.
- **Missing live updates:** Verify API gateway is consuming Kafka (not Redis Pub/Sub). Check SSE connection with session ID.
- **Stale analytics cache:** Aggregation worker runs every 5 minutes. Check data-ingestion logs for worker errors.

## Operational Commands

```bash
redis-cli CONFIG GET maxmemory maxmemory-policy
redis-cli DBSIZE
redis-cli --bigkeys
redis-cli PUBSUB CHANNELS
redis-cli GET analytics:city-stats:computed
redis-cli HGETALL sensor:latest:{sensor_id}
```
