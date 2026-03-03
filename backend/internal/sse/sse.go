package sse

import (
	"encoding/json"
	"sync"

	"github.com/rs/zerolog/log"
)

// Event represents an SSE event
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Client represents a connected SSE client
type Client chan string

// Broadcaster manages SSE client connections and message broadcasting
type Broadcaster struct {
	// Registered clients map: Client -> SessionID (string)
	clients map[Client]string

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Struct for client registration with optional session
	NewClients chan clientRegistration

	// Channel for client disconnections
	ClosingClients chan Client

	// Channel for broadcasting messages
	Messages chan messageWrapper
}

type clientRegistration struct {
	client    Client
	sessionID string
}

type messageWrapper struct {
	sessionID string
	data      string
}

// NewBroadcaster creates a new Broadcaster instance
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients:        make(map[Client]string),
		NewClients:     make(chan clientRegistration),
		ClosingClients: make(chan Client),
		Messages:       make(chan messageWrapper, 100),
	}
}

// NewClientRegistration creates a new registration object
func NewClientRegistration(client Client, sessionID string) clientRegistration {
	return clientRegistration{
		client:    client,
		sessionID: sessionID,
	}
}

// Listen starts the broadcaster's main loop
func (b *Broadcaster) Listen() {
	for {
		select {
		case reg := <-b.NewClients:
			b.mu.Lock()
			b.clients[reg.client] = reg.sessionID
			b.mu.Unlock()
			log.Info().
				Int("total_clients", len(b.clients)).
				Str("session_id", reg.sessionID).
				Msg("New SSE client connected")

		case s := <-b.ClosingClients:
			b.mu.Lock()
			delete(b.clients, s)
			b.mu.Unlock()
			log.Info().Int("total_clients", len(b.clients)).Msg("SSE client disconnected")

		case msg := <-b.Messages:
			b.mu.RLock()
			for client, sessionID := range b.clients {
				// Only send if session matches or if message is global (msg.sessionID == "")
				if msg.sessionID == "" || msg.sessionID == sessionID {
					select {
					case client <- msg.data:
					default:
						log.Warn().Msg("SSE client buffer full, dropping message")
					}
				}
			}
			b.mu.RUnlock()
		}
	}
}

// BroadcastSensorUpdate broadcasts a sensor update to a specific session
func (b *Broadcaster) BroadcastSensorUpdate(sessionID string, data interface{}) {
	b.broadcast(sessionID, "sensor_update", data)
}

// BroadcastStats broadcasts global stats updates to all connected clients.
func (b *Broadcaster) BroadcastStats(data interface{}) {
	b.broadcast("", "stats_update", data)
}

func (b *Broadcaster) broadcast(sessionID string, eventType string, data interface{}) {
	event := Event{
		Type: eventType,
		Data: data,
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal SSE event")
		return
	}

	b.Messages <- messageWrapper{
		sessionID: sessionID,
		data:      string(jsonData),
	}
}

// GetStats returns broadcaster statistics
func (b *Broadcaster) GetStats() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return map[string]interface{}{
		"total_clients": len(b.clients),
	}
}
