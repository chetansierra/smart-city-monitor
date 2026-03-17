package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SensorType represents the type of sensor
type SensorType string

const (
	SensorTypeTemperature SensorType = "temperature"
	SensorTypePollution   SensorType = "pollution"
	SensorTypeHumidity    SensorType = "humidity"
	SensorTypeNoise       SensorType = "noise"
)

// SensorStatus represents the operational status of a sensor.
// Migration 009 enforces that only "active" sensors exist in the database;
// deactivation is done by deleting the row, not changing status.
type SensorStatus string

const (
	SensorStatusActive SensorStatus = "active"
)

// Location represents geographic coordinates
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Sensor represents a physical sensor device
type Sensor struct {
	ID        uuid.UUID    `json:"id" db:"id"`
	SessionID *uuid.UUID   `json:"session_id,omitempty" db:"session_id"`
	Name      string       `json:"name" db:"name"`
	Type      SensorType   `json:"type" db:"type"`
	Location  Location     `json:"location"`
	Latitude  float64      `json:"-" db:"latitude"`  // For DB storage
	Longitude float64      `json:"-" db:"longitude"` // For DB storage
	Status    SensorStatus `json:"status" db:"status"`
	Config    string       `json:"config,omitempty" db:"config"` // JSONB field
	CreatedAt time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt time.Time    `json:"updated_at" db:"updated_at"`
}

// SensorReading represents a single sensor measurement
type SensorReading struct {
	ID         int64      `json:"id,omitempty" db:"id"`
	SensorID   uuid.UUID  `json:"sensor_id" db:"sensor_id"`
	SessionID  *uuid.UUID `json:"session_id,omitempty" db:"session_id"`
	SensorType SensorType `json:"sensor_type" db:"sensor_type"`
	Value      float64    `json:"value" db:"value"`
	Unit       string     `json:"unit" db:"unit"`
	Location   Location   `json:"location"`
	Latitude   float64    `json:"-" db:"latitude"`  // For DB storage
	Longitude  float64    `json:"-" db:"longitude"` // For DB storage
	Timestamp  time.Time  `json:"timestamp" db:"timestamp"`
}

// MarshalJSON customizes JSON serialization to ensure consistent timestamp format
func (s *SensorReading) MarshalJSON() ([]byte, error) {
	type Alias SensorReading
	return json.Marshal(&struct {
		*Alias
		Timestamp string `json:"timestamp"`
	}{
		Alias:     (*Alias)(s),
		Timestamp: s.Timestamp.Format(time.RFC3339Nano),
	})
}

// KafkaMessage represents the message format for Kafka
type KafkaMessage struct {
	SensorID   string   `json:"sensor_id"`
	SessionID  string   `json:"session_id,omitempty"`
	SensorType string   `json:"sensor_type"`
	Value      float64  `json:"value"`
	Unit       string   `json:"unit"`
	Location   Location `json:"location"`
	Timestamp  string   `json:"timestamp"`
}

// SensorAggregate represents aggregated sensor data
type SensorAggregate struct {
	ID              int64      `json:"id" db:"id"`
	SensorID        uuid.UUID  `json:"sensor_id" db:"sensor_id"`
	SensorType      SensorType `json:"sensor_type" db:"sensor_type"`
	AggregationType string     `json:"aggregation_type" db:"aggregation_type"` // hourly, daily
	AvgValue        float64    `json:"avg_value" db:"avg_value"`
	MinValue        float64    `json:"min_value" db:"min_value"`
	MaxValue        float64    `json:"max_value" db:"max_value"`
	PeriodStart     time.Time  `json:"period_start" db:"period_start"`
	PeriodEnd       time.Time  `json:"period_end" db:"period_end"`
}
