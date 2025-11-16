package ssh

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config == nil {
		t.Fatal("DefaultRetryConfig() returned nil")
	}

	if config.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", config.MaxAttempts)
	}

	if config.InitialInterval != 1*time.Second {
		t.Errorf("InitialInterval = %v, want 1s", config.InitialInterval)
	}

	if config.MaxInterval != 30*time.Second {
		t.Errorf("MaxInterval = %v, want 30s", config.MaxInterval)
	}

	if config.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f, want 2.0", config.Multiplier)
	}

	if config.MaxElapsedTime != 2*time.Minute {
		t.Errorf("MaxElapsedTime = %v, want 2m", config.MaxElapsedTime)
	}

	if config.Logger == nil {
		t.Error("Logger is nil")
	}
}

func TestWithRetry_Success(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
		Logger:          logrus.New(),
	}

	attempts := 0
	operation := func() error {
		attempts++
		return nil
	}

	err := WithRetry(operation, config)
	if err != nil {
		t.Errorf("WithRetry() error = %v, want nil", err)
	}

	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestWithRetry_RetryableError(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
		Logger:          logrus.New(),
	}

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 3 {
			// Return a retryable error (connection error)
			return NewConnectionError("localhost", 22, "user", "connection failed", errors.New("connection refused"))
		}
		return nil
	}

	err := WithRetry(operation, config)
	if err != nil {
		t.Errorf("WithRetry() error = %v, want nil", err)
	}

	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
		Logger:          logrus.New(),
	}

	attempts := 0
	nonRetryableErr := NewAuthenticationError("localhost", "user", AuthMethodPassword, "authentication failed", nil)

	operation := func() error {
		attempts++
		return nonRetryableErr
	}

	err := WithRetry(operation, config)
	if err == nil {
		t.Fatal("WithRetry() error = nil, want non-nil")
	}

	// Should only attempt once since error is non-retryable
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}

	// Verify the error is returned
	if !IsAuthenticationError(err) {
		t.Errorf("expected AuthenticationError, got %T", err)
	}
}

func TestWithRetry_MaxAttempts(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  10 * time.Second, // Long enough to not timeout
		Logger:          logrus.New(),
	}

	attempts := 0
	operation := func() error {
		attempts++
		// Use a retryable error pattern
		return NewConnectionError("localhost", 22, "user", "connection failed", errors.New("connection refused"))
	}

	err := WithRetry(operation, config)
	if err == nil {
		t.Fatal("WithRetry() error = nil, want non-nil")
	}

	// MaxAttempts=3 means 3 retries, so 4 total attempts (1 initial + 3 retries)
	if attempts != 4 {
		t.Errorf("attempts = %d, want 4 (1 initial + 3 retries)", attempts)
	}
}

func TestWithRetry_NilConfig(t *testing.T) {
	attempts := 0
	operation := func() error {
		attempts++
		return nil
	}

	err := WithRetry(operation, nil)
	if err != nil {
		t.Errorf("WithRetry() error = %v, want nil", err)
	}

	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestWithRetryContext_Success(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
		Logger:          logrus.New(),
	}

	attempts := 0
	operation := func() error {
		attempts++
		return nil
	}

	err := WithRetryContext(ctx, operation, config)
	if err != nil {
		t.Errorf("WithRetryContext() error = %v, want nil", err)
	}

	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestWithRetryContext_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := &RetryConfig{
		MaxAttempts:     10,
		InitialInterval: 50 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  10 * time.Second,
		Logger:          logrus.New(),
	}

	attempts := 0
	operation := func() error {
		attempts++
		if attempts == 2 {
			// Cancel context on second attempt
			cancel()
		}
		return NewConnectionError("localhost", 22, "user", "connection failed", errors.New("connection refused"))
	}

	err := WithRetryContext(ctx, operation, config)
	if err == nil {
		t.Fatal("WithRetryContext() error = nil, want non-nil")
	}

	// Should stop after context cancellation
	if attempts > 3 {
		t.Errorf("attempts = %d, want <= 3", attempts)
	}
}

func TestWithRetryContext_NilConfig(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	operation := func() error {
		attempts++
		return nil
	}

	err := WithRetryContext(ctx, operation, nil)
	if err != nil {
		t.Errorf("WithRetryContext() error = %v, want nil", err)
	}

	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestRetryConnection_Success(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
		Logger:          logrus.New(),
	}

	attempts := 0
	connectFunc := func() error {
		attempts++
		return nil
	}

	err := RetryConnection("localhost", 22, "testuser", connectFunc, config)
	if err != nil {
		t.Errorf("RetryConnection() error = %v, want nil", err)
	}

	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestRetryConnection_WithRetries(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
		Logger:          logrus.New(),
	}

	attempts := 0
	connectFunc := func() error {
		attempts++
		if attempts < 2 {
			return errors.New("connection refused")
		}
		return nil
	}

	err := RetryConnection("localhost", 22, "testuser", connectFunc, config)
	if err != nil {
		t.Errorf("RetryConnection() error = %v, want nil", err)
	}

	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestRetryConnection_NilConfig(t *testing.T) {
	attempts := 0
	connectFunc := func() error {
		attempts++
		return nil
	}

	err := RetryConnection("localhost", 22, "testuser", connectFunc, nil)
	if err != nil {
		t.Errorf("RetryConnection() error = %v, want nil", err)
	}

	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestRetryConnectionContext_Success(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
		Logger:          logrus.New(),
	}

	attempts := 0
	connectFunc := func() error {
		attempts++
		return nil
	}

	err := RetryConnectionContext(ctx, "localhost", 22, "testuser", connectFunc, config)
	if err != nil {
		t.Errorf("RetryConnectionContext() error = %v, want nil", err)
	}

	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestRetryConnectionContext_NilConfig(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	connectFunc := func() error {
		attempts++
		return nil
	}

	err := RetryConnectionContext(ctx, "localhost", 22, "testuser", connectFunc, nil)
	if err != nil {
		t.Errorf("RetryConnectionContext() error = %v, want nil", err)
	}

	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestCalculateNextInterval(t *testing.T) {
	tests := []struct {
		name         string
		attempt      int
		baseInterval time.Duration
		multiplier   float64
		maxInterval  time.Duration
		wantMin      time.Duration
		wantMax      time.Duration
	}{
		{
			name:         "first attempt",
			attempt:      1,
			baseInterval: 1 * time.Second,
			multiplier:   2.0,
			maxInterval:  30 * time.Second,
			wantMin:      900 * time.Millisecond,  // 1s - 10% jitter
			wantMax:      1100 * time.Millisecond, // 1s + 10% jitter
		},
		{
			name:         "second attempt",
			attempt:      2,
			baseInterval: 1 * time.Second,
			multiplier:   2.0,
			maxInterval:  30 * time.Second,
			wantMin:      1800 * time.Millisecond, // 2s - 10% jitter
			wantMax:      2200 * time.Millisecond, // 2s + 10% jitter
		},
		{
			name:         "third attempt",
			attempt:      3,
			baseInterval: 1 * time.Second,
			multiplier:   2.0,
			maxInterval:  30 * time.Second,
			wantMin:      3600 * time.Millisecond, // 4s - 10% jitter
			wantMax:      4400 * time.Millisecond, // 4s + 10% jitter
		},
		{
			name:         "capped at max interval",
			attempt:      10,
			baseInterval: 1 * time.Second,
			multiplier:   2.0,
			maxInterval:  10 * time.Second,
			wantMin:      9 * time.Second,  // 10s - 10% jitter
			wantMax:      11 * time.Second, // 10s + 10% jitter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interval := CalculateNextInterval(tt.attempt, tt.baseInterval, tt.multiplier, tt.maxInterval)

			if interval < tt.wantMin || interval > tt.wantMax {
				t.Errorf("CalculateNextInterval() = %v, want between %v and %v", interval, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestPowFloat(t *testing.T) {
	tests := []struct {
		name string
		base float64
		exp  float64
		want float64
	}{
		{"2^0", 2.0, 0.0, 1.0},
		{"2^1", 2.0, 1.0, 2.0},
		{"2^2", 2.0, 2.0, 4.0},
		{"2^3", 2.0, 3.0, 8.0},
		{"3^2", 3.0, 2.0, 9.0},
		{"1.5^2", 1.5, 2.0, 2.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := powFloat(tt.base, tt.exp)
			if got != tt.want {
				t.Errorf("powFloat(%v, %v) = %v, want %v", tt.base, tt.exp, got, tt.want)
			}
		})
	}
}

func TestRandomFloat(t *testing.T) {
	// Test that randomFloat returns values between 0 and 1
	for i := 0; i < 100; i++ {
		val := randomFloat()
		if val < 0.0 || val > 1.0 {
			t.Errorf("randomFloat() = %v, want value between 0.0 and 1.0", val)
		}
	}

	// Test that it generates different values
	values := make(map[float64]bool)
	for i := 0; i < 100; i++ {
		values[randomFloat()] = true
	}

	if len(values) < 50 {
		t.Errorf("randomFloat() generated only %d unique values in 100 calls, want more variation", len(values))
	}
}

func TestWithRetry_LoggingNil(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
		Logger:          nil, // Test with nil logger
	}

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 2 {
			return NewConnectionError("localhost", 22, "user", "connection failed", errors.New("connection refused"))
		}
		return nil
	}

	err := WithRetry(operation, config)
	if err != nil {
		t.Errorf("WithRetry() error = %v, want nil", err)
	}

	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestRetryConnection_WrapsNonConnectionError(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:     2,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
		Logger:          logrus.New(),
	}

	attempts := 0
	connectFunc := func() error {
		attempts++
		if attempts == 1 {
			// Return a retryable generic error (connection refused)
			return fmt.Errorf("connection refused")
		}
		return nil
	}

	err := RetryConnection("testhost", 2222, "testuser", connectFunc, config)
	if err != nil {
		t.Errorf("RetryConnection() error = %v, want nil", err)
	}

	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestWithRetryContext_ContextCancelledBeforeFirstAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := &RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
		Logger:          logrus.New(),
	}

	attempts := 0
	operation := func() error {
		attempts++
		return errors.New("should not succeed")
	}

	err := WithRetryContext(ctx, operation, config)
	if err == nil {
		t.Fatal("WithRetryContext() error = nil, want context cancelled error")
	}

	// May attempt once before checking context
	if attempts > 1 {
		t.Errorf("attempts = %d, want <= 1", attempts)
	}
}
