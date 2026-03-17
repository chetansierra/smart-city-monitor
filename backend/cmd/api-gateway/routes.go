package main

import (
	"github.com/chetansierra/smart-city-monitor/cmd/api-gateway/handlers"
	"github.com/chetansierra/smart-city-monitor/internal/middleware"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/gofiber/fiber/v2"
)

// setupRoutes configures all API routes
func setupRoutes(app *fiber.App, healthHandler *handlers.HealthHandler, sensorsHandler *handlers.SensorsHandler, readingsHandler *handlers.ReadingsHandler, analyticsHandler *handlers.AnalyticsHandler, sseHandler *handlers.SSEHandler, metricsHandler *handlers.MetricsHandler, adminHandler *handlers.AdminHandler, pipelineHandler *handlers.PipelineHandler, redisClient *redis.Client) {
	// Health check endpoint
	app.Get("/health", healthHandler.Check)

	// SSE stats endpoint
	app.Get("/stream/stats", sseHandler.GetStats)
	app.Get("/api/v1/stream/history", sseHandler.GetSessionHistory)

	// SSE streaming endpoint
	app.Get("/stream", sseHandler.HandleStream)

	// API v1 routes
	v1 := app.Group("/api/v1")

	// Apply rate limiting to all API routes
	v1.Use(middleware.PerEndpointRateLimiter(redisClient))

	// Sensor routes
	sensors := v1.Group("/sensors")
	sensors.Get("/", sensorsHandler.ListAll)                    // GET /api/v1/sensors
	sensors.Get("/footprint", sensorsHandler.GetFootprint)      // GET /api/v1/sensors/footprint
	sensors.Post("/start", sensorsHandler.StartSensor)          // POST /api/v1/sensors/start
	sensors.Delete("/:id", sensorsHandler.DeleteSensor)         // DELETE /api/v1/sensors/:id
	sensors.Get("/:id", sensorsHandler.GetByID)                 // GET /api/v1/sensors/:id
	sensors.Get("/:id/latest", sensorsHandler.GetLatest)        // GET /api/v1/sensors/:id/latest
	sensors.Get("/:id/readings", readingsHandler.GetBySensorID) // GET /api/v1/sensors/:id/readings

	// Session routes
	session := v1.Group("/session")
	session.Get("/config", sensorsHandler.GetSessionConfig)       // GET /api/v1/session/config
	session.Put("/config", sensorsHandler.UpdateSessionConfig)   // PUT /api/v1/session/config
	session.Delete("/sensors", sensorsHandler.TeardownSession)   // DELETE /api/v1/session/sensors
	session.Post("/teardown", sensorsHandler.TeardownSession)    // POST /api/v1/session/teardown (for sendBeacon)

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
	analytics.Get("/hourly", analyticsHandler.GetHourlyAggregations)            // GET /api/v1/analytics/hourly
	analytics.Get("/compare", analyticsHandler.GetComparisonData)               // GET /api/v1/analytics/compare
	analytics.Get("/zones", analyticsHandler.GetZoneAnalytics)                  // GET /api/v1/analytics/zones
	analytics.Get("/anomalies", analyticsHandler.GetAnomalies)                  // GET /api/v1/analytics/anomalies
	analytics.Get("/events", analyticsHandler.GetDetectedEvents)               // GET /api/v1/analytics/events

	// Metrics routes
	metrics := v1.Group("/metrics")
	metrics.Get("/kafka", metricsHandler.GetKafkaMetrics)  // GET /api/v1/metrics/kafka
	metrics.Get("/redis", metricsHandler.GetRedisMetrics)  // GET /api/v1/metrics/redis
	metrics.Get("/system", metricsHandler.GetSystemHealth) // GET /api/v1/metrics/system
	metrics.Get("/nerds", metricsHandler.GetNerdStats)     // GET /api/v1/metrics/nerds
	metrics.Get("/nerds/global", metricsHandler.GetNerdStats)
	metrics.Get("/nerds/session", metricsHandler.GetSessionNerdStats)

	// Pipeline visualization routes
	pipeline := v1.Group("/pipeline")
	pipeline.Get("/flow/stats", pipelineHandler.GetDataFlowStats)                   // GET /api/v1/pipeline/flow/stats
	pipeline.Get("/kafka/topics", pipelineHandler.GetKafkaTopics)                   // GET /api/v1/pipeline/kafka/topics
	pipeline.Get("/kafka/topics/:topic", pipelineHandler.GetKafkaTopicDetail)       // GET /api/v1/pipeline/kafka/topics/:topic
	pipeline.Get("/kafka/topics/:topic/messages", pipelineHandler.GetKafkaMessages) // GET /api/v1/pipeline/kafka/topics/:topic/messages
	pipeline.Get("/redis/keys", pipelineHandler.GetRedisKeys)                       // GET /api/v1/pipeline/redis/keys
	pipeline.Get("/redis/keys/:key", pipelineHandler.GetRedisKeyDetail)             // GET /api/v1/pipeline/redis/keys/:key
	pipeline.Get("/dlq", pipelineHandler.GetDLQMessages)                             // GET /api/v1/pipeline/dlq
	pipeline.Get("/stream/connections", sseHandler.GetStats)                        // GET /api/v1/pipeline/stream/connections

	// Admin routes
	admin := v1.Group("/admin")
	admin.Post("/sensors/control", adminHandler.ControlSensors)                 // POST /api/v1/admin/sensors/control
	admin.Post("/sensors/start-all", adminHandler.StartAllSensors)              // POST /api/v1/admin/sensors/start-all
	admin.Post("/sensors/stop-all", adminHandler.StopAllSensors)                // POST /api/v1/admin/sensors/stop-all
	admin.Post("/simulation/rate", adminHandler.SetSimulationRate)              // POST /api/v1/admin/simulation/rate
	admin.Get("/simulation/status", adminHandler.GetSimulationStatus)           // GET /api/v1/admin/simulation/status
	admin.Post("/system/clear-cache", adminHandler.ClearRedisCache)             // POST /api/v1/admin/system/clear-cache
	admin.Post("/system/reset", adminHandler.ResetSimulation)                   // POST /api/v1/admin/system/reset
	admin.Get("/scenarios", adminHandler.GetScenarios)                          // GET /api/v1/admin/scenarios
	admin.Post("/scenarios/activate", adminHandler.ActivateScenario)            // POST /api/v1/admin/scenarios/activate
	admin.Get("/scenarios/current", adminHandler.GetCurrentScenario)            // GET /api/v1/admin/scenarios/current
	admin.Get("/simulation/config", adminHandler.GetSimulationConfig)           // GET /api/v1/admin/simulation/config
	admin.Post("/simulation/threshold", adminHandler.SetSensorThreshold)        // POST /api/v1/admin/simulation/threshold
	admin.Post("/simulation/behavior", adminHandler.SetSensorBehavior)          // POST /api/v1/admin/simulation/behavior
	admin.Post("/simulation/time-compression", adminHandler.SetTimeCompression) // POST /api/v1/admin/simulation/time-compression
	admin.Post("/simulation/chaos-mode", adminHandler.SetChaosMode)             // POST /api/v1/admin/simulation/chaos-mode
	admin.Post("/simulation/reset-config", adminHandler.ResetSimulationConfig)  // POST /api/v1/admin/simulation/reset-config
}
