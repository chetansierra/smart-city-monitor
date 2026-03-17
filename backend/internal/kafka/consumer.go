package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/chetansierra/smart-city-monitor/internal/resilience"
	"github.com/rs/zerolog/log"
)

// Consumer wraps a Kafka consumer
type Consumer struct {
	consumer    sarama.ConsumerGroup
	ready       chan bool
	dlqProducer *DLQProducer
	retryConfig *resilience.RetryConfig
	sourceTopic string
}

// ConsumerConfig holds Kafka consumer configuration
type ConsumerConfig struct {
	Brokers       []string
	GroupID       string
	Topics        []string
	InitialOffset int64 // sarama.OffsetNewest or sarama.OffsetOldest
	DLQProducer   *DLQProducer
	RetryConfig   *resilience.RetryConfig
}

// MessageHandler is a function that processes consumed messages
type MessageHandler func(message []byte) error

// TopicMessageHandler is a function that processes consumed messages with topic context
type TopicMessageHandler func(topic string, message []byte) error

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

	sourceTopic := ""
	if len(cfg.Topics) > 0 {
		sourceTopic = cfg.Topics[0]
	}

	log.Info().Str("group", cfg.GroupID).Strs("topics", cfg.Topics).Msg("Kafka consumer created")
	return &Consumer{
		consumer:    consumer,
		ready:       make(chan bool),
		dlqProducer: cfg.DLQProducer,
		retryConfig: cfg.RetryConfig,
		sourceTopic: sourceTopic,
	}, nil
}

// Consume starts consuming messages from Kafka and blocks until the context is cancelled
func (c *Consumer) Consume(ctx context.Context, topics []string, handler MessageHandler) error {
	for {
		consumerHandler := &consumerGroupHandler{
			ready:       c.ready,
			handler:     handler,
			dlqProducer: c.dlqProducer,
			retryConfig: c.retryConfig,
			sourceTopic: c.sourceTopic,
		}

		if err := c.consumer.Consume(ctx, topics, consumerHandler); err != nil {
			return fmt.Errorf("error from consumer: %w", err)
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil
		}

		c.ready = make(chan bool)
	}
}

// Close closes the consumer
func (c *Consumer) Close() error {
	if err := c.consumer.Close(); err != nil {
		return fmt.Errorf("failed to close consumer: %w", err)
	}
	log.Info().Msg("Kafka consumer closed")
	return nil
}

// ConsumeWithTopics starts consuming messages from multiple Kafka topics, passing the topic name to the handler.
func (c *Consumer) ConsumeWithTopics(ctx context.Context, topics []string, handler TopicMessageHandler) error {
	for {
		consumerHandler := &topicConsumerGroupHandler{
			ready:   make(chan bool),
			handler: handler,
		}

		if err := c.consumer.Consume(ctx, topics, consumerHandler); err != nil {
			return fmt.Errorf("error from consumer: %w", err)
		}

		if ctx.Err() != nil {
			return nil
		}
	}
}

// topicConsumerGroupHandler implements sarama.ConsumerGroupHandler with topic-aware routing.
// No retry/DLQ — suitable for real-time consumers where dropped messages are acceptable.
type topicConsumerGroupHandler struct {
	ready   chan bool
	handler TopicMessageHandler
}

func (h *topicConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

func (h *topicConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *topicConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}
			if err := h.handler(message.Topic, message.Value); err != nil {
				log.Warn().Err(err).Str("topic", message.Topic).Int64("offset", message.Offset).Msg("Error processing message (skipped)")
			}
			session.MarkMessage(message, "")
		case <-session.Context().Done():
			return nil
		}
	}
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	ready       chan bool
	handler     MessageHandler
	dlqProducer *DLQProducer
	retryConfig *resilience.RetryConfig
	sourceTopic string
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

// ConsumeClaim processes messages from a partition with retry and DLQ support.
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			var handlerErr error

			if h.retryConfig != nil {
				retryCount := 0
				handlerErr = resilience.WithRetry(
					session.Context(),
					*h.retryConfig,
					func() error {
						retryCount++
						err := h.handler(message.Value)
						if err != nil && retryCount > 1 {
							log.Warn().
								Err(err).
								Int("attempt", retryCount).
								Int64("offset", message.Offset).
								Msg("Retrying message processing")
						}
						return err
					},
				)

				if handlerErr != nil && h.dlqProducer != nil {
					// All retries exhausted — send to DLQ
					dlqErr := h.dlqProducer.SendToDLQ(message.Value, h.sourceTopic, handlerErr, retryCount)
					if dlqErr != nil {
						log.Error().Err(dlqErr).Msg("Failed to send message to DLQ after retries exhausted")
						// Don't mark as processed if both handler and DLQ fail
						continue
					}
					// DLQ succeeded — mark as processed so we don't re-process
					log.Warn().
						Int64("offset", message.Offset).
						Str("topic", h.sourceTopic).
						Msg("Message sent to DLQ after all retries exhausted")
				}
			} else {
				// No retry config — original behavior
				handlerErr = h.handler(message.Value)
				if handlerErr != nil {
					log.Error().Err(handlerErr).Msg("Error processing message")
				}
			}

			// Mark message as processed only after successful handling or DLQ
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

// ReadDLQMessages reads recent messages from the DLQ topic for inspection.
func ReadDLQMessages(brokers []string, dlqTopic string, limit int) ([]DLQMessage, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0
	config.Consumer.Return.Errors = true

	consumer, err := sarama.NewConsumer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}
	defer consumer.Close()

	partitions, err := consumer.Partitions(dlqTopic)
	if err != nil {
		return nil, fmt.Errorf("failed to get partitions: %w", err)
	}

	var messages []DLQMessage

	for _, partition := range partitions {
		client, err := sarama.NewClient(brokers, config)
		if err != nil {
			continue
		}

		newestOffset, err := client.GetOffset(dlqTopic, partition, sarama.OffsetNewest)
		client.Close()
		if err != nil || newestOffset <= 0 {
			continue
		}

		startOffset := newestOffset - int64(limit)
		if startOffset < 0 {
			startOffset = 0
		}

		pc, err := consumer.ConsumePartition(dlqTopic, partition, startOffset)
		if err != nil {
			continue
		}

		timeout := time.After(3 * time.Second)
		for len(messages) < limit {
			select {
			case msg := <-pc.Messages():
				dlqMsg := DLQMessage{
					Payload:   json.RawMessage(msg.Value),
					Offset:    msg.Offset,
					Partition: msg.Partition,
				}
				for _, h := range msg.Headers {
					switch string(h.Key) {
					case "error":
						dlqMsg.Error = string(h.Value)
					case "original-topic":
						dlqMsg.OriginalTopic = string(h.Value)
					case "retry-count":
						fmt.Sscanf(string(h.Value), "%d", &dlqMsg.RetryCount)
					case "failed-at":
						dlqMsg.FailedAt = string(h.Value)
					}
				}
				messages = append(messages, dlqMsg)
			case <-timeout:
				pc.Close()
				return messages, nil
			}
		}
		pc.Close()
	}

	return messages, nil
}
