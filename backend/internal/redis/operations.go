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
		"timestamp":   reading.Timestamp.Unix(),
	}

	if err := c.HSet(ctx, key, data).Err(); err != nil {
		return fmt.Errorf("failed to set latest reading: %w", err)
	}

	// Set expiration to 1 hour
	if err := c.Expire(ctx, key, time.Hour).Err(); err != nil {
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
		"timestamp": reading.Timestamp.Unix(),
	}

	// Add to stream with max length of 1000 entries
	if err := c.XAdd(ctx, &redis.XAddArgs{
		Stream: key,
		MaxLen: 1000,
		Approx: true,
		Values: values,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to stream: %w", err)
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
