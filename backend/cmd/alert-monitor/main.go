package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/chetansierra/smart-city-monitor/internal/alerts"
	"github.com/chetansierra/smart-city-monitor/internal/config"
	"github.com/chetansierra/smart-city-monitor/internal/kafka"
	"github.com/chetansierra/smart-city-monitor/internal/logger"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Setup logger
	logger.Setup(logger.Config{
		Level:      cfg.App.LogLevel,
		Service:    "alert-monitor",
		PrettyLogs: cfg.App.Environment == "development",
	})

	log.Info().Msg("Starting Alert Monitor Service")

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
	log.Info().Msg("Connected to PostgreSQL")

	// Create Kafka producer for alerts
	producerCfg := kafka.ProducerConfig{
		Brokers: cfg.Kafka.Brokers,
	}

	producer, err := kafka.NewProducer(producerCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Kafka producer")
	}
	defer producer.Close()
	log.Info().Msg("Kafka producer created")

	// Create alert detector
	detector := alerts.NewDetector(db, producer)

	// Create Kafka consumer
	consumerCfg := kafka.ConsumerConfig{
		Brokers: cfg.Kafka.Brokers,
		GroupID: "alert-monitor-group",
		Topics:  []string{cfg.Kafka.TopicSensorReadings},
	}

	consumer, err := kafka.NewConsumer(consumerCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Kafka consumer")
	}
	defer consumer.Close()
	log.Info().Msg("Kafka consumer created")

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Process messages
	go func() {
		log.Info().Msg("Starting to consume sensor readings for alert detection")

		err := consumer.Consume(ctx, []string{cfg.Kafka.TopicSensorReadings}, func(value []byte) error {
			var reading alerts.SensorReading
			if err := json.Unmarshal(value, &reading); err != nil {
				log.Error().
					Err(err).
					Str("value", string(value)).
					Msg("Failed to unmarshal sensor reading")
				return nil // Skip invalid messages
			}

			// Check reading for alert conditions
			if err := detector.CheckReading(ctx, &reading); err != nil {
				log.Error().
					Err(err).
					Str("sensor_id", reading.SensorID).
					Msg("Failed to check reading for alerts")
			}

			return nil
		})

		if err != nil {
			log.Error().Err(err).Msg("Consumer error")
			cancel()
		}
	}()

	// Wait for shutdown signal
	<-quit
	log.Info().Msg("Shutting down Alert Monitor Service...")

	cancel()
	log.Info().Msg("Alert Monitor Service stopped")
}
