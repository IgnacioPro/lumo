package ssh

import (
	"strings"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/config"
	"github.com/sirupsen/logrus"
)

func newValidClientConfig() *ClientConfig {
	baseConfig := config.SSHConfig{
		Port:          22,
		Timeout:       30 * time.Second,
		KeepAlive:     15 * time.Second,
		MaxRetries:    3,
		RetryInterval: 5 * time.Second,
	}

	return &ClientConfig{
		BaseConfig:           baseConfig,
		PreferredAuthMethods: []AuthMethod{AuthMethodAgent},
		CommandTimeout:       5 * time.Minute,
		OutputBufferSize:     1024 * 1024,
	}
}

func TestNewClient(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		client, err := NewClient(nil, logrus.New())
		if err == nil {
			t.Error("NewClient() with nil config should return error")
		}
		if client != nil {
			t.Error("NewClient() should return nil client on error")
		}
		if !strings.Contains(err.Error(), "config cannot be nil") {
			t.Errorf("Error should mention nil config, got: %v", err)
		}
	})

	t.Run("invalid config - no auth methods", func(t *testing.T) {
		cfg := newValidClientConfig()
		cfg.PreferredAuthMethods = []AuthMethod{} // No auth methods

		client, err := NewClient(cfg, logrus.New())
		if err == nil {
			t.Error("NewClient() with invalid config should return error")
		}
		if client != nil {
			t.Error("NewClient() should return nil client on error")
		}
	})

	t.Run("valid config", func(t *testing.T) {
		cfg := newValidClientConfig()

		client, err := NewClient(cfg, logrus.New())
		if err != nil {
			t.Fatalf("NewClient() unexpected error: %v", err)
		}

		if client == nil {
			t.Fatal("NewClient() returned nil client")
		}

		if client.config != cfg {
			t.Error("Client config not set correctly")
		}

		if client.logger == nil {
			t.Error("Client logger should not be nil")
		}

		if client.status != StatusDisconnected {
			t.Errorf("Initial status = %v, want %v", client.status, StatusDisconnected)
		}

		if client.connInfo == nil {
			t.Error("Connection info should be initialized")
		}

		if client.retryConfig == nil {
			t.Error("Retry config should be initialized")
		}
	})

	t.Run("nil logger - should use default", func(t *testing.T) {
		cfg := newValidClientConfig()

		client, err := NewClient(cfg, nil)
		if err != nil {
			t.Fatalf("NewClient() unexpected error: %v", err)
		}

		if client.logger == nil {
			t.Error("Client logger should not be nil when nil logger provided")
		}
	})
}

func TestClient_IsConnected(t *testing.T) {
	cfg := newValidClientConfig()
	client, err := NewClient(cfg, logrus.New())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Initially should not be connected
	if client.IsConnected() {
		t.Error("IsConnected() = true, want false (initial state)")
	}

	// Test defensive code: status=connected but sshClient=nil
	client.mu.Lock()
	client.status = StatusConnected
	client.sshClient = nil
	client.mu.Unlock()

	if client.IsConnected() {
		t.Error("IsConnected() = true when sshClient is nil, want false")
	}
}

func TestClient_GetStatus(t *testing.T) {
	cfg := newValidClientConfig()
	client, err := NewClient(cfg, logrus.New())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Initial status
	if status := client.GetStatus(); status != StatusDisconnected {
		t.Errorf("GetStatus() = %v, want %v", status, StatusDisconnected)
	}

	// Test status transitions
	testStatuses := []ConnectionStatus{
		StatusConnecting,
		StatusConnected,
		StatusReconnecting,
		StatusFailed,
		StatusDisconnected,
	}

	for _, expectedStatus := range testStatuses {
		client.mu.Lock()
		client.status = expectedStatus
		client.mu.Unlock()

		if status := client.GetStatus(); status != expectedStatus {
			t.Errorf("GetStatus() = %v, want %v", status, expectedStatus)
		}
	}
}

func TestClient_GetConnectionInfo(t *testing.T) {
	cfg := newValidClientConfig()
	client, err := NewClient(cfg, logrus.New())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test initial connection info
	info := client.GetConnectionInfo()
	if info == nil {
		t.Fatal("GetConnectionInfo() returned nil")
	}

	if info.Status != StatusDisconnected {
		t.Errorf("ConnectionInfo.Status = %v, want %v", info.Status, StatusDisconnected)
	}

	// Verify it returns a copy
	originalInfo := client.connInfo
	returnedInfo := client.GetConnectionInfo()

	if originalInfo == returnedInfo {
		t.Error("GetConnectionInfo() should return a copy, not the original pointer")
	}

	// Update connection info and verify changes
	client.mu.Lock()
	client.connInfo.Host = "example.com"
	client.connInfo.Port = 22
	client.connInfo.User = "testuser"
	client.connInfo.Status = StatusConnecting
	client.mu.Unlock()

	info = client.GetConnectionInfo()
	if info.Host != "example.com" {
		t.Errorf("Host = %q, want 'example.com'", info.Host)
	}
	if info.Port != 22 {
		t.Errorf("Port = %d, want 22", info.Port)
	}
	if info.User != "testuser" {
		t.Errorf("User = %q, want 'testuser'", info.User)
	}
}

func TestClient_Execute_NotConnected(t *testing.T) {
	cfg := newValidClientConfig()
	client, err := NewClient(cfg, logrus.New())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Execute should fail when not connected
	result, err := client.Execute("ls", nil)

	if err == nil {
		t.Error("Execute() on disconnected client should return error")
	}

	if result != nil {
		t.Error("Execute() should return nil result when not connected")
	}

	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("Error should mention 'not connected', got: %v", err)
	}
}

func TestClient_GetSSHClient_NotConnected(t *testing.T) {
	cfg := newValidClientConfig()
	client, err := NewClient(cfg, logrus.New())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Should return nil when not connected
	sshClient := client.GetSSHClient()
	if sshClient != nil {
		t.Error("GetSSHClient() should return nil when not connected")
	}
}

func TestClient_ThreadSafety(t *testing.T) {
	cfg := newValidClientConfig()
	client, err := NewClient(cfg, logrus.New())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	done := make(chan bool)

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = client.GetStatus()
			_ = client.IsConnected()
			_ = client.GetConnectionInfo()
			_ = client.GetSSHClient()
		}
		done <- true
	}()

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			client.mu.Lock()
			client.status = StatusConnecting
			client.connInfo.Host = "example.com"
			client.mu.Unlock()

			client.mu.Lock()
			client.status = StatusDisconnected
			client.connInfo.Host = ""
			client.mu.Unlock()
		}
		done <- true
	}()

	// Wait for both
	<-done
	<-done
	// If we reach here without deadlock, thread safety works
}

func TestClient_SetLogger(t *testing.T) {
	cfg := newValidClientConfig()
	client, err := NewClient(cfg, logrus.New())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("sets new logger", func(t *testing.T) {
		newLogger := logrus.New()
		newLogger.SetLevel(logrus.DebugLevel)

		client.SetLogger(newLogger)

		client.mu.RLock()
		actualLogger := client.logger
		client.mu.RUnlock()

		if actualLogger != newLogger {
			t.Error("SetLogger() did not set the new logger")
		}
	})

	t.Run("ignores nil logger", func(t *testing.T) {
		originalLogger := logrus.New()
		client.SetLogger(originalLogger)

		client.SetLogger(nil)

		client.mu.RLock()
		actualLogger := client.logger
		client.mu.RUnlock()

		if actualLogger != originalLogger {
			t.Error("SetLogger(nil) should not change the logger")
		}
	})

	t.Run("thread-safe logger setting", func(t *testing.T) {
		done := make(chan bool)

		// Writer goroutines
		for i := 0; i < 10; i++ {
			go func() {
				for j := 0; j < 10; j++ {
					logger := logrus.New()
					client.SetLogger(logger)
				}
				done <- true
			}()
		}

		// Reader goroutines
		for i := 0; i < 10; i++ {
			go func() {
				for j := 0; j < 10; j++ {
					client.mu.RLock()
					_ = client.logger
					client.mu.RUnlock()
				}
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 20; i++ {
			<-done
		}
		// If we reach here without deadlock/race, it's thread-safe
	})
}

func TestClient_SetRetryConfig(t *testing.T) {
	cfg := newValidClientConfig()
	client, err := NewClient(cfg, logrus.New())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("sets new retry config", func(t *testing.T) {
		newConfig := &RetryConfig{
			MaxAttempts:     5,
			InitialInterval: 2 * time.Second,
			MaxInterval:     30 * time.Second,
			Multiplier:      3.0,
			MaxElapsedTime:  5 * time.Minute,
		}

		client.SetRetryConfig(newConfig)

		client.mu.RLock()
		actualConfig := client.retryConfig
		client.mu.RUnlock()

		if actualConfig != newConfig {
			t.Error("SetRetryConfig() did not set the new retry config")
		}

		if actualConfig.MaxAttempts != 5 {
			t.Errorf("MaxAttempts = %d, want 5", actualConfig.MaxAttempts)
		}

		if actualConfig.Multiplier != 3.0 {
			t.Errorf("Multiplier = %f, want 3.0", actualConfig.Multiplier)
		}
	})

	t.Run("ignores nil config", func(t *testing.T) {
		originalConfig := &RetryConfig{
			MaxAttempts:     3,
			InitialInterval: 1 * time.Second,
			MaxInterval:     10 * time.Second,
			Multiplier:      2.0,
			MaxElapsedTime:  2 * time.Minute,
		}
		client.SetRetryConfig(originalConfig)

		client.SetRetryConfig(nil)

		client.mu.RLock()
		actualConfig := client.retryConfig
		client.mu.RUnlock()

		if actualConfig != originalConfig {
			t.Error("SetRetryConfig(nil) should not change the retry config")
		}
	})

	t.Run("thread-safe config setting", func(t *testing.T) {
		done := make(chan bool)

		// Writer goroutines
		for i := 0; i < 10; i++ {
			go func(idx int) {
				for j := 0; j < 10; j++ {
					config := &RetryConfig{
						MaxAttempts:     idx + 1,
						InitialInterval: time.Duration(idx+1) * time.Second,
						MaxInterval:     time.Duration(idx+10) * time.Second,
						Multiplier:      float64(idx + 1),
						MaxElapsedTime:  time.Duration(idx+5) * time.Minute,
					}
					client.SetRetryConfig(config)
				}
				done <- true
			}(i)
		}

		// Reader goroutines
		for i := 0; i < 10; i++ {
			go func() {
				for j := 0; j < 10; j++ {
					client.mu.RLock()
					_ = client.retryConfig
					client.mu.RUnlock()
				}
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 20; i++ {
			<-done
		}
		// If we reach here without deadlock/race, it's thread-safe
	})
}
