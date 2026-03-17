package aggregation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/chetansierra/smart-city-monitor/internal/zones"
	"github.com/rs/zerolog/log"
)

// AggregationWorker periodically computes hourly aggregates and caches analytics.
type AggregationWorker struct {
	db    *postgres.DB
	redis *redis.Client
}

// NewAggregationWorker creates a new worker.
func NewAggregationWorker(db *postgres.DB, redis *redis.Client) *AggregationWorker {
	return &AggregationWorker{db: db, redis: redis}
}

// Start runs the aggregation loop every 5 minutes.
func (w *AggregationWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Info().Msg("Starting aggregation worker (5 min interval)")

	// Run once on startup after a short delay
	time.Sleep(30 * time.Second)
	w.run(ctx)

	for {
		select {
		case <-ticker.C:
			w.run(ctx)
		case <-ctx.Done():
			log.Info().Msg("Stopping aggregation worker")
			return
		}
	}
}

func (w *AggregationWorker) run(ctx context.Context) {
	start := time.Now()

	// Hourly aggregates are now computed inline by data-ingestion (in-memory accumulation).
	// This worker only caches city and zone stats from Redis latest readings.

	if err := w.cacheCityStats(ctx); err != nil {
		log.Error().Err(err).Msg("Aggregation worker: failed to cache city stats")
	}

	if err := w.cacheZoneStats(ctx); err != nil {
		log.Error().Err(err).Msg("Aggregation worker: failed to cache zone stats")
	}

	log.Debug().Dur("duration", time.Since(start)).Msg("Aggregation worker cycle complete")
}

type cityStatsComputed struct {
	AvgTemperature float64   `json:"avg_temperature"`
	AvgPollution   float64   `json:"avg_pollution"`
	AvgHumidity    float64   `json:"avg_humidity"`
	AvgNoise       float64   `json:"avg_noise"`
	Timestamp      time.Time `json:"timestamp"`
}

func (w *AggregationWorker) cacheCityStats(ctx context.Context) error {
	sensors, err := w.db.GetAllSensors(ctx)
	if err != nil {
		return err
	}

	var tempSum, pollutionSum, humiditySum, noiseSum float64
	var tempCount, pollutionCount, humidityCount, noiseCount int

	for _, sensor := range sensors {
		key := fmt.Sprintf("sensor:latest:%s", sensor.ID.String())
		data, err := w.redis.HGetAll(ctx, key).Result()
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

	stats := cityStatsComputed{Timestamp: time.Now().UTC()}
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

	statsJSON, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	return w.redis.Set(ctx, "analytics:city-stats:computed", statsJSON, 5*time.Minute).Err()
}

func (w *AggregationWorker) cacheZoneStats(ctx context.Context) error {
	sensorTypes := []models.SensorType{
		models.SensorTypeTemperature,
		models.SensorTypePollution,
		models.SensorTypeHumidity,
		models.SensorTypeNoise,
	}

	for _, sensorType := range sensorTypes {
		sensors, err := w.db.GetSensorsByType(ctx, sensorType)
		if err != nil {
			continue
		}

		for zoneName, bounds := range zones.Zones {
			var totalValue float64
			var count int

			for _, sensor := range sensors {
				if sensor.Latitude >= bounds.MinLat && sensor.Latitude <= bounds.MaxLat &&
					sensor.Longitude >= bounds.MinLng && sensor.Longitude <= bounds.MaxLng {

					key := fmt.Sprintf("sensor:latest:%s", sensor.ID.String())
					data, err := w.redis.HGetAll(ctx, key).Result()
					if err != nil || len(data) == 0 {
						continue
					}

					var value float64
					if v, ok := data["value"]; ok {
						fmt.Sscanf(v, "%f", &value)
					}
					totalValue += value
					count++
				}
			}

			if count > 0 {
				avg := totalValue / float64(count)
				cacheKey := fmt.Sprintf("analytics:zone:%s:%s", zoneName, string(sensorType))
				data, _ := json.Marshal(map[string]interface{}{
					"zone":         zoneName,
					"sensor_type":  string(sensorType),
					"avg_value":    avg,
					"sensor_count": count,
					"timestamp":    time.Now().UTC(),
				})
				w.redis.Set(ctx, cacheKey, data, 5*time.Minute)
			}
		}
	}

	return nil
}
