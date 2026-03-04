-- Create sensor_aggregates table for pre-computed statistics
CREATE TABLE IF NOT EXISTS sensor_aggregates (
    id BIGSERIAL PRIMARY KEY,
    sensor_id UUID,
    session_id UUID,
    sensor_type VARCHAR(50) NOT NULL,
    aggregation_type VARCHAR(20) NOT NULL CHECK (aggregation_type IN ('hourly', 'daily')),
    avg_value DECIMAL(10, 2) NOT NULL,
    min_value DECIMAL(10, 2) NOT NULL,
    max_value DECIMAL(10, 2) NOT NULL,
    period_start TIMESTAMP NOT NULL,
    period_end TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_sensor_aggregates_sensor_id
        FOREIGN KEY (sensor_id) REFERENCES sensors(id) ON DELETE CASCADE
);

-- Add unique constraint to prevent duplicate aggregations
CREATE UNIQUE INDEX IF NOT EXISTS idx_sensor_aggregates_unique
    ON sensor_aggregates(sensor_id, aggregation_type, period_start);

-- Add comments
COMMENT ON TABLE sensor_aggregates IS 'Pre-computed hourly and daily statistics for sensor data';
COMMENT ON COLUMN sensor_aggregates.aggregation_type IS 'Type of aggregation: hourly or daily';
COMMENT ON COLUMN sensor_aggregates.session_id IS 'Session that owns this sensor, NULL for seeded sensors';
