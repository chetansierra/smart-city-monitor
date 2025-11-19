package kafka

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/IBM/sarama"
)

// Producer wraps a Kafka producer
type Producer struct {
	producer sarama.SyncProducer
}

// ProducerConfig holds Kafka producer configuration
type ProducerConfig struct {
	Brokers []string
}

// NewProducer creates a new Kafka producer
func NewProducer(cfg ProducerConfig) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(cfg.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	log.Printf("Kafka producer connected to: %v", cfg.Brokers)
	return &Producer{producer: producer}, nil
}

// SendMessage sends a message to a Kafka topic
func (p *Producer) SendMessage(topic string, key string, message interface{}) error {
	// Marshal message to JSON
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create Kafka message
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(messageBytes),
	}

	// Send message
	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	log.Printf("Message sent to topic=%s partition=%d offset=%d", topic, partition, offset)
	return nil
}

// SendMessageBatch sends multiple messages to a Kafka topic
func (p *Producer) SendMessageBatch(topic string, messages []interface{}) error {
	var producerMessages []*sarama.ProducerMessage

	for i, msg := range messages {
		messageBytes, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message %d: %w", i, err)
		}

		producerMessages = append(producerMessages, &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(messageBytes),
		})
	}

	// Send all messages
	err := p.producer.SendMessages(producerMessages)
	if err != nil {
		return fmt.Errorf("failed to send batch messages: %w", err)
	}

	log.Printf("Batch of %d messages sent to topic=%s", len(messages), topic)
	return nil
}

// Close closes the producer
func (p *Producer) Close() error {
	if err := p.producer.Close(); err != nil {
		return fmt.Errorf("failed to close producer: %w", err)
	}
	log.Println("Kafka producer closed")
	return nil
}
