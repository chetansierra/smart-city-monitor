-- Create monthly partitions for sensor_readings
-- Current month and future months

-- 2025 Partitions
CREATE TABLE IF NOT EXISTS sensor_readings_2025_01 PARTITION OF sensor_readings
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2025_02 PARTITION OF sensor_readings
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

-- 2026 Partitions (Current Year)
CREATE TABLE IF NOT EXISTS sensor_readings_2026_01 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_02 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_03 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

-- Default partition to prevent crashes
CREATE TABLE IF NOT EXISTS sensor_readings_default PARTITION OF sensor_readings DEFAULT;

-- Add comments
COMMENT ON TABLE sensor_readings_2026_01 IS 'Partition for January 2026 sensor readings';
COMMENT ON TABLE sensor_readings_2026_02 IS 'Partition for February 2026 sensor readings';
COMMENT ON TABLE sensor_readings_2026_03 IS 'Partition for March 2026 sensor readings';
COMMENT ON TABLE sensor_readings_default IS 'Default partition for sensor readings';
