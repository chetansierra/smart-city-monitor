package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// AnalyticsHandler handles analytics-related requests
type AnalyticsHandler struct {
	db          *postgres.DB
	redisClient *redis.Client
}

// NewAnalyticsHandler creates a new analytics handler
func NewAnalyticsHandler(db *postgres.DB, redisClient *redis.Client) *AnalyticsHandler {
	return &AnalyticsHandler{
		db:          db,
		redisClient: redisClient,
	}
}

// CityStats represents city-wide statistics
type CityStats struct {
	AvgTemperature float64   `json:"avg_temperature"`
	AvgPollution   float64   `json:"avg_pollution"`
	AvgHumidity    float64   `json:"avg_humidity"`
	AvgNoise       float64   `json:"avg_noise"`
	Timestamp      time.Time `json:"timestamp"`
}

// GetCityStats handles GET /api/v1/analytics/city-stats
func (h *AnalyticsHandler) GetCityStats(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to get from cache first
	cacheKey := "analytics:city-stats"
	cached, err := h.redisClient.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var stats CityStats
		if err := json.Unmarshal([]byte(cached), &stats); err == nil {
			log.Debug().Msg("Returning cached city stats")
			return c.JSON(APIResponse{
				Success: true,
				Data:    stats,
			})
		}
	}

	// Calculate stats from latest readings in Redis
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

	var tempSum, pollutionSum, humiditySum, noiseSum float64
	var tempCount, pollutionCount, humidityCount, noiseCount int

	for _, sensor := range sensors {
		key := fmt.Sprintf("sensor:latest:%s", sensor.ID.String())
		data, err := h.redisClient.HGetAll(ctx, key).Result()
		if err != nil || len(data) == 0 {
			continue
		}

		var value float64
		if v, ok := data["value"]; ok {
			fmt.Sscanf(v, "%f", &value)
		}

		switch sensor.Type {
		case models.SensorTypeTemperature:
			tempSum += value
			tempCount++
		case models.SensorTypePollution:
			pollutionSum += value
			pollutionCount++
		case models.SensorTypeHumidity:
			humiditySum += value
			humidityCount++
		case models.SensorTypeNoise:
			noiseSum += value
			noiseCount++
		}
	}

	stats := CityStats{
		AvgTemperature: 0,
		AvgPollution:   0,
		AvgHumidity:    0,
		AvgNoise:       0,
		Timestamp:      time.Now().UTC(),
	}

	if tempCount > 0 {
		stats.AvgTemperature = tempSum / float64(tempCount)
	}
	if pollutionCount > 0 {
		stats.AvgPollution = pollutionSum / float64(pollutionCount)
	}
	if humidityCount > 0 {
		stats.AvgHumidity = humiditySum / float64(humidityCount)
	}
	if noiseCount > 0 {
		stats.AvgNoise = noiseSum / float64(noiseCount)
	}

	// Cache for 5 minutes
	statsJSON, _ := json.Marshal(stats)
	h.redisClient.Set(ctx, cacheKey, statsJSON, 5*time.Minute)

	return c.JSON(APIResponse{
		Success: true,
		Data:    stats,
	})
}

// SensorStats represents statistics for a sensor
type SensorStats struct {
	SensorID    string    `json:"sensor_id"`
	SensorType  string    `json:"sensor_type"`
	AvgValue    float64   `json:"avg_value"`
	MinValue    float64   `json:"min_value"`
	MaxValue    float64   `json:"max_value"`
	Count       int       `json:"count"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
}

// GetSensorHourlyStats handles GET /api/v1/analytics/sensors/:id/hourly
func (h *AnalyticsHandler) GetSensorHourlyStats(c *fiber.Ctx) error {
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

	// Parse time range
	fromStr := c.Query("from", "")
	toStr := c.Query("to", "")

	var from, to time.Time
	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
				Success: false,
				Error: &APIError{
					Code:    "INVALID_TIMESTAMP",
					Message: "Invalid 'from' timestamp format",
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
					Message: "Invalid 'to' timestamp format",
				},
			})
		}
	}

	// Default to last 24 hours
	if fromStr == "" && toStr == "" {
		to = time.Now()
		from = to.Add(-24 * time.Hour)
	} else if fromStr == "" {
		from = to.Add(-24 * time.Hour)
	} else if toStr == "" {
		to = time.Now()
	}

	// Get readings and calculate hourly stats
	readings, err := h.db.GetReadingsInTimeRange(ctx, sensorID, from, to)
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

	// Group by hour and calculate stats
	hourlyStats := make(map[string]*SensorStats)

	for _, reading := range readings {
		hourKey := reading.Timestamp.Truncate(time.Hour).Format(time.RFC3339)

		if _, exists := hourlyStats[hourKey]; !exists {
			hourlyStats[hourKey] = &SensorStats{
				SensorID:    sensorID.String(),
				SensorType:  string(reading.SensorType),
				MinValue:    reading.Value,
				MaxValue:    reading.Value,
				PeriodStart: reading.Timestamp.Truncate(time.Hour),
				PeriodEnd:   reading.Timestamp.Truncate(time.Hour).Add(time.Hour),
			}
		}

		stat := hourlyStats[hourKey]
		stat.AvgValue += reading.Value
		stat.Count++

		if reading.Value < stat.MinValue {
			stat.MinValue = reading.Value
		}
		if reading.Value > stat.MaxValue {
			stat.MaxValue = reading.Value
		}
	}

	// Calculate averages
	result := make([]SensorStats, 0, len(hourlyStats))
	for _, stat := range hourlyStats {
		if stat.Count > 0 {
			stat.AvgValue = stat.AvgValue / float64(stat.Count)
		}
		result = append(result, *stat)
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    result,
		Meta: &Meta{
			Total: len(result),
		},
	})
}

// TopSensor represents a sensor with its average value
type TopSensor struct {
	SensorID   string  `json:"sensor_id"`
	SensorName string  `json:"sensor_name"`
	SensorType string  `json:"sensor_type"`
	AvgValue   float64 `json:"avg_value"`
	Unit       string  `json:"unit"`
	Location   struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"location"`
}

// GetTopPolluted handles GET /api/v1/analytics/top-polluted
func (h *AnalyticsHandler) GetTopPolluted(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	limit := c.QueryInt("limit", 10)
	if limit < 1 || limit > 50 {
		limit = 10
	}

	// Try cache first
	cacheKey := fmt.Sprintf("analytics:top-polluted:%d", limit)
	cached, err := h.redisClient.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var topSensors []TopSensor
		if err := json.Unmarshal([]byte(cached), &topSensors); err == nil {
			log.Debug().Msg("Returning cached top polluted")
			return c.JSON(APIResponse{
				Success: true,
				Data:    topSensors,
			})
		}
	}

	// Get all pollution sensors
	sensors, err := h.db.GetSensorsByType(ctx, models.SensorTypePollution)
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

	// Get latest values and calculate average
	topSensors := make([]TopSensor, 0)
	for _, sensor := range sensors {
		key := fmt.Sprintf("sensor:latest:%s", sensor.ID.String())
		data, err := h.redisClient.HGetAll(ctx, key).Result()
		if err != nil || len(data) == 0 {
			continue
		}

		var value float64
		if v, ok := data["value"]; ok {
			fmt.Sscanf(v, "%f", &value)
		}

		topSensor := TopSensor{
			SensorID:   sensor.ID.String(),
			SensorName: sensor.Name,
			SensorType: string(sensor.Type),
			AvgValue:   value,
			Unit:       "µg/m³",
		}
		topSensor.Location.Latitude = sensor.Latitude
		topSensor.Location.Longitude = sensor.Longitude

		topSensors = append(topSensors, topSensor)
	}

	// Sort by value (descending)
	for i := 0; i < len(topSensors)-1; i++ {
		for j := i + 1; j < len(topSensors); j++ {
			if topSensors[j].AvgValue > topSensors[i].AvgValue {
				topSensors[i], topSensors[j] = topSensors[j], topSensors[i]
			}
		}
	}

	// Limit results
	if len(topSensors) > limit {
		topSensors = topSensors[:limit]
	}

	// Cache for 10 minutes
	sensorsJSON, _ := json.Marshal(topSensors)
	h.redisClient.Set(ctx, cacheKey, sensorsJSON, 10*time.Minute)

	return c.JSON(APIResponse{
		Success: true,
		Data:    topSensors,
		Meta: &Meta{
			Total: len(topSensors),
		},
	})
}

// GetTopTemperature handles GET /api/v1/analytics/top-temperature
func (h *AnalyticsHandler) GetTopTemperature(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	limit := c.QueryInt("limit", 10)
	if limit < 1 || limit > 50 {
		limit = 10
	}

	// Try cache first
	cacheKey := fmt.Sprintf("analytics:top-temperature:%d", limit)
	cached, err := h.redisClient.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var topSensors []TopSensor
		if err := json.Unmarshal([]byte(cached), &topSensors); err == nil {
			log.Debug().Msg("Returning cached top temperature")
			return c.JSON(APIResponse{
				Success: true,
				Data:    topSensors,
			})
		}
	}

	// Get all temperature sensors
	sensors, err := h.db.GetSensorsByType(ctx, models.SensorTypeTemperature)
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

	// Get latest values
	topSensors := make([]TopSensor, 0)
	for _, sensor := range sensors {
		key := fmt.Sprintf("sensor:latest:%s", sensor.ID.String())
		data, err := h.redisClient.HGetAll(ctx, key).Result()
		if err != nil || len(data) == 0 {
			continue
		}

		var value float64
		if v, ok := data["value"]; ok {
			fmt.Sscanf(v, "%f", &value)
		}

		topSensor := TopSensor{
			SensorID:   sensor.ID.String(),
			SensorName: sensor.Name,
			SensorType: string(sensor.Type),
			AvgValue:   value,
			Unit:       "celsius",
		}
		topSensor.Location.Latitude = sensor.Latitude
		topSensor.Location.Longitude = sensor.Longitude

		topSensors = append(topSensors, topSensor)
	}

	// Sort by value (descending)
	for i := 0; i < len(topSensors)-1; i++ {
		for j := i + 1; j < len(topSensors); j++ {
			if topSensors[j].AvgValue > topSensors[i].AvgValue {
				topSensors[i], topSensors[j] = topSensors[j], topSensors[i]
			}
		}
	}

	// Limit results
	if len(topSensors) > limit {
		topSensors = topSensors[:limit]
	}

	// Cache for 10 minutes
	sensorsJSON, _ := json.Marshal(topSensors)
	h.redisClient.Set(ctx, cacheKey, sensorsJSON, 10*time.Minute)

	return c.JSON(APIResponse{
		Success: true,
		Data:    topSensors,
		Meta: &Meta{
			Total: len(topSensors),
		},
	})
}

// GetQuietest handles GET /api/v1/analytics/quietest
func (h *AnalyticsHandler) GetQuietest(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	limit := c.QueryInt("limit", 10)
	if limit < 1 || limit > 50 {
		limit = 10
	}

	// Try cache first
	cacheKey := fmt.Sprintf("analytics:quietest:%d", limit)
	cached, err := h.redisClient.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var topSensors []TopSensor
		if err := json.Unmarshal([]byte(cached), &topSensors); err == nil {
			log.Debug().Msg("Returning cached quietest")
			return c.JSON(APIResponse{
				Success: true,
				Data:    topSensors,
			})
		}
	}

	// Get all noise sensors
	sensors, err := h.db.GetSensorsByType(ctx, models.SensorTypeNoise)
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

	// Get latest values
	quietestSensors := make([]TopSensor, 0)
	for _, sensor := range sensors {
		key := fmt.Sprintf("sensor:latest:%s", sensor.ID.String())
		data, err := h.redisClient.HGetAll(ctx, key).Result()
		if err != nil || len(data) == 0 {
			continue
		}

		var value float64
		if v, ok := data["value"]; ok {
			fmt.Sscanf(v, "%f", &value)
		}

		topSensor := TopSensor{
			SensorID:   sensor.ID.String(),
			SensorName: sensor.Name,
			SensorType: string(sensor.Type),
			AvgValue:   value,
			Unit:       "dB",
		}
		topSensor.Location.Latitude = sensor.Latitude
		topSensor.Location.Longitude = sensor.Longitude

		quietestSensors = append(quietestSensors, topSensor)
	}

	// Sort by value (ascending for quietest)
	for i := 0; i < len(quietestSensors)-1; i++ {
		for j := i + 1; j < len(quietestSensors); j++ {
			if quietestSensors[j].AvgValue < quietestSensors[i].AvgValue {
				quietestSensors[i], quietestSensors[j] = quietestSensors[j], quietestSensors[i]
			}
		}
	}

	// Limit results
	if len(quietestSensors) > limit {
		quietestSensors = quietestSensors[:limit]
	}

	// Cache for 10 minutes
	sensorsJSON, _ := json.Marshal(quietestSensors)
	h.redisClient.Set(ctx, cacheKey, sensorsJSON, 10*time.Minute)

	return c.JSON(APIResponse{
		Success: true,
		Data:    quietestSensors,
		Meta: &Meta{
			Total: len(quietestSensors),
		},
	})
}
