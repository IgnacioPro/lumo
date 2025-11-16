package ssh

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/sirupsen/logrus"
)

// RetryConfig contains configuration for retry behavior
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts (0 = no retry, -1 = unlimited)
	MaxAttempts int

	// InitialInterval is the initial retry interval
	InitialInterval time.Duration

	// MaxInterval is the maximum retry interval
	MaxInterval time.Duration

	// Multiplier is the multiplier for exponential backoff
	Multiplier float64

	// MaxElapsedTime is the maximum total time spent retrying
	MaxElapsedTime time.Duration

	// Logger for retry attempts
	Logger *logrus.Logger
}

// DefaultRetryConfig returns a RetryConfig with sensible defaults
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
		MaxElapsedTime:  2 * time.Minute,
		Logger:          logrus.New(),
	}
}

// WithRetry executes the given function with retry logic using exponential backoff
func WithRetry(operation func() error, config *RetryConfig) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	// Create exponential backoff
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = config.InitialInterval
	expBackoff.MaxInterval = config.MaxInterval
	expBackoff.Multiplier = config.Multiplier
	expBackoff.MaxElapsedTime = config.MaxElapsedTime

	// Reset the backoff to ensure clean state
	expBackoff.Reset()

	// Wrap with max retries if specified
	var b backoff.BackOff = expBackoff
	if config.MaxAttempts > 0 {
		b = backoff.WithMaxRetries(expBackoff, uint64(config.MaxAttempts))
	}

	attempt := 0
	retryFunc := func() error {
		attempt++
		err := operation()

		if err == nil {
			if attempt > 1 && config.Logger != nil {
				config.Logger.Infof("Operation succeeded after %d attempts", attempt)
			}
			return nil
		}

		// Check if error is retryable
		if !IsRetryable(err) {
			if config.Logger != nil {
				config.Logger.Warnf("Non-retryable error encountered: %v", err)
			}
			return backoff.Permanent(err)
		}

		if config.Logger != nil {
			config.Logger.Warnf("Attempt %d failed: %v", attempt, err)
		}

		return err
	}

	// Add notify function to log backoff wait times
	notifyFunc := func(err error, duration time.Duration) {
		if config.Logger != nil {
			config.Logger.Infof("Retrying in %v after error: %v", duration, err)
		}
	}

	return backoff.RetryNotify(retryFunc, b, notifyFunc)
}

// WithRetryContext executes the given function with retry logic and context support
func WithRetryContext(ctx context.Context, operation func() error, config *RetryConfig) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	// Create exponential backoff
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = config.InitialInterval
	expBackoff.MaxInterval = config.MaxInterval
	expBackoff.Multiplier = config.Multiplier
	expBackoff.MaxElapsedTime = config.MaxElapsedTime

	// Reset the backoff
	expBackoff.Reset()

	// Wrap with max retries if specified
	var b backoff.BackOff = expBackoff
	if config.MaxAttempts > 0 {
		b = backoff.WithMaxRetries(expBackoff, uint64(config.MaxAttempts))
	}

	// Wrap with context
	b = backoff.WithContext(b, ctx)

	attempt := 0
	retryFunc := func() error {
		// Check context before attempting
		select {
		case <-ctx.Done():
			return backoff.Permanent(fmt.Errorf("context cancelled: %w", ctx.Err()))
		default:
		}

		attempt++
		err := operation()

		if err == nil {
			if attempt > 1 && config.Logger != nil {
				config.Logger.Infof("Operation succeeded after %d attempts", attempt)
			}
			return nil
		}

		// Check if error is retryable
		if !IsRetryable(err) {
			if config.Logger != nil {
				config.Logger.Warnf("Non-retryable error encountered: %v", err)
			}
			return backoff.Permanent(err)
		}

		if config.Logger != nil {
			config.Logger.Warnf("Attempt %d failed: %v", attempt, err)
		}

		return err
	}

	// Add notify function
	notifyFunc := func(err error, duration time.Duration) {
		if config.Logger != nil {
			config.Logger.Infof("Retrying in %v after error: %v", duration, err)
		}
	}

	return backoff.RetryNotify(retryFunc, b, notifyFunc)
}

// RetryConnection is a specialized retry function for SSH connections
func RetryConnection(host string, port int, user string, connectFunc func() error, config *RetryConfig) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	if config.Logger != nil {
		config.Logger.Infof("Attempting to connect to %s@%s:%d", user, host, port)
	}

	operation := func() error {
		err := connectFunc()
		if err != nil {
			// Wrap with connection error if it's not already
			if !IsConnectionError(err) {
				err = NewConnectionError(host, port, user, "connection attempt failed", err)
			}
		}
		return err
	}

	return WithRetry(operation, config)
}

// RetryConnectionContext is a specialized retry function with context support
func RetryConnectionContext(ctx context.Context, host string, port int, user string, connectFunc func() error, config *RetryConfig) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	if config.Logger != nil {
		config.Logger.Infof("Attempting to connect to %s@%s:%d", user, host, port)
	}

	operation := func() error {
		err := connectFunc()
		if err != nil {
			// Wrap with connection error if it's not already
			if !IsConnectionError(err) {
				err = NewConnectionError(host, port, user, "connection attempt failed", err)
			}
		}
		return err
	}

	return WithRetryContext(ctx, operation, config)
}

// CalculateNextInterval calculates the next retry interval with jitter
func CalculateNextInterval(attempt int, baseInterval time.Duration, multiplier float64, maxInterval time.Duration) time.Duration {
	interval := float64(baseInterval) * powFloat(multiplier, float64(attempt-1))

	if interval > float64(maxInterval) {
		interval = float64(maxInterval)
	}

	// Add jitter (±10%)
	jitter := interval * 0.1
	interval += (jitter * 2 * (randomFloat() - 0.5))

	return time.Duration(interval)
}

// Helper functions

func powFloat(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// Simple random float generator (for jitter)
var (
	randomSeed  = time.Now().UnixNano()
	randomMutex sync.Mutex
)

func randomFloat() float64 {
	randomMutex.Lock()
	defer randomMutex.Unlock()
	randomSeed = (randomSeed*1103515245 + 12345) & 0x7fffffff
	return float64(randomSeed) / float64(0x7fffffff)
}
