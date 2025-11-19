-- Create alerts table for threshold breach notifications
CREATE TABLE IF NOT EXISTS alerts (
    id BIGSERIAL PRIMARY KEY,
    sensor_id UUID NOT NULL,
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    message TEXT NOT NULL,
    value DECIMAL(10, 2) NOT NULL,
    threshold DECIMAL(10, 2) NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    acknowledged BOOLEAN NOT NULL DEFAULT FALSE,
    acknowledged_at TIMESTAMP,
    acknowledged_by VARCHAR(100),
    CONSTRAINT fk_alerts_sensor_id
        FOREIGN KEY (sensor_id) REFERENCES sensors(id) ON DELETE CASCADE
);

-- Add comments
COMMENT ON TABLE alerts IS 'Alerts generated when sensor values breach defined thresholds';
COMMENT ON COLUMN alerts.alert_type IS 'Type of alert (e.g., high_temperature, poor_air_quality)';
COMMENT ON COLUMN alerts.severity IS 'Alert severity level: low, medium, high, or critical';
COMMENT ON COLUMN alerts.acknowledged IS 'Whether the alert has been acknowledged by an operator';
