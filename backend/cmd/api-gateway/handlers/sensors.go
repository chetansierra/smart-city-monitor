package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// SensorsHandler handles sensor-related requests
type SensorsHandler struct {
	db          *postgres.DB
	redisClient *redis.Client
}

// NewSensorsHandler creates a new sensors handler
func NewSensorsHandler(db *postgres.DB, redisClient *redis.Client) *SensorsHandler {
	return &SensorsHandler{
		db:          db,
		redisClient: redisClient,
	}
}

// APIResponse is a standardized response format
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// APIError represents an error response
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Meta represents pagination and other metadata
type Meta struct {
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
	Total int `json:"total,omitempty"`
}

// ListAll handles GET /api/v1/sensors
func (h *SensorsHandler) ListAll(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	sensorType := c.Query("type", "")

	// Validate pagination
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 1000 {
		limit = 50
	}

	log.Info().
		Int("page", page).
		Int("limit", limit).
		Str("type", sensorType).
		Msg("Listing sensors")

	// Get sensors from database
	sensors, err := h.db.GetAllSensors(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch sensors")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to fetch sensors",
			},
		})
	}

	// Filter by type if specified
	if sensorType != "" {
		filtered := make([]models.Sensor, 0)
		for _, sensor := range sensors {
			if string(sensor.Type) == sensorType {
				filtered = append(filtered, sensor)
			}
		}
		sensors = filtered
	}

	// Apply pagination
	total := len(sensors)
	offset := (page - 1) * limit
	end := offset + limit

	if offset >= total {
		sensors = []models.Sensor{}
	} else {
		if end > total {
			end = total
		}
		sensors = sensors[offset:end]
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    sensors,
		Meta: &Meta{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	})
}

// GetByID handles GET /api/v1/sensors/:id
func (h *SensorsHandler) GetByID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Parse sensor ID
	idStr := c.Params("id")
	sensorID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_SENSOR_ID",
				Message: "Invalid sensor ID format",
			},
		})
	}

	log.Info().Str("sensor_id", sensorID.String()).Msg("Fetching sensor by ID")

	// Get sensor from database
	sensor, err := h.db.GetSensorByID(ctx, sensorID)
	if err != nil {
		log.Error().Err(err).Str("sensor_id", sensorID.String()).Msg("Sensor not found")
		return c.Status(fiber.StatusNotFound).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "SENSOR_NOT_FOUND",
				Message: fmt.Sprintf("Sensor with ID %s not found", sensorID.String()),
			},
		})
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    sensor,
	})
}

// LatestReading represents the latest reading from Redis
type LatestReading struct {
	SensorID   string  `json:"sensor_id"`
	SensorType string  `json:"sensor_type"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Timestamp  string  `json:"timestamp"`
}

// GetLatest handles GET /api/v1/sensors/:id/latest
func (h *SensorsHandler) GetLatest(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Parse sensor ID
	idStr := c.Params("id")
	sensorID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_SENSOR_ID",
				Message: "Invalid sensor ID format",
			},
		})
	}

	log.Info().Str("sensor_id", sensorID.String()).Msg("Fetching latest reading from Redis")

	// Get latest reading from Redis
	key := fmt.Sprintf("sensor:latest:%s", sensorID.String())
	data, err := h.redisClient.HGetAll(ctx, key).Result()
	if err != nil {
		log.Error().Err(err).Str("sensor_id", sensorID.String()).Msg("Failed to fetch from Redis")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "REDIS_ERROR",
				Message: "Failed to fetch latest reading",
			},
		})
	}

	if len(data) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "NO_RECENT_DATA",
				Message: "No recent data available for this sensor",
			},
		})
	}

	// Parse the hash fields into LatestReading
	var value float64
	var latitude float64
	var longitude float64

	if v, ok := data["value"]; ok {
		fmt.Sscanf(v, "%f", &value)
	}
	if lat, ok := data["latitude"]; ok {
		fmt.Sscanf(lat, "%f", &latitude)
	}
	if lon, ok := data["longitude"]; ok {
		fmt.Sscanf(lon, "%f", &longitude)
	}

	reading := LatestReading{
		SensorID:   data["sensor_id"],
		SensorType: data["sensor_type"],
		Value:      value,
		Unit:       data["unit"],
		Latitude:   latitude,
		Longitude:  longitude,
		Timestamp:  data["timestamp"],
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    reading,
	})
}
