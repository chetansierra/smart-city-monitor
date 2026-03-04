package handlers

import (
	"context"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/gofiber/fiber/v2"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	db          *postgres.DB
	redisClient *redis.Client
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *postgres.DB, redisClient *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:          db,
		redisClient: redisClient,
	}
}

// HealthStatus represents the health status of a service
type HealthStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// HealthResponse represents the overall health check response
type HealthResponse struct {
	Status           string                  `json:"status"`
	Timestamp        string                  `json:"timestamp"`
	Services         map[string]HealthStatus `json:"services"`
	Uptime           string                  `json:"uptime,omitempty"`
	LastDataReceived string                  `json:"last_data_received,omitempty"`
	MessageCount     int64                   `json:"message_count,omitempty"`
}

var startTime = time.Now()

// Check handles GET /health
func (h *HealthHandler) Check(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	services := make(map[string]HealthStatus)
	overallStatus := "healthy"

	// Check PostgreSQL
	if err := h.db.PingContext(ctx); err != nil {
		services["postgres"] = HealthStatus{
			Status:  "unhealthy",
			Message: err.Error(),
		}
		overallStatus = "unhealthy"
	} else {
		services["postgres"] = HealthStatus{
			Status: "healthy",
		}
	}

	// Check Redis
	if err := h.redisClient.Ping(ctx).Err(); err != nil {
		services["redis"] = HealthStatus{
			Status:  "unhealthy",
			Message: err.Error(),
		}
		overallStatus = "unhealthy"
	} else {
		services["redis"] = HealthStatus{
			Status: "healthy",
		}
	}

	// Calculate uptime
	uptime := time.Since(startTime).Round(time.Second).String()

	// Check last data timestamp from Redis
	var lastDataReceived string
	var messageCount int64

	// Try to get a recent sensor reading timestamp
	if sensors, err := h.db.GetAllSensors(ctx); err == nil && len(sensors) > 0 {
		// Check the most recent reading timestamp in Redis
		for _, sensor := range sensors[:1] { // Just check first sensor
			if data, err := h.redisClient.HGetAll(ctx, "sensor:latest:"+sensor.ID.String()).Result(); err == nil {
				if ts, ok := data["timestamp"]; ok {
					lastDataReceived = ts
					break
				}
			}
		}
	}

	response := HealthResponse{
		Status:           overallStatus,
		Timestamp:        time.Now().UTC().Format(time.RFC3339Nano),
		Services:         services,
		Uptime:           uptime,
		LastDataReceived: lastDataReceived,
		MessageCount:     messageCount,
	}

	statusCode := fiber.StatusOK
	if overallStatus == "unhealthy" {
		statusCode = fiber.StatusServiceUnavailable
	}

	return c.Status(statusCode).JSON(response)
}
