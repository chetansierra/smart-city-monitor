package middleware

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// RateLimitConfig defines rate limiting configuration
type RateLimitConfig struct {
	// Max number of requests allowed within the window
	Max int
	// Time window for rate limiting
	Window time.Duration
	// Optional: Custom key generator (defaults to IP-based)
	KeyGenerator func(*fiber.Ctx) string
	// Optional: Custom response when limit exceeded
	LimitReachedHandler func(*fiber.Ctx) error
	// Redis client for distributed rate limiting
	RedisClient *redis.Client
}

// RateLimiter creates a new rate limiting middleware
func RateLimiter(config RateLimitConfig) fiber.Handler {
	// Set defaults
	if config.Max == 0 {
		config.Max = 100 // Default: 100 requests
	}
	if config.Window == 0 {
		config.Window = 1 * time.Minute // Default: per minute
	}
	if config.KeyGenerator == nil {
		config.KeyGenerator = func(c *fiber.Ctx) string {
			// Use IP address as default key
			return c.IP()
		}
	}
	if config.LimitReachedHandler == nil {
		config.LimitReachedHandler = func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "Too many requests. Please try again later.",
				},
			})
		}
	}

	return func(c *fiber.Ctx) error {
		// Generate rate limit key
		key := fmt.Sprintf("ratelimit:%s", config.KeyGenerator(c))

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Get current count from Redis
		count, err := config.RedisClient.Incr(ctx, key)
		if err != nil {
			log.Error().Err(err).Str("key", key).Msg("Failed to increment rate limit counter")
			// On Redis error, allow the request (fail open)
			return c.Next()
		}

		// Set expiration on first request
		if count == 1 {
			err = config.RedisClient.Client.Expire(ctx, key, config.Window).Err()
			if err != nil {
				log.Error().Err(err).Str("key", key).Msg("Failed to set rate limit expiration")
			}
		}

		// Set rate limit headers (use config.Window as reset estimate to avoid extra Redis TTL call)
		c.Set("X-RateLimit-Limit", strconv.Itoa(config.Max))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(max(0, config.Max-int(count))))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(config.Window).Unix(), 10))

		// Check if limit exceeded
		if count > int64(config.Max) {
			c.Set("Retry-After", strconv.FormatInt(int64(config.Window.Seconds()), 10))
			log.Warn().
				Str("key", key).
				Int64("count", count).
				Int("max", config.Max).
				Msg("Rate limit exceeded")
			return config.LimitReachedHandler(c)
		}

		return c.Next()
	}
}

// PerEndpointRateLimiter creates rate limiters for specific endpoints
func PerEndpointRateLimiter(redisClient *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()

		// Define rate limits per endpoint type
		var config RateLimitConfig

		switch {
		case path == "/api/v1/sensors" && c.Method() == "POST":
			// Stricter limit for sensor creation
			config = RateLimitConfig{
				Max:         10,
				Window:      1 * time.Minute,
				RedisClient: redisClient,
				KeyGenerator: func(c *fiber.Ctx) string {
					return fmt.Sprintf("%s:%s:%s", c.IP(), c.Method(), c.Path())
				},
			}
		case path == "/api/v1/readings/latest":
			// Higher limit for read-heavy endpoints
			config = RateLimitConfig{
				Max:         200,
				Window:      1 * time.Minute,
				RedisClient: redisClient,
				KeyGenerator: func(c *fiber.Ctx) string {
					return fmt.Sprintf("%s:%s", c.IP(), c.Path())
				},
			}
		case contains(path, "/analytics/"):
			// Moderate limit for analytics (potentially expensive queries)
			config = RateLimitConfig{
				Max:         50,
				Window:      1 * time.Minute,
				RedisClient: redisClient,
				KeyGenerator: func(c *fiber.Ctx) string {
					return fmt.Sprintf("%s:%s", c.IP(), "analytics")
				},
			}
		default:
			// Default rate limit
			config = RateLimitConfig{
				Max:         100,
				Window:      1 * time.Minute,
				RedisClient: redisClient,
				KeyGenerator: func(c *fiber.Ctx) string {
					return c.IP()
				},
			}
		}

		// Apply rate limiting
		limiter := RateLimiter(config)
		return limiter(c)
	}
}

// APIKeyRateLimiter creates rate limiters based on API keys
// For future use when authentication is implemented
func APIKeyRateLimiter(redisClient *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := c.Get("X-API-Key")

		if apiKey == "" {
			// No API key, use IP-based rate limiting
			config := RateLimitConfig{
				Max:         50, // Lower limit for unauthenticated requests
				Window:      1 * time.Minute,
				RedisClient: redisClient,
				KeyGenerator: func(c *fiber.Ctx) string {
					return fmt.Sprintf("ip:%s", c.IP())
				},
			}
			limiter := RateLimiter(config)
			return limiter(c)
		}

		// API key present, use higher limits
		config := RateLimitConfig{
			Max:         500, // Higher limit for authenticated requests
			Window:      1 * time.Minute,
			RedisClient: redisClient,
			KeyGenerator: func(c *fiber.Ctx) string {
				return fmt.Sprintf("apikey:%s", apiKey)
			},
		}
		limiter := RateLimiter(config)
		return limiter(c)
	}
}

// Helper functions

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) > len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
