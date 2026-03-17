package kafka

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog/log"
)

// DLQProducer sends failed messages to a dead-letter queue topic.
type DLQProducer struct {
	producer *Producer
	topic    string
}

// NewDLQProducer wraps an existing producer for DLQ use.
func NewDLQProducer(producer *Producer, topic string) *DLQProducer {
	return &DLQProducer{
		producer: producer,
		topic:    topic,
	}
}

// SendToDLQ sends a failed message to the DLQ topic with error metadata in headers.
func (d *DLQProducer) SendToDLQ(originalMsg []byte, originalTopic string, err error, retryCount int) error {
	headers := []sarama.RecordHeader{
		{Key: []byte("error"), Value: []byte(err.Error())},
		{Key: []byte("original-topic"), Value: []byte(originalTopic)},
		{Key: []byte("retry-count"), Value: []byte(strconv.Itoa(retryCount))},
		{Key: []byte("failed-at"), Value: []byte(time.Now().UTC().Format(time.RFC3339Nano))},
	}

	msg := &sarama.ProducerMessage{
		Topic:   d.topic,
		Value:   sarama.ByteEncoder(originalMsg),
		Headers: headers,
	}

	_, _, sendErr := d.producer.producer.SendMessage(msg)
	if sendErr != nil {
		log.Error().Err(sendErr).Str("topic", d.topic).Msg("Failed to send message to DLQ")
		return fmt.Errorf("failed to send to DLQ: %w", sendErr)
	}

	log.Warn().
		Str("dlq_topic", d.topic).
		Str("original_topic", originalTopic).
		Int("retry_count", retryCount).
		Str("error", err.Error()).
		Msg("Message sent to DLQ")

	return nil
}

// DLQMessage represents a message retrieved from the DLQ for inspection.
type DLQMessage struct {
	OriginalTopic string          `json:"original_topic"`
	Error         string          `json:"error"`
	RetryCount    int             `json:"retry_count"`
	FailedAt      string          `json:"failed_at"`
	Payload       json.RawMessage `json:"payload"`
	Offset        int64           `json:"offset"`
	Partition     int32           `json:"partition"`
}
