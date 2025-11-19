package websocket

import (
	"encoding/json"
	"time"

	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = 30 * time.Second

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

// Client represents a WebSocket client connection
type Client struct {
	id   string
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// ClientMessage represents a message from the client
type ClientMessage struct {
	Action   string `json:"action"`    // "subscribe" or "unsubscribe"
	SensorID string `json:"sensor_id"` // UUID or "all"
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		id:   uuid.New().String(),
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
	}
}

// ReadPump reads messages from the WebSocket connection
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Str("client_id", c.id).Msg("WebSocket read error")
			}
			break
		}

		// Parse client message
		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			log.Error().
				Err(err).
				Str("client_id", c.id).
				Str("message", string(message)).
				Msg("Failed to parse client message")

			// Send error response
			c.sendError("Invalid message format")
			continue
		}

		// Handle client action
		c.handleAction(&clientMsg)
	}
}

// WritePump writes messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Error().Err(err).Str("client_id", c.id).Msg("Failed to write message")
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Warn().Err(err).Str("client_id", c.id).Msg("Failed to send ping")
				return
			}
		}
	}
}

// handleAction handles client subscription actions
func (c *Client) handleAction(msg *ClientMessage) {
	switch msg.Action {
	case "subscribe":
		if msg.SensorID == "" {
			c.sendError("sensor_id is required")
			return
		}

		if !IsValidSensorID(msg.SensorID) {
			c.sendError("Invalid sensor_id format")
			return
		}

		c.hub.Subscribe(c, msg.SensorID)
		c.sendSuccess("subscribed", msg.SensorID)

	case "unsubscribe":
		if msg.SensorID == "" {
			c.sendError("sensor_id is required")
			return
		}

		c.hub.Unsubscribe(c, msg.SensorID)
		c.sendSuccess("unsubscribed", msg.SensorID)

	case "ping":
		c.sendPong()

	default:
		c.sendError("Unknown action: " + msg.Action)
	}
}

// sendSuccess sends a success response to the client
func (c *Client) sendSuccess(action, sensorID string) {
	response := map[string]interface{}{
		"type":      "response",
		"success":   true,
		"action":    action,
		"sensor_id": sensorID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(response)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal success response")
		return
	}

	select {
	case c.send <- data:
	default:
		log.Warn().Str("client_id", c.id).Msg("Send buffer full, dropping success response")
	}
}

// sendError sends an error response to the client
func (c *Client) sendError(message string) {
	response := map[string]interface{}{
		"type":      "error",
		"success":   false,
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(response)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal error response")
		return
	}

	select {
	case c.send <- data:
	default:
		log.Warn().Str("client_id", c.id).Msg("Send buffer full, dropping error response")
	}
}

// sendPong sends a pong response to the client
func (c *Client) sendPong() {
	response := map[string]interface{}{
		"type":      "pong",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(response)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal pong response")
		return
	}

	select {
	case c.send <- data:
	default:
		log.Warn().Str("client_id", c.id).Msg("Send buffer full, dropping pong response")
	}
}
