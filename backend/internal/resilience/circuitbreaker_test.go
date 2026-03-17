package resilience

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreakerClosedState(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	})

	assert.Equal(t, "closed", cb.State())

	err := cb.Execute(func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, "closed", cb.State())
}

func TestCircuitBreakerOpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	})

	testErr := errors.New("test error")

	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return testErr })
	}

	assert.Equal(t, "open", cb.State())

	err := cb.Execute(func() error { return nil })
	assert.ErrorIs(t, err, ErrCircuitOpen)
}

func TestCircuitBreakerTransitionsToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
	})

	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error { return testErr })
	}
	assert.Equal(t, "open", cb.State())

	time.Sleep(60 * time.Millisecond)

	assert.Equal(t, "half-open", cb.State())

	err := cb.Execute(func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, "closed", cb.State())
}

func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})

	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error { return testErr })
	}

	time.Sleep(60 * time.Millisecond)

	// Fail in half-open → back to open
	err := cb.Execute(func() error { return testErr })
	assert.Error(t, err)
	assert.Equal(t, "open", cb.State())
}

func TestCircuitBreakerConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 100,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cb.Execute(func() error { return nil })
		}()
	}
	wg.Wait()

	assert.Equal(t, "closed", cb.State())
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		Timeout:          1 * time.Minute,
	})

	testErr := errors.New("fail")
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error { return testErr })
	}
	require.Equal(t, "open", cb.State())

	cb.Reset()
	assert.Equal(t, "closed", cb.State())

	err := cb.Execute(func() error { return nil })
	assert.NoError(t, err)
}
