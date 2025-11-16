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
