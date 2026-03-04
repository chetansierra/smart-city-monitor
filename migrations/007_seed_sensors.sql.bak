-- Seed 50 sensors across a virtual smart city grid
-- City bounds: Latitude 40.70 to 40.80, Longitude -74.02 to -73.92 (simulating NYC-like area)

INSERT INTO sensors (name, type, latitude, longitude, status, config) VALUES
-- Temperature sensors (12)
('Downtown Temp 1', 'temperature', 40.7128, -74.0060, 'active', '{"calibration_offset": 0.0}'),
('Downtown Temp 2', 'temperature', 40.7180, -74.0100, 'active', '{"calibration_offset": 0.2}'),
('Midtown Temp 1', 'temperature', 40.7589, -73.9851, 'active', '{"calibration_offset": -0.1}'),
('Midtown Temp 2', 'temperature', 40.7614, -73.9776, 'active', '{"calibration_offset": 0.0}'),
('Uptown Temp 1', 'temperature', 40.7829, -73.9654, 'active', '{"calibration_offset": 0.3}'),
('Uptown Temp 2', 'temperature', 40.7889, -73.9565, 'active', '{"calibration_offset": -0.2}'),
('East Side Temp 1', 'temperature', 40.7489, -73.9680, 'active', '{"calibration_offset": 0.1}'),
('East Side Temp 2', 'temperature', 40.7520, -73.9615, 'active', '{"calibration_offset": 0.0}'),
('West Side Temp 1', 'temperature', 40.7395, -74.0027, 'active', '{"calibration_offset": -0.15}'),
('West Side Temp 2', 'temperature', 40.7450, -74.0100, 'active', '{"calibration_offset": 0.25}'),
('Harbor Temp 1', 'temperature', 40.7050, -74.0130, 'active', '{"calibration_offset": -0.3}'),
('Park Temp 1', 'temperature', 40.7812, -73.9665, 'active', '{"calibration_offset": 0.0}'),

-- Pollution sensors (PM2.5) (14)
('Downtown Air 1', 'pollution', 40.7080, -74.0080, 'active', '{"threshold_critical": 150}'),
('Downtown Air 2', 'pollution', 40.7150, -74.0050, 'active', '{"threshold_critical": 150}'),
('Industrial Air 1', 'pollution', 40.7200, -74.0140, 'active', '{"threshold_critical": 200}'),
('Industrial Air 2', 'pollution', 40.7250, -74.0180, 'active', '{"threshold_critical": 200}'),
('Midtown Air 1', 'pollution', 40.7580, -73.9800, 'active', '{"threshold_critical": 150}'),
('Midtown Air 2', 'pollution', 40.7630, -73.9740, 'active', '{"threshold_critical": 150}'),
('Highway Air 1', 'pollution', 40.7700, -73.9900, 'active', '{"threshold_critical": 180}'),
('Highway Air 2', 'pollution', 40.7450, -73.9950, 'active', '{"threshold_critical": 180}'),
('Residential Air 1', 'pollution', 40.7350, -73.9750, 'active', '{"threshold_critical": 120}'),
('Residential Air 2', 'pollution', 40.7400, -73.9700, 'active', '{"threshold_critical": 120}'),
('Park Air 1', 'pollution', 40.7795, -73.9632, 'active', '{"threshold_critical": 100}'),
('Park Air 2', 'pollution', 40.7685, -73.9815, 'active', '{"threshold_critical": 100}'),
('Waterfront Air 1', 'pollution', 40.7100, -74.0150, 'active', '{"threshold_critical": 130}'),
('Waterfront Air 2', 'pollution', 40.7320, -74.0060, 'active', '{"threshold_critical": 130}'),

-- Humidity sensors (12)
('Downtown Humidity 1', 'humidity', 40.7110, -74.0070, 'active', '{"sampling_rate": 60}'),
('Downtown Humidity 2', 'humidity', 40.7170, -74.0090, 'active', '{"sampling_rate": 60}'),
('Midtown Humidity 1', 'humidity', 40.7575, -73.9860, 'active', '{"sampling_rate": 60}'),
('Midtown Humidity 2', 'humidity', 40.7620, -73.9790, 'active', '{"sampling_rate": 60}'),
('Uptown Humidity 1', 'humidity', 40.7850, -73.9600, 'active', '{"sampling_rate": 60}'),
('Uptown Humidity 2', 'humidity', 40.7900, -73.9540, 'active', '{"sampling_rate": 60}'),
('East Humidity 1', 'humidity', 40.7510, -73.9650, 'active', '{"sampling_rate": 60}'),
('East Humidity 2', 'humidity', 40.7470, -73.9720, 'active', '{"sampling_rate": 60}'),
('West Humidity 1', 'humidity', 40.7380, -74.0050, 'active', '{"sampling_rate": 60}'),
('West Humidity 2', 'humidity', 40.7430, -74.0120, 'active', '{"sampling_rate": 60}'),
('Harbor Humidity 1', 'humidity', 40.7070, -74.0120, 'active', '{"sampling_rate": 60}'),
('Park Humidity 1', 'humidity', 40.7800, -73.9650, 'active', '{"sampling_rate": 60}'),

-- Noise sensors (12)
('Downtown Noise 1', 'noise', 40.7120, -74.0055, 'active', '{"threshold_high": 85}'),
('Downtown Noise 2', 'noise', 40.7160, -74.0085, 'active', '{"threshold_high": 85}'),
('Construction Noise 1', 'noise', 40.7220, -74.0110, 'active', '{"threshold_high": 90}'),
('Construction Noise 2', 'noise', 40.7270, -74.0150, 'active', '{"threshold_high": 90}'),
('Traffic Noise 1', 'noise', 40.7560, -73.9830, 'active', '{"threshold_high": 85}'),
('Traffic Noise 2', 'noise', 40.7610, -73.9770, 'active', '{"threshold_high": 85}'),
('Highway Noise 1', 'noise', 40.7680, -73.9880, 'active', '{"threshold_high": 88}'),
('Highway Noise 2', 'noise', 40.7440, -73.9930, 'active', '{"threshold_high": 88}'),
('Residential Noise 1', 'noise', 40.7360, -73.9770, 'active', '{"threshold_high": 70}'),
('Residential Noise 2', 'noise', 40.7410, -73.9710, 'active', '{"threshold_high": 70}'),
('Park Noise 1', 'noise', 40.7785, -73.9645, 'active', '{"threshold_high": 65}'),
('Quiet Zone Noise 1', 'noise', 40.7420, -73.9680, 'active', '{"threshold_high": 60}');

-- Verify count
SELECT
    type,
    COUNT(*) as sensor_count
FROM sensors
GROUP BY type
ORDER BY type;

SELECT
    'Total sensors created: ' || COUNT(*)::TEXT as summary
FROM sensors;
