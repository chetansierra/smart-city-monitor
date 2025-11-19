package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Hub manages WebSocket client connections and message broadcasting
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Client subscriptions: sensor_id -> set of clients
	subscriptions map[string]map[*Client]bool

	// Register requests from clients
	Register chan *Client

	// Unregister requests from clients
	Unregister chan *Client

	// Broadcast messages to clients
	broadcast chan *BroadcastMessage

	// Mutex for thread-safe operations
	mu sync.RWMutex
}

// BroadcastMessage represents a message to be sent to clients
type BroadcastMessage struct {
	SensorID string
	Data     []byte
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:       make(map[*Client]bool),
		subscriptions: make(map[string]map[*Client]bool),
		Register:      make(chan *Client),
		Unregister:    make(chan *Client),
		broadcast:     make(chan *BroadcastMessage, 256),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Shutting down WebSocket hub")
			h.closeAllConnections()
			return

		case client := <-h.Register:
			h.registerClient(client)

		case client := <-h.Unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)

		case <-ticker.C:
			// Periodic health check
			h.mu.RLock()
			clientCount := len(h.clients)
			subCount := len(h.subscriptions)
			h.mu.RUnlock()

			log.Debug().
				Int("clients", clientCount).
				Int("subscriptions", subCount).
				Msg("WebSocket hub health check")
		}
	}
}

// registerClient adds a client to the hub
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = true
	log.Info().
		Str("client_id", client.id).
		Int("total_clients", len(h.clients)).
		Msg("Client registered")
}

// unregisterClient removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		// Remove from all subscriptions
		for sensorID, subscribers := range h.subscriptions {
			delete(subscribers, client)
			if len(subscribers) == 0 {
				delete(h.subscriptions, sensorID)
			}
		}

		// Remove from clients
		delete(h.clients, client)
		close(client.send)

		log.Info().
			Str("client_id", client.id).
			Int("total_clients", len(h.clients)).
			Msg("Client unregistered")
	}
}

// Subscribe subscribes a client to a sensor
func (h *Hub) Subscribe(client *Client, sensorID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.subscriptions[sensorID] == nil {
		h.subscriptions[sensorID] = make(map[*Client]bool)
	}

	h.subscriptions[sensorID][client] = true

	log.Info().
		Str("client_id", client.id).
		Str("sensor_id", sensorID).
		Int("subscribers", len(h.subscriptions[sensorID])).
		Msg("Client subscribed to sensor")
}

// Unsubscribe unsubscribes a client from a sensor
func (h *Hub) Unsubscribe(client *Client, sensorID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subscribers, ok := h.subscriptions[sensorID]; ok {
		delete(subscribers, client)
		if len(subscribers) == 0 {
			delete(h.subscriptions, sensorID)
		}

		log.Info().
			Str("client_id", client.id).
			Str("sensor_id", sensorID).
			Msg("Client unsubscribed from sensor")
	}
}

// GetSubscribers returns the number of subscribers for a sensor
func (h *Hub) GetSubscribers(sensorID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if subscribers, ok := h.subscriptions[sensorID]; ok {
		return len(subscribers)
	}
	return 0
}

// broadcastMessage sends a message to all subscribed clients
func (h *Hub) broadcastMessage(msg *BroadcastMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// If sensorID is "all", broadcast to all clients
	if msg.SensorID == "all" {
		for client := range h.clients {
			select {
			case client.send <- msg.Data:
			default:
				// Client's send channel is full, skip
				log.Warn().
					Str("client_id", client.id).
					Msg("Client send buffer full, dropping message")
			}
		}
		return
	}

	// Broadcast to subscribed clients for specific sensor
	if subscribers, ok := h.subscriptions[msg.SensorID]; ok {
		for client := range subscribers {
			select {
			case client.send <- msg.Data:
			default:
				log.Warn().
					Str("client_id", client.id).
					Str("sensor_id", msg.SensorID).
					Msg("Client send buffer full, dropping message")
			}
		}
	}

	// Also send to clients subscribed to "all"
	if allSubscribers, ok := h.subscriptions["all"]; ok {
		for client := range allSubscribers {
			select {
			case client.send <- msg.Data:
			default:
				log.Warn().
					Str("client_id", client.id).
					Msg("Client send buffer full, dropping message")
			}
		}
	}
}

// BroadcastSensorUpdate broadcasts a sensor update to subscribed clients
func (h *Hub) BroadcastSensorUpdate(sensorID string, data interface{}) {
	message := map[string]interface{}{
		"type": "sensor_update",
		"data": data,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal sensor update")
		return
	}

	h.broadcast <- &BroadcastMessage{
		SensorID: sensorID,
		Data:     jsonData,
	}
}

// BroadcastAlert broadcasts an alert to all clients
func (h *Hub) BroadcastAlert(alert interface{}) {
	message := map[string]interface{}{
		"type": "alert",
		"data": alert,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal alert")
		return
	}

	h.broadcast <- &BroadcastMessage{
		SensorID: "all",
		Data:     jsonData,
	}
}

// GetStats returns hub statistics
func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"total_clients":       len(h.clients),
		"total_subscriptions": len(h.subscriptions),
	}
}

// closeAllConnections closes all client connections
func (h *Hub) closeAllConnections() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		close(client.send)
	}

	h.clients = make(map[*Client]bool)
	h.subscriptions = make(map[string]map[*Client]bool)
}

// IsValidSensorID checks if a sensor ID is valid
func IsValidSensorID(sensorID string) bool {
	if sensorID == "all" {
		return true
	}

	_, err := uuid.Parse(sensorID)
	return err == nil
}
