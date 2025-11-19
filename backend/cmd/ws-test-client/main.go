package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	url := "ws://localhost:8080/ws"
	fmt.Printf("Connecting to %s...\n", url)

	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("Dial error:", err)
	}
	defer c.Close()

	fmt.Println("Connected!")

	done := make(chan struct{})

	// Read messages
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("Read error:", err)
				return
			}
			fmt.Printf("Received: %s\n", message)
		}
	}()

	// Subscribe to all sensors
	subscribeMsg := map[string]string{
		"action":    "subscribe",
		"sensor_id": "all",
	}
	data, _ := json.Marshal(subscribeMsg)
	fmt.Printf("Sending: %s\n", string(data))

	err = c.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Println("Write error:", err)
		return
	}

	// Wait for 10 seconds to receive messages
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	select {
	case <-done:
		return
	case <-ticker.C:
		fmt.Println("\nTimeout - closing connection")
	case <-interrupt:
		fmt.Println("\nInterrupt received - closing connection")

		// Cleanly close the connection
		err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("Write close error:", err)
			return
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}
}
