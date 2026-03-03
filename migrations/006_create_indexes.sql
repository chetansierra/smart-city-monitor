-- Create indexes for optimized queries

-- Sensors table indexes
CREATE INDEX IF NOT EXISTS idx_sensors_type ON sensors(type);
CREATE INDEX IF NOT EXISTS idx_sensors_status ON sensors(status);
CREATE INDEX IF NOT EXISTS idx_sensors_location ON sensors(latitude, longitude);

-- Sensor readings indexes (applied to partitions)
CREATE INDEX IF NOT EXISTS idx_sensor_readings_sensor_id ON sensor_readings(sensor_id);
CREATE INDEX IF NOT EXISTS idx_sensor_readings_type ON sensor_readings(sensor_type);
CREATE INDEX IF NOT EXISTS idx_sensor_readings_timestamp ON sensor_readings(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_sensor_readings_sensor_timestamp ON sensor_readings(sensor_id, timestamp DESC);

-- Aggregates table indexes
CREATE INDEX IF NOT EXISTS idx_sensor_aggregates_sensor_id ON sensor_aggregates(sensor_id);
CREATE INDEX IF NOT EXISTS idx_sensor_aggregates_type ON sensor_aggregates(sensor_type);
CREATE INDEX IF NOT EXISTS idx_sensor_aggregates_period ON sensor_aggregates(period_start, period_end);

-- Add comments
COMMENT ON INDEX idx_sensors_type IS 'Index for filtering sensors by type';
COMMENT ON INDEX idx_sensor_readings_sensor_timestamp IS 'Composite index for time-series queries per sensor';
