package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	Kafka     KafkaConfig
	Redis     RedisConfig
	Postgres  PostgresConfig
	API       APIConfig
	Simulator SimulatorConfig
	Data      DataConfig
	App       AppConfig
}

// KafkaConfig holds Kafka-related configuration
type KafkaConfig struct {
	Brokers                []string
	InternalBrokers        []string
	TopicSensorReadings    string
	TopicAdminCommands     string
	ConsumerGroupIngestion string
	ConsumerGroupAnalytics string
}

// RedisConfig holds Redis-related configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// PostgresConfig holds PostgreSQL-related configuration
type PostgresConfig struct {
	Host               string
	Port               string
	Database           string
	User               string
	Password           string
	SSLMode            string
	MaxConnections     int
	MaxIdleConnections int
}

// APIConfig holds API server configuration
type APIConfig struct {
	Port           string
	Host           string
	WebSocketPath  string
	AllowedOrigins string
}

// SimulatorConfig holds sensor simulator configuration
type SimulatorConfig struct {
	SensorCount           int
	GenerationIntervalMS  int
	EnableRushHour        bool
	EnableWeatherPatterns bool
	StartEmpty            bool
}

// DataConfig holds data retention settings
type DataConfig struct {
	RetentionDays int
}

// AppConfig holds general application settings
type AppConfig struct {
	LogLevel    string
	Environment string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env file from current directory or parent directory
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	config := &Config{
		Kafka: KafkaConfig{
			Brokers:                []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
			InternalBrokers:        []string{getEnv("KAFKA_INTERNAL_BROKERS", "kafka:9093")},
			TopicSensorReadings:    getEnvWithFallback("KAFKA_TOPIC_SENSOR_READINGS", "sensor-readings", "KAFKA_TOPIC_READINGS"),
			TopicAdminCommands:     getEnv("KAFKA_TOPIC_ADMIN_COMMANDS", "admin-commands"),
			ConsumerGroupIngestion: getEnv("KAFKA_CONSUMER_GROUP_INGESTION", "data-ingestion-group"),
			ConsumerGroupAnalytics: getEnv("KAFKA_CONSUMER_GROUP_ANALYTICS", "analytics-group"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		Postgres: PostgresConfig{
			Host:               getEnv("POSTGRES_HOST", "localhost"),
			Port:               getEnv("POSTGRES_PORT", "5432"),
			Database:           getEnvWithFallback("POSTGRES_DB", "smart_city", "POSTGRES_DATABASE"),
			User:               getEnv("POSTGRES_USER", "admin"),
			Password:           getEnv("POSTGRES_PASSWORD", "password"),
			SSLMode:            getEnvWithFallback("POSTGRES_SSLMODE", "disable", "POSTGRES_SSL_MODE"),
			MaxConnections:     getEnvAsInt("POSTGRES_MAX_CONNECTIONS", 25),
			MaxIdleConnections: getEnvAsInt("POSTGRES_MAX_IDLE_CONNECTIONS", 5),
		},
		API: APIConfig{
			Port:           getEnv("API_PORT", "8080"),
			Host:           getEnv("API_HOST", "0.0.0.0"),
			WebSocketPath:  getEnv("WEBSOCKET_PATH", "/ws"),
			AllowedOrigins: getEnv("ALLOWED_ORIGINS", "*"),
		},
		Simulator: SimulatorConfig{
			SensorCount:           getEnvAsInt("SENSOR_COUNT", 50),
			GenerationIntervalMS:  getEnvAsInt("GENERATION_INTERVAL_MS", 1000),
			EnableRushHour:        getEnvAsBool("ENABLE_RUSH_HOUR", true),
			EnableWeatherPatterns: getEnvAsBool("ENABLE_WEATHER_PATTERNS", true),
			StartEmpty:            getEnvAsBool("SIMULATOR_START_EMPTY", false),
		},
		Data: DataConfig{
			RetentionDays: getEnvAsInt("RETENTION_DAYS", 7),
		},
		App: AppConfig{
			LogLevel:    getEnvWithFallback("LOG_LEVEL", "info", "APP_LOG_LEVEL"),
			Environment: getEnvWithFallback("ENVIRONMENT", "development", "APP_ENVIRONMENT"),
		},
	}

	return config, nil
}

// Validate checks required configuration for a given service before startup.
func (c *Config) Validate(service string) error {
	var missing []string
	require := func(key, value string) {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}

	// App-level requirements
	require("ENVIRONMENT", c.App.Environment)
	require("LOG_LEVEL", c.App.LogLevel)

	// Shared infra requirements
	require("POSTGRES_HOST", c.Postgres.Host)
	require("POSTGRES_PORT", c.Postgres.Port)
	require("POSTGRES_DB", c.Postgres.Database)
	require("POSTGRES_USER", c.Postgres.User)
	require("POSTGRES_PASSWORD", c.Postgres.Password)
	require("REDIS_ADDR", c.Redis.Addr)

	// Kafka requirements
	if len(c.Kafka.Brokers) == 0 || strings.TrimSpace(c.Kafka.Brokers[0]) == "" {
		missing = append(missing, "KAFKA_BROKERS")
	}
	require("KAFKA_TOPIC_SENSOR_READINGS", c.Kafka.TopicSensorReadings)

	switch service {
	case "data-ingestion":
		require("KAFKA_CONSUMER_GROUP_INGESTION", c.Kafka.ConsumerGroupIngestion)
	case "sensor-simulator":
		if c.Simulator.GenerationIntervalMS <= 0 {
			return fmt.Errorf("invalid GENERATION_INTERVAL_MS: %d", c.Simulator.GenerationIntervalMS)
		}
	}

	if c.Postgres.MaxConnections <= 0 {
		return fmt.Errorf("invalid POSTGRES_MAX_CONNECTIONS: %d", c.Postgres.MaxConnections)
	}
	if c.Postgres.MaxIdleConnections < 0 {
		return fmt.Errorf("invalid POSTGRES_MAX_IDLE_CONNECTIONS: %d", c.Postgres.MaxIdleConnections)
	}
	if c.Redis.DB < 0 {
		return fmt.Errorf("invalid REDIS_DB: %d", c.Redis.DB)
	}

	for _, topic := range []string{c.Kafka.TopicSensorReadings, c.Kafka.TopicAdminCommands} {
		if topic == "" {
			continue
		}
		if strings.ContainsAny(topic, " \t\n\r") {
			return fmt.Errorf("invalid Kafka topic name %q: topic names cannot contain whitespace", topic)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration for %s: %s", service, strings.Join(missing, ", "))
	}

	return nil
}

// ConnectionString returns PostgreSQL connection string
func (c *PostgresConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvWithFallback reads key first and then falls back to legacy aliases.
func getEnvWithFallback(key, defaultValue string, aliases ...string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	for _, alias := range aliases {
		if value := os.Getenv(alias); value != "" {
			return value
		}
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultValue
}
