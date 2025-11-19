-- ================================================
-- Smart City Monitor - Monitoring Queries
-- ================================================

-- ================================================
-- 1. SYSTEM HEALTH CHECKS
-- ================================================

-- Database size
SELECT
    pg_size_pretty(pg_database_size('smart_city')) as database_size;

-- Table sizes
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size,
    pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)) AS table_size,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename) - pg_relation_size(schemaname||'.'||tablename)) AS indexes_size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Active connections
SELECT
    COUNT(*) as active_connections,
    COUNT(*) FILTER (WHERE state = 'active') as active_queries,
    COUNT(*) FILTER (WHERE state = 'idle') as idle_connections
FROM pg_stat_activity
WHERE datname = 'smart_city';

-- Long running queries
SELECT
    pid,
    now() - query_start AS duration,
    state,
    query
FROM pg_stat_activity
WHERE state != 'idle'
AND now() - query_start > interval '1 minute'
ORDER BY duration DESC;

-- ================================================
-- 2. DATA STATISTICS
-- ================================================

-- Total readings count
SELECT COUNT(*) as total_readings FROM sensor_readings;

-- Readings by sensor type
SELECT
    sensor_type,
    COUNT(*) as count,
    ROUND(AVG(value), 2) as avg_value,
    ROUND(MIN(value), 2) as min_value,
    ROUND(MAX(value), 2) as max_value,
    ROUND(STDDEV(value), 2) as stddev
FROM sensor_readings
GROUP BY sensor_type
ORDER BY count DESC;

-- Readings per hour (last 24 hours)
SELECT
    DATE_TRUNC('hour', timestamp) as hour,
    COUNT(*) as readings_count
FROM sensor_readings
WHERE timestamp > NOW() - INTERVAL '24 hours'
GROUP BY hour
ORDER BY hour DESC;

-- Readings per day (last 7 days)
SELECT
    DATE(timestamp) as day,
    COUNT(*) as readings_count,
    COUNT(DISTINCT sensor_id) as active_sensors
FROM sensor_readings
WHERE timestamp > NOW() - INTERVAL '7 days'
GROUP BY day
ORDER BY day DESC;

-- ================================================
-- 3. SENSOR HEALTH
-- ================================================

-- Active sensors (sent data in last 5 minutes)
SELECT
    COUNT(DISTINCT sensor_id) as active_sensors
FROM sensor_readings
WHERE timestamp > NOW() - INTERVAL '5 minutes';

-- Sensors by activity
SELECT
    s.id,
    s.name,
    s.type,
    COUNT(sr.id) as reading_count,
    MAX(sr.timestamp) as last_reading
FROM sensors s
LEFT JOIN sensor_readings sr ON s.id = sr.sensor_id
    AND sr.timestamp > NOW() - INTERVAL '1 hour'
GROUP BY s.id, s.name, s.type
ORDER BY reading_count DESC;

-- Inactive sensors (no data in last 10 minutes)
SELECT
    s.id,
    s.name,
    s.type,
    s.status,
    MAX(sr.timestamp) as last_reading,
    NOW() - MAX(sr.timestamp) as time_since_last_reading
FROM sensors s
LEFT JOIN sensor_readings sr ON s.id = sr.sensor_id
GROUP BY s.id, s.name, s.type, s.status
HAVING MAX(sr.timestamp) < NOW() - INTERVAL '10 minutes'
    OR MAX(sr.timestamp) IS NULL
ORDER BY last_reading DESC NULLS LAST;

-- ================================================
-- 4. ANOMALY DETECTION
-- ================================================

-- Readings outside normal ranges
SELECT
    sensor_id,
    sensor_type,
    value,
    unit,
    timestamp,
    CASE
        WHEN sensor_type = 'temperature' AND (value < 0 OR value > 50) THEN 'Temperature out of range'
        WHEN sensor_type = 'pollution' AND value > 200 THEN 'High pollution alert'
        WHEN sensor_type = 'humidity' AND (value < 0 OR value > 100) THEN 'Humidity out of range'
        WHEN sensor_type = 'noise' AND value > 100 THEN 'Extreme noise alert'
    END as alert_reason
FROM sensor_readings
WHERE timestamp > NOW() - INTERVAL '1 hour'
AND (
    (sensor_type = 'temperature' AND (value < 0 OR value > 50))
    OR (sensor_type = 'pollution' AND value > 200)
    OR (sensor_type = 'humidity' AND (value < 0 OR value > 100))
    OR (sensor_type = 'noise' AND value > 100)
)
ORDER BY timestamp DESC
LIMIT 20;

-- Sudden spikes (value changed by >50% from previous reading)
WITH ranked_readings AS (
    SELECT
        sensor_id,
        sensor_type,
        value,
        timestamp,
        LAG(value) OVER (PARTITION BY sensor_id ORDER BY timestamp) as prev_value
    FROM sensor_readings
    WHERE timestamp > NOW() - INTERVAL '1 hour'
)
SELECT
    sensor_id,
    sensor_type,
    prev_value,
    value,
    ROUND(((value - prev_value) / NULLIF(prev_value, 0) * 100)::numeric, 2) as percent_change,
    timestamp
FROM ranked_readings
WHERE prev_value IS NOT NULL
AND ABS((value - prev_value) / NULLIF(prev_value, 0)) > 0.5
ORDER BY timestamp DESC
LIMIT 20;

-- ================================================
-- 5. PERFORMANCE METRICS
-- ================================================

-- Data ingestion rate (readings per second)
SELECT
    COUNT(*) / EXTRACT(EPOCH FROM (MAX(timestamp) - MIN(timestamp))) as readings_per_second
FROM sensor_readings
WHERE timestamp > NOW() - INTERVAL '1 minute';

-- Partition statistics
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size,
    n_live_tup as row_count,
    last_vacuum,
    last_autovacuum
FROM pg_stat_user_tables
WHERE tablename LIKE 'sensor_readings%'
ORDER BY tablename;

-- Index usage statistics
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan as index_scans,
    idx_tup_read as tuples_read,
    idx_tup_fetch as tuples_fetched
FROM pg_stat_user_indexes
WHERE schemaname = 'public'
ORDER BY idx_scan DESC;

-- ================================================
-- 6. ENVIRONMENTAL INSIGHTS
-- ================================================

-- Average pollution by hour of day
SELECT
    EXTRACT(HOUR FROM timestamp) as hour_of_day,
    ROUND(AVG(value), 2) as avg_pollution,
    COUNT(*) as sample_count
FROM sensor_readings
WHERE sensor_type = 'pollution'
AND timestamp > NOW() - INTERVAL '7 days'
GROUP BY hour_of_day
ORDER BY hour_of_day;

-- Hotspot analysis (areas with highest average pollution)
SELECT
    sensor_id,
    ROUND(AVG(value), 2) as avg_pollution,
    ROUND(MAX(value), 2) as max_pollution,
    COUNT(*) as reading_count
FROM sensor_readings
WHERE sensor_type = 'pollution'
AND timestamp > NOW() - INTERVAL '24 hours'
GROUP BY sensor_id
ORDER BY avg_pollution DESC
LIMIT 10;

-- Temperature trends
SELECT
    DATE_TRUNC('hour', timestamp) as hour,
    ROUND(AVG(value), 2) as avg_temp,
    ROUND(MIN(value), 2) as min_temp,
    ROUND(MAX(value), 2) as max_temp
FROM sensor_readings
WHERE sensor_type = 'temperature'
AND timestamp > NOW() - INTERVAL '24 hours'
GROUP BY hour
ORDER BY hour DESC
LIMIT 24;

-- Noise levels by time of day
SELECT
    CASE
        WHEN EXTRACT(HOUR FROM timestamp) BETWEEN 0 AND 5 THEN 'Night (00-05)'
        WHEN EXTRACT(HOUR FROM timestamp) BETWEEN 6 AND 11 THEN 'Morning (06-11)'
        WHEN EXTRACT(HOUR FROM timestamp) BETWEEN 12 AND 17 THEN 'Afternoon (12-17)'
        ELSE 'Evening (18-23)'
    END as time_period,
    ROUND(AVG(value), 2) as avg_noise,
    ROUND(MAX(value), 2) as max_noise,
    COUNT(*) as sample_count
FROM sensor_readings
WHERE sensor_type = 'noise'
AND timestamp > NOW() - INTERVAL '7 days'
GROUP BY time_period
ORDER BY
    CASE time_period
        WHEN 'Night (00-05)' THEN 1
        WHEN 'Morning (06-11)' THEN 2
        WHEN 'Afternoon (12-17)' THEN 3
        ELSE 4
    END;
