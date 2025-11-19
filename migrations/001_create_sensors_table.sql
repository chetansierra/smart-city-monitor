-- Create sensors table
CREATE TABLE IF NOT EXISTS sensors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('temperature', 'pollution', 'humidity', 'noise')),
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'error')),
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Add comment to table
COMMENT ON TABLE sensors IS 'Physical sensor devices deployed across the smart city';
COMMENT ON COLUMN sensors.type IS 'Type of sensor: temperature, pollution, humidity, or noise';
COMMENT ON COLUMN sensors.status IS 'Operational status: active, inactive, or error';
COMMENT ON COLUMN sensors.config IS 'Additional configuration in JSON format';
