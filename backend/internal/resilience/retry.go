package resilience

import (
	"context"
	"math"
	"math/rand"
	"time"
)

type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
	}
}

func WithRetry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 100 * time.Millisecond
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 10 * time.Second
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = 2.0
	}

	var lastErr error
	backoff := cfg.InitialBackoff

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if attempt == cfg.MaxRetries {
			break
		}

		// Check context before sleeping
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Add 0-25% jitter
		jitter := time.Duration(float64(backoff) * rand.Float64() * 0.25)
		sleepDuration := backoff + jitter

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepDuration):
		}

		// Calculate next backoff
		backoff = time.Duration(math.Min(
			float64(backoff)*cfg.Multiplier,
			float64(cfg.MaxBackoff),
		))
	}

	return lastErr
}
