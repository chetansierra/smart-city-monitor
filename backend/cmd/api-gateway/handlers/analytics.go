package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/chetansierra/smart-city-monitor/internal/zones"
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

	// Try pre-computed cache from aggregation worker first
	computedKey := "analytics:city-stats:computed"
	computedCached, err := h.redisClient.Get(ctx, computedKey).Result()
	if err == nil && computedCached != "" {
		var stats CityStats
		if err := json.Unmarshal([]byte(computedCached), &stats); err == nil {
			log.Debug().Msg("Returning pre-computed city stats")
			return c.JSON(APIResponse{
				Success: true,
				Data:    stats,
			})
		}
	}

	// Fall back to existing cache
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

	// Use pre-computed hourly aggregates (sensor_readings no longer persisted)
	aggregates, err := h.db.GetHourlyAggregates(ctx, sensorID, from, to)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch hourly aggregates")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to fetch hourly aggregates",
			},
		})
	}

	result := make([]SensorStats, 0, len(aggregates))
	for _, a := range aggregates {
		result = append(result, SensorStats{
			SensorID:    sensorID.String(),
			SensorType:  string(a.SensorType),
			AvgValue:    a.AvgValue,
			MinValue:    a.MinValue,
			MaxValue:    a.MaxValue,
			Count:       1,
			PeriodStart: a.PeriodStart,
			PeriodEnd:   a.PeriodEnd,
		})
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

	// Sort by value (descending) - O(n log n)
	sort.Slice(topSensors, func(i, j int) bool {
		return topSensors[i].AvgValue > topSensors[j].AvgValue
	})

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

	// Sort by value (descending) - O(n log n)
	sort.Slice(topSensors, func(i, j int) bool {
		return topSensors[i].AvgValue > topSensors[j].AvgValue
	})

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

	// Sort by value (ascending for quietest) - O(n log n)
	sort.Slice(quietestSensors, func(i, j int) bool {
		return quietestSensors[i].AvgValue < quietestSensors[j].AvgValue
	})

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

// HourlyAggregation represents hourly aggregated data for all sensors
type HourlyAggregation struct {
	Hour           time.Time          `json:"hour"`
	SensorType     string             `json:"sensor_type"`
	AvgValue       float64            `json:"avg_value"`
	MinValue       float64            `json:"min_value"`
	MaxValue       float64            `json:"max_value"`
	Count          int                `json:"count"`
}

// GetHourlyAggregations handles GET /api/v1/analytics/hourly
func (h *AnalyticsHandler) GetHourlyAggregations(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Parse time range
	fromStr := c.Query("from", "")
	toStr := c.Query("to", "")
	sensorType := c.Query("sensor_type", "")

	var from, to time.Time
	var err error

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

	// Use pre-computed aggregates only (sensor_readings no longer persisted)
	aggregates, err := h.db.GetHourlyAggregatesForAll(ctx, from, to, sensorType)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch hourly aggregates")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to fetch hourly aggregates",
			},
		})
	}

	result := make([]HourlyAggregation, 0, len(aggregates))
	for _, a := range aggregates {
		result = append(result, HourlyAggregation{
			Hour:       a.PeriodStart,
			SensorType: string(a.SensorType),
			AvgValue:   a.AvgValue,
			MinValue:   a.MinValue,
			MaxValue:   a.MaxValue,
			Count:      1,
		})
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    result,
		Meta:    &Meta{Total: len(result)},
	})
}

// ComparisonData represents data for time-series comparison
type ComparisonData struct {
	SensorID   string                 `json:"sensor_id"`
	SensorName string                 `json:"sensor_name"`
	SensorType string                 `json:"sensor_type"`
	Periods    map[string]PeriodStats `json:"periods"`
}

// PeriodStats represents statistics for a time period
type PeriodStats struct {
	PeriodName string      `json:"period_name"`
	AvgValue   float64     `json:"avg_value"`
	MinValue   float64     `json:"min_value"`
	MaxValue   float64     `json:"max_value"`
	Count      int         `json:"count"`
	Timestamps []time.Time `json:"timestamps"`
	Values     []float64   `json:"values"`
}

// GetComparisonData handles GET /api/v1/analytics/compare
func (h *AnalyticsHandler) GetComparisonData(c *fiber.Ctx) error {
	_, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Parse sensor IDs
	sensorIDs := c.Query("sensor_ids", "")
	if sensorIDs == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "MISSING_SENSOR_IDS",
				Message: "sensor_ids query parameter is required",
			},
		})
	}

	// Parse periods (e.g., "today,yesterday,last_week")
	_ = c.Query("periods", "today,yesterday")

	// TODO: Implement comparison logic
	// This is a placeholder response for now
	log.Debug().Str("sensor_ids", sensorIDs).Msg("Comparison data requested")

	return c.JSON(APIResponse{
		Success: true,
		Data:    []ComparisonData{},
		Meta: &Meta{
			Total: 0,
		},
	})
}

// ZoneStats represents statistics for a geographic zone
type ZoneStats struct {
	ZoneName    string       `json:"zone_name"`
	SensorType  string       `json:"sensor_type"`
	AvgValue    float64      `json:"avg_value"`
	MinValue    float64      `json:"min_value"`
	MaxValue    float64      `json:"max_value"`
	SensorCount int          `json:"sensor_count"`
	TopSensors  []TopSensor  `json:"top_sensors"`
	Coordinates []Coordinate `json:"coordinates"`
}

// Coordinate represents a lat/lng point
type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Value     float64 `json:"value"`
}

// GetZoneAnalytics handles GET /api/v1/analytics/zones
func (h *AnalyticsHandler) GetZoneAnalytics(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sensorType := c.Query("sensor_type", "pollution")

	// Get all sensors of the specified type
	sensors, err := h.db.GetSensorsByType(ctx, models.SensorType(sensorType))
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

	zoneStats := make(map[string]*ZoneStats)

	for zoneName, bounds := range zones.Zones {
		zoneStats[zoneName] = &ZoneStats{
			ZoneName:    zoneName,
			SensorType:  sensorType,
			TopSensors:  []TopSensor{},
			Coordinates: []Coordinate{},
		}

		var totalValue float64
		var count int
		minVal := 999999.0
		maxVal := -999999.0

		for _, sensor := range sensors {
			// Check if sensor is in zone
			if sensor.Latitude >= bounds.MinLat && sensor.Latitude <= bounds.MaxLat &&
				sensor.Longitude >= bounds.MinLng && sensor.Longitude <= bounds.MaxLng {

				// Get latest value
				key := fmt.Sprintf("sensor:latest:%s", sensor.ID.String())
				data, err := h.redisClient.HGetAll(ctx, key).Result()
				if err != nil || len(data) == 0 {
					continue
				}

				var value float64
				if v, ok := data["value"]; ok {
					fmt.Sscanf(v, "%f", &value)
				}

				totalValue += value
				count++

				if value < minVal {
					minVal = value
				}
				if value > maxVal {
					maxVal = value
				}

				zoneStats[zoneName].Coordinates = append(zoneStats[zoneName].Coordinates, Coordinate{
					Latitude:  sensor.Latitude,
					Longitude: sensor.Longitude,
					Value:     value,
				})

				zoneStats[zoneName].TopSensors = append(zoneStats[zoneName].TopSensors, TopSensor{
					SensorID:   sensor.ID.String(),
					SensorName: sensor.Name,
					SensorType: string(sensor.Type),
					AvgValue:   value,
				})
			}
		}

		if count > 0 {
			zoneStats[zoneName].AvgValue = totalValue / float64(count)
			zoneStats[zoneName].MinValue = minVal
			zoneStats[zoneName].MaxValue = maxVal
			zoneStats[zoneName].SensorCount = count
		}
	}

	// Convert map to slice
	result := make([]ZoneStats, 0, len(zoneStats))
	for _, stats := range zoneStats {
		result = append(result, *stats)
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    result,
		Meta: &Meta{
			Total: len(result),
		},
	})
}

// GetAnomalies handles GET /api/v1/analytics/anomalies
func (h *AnalyticsHandler) GetAnomalies(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	limit := c.QueryInt("limit", 50)
	if limit < 1 || limit > 200 {
		limit = 50
	}

	sinceStr := c.Query("since", "")
	since := time.Now().Add(-1 * time.Hour)
	if sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	anomalies, err := h.db.GetRecentAnomalies(ctx, limit, since)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch anomalies")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to fetch anomalies",
			},
		})
	}

	if anomalies == nil {
		anomalies = []models.AnomalyEvent{}
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    anomalies,
		Meta: &Meta{
			Total: len(anomalies),
		},
	})
}

// GetDetectedEvents handles GET /api/v1/analytics/events
func (h *AnalyticsHandler) GetDetectedEvents(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	limit := c.QueryInt("limit", 20)
	if limit < 1 || limit > 100 {
		limit = 20
	}

	sinceStr := c.Query("since", "")
	since := time.Now().Add(-1 * time.Hour)
	if sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	events, err := h.db.GetRecentDetectedEvents(ctx, limit, since)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch detected events")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to fetch detected events",
			},
		})
	}

	if events == nil {
		events = []models.DetectedEvent{}
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    events,
		Meta: &Meta{
			Total: len(events),
		},
	})
}
