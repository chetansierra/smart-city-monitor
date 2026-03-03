-- Add session_id column to sensors and readings for multi-tenancy isolation
ALTER TABLE sensors ADD COLUMN session_id UUID;
ALTER TABLE sensor_readings ADD COLUMN session_id UUID;

-- Create indexes for performance
CREATE INDEX idx_sensors_session_id ON sensors(session_id);
CREATE INDEX idx_sensor_readings_session_id ON sensor_readings(session_id);

-- Update existing data to have a "default" session ID if necessary (optional)
-- UPDATE sensors SET session_id = '00000000-0000-0000-0000-000000000000' WHERE session_id IS NULL;
