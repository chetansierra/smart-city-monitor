package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/chetansierra/smart-city-monitor/cmd/api-gateway/handlers"
	"github.com/chetansierra/smart-city-monitor/internal/config"
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

	// Create Kafka producer for admin control commands
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Producer.RequiredAcks = sarama.WaitForAll
	kafkaConfig.Producer.Retry.Max = 5
	kafkaConfig.Producer.Return.Successes = true

	kafkaProducer, err := sarama.NewSyncProducer(cfg.Kafka.Brokers, kafkaConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Kafka producer")
	}
	defer kafkaProducer.Close()
	log.Info().Msg("Kafka producer created")

	// Create SSE broadcaster
	broadcaster := sse.NewBroadcaster()
	go broadcaster.Listen()
	log.Info().Msg("SSE broadcaster started")

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "Smart City API Gateway",
		ErrorHandler: customErrorHandler,
	})

	// Middleware
	app.Use(recover.New())   // Recover from panics
	app.Use(requestid.New()) // Add request ID to each request

	// CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*", // Allow all origins for development (restrict in production)
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
	app.Use(middleware.RequestSizeLimit(10 * 1024 * 1024)) // 10MB limit
	app.Use(middleware.SanitizeInput())

	// Create handlers
	healthHandler := handlers.NewHealthHandler(db, redisClient)
	sensorsHandler := handlers.NewSensorsHandler(db, redisClient)
	readingsHandler := handlers.NewReadingsHandler(db, redisClient)
	analyticsHandler := handlers.NewAnalyticsHandler(db, redisClient)
	sseHandler := handlers.NewSSEHandler(broadcaster, redisClient)
	metricsHandler := handlers.NewMetricsHandler(db, redisClient, broadcaster, cfg.Kafka.Brokers)
	adminHandler := handlers.NewAdminHandler(db, redisClient, kafkaProducer)
	pipelineHandler := handlers.NewPipelineHandler(redisClient, cfg.Kafka.Brokers)

	// Start Redis Pub/Sub listener for real-time updates
	go sseHandler.StartRedisPubSubListener(context.Background())
	log.Info().Msg("Started Redis Pub/Sub listener for SSE updates")

	// Start Session Inactivity Monitor
	go sensorsHandler.StartInactivityMonitor(context.Background())
	log.Info().Msg("Started session inactivity monitor")

	// Start nerd-stats aggregator for cached metrics + SSE updates
	go metricsHandler.StartNerdStatsAggregator(context.Background())
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
