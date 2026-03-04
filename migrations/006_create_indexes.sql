-- Indexes for optimized queries

-- sensors table
CREATE INDEX IF NOT EXISTS idx_sensors_type ON sensors(type);
CREATE INDEX IF NOT EXISTS idx_sensors_status ON sensors(status);
CREATE INDEX IF NOT EXISTS idx_sensors_location ON sensors(latitude, longitude);
CREATE INDEX IF NOT EXISTS idx_sensors_session_id ON sensors(session_id);

-- sensor_readings (applied to all partitions)
CREATE INDEX IF NOT EXISTS idx_sensor_readings_sensor_id ON sensor_readings(sensor_id);
CREATE INDEX IF NOT EXISTS idx_sensor_readings_type ON sensor_readings(sensor_type);
CREATE INDEX IF NOT EXISTS idx_sensor_readings_timestamp ON sensor_readings(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_sensor_readings_sensor_timestamp ON sensor_readings(sensor_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_sensor_readings_session_id ON sensor_readings(session_id);

-- sensor_aggregates
CREATE INDEX IF NOT EXISTS idx_sensor_aggregates_sensor_id ON sensor_aggregates(sensor_id);
CREATE INDEX IF NOT EXISTS idx_sensor_aggregates_session_id ON sensor_aggregates(session_id);
CREATE INDEX IF NOT EXISTS idx_sensor_aggregates_type ON sensor_aggregates(sensor_type);
CREATE INDEX IF NOT EXISTS idx_sensor_aggregates_period ON sensor_aggregates(period_start, period_end);

-- Comments
COMMENT ON INDEX idx_sensors_type IS 'Filter sensors by type';
COMMENT ON INDEX idx_sensor_readings_sensor_timestamp IS 'Composite index for time-series queries per sensor';
COMMENT ON INDEX idx_sensor_aggregates_session_id IS 'Filter aggregates by session';
