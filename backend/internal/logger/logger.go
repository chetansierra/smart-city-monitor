package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config holds logger configuration
type Config struct {
	Level      string // debug, info, warn, error
	Service    string // service name
	PrettyLogs bool   // enable pretty console output
}

// Setup initializes the global logger
func Setup(cfg Config) {
	// Set log level
	level := parseLevel(cfg.Level)
	zerolog.SetGlobalLevel(level)

	// Configure time format
	zerolog.TimeFieldFormat = time.RFC3339

	// Pretty logging for development
	if cfg.PrettyLogs {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
		})
	}

	// Add service name to all logs
	log.Logger = log.With().Str("service", cfg.Service).Logger()

	log.Info().
		Str("level", cfg.Level).
		Str("service", cfg.Service).
		Bool("pretty", cfg.PrettyLogs).
		Msg("Logger initialized")
}

// parseLevel converts string level to zerolog.Level
func parseLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// GetLogger returns a logger with context
func GetLogger() *zerolog.Logger {
	return &log.Logger
}

// WithContext returns a logger with additional context fields
func WithContext(fields map[string]interface{}) *zerolog.Logger {
	logger := log.Logger
	for key, value := range fields {
		logger = logger.With().Interface(key, value).Logger()
	}
	return &logger
}
