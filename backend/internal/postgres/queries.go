package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/google/uuid"
)

// GetAllSensors retrieves all sensors from the database
func (db *DB) GetAllSensors(ctx context.Context) ([]models.Sensor, error) {
	query := `
		SELECT id, name, type, latitude, longitude, status, config, created_at, updated_at
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
			&s.ID, &s.Name, &s.Type, &s.Latitude, &s.Longitude,
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
		SELECT id, name, type, latitude, longitude, status, config, created_at, updated_at
		FROM sensors
		WHERE id = $1
	`

	var s models.Sensor
	err := db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.Name, &s.Type, &s.Latitude, &s.Longitude,
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
		SELECT id, name, type, latitude, longitude, status, config, created_at, updated_at
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
			&s.ID, &s.Name, &s.Type, &s.Latitude, &s.Longitude,
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

// InsertSensorReading inserts a new sensor reading
func (db *DB) InsertSensorReading(ctx context.Context, reading *models.SensorReading) error {
	query := `
		INSERT INTO sensor_readings (sensor_id, sensor_type, value, unit, latitude, longitude, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := db.ExecContext(
		ctx, query,
		reading.SensorID, reading.SensorType, reading.Value, reading.Unit,
		reading.Latitude, reading.Longitude, reading.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to insert sensor reading: %w", err)
	}

	return nil
}

// InsertSensorReadingsBatch inserts multiple sensor readings in a single transaction
func (db *DB) InsertSensorReadingsBatch(ctx context.Context, readings []models.SensorReading) error {
	if len(readings) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO sensor_readings (sensor_id, sensor_type, value, unit, latitude, longitude, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, reading := range readings {
		_, err := stmt.ExecContext(
			ctx,
			reading.SensorID, reading.SensorType, reading.Value, reading.Unit,
			reading.Latitude, reading.Longitude, reading.Timestamp,
		)
		if err != nil {
			return fmt.Errorf("failed to insert reading: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetRecentReadings retrieves recent readings for a sensor
func (db *DB) GetRecentReadings(ctx context.Context, sensorID uuid.UUID, limit int) ([]models.SensorReading, error) {
	query := `
		SELECT id, sensor_id, sensor_type, value, unit, latitude, longitude, timestamp
		FROM sensor_readings
		WHERE sensor_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := db.QueryContext(ctx, query, sensorID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent readings: %w", err)
	}
	defer rows.Close()

	var readings []models.SensorReading
	for rows.Next() {
		var r models.SensorReading
		err := rows.Scan(
			&r.ID, &r.SensorID, &r.SensorType, &r.Value, &r.Unit,
			&r.Latitude, &r.Longitude, &r.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan reading: %w", err)
		}

		r.Location = models.Location{
			Latitude:  r.Latitude,
			Longitude: r.Longitude,
		}

		readings = append(readings, r)
	}

	return readings, rows.Err()
}

// GetReadingsInTimeRange retrieves readings within a time range
func (db *DB) GetReadingsInTimeRange(ctx context.Context, sensorID uuid.UUID, from, to time.Time) ([]models.SensorReading, error) {
	query := `
		SELECT id, sensor_id, sensor_type, value, unit, latitude, longitude, timestamp
		FROM sensor_readings
		WHERE sensor_id = $1 AND timestamp >= $2 AND timestamp <= $3
		ORDER BY timestamp DESC
	`

	rows, err := db.QueryContext(ctx, query, sensorID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query readings in time range: %w", err)
	}
	defer rows.Close()

	var readings []models.SensorReading
	for rows.Next() {
		var r models.SensorReading
		err := rows.Scan(
			&r.ID, &r.SensorID, &r.SensorType, &r.Value, &r.Unit,
			&r.Latitude, &r.Longitude, &r.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan reading: %w", err)
		}

		r.Location = models.Location{
			Latitude:  r.Latitude,
			Longitude: r.Longitude,
		}

		readings = append(readings, r)
	}

	return readings, rows.Err()
}

// InsertAlert inserts a new alert
func (db *DB) InsertAlert(ctx context.Context, alert *models.Alert) error {
	query := `
		INSERT INTO alerts (sensor_id, alert_type, severity, message, value, threshold, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	err := db.QueryRowContext(
		ctx, query,
		alert.SensorID, alert.AlertType, alert.Severity, alert.Message,
		alert.Value, alert.Threshold, alert.Timestamp,
	).Scan(&alert.ID)

	if err != nil {
		return fmt.Errorf("failed to insert alert: %w", err)
	}

	return nil
}

// GetUnacknowledgedAlerts retrieves all unacknowledged alerts
func (db *DB) GetUnacknowledgedAlerts(ctx context.Context) ([]models.Alert, error) {
	query := `
		SELECT id, sensor_id, alert_type, severity, message, value, threshold, timestamp, acknowledged
		FROM alerts
		WHERE acknowledged = FALSE
		ORDER BY timestamp DESC
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query unacknowledged alerts: %w", err)
	}
	defer rows.Close()

	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		err := rows.Scan(
			&a.ID, &a.SensorID, &a.AlertType, &a.Severity, &a.Message,
			&a.Value, &a.Threshold, &a.Timestamp, &a.Acknowledged,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert: %w", err)
		}
		alerts = append(alerts, a)
	}

	return alerts, rows.Err()
}
