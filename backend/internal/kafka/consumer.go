package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/IBM/sarama"
)

// Consumer wraps a Kafka consumer
type Consumer struct {
	consumer sarama.ConsumerGroup
	ready    chan bool
}

// ConsumerConfig holds Kafka consumer configuration
type ConsumerConfig struct {
	Brokers       []string
	GroupID       string
	Topics        []string
	InitialOffset int64 // sarama.OffsetNewest or sarama.OffsetOldest
}

// MessageHandler is a function that processes consumed messages
type MessageHandler func(message []byte) error

// NewConsumer creates a new Kafka consumer
func NewConsumer(cfg ConsumerConfig) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = cfg.InitialOffset
	config.Version = sarama.V2_6_0_0

	consumer, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	log.Printf("Kafka consumer created with group=%s topics=%v", cfg.GroupID, cfg.Topics)
	return &Consumer{
		consumer: consumer,
		ready:    make(chan bool),
	}, nil
}

// Consume starts consuming messages from Kafka
func (c *Consumer) Consume(ctx context.Context, topics []string, handler MessageHandler) error {
	consumerHandler := &consumerGroupHandler{
		ready:   c.ready,
		handler: handler,
	}

	go func() {
		for {
			if err := c.consumer.Consume(ctx, topics, consumerHandler); err != nil {
				log.Printf("Error from consumer: %v", err)
			}

			// Check if context was cancelled
			if ctx.Err() != nil {
				return
			}

			c.ready = make(chan bool)
		}
	}()

	<-c.ready
	log.Println("Kafka consumer is ready")
	return nil
}

// Close closes the consumer
func (c *Consumer) Close() error {
	if err := c.consumer.Close(); err != nil {
		return fmt.Errorf("failed to close consumer: %w", err)
	}
	log.Println("Kafka consumer closed")
	return nil
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	ready   chan bool
	handler MessageHandler
}

// Setup is run at the beginning of a new session
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

// Cleanup is run at the end of a session
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim processes messages from a partition
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			// Process message
			if err := h.handler(message.Value); err != nil {
				log.Printf("Error processing message: %v", err)
				// Continue processing even if handler fails
			}

			// Mark message as processed
			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
		}
	}
}

// UnmarshalMessage is a helper to unmarshal JSON messages
func UnmarshalMessage(data []byte, v interface{}) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}
	return nil
}
