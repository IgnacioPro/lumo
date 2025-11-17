package ssh

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// HealthChecker manages connection health monitoring and keep-alive
type HealthChecker struct {
	client   *ssh.Client
	interval time.Duration
	logger   *logrus.Logger

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex
	running    bool
	lastCheck  time.Time
	lastStatus bool

	// Callback for when connection becomes unhealthy
	onUnhealthy func()
}

// NewHealthChecker creates a new health checker for an SSH client
func NewHealthChecker(client *ssh.Client, interval time.Duration, logger *logrus.Logger) *HealthChecker {
	if logger == nil {
		logger = logrus.New()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &HealthChecker{
		client:     client,
		interval:   interval,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		running:    false,
		lastStatus: true,
	}
}

// Start begins the health check monitoring in a background goroutine
func (h *HealthChecker) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return fmt.Errorf("health checker is already running")
	}

	if h.client == nil {
		return fmt.Errorf("SSH client is nil")
	}

	h.running = true
	h.wg.Add(1)

	go h.runHealthChecks()

	h.logger.Debugf("Health checker started with interval %v", h.interval)
	return nil
}

// Stop gracefully stops the health check monitoring
func (h *HealthChecker) Stop() {
	h.mu.Lock()
	if !h.running {
		h.mu.Unlock()
		return
	}
	h.running = false
	h.mu.Unlock()

	h.cancel()
	h.wg.Wait()

	h.logger.Debug("Health checker stopped")
}

// IsRunning returns true if the health checker is currently running
func (h *HealthChecker) IsRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}

// GetLastCheckTime returns the time of the last health check
func (h *HealthChecker) GetLastCheckTime() time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastCheck
}

// GetLastStatus returns the status of the last health check
func (h *HealthChecker) GetLastStatus() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastStatus
}

// SetUnhealthyCallback sets a callback function to be called when connection becomes unhealthy
func (h *HealthChecker) SetUnhealthyCallback(callback func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onUnhealthy = callback
}

// CheckHealth performs a manual health check
func (h *HealthChecker) CheckHealth() error {
	if h.client == nil {
		return fmt.Errorf("SSH client is nil")
	}

	// Send a keep-alive request
	_, _, err := h.client.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil {
		h.updateStatus(false)
		return &HealthCheckError{
			Host:    h.getHostFromClient(),
			Message: "keep-alive request failed",
			Err:     err,
		}
	}

	h.updateStatus(true)
	return nil
}

// runHealthChecks is the main loop for periodic health checks
func (h *HealthChecker) runHealthChecks() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	h.logger.Debug("Health check loop started")

	for {
		select {
		case <-h.ctx.Done():
			h.logger.Debug("Health check loop stopped due to context cancellation")
			return

		case <-ticker.C:
			h.performHealthCheck()
		}
	}
}

// performHealthCheck executes a single health check
func (h *HealthChecker) performHealthCheck() {
	h.logger.Debug("Performing health check...")

	err := h.CheckHealth()

	h.mu.Lock()
	h.lastCheck = time.Now()
	wasHealthy := h.lastStatus
	h.mu.Unlock()

	if err != nil {
		h.logger.Warnf("Health check failed: %v", err)

		// If connection was healthy before and now unhealthy, trigger callback
		if wasHealthy {
			h.logger.Warn("Connection became unhealthy")
			h.mu.RLock()
			callback := h.onUnhealthy
			h.mu.RUnlock()

			if callback != nil {
				go callback()
			}
		}
	} else {
		if !wasHealthy {
			h.logger.Info("Connection became healthy")
		}
		h.logger.Debug("Health check passed")
	}
}

// updateStatus updates the last check status
func (h *HealthChecker) updateStatus(healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastStatus = healthy
	h.lastCheck = time.Now()
}

// getHostFromClient extracts host information from the SSH client (best effort)
func (h *HealthChecker) getHostFromClient() string {
	if h.client == nil {
		return "unknown"
	}

	conn := h.client.Conn
	if conn == nil {
		return "unknown"
	}

	return conn.RemoteAddr().String()
}

// SendKeepAlive sends a single keep-alive packet
func SendKeepAlive(client *ssh.Client) error {
	if client == nil {
		return fmt.Errorf("SSH client is nil")
	}

	_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
	return err
}

// SendKeepAliveWithTimeout sends a keep-alive with a timeout
func SendKeepAliveWithTimeout(client *ssh.Client, timeout time.Duration) error {
	if client == nil {
		return fmt.Errorf("SSH client is nil")
	}

	done := make(chan error, 1)

	go func() {
		_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("keep-alive request timed out after %v", timeout)
	}
}

// PingServer sends a simple command to verify the connection is alive
func PingServer(client *ssh.Client) error {
	if client == nil {
		return fmt.Errorf("SSH client is nil")
	}

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session for ping: %w", err)
	}
	defer func() {
		_ = session.Close()
	}()

	// Run a simple command that should always succeed
	if err := session.Run("true"); err != nil {
		return fmt.Errorf("ping command failed: %w", err)
	}

	return nil
}

// PingServerWithTimeout pings the server with a timeout
func PingServerWithTimeout(client *ssh.Client, timeout time.Duration) error {
	if client == nil {
		return fmt.Errorf("SSH client is nil")
	}

	done := make(chan error, 1)

	go func() {
		done <- PingServer(client)
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("ping timed out after %v", timeout)
	}
}

// WaitForConnection waits for a connection to become healthy with timeout
func WaitForConnection(checker *HealthChecker, timeout time.Duration) error {
	if checker == nil {
		return fmt.Errorf("health checker is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for connection to become healthy")

		case <-ticker.C:
			if err := checker.CheckHealth(); err == nil {
				return nil
			}
		}
	}
}

// MonitorConnectionHealth monitors connection health and returns a channel for status updates
func MonitorConnectionHealth(client *ssh.Client, interval time.Duration) (<-chan bool, func()) {
	statusChan := make(chan bool, 1)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer close(statusChan)

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				err := SendKeepAlive(client)
				healthy := err == nil

				select {
				case statusChan <- healthy:
				default:
					// Channel is full, skip this update
				}
			}
		}
	}()

	return statusChan, cancel
}
