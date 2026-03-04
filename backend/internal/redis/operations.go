package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	CommandCreateSensor    = "create_sensor"
	CommandDeleteSensor    = "delete_sensor"
	CommandShutdownSession = "shutdown_session"
	ChannelSimulation      = "simulation:commands"
)

type SimulationCommand struct {
	Type      string    `json:"type"`
	SessionID uuid.UUID `json:"session_id"`
	Payload   any       `json:"payload,omitempty"`
}

// SetLatestReading stores the latest sensor reading in Redis
func (c *Client) SetLatestReading(ctx context.Context, reading *models.SensorReading) error {
	key := fmt.Sprintf("sensor:latest:%s", reading.SensorID.String())

	data := map[string]interface{}{
		"sensor_id":   reading.SensorID.String(),
		"sensor_type": string(reading.SensorType),
		"value":       reading.Value,
		"unit":        reading.Unit,
		"latitude":    reading.Latitude,
		"longitude":   reading.Longitude,
		"timestamp":   reading.Timestamp.Format(time.RFC3339Nano),
		"session_id":  "",
	}
	if reading.SessionID != nil {
		data["session_id"] = reading.SessionID.String()
	}

	if err := c.HSet(ctx, key, data).Err(); err != nil {
		return fmt.Errorf("failed to set latest reading: %w", err)
	}

	// Keep latest-reading cache short-lived to prevent stale buildup.
	if err := c.Expire(ctx, key, latestReadingTTL).Err(); err != nil {
		return fmt.Errorf("failed to set expiration: %w", err)
	}

	return nil
}

// GetLatestReading retrieves the latest sensor reading from Redis
func (c *Client) GetLatestReading(ctx context.Context, sensorID uuid.UUID) (*models.SensorReading, error) {
	key := fmt.Sprintf("sensor:latest:%s", sensorID.String())

	data, err := c.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest reading: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no reading found for sensor %s", sensorID)
	}

	// Parse the data (simplified - in production you'd want better parsing)
	reading := &models.SensorReading{
		SensorID: sensorID,
	}

	return reading, nil
}

// AddToSensorStream adds a reading to a sensor's time-series stream
func (c *Client) AddToSensorStream(ctx context.Context, reading *models.SensorReading) error {
	key := fmt.Sprintf("sensor:stream:%s", reading.SensorID.String())

	values := map[string]interface{}{
		"value":     reading.Value,
		"timestamp": reading.Timestamp.Format(time.RFC3339Nano),
	}

	// Keep per-sensor stream bounded and short-lived.
	if err := c.XAdd(ctx, &redis.XAddArgs{
		Stream: key,
		MaxLen: sensorStreamMaxLen,
		Approx: true,
		Values: values,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to stream: %w", err)
	}
	if err := c.Expire(ctx, key, sensorStreamTTL).Err(); err != nil {
		return fmt.Errorf("failed to set stream expiration: %w", err)
	}

	return nil
}

// AddSensorGeo adds a sensor to the geospatial index
func (c *Client) AddSensorGeo(ctx context.Context, sensor *models.Sensor) error {
	key := "sensors:geo"

	if err := c.GeoAdd(ctx, key, &redis.GeoLocation{
		Name:      sensor.ID.String(),
		Longitude: sensor.Longitude,
		Latitude:  sensor.Latitude,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add sensor to geo index: %w", err)
	}

	return nil
}

// GetNearbySensors finds sensors within a radius
func (c *Client) GetNearbySensors(ctx context.Context, lat, lon, radiusKm float64) ([]string, error) {
	key := "sensors:geo"

	results, err := c.GeoRadius(ctx, key, lon, lat, &redis.GeoRadiusQuery{
		Radius: radiusKm,
		Unit:   "km",
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to query nearby sensors: %w", err)
	}

	sensorIDs := make([]string, len(results))
	for i, result := range results {
		sensorIDs[i] = result.Name
	}

	return sensorIDs, nil
}

// SetCityStats caches city-wide statistics
func (c *Client) SetCityStats(ctx context.Context, stats interface{}) error {
	key := "city:stats"

	data, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("failed to marshal city stats: %w", err)
	}

	// Cache for 5 minutes
	if err := c.Set(ctx, key, data, 5*time.Minute).Err(); err != nil {
		return fmt.Errorf("failed to set city stats: %w", err)
	}

	return nil
}

// GetCityStats retrieves cached city-wide statistics
func (c *Client) GetCityStats(ctx context.Context) ([]byte, error) {
	key := "city:stats"

	data, err := c.Get(ctx, key).Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get city stats: %w", err)
	}

	return data, nil
}

// AddToPollutionLeaderboard adds a sensor to the pollution leaderboard (sorted set)
func (c *Client) AddToPollutionLeaderboard(ctx context.Context, sensorID uuid.UUID, value float64) error {
	key := "pollution:leaderboard"

	if err := c.ZAdd(ctx, key, redis.Z{
		Score:  value,
		Member: sensorID.String(),
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to pollution leaderboard: %w", err)
	}

	return nil
}

// GetTopPolluted retrieves the most polluted sensors
func (c *Client) GetTopPolluted(ctx context.Context, limit int) ([]string, error) {
	key := "pollution:leaderboard"

	// Get top N with highest scores (most polluted)
	results, err := c.ZRevRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get top polluted: %w", err)
	}

	return results, nil
}

// PublishSensorUpdate publishes a sensor update to Pub/Sub
func (c *Client) PublishSensorUpdate(ctx context.Context, reading *models.SensorReading) error {
	channel := "sensor:updates"

	data, err := json.Marshal(reading)
	if err != nil {
		return fmt.Errorf("failed to marshal reading: %w", err)
	}

	if err := c.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish update: %w", err)
	}

	return nil
}

// Incr increments the value of a key by 1 and returns the new value
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	result, err := c.Client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}
	return result, nil
}

// TTL returns the remaining time to live of a key
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	result, err := c.Client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for key %s: %w", key, err)
	}
	return result, nil
}

const latestReadingTTL = 30 * time.Minute
const sensorStreamTTL = 30 * time.Minute
const sensorStreamMaxLen = 300
const sessionHeartbeatTTL = 24 * time.Hour
const sessionStreamTTL = 90 * time.Second
const sessionStreamMaxLen = 180
const sessionConfigTTL = 24 * time.Hour
const statsVisitorsSeenKey = "stats:visitors:seen_sessions"
const statsGoroutinesGlobalKey = "stats:goroutines:global"
const statsSessionWorkersPrefix = "stats:session:workers:"

// SessionConfig stores per-session origin and spawn behavior for sensors.
type SessionConfig struct {
	OriginLat     float64   `json:"origin_lat"`
	OriginLon     float64   `json:"origin_lon"`
	SpawnRadiusKm float64   `json:"spawn_radius_km"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SetSessionHeartbeat updates the heartbeat for a session with 24h TTL
func (c *Client) SetSessionHeartbeat(ctx context.Context, sessionID uuid.UUID) error {
	key := fmt.Sprintf("session:heartbeat:%s", sessionID.String())
	return c.Set(ctx, key, time.Now().Unix(), sessionHeartbeatTTL).Err()
}

// SetSessionConfig stores per-session config with 24h TTL.
func (c *Client) SetSessionConfig(ctx context.Context, sessionID uuid.UUID, cfg SessionConfig) error {
	key := fmt.Sprintf("session:config:%s", sessionID.String())
	if cfg.UpdatedAt.IsZero() {
		cfg.UpdatedAt = time.Now().UTC()
	}
	pipe := c.TxPipeline()
	pipe.HSet(ctx, key, map[string]interface{}{
		"origin_lat":      cfg.OriginLat,
		"origin_lon":      cfg.OriginLon,
		"spawn_radius_km": cfg.SpawnRadiusKm,
		"updated_at":      cfg.UpdatedAt.Format(time.RFC3339Nano),
	})
	pipe.Expire(ctx, key, sessionConfigTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// GetSessionConfig retrieves per-session config.
func (c *Client) GetSessionConfig(ctx context.Context, sessionID uuid.UUID) (SessionConfig, bool, error) {
	key := fmt.Sprintf("session:config:%s", sessionID.String())
	data, err := c.HGetAll(ctx, key).Result()
	if err != nil {
		return SessionConfig{}, false, err
	}
	if len(data) == 0 {
		return SessionConfig{}, false, nil
	}

	cfg := SessionConfig{}
	if _, err := fmt.Sscanf(data["origin_lat"], "%f", &cfg.OriginLat); err != nil {
		return SessionConfig{}, false, fmt.Errorf("invalid origin_lat in redis: %w", err)
	}
	if _, err := fmt.Sscanf(data["origin_lon"], "%f", &cfg.OriginLon); err != nil {
		return SessionConfig{}, false, fmt.Errorf("invalid origin_lon in redis: %w", err)
	}
	if _, err := fmt.Sscanf(data["spawn_radius_km"], "%f", &cfg.SpawnRadiusKm); err != nil {
		return SessionConfig{}, false, fmt.Errorf("invalid spawn_radius_km in redis: %w", err)
	}
	if rawTS, ok := data["updated_at"]; ok && rawTS != "" {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, rawTS); parseErr == nil {
			cfg.UpdatedAt = parsed
		}
	}
	return cfg, true, nil
}

// RegisterSessionVisit records a visitor via HyperLogLog (fixed ~12KB memory).
func (c *Client) RegisterSessionVisit(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	added, err := c.PFAdd(ctx, statsVisitorsSeenKey, sessionID.String()).Result()
	if err != nil {
		return false, err
	}
	return added > 0, nil
}

// GetTotalVisitors returns the approximate unique sessions ever seen (HyperLogLog).
func (c *Client) GetTotalVisitors(ctx context.Context) (int64, error) {
	return c.PFCount(ctx, statsVisitorsSeenKey).Result()
}

// SetGlobalGoroutines stores the latest process-wide goroutine count.
func (c *Client) SetGlobalGoroutines(ctx context.Context, count int64) error {
	return c.Set(ctx, statsGoroutinesGlobalKey, count, 5*time.Minute).Err()
}

// GetGlobalGoroutines retrieves the latest process-wide goroutine count.
func (c *Client) GetGlobalGoroutines(ctx context.Context) (int64, error) {
	value, err := c.Get(ctx, statsGoroutinesGlobalKey).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return value, nil
}

// IncrSessionWorkerGoroutines increments worker goroutines tracked for a session.
func (c *Client) IncrSessionWorkerGoroutines(ctx context.Context, sessionID string) (int64, error) {
	if sessionID == "" {
		return 0, nil
	}
	key := statsSessionWorkersPrefix + sessionID
	value, err := c.Incr(ctx, key)
	if err != nil {
		return 0, err
	}
	_ = c.Expire(ctx, key, sessionHeartbeatTTL).Err()
	return value, nil
}

// DecrSessionWorkerGoroutines decrements worker goroutines tracked for a session.
func (c *Client) DecrSessionWorkerGoroutines(ctx context.Context, sessionID string) (int64, error) {
	if sessionID == "" {
		return 0, nil
	}
	key := statsSessionWorkersPrefix + sessionID
	value, err := c.Client.Decr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		_ = c.Del(ctx, key).Err()
		return 0, nil
	}
	_ = c.Expire(ctx, key, sessionHeartbeatTTL).Err()
	return value, nil
}

// GetSessionWorkerGoroutines returns the tracked worker goroutines for a session.
func (c *Client) GetSessionWorkerGoroutines(ctx context.Context, sessionID string) (int64, error) {
	if sessionID == "" {
		return 0, nil
	}
	key := statsSessionWorkersPrefix + sessionID
	value, err := c.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, nil
	}
	return value, nil
}

// GetActiveSessions returns all session IDs that have an active heartbeat
func (c *Client) GetActiveSessions(ctx context.Context) ([]uuid.UUID, error) {
	var sessions []uuid.UUID
	iter := c.Scan(ctx, 0, "session:heartbeat:*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		sessionIDStr := key[len("session:heartbeat:"):]
		id, err := uuid.Parse(sessionIDStr)
		if err == nil {
			sessions = append(sessions, id)
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

// PublishSimulationCommand sends a command to the simulator
func (c *Client) PublishSimulationCommand(ctx context.Context, cmd SimulationCommand) error {
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	fmt.Printf("[DEBUG] Publishing simulation command to channel %s: %s\n", ChannelSimulation, string(data))
	return c.Publish(ctx, ChannelSimulation, data).Err()
}

// AppendSessionEvent appends a live event payload for a user session.
func (c *Client) AppendSessionEvent(ctx context.Context, sessionID string, payload string) error {
	if sessionID == "" || payload == "" {
		return nil
	}
	key := fmt.Sprintf("session:events:%s", sessionID)
	listLen, err := c.LPush(ctx, key, payload).Result()
	if err != nil {
		return err
	}
	// Only set expire when the key is new (first element pushed)
	if listLen == 1 {
		_ = c.Expire(ctx, key, sessionStreamTTL).Err()
	}
	// Periodically trim to prevent unbounded growth
	if listLen > int64(sessionStreamMaxLen) {
		_ = c.LTrim(ctx, key, 0, int64(sessionStreamMaxLen)-1).Err()
	}
	return nil
}

// GetSessionEvents returns cached live events for a session.
func (c *Client) GetSessionEvents(ctx context.Context, sessionID string, limit int64) ([]string, error) {
	if sessionID == "" {
		return []string{}, nil
	}
	if limit <= 0 || limit > 1000 {
		limit = sessionStreamMaxLen
	}
	key := fmt.Sprintf("session:events:%s", sessionID)
	return c.LRange(ctx, key, 0, limit-1).Result()
}
