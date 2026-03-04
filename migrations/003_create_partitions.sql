-- Monthly partitions for sensor_readings
-- Add a new partition before each month begins.
-- A DEFAULT partition is kept as a safety net so inserts never fail
-- if a named partition for that month hasn't been added yet.

-- 2026 partitions
CREATE TABLE IF NOT EXISTS sensor_readings_2026_01 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_02 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_03 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_04 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_05 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_06 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_07 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_08 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_09 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_10 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_11 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2026_12 PARTITION OF sensor_readings
    FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

-- 2027 partitions (add more as needed)
CREATE TABLE IF NOT EXISTS sensor_readings_2027_01 PARTITION OF sensor_readings
    FOR VALUES FROM ('2027-01-01') TO ('2027-02-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2027_02 PARTITION OF sensor_readings
    FOR VALUES FROM ('2027-02-01') TO ('2027-03-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2027_03 PARTITION OF sensor_readings
    FOR VALUES FROM ('2027-03-01') TO ('2027-04-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2027_04 PARTITION OF sensor_readings
    FOR VALUES FROM ('2027-04-01') TO ('2027-05-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2027_05 PARTITION OF sensor_readings
    FOR VALUES FROM ('2027-05-01') TO ('2027-06-01');

CREATE TABLE IF NOT EXISTS sensor_readings_2027_06 PARTITION OF sensor_readings
    FOR VALUES FROM ('2027-06-01') TO ('2027-07-01');

-- Default partition catches any rows outside defined ranges.
-- This prevents insert failures when a new month partition hasn't been added yet.
CREATE TABLE IF NOT EXISTS sensor_readings_default PARTITION OF sensor_readings DEFAULT;
