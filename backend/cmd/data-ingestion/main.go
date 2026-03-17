package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/chetansierra/smart-city-monitor/internal/aggregation"
	"github.com/chetansierra/smart-city-monitor/internal/anomaly"
	"github.com/chetansierra/smart-city-monitor/internal/config"
	"github.com/chetansierra/smart-city-monitor/internal/kafka"
	"github.com/chetansierra/smart-city-monitor/internal/logger"
	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/chetansierra/smart-city-monitor/internal/resilience"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}
	if err := cfg.Validate("data-ingestion"); err != nil {
		log.Fatal().Err(err).Msg("Invalid configuration")
	}

	logger.Setup(logger.Config{
		Level:      cfg.App.LogLevel,
		Service:    "data-ingestion",
		PrettyLogs: cfg.App.Environment == "development",
	})

	log.Info().Msg("Starting Data Ingestion Service")

	// Ensure Kafka topics exist
	topicSpecs := []kafka.TopicSpec{
		{Name: cfg.Kafka.TopicSensorReadings, NumPartitions: int32(cfg.Kafka.NumPartitions), ReplicationFactor: 1},
		{Name: cfg.Kafka.TopicDLQ, NumPartitions: 1, ReplicationFactor: 1},
		{Name: cfg.Kafka.TopicAnomalies, NumPartitions: 3, ReplicationFactor: 1},
		{Name: cfg.Kafka.TopicEvents, NumPartitions: 3, ReplicationFactor: 1},
	}
	if err := kafka.EnsureTopics(cfg.Kafka.Brokers, topicSpecs); err != nil {
		log.Warn().Err(err).Msg("Failed to ensure Kafka topics (will rely on auto-create)")
	}

	// Connect to PostgreSQL
	db, err := postgres.Connect(postgres.Config{
		Host:               cfg.Postgres.Host,
		Port:               cfg.Postgres.Port,
		Database:           cfg.Postgres.Database,
		User:               cfg.Postgres.User,
		Password:           cfg.Postgres.Password,
		SSLMode:            cfg.Postgres.SSLMode,
		MaxConnections:     cfg.Postgres.MaxConnections,
		MaxIdleConnections: cfg.Postgres.MaxIdleConnections,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer db.Close()
	log.Info().Str("host", cfg.Postgres.Host).Msg("Connected to PostgreSQL")

	// Connect to Redis
	redisClient, err := redis.Connect(redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer redisClient.Close()
	log.Info().Str("addr", cfg.Redis.Addr).Msg("Connected to Redis")

	// Create circuit breakers
	redisCB := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		FailureThreshold: cfg.Resilience.CircuitBreakerThreshold,
		SuccessThreshold: 2,
		Timeout:          time.Duration(cfg.Resilience.CircuitBreakerTimeout) * time.Second,
	})

	// Create DLQ producer
	dlqProducer, err := kafka.NewProducer(kafka.ProducerConfig{Brokers: cfg.Kafka.Brokers})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create DLQ producer (DLQ disabled)")
	}
	var dlq *kafka.DLQProducer
	if dlqProducer != nil {
		dlq = kafka.NewDLQProducer(dlqProducer, cfg.Kafka.TopicDLQ)
	}

	// Create anomaly producer (for publishing to sensor-anomalies topic)
	anomalyProducer, err := kafka.NewProducer(kafka.ProducerConfig{Brokers: cfg.Kafka.Brokers})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create anomaly producer")
	}

	// Retry config
	retryConfig := &resilience.RetryConfig{
		MaxRetries:     cfg.Resilience.MaxRetries,
		InitialBackoff: time.Duration(cfg.Resilience.RetryInitialBackoffMS) * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		Multiplier:     2.0,
	}

	// Create Kafka consumer with DLQ and retry
	consumer, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers:       cfg.Kafka.Brokers,
		GroupID:       cfg.Kafka.ConsumerGroupIngestion,
		Topics:        []string{cfg.Kafka.TopicSensorReadings},
		InitialOffset: sarama.OffsetNewest,
		DLQProducer:   dlq,
		RetryConfig:   retryConfig,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Kafka consumer")
	}
	defer consumer.Close()

	// Create anomaly and pattern detectors
	anomalyDetector := anomaly.NewAnomalyDetector()
	patternDetector := anomaly.NewPatternDetector()

	// Create ingestion service
	service := &IngestionService{
		db:              db,
		redis:           redisClient,
		mu:              &sync.Mutex{},
		messageCount:    0,
		redisCB:         redisCB,
		anomalyDetector: anomalyDetector,
		patternDetector: patternDetector,
		anomalyProducer: anomalyProducer,
		anomalyTopic:    cfg.Kafka.TopicAnomalies,
		eventsTopic:     cfg.Kafka.TopicEvents,
		hourlyAccum:     make(map[string]*hourlyBucket),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start hourly aggregate flusher (replaces batch flusher + aggregation worker's SQL query)
	go service.startAggregateFlusher(ctx)

	// Start aggregation worker
	aggWorker := aggregation.NewAggregationWorker(db, redisClient)
	go aggWorker.Start(ctx)

	// Start lag monitor
	lagMonitor := kafka.NewLagMonitor(cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroupIngestion,
		[]string{cfg.Kafka.TopicSensorReadings}, redisClient.Client)
	go lagMonitor.Start(ctx)

	// Start consuming messages
	go func() {
		if err := consumer.Consume(ctx, []string{cfg.Kafka.TopicSensorReadings}, service.handleMessage); err != nil {
			log.Fatal().Err(err).Msg("Consumer error")
		}
	}()

	log.Info().
		Str("topic", cfg.Kafka.TopicSensorReadings).
		Str("dlq_topic", cfg.Kafka.TopicDLQ).
		Str("consumer_group", cfg.Kafka.ConsumerGroupIngestion).
		Msg("Data Ingestion Service is running (no sensor_readings persistence)")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info().Msg("Shutting down Data Ingestion Service")
	cancel()

	// Wait for in-flight anomaly/event handlers to finish
	service.wg.Wait()

	// Final flush of hourly aggregates
	service.flushHourlyAggregates()

	if dlqProducer != nil {
		dlqProducer.Close()
	}
	if anomalyProducer != nil {
		anomalyProducer.Close()
	}

	log.Info().Int("total_messages", service.messageCount).Msg("Data Ingestion Service stopped gracefully")
}

// hourlyBucket accumulates per-sensor-per-hour stats in memory.
type hourlyBucket struct {
	SensorID   uuid.UUID
	SensorType models.SensorType
	Sum        float64
	Min        float64
	Max        float64
	Count      int
	PeriodStart time.Time
}

// IngestionService handles data ingestion
type IngestionService struct {
	db              *postgres.DB
	redis           *redis.Client
	mu              *sync.Mutex
	wg              sync.WaitGroup
	messageCount    int
	redisCB         *resilience.CircuitBreaker
	anomalyDetector *anomaly.AnomalyDetector
	patternDetector *anomaly.PatternDetector
	anomalyProducer *kafka.Producer
	anomalyTopic    string
	eventsTopic     string
	hourlyAccum     map[string]*hourlyBucket // key: "sensorID|hour"
}

// handleMessage processes a single Kafka message
func (s *IngestionService) handleMessage(messageBytes []byte) error {
	var kafkaMsg models.KafkaMessage
	if err := json.Unmarshal(messageBytes, &kafkaMsg); err != nil {
		log.Warn().Err(err).Msg("Failed to unmarshal message")
		return nil // Don't retry bad messages
	}

	sensorID, err := uuid.Parse(kafkaMsg.SensorID)
	if err != nil {
		log.Warn().Err(err).Str("sensor_id", kafkaMsg.SensorID).Msg("Invalid sensor ID")
		return nil
	}

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

	s.mu.Lock()
	s.messageCount++
	s.mu.Unlock()

	// Update Redis with circuit breaker protection
	ctx := context.Background()
	if err := s.redisCB.Execute(func() error {
		if err := s.redis.SetLatestReading(ctx, &reading); err != nil {
			return err
		}
		if err := s.redis.AddToSensorStream(ctx, &reading); err != nil {
			return err
		}
		if reading.SensorType == models.SensorTypePollution {
			if err := s.redis.AddToPollutionLeaderboard(ctx, reading.SensorID, reading.Value); err != nil {
				return err
			}
		}
		// Update global and session reading counters
		_ = s.redis.IncrTotalReadings(ctx)
		_ = s.redis.AddDistinctSensor(ctx, reading.SensorID.String())
		if reading.SessionID != nil {
			_ = s.redis.UpdateSessionReadingStats(ctx, reading.SessionID.String(), reading.Value)
			_ = s.redis.TrackSessionReading(ctx, reading.SessionID.String())
		}
		return nil
	}); err != nil {
		if errors.Is(err, resilience.ErrCircuitOpen) {
			log.Debug().Msg("Redis circuit open — skipping cache update")
		} else {
			log.Warn().Err(err).Msg("Redis update failed")
		}
	}

	// Accumulate for hourly aggregates (in-memory)
	s.accumulateHourly(&reading)

	// Anomaly detection (best-effort)
	if anomalyEvent := s.anomalyDetector.ProcessReading(&reading); anomalyEvent != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleAnomaly(ctx, anomalyEvent)
		}()
	}

	// Pattern detection (best-effort)
	if detectedEvent := s.patternDetector.ProcessReading(&reading); detectedEvent != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleDetectedEvent(ctx, detectedEvent)
		}()
	}

	return nil
}

func (s *IngestionService) handleAnomaly(ctx context.Context, event *models.AnomalyEvent) {
	// Store in DB
	if err := s.db.InsertAnomalyEvent(ctx, event); err != nil {
		log.Error().Err(err).Msg("Failed to insert anomaly event")
	}

	// Publish to Kafka anomalies topic (API gateway consumes directly)
	if s.anomalyProducer != nil {
		s.anomalyProducer.SendMessage(s.anomalyTopic, event.SensorID.String(), event)
	}
}

func (s *IngestionService) handleDetectedEvent(ctx context.Context, event *models.DetectedEvent) {
	if err := s.db.InsertDetectedEvent(ctx, event); err != nil {
		log.Error().Err(err).Msg("Failed to insert detected event")
	}

	// Publish to Kafka events topic (API gateway consumes directly)
	if s.anomalyProducer != nil {
		s.anomalyProducer.SendMessage(s.eventsTopic, event.Zone, event)
	}
}

// accumulateHourly adds a reading to the in-memory hourly accumulator.
func (s *IngestionService) accumulateHourly(reading *models.SensorReading) {
	hourStart := reading.Timestamp.Truncate(time.Hour)
	key := fmt.Sprintf("%s|%s", reading.SensorID.String(), hourStart.Format(time.RFC3339))

	s.mu.Lock()
	defer s.mu.Unlock()

	bucket, exists := s.hourlyAccum[key]
	if !exists {
		s.hourlyAccum[key] = &hourlyBucket{
			SensorID:    reading.SensorID,
			SensorType:  reading.SensorType,
			Sum:         reading.Value,
			Min:         reading.Value,
			Max:         reading.Value,
			Count:       1,
			PeriodStart: hourStart,
		}
		return
	}
	bucket.Sum += reading.Value
	bucket.Count++
	if reading.Value < bucket.Min {
		bucket.Min = reading.Value
	}
	if reading.Value > bucket.Max {
		bucket.Max = reading.Value
	}
}

// flushHourlyAggregates writes accumulated hourly stats to sensor_aggregates.
func (s *IngestionService) flushHourlyAggregates() {
	s.mu.Lock()
	if len(s.hourlyAccum) == 0 {
		s.mu.Unlock()
		return
	}
	// Swap out the accumulator
	accum := s.hourlyAccum
	s.hourlyAccum = make(map[string]*hourlyBucket)
	s.mu.Unlock()

	ctx := context.Background()
	flushed := 0
	for _, bucket := range accum {
		avg := bucket.Sum / float64(bucket.Count)
		periodEnd := bucket.PeriodStart.Add(time.Hour)

		_, err := s.db.ExecContext(ctx, `
			INSERT INTO sensor_aggregates (sensor_id, sensor_type, aggregation_type, avg_value, min_value, max_value, reading_count, period_start, period_end)
			VALUES ($1, $2, 'hourly', $3, $4, $5, $6, $7, $8)
			ON CONFLICT (sensor_id, aggregation_type, period_start)
			DO UPDATE SET
				avg_value = EXCLUDED.avg_value,
				min_value = LEAST(sensor_aggregates.min_value, EXCLUDED.min_value),
				max_value = GREATEST(sensor_aggregates.max_value, EXCLUDED.max_value),
				reading_count = sensor_aggregates.reading_count + EXCLUDED.reading_count
		`, bucket.SensorID, bucket.SensorType, avg, bucket.Min, bucket.Max, bucket.Count, bucket.PeriodStart, periodEnd)
		if err != nil {
			log.Error().Err(err).Str("sensor_id", bucket.SensorID.String()).Msg("Failed to upsert hourly aggregate")
			continue
		}
		flushed++
	}

	if flushed > 0 {
		log.Debug().Int("buckets_flushed", flushed).Msg("Flushed hourly aggregates to PostgreSQL")
	}
}

// startAggregateFlusher periodically flushes hourly accumulators to sensor_aggregates.
func (s *IngestionService) startAggregateFlusher(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Info().Msg("Starting hourly aggregate flusher (5 min interval)")

	for {
		select {
		case <-ticker.C:
			s.flushHourlyAggregates()
		case <-ctx.Done():
			return
		}
	}
}
