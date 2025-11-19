package handlers

import (
	"context"
	"encoding/json"

	"github.com/chetansierra/smart-city-monitor/internal/kafka"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	ws "github.com/chetansierra/smart-city-monitor/internal/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/rs/zerolog/log"
)

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub         *ws.Hub
	redisClient *redis.Client
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *ws.Hub, redisClient *redis.Client) *WebSocketHandler {
	return &WebSocketHandler{
		hub:         hub,
		redisClient: redisClient,
	}
}

// HandleConnection handles WebSocket upgrade and client management
func (h *WebSocketHandler) HandleConnection(c *websocket.Conn) {
	client := ws.NewClient(h.hub, c)

	// Register client with hub
	h.hub.Register <- client

	// Start goroutines for reading and writing
	go client.WritePump()
	client.ReadPump() // This blocks until connection closes
}

// StartRedisPubSubListener starts listening to Redis pub/sub for sensor updates
func (h *WebSocketHandler) StartRedisPubSubListener(ctx context.Context) {
	pubsub := h.redisClient.Subscribe(ctx, "sensor:updates")
	defer pubsub.Close()

	log.Info().Msg("WebSocket handler listening to Redis sensor:updates channel")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Stopping Redis pub/sub listener")
			return

		case msg := <-pubsub.Channel():
			if msg == nil {
				continue
			}

			// Parse the sensor update message
			var update map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &update); err != nil {
				log.Error().
					Err(err).
					Str("payload", msg.Payload).
					Msg("Failed to parse sensor update from Redis")
				continue
			}

			// Extract sensor ID
			sensorID, ok := update["sensor_id"].(string)
			if !ok {
				log.Warn().
					Interface("update", update).
					Msg("Sensor update missing sensor_id field")
				continue
			}

			// Broadcast to WebSocket clients
			h.hub.BroadcastSensorUpdate(sensorID, update)

			log.Debug().
				Str("sensor_id", sensorID).
				Msg("Broadcasted sensor update to WebSocket clients")
		}
	}
}

// StartAlertListener starts listening to Kafka alerts topic and broadcasts to WebSocket clients
func (h *WebSocketHandler) StartAlertListener(ctx context.Context, brokers []string, topic string) {
	consumerCfg := kafka.ConsumerConfig{
		Brokers: brokers,
		GroupID: "api-gateway-alerts-group",
		Topics:  []string{topic},
	}

	consumer, err := kafka.NewConsumer(consumerCfg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Kafka alert consumer")
		return
	}
	defer consumer.Close()

	log.Info().Str("topic", topic).Msg("WebSocket handler listening to Kafka alerts")

	err = consumer.Consume(ctx, []string{topic}, func(value []byte) error {
		// Parse alert message
		var alert map[string]interface{}
		if err := json.Unmarshal(value, &alert); err != nil {
			log.Error().
				Err(err).
				Str("value", string(value)).
				Msg("Failed to parse alert from Kafka")
			return nil // Skip invalid messages
		}

		// Broadcast alert to all WebSocket clients
		h.hub.BroadcastAlert(alert)

		log.Info().
			Interface("alert", alert).
			Msg("Broadcasted alert to WebSocket clients")

		return nil
	})

	if err != nil && err != context.Canceled {
		log.Error().Err(err).Msg("Kafka alert consumer error")
	}
}

// GetStats returns WebSocket hub statistics
func (h *WebSocketHandler) GetStats(c *fiber.Ctx) error {
	stats := h.hub.GetStats()
	return c.JSON(APIResponse{
		Success: true,
		Data:    stats,
	})
}
