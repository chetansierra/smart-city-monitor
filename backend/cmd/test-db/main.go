package main

import (
	"fmt"
	"log"
	"os"

	"github.com/chetansierra/smart-city-monitor/internal/config"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
)

func main() {
	fmt.Println("=== Testing PostgreSQL Connection ===")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Test connection
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

	if err := postgres.TestConnection(dbCfg); err != nil {
		log.Fatalf("Database test failed: %v", err)
	}

	fmt.Println("\n✅ All database tests passed!")
	os.Exit(0)
}
