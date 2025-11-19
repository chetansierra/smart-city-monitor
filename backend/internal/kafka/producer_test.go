package kafka

import (
	"encoding/json"
	"testing"

	"github.com/chetansierra/smart-city-monitor/internal/models"
)

func TestKafkaMessageSerialization(t *testing.T) {
	msg := models.KafkaMessage{
		SensorID:   "123e4567-e89b-12d3-a456-426614174000",
		SensorType: "temperature",
		Value:      25.5,
		Unit:       "celsius",
		Location: models.Location{
			Latitude:  40.7128,
			Longitude: -74.0060,
		},
		Timestamp: "2025-01-01T12:00:00Z",
	}

	// Test JSON serialization
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Test JSON deserialization
	var decoded models.KafkaMessage
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	// Verify fields
	if decoded.SensorID != msg.SensorID {
		t.Errorf("Expected SensorID %s, got %s", msg.SensorID, decoded.SensorID)
	}

	if decoded.Value != msg.Value {
		t.Errorf("Expected Value %f, got %f", msg.Value, decoded.Value)
	}

	if decoded.Location.Latitude != msg.Location.Latitude {
		t.Errorf("Expected Latitude %f, got %f", msg.Location.Latitude, decoded.Location.Latitude)
	}
}

func TestProducerConfig(t *testing.T) {
	cfg := ProducerConfig{
		Brokers: []string{"localhost:9092"},
	}

	if len(cfg.Brokers) == 0 {
		t.Error("Brokers should not be empty")
	}

	if cfg.Brokers[0] != "localhost:9092" {
		t.Errorf("Expected broker localhost:9092, got %s", cfg.Brokers[0])
	}
}

func TestConsumerConfig(t *testing.T) {
	cfg := ConsumerConfig{
		Brokers: []string{"localhost:9092"},
		GroupID: "test-group",
		Topics:  []string{"sensor-readings"},
	}

	if cfg.GroupID == "" {
		t.Error("GroupID should not be empty")
	}

	if len(cfg.Topics) == 0 {
		t.Error("Topics should not be empty")
	}

	if cfg.Topics[0] != "sensor-readings" {
		t.Errorf("Expected topic sensor-readings, got %s", cfg.Topics[0])
	}
}

// Example integration test (requires Kafka)
// func TestSendMessage(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("Skipping integration test")
// 	}
//
// 	cfg := ProducerConfig{
// 		Brokers: []string{"localhost:9092"},
// 	}
//
// 	producer, err := NewProducer(cfg)
// 	if err != nil {
// 		t.Fatalf("Failed to create producer: %v", err)
// 	}
// 	defer producer.Close()
//
// 	msg := models.KafkaMessage{
// 		SensorID:   "test-sensor",
// 		SensorType: "temperature",
// 		Value:      25.5,
// 		Unit:       "celsius",
// 		Location: models.Location{
// 			Latitude:  40.7128,
// 			Longitude: -74.0060,
// 		},
// 		Timestamp: time.Now().Format(time.RFC3339),
// 	}
//
// 	err = producer.SendMessage("test-topic", "test-key", msg)
// 	if err != nil {
// 		t.Errorf("Failed to send message: %v", err)
// 	}
// }
