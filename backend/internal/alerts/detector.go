package alerts

import (
	"context"
	"fmt"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/kafka"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Detector monitors sensor readings and generates alerts
type Detector struct {
	db            *postgres.DB
	kafkaProducer *kafka.Producer
}

// NewDetector creates a new alert detector
func NewDetector(db *postgres.DB, kafkaProducer *kafka.Producer) *Detector {
	return &Detector{
		db:            db,
		kafkaProducer: kafkaProducer,
	}
}

// SensorReading represents a sensor reading from Kafka
type SensorReading struct {
	SensorID   string    `json:"sensor_id"`
	SensorType string    `json:"sensor_type"`
	SensorName string    `json:"sensor_name"`
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"`
	Timestamp  time.Time `json:"timestamp"`
	Location   struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"location"`
}

// Alert represents an alert to be created
type Alert struct {
	ID         int64     `json:"id,omitempty"`
	AlertID    string    `json:"alert_id"`
	SensorID   string    `json:"sensor_id"`
	SensorName string    `json:"sensor_name"`
	SensorType string    `json:"sensor_type"`
	AlertType  string    `json:"alert_type"`
	Severity   string    `json:"severity"`
	Value      float64   `json:"value"`
	Threshold  float64   `json:"threshold"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	Location   struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"location"`
}

// ThresholdConfig defines alert thresholds for different sensor types
type ThresholdConfig struct {
	Low      float64
	High     float64
	Critical float64
}

var thresholds = map[string]ThresholdConfig{
	"temperature": {
		Low:      0.0,  // < 0°C
		High:     35.0, // > 35°C
		Critical: 40.0, // > 40°C
	},
	"pollution": {
		Low:      0.0,   // Not used for pollution
		High:     100.0, // > 100 µg/m³
		Critical: 150.0, // > 150 µg/m³
	},
	"humidity": {
		Low:      20.0, // < 20%
		High:     85.0, // > 85%
		Critical: 95.0, // > 95%
	},
	"noise": {
		Low:      0.0,  // Not used for noise
		High:     85.0, // > 85 dB
		Critical: 95.0, // > 95 dB
	},
}

// CheckReading checks if a reading exceeds thresholds and creates alerts
func (d *Detector) CheckReading(ctx context.Context, reading *SensorReading) error {
	config, exists := thresholds[reading.SensorType]
	if !exists {
		return nil // No thresholds defined for this sensor type
	}

	var alertType string
	var severity string
	var threshold float64

	// Determine alert type and severity
	switch reading.SensorType {
	case "temperature":
		if reading.Value > config.Critical {
			alertType = "critical_temperature"
			severity = "critical"
			threshold = config.Critical
		} else if reading.Value > config.High {
			alertType = "high_temperature"
			severity = "high"
			threshold = config.High
		} else if reading.Value < config.Low {
			alertType = "low_temperature"
			severity = "medium"
			threshold = config.Low
		}

	case "pollution":
		if reading.Value > config.Critical {
			alertType = "critical_pollution"
			severity = "critical"
			threshold = config.Critical
		} else if reading.Value > config.High {
			alertType = "high_pollution"
			severity = "high"
			threshold = config.High
		}

	case "humidity":
		if reading.Value > config.Critical {
			alertType = "critical_humidity"
			severity = "critical"
			threshold = config.Critical
		} else if reading.Value > config.High {
			alertType = "high_humidity"
			severity = "high"
			threshold = config.High
		} else if reading.Value < config.Low {
			alertType = "low_humidity"
			severity = "medium"
			threshold = config.Low
		}

	case "noise":
		if reading.Value > config.Critical {
			alertType = "critical_noise"
			severity = "critical"
			threshold = config.Critical
		} else if reading.Value > config.High {
			alertType = "high_noise"
			severity = "high"
			threshold = config.High
		}
	}

	// No alert condition met
	if alertType == "" {
		return nil
	}

	// Check if we already have a recent alert for this sensor/type (within last hour)
	isDuplicate, err := d.checkDuplicateAlert(ctx, reading.SensorID, alertType)
	if err != nil {
		log.Error().Err(err).Str("sensor_id", reading.SensorID).Msg("Failed to check duplicate alert")
		return err
	}

	if isDuplicate {
		log.Debug().
			Str("sensor_id", reading.SensorID).
			Str("alert_type", alertType).
			Msg("Skipping duplicate alert")
		return nil
	}

	// Create alert
	alert := &Alert{
		AlertID:    uuid.New().String(),
		SensorID:   reading.SensorID,
		SensorName: reading.SensorName,
		SensorType: reading.SensorType,
		AlertType:  alertType,
		Severity:   severity,
		Value:      reading.Value,
		Threshold:  threshold,
		Message:    d.formatMessage(alertType, reading),
		Timestamp:  reading.Timestamp,
		Location:   reading.Location,
	}

	// Save alert to database
	err = d.saveAlert(ctx, alert)
	if err != nil {
		log.Error().Err(err).Msg("Failed to save alert")
		return err
	}

	// Publish alert to Kafka
	err = d.publishAlert(alert)
	if err != nil {
		log.Error().Err(err).Msg("Failed to publish alert to Kafka")
		// Don't return error - alert is already saved
	}

	log.Info().
		Str("alert_id", alert.AlertID).
		Str("sensor_id", reading.SensorID).
		Str("alert_type", alertType).
		Str("severity", severity).
		Float64("value", reading.Value).
		Float64("threshold", threshold).
		Msg("Alert created")

	return nil
}

// checkDuplicateAlert checks if a similar alert exists within the last hour
func (d *Detector) checkDuplicateAlert(ctx context.Context, sensorID, alertType string) (bool, error) {
	query := `
		SELECT COUNT(*) FROM alerts
		WHERE sensor_id = $1
		  AND alert_type = $2
		  AND timestamp > NOW() - INTERVAL '1 hour'
		  AND acknowledged = false
	`

	var count int
	err := d.db.QueryRowContext(ctx, query, sensorID, alertType).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// saveAlert saves an alert to the database
func (d *Detector) saveAlert(ctx context.Context, alert *Alert) error {
	query := `
		INSERT INTO alerts (sensor_id, alert_type, severity, message, value, threshold, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	sensorUUID, err := uuid.Parse(alert.SensorID)
	if err != nil {
		return fmt.Errorf("invalid sensor UUID: %w", err)
	}

	err = d.db.QueryRowContext(
		ctx,
		query,
		sensorUUID,
		alert.AlertType,
		alert.Severity,
		alert.Message,
		alert.Value,
		alert.Threshold,
		alert.Timestamp,
	).Scan(&alert.ID)

	return err
}

// publishAlert publishes an alert to Kafka
func (d *Detector) publishAlert(alert *Alert) error {
	return d.kafkaProducer.SendMessage("alerts", alert.AlertID, alert)
}

// formatMessage creates a human-readable message for an alert
func (d *Detector) formatMessage(alertType string, reading *SensorReading) string {
	messages := map[string]string{
		"critical_temperature": fmt.Sprintf("Critical temperature detected at %s: %.2f%s", reading.SensorName, reading.Value, reading.Unit),
		"high_temperature":     fmt.Sprintf("High temperature detected at %s: %.2f%s", reading.SensorName, reading.Value, reading.Unit),
		"low_temperature":      fmt.Sprintf("Low temperature detected at %s: %.2f%s", reading.SensorName, reading.Value, reading.Unit),
		"critical_pollution":   fmt.Sprintf("Critical air pollution at %s: %.2f %s", reading.SensorName, reading.Value, reading.Unit),
		"high_pollution":       fmt.Sprintf("High air pollution at %s: %.2f %s", reading.SensorName, reading.Value, reading.Unit),
		"critical_humidity":    fmt.Sprintf("Critical humidity level at %s: %.2f%s", reading.SensorName, reading.Value, reading.Unit),
		"high_humidity":        fmt.Sprintf("High humidity level at %s: %.2f%s", reading.SensorName, reading.Value, reading.Unit),
		"low_humidity":         fmt.Sprintf("Low humidity level at %s: %.2f%s", reading.SensorName, reading.Value, reading.Unit),
		"critical_noise":       fmt.Sprintf("Critical noise level at %s: %.2f %s", reading.SensorName, reading.Value, reading.Unit),
		"high_noise":           fmt.Sprintf("High noise level at %s: %.2f %s", reading.SensorName, reading.Value, reading.Unit),
	}

	msg, exists := messages[alertType]
	if !exists {
		return fmt.Sprintf("Alert at %s: %.2f %s", reading.SensorName, reading.Value, reading.Unit)
	}

	return msg
}
