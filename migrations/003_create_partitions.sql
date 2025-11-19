-- Create monthly partitions for sensor_readings
-- Current month and next 2 months

-- November 2024
CREATE TABLE IF NOT EXISTS sensor_readings_2024_11 PARTITION OF sensor_readings
    FOR VALUES FROM ('2024-11-01') TO ('2024-12-01');

-- December 2024
CREATE TABLE IF NOT EXISTS sensor_readings_2024_12 PARTITION OF sensor_readings
    FOR VALUES FROM ('2024-12-01') TO ('2025-01-01');

-- January 2025
CREATE TABLE IF NOT EXISTS sensor_readings_2025_01 PARTITION OF sensor_readings
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

-- February 2025
CREATE TABLE IF NOT EXISTS sensor_readings_2025_02 PARTITION OF sensor_readings
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

-- Add comments
COMMENT ON TABLE sensor_readings_2024_11 IS 'Partition for November 2024 sensor readings';
COMMENT ON TABLE sensor_readings_2024_12 IS 'Partition for December 2024 sensor readings';
COMMENT ON TABLE sensor_readings_2025_01 IS 'Partition for January 2025 sensor readings';
COMMENT ON TABLE sensor_readings_2025_02 IS 'Partition for February 2025 sensor readings';
