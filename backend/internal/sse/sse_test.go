package sse

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBroadcaster(t *testing.T) {
	b := NewBroadcaster()
	go b.Listen()

	// Test client registration
	client1 := make(Client, 4)
	b.NewClients <- NewClientRegistration(client1, "session1")
	time.Sleep(10 * time.Millisecond)

	b.mu.RLock()
	assert.Equal(t, "session1", b.clients[client1])
	b.mu.RUnlock()

	// Test sensor update broadcast (correct session)
	data1 := map[string]string{"sensor": "temp-001"}
	b.BroadcastSensorUpdate("session1", data1)

	select {
	case msg := <-client1:
		var event Event
		err := json.Unmarshal([]byte(msg), &event)
		assert.NoError(t, err)
		assert.Equal(t, "sensor_update", event.Type)
	case <-time.After(100 * time.Millisecond):
		t.Errorf("Timed out waiting for message")
	}

	// Test global stats broadcast
	client2 := make(Client, 4)
	b.NewClients <- NewClientRegistration(client2, "session2")
	time.Sleep(10 * time.Millisecond)

	data2 := map[string]float64{"throughput": 12.3}
	b.BroadcastStats(data2)

	// client2 should receive it
	select {
	case msg := <-client2:
		assert.Contains(t, msg, "stats_update")
	case <-time.After(100 * time.Millisecond):
		t.Errorf("client2 timed out")
	}

	// client1 should also receive it (global broadcast)
	select {
	case msg := <-client1:
		assert.Contains(t, msg, "stats_update")
	case <-time.After(50 * time.Millisecond):
		t.Errorf("client1 timed out")
	}

	// Test client disconnection
	b.ClosingClients <- client1
	time.Sleep(10 * time.Millisecond)

	b.mu.RLock()
	_, ok := b.clients[client1]
	assert.False(t, ok)
	b.mu.RUnlock()
}
