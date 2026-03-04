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

// GetAll handles GET /api/v1/readings
func (h *ReadingsHandler) GetAll(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	sensorIDStr := c.Query("sensor_id", "")
	sensorType := c.Query("type", "")
	fromStr := c.Query("from", "")
	toStr := c.Query("to", "")
	sortOrder := c.Query("sort", "desc")

	// Validate pagination
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 1000 {
		limit = 50
	}

	// Validate sort order
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	log.Info().
		Int("page", page).
		Int("limit", limit).
		Str("sensor_id", sensorIDStr).
		Str("type", sensorType).
		Str("from", fromStr).
		Str("to", toStr).
		Str("sort", sortOrder).
		Msg("Fetching readings")

	// Parse optional sensor ID
	var sensorID *uuid.UUID
	if sensorIDStr != "" {
		parsed, err := uuid.Parse(sensorIDStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
				Success: false,
				Error: &APIError{
					Code:    "INVALID_SENSOR_ID",
					Message: "Invalid sensor ID format",
				},
			})
		}
		sensorID = &parsed
	}

	// Parse timestamps
	var from, to time.Time
	var err error

	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
				Success: false,
				Error: &APIError{
					Code:    "INVALID_TIMESTAMP",
					Message: "Invalid 'from' timestamp format. Use RFC3339 format (e.g., 2025-11-19T00:00:00Z)",
				},
			})
		}
	}

	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
				Success: false,
				Error: &APIError{
					Code:    "INVALID_TIMESTAMP",
					Message: "Invalid 'to' timestamp format. Use RFC3339 format (e.g., 2025-11-19T23:59:59Z)",
				},
			})
		}
	}

	// Set default time range if not specified (last 24 hours)
	if fromStr == "" && toStr == "" {
		to = time.Now()
		from = to.Add(-24 * time.Hour)
	} else if fromStr == "" {
		from = to.Add(-24 * time.Hour)
	} else if toStr == "" {
		to = time.Now()
	}

	// Validate time range
	if from.After(to) {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_TIME_RANGE",
				Message: "'from' timestamp cannot be after 'to' timestamp",
			},
		})
	}

	// Fetch readings based on filters
	var readings []models.SensorReading

	if sensorID != nil {
		// Get readings for specific sensor
		readings, err = h.db.GetReadingsInTimeRange(ctx, *sensorID, from, to)
		if err != nil {
			log.Error().Err(err).Msg("Failed to fetch readings for sensor")
			return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
				Success: false,
				Error: &APIError{
					Code:    "DATABASE_ERROR",
					Message: "Failed to fetch readings",
				},
			})
		}
		// Filter by sensor type if specified
		if sensorType != "" {
			filtered := make([]models.SensorReading, 0)
			for _, reading := range readings {
				if string(reading.SensorType) == sensorType {
					filtered = append(filtered, reading)
				}
			}
			readings = filtered
		}
	} else {
		// Get readings for all sensors in a single query
		readings, err = h.db.GetReadingsInTimeRangeForAllSensors(ctx, from, to, sensorType)
		if err != nil {
			log.Error().Err(err).Msg("Failed to fetch readings")
			return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
				Success: false,
				Error: &APIError{
					Code:    "DATABASE_ERROR",
					Message: "Failed to fetch readings",
				},
			})
		}
	}

	// Sort readings
	if sortOrder == "asc" {
		// Readings are by default DESC from DB, so reverse for ASC
		for i, j := 0, len(readings)-1; i < j; i, j = i+1, j-1 {
			readings[i], readings[j] = readings[j], readings[i]
		}
	}

	// Apply pagination
	total := len(readings)
	offset := (page - 1) * limit
	end := offset + limit

	if offset >= total {
		readings = []models.SensorReading{}
	} else {
		if end > total {
			end = total
		}
		readings = readings[offset:end]
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    readings,
		Meta: &Meta{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	})
}

// GetBySensorID handles GET /api/v1/sensors/:id/readings
func (h *ReadingsHandler) GetBySensorID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Parse query parameters
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	fromStr := c.Query("from", "")
	toStr := c.Query("to", "")
	sortOrder := c.Query("sort", "desc")

	// Validate pagination
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 1000 {
		limit = 50
	}

	// Validate sort order
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	log.Info().
		Str("sensor_id", sensorID.String()).
		Int("page", page).
		Int("limit", limit).
		Str("from", fromStr).
		Str("to", toStr).
		Str("sort", sortOrder).
		Msg("Fetching readings for sensor")

	// Parse timestamps
	var from, to time.Time

	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
				Success: false,
				Error: &APIError{
					Code:    "INVALID_TIMESTAMP",
					Message: "Invalid 'from' timestamp format. Use RFC3339 format (e.g., 2025-11-19T00:00:00Z)",
				},
			})
		}
	}

	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
				Success: false,
				Error: &APIError{
					Code:    "INVALID_TIMESTAMP",
					Message: "Invalid 'to' timestamp format. Use RFC3339 format (e.g., 2025-11-19T23:59:59Z)",
				},
			})
		}
	}

	// Set default time range if not specified (last 24 hours)
	if fromStr == "" && toStr == "" {
		to = time.Now()
		from = to.Add(-24 * time.Hour)
	} else if fromStr == "" {
		from = to.Add(-24 * time.Hour)
	} else if toStr == "" {
		to = time.Now()
	}

	// Validate time range
	if from.After(to) {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_TIME_RANGE",
				Message: "'from' timestamp cannot be after 'to' timestamp",
			},
		})
	}

	// Get readings from database
	readings, err := h.db.GetReadingsInTimeRange(ctx, sensorID, from, to)
	if err != nil {
		log.Error().Err(err).Str("sensor_id", sensorID.String()).Msg("Failed to fetch readings")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to fetch readings",
			},
		})
	}

	// Sort readings if asc requested (default from DB is desc)
	if sortOrder == "asc" {
		for i, j := 0, len(readings)-1; i < j; i, j = i+1, j-1 {
			readings[i], readings[j] = readings[j], readings[i]
		}
	}

	// Apply pagination
	total := len(readings)
	offset := (page - 1) * limit
	end := offset + limit

	if offset >= total {
		readings = []models.SensorReading{}
	} else {
		if end > total {
			end = total
		}
		readings = readings[offset:end]
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    readings,
		Meta: &Meta{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	})
}

// GetLatest handles GET /api/v1/readings/latest
func (h *ReadingsHandler) GetLatest(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Parse query parameters
	sensorType := c.Query("type", "")

	log.Info().
		Str("type", sensorType).
		Msg("Fetching latest readings for all sensors")

	// Get all sensors
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

	// Fetch latest reading for each sensor from Redis
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

	latestReadings := make([]LatestReadingWithSensor, 0)

	for _, sensor := range sensors {
		key := fmt.Sprintf("sensor:latest:%s", sensor.ID.String())
		data, err := h.redisClient.HGetAll(ctx, key).Result()
		if err != nil {
			log.Warn().Err(err).Str("sensor_id", sensor.ID.String()).Msg("Failed to fetch latest reading from Redis")
			continue
		}

		if len(data) == 0 {
			log.Debug().Str("sensor_id", sensor.ID.String()).Msg("No recent data in Redis")
			continue
		}

		// Parse the hash fields
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

		reading := LatestReadingWithSensor{
			SensorID:   sensor.ID.String(),
			SensorName: sensor.Name,
			SensorType: data["sensor_type"],
			Value:      value,
			Unit:       data["unit"],
			Latitude:   latitude,
			Longitude:  longitude,
			Timestamp:  data["timestamp"],
		}

		latestReadings = append(latestReadings, reading)
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    latestReadings,
		Meta: &Meta{
			Total: len(latestReadings),
		},
	})
}
