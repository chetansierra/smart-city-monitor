-- Create sensor_readings table (partitioned by timestamp)
CREATE TABLE IF NOT EXISTS sensor_readings (
    id BIGSERIAL,
    sensor_id UUID NOT NULL,
    sensor_type VARCHAR(50) NOT NULL,
    value DECIMAL(10, 2) NOT NULL,
    unit VARCHAR(20) NOT NULL,
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (timestamp, sensor_id)
) PARTITION BY RANGE (timestamp);

-- Add foreign key constraint
ALTER TABLE sensor_readings
ADD CONSTRAINT fk_sensor_readings_sensor_id
FOREIGN KEY (sensor_id) REFERENCES sensors(id) ON DELETE CASCADE;

-- Add comments
COMMENT ON TABLE sensor_readings IS 'Time-series data from sensor measurements, partitioned by month';
COMMENT ON COLUMN sensor_readings.value IS 'Measured value (temperature, PM2.5, humidity %, decibels)';
COMMENT ON COLUMN sensor_readings.unit IS 'Unit of measurement (celsius, µg/m³, %, dB)';
