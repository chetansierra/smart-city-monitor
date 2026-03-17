package kafka

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
	"github.com/chetansierra/smart-city-monitor/internal/resilience"
	"github.com/rs/zerolog/log"
)

// Producer wraps a Kafka producer with circuit breaker protection.
type Producer struct {
	producer       sarama.SyncProducer
	circuitBreaker *resilience.CircuitBreaker
	messageCount   atomic.Int64
	lastLogTime    atomic.Int64
}

// ProducerConfig holds Kafka producer configuration
type ProducerConfig struct {
	Brokers        []string
	CircuitBreaker *resilience.CircuitBreaker
}

// NewProducer creates a new Kafka producer with idempotent delivery and LZ4 compression.
func NewProducer(cfg ProducerConfig) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true
	config.Producer.Idempotent = true
	config.Net.MaxOpenRequests = 1
	config.Producer.Compression = sarama.CompressionLZ4

	producer, err := sarama.NewSyncProducer(cfg.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	p := &Producer{
		producer:       producer,
		circuitBreaker: cfg.CircuitBreaker,
	}
	p.lastLogTime.Store(time.Now().Unix())

	log.Info().Strs("brokers", cfg.Brokers).Msg("Kafka producer connected (idempotent, LZ4)")
	return p, nil
}

// SendMessage sends a message to a Kafka topic, wrapped with circuit breaker.
func (p *Producer) SendMessage(topic string, key string, message interface{}) error {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(messageBytes),
	}

	sendFn := func() error {
		_, _, err := p.producer.SendMessage(msg)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
		return nil
	}

	if p.circuitBreaker != nil {
		err = p.circuitBreaker.Execute(sendFn)
	} else {
		err = sendFn()
	}

	if err != nil {
		return err
	}

	// Periodic logging instead of per-message
	count := p.messageCount.Add(1)
	now := time.Now().Unix()
	lastLog := p.lastLogTime.Load()
	if count%1000 == 0 || now-lastLog >= 30 {
		p.lastLogTime.Store(now)
		log.Debug().Int64("total_sent", count).Str("topic", topic).Msg("Kafka messages sent")
	}

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

	err := p.producer.SendMessages(producerMessages)
	if err != nil {
		return fmt.Errorf("failed to send batch messages: %w", err)
	}

	log.Debug().Int("batch_size", len(messages)).Str("topic", topic).Msg("Batch sent")
	return nil
}

// SendRawMessage sends a raw byte message to a topic (used by DLQ and anomaly producers).
func (p *Producer) SendRawMessage(msg *sarama.ProducerMessage) error {
	_, _, err := p.producer.SendMessage(msg)
	return err
}

// Close closes the producer
func (p *Producer) Close() error {
	if err := p.producer.Close(); err != nil {
		return fmt.Errorf("failed to close producer: %w", err)
	}
	log.Info().Msg("Kafka producer closed")
	return nil
}

// CircuitBreakerState returns the circuit breaker state, or "none" if no CB is configured.
func (p *Producer) CircuitBreakerState() string {
	if p.circuitBreaker == nil {
		return "none"
	}
	return p.circuitBreaker.State()
}
