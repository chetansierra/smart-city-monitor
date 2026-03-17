# Smart City Monitor — PostgreSQL Architecture

## Role

PostgreSQL is the durable source of truth for sensor registration, pre-computed aggregates, anomaly events, and detected pattern events. It does NOT store raw sensor readings (that is a legacy table).

## Deployment

- **Image:** `postgres:16-alpine`
- **Port:** 5433 (host) → 5432 (container). Uses 5433 to avoid conflicts with local PostgreSQL installations.
- **Database:** `smart_city`
- **Credentials:** `admin` / `password` (development)
- **Volume:** `postgres-data`
- **Migrations:** Auto-applied from `migrations/` directory on first start via Docker entrypoint.

## Schema

### `sensors` Table

Sensor registry. Each sensor has a UUID primary key, type, location, status, and optional session_id.

```sql
CREATE TABLE sensors (
    id UUID PRIMARY KEY,
    name VARCHAR(100),
    type VARCHAR(50),        -- temperature, pollution, humidity, noise
    location GEOGRAPHY(POINT, 4326),
    status VARCHAR(20),
    config JSONB,
    session_id UUID,         -- NULL for system sensors, set for user-created
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

### `sensor_readings` Table (LEGACY — NOT WRITTEN TO)

This table exists in the schema but is no longer written to. Raw readings are NOT persisted to PostgreSQL. Stats come from Redis counters and `sensor_aggregates`.

```sql
CREATE TABLE sensor_readings (
    id BIGSERIAL,
    sensor_id UUID REFERENCES sensors(id),
    sensor_type VARCHAR(50),
    value DECIMAL(10, 2),
    unit VARCHAR(20),
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    timestamp TIMESTAMP NOT NULL,
    PRIMARY KEY (timestamp, sensor_id)
) PARTITION BY RANGE (timestamp);
```

### `sensor_aggregates` Table

Pre-computed hourly and daily aggregates. Populated by the data-ingestion aggregation worker every 5 minutes.

```sql
CREATE TABLE sensor_aggregates (
    id BIGSERIAL PRIMARY KEY,
    sensor_id UUID,
    sensor_type VARCHAR(50),
    aggregation_type VARCHAR(20),  -- hourly, daily
    avg_value DECIMAL(10, 2),
    min_value DECIMAL(10, 2),
    max_value DECIMAL(10, 2),
    count INTEGER,
    period_start TIMESTAMP,
    period_end TIMESTAMP
);
```

### `anomaly_events` Table

Stores z-score anomaly detections from the data-ingestion anomaly detector.

```sql
CREATE TABLE anomaly_events (
    id BIGSERIAL PRIMARY KEY,
    sensor_id UUID,
    sensor_type VARCHAR(50),
    value DECIMAL(10, 2),
    z_score DECIMAL(10, 4),
    expected_mean DECIMAL(10, 4),
    expected_stddev DECIMAL(10, 4),
    zone VARCHAR(50),
    detected_at TIMESTAMP
);
```

Migration: `migrations/010_stream_processing.sql`

### `detected_events` Table

Stores zone-level pattern detections (industrial_incident, heatwave, rush_hour).

```sql
CREATE TABLE detected_events (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(50),
    zone VARCHAR(50),
    severity VARCHAR(20),
    description TEXT,
    sensor_ids UUID[],
    detected_at TIMESTAMP
);
```

Migration: `migrations/010_stream_processing.sql`

## Data Flow

| Writer | Table | Frequency |
|--------|-------|-----------|
| Data Ingestion (aggregation worker) | `sensor_aggregates` | Every 5 minutes (in-memory flush) |
| Data Ingestion (anomaly detector) | `anomaly_events` | On detection (z-score > 2.5) |
| Data Ingestion (pattern detector) | `detected_events` | On detection (with 5-min debounce) |
| API Gateway | `sensors` | On sensor CRUD operations |

| Reader | Tables | Purpose |
|--------|--------|---------|
| API Gateway | `sensors`, `sensor_aggregates`, `anomaly_events`, `detected_events` | REST API responses |
| Sensor Simulator | `sensors` | Sync active sensor list every 5s |

## Queries Used by Analytics

Analytics endpoints serve data exclusively from pre-computed sources:

- **City stats:** Redis cache `analytics:city-stats:computed` (written by aggregation worker).
- **Zone stats:** Redis cache `analytics:zone:{zone}:{type}` (written by aggregation worker).
- **Hourly aggregations:** `SELECT` from `sensor_aggregates` table.
- **Anomalies:** `SELECT` from `anomaly_events` table with limit/since filters.
- **Detected events:** `SELECT` from `detected_events` table with limit/since filters.

There is NO fallback to querying raw `sensor_readings`.

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `POSTGRES_HOST` | 127.0.0.1 | Database host |
| `POSTGRES_PORT` | 5433 | Database port (host-mapped) |
| `POSTGRES_DB` | smart_city | Database name |
| `POSTGRES_USER` | admin | Database user |
| `POSTGRES_PASSWORD` | password | Database password |
| `POSTGRES_SSLMODE` | disable | SSL mode |
| `POSTGRES_MAX_CONNECTIONS` | 25 | Connection pool max |
| `POSTGRES_MAX_IDLE_CONNECTIONS` | 5 | Connection pool idle max |

## Troubleshooting

- **Missing aggregates:** Aggregation worker runs every 5 minutes. Check data-ingestion logs. Verify with `SELECT COUNT(*) FROM sensor_aggregates`.
- **Partition errors:** Monthly partitions for `sensor_readings` are defined in `migrations/003_create_partitions.sql`. Add new partitions for current/future months if needed.
- **Connection issues:** Verify port 5433 mapping. Check `docker compose logs postgres`.
- **Web UI:** Adminer is available at `http://localhost:8083` (server: `postgres`, user: `admin`, password: `password`, database: `smart_city`).
