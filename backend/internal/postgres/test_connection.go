package postgres

import (
	"context"
	"fmt"
	"time"
)

// TestConnection tests the database connection and queries
func TestConnection(cfg Config) error {
	// Connect
	db, err := Connect(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer db.Close()

	fmt.Println("✓ Database connection successful")

	// Health check
	if err := db.Health(); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	fmt.Println("✓ Health check passed")

	// Query sensors
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sensors, err := db.GetAllSensors(ctx)
	if err != nil {
		return fmt.Errorf("failed to get sensors: %w", err)
	}

	fmt.Printf("✓ Found %d sensors in database\n", len(sensors))

	// Show sensor type breakdown
	typeCount := make(map[string]int)
	for _, s := range sensors {
		typeCount[string(s.Type)]++
	}

	fmt.Println("\nSensor breakdown by type:")
	for sType, count := range typeCount {
		fmt.Printf("  - %s: %d sensors\n", sType, count)
	}

	return nil
}
