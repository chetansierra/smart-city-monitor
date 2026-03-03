package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/chetansierra/smart-city-monitor/internal/scenario"
	"github.com/chetansierra/smart-city-monitor/internal/simulation"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// AdminHandler handles admin control endpoints
type AdminHandler struct {
	db            *postgres.DB
	redisClient   *redis.Client
	kafkaProducer sarama.SyncProducer
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(db *postgres.DB, redisClient *redis.Client, kafkaProducer sarama.SyncProducer) *AdminHandler {
	return &AdminHandler{
		db:            db,
		redisClient:   redisClient,
		kafkaProducer: kafkaProducer,
	}
}

// SensorControlRequest represents a sensor control request
type SensorControlRequest struct {
	SensorIDs []string `json:"sensor_ids"`
	Action    string   `json:"action"` // currently supports only "start"
}

// SimulationRateRequest represents a simulation rate change request
type SimulationRateRequest struct {
	ReadingsPerSecond int  `json:"readings_per_second"`
	BurstMode         bool `json:"burst_mode"`
	BatchSize         int  `json:"batch_size"`
}

// getSessionID extracts the session ID from the X-Session-ID header
func getSessionID(c *fiber.Ctx) (*uuid.UUID, error) {
	sessionIDStr := c.Get("X-Session-ID")
	if sessionIDStr == "" {
		return nil, nil // No session ID is allowed for backward compatibility or global ops
	}

	id, err := uuid.Parse(sessionIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid session id: %w", err)
	}
	return &id, nil
}

// ControlSensors handles POST /api/admin/sensors/control
func (h *AdminHandler) ControlSensors(c *fiber.Ctx) error {
	var req SensorControlRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
			},
		})
	}

	sessionID, err := getSessionID(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_SESSION",
				Message: err.Error(),
			},
		})
	}

	if req.Action != "start" {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_ACTION",
				Message: "Action must be 'start'. Sensors are active while they exist.",
			},
		})
	}

	if len(req.SensorIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_REQUEST",
				Message: "sensor_ids must contain at least one sensor ID",
			},
		})
	}

	// Send control commands to Kafka
	successfulIDs := make([]uuid.UUID, 0, len(req.SensorIDs))
	for _, sensorID := range req.SensorIDs {
		command := map[string]interface{}{
			"sensor_id":  sensorID,
			"session_id": sessionID,
			"action":     req.Action,
			"timestamp":  time.Now().Unix(),
		}

		commandBytes, err := json.Marshal(command)
		if err != nil {
			log.Error().Err(err).Str("sensor_id", sensorID).Msg("Failed to marshal command")
			continue
		}

		message := &sarama.ProducerMessage{
			Topic: "sensor-control",
			Key:   sarama.StringEncoder(sensorID),
			Value: sarama.ByteEncoder(commandBytes),
		}

		if _, _, err := h.kafkaProducer.SendMessage(message); err != nil {
			log.Error().Err(err).Str("sensor_id", sensorID).Msg("Failed to send control message")
			continue
		}

		parsedID, err := uuid.Parse(sensorID)
		if err != nil {
			log.Warn().Str("sensor_id", sensorID).Msg("Skipping status update for invalid sensor ID")
			continue
		}
		successfulIDs = append(successfulIDs, parsedID)
	}

	if len(successfulIDs) == 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "KAFKA_ERROR",
				Message: "Failed to send control command to sensors",
			},
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.db.UpdateSensorsStatus(ctx, successfulIDs, models.SensorStatusActive); err != nil {
		log.Error().
			Err(err).
			Str("action", req.Action).
			Int("successful_sensors", len(successfulIDs)).
			Msg("Failed to persist sensor status changes")
	}

	log.Info().
		Str("action", req.Action).
		Int("sensor_count", len(successfulIDs)).
		Msg("Sensor control command sent")

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"action":        req.Action,
			"sensors_count": len(successfulIDs),
			"message":       fmt.Sprintf("Successfully sent %s command to %d sensors", req.Action, len(successfulIDs)),
		},
	})
}

// StartAllSensors handles POST /api/admin/sensors/start-all
func (h *AdminHandler) StartAllSensors(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get all sensors
	sessionID, err := getSessionID(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_SESSION",
				Message: err.Error(),
			},
		})
	}

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

	// Send start command to all sensors
	successCount := 0
	successfulIDs := make([]uuid.UUID, 0, len(sensors))
	for _, sensor := range sensors {
		command := map[string]interface{}{
			"sensor_id":  sensor.ID.String(),
			"session_id": sessionID,
			"action":     "start",
			"timestamp":  time.Now().Unix(),
		}

		commandBytes, _ := json.Marshal(command)
		message := &sarama.ProducerMessage{
			Topic: "sensor-control",
			Key:   sarama.StringEncoder(sensor.ID.String()),
			Value: sarama.ByteEncoder(commandBytes),
		}

		if _, _, err := h.kafkaProducer.SendMessage(message); err != nil {
			log.Error().Err(err).Str("sensor_id", sensor.ID.String()).Msg("Failed to send start command")
			continue
		}
		successCount++
		successfulIDs = append(successfulIDs, sensor.ID)
	}

	if len(successfulIDs) > 0 {
		if err := h.db.UpdateSensorsStatus(ctx, successfulIDs, models.SensorStatusActive); err != nil {
			log.Error().Err(err).Int("sensor_count", len(successfulIDs)).Msg("Failed to persist start-all sensor statuses")
		}
	}

	log.Info().Int("sensor_count", successCount).Msg("Start-all command sent")

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message":       "All sensors started",
			"sensors_count": successCount,
		},
	})
}

// StopAllSensors handles POST /api/admin/sensors/stop-all
func (h *AdminHandler) StopAllSensors(c *fiber.Ctx) error {
	return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "UNSUPPORTED_OPERATION",
			Message: "Stopping sensors is no longer supported. Delete sensors instead.",
		},
	})
}

// SetSimulationRate handles POST /api/admin/simulation/rate
func (h *AdminHandler) SetSimulationRate(c *fiber.Ctx) error {
	var req SimulationRateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
			},
		})
	}

	// Validate rate
	if req.ReadingsPerSecond < 1 || req.ReadingsPerSecond > 1000 {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_RATE",
				Message: "Readings per second must be between 1 and 1000",
			},
		})
	}

	if req.BatchSize < 1 || req.BatchSize > 100 {
		req.BatchSize = 10 // Default
	}

	// Send rate control command to Kafka
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	command := map[string]interface{}{
		"type":                "rate_control",
		"readings_per_second": req.ReadingsPerSecond,
		"burst_mode":          req.BurstMode,
		"batch_size":          req.BatchSize,
		"timestamp":           time.Now().Unix(),
	}

	commandBytes, _ := json.Marshal(command)
	message := &sarama.ProducerMessage{
		Topic: "sensor-control",
		Key:   sarama.StringEncoder("rate_control"),
		Value: sarama.ByteEncoder(commandBytes),
	}

	if _, _, err := h.kafkaProducer.SendMessage(message); err != nil {
		log.Error().Err(err).Msg("Failed to send rate control message")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "KAFKA_ERROR",
				Message: "Failed to update simulation rate",
			},
		})
	}

	// Store current rate in Redis for status queries
	rateKey := "simulation:rate"
	rateData := map[string]interface{}{
		"readings_per_second": req.ReadingsPerSecond,
		"burst_mode":          req.BurstMode,
		"batch_size":          req.BatchSize,
		"updated_at":          time.Now().Unix(),
	}
	h.redisClient.HSet(ctx, rateKey, rateData)

	log.Info().
		Int("readings_per_second", req.ReadingsPerSecond).
		Bool("burst_mode", req.BurstMode).
		Int("batch_size", req.BatchSize).
		Msg("Simulation rate updated")

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message":             "Simulation rate updated",
			"readings_per_second": req.ReadingsPerSecond,
			"burst_mode":          req.BurstMode,
			"batch_size":          req.BatchSize,
		},
	})
}

// ClearRedisCache handles POST /api/admin/system/clear-cache
func (h *AdminHandler) ClearRedisCache(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get all keys matching sensor:latest:*
	pattern := "sensor:latest:*"
	keys, err := h.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Redis keys")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "REDIS_ERROR",
				Message: "Failed to get cache keys",
			},
		})
	}

	// Delete keys if any exist
	deletedCount := 0
	if len(keys) > 0 {
		result, err := h.redisClient.Del(ctx, keys...).Result()
		if err != nil {
			log.Error().Err(err).Msg("Failed to delete Redis keys")
			return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
				Success: false,
				Error: &APIError{
					Code:    "REDIS_ERROR",
					Message: "Failed to clear cache",
				},
			})
		}
		deletedCount = int(result)
	}

	log.Info().Int("keys_deleted", deletedCount).Msg("Redis cache cleared")

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message":      "Cache cleared successfully",
			"keys_deleted": deletedCount,
		},
	})
}

// ResetSimulation handles POST /api/admin/system/reset
func (h *AdminHandler) ResetSimulation(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send reset command to Kafka
	command := map[string]interface{}{
		"type":      "reset",
		"timestamp": time.Now().Unix(),
	}

	commandBytes, _ := json.Marshal(command)
	message := &sarama.ProducerMessage{
		Topic: "sensor-control",
		Key:   sarama.StringEncoder("reset"),
		Value: sarama.ByteEncoder(commandBytes),
	}

	if _, _, err := h.kafkaProducer.SendMessage(message); err != nil {
		log.Error().Err(err).Msg("Failed to send reset message")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "KAFKA_ERROR",
				Message: "Failed to send reset command",
			},
		})
	}

	// Clear Redis cache
	pattern := "sensor:latest:*"
	keys, _ := h.redisClient.Keys(ctx, pattern).Result()
	if len(keys) > 0 {
		h.redisClient.Del(ctx, keys...)
	}

	log.Info().Msg("Simulation reset initiated")

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message": "Simulation reset successfully",
		},
	})
}

// GetSimulationStatus handles GET /api/admin/simulation/status
func (h *AdminHandler) GetSimulationStatus(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get current rate from Redis
	rateKey := "simulation:rate"
	rateData, err := h.redisClient.HGetAll(ctx, rateKey).Result()

	status := map[string]interface{}{
		"active": true,
	}

	if err == nil && len(rateData) > 0 {
		status["readings_per_second"] = rateData["readings_per_second"]
		status["burst_mode"] = rateData["burst_mode"]
		status["batch_size"] = rateData["batch_size"]
		status["updated_at"] = rateData["updated_at"]
	} else {
		// Default values
		status["readings_per_second"] = "10"
		status["burst_mode"] = "false"
		status["batch_size"] = "10"
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    status,
	})
}

// ScenarioRequest represents a scenario activation request
type ScenarioRequest struct {
	ScenarioType string `json:"scenario_type"`      // "normal", "rush_hour", "heatwave", "industrial_incident"
	Duration     int    `json:"duration,omitempty"` // Duration in minutes (optional override)
}

// GetScenarios handles GET /api/admin/scenarios
func (h *AdminHandler) GetScenarios(c *fiber.Ctx) error {
	scenarios := scenario.GetAllScenarios()

	// Get current active scenario from Redis
	ctx := context.Background()
	currentScenarioJSON, err := h.redisClient.Get(ctx, "scenario:active").Result()

	scenarioList := []map[string]interface{}{}
	for _, s := range scenarios {
		scenarioMap := map[string]interface{}{
			"type":        s.Type,
			"name":        s.Name,
			"description": s.Description,
			"duration":    s.Duration.Minutes(),
			"active":      false,
		}

		// Check if this is the active scenario
		if err == nil && currentScenarioJSON != "" {
			var activeScenario scenario.Scenario
			if json.Unmarshal([]byte(currentScenarioJSON), &activeScenario) == nil {
				if activeScenario.Type == s.Type {
					scenarioMap["active"] = true
					scenarioMap["started_at"] = activeScenario.StartedAt
				}
			}
		}

		scenarioList = append(scenarioList, scenarioMap)
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    scenarioList,
	})
}

// ActivateScenario handles POST /api/admin/scenarios/activate
func (h *AdminHandler) ActivateScenario(c *fiber.Ctx) error {
	var req ScenarioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
			},
		})
	}

	// Validate scenario type
	scenarios := scenario.GetAllScenarios()
	selectedScenario, exists := scenarios[scenario.ScenarioType(req.ScenarioType)]
	if !exists {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_SCENARIO",
				Message: fmt.Sprintf("Invalid scenario type: %s", req.ScenarioType),
			},
		})
	}

	// Override duration if provided
	if req.Duration > 0 {
		selectedScenario.Duration = time.Duration(req.Duration) * time.Minute
	}

	// Mark as active and set start time
	selectedScenario.Active = true
	selectedScenario.StartedAt = time.Now()

	// Store in Redis
	ctx := context.Background()
	scenarioJSON, err := selectedScenario.ToJSON()
	if err != nil {
		log.Error().Err(err).Msg("Failed to serialize scenario")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "SERIALIZATION_ERROR",
				Message: "Failed to activate scenario",
			},
		})
	}

	// Store with expiration if duration is set
	if selectedScenario.Duration > 0 {
		err = h.redisClient.Set(ctx, "scenario:active", scenarioJSON, selectedScenario.Duration).Err()
	} else {
		err = h.redisClient.Set(ctx, "scenario:active", scenarioJSON, 0).Err() // No expiration for normal scenario
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to store scenario in Redis")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "REDIS_ERROR",
				Message: "Failed to activate scenario",
			},
		})
	}

	// Send scenario activation command to Kafka
	command := map[string]interface{}{
		"type":      "scenario_activate",
		"scenario":  req.ScenarioType,
		"duration":  selectedScenario.Duration.Minutes(),
		"timestamp": time.Now().Unix(),
	}

	commandBytes, _ := json.Marshal(command)
	message := &sarama.ProducerMessage{
		Topic: "scenario-control",
		Key:   sarama.StringEncoder(req.ScenarioType),
		Value: sarama.ByteEncoder(commandBytes),
	}

	_, _, err = h.kafkaProducer.SendMessage(message)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send scenario command to Kafka")
	}

	log.Info().
		Str("scenario", req.ScenarioType).
		Float64("duration_minutes", selectedScenario.Duration.Minutes()).
		Msg("Scenario activated")

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message":    fmt.Sprintf("Scenario '%s' activated successfully", selectedScenario.Name),
			"scenario":   selectedScenario,
			"started_at": selectedScenario.StartedAt,
		},
	})
}

// GetCurrentScenario handles GET /api/admin/scenarios/current
func (h *AdminHandler) GetCurrentScenario(c *fiber.Ctx) error {
	ctx := context.Background()
	scenarioJSON, err := h.redisClient.Get(ctx, "scenario:active").Result()

	if err != nil || scenarioJSON == "" {
		// No active scenario, return normal
		normalScenario := scenario.GetNormalScenario()
		return c.JSON(APIResponse{
			Success: true,
			Data:    normalScenario,
		})
	}

	currentScenario, err := scenario.FromJSON(scenarioJSON)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse scenario from Redis")
		normalScenario := scenario.GetNormalScenario()
		return c.JSON(APIResponse{
			Success: true,
			Data:    normalScenario,
		})
	}

	// Check if scenario has expired
	if currentScenario.Duration > 0 {
		elapsed := time.Since(currentScenario.StartedAt)
		if elapsed > currentScenario.Duration {
			// Scenario expired, return to normal
			h.redisClient.Del(ctx, "scenario:active")
			normalScenario := scenario.GetNormalScenario()
			return c.JSON(APIResponse{
				Success: true,
				Data:    normalScenario,
			})
		}

		// Add remaining time info
		remainingMinutes := (currentScenario.Duration - elapsed).Minutes()
		return c.JSON(APIResponse{
			Success: true,
			Data: map[string]interface{}{
				"scenario":          currentScenario,
				"remaining_minutes": remainingMinutes,
				"progress":          elapsed.Seconds() / currentScenario.Duration.Seconds(),
			},
		})
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    currentScenario,
	})
}

// GetSimulationConfig handles GET /api/admin/simulation/config
func (h *AdminHandler) GetSimulationConfig(c *fiber.Ctx) error {
	ctx := context.Background()
	configJSON, err := h.redisClient.Get(ctx, "simulation:config").Result()

	if err != nil || configJSON == "" {
		// Return default config
		defaultConfig := simulation.NewSimulationConfig()
		return c.JSON(APIResponse{
			Success: true,
			Data:    defaultConfig,
		})
	}

	config, err := simulation.FromJSON(configJSON)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse simulation config from Redis")
		defaultConfig := simulation.NewSimulationConfig()
		return c.JSON(APIResponse{
			Success: true,
			Data:    defaultConfig,
		})
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    config,
	})
}

// SetThresholdRequest represents a threshold override request
type SetThresholdRequest struct {
	SensorID      string  `json:"sensor_id"`
	WarningMin    float64 `json:"warning_min"`
	WarningMax    float64 `json:"warning_max"`
	CriticalMin   float64 `json:"critical_min"`
	CriticalMax   float64 `json:"critical_max"`
	CustomMessage string  `json:"custom_message,omitempty"`
}

// SetSensorThreshold handles POST /api/admin/simulation/threshold
func (h *AdminHandler) SetSensorThreshold(c *fiber.Ctx) error {
	var req SetThresholdRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
			},
		})
	}

	// Load existing config
	ctx := context.Background()
	configJSON, _ := h.redisClient.Get(ctx, "simulation:config").Result()

	var config *simulation.SimulationConfig
	if configJSON != "" {
		config, _ = simulation.FromJSON(configJSON)
	}
	if config == nil {
		config = simulation.NewSimulationConfig()
	}

	// Apply threshold
	threshold := &simulation.ThresholdConfig{
		WarningMin:    req.WarningMin,
		WarningMax:    req.WarningMax,
		CriticalMin:   req.CriticalMin,
		CriticalMax:   req.CriticalMax,
		Enabled:       true,
		CustomMessage: req.CustomMessage,
	}
	config.ApplyThreshold(req.SensorID, threshold)

	// Save to Redis
	updatedJSON, _ := config.ToJSON()
	h.redisClient.Set(ctx, "simulation:config", updatedJSON, 0)

	// Send update to Kafka
	command := map[string]interface{}{
		"type":      "threshold_update",
		"sensor_id": req.SensorID,
		"threshold": threshold,
		"timestamp": time.Now().Unix(),
	}
	commandBytes, _ := json.Marshal(command)
	message := &sarama.ProducerMessage{
		Topic: "simulation-control",
		Key:   sarama.StringEncoder(req.SensorID),
		Value: sarama.ByteEncoder(commandBytes),
	}
	h.kafkaProducer.SendMessage(message)

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message":   "Threshold updated successfully",
			"sensor_id": req.SensorID,
			"threshold": threshold,
		},
	})
}

// SetSensorBehaviorRequest represents a sensor behavior customization request
type SetSensorBehaviorRequest struct {
	SensorID   string  `json:"sensor_id"`
	Pattern    string  `json:"pattern"`
	MinValue   float64 `json:"min_value"`
	MaxValue   float64 `json:"max_value"`
	Variance   float64 `json:"variance"`
	UpdateRate int     `json:"update_rate"`
}

// SetSensorBehavior handles POST /api/admin/simulation/behavior
func (h *AdminHandler) SetSensorBehavior(c *fiber.Ctx) error {
	var req SetSensorBehaviorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
			},
		})
	}

	// Load existing config
	ctx := context.Background()
	configJSON, _ := h.redisClient.Get(ctx, "simulation:config").Result()

	var config *simulation.SimulationConfig
	if configJSON != "" {
		config, _ = simulation.FromJSON(configJSON)
	}
	if config == nil {
		config = simulation.NewSimulationConfig()
	}

	// Apply behavior
	behavior := &simulation.SensorBehavior{
		Pattern:    simulation.SensorBehaviorPattern(req.Pattern),
		MinValue:   req.MinValue,
		MaxValue:   req.MaxValue,
		Variance:   req.Variance,
		UpdateRate: req.UpdateRate,
		Enabled:    true,
	}
	config.ApplySensorBehavior(req.SensorID, behavior)

	// Save to Redis
	updatedJSON, _ := config.ToJSON()
	h.redisClient.Set(ctx, "simulation:config", updatedJSON, 0)

	// Send update to Kafka
	command := map[string]interface{}{
		"type":      "behavior_update",
		"sensor_id": req.SensorID,
		"behavior":  behavior,
		"timestamp": time.Now().Unix(),
	}
	commandBytes, _ := json.Marshal(command)
	message := &sarama.ProducerMessage{
		Topic: "simulation-control",
		Key:   sarama.StringEncoder(req.SensorID),
		Value: sarama.ByteEncoder(commandBytes),
	}
	h.kafkaProducer.SendMessage(message)

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message":   "Sensor behavior updated successfully",
			"sensor_id": req.SensorID,
			"behavior":  behavior,
		},
	})
}

// SetTimeCompressionRequest represents a time compression request
type SetTimeCompressionRequest struct {
	Multiplier float64 `json:"multiplier"` // 1.0 = normal, 2.0 = 2x, etc.
}

// SetTimeCompression handles POST /api/admin/simulation/time-compression
func (h *AdminHandler) SetTimeCompression(c *fiber.Ctx) error {
	var req SetTimeCompressionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
			},
		})
	}

	// Validate multiplier
	if req.Multiplier < 0.1 || req.Multiplier > 10.0 {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_MULTIPLIER",
				Message: "Multiplier must be between 0.1 and 10.0",
			},
		})
	}

	// Load existing config
	ctx := context.Background()
	configJSON, _ := h.redisClient.Get(ctx, "simulation:config").Result()

	var config *simulation.SimulationConfig
	if configJSON != "" {
		config, _ = simulation.FromJSON(configJSON)
	}
	if config == nil {
		config = simulation.NewSimulationConfig()
	}

	// Apply time compression
	config.SetTimeCompression(req.Multiplier)

	// Save to Redis
	updatedJSON, _ := config.ToJSON()
	h.redisClient.Set(ctx, "simulation:config", updatedJSON, 0)

	// Send update to Kafka
	command := map[string]interface{}{
		"type":       "time_compression",
		"multiplier": req.Multiplier,
		"timestamp":  time.Now().Unix(),
	}
	commandBytes, _ := json.Marshal(command)
	message := &sarama.ProducerMessage{
		Topic: "simulation-control",
		Value: sarama.ByteEncoder(commandBytes),
	}
	h.kafkaProducer.SendMessage(message)

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message":     "Time compression updated successfully",
			"multiplier":  req.Multiplier,
			"compression": config.TimeCompression,
		},
	})
}

// SetChaosMode handles POST /api/admin/simulation/chaos-mode
func (h *AdminHandler) SetChaosMode(c *fiber.Ctx) error {
	var chaosConfig simulation.ChaosConfig
	if err := c.BodyParser(&chaosConfig); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
			},
		})
	}

	// Load existing config
	ctx := context.Background()
	configJSON, _ := h.redisClient.Get(ctx, "simulation:config").Result()

	var config *simulation.SimulationConfig
	if configJSON != "" {
		config, _ = simulation.FromJSON(configJSON)
	}
	if config == nil {
		config = simulation.NewSimulationConfig()
	}

	// Apply chaos mode
	config.SetChaosMode(&chaosConfig)

	// Save to Redis
	updatedJSON, _ := config.ToJSON()
	h.redisClient.Set(ctx, "simulation:config", updatedJSON, 0)

	// Send update to Kafka
	command := map[string]interface{}{
		"type":         "chaos_mode",
		"chaos_config": chaosConfig,
		"timestamp":    time.Now().Unix(),
	}
	commandBytes, _ := json.Marshal(command)
	message := &sarama.ProducerMessage{
		Topic: "simulation-control",
		Value: sarama.ByteEncoder(commandBytes),
	}
	h.kafkaProducer.SendMessage(message)

	status := "disabled"
	if chaosConfig.Enabled {
		status = "enabled"
	}

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Chaos mode %s successfully", status),
			"config":  chaosConfig,
		},
	})
}

// ResetSimulationConfig handles POST /api/admin/simulation/reset-config
func (h *AdminHandler) ResetSimulationConfig(c *fiber.Ctx) error {
	// Create default config
	config := simulation.NewSimulationConfig()

	// Save to Redis
	ctx := context.Background()
	configJSON, _ := config.ToJSON()
	h.redisClient.Set(ctx, "simulation:config", configJSON, 0)

	// Send reset command to Kafka
	command := map[string]interface{}{
		"type":      "config_reset",
		"timestamp": time.Now().Unix(),
	}
	commandBytes, _ := json.Marshal(command)
	message := &sarama.ProducerMessage{
		Topic: "simulation-control",
		Value: sarama.ByteEncoder(commandBytes),
	}
	h.kafkaProducer.SendMessage(message)

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message": "Simulation configuration reset to defaults",
			"config":  config,
		},
	})
}
