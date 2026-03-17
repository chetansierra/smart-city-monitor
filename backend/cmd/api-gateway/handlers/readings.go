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

// ReadingsHandler handles readings-related requests
type ReadingsHandler struct {
	db          *postgres.DB
	redisClient *redis.Client
}

// NewReadingsHandler creates a new readings handler
func NewReadingsHandler(db *postgres.DB, redisClient *redis.Client) *ReadingsHandler {
	return &ReadingsHandler{
		db:          db,
		redisClient: redisClient,
	}
}

// LatestReadingWithSensor represents a sensor's latest reading.
type LatestReadingWithSensor struct {
	SensorID   string  `json:"sensor_id"`
	SensorName string  `json:"sensor_name"`
	SensorType string  `json:"sensor_type"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Timestamp  string  `json:"timestamp"`
}

// GetAll handles GET /api/v1/readings — delegates to GetLatest (sensor_readings table no longer used)
func (h *ReadingsHandler) GetAll(c *fiber.Ctx) error {
	return h.GetLatest(c)
}

// GetBySensorID handles GET /api/v1/sensors/:id/readings — returns latest reading from Redis
func (h *ReadingsHandler) GetBySensorID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

	sensor, err := h.db.GetSensorByID(ctx, sensorID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "SENSOR_NOT_FOUND",
				Message: "Sensor not found",
			},
		})
	}

	readings := h.fetchLatestReadings(ctx, []models.Sensor{*sensor})

	return c.JSON(APIResponse{
		Success: true,
		Data:    readings,
		Meta:    &Meta{Total: len(readings)},
	})
}

// GetLatest handles GET /api/v1/readings/latest
func (h *ReadingsHandler) GetLatest(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sensorType := c.Query("type", "")

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

	if sensorType != "" {
		filtered := make([]models.Sensor, 0)
		for _, s := range sensors {
			if string(s.Type) == sensorType {
				filtered = append(filtered, s)
			}
		}
		sensors = filtered
	}

	readings := h.fetchLatestReadings(ctx, sensors)

	return c.JSON(APIResponse{
		Success: true,
		Data:    readings,
		Meta:    &Meta{Total: len(readings)},
	})
}

// fetchLatestReadings gets the latest reading for each sensor from Redis.
func (h *ReadingsHandler) fetchLatestReadings(ctx context.Context, sensors []models.Sensor) []LatestReadingWithSensor {
	result := make([]LatestReadingWithSensor, 0, len(sensors))

	for _, sensor := range sensors {
		key := fmt.Sprintf("sensor:latest:%s", sensor.ID.String())
		data, err := h.redisClient.HGetAll(ctx, key).Result()
		if err != nil || len(data) == 0 {
			continue
		}

		var value, latitude, longitude float64
		if v, ok := data["value"]; ok {
			fmt.Sscanf(v, "%f", &value)
		}
		if v, ok := data["latitude"]; ok {
			fmt.Sscanf(v, "%f", &latitude)
		}
		if v, ok := data["longitude"]; ok {
			fmt.Sscanf(v, "%f", &longitude)
		}

		result = append(result, LatestReadingWithSensor{
			SensorID:   sensor.ID.String(),
			SensorName: sensor.Name,
			SensorType: data["sensor_type"],
			Value:      value,
			Unit:       data["unit"],
			Latitude:   latitude,
			Longitude:  longitude,
			Timestamp:  data["timestamp"],
		})
	}

	return result
}
