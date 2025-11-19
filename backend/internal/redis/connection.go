package redis

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

// Client wraps a Redis client
type Client struct {
	*redis.Client
}

// Config holds Redis connection configuration
type Config struct {
	Addr     string
	Password string
	DB       int
}

// Connect establishes a connection to Redis
func Connect(cfg Config) (*Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	log.Printf("Redis client connected to: %s", cfg.Addr)
	return &Client{client}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	if err := c.Client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis client: %w", err)
	}
	log.Println("Redis client closed")
	return nil
}

// Health checks Redis connection health
func (c *Client) Health(ctx context.Context) error {
	if err := c.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis health check failed: %w", err)
	}
	return nil
}
