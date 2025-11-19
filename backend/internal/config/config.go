package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	Kafka     KafkaConfig
	Redis     RedisConfig
	Postgres  PostgresConfig
	API       APIConfig
	Simulator SimulatorConfig
	App       AppConfig
}

// KafkaConfig holds Kafka-related configuration
type KafkaConfig struct {
	Brokers                []string
	InternalBrokers        []string
	TopicSensorReadings    string
	TopicAdminCommands     string
	TopicAlerts            string
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
	Port          string
	Host          string
	WebSocketPath string
}

// SimulatorConfig holds sensor simulator configuration
type SimulatorConfig struct {
	SensorCount           int
	GenerationIntervalMS  int
	EnableRushHour        bool
	EnableWeatherPatterns bool
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
			TopicSensorReadings:    getEnv("KAFKA_TOPIC_SENSOR_READINGS", "sensor-readings"),
			TopicAdminCommands:     getEnv("KAFKA_TOPIC_ADMIN_COMMANDS", "admin-commands"),
			TopicAlerts:            getEnv("KAFKA_TOPIC_ALERTS", "alerts"),
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
			Database:           getEnv("POSTGRES_DB", "smart_city"),
			User:               getEnv("POSTGRES_USER", "admin"),
			Password:           getEnv("POSTGRES_PASSWORD", "password"),
			SSLMode:            getEnv("POSTGRES_SSLMODE", "disable"),
			MaxConnections:     getEnvAsInt("POSTGRES_MAX_CONNECTIONS", 25),
			MaxIdleConnections: getEnvAsInt("POSTGRES_MAX_IDLE_CONNECTIONS", 5),
		},
		API: APIConfig{
			Port:          getEnv("API_PORT", "8080"),
			Host:          getEnv("API_HOST", "0.0.0.0"),
			WebSocketPath: getEnv("WEBSOCKET_PATH", "/ws"),
		},
		Simulator: SimulatorConfig{
			SensorCount:           getEnvAsInt("SENSOR_COUNT", 50),
			GenerationIntervalMS:  getEnvAsInt("GENERATION_INTERVAL_MS", 1000),
			EnableRushHour:        getEnvAsBool("ENABLE_RUSH_HOUR", true),
			EnableWeatherPatterns: getEnvAsBool("ENABLE_WEATHER_PATTERNS", true),
		},
		App: AppConfig{
			LogLevel:    getEnv("LOG_LEVEL", "info"),
			Environment: getEnv("ENVIRONMENT", "development"),
		},
	}

	return config, nil
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
