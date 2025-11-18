package remediation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// Mock executor for disk action testing
type diskMockExecutor struct {
	responses map[string]mockResponse
}

type mockResponse struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func (m *diskMockExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	return m.ExecuteWithContext(context.Background(), command)
}

func (m *diskMockExecutor) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	// Check for exact matches first
	if resp, ok := m.responses[command]; ok {
		return resp.stdout, resp.stderr, resp.exitCode, resp.err
	}

	// Check for pattern matches (for find commands with varying parameters)
	for pattern, resp := range m.responses {
		if strings.Contains(command, pattern) {
			return resp.stdout, resp.stderr, resp.exitCode, resp.err
		}
	}

	// Default response for unknown commands
	return "", "command not mocked", 127, nil
}

// Test CleanLogsAction

func TestCleanLogsAction_Validate(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("validates when /var/log is accessible", func(t *testing.T) {
		action := NewCleanLogsAction(30, false, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"test -d /var/log && test -r /var/log": {
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

	t.Run("fails when /var/log not accessible", func(t *testing.T) {
		action := NewCleanLogsAction(30, false, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"test -d /var/log && test -r /var/log": {
					stdout:   "",
					stderr:   "Permission denied",
					exitCode: 1,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err == nil {
			t.Error("Validate() expected error when /var/log not accessible, got nil")
		}

		if !strings.Contains(err.Error(), "/var/log not accessible") {
			t.Errorf("Validate() error = %q, want to contain '/var/log not accessible'", err.Error())
		}
	})

	t.Run("fails when executor is nil", func(t *testing.T) {
		action := NewCleanLogsAction(30, false, logger)

		err := action.Validate(context.Background(), nil)

		if err == nil {
			t.Error("Validate() expected error with nil executor, got nil")
		}

		if !strings.Contains(err.Error(), "executor is nil") {
			t.Errorf("Validate() error = %q, want to contain 'executor is nil'", err.Error())
		}
	})
}

func TestCleanLogsAction_Execute(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("successfully cleans old log files", func(t *testing.T) {
		action := NewCleanLogsAction(30, false, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"find /var/log -type f -name '*.log' -mtime +30": {
					stdout:   "/var/log/old1.log\n/var/log/old2.log\n/var/log/old3.log",
					exitCode: 0,
				},
				"find /var/log -type f -name '*.log' -mtime +30 -delete": {
					stdout:   "",
					exitCode: 0,
				},
				"df -h /var/log": {
					stdout:   "Filesystem      Size  Used Avail Use% Mounted on\n/dev/sda1        50G   30G   18G  63% /",
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

		if !result.Succeeded() {
			t.Error("Execute() result.Succeeded() = false, want true")
		}

		if !strings.Contains(result.Message, "deleted 3 old log files") {
			t.Errorf("Execute() message = %q, want to mention 3 files", result.Message)
		}

		if len(result.ChangesApplied) != 2 {
			t.Errorf("Execute() len(ChangesApplied) = %d, want 2", len(result.ChangesApplied))
		}
	})

	t.Run("handles no old log files found", func(t *testing.T) {
		action := NewCleanLogsAction(30, false, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"find /var/log -type f -name '*.log' -mtime +30": {
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

		if !strings.Contains(result.Message, "No old log files found") {
			t.Errorf("Execute() message = %q, want to mention no files found", result.Message)
		}
	})

	t.Run("dry-run mode simulates without deleting", func(t *testing.T) {
		action := NewCleanLogsAction(30, true, logger) // dryRun = true
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"find /var/log -type f -name '*.log' -mtime +30": {
					stdout:   "/var/log/old1.log\n/var/log/old2.log",
					exitCode: 0,
				},
				"df -h /var/log": {
					stdout:   "Filesystem      Size  Used Avail Use% Mounted on\n/dev/sda1        50G   30G   18G  63% /",
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

		if !strings.Contains(result.Message, "Would delete") {
			t.Errorf("Execute() message = %q, want to mention 'Would delete' for dry-run", result.Message)
		}

		if !strings.Contains(result.Message, "dry-run mode") {
			t.Errorf("Execute() message = %q, want to mention 'dry-run mode'", result.Message)
		}
	})

	t.Run("fails when find command fails", func(t *testing.T) {
		action := NewCleanLogsAction(30, false, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"find /var/log -type f -name '*.log' -mtime +30": {
					stdout:   "",
					stderr:   "Permission denied",
					exitCode: 1,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err == nil {
			t.Error("Execute() expected error when find fails, got nil")
		}

		if result.Status != StatusFailed {
			t.Errorf("Execute() result.Status = %q, want %q", result.Status, StatusFailed)
		}

		if !result.Failed() {
			t.Error("Execute() result.Failed() = false, want true")
		}
	})

	t.Run("fails when delete command fails", func(t *testing.T) {
		action := NewCleanLogsAction(30, false, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"find /var/log -type f -name '*.log' -mtime +30": {
					stdout:   "/var/log/old1.log",
					exitCode: 0,
				},
				"find /var/log -type f -name '*.log' -mtime +30 -delete": {
					stdout:   "",
					stderr:   "Permission denied",
					exitCode: 1,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err == nil {
			t.Error("Execute() expected error when delete fails, got nil")
		}

		if result.Status != StatusFailed {
			t.Errorf("Execute() result.Status = %q, want %q", result.Status, StatusFailed)
		}

		if !strings.Contains(result.Error, "Permission denied") {
			t.Errorf("Execute() result.Error = %q, want to contain 'Permission denied'", result.Error)
		}
	})

	t.Run("sets duration correctly", func(t *testing.T) {
		action := NewCleanLogsAction(30, false, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"find /var/log -type f -name '*.log' -mtime +30": {
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

		if result.StartTime.IsZero() {
			t.Error("Execute() result.StartTime is zero, want valid time")
		}

		if result.EndTime.IsZero() {
			t.Error("Execute() result.EndTime is zero, want valid time")
		}

		if !result.EndTime.After(result.StartTime) {
			t.Error("Execute() result.EndTime should be after StartTime")
		}
	})
}

func TestCleanLogsAction_Rollback(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("returns error indicating rollback not supported", func(t *testing.T) {
		action := NewCleanLogsAction(30, false, logger)
		executor := &diskMockExecutor{}
		result := &ActionResult{ActionID: "test"}

		err := action.Rollback(context.Background(), executor, result)

		if err == nil {
			t.Error("Rollback() expected error, got nil")
		}

		if !strings.Contains(err.Error(), "cannot be rolled back") {
			t.Errorf("Rollback() error = %q, want to mention 'cannot be rolled back'", err.Error())
		}
	})
}

func TestCleanLogsAction_GetDiskUsage(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("parses disk usage correctly", func(t *testing.T) {
		action := NewCleanLogsAction(30, false, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"df -h /var/log | tail -1 | awk '{print $5}'": {
					stdout:   "63%",
					exitCode: 0,
				},
			},
		}

		usage, err := action.getDiskUsage(context.Background(), executor)

		if err != nil {
			t.Errorf("getDiskUsage() unexpected error: %v", err)
		}

		if usage != "63%" {
			t.Errorf("getDiskUsage() = %q, want '63%%'", usage)
		}
	})

	t.Run("handles empty output gracefully", func(t *testing.T) {
		action := NewCleanLogsAction(30, false, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"df -h /var/log": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		usage, err := action.getDiskUsage(context.Background(), executor)

		if err != nil {
			t.Errorf("getDiskUsage() unexpected error: %v", err)
		}

		if usage != "" {
			t.Errorf("getDiskUsage() = %q, want empty string", usage)
		}
	})
}

// Test CleanTempAction

func TestCleanTempAction_Validate(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("validates when /tmp is accessible", func(t *testing.T) {
		action := NewCleanTempAction(7, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"test -d /tmp && test -w /tmp": {
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

	t.Run("fails when /tmp not accessible", func(t *testing.T) {
		action := NewCleanTempAction(7, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"test -d /tmp && test -w /tmp": {
					stdout:   "",
					stderr:   "Permission denied",
					exitCode: 1,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err == nil {
			t.Error("Validate() expected error when /tmp not accessible, got nil")
		}

		if !strings.Contains(err.Error(), "/tmp not accessible") {
			t.Errorf("Validate() error = %q, want to contain '/tmp not accessible'", err.Error())
		}
	})
}

func TestCleanTempAction_Execute(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("successfully cleans temp files", func(t *testing.T) {
		action := NewCleanTempAction(7, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"find /tmp -type f -mtime +7 2>/dev/null | wc -l": {
					stdout:   "2",
					exitCode: 0,
				},
				"find /tmp -type f -mtime +7 -delete 2>/dev/null": {
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

		if !strings.Contains(result.Message, "cleaned 2") {
			t.Errorf("Execute() message = %q, want to mention cleaned 2 files", result.Message)
		}
	})

	t.Run("handles no temp files found", func(t *testing.T) {
		action := NewCleanTempAction(7, logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"find /tmp -type f -mtime +7 2>/dev/null | wc -l": {
					stdout:   "0",
					exitCode: 0,
				},
				"find /tmp -type f -mtime +7 -delete 2>/dev/null": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		if !strings.Contains(result.Message, "cleaned 0") {
			t.Errorf("Execute() message = %q, want to mention cleaned 0 files", result.Message)
		}
	})
}

func TestCleanTempAction_Rollback(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("returns error indicating rollback not supported", func(t *testing.T) {
		action := NewCleanTempAction(7, logger)
		executor := &diskMockExecutor{}
		result := &ActionResult{ActionID: "test"}

		err := action.Rollback(context.Background(), executor, result)

		if err == nil {
			t.Error("Rollback() expected error, got nil")
		}

		if !strings.Contains(err.Error(), "cannot be rolled back") {
			t.Errorf("Rollback() error = %q, want to mention 'cannot be rolled back'", err.Error())
		}
	})
}

// Test CleanCacheAction

func TestCleanCacheAction_Validate(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("validates successfully", func(t *testing.T) {
		action := NewCleanCacheAction(logger)
		executor := &diskMockExecutor{}

		err := action.Validate(context.Background(), executor)

		if err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("fails when executor is nil", func(t *testing.T) {
		action := NewCleanCacheAction(logger)

		err := action.Validate(context.Background(), nil)

		if err == nil {
			t.Error("Validate() expected error with nil executor, got nil")
		}
	})
}

func TestCleanCacheAction_Execute(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("successfully cleans cache", func(t *testing.T) {
		action := NewCleanCacheAction(logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"find ~/.cache -type f 2>/dev/null | wc -l": {
					stdout:   "150",
					exitCode: 0,
				},
				"rm -rf ~/.cache/* 2>/dev/null": {
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

		if !strings.Contains(result.Message, "cleaned 150") {
			t.Errorf("Execute() message = %q, want to mention cleaned 150 files", result.Message)
		}
	})

	t.Run("handles zero cache files", func(t *testing.T) {
		action := NewCleanCacheAction(logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"find ~/.cache -type f 2>/dev/null | wc -l": {
					stdout:   "0",
					exitCode: 0,
				},
				"rm -rf ~/.cache/* 2>/dev/null": {
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
	})

	t.Run("fails when rm command fails", func(t *testing.T) {
		action := NewCleanCacheAction(logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"find ~/.cache -type f 2>/dev/null | wc -l": {
					stdout:   "50",
					exitCode: 0,
				},
				"rm -rf ~/.cache/* 2>/dev/null": {
					stdout:   "",
					stderr:   "Permission denied",
					exitCode: 1,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err == nil {
			t.Error("Execute() expected error when rm fails, got nil")
		}

		if result.Status != StatusFailed {
			t.Errorf("Execute() result.Status = %q, want %q", result.Status, StatusFailed)
		}
	})
}

func TestCleanCacheAction_Rollback(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("returns error indicating rollback not supported", func(t *testing.T) {
		action := NewCleanCacheAction(logger)
		executor := &diskMockExecutor{}
		result := &ActionResult{ActionID: "test"}

		err := action.Rollback(context.Background(), executor, result)

		if err == nil {
			t.Error("Rollback() expected error, got nil")
		}

		if !strings.Contains(err.Error(), "cannot be rolled back") {
			t.Errorf("Rollback() error = %q, want to mention 'cannot be rolled back'", err.Error())
		}
	})
}

// Test CleanAptCacheAction

func TestCleanAptCacheAction_Validate(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("validates when apt-get is available", func(t *testing.T) {
		action := NewCleanAptCacheAction(logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"which apt-get": {
					stdout:   "/usr/bin/apt-get",
					exitCode: 0,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("fails when apt-get not available", func(t *testing.T) {
		action := NewCleanAptCacheAction(logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"which apt-get": {
					stdout:   "",
					exitCode: 1,
				},
			},
		}

		err := action.Validate(context.Background(), executor)

		if err == nil {
			t.Error("Validate() expected error when apt-get not available, got nil")
		}

		if !strings.Contains(err.Error(), "apt-get not available") {
			t.Errorf("Validate() error = %q, want to contain 'apt-get not available'", err.Error())
		}
	})

	t.Run("fails when executor is nil", func(t *testing.T) {
		action := NewCleanAptCacheAction(logger)

		err := action.Validate(context.Background(), nil)

		if err == nil {
			t.Error("Validate() expected error with nil executor, got nil")
		}
	})
}

func TestCleanAptCacheAction_Execute(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("successfully cleans apt cache", func(t *testing.T) {
		action := NewCleanAptCacheAction(logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"du -sh /var/cache/apt/archives 2>/dev/null | awk '{print $1}'": {
					stdout:   "250M",
					exitCode: 0,
				},
				"apt-get clean": {
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

		if !strings.Contains(result.Message, "Successfully cleaned APT") {
			t.Errorf("Execute() message = %q, want to mention APT cache cleaned", result.Message)
		}

		if len(result.ChangesApplied) != 1 {
			t.Errorf("Execute() len(ChangesApplied) = %d, want 1", len(result.ChangesApplied))
		}

		if !strings.Contains(result.ChangesApplied[0], "250M") {
			t.Errorf("Execute() ChangesApplied[0] = %q, want to mention initial size 250M", result.ChangesApplied[0])
		}
	})

	t.Run("handles zero cache size", func(t *testing.T) {
		action := NewCleanAptCacheAction(logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"du -sh /var/cache/apt/archives 2>/dev/null | awk '{print $1}'": {
					stdout:   "0",
					exitCode: 0,
				},
				"apt-get clean": {
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
	})

	t.Run("fails when apt-get clean fails", func(t *testing.T) {
		action := NewCleanAptCacheAction(logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"du -sh /var/cache/apt/archives 2>/dev/null | awk '{print $1}'": {
					stdout:   "250M",
					exitCode: 0,
				},
				"apt-get clean": {
					stdout:   "",
					stderr:   "Permission denied",
					exitCode: 1,
				},
			},
		}

		result, err := action.Execute(context.Background(), executor)

		if err == nil {
			t.Error("Execute() expected error when apt-get clean fails, got nil")
		}

		if result.Status != StatusFailed {
			t.Errorf("Execute() result.Status = %q, want %q", result.Status, StatusFailed)
		}

		if !strings.Contains(result.Error, "Permission denied") {
			t.Errorf("Execute() result.Error = %q, want to contain 'Permission denied'", result.Error)
		}
	})

	t.Run("sets duration correctly", func(t *testing.T) {
		action := NewCleanAptCacheAction(logger)
		executor := &diskMockExecutor{
			responses: map[string]mockResponse{
				"du -sh /var/cache/apt/archives 2>/dev/null | awk '{print $1}'": {
					stdout:   "100M",
					exitCode: 0,
				},
				"apt-get clean": {
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

func TestCleanAptCacheAction_Rollback(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&diskTestLogWriter{t})

	t.Run("returns error indicating rollback not supported", func(t *testing.T) {
		action := NewCleanAptCacheAction(logger)
		executor := &diskMockExecutor{}
		result := &ActionResult{ActionID: "test"}

		err := action.Rollback(context.Background(), executor, result)

		if err == nil {
			t.Error("Rollback() expected error, got nil")
		}

		if !strings.Contains(err.Error(), "cannot be rolled back") {
			t.Errorf("Rollback() error = %q, want to mention 'cannot be rolled back'", err.Error())
		}
	})
}

// Test log writer for disk action tests
type diskTestLogWriter struct {
	t *testing.T
}

func (w *diskTestLogWriter) Write(p []byte) (n int, err error) {
	w.t.Log(string(p))
	return len(p), nil
}
