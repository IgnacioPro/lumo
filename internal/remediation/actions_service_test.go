package remediation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// Mock executor for service action testing
type serviceMockExecutor struct {
	responses map[string]mockServiceResponse
}

type mockServiceResponse struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func (m *serviceMockExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	return m.ExecuteWithContext(context.Background(), command)
}

func (m *serviceMockExecutor) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	// Check for exact matches first
	if resp, ok := m.responses[command]; ok {
		return resp.stdout, resp.stderr, resp.exitCode, resp.err
	}

	// Check for pattern matches
	for pattern, resp := range m.responses {
		if strings.Contains(command, pattern) {
			return resp.stdout, resp.stderr, resp.exitCode, resp.err
		}
	}

	// Default response for unknown commands
	return "", "command not mocked", 127, nil
}

// Test RestartServiceAction

func TestRestartServiceAction_Validate(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&serviceTestLogWriter{t})

	t.Run("validates when systemctl and service available", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"which systemctl": {
					stdout:   "/usr/bin/systemctl",
					exitCode: 0,
				},
				"systemctl list-unit-files": {
					stdout:   "nginx.service                enabled",
					exitCode: 0,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("fails when systemctl not available", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"which systemctl": {
					stdout:   "",
					exitCode: 1,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err == nil {
			t.Error("Validate() expected error when systemctl not available, got nil")
		}

		if !strings.Contains(err.Error(), "systemctl not available") {
			t.Errorf("Validate() error = %q, want to contain 'systemctl not available'", err.Error())
		}
	})

	t.Run("fails when service not found", func(t *testing.T) {
		action := NewRestartServiceAction("nonexistent", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"which systemctl": {
					stdout:   "/usr/bin/systemctl",
					exitCode: 0,
				},
				"systemctl list-unit-files": {
					stdout:   "",
					exitCode: 1,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err == nil {
			t.Error("Validate() expected error when service not found, got nil")
		}

		if !strings.Contains(err.Error(), "service nonexistent not found") {
			t.Errorf("Validate() error = %q, want to contain 'service nonexistent not found'", err.Error())
		}
	})

	t.Run("fails when executor is nil", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)

		err := action.Validate(context.Background(), nil)

		if err == nil {
			t.Error("Validate() expected error with nil executor, got nil")
		}
	})
}

func TestRestartServiceAction_Execute(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&serviceTestLogWriter{t})

	t.Run("successfully restarts service", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl is-active nginx": {
					stdout:   "active",
					exitCode: 0,
				},
				"systemctl restart nginx": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		if result.Status != StatusSuccess {
			t.Errorf("Execute() status = %q, want %q", result.Status, StatusSuccess)
		}

		if !strings.Contains(result.Message, "Successfully restarted nginx") {
			t.Errorf("Execute() message = %q, want to mention successful restart", result.Message)
		}

		if len(result.ChangesApplied) != 2 {
			t.Errorf("Execute() len(ChangesApplied) = %d, want 2", len(result.ChangesApplied))
		}

		if result.RollbackData == nil {
			t.Error("Execute() RollbackData is nil, want non-nil for reversible action")
		}
	})

	t.Run("handles service restart from failed state", func(t *testing.T) {
		action := NewRestartServiceAction("mysql", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl is-active mysql": {
					stdout:   "failed",
					exitCode: 3,
				},
				"systemctl restart mysql": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		if result.Status != StatusSuccess {
			t.Errorf("Execute() status = %q, want %q", result.Status, StatusSuccess)
		}

		if !strings.Contains(result.ChangesApplied[1], "Status: failed → ") {
			t.Errorf("Execute() ChangesApplied should show state transition from failed, got: %v", result.ChangesApplied)
		}
	})

	t.Run("fails when restart command fails", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl is-active nginx": {
					stdout:   "active",
					exitCode: 0,
				},
				"systemctl restart nginx": {
					stdout:   "",
					stderr:   "Job for nginx.service failed",
					exitCode: 1,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err == nil {
			t.Error("Execute() expected error when restart fails, got nil")
		}

		if result.Status != StatusFailed {
			t.Errorf("Execute() result.Status = %q, want %q", result.Status, StatusFailed)
		}

		if !strings.Contains(result.Error, "Job for nginx.service failed") {
			t.Errorf("Execute() result.Error = %q, want to contain error message", result.Error)
		}
	})

	t.Run("sets duration correctly", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl is-active nginx": {
					stdout:   "active",
					exitCode: 0,
				},
				"systemctl restart nginx": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		if result.Duration == 0 {
			t.Error("Execute() result.Duration = 0, want > 0")
		}

		if result.StartTime.IsZero() || result.EndTime.IsZero() {
			t.Error("Execute() StartTime or EndTime is zero")
		}
	})
}

func TestRestartServiceAction_Rollback(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&serviceTestLogWriter{t})

	t.Run("stops service when previous status was inactive", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl stop nginx": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		result := &ActionResult{
			ActionID: "test",
			RollbackData: map[string]interface{}{
				"previous_status": "inactive",
				"service_name":    "nginx",
			},
		}

		err := action.Rollback(context.Background(), executor, result)

		if err != nil {
			t.Errorf("Rollback() unexpected error: %v", err)
		}
	})

	t.Run("stops service when previous status was failed", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl stop nginx": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		result := &ActionResult{
			ActionID: "test",
			RollbackData: map[string]interface{}{
				"previous_status": "failed",
				"service_name":    "nginx",
			},
		}

		err := action.Rollback(context.Background(), executor, result)

		if err != nil {
			t.Errorf("Rollback() unexpected error: %v", err)
		}
	})

	t.Run("does not stop service when previous status was active", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{},
		}

		result := &ActionResult{
			ActionID: "test",
			RollbackData: map[string]interface{}{
				"previous_status": "active",
				"service_name":    "nginx",
			},
		}

		err := action.Rollback(context.Background(), executor, result)

		if err != nil {
			t.Errorf("Rollback() unexpected error: %v", err)
		}
	})

	t.Run("fails when no rollback data available", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		executor := &serviceMockExecutor{}
		result := &ActionResult{ActionID: "test"}

		err := action.Rollback(context.Background(), executor, result)

		if err == nil {
			t.Error("Rollback() expected error when no rollback data, got nil")
		}

		if !strings.Contains(err.Error(), "no rollback data") {
			t.Errorf("Rollback() error = %q, want to mention 'no rollback data'", err.Error())
		}
	})

	t.Run("fails when stop command fails during rollback", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl stop nginx": {
					stdout:   "",
					stderr:   "Permission denied",
					exitCode: 1,
				},
			},
		}

		result := &ActionResult{
			ActionID: "test",
			RollbackData: map[string]interface{}{
				"previous_status": "inactive",
				"service_name":    "nginx",
			},
		}

		err := action.Rollback(context.Background(), executor, result)

		if err == nil {
			t.Error("Rollback() expected error when stop fails, got nil")
		}

		if !strings.Contains(err.Error(), "failed to stop service during rollback") {
			t.Errorf("Rollback() error = %q, want to mention stop failure", err.Error())
		}
	})
}

func TestRestartServiceAction_GetServiceStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&serviceTestLogWriter{t})

	t.Run("returns service status", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl is-active nginx": {
					stdout:   "active\n",
					exitCode: 0,
				},
			},
		}

		status, err := action.getServiceStatus(context.Background(), executor)

		if err != nil {
			t.Errorf("getServiceStatus() unexpected error: %v", err)
		}

		if status != "active" {
			t.Errorf("getServiceStatus() = %q, want 'active'", status)
		}
	})

	t.Run("handles failed service status", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl is-active nginx": {
					stdout:   "failed",
					exitCode: 3,
				},
			},
		}

		status, err := action.getServiceStatus(context.Background(), executor)

		if err != nil {
			t.Errorf("getServiceStatus() unexpected error: %v", err)
		}

		if status != "failed" {
			t.Errorf("getServiceStatus() = %q, want 'failed'", status)
		}
	})
}

// Test StartServiceAction

func TestStartServiceAction_Validate(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&serviceTestLogWriter{t})

	t.Run("validates when systemctl and service available", func(t *testing.T) {
		action := NewStartServiceAction("postgresql", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"which systemctl": {
					stdout:   "/usr/bin/systemctl",
					exitCode: 0,
				},
				"systemctl list-unit-files": {
					stdout:   "postgresql.service                enabled",
					exitCode: 0,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("fails when systemctl not available", func(t *testing.T) {
		action := NewStartServiceAction("postgresql", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"which systemctl": {
					stdout:   "",
					exitCode: 1,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err == nil {
			t.Error("Validate() expected error when systemctl not available, got nil")
		}

		if !strings.Contains(err.Error(), "systemctl not available") {
			t.Errorf("Validate() error = %q, want to contain 'systemctl not available'", err.Error())
		}
	})

	t.Run("fails when executor is nil", func(t *testing.T) {
		action := NewStartServiceAction("postgresql", logger)

		err := action.Validate(context.Background(), nil)

		if err == nil {
			t.Error("Validate() expected error with nil executor, got nil")
		}
	})
}

func TestStartServiceAction_Execute(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&serviceTestLogWriter{t})

	t.Run("successfully starts service", func(t *testing.T) {
		action := NewStartServiceAction("postgresql", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl start postgresql": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		if result.Status != StatusSuccess {
			t.Errorf("Execute() status = %q, want %q", result.Status, StatusSuccess)
		}

		if !strings.Contains(result.Message, "Successfully started postgresql") {
			t.Errorf("Execute() message = %q, want to mention successful start", result.Message)
		}

		if len(result.ChangesApplied) != 1 {
			t.Errorf("Execute() len(ChangesApplied) = %d, want 1", len(result.ChangesApplied))
		}

		if result.RollbackData == nil {
			t.Error("Execute() RollbackData is nil, want non-nil for reversible action")
		}
	})

	t.Run("fails when start command fails", func(t *testing.T) {
		action := NewStartServiceAction("postgresql", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl start postgresql": {
					stdout:   "",
					stderr:   "Job for postgresql.service failed",
					exitCode: 1,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err == nil {
			t.Error("Execute() expected error when start fails, got nil")
		}

		if result.Status != StatusFailed {
			t.Errorf("Execute() result.Status = %q, want %q", result.Status, StatusFailed)
		}

		if !strings.Contains(result.Error, "Job for postgresql.service failed") {
			t.Errorf("Execute() result.Error = %q, want to contain error message", result.Error)
		}
	})

	t.Run("sets duration correctly", func(t *testing.T) {
		action := NewStartServiceAction("postgresql", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl start postgresql": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		if result.Duration == 0 {
			t.Error("Execute() result.Duration = 0, want > 0")
		}

		if result.StartTime.IsZero() || result.EndTime.IsZero() {
			t.Error("Execute() StartTime or EndTime is zero")
		}
	})
}

func TestStartServiceAction_Rollback(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&serviceTestLogWriter{t})

	t.Run("stops service during rollback", func(t *testing.T) {
		action := NewStartServiceAction("postgresql", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl stop postgresql": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		result := &ActionResult{
			ActionID: "test",
			RollbackData: map[string]interface{}{
				"service_name": "postgresql",
			},
		}

		err := action.Rollback(context.Background(), executor, result)

		if err != nil {
			t.Errorf("Rollback() unexpected error: %v", err)
		}
	})

	t.Run("fails when stop command fails during rollback", func(t *testing.T) {
		action := NewStartServiceAction("postgresql", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl stop postgresql": {
					stdout:   "",
					stderr:   "Permission denied",
					exitCode: 1,
				},
			},
		}

		result := &ActionResult{
			ActionID: "test",
			RollbackData: map[string]interface{}{
				"service_name": "postgresql",
			},
		}

		err := action.Rollback(context.Background(), executor, result)

		if err == nil {
			t.Error("Rollback() expected error when stop fails, got nil")
		}

		if !strings.Contains(err.Error(), "failed to stop service during rollback") {
			t.Errorf("Rollback() error = %q, want to mention stop failure", err.Error())
		}
	})
}

// Test StopServiceAction

func TestStopServiceAction_Validate(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&serviceTestLogWriter{t})

	t.Run("validates when systemctl available", func(t *testing.T) {
		action := NewStopServiceAction("apache2", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"which systemctl": {
					stdout:   "/usr/bin/systemctl",
					exitCode: 0,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("fails when systemctl not available", func(t *testing.T) {
		action := NewStopServiceAction("apache2", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"which systemctl": {
					stdout:   "",
					exitCode: 1,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err == nil {
			t.Error("Validate() expected error when systemctl not available, got nil")
		}

		if !strings.Contains(err.Error(), "systemctl not available") {
			t.Errorf("Validate() error = %q, want to contain 'systemctl not available'", err.Error())
		}
	})

	t.Run("fails when executor is nil", func(t *testing.T) {
		action := NewStopServiceAction("apache2", logger)

		err := action.Validate(context.Background(), nil)

		if err == nil {
			t.Error("Validate() expected error with nil executor, got nil")
		}
	})
}

func TestStopServiceAction_Execute(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&serviceTestLogWriter{t})

	t.Run("successfully stops service", func(t *testing.T) {
		action := NewStopServiceAction("apache2", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl stop apache2": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		if result.Status != StatusSuccess {
			t.Errorf("Execute() status = %q, want %q", result.Status, StatusSuccess)
		}

		if !strings.Contains(result.Message, "Successfully stopped apache2") {
			t.Errorf("Execute() message = %q, want to mention successful stop", result.Message)
		}

		if len(result.ChangesApplied) != 1 {
			t.Errorf("Execute() len(ChangesApplied) = %d, want 1", len(result.ChangesApplied))
		}

		if result.RollbackData == nil {
			t.Error("Execute() RollbackData is nil, want non-nil for reversible action")
		}
	})

	t.Run("fails when stop command fails", func(t *testing.T) {
		action := NewStopServiceAction("apache2", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl stop apache2": {
					stdout:   "",
					stderr:   "Failed to stop apache2.service",
					exitCode: 1,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err == nil {
			t.Error("Execute() expected error when stop fails, got nil")
		}

		if result.Status != StatusFailed {
			t.Errorf("Execute() result.Status = %q, want %q", result.Status, StatusFailed)
		}

		if !strings.Contains(result.Error, "Failed to stop apache2.service") {
			t.Errorf("Execute() result.Error = %q, want to contain error message", result.Error)
		}
	})

	t.Run("sets duration correctly", func(t *testing.T) {
		action := NewStopServiceAction("apache2", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl stop apache2": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		if result.Duration == 0 {
			t.Error("Execute() result.Duration = 0, want > 0")
		}

		if result.StartTime.IsZero() || result.EndTime.IsZero() {
			t.Error("Execute() StartTime or EndTime is zero")
		}
	})
}

func TestStopServiceAction_Rollback(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&serviceTestLogWriter{t})

	t.Run("starts service during rollback", func(t *testing.T) {
		action := NewStopServiceAction("apache2", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl start apache2": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		result := &ActionResult{
			ActionID: "test",
			RollbackData: map[string]interface{}{
				"service_name": "apache2",
			},
		}

		err := action.Rollback(context.Background(), executor, result)

		if err != nil {
			t.Errorf("Rollback() unexpected error: %v", err)
		}
	})

	t.Run("fails when start command fails during rollback", func(t *testing.T) {
		action := NewStopServiceAction("apache2", logger)
		executor := &serviceMockExecutor{
			responses: map[string]mockServiceResponse{
				"systemctl start apache2": {
					stdout:   "",
					stderr:   "Permission denied",
					exitCode: 1,
				},
			},
		}

		result := &ActionResult{
			ActionID: "test",
			RollbackData: map[string]interface{}{
				"service_name": "apache2",
			},
		}

		err := action.Rollback(context.Background(), executor, result)

		if err == nil {
			t.Error("Rollback() expected error when start fails, got nil")
		}

		if !strings.Contains(err.Error(), "failed to start service during rollback") {
			t.Errorf("Rollback() error = %q, want to mention start failure", err.Error())
		}
	})
}

// Test log writer for service action tests
type serviceTestLogWriter struct {
	t *testing.T
}

func (w *serviceTestLogWriter) Write(p []byte) (n int, err error) {
	w.t.Log(string(p))
	return len(p), nil
}
