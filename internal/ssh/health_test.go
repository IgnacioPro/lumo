package ssh

import (
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestNewHealthChecker(t *testing.T) {
	t.Run("with valid client and logger", func(t *testing.T) {
		// We can't create a real SSH client easily, so we'll test with nil
		// but verify the structure is set up correctly
		logger := logrus.New()
		interval := 10 * time.Second

		hc := NewHealthChecker(nil, interval, logger)

		if hc == nil {
			t.Fatal("NewHealthChecker() returned nil")
		}

		if hc.interval != interval {
			t.Errorf("interval = %v, want %v", hc.interval, interval)
		}

		if hc.logger != logger {
			t.Error("logger not set correctly")
		}

		if hc.running {
			t.Error("running should be false initially")
		}

		if !hc.lastStatus {
			t.Error("lastStatus should be true initially")
		}

		if hc.ctx == nil {
			t.Error("context should be initialized")
		}

		if hc.cancel == nil {
			t.Error("cancel function should be initialized")
		}
	})

	t.Run("with nil logger", func(t *testing.T) {
		interval := 5 * time.Second

		hc := NewHealthChecker(nil, interval, nil)

		if hc == nil {
			t.Fatal("NewHealthChecker() returned nil")
		}

		if hc.logger == nil {
			t.Error("logger should be initialized with default when nil provided")
		}
	})
}

func TestHealthChecker_IsRunning(t *testing.T) {
	hc := NewHealthChecker(nil, 10*time.Second, logrus.New())

	// Initially not running
	if hc.IsRunning() {
		t.Error("IsRunning() = true, want false initially")
	}

	// Manually set running state
	hc.mu.Lock()
	hc.running = true
	hc.mu.Unlock()

	if !hc.IsRunning() {
		t.Error("IsRunning() = false, want true after setting running")
	}

	// Reset
	hc.mu.Lock()
	hc.running = false
	hc.mu.Unlock()

	if hc.IsRunning() {
		t.Error("IsRunning() = true, want false after reset")
	}
}

func TestHealthChecker_GetLastCheckTime(t *testing.T) {
	hc := NewHealthChecker(nil, 10*time.Second, logrus.New())

	// Initially zero time
	initialTime := hc.GetLastCheckTime()
	if !initialTime.IsZero() {
		t.Errorf("GetLastCheckTime() = %v, want zero time initially", initialTime)
	}

	// Set a specific time
	testTime := time.Now()
	hc.mu.Lock()
	hc.lastCheck = testTime
	hc.mu.Unlock()

	retrievedTime := hc.GetLastCheckTime()
	if !retrievedTime.Equal(testTime) {
		t.Errorf("GetLastCheckTime() = %v, want %v", retrievedTime, testTime)
	}
}

func TestHealthChecker_GetLastStatus(t *testing.T) {
	hc := NewHealthChecker(nil, 10*time.Second, logrus.New())

	// Initially true
	if !hc.GetLastStatus() {
		t.Error("GetLastStatus() = false, want true initially")
	}

	// Set to false
	hc.mu.Lock()
	hc.lastStatus = false
	hc.mu.Unlock()

	if hc.GetLastStatus() {
		t.Error("GetLastStatus() = true, want false after setting")
	}

	// Set back to true
	hc.mu.Lock()
	hc.lastStatus = true
	hc.mu.Unlock()

	if !hc.GetLastStatus() {
		t.Error("GetLastStatus() = false, want true after resetting")
	}
}

func TestHealthChecker_SetUnhealthyCallback(t *testing.T) {
	hc := NewHealthChecker(nil, 10*time.Second, logrus.New())

	// Verify callback is initially nil
	hc.mu.RLock()
	if hc.onUnhealthy != nil {
		t.Error("onUnhealthy should be nil initially")
	}
	hc.mu.RUnlock()

	// Set a callback
	called := false
	callback := func() {
		called = true
	}

	hc.SetUnhealthyCallback(callback)

	// Verify callback was set
	hc.mu.RLock()
	if hc.onUnhealthy == nil {
		t.Error("onUnhealthy should not be nil after SetUnhealthyCallback")
	}
	hc.mu.RUnlock()

	// Test that callback works (manually trigger it)
	hc.mu.RLock()
	cb := hc.onUnhealthy
	hc.mu.RUnlock()

	if cb != nil {
		cb()
	}

	if !called {
		t.Error("Callback was not executed")
	}
}

func TestHealthChecker_Start_Errors(t *testing.T) {
	t.Run("nil SSH client", func(t *testing.T) {
		hc := NewHealthChecker(nil, 10*time.Second, logrus.New())

		err := hc.Start()

		if err == nil {
			t.Error("Start() with nil client should return error")
		}

		if !strings.Contains(err.Error(), "SSH client is nil") {
			t.Errorf("Error should mention nil client, got: %v", err)
		}

		// Should not be running after error
		if hc.IsRunning() {
			t.Error("IsRunning() = true after failed Start()")
		}
	})

	t.Run("already running", func(t *testing.T) {
		hc := NewHealthChecker(nil, 10*time.Second, logrus.New())

		// Manually set running state
		hc.mu.Lock()
		hc.running = true
		hc.mu.Unlock()

		err := hc.Start()

		if err == nil {
			t.Error("Start() when already running should return error")
		}

		if !strings.Contains(err.Error(), "already running") {
			t.Errorf("Error should mention 'already running', got: %v", err)
		}
	})
}

func TestHealthChecker_Stop_NotRunning(t *testing.T) {
	hc := NewHealthChecker(nil, 10*time.Second, logrus.New())

	// Should not panic when stopping a health checker that's not running
	hc.Stop()

	// Verify it's still not running
	if hc.IsRunning() {
		t.Error("IsRunning() = true after Stop() on non-running checker")
	}
}

func TestHealthChecker_CheckHealth_NilClient(t *testing.T) {
	hc := NewHealthChecker(nil, 10*time.Second, logrus.New())

	err := hc.CheckHealth()

	if err == nil {
		t.Error("CheckHealth() with nil client should return error")
	}

	if !strings.Contains(err.Error(), "SSH client is nil") {
		t.Errorf("Error should mention nil client, got: %v", err)
	}
}

func TestSendKeepAlive_NilClient(t *testing.T) {
	err := SendKeepAlive(nil)

	if err == nil {
		t.Error("SendKeepAlive() with nil client should return error")
	}

	if !strings.Contains(err.Error(), "SSH client is nil") {
		t.Errorf("Error should mention nil client, got: %v", err)
	}
}

func TestSendKeepAliveWithTimeout_NilClient(t *testing.T) {
	err := SendKeepAliveWithTimeout(nil, 5*time.Second)

	if err == nil {
		t.Error("SendKeepAliveWithTimeout() with nil client should return error")
	}

	if !strings.Contains(err.Error(), "SSH client is nil") {
		t.Errorf("Error should mention nil client, got: %v", err)
	}
}

func TestPingServer_NilClient(t *testing.T) {
	err := PingServer(nil)

	if err == nil {
		t.Error("PingServer() with nil client should return error")
	}

	if !strings.Contains(err.Error(), "SSH client is nil") {
		t.Errorf("Error should mention nil client, got: %v", err)
	}
}

func TestPingServerWithTimeout_NilClient(t *testing.T) {
	err := PingServerWithTimeout(nil, 5*time.Second)

	if err == nil {
		t.Error("PingServerWithTimeout() with nil client should return error")
	}

	if !strings.Contains(err.Error(), "SSH client is nil") {
		t.Errorf("Error should mention nil client, got: %v", err)
	}
}

func TestWaitForConnection_NilChecker(t *testing.T) {
	err := WaitForConnection(nil, 5*time.Second)

	if err == nil {
		t.Error("WaitForConnection() with nil checker should return error")
	}

	if !strings.Contains(err.Error(), "health checker is nil") {
		t.Errorf("Error should mention nil checker, got: %v", err)
	}
}

func TestWaitForConnection_Timeout(t *testing.T) {
	// Create a health checker with nil client - CheckHealth will always fail
	hc := NewHealthChecker(nil, 10*time.Second, logrus.New())

	// Use a very short timeout to ensure the test completes quickly
	err := WaitForConnection(hc, 100*time.Millisecond)

	if err == nil {
		t.Error("WaitForConnection() should timeout when health check always fails")
	}

	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("Error should mention timeout, got: %v", err)
	}
}

func TestMonitorConnectionHealth_NilClient(t *testing.T) {
	// MonitorConnectionHealth doesn't validate client upfront,
	// but the goroutine will send false status when SendKeepAlive fails
	statusChan, cancel := MonitorConnectionHealth(nil, 50*time.Millisecond)
	defer cancel()

	// Wait for first status update
	select {
	case healthy := <-statusChan:
		if healthy {
			t.Error("MonitorConnectionHealth() with nil client should report unhealthy")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("MonitorConnectionHealth() should send status within timeout")
	}

	// Cancel and verify channel closes
	cancel()

	select {
	case _, ok := <-statusChan:
		if ok {
			t.Error("Status channel should be closed after cancel")
		}
	case <-time.After(200 * time.Millisecond):
		// Channel should close when context is cancelled
	}
}

func TestHealthChecker_UpdateStatus(t *testing.T) {
	hc := NewHealthChecker(nil, 10*time.Second, logrus.New())

	// Initial status is true
	if !hc.GetLastStatus() {
		t.Error("Initial status should be true")
	}

	// Record time before update
	timeBefore := time.Now()

	// Update to false
	hc.updateStatus(false)

	// Verify status changed
	if hc.GetLastStatus() {
		t.Error("Status should be false after updateStatus(false)")
	}

	// Verify lastCheck was updated
	lastCheck := hc.GetLastCheckTime()
	if lastCheck.Before(timeBefore) {
		t.Error("lastCheck should be updated to current time")
	}

	// Update back to true
	hc.updateStatus(true)

	if !hc.GetLastStatus() {
		t.Error("Status should be true after updateStatus(true)")
	}
}

func TestHealthChecker_GetHostFromClient_NilClient(t *testing.T) {
	hc := NewHealthChecker(nil, 10*time.Second, logrus.New())

	host := hc.getHostFromClient()

	if host != "unknown" {
		t.Errorf("getHostFromClient() with nil client = %q, want 'unknown'", host)
	}
}

func TestHealthChecker_ThreadSafety(t *testing.T) {
	hc := NewHealthChecker(nil, 10*time.Second, logrus.New())

	done := make(chan bool, 2)

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = hc.IsRunning()
			_ = hc.GetLastCheckTime()
			_ = hc.GetLastStatus()
		}
		done <- true
	}()

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			hc.SetUnhealthyCallback(func() {})
			hc.updateStatus(i%2 == 0)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done
	// If we reach here without deadlock or data race, thread safety works
}

// Note: mockSSHClient removed as it was unused
// If needed in the future, we would need to refactor to use interfaces
