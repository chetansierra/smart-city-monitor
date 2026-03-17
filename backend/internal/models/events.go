package models

import (
	"time"

	"github.com/google/uuid"
)

// AnomalyEvent represents a detected anomaly in sensor readings.
type AnomalyEvent struct {
	ID             int64      `json:"id,omitempty" db:"id"`
	SensorID       uuid.UUID  `json:"sensor_id" db:"sensor_id"`
	SensorType     SensorType `json:"sensor_type" db:"sensor_type"`
	Value          float64    `json:"value" db:"value"`
	ExpectedMean   float64    `json:"expected_mean" db:"expected_mean"`
	ExpectedStdDev float64    `json:"expected_stddev" db:"expected_stddev"`
	ZScore         float64    `json:"z_score" db:"z_score"`
	Timestamp      time.Time  `json:"timestamp" db:"timestamp"`
	CreatedAt      time.Time  `json:"created_at,omitempty" db:"created_at"`
}

// DetectedEvent represents a higher-level pattern detected across multiple sensors.
type DetectedEvent struct {
	ID          int64       `json:"id,omitempty" db:"id"`
	EventType   string      `json:"event_type" db:"event_type"`
	Zone        string      `json:"zone" db:"zone"`
	Description string      `json:"description" db:"description"`
	SensorIDs   []uuid.UUID `json:"sensor_ids" db:"sensor_ids"`
	Timestamp   time.Time   `json:"timestamp" db:"timestamp"`
	ExpiresAt   time.Time   `json:"expires_at" db:"expires_at"`
	CreatedAt   time.Time   `json:"created_at,omitempty" db:"created_at"`
}
