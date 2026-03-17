package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chetansierra/smart-city-monitor/cmd/api-gateway/handlers"
	"github.com/chetansierra/smart-city-monitor/internal/config"
	"github.com/chetansierra/smart-city-monitor/internal/kafka"
	"github.com/chetansierra/smart-city-monitor/internal/logger"
	"github.com/chetansierra/smart-city-monitor/internal/middleware"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/chetansierra/smart-city-monitor/internal/sse"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}
	if err := cfg.Validate("api-gateway"); err != nil {
		log.Fatal().Err(err).Msg("Invalid configuration")
	}

	// Setup logger
	logger.Setup(logger.Config{
		Level:      cfg.App.LogLevel,
		Service:    "api-gateway",
		PrettyLogs: cfg.App.Environment == "development",
	})

	log.Info().Msg("Starting Smart City API Gateway")

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
	log.Info().Msg("Connected to Redis")

	// Ensure Kafka topics exist
	topicSpecs := []kafka.TopicSpec{
		{Name: cfg.Kafka.TopicSensorReadings, NumPartitions: int32(cfg.Kafka.NumPartitions), ReplicationFactor: 1},
		{Name: cfg.Kafka.TopicAnomalies, NumPartitions: 3, ReplicationFactor: 1},
		{Name: cfg.Kafka.TopicEvents, NumPartitions: 3, ReplicationFactor: 1},
	}
	if err := kafka.EnsureTopics(cfg.Kafka.Brokers, topicSpecs); err != nil {
		log.Warn().Err(err).Msg("Failed to ensure Kafka topics (will rely on auto-create)")
	}

	// Create SSE broadcaster
	broadcaster := sse.NewBroadcaster()
	go broadcaster.Listen()
	log.Info().Msg("SSE broadcaster started")

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "Smart City API Gateway",
		BodyLimit:    10 * 1024 * 1024, // 10MB
		ErrorHandler: customErrorHandler,
	})

	// Middleware
	app.Use(recover.New())   // Recover from panics
	app.Use(requestid.New()) // Add request ID to each request

	// CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.API.AllowedOrigins,
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Session-ID",
	}))

	// Logging middleware
	app.Use(fiberlogger.New(fiberlogger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path} | ${ip}\n",
		TimeFormat: "15:04:05",
		TimeZone:   "Local",
	}))

	// Security middleware
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.ValidateContentType())
	app.Use(middleware.SanitizeInput())

	// Create handlers
	healthHandler := handlers.NewHealthHandler(db, redisClient, cfg.Kafka.Brokers...)
	sensorsHandler := handlers.NewSensorsHandler(db, redisClient)
	readingsHandler := handlers.NewReadingsHandler(db, redisClient)
	analyticsHandler := handlers.NewAnalyticsHandler(db, redisClient)
	kafkaTopics := []string{cfg.Kafka.TopicSensorReadings, cfg.Kafka.TopicAnomalies, cfg.Kafka.TopicEvents}
	sseHandler := handlers.NewSSEHandler(broadcaster, redisClient, cfg.Kafka.Brokers, kafkaTopics, cfg.Kafka.ConsumerGroupGateway)
	metricsHandler := handlers.NewMetricsHandler(db, redisClient, broadcaster, cfg.Kafka.Brokers)
	adminHandler := handlers.NewAdminHandler(db, redisClient)
	pipelineHandler := handlers.NewPipelineHandler(redisClient, cfg.Kafka.Brokers, metricsHandler, broadcaster)

	// Create cancellable context for background goroutines
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	// Start Kafka consumer for real-time SSE updates (replaces Redis Pub/Sub)
	go sseHandler.StartKafkaConsumer(bgCtx)
	log.Info().Strs("topics", kafkaTopics).Msg("Started Kafka consumer for SSE updates")

	// Start Session Inactivity Monitor
	go sensorsHandler.StartInactivityMonitor(bgCtx)
	log.Info().Msg("Started session inactivity monitor")

	// Start nerd-stats aggregator for cached metrics + SSE updates
	go metricsHandler.StartNerdStatsAggregator(bgCtx)
	log.Info().Msg("Started nerd stats aggregator")

	// Setup routes
	setupRoutes(app, healthHandler, sensorsHandler, readingsHandler, analyticsHandler, sseHandler, metricsHandler, adminHandler, pipelineHandler, redisClient)

	// Start server in a goroutine
	go func() {
		port := "8080"
		log.Info().Str("port", port).Msg("API Gateway listening")
		if err := app.Listen(":" + port); err != nil {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down API Gateway...")

	// Cancel background goroutines (Kafka consumer, inactivity monitor, nerd stats)
	bgCancel()

	// Give outstanding requests 10 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Error().Err(err).Msg("API Gateway forced to shutdown")
	}

	log.Info().Msg("API Gateway stopped")
}

// customErrorHandler handles errors globally
func customErrorHandler(c *fiber.Ctx, err error) error {
	// Default status code 500
	code := fiber.StatusInternalServerError

	// Check if it's a Fiber error
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	log.Error().
		Err(err).
		Int("status", code).
		Str("path", c.Path()).
		Str("method", c.Method()).
		Msg("Request error")

	// Send error response
	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":    fmt.Sprintf("ERROR_%d", code),
			"message": err.Error(),
		},
	})
}
