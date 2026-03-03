package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/chetansierra/smart-city-monitor/internal/sse"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"
)

// SSEHandler handles Server-Sent Events connections
type SSEHandler struct {
	broadcaster *sse.Broadcaster
	redisClient *redis.Client
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(broadcaster *sse.Broadcaster, redisClient *redis.Client) *SSEHandler {
	return &SSEHandler{
		broadcaster: broadcaster,
		redisClient: redisClient,
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

// StartRedisPubSubListener starts listening to Redis pub/sub for sensor updates
func (h *SSEHandler) StartRedisPubSubListener(ctx context.Context) {
	pubsub := h.redisClient.Subscribe(ctx, "sensor:updates")
	defer pubsub.Close()

	log.Info().Msg("SSE handler listening to Redis sensor:updates channel")

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

			// Extract session ID if available
			var sessionID string
			if sid, ok := update["session_id"].(string); ok {
				sessionID = sid
			}

			// Cache live event per-session for short-lived page reload recovery.
			if sessionID != "" {
				if err := h.redisClient.AppendSessionEvent(ctx, sessionID, msg.Payload); err != nil {
					log.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to cache session live event")
				}
			}

			// Broadcast to SSE clients
			h.broadcaster.BroadcastSensorUpdate(sessionID, update)
		}
	}
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
