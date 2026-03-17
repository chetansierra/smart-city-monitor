-- Anomaly events table for z-score based anomaly detection
CREATE TABLE IF NOT EXISTS anomaly_events (
    id BIGSERIAL PRIMARY KEY,
    sensor_id UUID NOT NULL REFERENCES sensors(id) ON DELETE CASCADE,
    sensor_type VARCHAR(50) NOT NULL,
    value DECIMAL(10,2) NOT NULL,
    expected_mean DECIMAL(10,2) NOT NULL,
    expected_stddev DECIMAL(10,2) NOT NULL,
    z_score DECIMAL(6,2) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_anomaly_events_ts ON anomaly_events(timestamp DESC);

-- Detected events table for zone-level pattern detection
CREATE TABLE IF NOT EXISTS detected_events (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    zone VARCHAR(50) NOT NULL,
    description TEXT,
    sensor_ids UUID[] NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_detected_events_ts ON detected_events(timestamp DESC);

-- Add reading_count column to sensor_aggregates for better analytics
ALTER TABLE sensor_aggregates ADD COLUMN IF NOT EXISTS reading_count INT DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_sensor_aggregates_period ON sensor_aggregates(period_start, aggregation_type);
