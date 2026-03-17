package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithRetrySuccess(t *testing.T) {
	calls := 0
	err := WithRetry(context.Background(), DefaultRetryConfig(), func() error {
		calls++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestWithRetryEventualSuccess(t *testing.T) {
	calls := 0
	err := WithRetry(context.Background(), RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
	}, func() error {
		calls++
		if calls < 3 {
			return errors.New("not yet")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestWithRetryAllFail(t *testing.T) {
	calls := 0
	testErr := errors.New("persistent failure")
	err := WithRetry(context.Background(), RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		Multiplier:     2.0,
	}, func() error {
		calls++
		return testErr
	})

	assert.ErrorIs(t, err, testErr)
	assert.Equal(t, 3, calls) // initial + 2 retries
}

func TestWithRetryContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	err := WithRetry(ctx, RetryConfig{
		MaxRetries:     10,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
		Multiplier:     2.0,
	}, func() error {
		calls++
		return errors.New("fail")
	})

	assert.Error(t, err)
	assert.True(t, calls < 10, "should have been cancelled before all retries")
}

func TestWithRetryBackoffTiming(t *testing.T) {
	start := time.Now()
	calls := 0

	_ = WithRetry(context.Background(), RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     2.0,
	}, func() error {
		calls++
		return errors.New("fail")
	})

	elapsed := time.Since(start)
	// Should take at least 10ms (first backoff) + 20ms (second backoff) = 30ms minimum
	assert.True(t, elapsed >= 25*time.Millisecond, "backoff should have taken at least 25ms, took %v", elapsed)
	assert.Equal(t, 3, calls)
}
