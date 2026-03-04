package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/chetansierra/smart-city-monitor/internal/config"
	"github.com/chetansierra/smart-city-monitor/internal/kafka"
	"github.com/chetansierra/smart-city-monitor/internal/logger"
	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}
	if err := cfg.Validate("data-ingestion"); err != nil {
		log.Fatal().Err(err).Msg("Invalid configuration")
	}

	// Setup logger
	logger.Setup(logger.Config{
		Level:      cfg.App.LogLevel,
		Service:    "data-ingestion",
		PrettyLogs: cfg.App.Environment == "development",
	})

	log.Info().Msg("Starting Data Ingestion Service")

	// Connect to PostgreSQL
	dbCfg := postgres.Config{
		Host:               cfg.Postgres.Host,
		Port:               cfg.Postgres.Port,
		Database:           cfg.Postgres.Database,
		User:               cfg.Postgres.User,
		Password:           cfg.Postgres.Password,
		SSLMode:            cfg.Postgres.SSLMode,
		MaxConnections:     cfg.Postgres.MaxConnections,
		MaxIdleConnections: cfg.Postgres.MaxIdleConnections,
	}

	db, err := postgres.Connect(dbCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer db.Close()
	log.Info().Str("host", cfg.Postgres.Host).Msg("Connected to PostgreSQL")

	// Connect to Redis
	redisCfg := redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	redisClient, err := redis.Connect(redisCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer redisClient.Close()
	log.Info().Str("addr", cfg.Redis.Addr).Msg("Connected to Redis")

	// Create Kafka consumer
	consumer, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers:       cfg.Kafka.Brokers,
		GroupID:       cfg.Kafka.ConsumerGroupIngestion,
		Topics:        []string{cfg.Kafka.TopicSensorReadings},
		InitialOffset: sarama.OffsetNewest,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Kafka consumer")
	}
	defer consumer.Close()

	// Create ingestion service
	service := &IngestionService{
		db:            db,
		redis:         redisClient,
		batchSize:     100,
		batchBuffer:   make([]models.SensorReading, 0, 100),
		mu:            &sync.Mutex{},
		messageCount:  0,
		retentionDays: cfg.Data.RetentionDays,
	}

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start batch flusher
	go service.startBatchFlusher(ctx)

	// Start retention cleaner
	go service.startRetentionCleaner(ctx)

	// Start consuming messages
	go func() {
		if err := consumer.Consume(ctx, []string{cfg.Kafka.TopicSensorReadings}, service.handleMessage); err != nil {
			log.Fatal().Err(err).Msg("Consumer error")
		}
	}()

	log.Info().
		Str("topic", cfg.Kafka.TopicSensorReadings).
		Str("consumer_group", cfg.Kafka.ConsumerGroupIngestion).
		Int("batch_size", service.batchSize).
		Msg("Data Ingestion Service is running")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info().Msg("Shutting down Data Ingestion Service")
	cancel()

	// Flush remaining messages
	service.flushBatch()

	log.Info().Int("total_messages", service.messageCount).Msg("Data Ingestion Service stopped gracefully")
}

// IngestionService handles data ingestion
type IngestionService struct {
	db            *postgres.DB
	redis         *redis.Client
	batchSize     int
	batchBuffer   []models.SensorReading
	mu            *sync.Mutex
	messageCount  int
	retentionDays int
}

// handleMessage processes a single Kafka message
func (s *IngestionService) handleMessage(messageBytes []byte) error {
	// Parse Kafka message
	var kafkaMsg models.KafkaMessage
	if err := json.Unmarshal(messageBytes, &kafkaMsg); err != nil {
		log.Warn().Err(err).Msg("Failed to unmarshal message")
		return nil // Don't fail, just skip bad messages
	}

	// Convert to SensorReading
	sensorID, err := uuid.Parse(kafkaMsg.SensorID)
	if err != nil {
		log.Warn().Err(err).Str("sensor_id", kafkaMsg.SensorID).Msg("Invalid sensor ID")
		return nil
	}

	// Try parsing with nanoseconds first, fallback to RFC3339 for backward compatibility
	timestamp, err := time.Parse(time.RFC3339Nano, kafkaMsg.Timestamp)
	if err != nil {
		timestamp, err = time.Parse(time.RFC3339, kafkaMsg.Timestamp)
		if err != nil {
			log.Warn().Err(err).Str("timestamp", kafkaMsg.Timestamp).Msg("Invalid timestamp")
			return nil
		}
	}

	var sessionID *uuid.UUID
	if kafkaMsg.SessionID != "" {
		id, err := uuid.Parse(kafkaMsg.SessionID)
		if err == nil {
			sessionID = &id
		}
	}

	reading := models.SensorReading{
		SensorID:   sensorID,
		SessionID:  sessionID,
		SensorType: models.SensorType(kafkaMsg.SensorType),
		Value:      kafkaMsg.Value,
		Unit:       kafkaMsg.Unit,
		Location: models.Location{
			Latitude:  kafkaMsg.Location.Latitude,
			Longitude: kafkaMsg.Location.Longitude,
		},
		Latitude:  kafkaMsg.Location.Latitude,
		Longitude: kafkaMsg.Location.Longitude,
		Timestamp: timestamp,
	}

	// Add to batch
	s.mu.Lock()
	s.batchBuffer = append(s.batchBuffer, reading)
	s.messageCount++
	shouldFlush := len(s.batchBuffer) >= s.batchSize
	s.mu.Unlock()

	// Update Redis immediately for real-time data
	ctx := context.Background()
	if err := s.redis.SetLatestReading(ctx, &reading); err != nil {
		log.Error().Err(err).Str("sensor_id", sensorID.String()).Msg("Failed to update Redis latest reading")
	}

	// Add to time-series stream
	if err := s.redis.AddToSensorStream(ctx, &reading); err != nil {
		log.Error().Err(err).Str("sensor_id", sensorID.String()).Msg("Failed to add to sensor stream")
	}

	// Update pollution leaderboard for pollution sensors
	if reading.SensorType == models.SensorTypePollution {
		if err := s.redis.AddToPollutionLeaderboard(ctx, reading.SensorID, reading.Value); err != nil {
			log.Error().Err(err).Str("sensor_id", sensorID.String()).Msg("Failed to update pollution leaderboard")
		}
	}

	// Publish update for SSE subscribers
	if err := s.redis.PublishSensorUpdate(ctx, &reading); err != nil {
		log.Error().Err(err).Str("sensor_id", sensorID.String()).Msg("Failed to publish sensor update")
	}

	// Flush batch if needed
	if shouldFlush {
		go s.flushBatch()
	}

	return nil
}

// flushBatch writes batched readings to PostgreSQL
func (s *IngestionService) flushBatch() {
	s.mu.Lock()
	if len(s.batchBuffer) == 0 {
		s.mu.Unlock()
		return
	}

	// Get batch and reset buffer
	batch := make([]models.SensorReading, len(s.batchBuffer))
	copy(batch, s.batchBuffer)
	s.batchBuffer = s.batchBuffer[:0]
	batchSize := len(batch)
	s.mu.Unlock()

	// Insert batch into PostgreSQL
	ctx := context.Background()
	if err := s.db.InsertSensorReadingsBatch(ctx, batch); err != nil {
		log.Error().Err(err).Int("batch_size", batchSize).Msg("Failed to insert batch into PostgreSQL")
		return
	}

	if batchSize >= 50 {
		log.Debug().Int("batch_size", batchSize).Msg("Inserted batch into PostgreSQL")
	}
}

// startRetentionCleaner periodically deletes old sensor readings.
func (s *IngestionService) startRetentionCleaner(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	retentionInterval := fmt.Sprintf("%d days", s.retentionDays)
	log.Info().Str("retention", retentionInterval).Msg("Starting data retention cleaner (runs every 6h)")

	for {
		select {
		case <-ticker.C:
			s.runRetentionCleanup(ctx, retentionInterval)
		case <-ctx.Done():
			return
		}
	}
}

func (s *IngestionService) runRetentionCleanup(ctx context.Context, retentionInterval string) {
	const batchLimit = 10000
	totalDeleted := int64(0)

	for {
		result, err := s.db.ExecContext(ctx,
			`DELETE FROM sensor_readings WHERE ctid IN (
				SELECT ctid FROM sensor_readings
				WHERE timestamp < NOW() - $1::interval
				LIMIT $2
			)`, retentionInterval, batchLimit)
		if err != nil {
			log.Error().Err(err).Msg("Retention cleaner: failed to delete old readings")
			break
		}
		affected, _ := result.RowsAffected()
		totalDeleted += affected
		if affected < batchLimit {
			break
		}
	}

	if totalDeleted > 0 {
		log.Info().Int64("deleted_rows", totalDeleted).Str("retention", retentionInterval).Msg("Retention cleaner: purged old readings")
	}
}

// startBatchFlusher periodically flushes batches
func (s *IngestionService) startBatchFlusher(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.flushBatch()
		case <-ctx.Done():
			return
		}
	}
}
