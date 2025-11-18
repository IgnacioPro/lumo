package remediation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// Mock executor for process action testing
type processMockExecutor struct {
	responses map[string]mockProcessResponse
	callCount map[string]int
}

type mockProcessResponse struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func (m *processMockExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	return m.ExecuteWithContext(context.Background(), command)
}

func (m *processMockExecutor) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	if m.callCount == nil {
		m.callCount = make(map[string]int)
	}

	// Track call count for stateful mocking
	m.callCount[command]++

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

// Test KillProcessAction

func TestKillProcessAction_Validate(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&processTestLogWriter{t})

	t.Run("validates when process exists and we have permission", func(t *testing.T) {
		action := NewKillProcessAction(1234, "test-process", "SIGTERM", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"ps -p 1234 -o pid= 2>/dev/null": {
					stdout:   "1234",
					exitCode: 0,
				},
				"kill -0 1234 2>/dev/null": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("fails when process does not exist", func(t *testing.T) {
		action := NewKillProcessAction(9999, "nonexistent", "SIGTERM", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"ps -p 9999 -o pid= 2>/dev/null": {
					stdout:   "",
					exitCode: 1,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err == nil {
			t.Error("Validate() expected error when process does not exist, got nil")
		}

		if !strings.Contains(err.Error(), "process 9999 does not exist") {
			t.Errorf("Validate() error = %q, want to contain 'process 9999 does not exist'", err.Error())
		}
	})

	t.Run("fails when insufficient permissions", func(t *testing.T) {
		action := NewKillProcessAction(1234, "restricted", "SIGTERM", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"ps -p 1234 -o pid= 2>/dev/null": {
					stdout:   "1234",
					exitCode: 0,
				},
				"kill -0 1234 2>/dev/null": {
					stdout:   "",
					exitCode: 1,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err == nil {
			t.Error("Validate() expected error for insufficient permissions, got nil")
		}

		if !strings.Contains(err.Error(), "insufficient permissions to kill process 1234") {
			t.Errorf("Validate() error = %q, want to mention insufficient permissions", err.Error())
		}
	})

	t.Run("fails when executor is nil", func(t *testing.T) {
		action := NewKillProcessAction(1234, "test", "SIGTERM", logger)

		err := action.Validate(context.Background(), nil)

		if err == nil {
			t.Error("Validate() expected error with nil executor, got nil")
		}
	})
}

func TestKillProcessAction_Execute(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&processTestLogWriter{t})

	t.Run("successfully kills process", func(t *testing.T) {
		action := NewKillProcessAction(1234, "test-process", "SIGTERM", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"ps -p 1234 -o pid,ppid,user,%cpu,%mem,cmd 2>/dev/null": {
					stdout:   "PID  PPID USER     %CPU %MEM CMD\n1234 1    root     0.0  1.0 /usr/bin/test-process",
					exitCode: 0,
				},
				"kill -SIGTERM 1234": {
					stdout:   "",
					exitCode: 0,
				},
				"ps -p 1234 -o pid= 2>/dev/null": {
					stdout:   "",
					exitCode: 1,
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

		if !strings.Contains(result.Message, "Successfully killed process 1234") {
			t.Errorf("Execute() message = %q, want to mention successful kill", result.Message)
		}

		if len(result.ChangesApplied) != 2 {
			t.Errorf("Execute() len(ChangesApplied) = %d, want 2", len(result.ChangesApplied))
		}
	})

	t.Run("fails when kill command fails", func(t *testing.T) {
		action := NewKillProcessAction(1234, "test-process", "SIGTERM", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"ps -p 1234 -o pid,ppid,user,%cpu,%mem,cmd 2>/dev/null": {
					stdout:   "PID  PPID USER     %CPU %MEM CMD\n1234 1    root     0.0  1.0 /usr/bin/test-process",
					exitCode: 0,
				},
				"kill -SIGTERM 1234": {
					stdout:   "",
					stderr:   "Operation not permitted",
					exitCode: 1,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err == nil {
			t.Error("Execute() expected error when kill fails, got nil")
		}

		if result.Status != StatusFailed {
			t.Errorf("Execute() result.Status = %q, want %q", result.Status, StatusFailed)
		}

		if !strings.Contains(result.Error, "Operation not permitted") {
			t.Errorf("Execute() result.Error = %q, want to contain error message", result.Error)
		}
	})

	t.Run("fails when process still exists after kill", func(t *testing.T) {
		action := NewKillProcessAction(1234, "stubborn-process", "SIGTERM", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"ps -p 1234 -o pid,ppid,user,%cpu,%mem,cmd 2>/dev/null": {
					stdout:   "PID  PPID USER     %CPU %MEM CMD\n1234 1    root     0.0  1.0 /usr/bin/stubborn",
					exitCode: 0,
				},
				"kill -SIGTERM 1234": {
					stdout:   "",
					exitCode: 0,
				},
				"ps -p 1234 -o pid= 2>/dev/null": {
					stdout:   "1234",
					exitCode: 0,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err == nil {
			t.Error("Execute() expected error when process still exists, got nil")
		}

		if result.Status != StatusFailed {
			t.Errorf("Execute() result.Status = %q, want %q", result.Status, StatusFailed)
		}

		if !strings.Contains(result.Message, "still running") {
			t.Errorf("Execute() result.Message = %q, want to mention still running", result.Message)
		}
	})

	t.Run("sets duration correctly", func(t *testing.T) {
		action := NewKillProcessAction(1234, "test", "SIGKILL", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"ps -p 1234 -o pid,ppid,user,%cpu,%mem,cmd 2>/dev/null": {
					stdout:   "PID  PPID USER     %CPU %MEM CMD\n1234 1    root     0.0  1.0 /usr/bin/test",
					exitCode: 0,
				},
				"kill -SIGKILL 1234": {
					stdout:   "",
					exitCode: 0,
				},
				"ps -p 1234 -o pid= 2>/dev/null": {
					stdout:   "",
					exitCode: 1,
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

func TestKillProcessAction_Rollback(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&processTestLogWriter{t})

	t.Run("rollback returns error for non-reversible action", func(t *testing.T) {
		action := NewKillProcessAction(1234, "test", "SIGKILL", logger)
		executor := &processMockExecutor{}
		result := &ActionResult{ActionID: "test"}

		err := action.Rollback(context.Background(), executor, result)

		if err == nil {
			t.Error("Rollback() expected error for non-reversible action, got nil")
		}

		if !strings.Contains(err.Error(), "cannot be rolled back") {
			t.Errorf("Rollback() error = %q, want to mention cannot be rolled back", err.Error())
		}
	})
}

// Test KillProcessGracefulAction

func TestKillProcessGracefulAction_Validate(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&processTestLogWriter{t})

	t.Run("validates when process exists and we have permission", func(t *testing.T) {
		action := NewKillProcessGracefulAction(5678, "graceful-test", 5*time.Second, logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"ps -p 5678 -o pid= 2>/dev/null": {
					stdout:   "5678",
					exitCode: 0,
				},
				"kill -0 5678 2>/dev/null": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("fails when process does not exist", func(t *testing.T) {
		action := NewKillProcessGracefulAction(9999, "nonexistent", 5*time.Second, logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"ps -p 9999 -o pid= 2>/dev/null": {
					stdout:   "",
					exitCode: 1,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err == nil {
			t.Error("Validate() expected error when process does not exist, got nil")
		}

		if !strings.Contains(err.Error(), "process 9999 does not exist") {
			t.Errorf("Validate() error = %q, want to contain 'process 9999 does not exist'", err.Error())
		}
	})
}

func TestKillProcessGracefulAction_Execute(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&processTestLogWriter{t})

	t.Run("successfully kills process with SIGTERM", func(t *testing.T) {
		action := NewKillProcessGracefulAction(5678, "cooperative", 1*time.Second, logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"ps -p 5678 -o pid,ppid,user,%cpu,%mem,cmd 2>/dev/null": {
					stdout:   "PID  PPID USER     %CPU %MEM CMD\n5678 1    root     0.0  1.0 /usr/bin/cooperative",
					exitCode: 0,
				},
				"kill -TERM 5678": {
					stdout:   "",
					exitCode: 0,
				},
				"ps -p 5678 -o pid= 2>/dev/null": {
					stdout:   "",
					exitCode: 1,
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

		if !strings.Contains(result.Message, "gracefully exited after SIGTERM") {
			t.Errorf("Execute() message = %q, want to mention graceful exit", result.Message)
		}

		if !strings.Contains(result.ChangesApplied[0], "terminated gracefully with SIGTERM") {
			t.Errorf("Execute() ChangesApplied[0] = %q, want to mention SIGTERM", result.ChangesApplied[0])
		}
	})

	t.Run("falls back to SIGKILL when process doesn't respond to SIGTERM", func(t *testing.T) {
		action := NewKillProcessGracefulAction(5678, "stubborn", 500*time.Millisecond, logger)

		// Create stateful mock that changes behavior after first ps check
		callCount := 0
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"ps -p 5678 -o pid,ppid,user,%cpu,%mem,cmd 2>/dev/null": {
					stdout:   "PID  PPID USER     %CPU %MEM CMD\n5678 1    root     0.0  1.0 /usr/bin/stubborn",
					exitCode: 0,
				},
				"kill -TERM 5678": {
					stdout:   "",
					exitCode: 0,
				},
				"kill -KILL 5678": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		// Mock ps command to return process exists for waitForProcessExit checks
		originalResponses := make(map[string]mockProcessResponse)
		for k, v := range executor.responses {
			originalResponses[k] = v
		}
		executor.responses["ps -p 5678 -o pid= 2>/dev/null"] = mockProcessResponse{
			stdout:   "5678",
			exitCode: 0,
		}

		result, err := action.Execute(context.Background(), executor)
		_ = callCount

		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		if result.Status != StatusSuccess {
			t.Errorf("Execute() status = %q, want %q", result.Status, StatusSuccess)
		}

		if !strings.Contains(result.Message, "forcefully killed after timeout") {
			t.Errorf("Execute() message = %q, want to mention forceful kill", result.Message)
		}

		if !strings.Contains(result.ChangesApplied[0], "SIGKILL after SIGTERM timeout") {
			t.Errorf("Execute() ChangesApplied[0] = %q, want to mention SIGKILL fallback", result.ChangesApplied[0])
		}
	})

	t.Run("fails when SIGTERM fails", func(t *testing.T) {
		action := NewKillProcessGracefulAction(5678, "protected", 1*time.Second, logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"ps -p 5678 -o pid,ppid,user,%cpu,%mem,cmd 2>/dev/null": {
					stdout:   "PID  PPID USER     %CPU %MEM CMD\n5678 1    root     0.0  1.0 /usr/bin/protected",
					exitCode: 0,
				},
				"kill -TERM 5678": {
					stdout:   "",
					stderr:   "Operation not permitted",
					exitCode: 1,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err == nil {
			t.Error("Execute() expected error when SIGTERM fails, got nil")
		}

		if result.Status != StatusFailed {
			t.Errorf("Execute() result.Status = %q, want %q", result.Status, StatusFailed)
		}
	})
}

func TestKillProcessGracefulAction_Rollback(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&processTestLogWriter{t})

	t.Run("rollback returns error for non-reversible action", func(t *testing.T) {
		action := NewKillProcessGracefulAction(5678, "test", 5*time.Second, logger)
		executor := &processMockExecutor{}
		result := &ActionResult{ActionID: "test"}

		err := action.Rollback(context.Background(), executor, result)

		if err == nil {
			t.Error("Rollback() expected error for non-reversible action, got nil")
		}

		if !strings.Contains(err.Error(), "cannot be rolled back") {
			t.Errorf("Rollback() error = %q, want to mention cannot be rolled back", err.Error())
		}
	})
}

// Test KillProcessByNameAction

func TestKillProcessByNameAction_Validate(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&processTestLogWriter{t})

	t.Run("validates when matching processes found", func(t *testing.T) {
		action := NewKillProcessByNameAction("nginx", "SIGTERM", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"pgrep -f 'nginx'": {
					stdout:   "1234\n1235\n1236",
					exitCode: 0,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("fails when no matching processes found", func(t *testing.T) {
		action := NewKillProcessByNameAction("nonexistent-proc", "SIGTERM", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"pgrep -f 'nonexistent-proc'": {
					stdout:   "",
					exitCode: 1,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err == nil {
			t.Error("Validate() expected error when no processes found, got nil")
		}

		if !strings.Contains(err.Error(), "no processes found matching pattern") {
			t.Errorf("Validate() error = %q, want to mention no processes found", err.Error())
		}
	})
}

func TestKillProcessByNameAction_Execute(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&processTestLogWriter{t})

	t.Run("successfully kills all matching processes", func(t *testing.T) {
		action := NewKillProcessByNameAction("nginx", "SIGTERM", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"pgrep -f 'nginx'": {
					stdout:   "1234\n1235\n1236",
					exitCode: 0,
				},
				"kill -SIGTERM 1234 2>&1": {
					stdout:   "",
					exitCode: 0,
				},
				"kill -SIGTERM 1235 2>&1": {
					stdout:   "",
					exitCode: 0,
				},
				"kill -SIGTERM 1236 2>&1": {
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

		if !strings.Contains(result.Message, "Killed 3 out of 3") {
			t.Errorf("Execute() message = %q, want to mention killed 3 out of 3", result.Message)
		}

		if !strings.Contains(result.ChangesApplied[0], "Terminated 3 processes") {
			t.Errorf("Execute() ChangesApplied[0] = %q, want to mention 3 processes", result.ChangesApplied[0])
		}
	})

	t.Run("succeeds with partial kills", func(t *testing.T) {
		action := NewKillProcessByNameAction("test-proc", "SIGTERM", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"pgrep -f 'test-proc'": {
					stdout:   "1234\n1235\n1236",
					exitCode: 0,
				},
				"kill -SIGTERM 1234 2>&1": {
					stdout:   "",
					exitCode: 0,
				},
				"kill -SIGTERM 1235 2>&1": {
					stdout:   "",
					stderr:   "Operation not permitted",
					exitCode: 1,
				},
				"kill -SIGTERM 1236 2>&1": {
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

		if !strings.Contains(result.Message, "Killed 2 out of 3") {
			t.Errorf("Execute() message = %q, want to mention killed 2 out of 3", result.Message)
		}
	})

	t.Run("fails when no processes could be killed", func(t *testing.T) {
		action := NewKillProcessByNameAction("protected", "SIGTERM", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"pgrep -f 'protected'": {
					stdout:   "1234\n1235",
					exitCode: 0,
				},
				"kill -SIGTERM 1234 2>&1": {
					stdout:   "",
					stderr:   "Operation not permitted",
					exitCode: 1,
				},
				"kill -SIGTERM 1235 2>&1": {
					stdout:   "",
					stderr:   "Operation not permitted",
					exitCode: 1,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err == nil {
			t.Error("Execute() expected error when no processes killed, got nil")
		}

		if result.Status != StatusFailed {
			t.Errorf("Execute() result.Status = %q, want %q", result.Status, StatusFailed)
		}

		if !strings.Contains(result.Message, "Failed to kill any matching processes") {
			t.Errorf("Execute() message = %q, want to mention failed to kill any", result.Message)
		}
	})

	t.Run("succeeds when no matching processes found during execute", func(t *testing.T) {
		action := NewKillProcessByNameAction("ephemeral", "SIGTERM", logger)
		executor := &processMockExecutor{
			responses: map[string]mockProcessResponse{
				"pgrep -f 'ephemeral'": {
					stdout:   "",
					exitCode: 1,
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

		if !strings.Contains(result.Message, "No matching processes found") {
			t.Errorf("Execute() message = %q, want to mention no matching processes", result.Message)
		}
	})
}

func TestKillProcessByNameAction_Rollback(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&processTestLogWriter{t})

	t.Run("rollback returns error for non-reversible action", func(t *testing.T) {
		action := NewKillProcessByNameAction("test", "SIGTERM", logger)
		executor := &processMockExecutor{}
		result := &ActionResult{ActionID: "test"}

		err := action.Rollback(context.Background(), executor, result)

		if err == nil {
			t.Error("Rollback() expected error for non-reversible action, got nil")
		}

		if !strings.Contains(err.Error(), "cannot be rolled back") {
			t.Errorf("Rollback() error = %q, want to mention cannot be rolled back", err.Error())
		}
	})
}

// Test log writer for process action tests
type processTestLogWriter struct {
	t *testing.T
}

func (w *processTestLogWriter) Write(p []byte) (n int, err error) {
	w.t.Log(string(p))
	return len(p), nil
}
