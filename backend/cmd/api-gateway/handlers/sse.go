package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/IBM/sarama"
	"github.com/chetansierra/smart-city-monitor/internal/kafka"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/chetansierra/smart-city-monitor/internal/sse"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"
)

// SSEHandler handles Server-Sent Events connections
type SSEHandler struct {
	broadcaster   *sse.Broadcaster
	redisClient   *redis.Client
	kafkaBrokers  []string
	kafkaTopics   []string
	consumerGroup string
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(broadcaster *sse.Broadcaster, redisClient *redis.Client, kafkaBrokers []string, kafkaTopics []string, consumerGroup string) *SSEHandler {
	return &SSEHandler{
		broadcaster:   broadcaster,
		redisClient:   redisClient,
		kafkaBrokers:  kafkaBrokers,
		kafkaTopics:   kafkaTopics,
		consumerGroup: consumerGroup,
	}
}

// HandleStream handles the SSE stream connection
func (h *SSEHandler) HandleStream(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	sessionID := c.Query("session_id")

	done := c.Context().Done()
	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		log.Info().Str("session_id", sessionID).Msg("New SSE stream client connected")
		client := make(sse.Client, 64)
		if sessionID != "" {
			if _, err := h.redisClient.IncrSessionWorkerGoroutines(context.Background(), sessionID); err != nil {
				log.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to increment session worker goroutines")
			}
		}
		h.broadcaster.NewClients <- sse.NewClientRegistration(client, sessionID)

		defer func() {
			if sessionID != "" {
				if _, err := h.redisClient.DecrSessionWorkerGoroutines(context.Background(), sessionID); err != nil {
					log.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to decrement session worker goroutines")
				}
			}
			h.broadcaster.ClosingClients <- client
		}()

		// Send initial keep-alive or sync message
		fmt.Fprintf(w, "event: connected\ndata: {\"message\": \"SSE connection established\"}\n\n")
		w.Flush()

		for {
			select {
			case <-done:
				log.Info().Msg("SSE stream client disconnected (context done)")
				return
			case msg := <-client:
				fmt.Fprintf(w, "data: %s\n\n", msg)
				if err := w.Flush(); err != nil {
					log.Error().Err(err).Msg("Failed to flush SSE data")
					return
				}
			}
		}
	}))

	return nil
}

// StartKafkaConsumer starts consuming from Kafka topics for real-time SSE updates.
// Replaces the previous Redis Pub/Sub listener — Kafka is the single source of truth.
func (h *SSEHandler) StartKafkaConsumer(ctx context.Context) {
	consumer, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers:       h.kafkaBrokers,
		GroupID:       h.consumerGroup,
		Topics:        h.kafkaTopics,
		InitialOffset: sarama.OffsetNewest,
		// No DLQ/retry — dropped real-time messages are acceptable
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Kafka consumer for SSE")
	}
	defer consumer.Close()

	log.Info().Strs("topics", h.kafkaTopics).Str("group", h.consumerGroup).Msg("SSE handler consuming from Kafka topics")

	if err := consumer.ConsumeWithTopics(ctx, h.kafkaTopics, h.handleKafkaMessage); err != nil {
		log.Error().Err(err).Msg("Kafka consumer for SSE stopped with error")
	}
}

// handleKafkaMessage routes messages from different Kafka topics to the appropriate SSE broadcast.
func (h *SSEHandler) handleKafkaMessage(topic string, message []byte) error {
	switch {
	case isSensorReadingsTopic(topic):
		var update map[string]interface{}
		if err := json.Unmarshal(message, &update); err != nil {
			log.Error().Err(err).Msg("Failed to parse sensor reading from Kafka")
			return nil // don't retry bad messages
		}

		var sessionID string
		if sid, ok := update["session_id"].(string); ok {
			sessionID = sid
		}

		if sessionID != "" {
			if err := h.redisClient.AppendSessionEvent(context.Background(), sessionID, string(message)); err != nil {
				log.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to cache session live event")
			}
		}

		h.broadcaster.BroadcastSensorUpdate(sessionID, update)

	case isAnomaliesTopic(topic):
		var anomalyData map[string]interface{}
		if err := json.Unmarshal(message, &anomalyData); err != nil {
			log.Error().Err(err).Msg("Failed to parse anomaly event from Kafka")
			return nil
		}
		h.broadcaster.BroadcastAnomaly(anomalyData)

	case isEventsTopic(topic):
		var eventData map[string]interface{}
		if err := json.Unmarshal(message, &eventData); err != nil {
			log.Error().Err(err).Msg("Failed to parse detected event from Kafka")
			return nil
		}
		h.broadcaster.BroadcastEvent(eventData)

	default:
		log.Warn().Str("topic", topic).Msg("Received message from unknown topic")
	}

	return nil
}

// Topic matching helpers — use suffix matching so topic names are configurable.
func isSensorReadingsTopic(topic string) bool {
	return topic == "sensor-readings"
}

func isAnomaliesTopic(topic string) bool {
	return topic == "sensor-anomalies"
}

func isEventsTopic(topic string) bool {
	return topic == "sensor-events"
}

// GetStats returns SSE broadcaster statistics
func (h *SSEHandler) GetStats(c *fiber.Ctx) error {
	stats := h.broadcaster.GetStats()
	return c.JSON(APIResponse{
		Success: true,
		Data:    stats,
	})
}

// GetSessionHistory returns recent cached live events for a session.
func (h *SSEHandler) GetSessionHistory(c *fiber.Ctx) error {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		sessionID = c.Get("X-Session-ID")
	}
	if sessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "MISSING_SESSION_ID",
				Message: "session_id query param or X-Session-ID header is required",
			},
		})
	}

	limit := int64(c.QueryInt("limit", 300))
	if limit <= 0 || limit > 1000 {
		limit = 300
	}

	ctx := context.Background()
	events, err := h.redisClient.GetSessionEvents(ctx, sessionID, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "REDIS_ERROR",
				Message: "Failed to fetch session history",
			},
		})
	}

	// Stored newest-first via LPUSH; return oldest-first for client replay.
	history := make([]map[string]interface{}, 0, len(events))
	for i := len(events) - 1; i >= 0; i-- {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(events[i]), &parsed); err != nil {
			continue
		}
		if ts, ok := parsed["timestamp"].(float64); ok {
			parsed["timestamp"] = strconv.FormatInt(int64(ts), 10)
		}
		history = append(history, parsed)
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    history,
		Meta: &Meta{
			Total: len(history),
		},
	})
}
