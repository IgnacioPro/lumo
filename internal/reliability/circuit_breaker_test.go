package reliability

import (
	"errors"
	"testing"

	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
)

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker("test-service")

	assert.NotNil(t, cb)
	assert.Equal(t, gobreaker.StateClosed, cb.State())
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	cb := NewCircuitBreaker("test-service")

	result, err := cb.Execute(func() (interface{}, error) {
		return "success", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, gobreaker.StateClosed, cb.State())
}

func TestCircuitBreaker_Execute_Failure(t *testing.T) {
	cb := NewCircuitBreaker("test-service")

	expectedErr := errors.New("test error")
	result, err := cb.Execute(func() (interface{}, error) {
		return nil, expectedErr
	})

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)
}

func TestCircuitBreaker_TripsAfterFailures(t *testing.T) {
	cb := NewCircuitBreaker("test-service")

	// Circuit breaker settings: 3 requests minimum, 60% failure ratio
	// So we need at least 3 requests with 2+ failures to trip

	// Execute 3 failing requests
	for i := 0; i < 3; i++ {
		_, _ = cb.Execute(func() (interface{}, error) {
			return nil, errors.New("failure")
		})
	}

	// Circuit should now be open
	assert.Equal(t, gobreaker.StateOpen, cb.State())

	// Verify that subsequent calls fail immediately
	_, err := cb.Execute(func() (interface{}, error) {
		return "should not execute", nil
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestCircuitBreaker_AllowsSuccessWhenClosed(t *testing.T) {
	cb := NewCircuitBreaker("test-service")

	// Execute successful requests
	for i := 0; i < 5; i++ {
		result, err := cb.Execute(func() (interface{}, error) {
			return i, nil
		})

		assert.NoError(t, err)
		assert.Equal(t, i, result)
	}

	// Circuit should still be closed
	assert.Equal(t, gobreaker.StateClosed, cb.State())
}

func TestCircuitBreaker_State(t *testing.T) {
	cb := NewCircuitBreaker("test-service")

	// Initially closed
	assert.Equal(t, gobreaker.StateClosed, cb.State())

	// Trip the circuit
	for i := 0; i < 3; i++ {
		_, _ = cb.Execute(func() (interface{}, error) {
			return nil, errors.New("failure")
		})
	}

	// Should be open
	assert.Equal(t, gobreaker.StateOpen, cb.State())
}
