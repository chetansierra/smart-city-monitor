package main

import (
	"github.com/chetansierra/smart-city-monitor/cmd/api-gateway/handlers"
	"github.com/chetansierra/smart-city-monitor/internal/middleware"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// setupRoutes configures all API routes
func setupRoutes(app *fiber.App, healthHandler *handlers.HealthHandler, sensorsHandler *handlers.SensorsHandler, readingsHandler *handlers.ReadingsHandler, analyticsHandler *handlers.AnalyticsHandler, alertsHandler *handlers.AlertsHandler, wsHandler *handlers.WebSocketHandler, redisClient *redis.Client) {
	// Health check endpoint
	app.Get("/health", healthHandler.Check)

	// WebSocket stats endpoint (before WebSocket middleware)
	app.Get("/ws/stats", wsHandler.GetStats)

	// WebSocket endpoint
	app.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client requested upgrade to the WebSocket protocol
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", websocket.New(wsHandler.HandleConnection))

	// API v1 routes
	v1 := app.Group("/api/v1")

	// Apply rate limiting to all API routes
	v1.Use(middleware.PerEndpointRateLimiter(redisClient))

	// Sensor routes
	sensors := v1.Group("/sensors")
	sensors.Get("/", sensorsHandler.ListAll)                    // GET /api/v1/sensors
	sensors.Get("/:id", sensorsHandler.GetByID)                 // GET /api/v1/sensors/:id
	sensors.Get("/:id/latest", sensorsHandler.GetLatest)        // GET /api/v1/sensors/:id/latest
	sensors.Get("/:id/readings", readingsHandler.GetBySensorID) // GET /api/v1/sensors/:id/readings
	sensors.Get("/:id/alerts", alertsHandler.GetBySensorID)     // GET /api/v1/sensors/:id/alerts

	// Readings routes
	readings := v1.Group("/readings")
	readings.Get("/", readingsHandler.GetAll)          // GET /api/v1/readings
	readings.Get("/latest", readingsHandler.GetLatest) // GET /api/v1/readings/latest

	// Analytics routes
	analytics := v1.Group("/analytics")
	analytics.Get("/city-stats", analyticsHandler.GetCityStats)                 // GET /api/v1/analytics/city-stats
	analytics.Get("/sensors/:id/hourly", analyticsHandler.GetSensorHourlyStats) // GET /api/v1/analytics/sensors/:id/hourly
	analytics.Get("/top-polluted", analyticsHandler.GetTopPolluted)             // GET /api/v1/analytics/top-polluted
	analytics.Get("/top-temperature", analyticsHandler.GetTopTemperature)       // GET /api/v1/analytics/top-temperature
	analytics.Get("/quietest", analyticsHandler.GetQuietest)                    // GET /api/v1/analytics/quietest

	// Alerts routes
	alerts := v1.Group("/alerts")
	alerts.Get("/", alertsHandler.GetAll)                      // GET /api/v1/alerts
	alerts.Get("/:id", alertsHandler.GetByID)                  // GET /api/v1/alerts/:id
	alerts.Post("/:id/acknowledge", alertsHandler.Acknowledge) // POST /api/v1/alerts/:id/acknowledge
}
