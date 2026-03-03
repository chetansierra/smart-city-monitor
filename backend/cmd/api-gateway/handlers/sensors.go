package handlers

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
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

const (
	defaultOriginLat     = 28.6139
	defaultOriginLon     = 77.2090
	defaultSpawnRadiusKm = 5.0
	minSpawnRadiusKm     = 1.0
	maxSpawnRadiusKm     = 100.0
	maxSensorsPerUser    = 50
	earthRadiusKm        = 6371.0
)

type sessionConfigResponse struct {
	OriginLat     float64 `json:"origin_lat"`
	OriginLon     float64 `json:"origin_lon"`
	SpawnRadiusKm float64 `json:"spawn_radius_km"`
}

type updateSessionConfigRequest struct {
	OriginLat     *float64 `json:"origin_lat"`
	OriginLon     *float64 `json:"origin_lon"`
	SpawnRadiusKm *float64 `json:"spawn_radius_km"`
}

type sensorFootprintPoint struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

// NewSensorsHandler creates a new sensors handler
func NewSensorsHandler(db *postgres.DB, redisClient *redis.Client) *SensorsHandler {
	return &SensorsHandler{
		db:          db,
		redisClient: redisClient,
	}
}

func parseSessionID(c *fiber.Ctx) (uuid.UUID, error) {
	sessionIDStr := c.Get("X-Session-ID")
	if sessionIDStr == "" {
		return uuid.Nil, fmt.Errorf("X-Session-ID header is required")
	}
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid session ID format")
	}
	return sessionID, nil
}

func clampRadius(radius float64) float64 {
	if radius < minSpawnRadiusKm {
		return minSpawnRadiusKm
	}
	if radius > maxSpawnRadiusKm {
		return maxSpawnRadiusKm
	}
	return radius
}

func validateCoordinates(lat, lon float64) error {
	if lat < -90 || lat > 90 {
		return fmt.Errorf("lat must be between -90 and 90")
	}
	if lon < -180 || lon > 180 {
		return fmt.Errorf("lon must be between -180 and 180")
	}
	return nil
}

func normalizeLon(lon float64) float64 {
	for lon > 180 {
		lon -= 360
	}
	for lon < -180 {
		lon += 360
	}
	return lon
}

func randomPointWithinRadius(originLat, originLon, radiusKm float64) (float64, float64) {
	distanceKm := radiusKm * math.Sqrt(rand.Float64())
	bearing := rand.Float64() * 2 * math.Pi

	lat1 := originLat * math.Pi / 180
	lon1 := originLon * math.Pi / 180
	angularDistance := distanceKm / earthRadiusKm

	lat2 := math.Asin(math.Sin(lat1)*math.Cos(angularDistance) +
		math.Cos(lat1)*math.Sin(angularDistance)*math.Cos(bearing))
	lon2 := lon1 + math.Atan2(
		math.Sin(bearing)*math.Sin(angularDistance)*math.Cos(lat1),
		math.Cos(angularDistance)-math.Sin(lat1)*math.Sin(lat2),
	)

	return lat2 * 180 / math.Pi, normalizeLon(lon2 * 180 / math.Pi)
}

func (h *SensorsHandler) getOrCreateSessionConfig(ctx context.Context, sessionID uuid.UUID) (redis.SessionConfig, error) {
	cfg, exists, err := h.redisClient.GetSessionConfig(ctx, sessionID)
	if err != nil {
		return redis.SessionConfig{}, err
	}
	if exists {
		cfg.SpawnRadiusKm = clampRadius(cfg.SpawnRadiusKm)
		if _, regErr := h.redisClient.RegisterSessionVisit(ctx, sessionID); regErr != nil {
			log.Warn().Err(regErr).Str("session_id", sessionID.String()).Msg("Failed to register session visit")
		}
		return cfg, nil
	}
	cfg = redis.SessionConfig{
		OriginLat:     defaultOriginLat,
		OriginLon:     defaultOriginLon,
		SpawnRadiusKm: defaultSpawnRadiusKm,
		UpdatedAt:     time.Now().UTC(),
	}
	if err := h.redisClient.SetSessionConfig(ctx, sessionID, cfg); err != nil {
		return redis.SessionConfig{}, err
	}
	if _, regErr := h.redisClient.RegisterSessionVisit(ctx, sessionID); regErr != nil {
		log.Warn().Err(regErr).Str("session_id", sessionID.String()).Msg("Failed to register session visit")
	}
	return cfg, nil
}

// GetSessionConfig handles GET /api/v1/session/config
func (h *SensorsHandler) GetSessionConfig(c *fiber.Ctx) error {
	sessionID, err := parseSessionID(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "MISSING_OR_INVALID_SESSION_ID",
				Message: err.Error(),
			},
		})
	}
	ctx := context.Background()
	cfg, err := h.getOrCreateSessionConfig(ctx, sessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID.String()).Msg("Failed to get session config")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "SESSION_CONFIG_ERROR",
				Message: "Failed to load session config",
			},
		})
	}
	_ = h.redisClient.SetSessionHeartbeat(ctx, sessionID)
	return c.JSON(APIResponse{
		Success: true,
		Data: sessionConfigResponse{
			OriginLat:     cfg.OriginLat,
			OriginLon:     cfg.OriginLon,
			SpawnRadiusKm: cfg.SpawnRadiusKm,
		},
	})
}

// UpdateSessionConfig handles PUT /api/v1/session/config
func (h *SensorsHandler) UpdateSessionConfig(c *fiber.Ctx) error {
	sessionID, err := parseSessionID(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "MISSING_OR_INVALID_SESSION_ID",
				Message: err.Error(),
			},
		})
	}

	var req updateSessionConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid JSON body",
			},
		})
	}

	ctx := context.Background()
	cfg, err := h.getOrCreateSessionConfig(ctx, sessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID.String()).Msg("Failed to load session config for update")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "SESSION_CONFIG_ERROR",
				Message: "Failed to load session config",
			},
		})
	}

	if req.OriginLat != nil {
		cfg.OriginLat = *req.OriginLat
	}
	if req.OriginLon != nil {
		cfg.OriginLon = *req.OriginLon
	}
	if err := validateCoordinates(cfg.OriginLat, cfg.OriginLon); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_COORDINATES",
				Message: err.Error(),
			},
		})
	}
	if req.SpawnRadiusKm != nil {
		cfg.SpawnRadiusKm = clampRadius(*req.SpawnRadiusKm)
	}
	cfg.UpdatedAt = time.Now().UTC()

	if err := h.redisClient.SetSessionConfig(ctx, sessionID, cfg); err != nil {
		log.Error().Err(err).Str("session_id", sessionID.String()).Msg("Failed to save session config")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "SESSION_CONFIG_ERROR",
				Message: "Failed to save session config",
			},
		})
	}
	_ = h.redisClient.SetSessionHeartbeat(ctx, sessionID)
	return c.JSON(APIResponse{
		Success: true,
		Data: sessionConfigResponse{
			OriginLat:     cfg.OriginLat,
			OriginLon:     cfg.OriginLon,
			SpawnRadiusKm: cfg.SpawnRadiusKm,
		},
	})
}

// StartSensor handles POST /api/v1/sensors/start
func (h *SensorsHandler) StartSensor(c *fiber.Ctx) error {
	sessionID, err := parseSessionID(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "MISSING_OR_INVALID_SESSION_ID",
				Message: err.Error(),
			},
		})
	}

	sensorType := c.Query("type", "temperature")
	sensorID := uuid.New()
	ctx := context.Background()
	sessionCfg, err := h.getOrCreateSessionConfig(ctx, sessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID.String()).Msg("Failed to load session config")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "SESSION_CONFIG_ERROR",
				Message: "Failed to load session configuration",
			},
		})
	}
	lat, lon, err := resolveSensorCoordinates(c, sessionCfg)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_COORDINATES",
				Message: err.Error(),
			},
		})
	}

	sensorCount, err := h.db.CountSensorsBySession(ctx, sessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID.String()).Msg("Failed to count session sensors")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to validate sensor limit",
			},
		})
	}
	if sensorCount >= maxSensorsPerUser {
		return c.Status(fiber.StatusConflict).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "SENSOR_LIMIT_REACHED",
				Message: fmt.Sprintf("Maximum %d sensors allowed", maxSensorsPerUser),
			},
		})
	}

	// Create sensor in database
	sensor := models.Sensor{
		ID:        sensorID,
		SessionID: &sessionID,
		Name:      fmt.Sprintf("%s-sensor-%s", sensorType, sensorID.String()[:8]),
		Type:      models.SensorType(sensorType),
		Location: models.Location{
			Latitude:  lat,
			Longitude: lon,
		},
		Status:    models.SensorStatusActive,
		Latitude:  lat,
		Longitude: lon,
		Config:    "{}",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	log.Info().
		Str("sensor_id", sensor.ID.String()).
		Str("session_id", sessionID.String()).
		Str("sensor_type", sensorType).
		Msg("Creating sensor record")

	if err := h.db.InsertSensor(ctx, &sensor); err != nil {
		log.Error().
			Err(err).
			Str("sensor_id", sensor.ID.String()).
			Str("session_id", sessionID.String()).
			Str("sensor_type", sensorType).
			Msg("Failed to persist sensor to database")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to create sensor record",
			},
		})
	}

	// Update heartbeat
	_ = h.redisClient.SetSessionHeartbeat(ctx, sessionID)

	// Publish command to simulator
	cmd := redis.SimulationCommand{
		Type:      redis.CommandCreateSensor,
		SessionID: sessionID,
		Payload: map[string]interface{}{
			"id":   sensorID,
			"type": sensorType,
			"lat":  sensor.Latitude,
			"lon":  sensor.Longitude,
		},
	}

	if err := h.redisClient.PublishSimulationCommand(ctx, cmd); err != nil {
		log.Error().Err(err).Msg("Failed to publish simulation command")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "SIMULATION_ERROR",
				Message: "Failed to trigger sensor simulation",
			},
		})
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    sensor,
	})
}

// GetFootprint handles GET /api/v1/sensors/footprint
// Returns lightweight map markers for either current session or global scope.
func (h *SensorsHandler) GetFootprint(c *fiber.Ctx) error {
	scope := c.Query("scope", "session")
	if scope != "session" && scope != "global" {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_SCOPE",
				Message: "scope must be either 'session' or 'global'",
			},
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		rows interface {
			Next() bool
			Scan(dest ...any) error
			Close() error
			Err() error
		}
		err error
	)

	if scope == "global" {
		rows, err = h.db.QueryContext(ctx, `
			SELECT id, type, latitude, longitude
			FROM sensors
			ORDER BY created_at DESC
		`)
	} else {
		sessionID, parseErr := parseSessionID(c)
		if parseErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
				Success: false,
				Error: &APIError{
					Code:    "MISSING_OR_INVALID_SESSION_ID",
					Message: parseErr.Error(),
				},
			})
		}
		rows, err = h.db.QueryContext(ctx, `
			SELECT id, type, latitude, longitude
			FROM sensors
			WHERE session_id = $1
			ORDER BY created_at DESC
		`, sessionID)
	}
	if err != nil {
		log.Error().Err(err).Str("scope", scope).Msg("Failed to query sensor footprint")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to fetch sensor footprint",
			},
		})
	}
	defer rows.Close()

	points := make([]sensorFootprintPoint, 0)
	typeCounts := map[string]int{
		"temperature": 0,
		"humidity":    0,
		"pollution":   0,
		"noise":       0,
	}

	for rows.Next() {
		var (
			id  uuid.UUID
			typ string
			lat float64
			lon float64
		)
		if scanErr := rows.Scan(&id, &typ, &lat, &lon); scanErr != nil {
			log.Error().Err(scanErr).Msg("Failed to scan sensor footprint row")
			continue
		}
		points = append(points, sensorFootprintPoint{
			ID:        id.String(),
			Type:      typ,
			Latitude:  lat,
			Longitude: lon,
		})
		if _, ok := typeCounts[typ]; ok {
			typeCounts[typ]++
		}
	}
	if rows.Err() != nil {
		log.Error().Err(rows.Err()).Msg("Sensor footprint row iteration failed")
	}

	return c.JSON(APIResponse{
		Success: true,
		Data: fiber.Map{
			"scope":  scope,
			"points": points,
			"counts": typeCounts,
			"total":  len(points),
		},
	})
}

// DeleteSensor handles DELETE /api/v1/sensors/:id
func (h *SensorsHandler) DeleteSensor(c *fiber.Ctx) error {
	sessionIDStr := c.Get("X-Session-ID")
	if sessionIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "MISSING_SESSION_ID",
				Message: "X-Session-ID header is required",
			},
		})
	}

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_SESSION_ID",
				Message: "Invalid session ID format",
			},
		})
	}

	sensorID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_SENSOR_ID",
				Message: "Invalid sensor ID format",
			},
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deleted, err := h.db.DeleteSensorByIDAndSession(ctx, sensorID, sessionID)
	if err != nil {
		log.Error().Err(err).Str("sensor_id", sensorID.String()).Msg("Failed to delete sensor")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to delete sensor",
			},
		})
	}

	if !deleted {
		return c.Status(fiber.StatusNotFound).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "SENSOR_NOT_FOUND",
				Message: "Sensor not found for this user session",
			},
		})
	}

	// Best-effort cleanup for latest cache/stream and simulator runtime state.
	h.redisClient.Del(ctx,
		fmt.Sprintf("sensor:latest:%s", sensorID.String()),
		fmt.Sprintf("sensor:stream:%s", sensorID.String()),
	)

	_ = h.redisClient.PublishSimulationCommand(ctx, redis.SimulationCommand{
		Type:      redis.CommandDeleteSensor,
		SessionID: sessionID,
		Payload: map[string]interface{}{
			"id": sensorID.String(),
		},
	})

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"sensor_id": sensorID.String(),
			"deleted":   true,
		},
	})
}

func resolveSensorCoordinates(c *fiber.Ctx, cfg redis.SessionConfig) (float64, float64, error) {
	latParam := c.Query("lat")
	lonParam := c.Query("lon")

	if latParam != "" || lonParam != "" {
		if latParam == "" || lonParam == "" {
			return 0, 0, fmt.Errorf("both lat and lon query parameters are required when setting location")
		}

		parsedLat, err := strconv.ParseFloat(latParam, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("lat must be a valid number")
		}
		parsedLon, err := strconv.ParseFloat(lonParam, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("lon must be a valid number")
		}
		if err := validateCoordinates(parsedLat, parsedLon); err != nil {
			return 0, 0, err
		}
		return parsedLat, parsedLon, nil
	}

	radiusKm := clampRadius(cfg.SpawnRadiusKm)
	if err := validateCoordinates(cfg.OriginLat, cfg.OriginLon); err != nil {
		return 0, 0, fmt.Errorf("session origin coordinates are invalid")
	}
	lat, lon := randomPointWithinRadius(cfg.OriginLat, cfg.OriginLon, radiusKm)
	if err := validateCoordinates(lat, lon); err != nil {
		return 0, 0, fmt.Errorf("failed to generate valid coordinates")
	}
	return lat, lon, nil
}

// StartInactivityMonitor starts a background loop to shutdown idle sessions
func (h *SensorsHandler) StartInactivityMonitor(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Info().Msg("Starting session inactivity monitor (sweep interval: 1m)")

	// Keep track of active sessions we know about
	activeSessions := make(map[string]time.Time)

	for {
		select {
		case <-ticker.C:
			// 1. Get current active heartbeats from Redis
			redisSessions, err := h.redisClient.GetActiveSessions(ctx)
			if err != nil {
				log.Error().Err(err).Msg("Failed to scan active sessions from Redis")
				continue
			}

			currentRedisMap := make(map[string]bool)
			for _, id := range redisSessions {
				currentRedisMap[id.String()] = true
				if _, exists := activeSessions[id.String()]; !exists {
					log.Info().Str("session_id", id.String()).Msg("New active session detected")
				}
				activeSessions[id.String()] = time.Now()
			}

			// 2. Identify sessions that were active but are no longer in Redis (TTL expired)
			for idStr, lastSeen := range activeSessions {
				if !currentRedisMap[idStr] {
					// Session expired in Redis (24h passed since last activity)
					sessionID, _ := uuid.Parse(idStr)
					log.Info().
						Str("session_id", idStr).
						Time("last_seen", lastSeen).
						Msg("Session inactivity detected, shutting down simulation")

					cmd := redis.SimulationCommand{
						Type:      redis.CommandShutdownSession,
						SessionID: sessionID,
					}
					_ = h.redisClient.PublishSimulationCommand(ctx, cmd)
					delete(activeSessions, idStr)
				}
			}

		case <-ctx.Done():
			return
		}
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
