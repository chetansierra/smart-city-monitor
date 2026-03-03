-- Enforce "existing sensor => active sensor" model.
UPDATE sensors
SET status = 'active'
WHERE status <> 'active';

ALTER TABLE sensors
DROP CONSTRAINT IF EXISTS sensors_status_check;

ALTER TABLE sensors
ADD CONSTRAINT sensors_status_check CHECK (status = 'active');

ALTER TABLE sensors
ALTER COLUMN status SET DEFAULT 'active';

COMMENT ON COLUMN sensors.status IS 'Derived state: if the sensor exists, it is active.';
