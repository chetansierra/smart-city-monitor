package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/google/uuid"
)

// GetAllSensors retrieves all sensors from the database
func (db *DB) GetAllSensors(ctx context.Context) ([]models.Sensor, error) {
	query := `
		SELECT id, session_id, name, type, latitude, longitude, status, config, created_at, updated_at
		FROM sensors
		ORDER BY name
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sensors: %w", err)
	}
	defer rows.Close()

	var sensors []models.Sensor
	for rows.Next() {
		var s models.Sensor
		err := rows.Scan(
			&s.ID, &s.SessionID, &s.Name, &s.Type, &s.Latitude, &s.Longitude,
			&s.Status, &s.Config, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sensor: %w", err)
		}

		// Populate location from lat/long
		s.Location = models.Location{
			Latitude:  s.Latitude,
			Longitude: s.Longitude,
		}

		sensors = append(sensors, s)
	}

	return sensors, rows.Err()
}

// GetSensorByID retrieves a single sensor by ID
func (db *DB) GetSensorByID(ctx context.Context, id uuid.UUID) (*models.Sensor, error) {
	query := `
		SELECT id, session_id, name, type, latitude, longitude, status, config, created_at, updated_at
		FROM sensors
		WHERE id = $1
	`

	var s models.Sensor
	err := db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.SessionID, &s.Name, &s.Type, &s.Latitude, &s.Longitude,
		&s.Status, &s.Config, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get sensor: %w", err)
	}

	s.Location = models.Location{
		Latitude:  s.Latitude,
		Longitude: s.Longitude,
	}

	return &s, nil
}

// GetSensorsByType retrieves sensors by type
func (db *DB) GetSensorsByType(ctx context.Context, sensorType models.SensorType) ([]models.Sensor, error) {
	query := `
		SELECT id, session_id, name, type, latitude, longitude, status, config, created_at, updated_at
		FROM sensors
		WHERE type = $1
		ORDER BY name
	`

	rows, err := db.QueryContext(ctx, query, sensorType)
	if err != nil {
		return nil, fmt.Errorf("failed to query sensors by type: %w", err)
	}
	defer rows.Close()

	var sensors []models.Sensor
	for rows.Next() {
		var s models.Sensor
		err := rows.Scan(
			&s.ID, &s.SessionID, &s.Name, &s.Type, &s.Latitude, &s.Longitude,
			&s.Status, &s.Config, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sensor: %w", err)
		}

		s.Location = models.Location{
			Latitude:  s.Latitude,
			Longitude: s.Longitude,
		}

		sensors = append(sensors, s)
	}

	return sensors, rows.Err()
}

// CountSensorsBySession returns how many sensors belong to a session.
func (db *DB) CountSensorsBySession(ctx context.Context, sessionID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM sensors WHERE session_id = $1`
	var count int
	if err := db.QueryRowContext(ctx, query, sessionID).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count sensors for session: %w", err)
	}
	return count, nil
}

// UpdateSensorsStatus updates the status for the provided sensor IDs.
// Since migration 009 enforces status = 'active', this is effectively a no-op
// that confirms the sensors exist. Only call with SensorStatusActive.
func (db *DB) UpdateSensorsStatus(ctx context.Context, sensorIDs []uuid.UUID, status models.SensorStatus) error {
	if len(sensorIDs) == 0 {
		return nil
	}

	args := make([]interface{}, 0, len(sensorIDs)+1)
	args = append(args, status)

	placeholders := make([]string, 0, len(sensorIDs))
	for i, id := range sensorIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+2))
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		UPDATE sensors
		SET status = $1, updated_at = NOW()
		WHERE id IN (%s)
	`, strings.Join(placeholders, ", "))

	_, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update sensor statuses: %w", err)
	}

	return nil
}

// DeleteSensorByIDAndSession deletes a sensor owned by the given session.
func (db *DB) DeleteSensorByIDAndSession(ctx context.Context, id uuid.UUID, sessionID uuid.UUID) (bool, error) {
	query := `
		DELETE FROM sensors
		WHERE id = $1 AND session_id = $2
	`

	result, err := db.ExecContext(ctx, query, id, sessionID)
	if err != nil {
		return false, fmt.Errorf("failed to delete sensor: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to inspect delete result: %w", err)
	}

	return rowsAffected > 0, nil
}

// CountAllActiveSensors returns the total number of sensors in the database.
func (db *DB) CountAllActiveSensors(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM sensors`
	var count int
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count all sensors: %w", err)
	}
	return count, nil
}

// GetSensorsOlderThan returns sensors created more than the given duration ago.
func (db *DB) GetSensorsOlderThan(ctx context.Context, age time.Duration) ([]models.Sensor, error) {
	query := `
		SELECT id, session_id, name, type, latitude, longitude, status, config, created_at, updated_at
		FROM sensors
		WHERE created_at < NOW() - $1::interval
	`

	rows, err := db.QueryContext(ctx, query, fmt.Sprintf("%d seconds", int(age.Seconds())))
	if err != nil {
		return nil, fmt.Errorf("failed to query expired sensors: %w", err)
	}
	defer rows.Close()

	var sensors []models.Sensor
	for rows.Next() {
		var s models.Sensor
		if err := rows.Scan(
			&s.ID, &s.SessionID, &s.Name, &s.Type, &s.Latitude, &s.Longitude,
			&s.Status, &s.Config, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan expired sensor: %w", err)
		}
		s.Location = models.Location{Latitude: s.Latitude, Longitude: s.Longitude}
		sensors = append(sensors, s)
	}
	return sensors, rows.Err()
}

// GetSensorsBySession returns all sensors belonging to a session.
func (db *DB) GetSensorsBySession(ctx context.Context, sessionID uuid.UUID) ([]models.Sensor, error) {
	query := `
		SELECT id, session_id, name, type, latitude, longitude, status, config, created_at, updated_at
		FROM sensors
		WHERE session_id = $1
	`
	rows, err := db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query sensors by session: %w", err)
	}
	defer rows.Close()

	var sensors []models.Sensor
	for rows.Next() {
		var s models.Sensor
		if err := rows.Scan(
			&s.ID, &s.SessionID, &s.Name, &s.Type, &s.Latitude, &s.Longitude,
			&s.Status, &s.Config, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session sensor: %w", err)
		}
		s.Location = models.Location{Latitude: s.Latitude, Longitude: s.Longitude}
		sensors = append(sensors, s)
	}
	return sensors, rows.Err()
}

// DeleteSensorsBySession deletes all sensors belonging to a session and returns the count.
func (db *DB) DeleteSensorsBySession(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	query := `DELETE FROM sensors WHERE session_id = $1`
	result, err := db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete session sensors: %w", err)
	}
	return result.RowsAffected()
}

// DeleteSensorByID deletes a sensor by ID regardless of session ownership.
func (db *DB) DeleteSensorByID(ctx context.Context, id uuid.UUID) (bool, error) {
	query := `DELETE FROM sensors WHERE id = $1`
	result, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return false, fmt.Errorf("failed to delete sensor: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to inspect delete result: %w", err)
	}
	return rowsAffected > 0, nil
}

// InsertAnomalyEvent inserts an anomaly event into the database.
func (db *DB) InsertAnomalyEvent(ctx context.Context, event *models.AnomalyEvent) error {
	query := `
		INSERT INTO anomaly_events (sensor_id, sensor_type, value, expected_mean, expected_stddev, z_score, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := db.ExecContext(ctx, query,
		event.SensorID, event.SensorType, event.Value,
		event.ExpectedMean, event.ExpectedStdDev, event.ZScore, event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to insert anomaly event: %w", err)
	}
	return nil
}

// InsertDetectedEvent inserts a detected pattern event into the database.
func (db *DB) InsertDetectedEvent(ctx context.Context, event *models.DetectedEvent) error {
	sensorIDStrs := make([]string, len(event.SensorIDs))
	for i, id := range event.SensorIDs {
		sensorIDStrs[i] = id.String()
	}

	query := `
		INSERT INTO detected_events (event_type, zone, description, sensor_ids, timestamp, expires_at)
		VALUES ($1, $2, $3, $4::uuid[], $5, $6)
	`
	_, err := db.ExecContext(ctx, query,
		event.EventType, event.Zone, event.Description,
		"{"+strings.Join(sensorIDStrs, ",")+"}",
		event.Timestamp, event.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert detected event: %w", err)
	}
	return nil
}

// GetRecentAnomalies returns recent anomaly events.
func (db *DB) GetRecentAnomalies(ctx context.Context, limit int, since time.Time) ([]models.AnomalyEvent, error) {
	query := `
		SELECT id, sensor_id, sensor_type, value, expected_mean, expected_stddev, z_score, timestamp, created_at
		FROM anomaly_events
		WHERE timestamp >= $1
		ORDER BY timestamp DESC
		LIMIT $2
	`
	rows, err := db.QueryContext(ctx, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query anomaly events: %w", err)
	}
	defer rows.Close()

	var events []models.AnomalyEvent
	for rows.Next() {
		var e models.AnomalyEvent
		if err := rows.Scan(&e.ID, &e.SensorID, &e.SensorType, &e.Value,
			&e.ExpectedMean, &e.ExpectedStdDev, &e.ZScore, &e.Timestamp, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan anomaly event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetRecentDetectedEvents returns recent detected events.
func (db *DB) GetRecentDetectedEvents(ctx context.Context, limit int, since time.Time) ([]models.DetectedEvent, error) {
	query := `
		SELECT id, event_type, zone, description, sensor_ids, timestamp, expires_at, created_at
		FROM detected_events
		WHERE timestamp >= $1
		ORDER BY timestamp DESC
		LIMIT $2
	`
	rows, err := db.QueryContext(ctx, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query detected events: %w", err)
	}
	defer rows.Close()

	var events []models.DetectedEvent
	for rows.Next() {
		var e models.DetectedEvent
		var sensorIDsStr string
		if err := rows.Scan(&e.ID, &e.EventType, &e.Zone, &e.Description,
			&sensorIDsStr, &e.Timestamp, &e.ExpiresAt, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan detected event: %w", err)
		}
		// Parse PostgreSQL UUID array format: {uuid1,uuid2,...}
		if len(sensorIDsStr) > 2 {
			raw := sensorIDsStr[1 : len(sensorIDsStr)-1] // strip { }
			for _, idStr := range strings.Split(raw, ",") {
				if id, parseErr := uuid.Parse(strings.TrimSpace(idStr)); parseErr == nil {
					e.SensorIDs = append(e.SensorIDs, id)
				}
			}
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetHourlyAggregates returns pre-computed hourly aggregates for a sensor.
func (db *DB) GetHourlyAggregates(ctx context.Context, sensorID uuid.UUID, from, to time.Time) ([]models.SensorAggregate, error) {
	query := `
		SELECT id, sensor_id, sensor_type, aggregation_type, avg_value, min_value, max_value, period_start, period_end
		FROM sensor_aggregates
		WHERE sensor_id = $1 AND aggregation_type = 'hourly' AND period_start >= $2 AND period_start <= $3
		ORDER BY period_start DESC
	`
	rows, err := db.QueryContext(ctx, query, sensorID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query hourly aggregates: %w", err)
	}
	defer rows.Close()

	var aggregates []models.SensorAggregate
	for rows.Next() {
		var a models.SensorAggregate
		if err := rows.Scan(&a.ID, &a.SensorID, &a.SensorType,
			&a.AggregationType, &a.AvgValue, &a.MinValue, &a.MaxValue,
			&a.PeriodStart, &a.PeriodEnd); err != nil {
			return nil, fmt.Errorf("failed to scan aggregate: %w", err)
		}
		aggregates = append(aggregates, a)
	}
	return aggregates, rows.Err()
}

// GetHourlyAggregatesForAll returns hourly aggregates for all sensors in a time range, optionally filtered by type.
func (db *DB) GetHourlyAggregatesForAll(ctx context.Context, from, to time.Time, sensorType string) ([]models.SensorAggregate, error) {
	var query string
	var args []interface{}

	if sensorType != "" {
		query = `
			SELECT id, sensor_id, sensor_type, aggregation_type, avg_value, min_value, max_value, period_start, period_end
			FROM sensor_aggregates
			WHERE aggregation_type = 'hourly' AND period_start >= $1 AND period_start <= $2 AND sensor_type = $3
			ORDER BY period_start DESC
		`
		args = []interface{}{from, to, sensorType}
	} else {
		query = `
			SELECT id, sensor_id, sensor_type, aggregation_type, avg_value, min_value, max_value, period_start, period_end
			FROM sensor_aggregates
			WHERE aggregation_type = 'hourly' AND period_start >= $1 AND period_start <= $2
			ORDER BY period_start DESC
		`
		args = []interface{}{from, to}
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query hourly aggregates: %w", err)
	}
	defer rows.Close()

	var aggregates []models.SensorAggregate
	for rows.Next() {
		var a models.SensorAggregate
		if err := rows.Scan(&a.ID, &a.SensorID, &a.SensorType,
			&a.AggregationType, &a.AvgValue, &a.MinValue, &a.MaxValue,
			&a.PeriodStart, &a.PeriodEnd); err != nil {
			return nil, fmt.Errorf("failed to scan aggregate: %w", err)
		}
		aggregates = append(aggregates, a)
	}
	return aggregates, rows.Err()
}

// InsertSensor inserts a new sensor into the database
func (db *DB) InsertSensor(ctx context.Context, s *models.Sensor) error {
	query := `
		INSERT INTO sensors (id, session_id, name, type, latitude, longitude, status, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	now := time.Now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = now
	}
	if s.Config == "" {
		// sensors.config is JSONB; keep inserts valid even when config is omitted.
		s.Config = "{}"
	}

	_, err := db.ExecContext(
		ctx, query,
		s.ID, s.SessionID, s.Name, s.Type, s.Latitude, s.Longitude,
		s.Status, s.Config, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert sensor: %w", err)
	}

	return nil
}
