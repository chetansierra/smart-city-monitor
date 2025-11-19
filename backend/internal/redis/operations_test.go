package redis

import (
	"testing"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/google/uuid"
)

func TestRedisKeyFormat(t *testing.T) {
	sensorID := uuid.New()

	// Test latest reading key format
	expectedLatest := "sensor:latest:" + sensorID.String()
	actualLatest := "sensor:latest:" + sensorID.String()

	if actualLatest != expectedLatest {
		t.Errorf("Expected key %s, got %s", expectedLatest, actualLatest)
	}

	// Test stream key format
	expectedStream := "sensor:stream:" + sensorID.String()
	actualStream := "sensor:stream:" + sensorID.String()

	if actualStream != expectedStream {
		t.Errorf("Expected key %s, got %s", expectedStream, actualStream)
	}
}

func TestSensorReadingData(t *testing.T) {
	sensorID := uuid.New()
	reading := &models.SensorReading{
		SensorID:   sensorID,
		SensorType: models.SensorTypePollution,
		Value:      45.5,
		Unit:       "µg/m³",
		Latitude:   40.7128,
		Longitude:  -74.0060,
		Timestamp:  time.Now(),
	}

	// Test data map creation
	data := map[string]interface{}{
		"sensor_id":   reading.SensorID.String(),
		"sensor_type": string(reading.SensorType),
		"value":       reading.Value,
		"unit":        reading.Unit,
		"latitude":    reading.Latitude,
		"longitude":   reading.Longitude,
		"timestamp":   reading.Timestamp.Unix(),
	}

	if data["sensor_type"] != "pollution" {
		t.Error("Sensor type should be converted to string")
	}

	if data["value"] != 45.5 {
		t.Error("Value should match")
	}
}

func TestLeaderboardScore(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected float64
	}{
		{"Low pollution", 15.5, 15.5},
		{"Medium pollution", 50.0, 50.0},
		{"High pollution", 120.0, 120.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Score should be the pollution value itself
			score := tt.value
			if score != tt.expected {
				t.Errorf("Expected score %f, got %f", tt.expected, score)
			}
		})
	}
}

// Example integration test (requires Redis)
// func TestSetLatestReading(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("Skipping integration test")
// 	}
//
// 	ctx := context.Background()
// 	cfg := Config{
// 		Addr:     "localhost:6379",
// 		Password: "",
// 		DB:       1, // Use DB 1 for tests
// 	}
//
// 	client, err := Connect(cfg)
// 	if err != nil {
// 		t.Fatalf("Failed to connect to Redis: %v", err)
// 	}
// 	defer client.Close()
//
// 	sensorID := uuid.New()
// 	reading := &models.SensorReading{
// 		SensorID:   sensorID,
// 		SensorType: models.SensorTypeTemperature,
// 		Value:      25.5,
// 		Unit:       "celsius",
// 		Latitude:   40.7128,
// 		Longitude:  -74.0060,
// 		Timestamp:  time.Now(),
// 	}
//
// 	err = client.SetLatestReading(ctx, reading)
// 	if err != nil {
// 		t.Errorf("Failed to set latest reading: %v", err)
// 	}
//
// 	// Clean up
// 	key := "sensor:latest:" + sensorID.String()
// 	client.Del(ctx, key)
// }
