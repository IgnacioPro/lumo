package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// Mock checker for testing
type mockChecker struct {
	name         string
	category     CheckCategory
	description  string
	requiresRoot bool
	runFunc      func(ctx context.Context, executor CommandExecutor) (*CheckResult, error)
	callCount    int
	mu           sync.Mutex
}

func (m *mockChecker) Name() string {
	return m.name
}

func (m *mockChecker) Category() CheckCategory {
	return m.category
}

func (m *mockChecker) Description() string {
	return m.description
}

func (m *mockChecker) RequiresRoot() bool {
	return m.requiresRoot
}

func (m *mockChecker) Run(ctx context.Context, executor CommandExecutor) (*CheckResult, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	if m.runFunc != nil {
		return m.runFunc(ctx, executor)
	}

	return &CheckResult{
		Name:      m.name,
		Category:  m.category,
		Status:    StatusCompleted,
		Severity:  SeverityOK,
		Message:   "Check passed",
		Timestamp: time.Now(),
	}, nil
}

func (m *mockChecker) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// Mock executor for testing
type mockExecutor struct {
	executeFunc func(command string, timeout time.Duration) (string, string, int, error)
}

func (m *mockExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	if m.executeFunc != nil {
		return m.executeFunc(command, timeout)
	}
	return "mock output", "", 0, nil
}

func (m *mockExecutor) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	if m.executeFunc != nil {
		return m.executeFunc(command, 30*time.Second)
	}
	return "mock output", "", 0, nil
}

func TestNewRunner(t *testing.T) {
	executor := &mockExecutor{}
	logger := logrus.New()

	t.Run("with all parameters", func(t *testing.T) {
		config := DefaultConfig()
		thresholds := DefaultThresholds()

		runner := NewRunner(config, thresholds, executor, logger)

		if runner == nil {
			t.Fatal("NewRunner() returned nil")
		}

		if runner.config != config {
			t.Error("Config not set correctly")
		}

		if runner.thresholds != thresholds {
			t.Error("Thresholds not set correctly")
		}

		if runner.executor != executor {
			t.Error("Executor not set correctly")
		}

		if runner.logger != logger {
			t.Error("Logger not set correctly")
		}

		if len(runner.checks) != 0 {
			t.Errorf("Expected 0 checks, got %d", len(runner.checks))
		}
	})

	t.Run("with nil config", func(t *testing.T) {
		runner := NewRunner(nil, nil, executor, logger)

		if runner == nil {
			t.Fatal("NewRunner() returned nil")
		}

		if runner.config == nil {
			t.Error("Config should be set to default")
		}

		if !runner.config.Parallel {
			t.Error("Default config should enable parallel execution")
		}
	})

	t.Run("with nil logger", func(t *testing.T) {
		config := DefaultConfig()
		runner := NewRunner(config, nil, executor, nil)

		if runner == nil {
			t.Fatal("NewRunner() returned nil")
		}

		if runner.logger == nil {
			t.Error("Logger should be initialized")
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if !config.Parallel {
		t.Error("Parallel should be true by default")
	}

	if config.CheckTimeout != 30*time.Second {
		t.Errorf("CheckTimeout = %v, want 30s", config.CheckTimeout)
	}

	if config.MaxConcurrent != 4 {
		t.Errorf("MaxConcurrent = %d, want 4", config.MaxConcurrent)
	}

	if !config.RetryFailedChecks {
		t.Error("RetryFailedChecks should be true by default")
	}

	if !config.Platform.AutoDetect {
		t.Error("Platform.AutoDetect should be true by default")
	}

	if config.Platform.ForceOS != "" {
		t.Errorf("Platform.ForceOS should be empty, got %q", config.Platform.ForceOS)
	}

	if len(config.EnabledChecks) != 0 {
		t.Errorf("EnabledChecks should be empty, got %d items", len(config.EnabledChecks))
	}
}

func TestRunner_RegisterChecker(t *testing.T) {
	runner := NewRunner(DefaultConfig(), nil, &mockExecutor{}, logrus.New())

	checker1 := &mockChecker{
		name:     "test_check_1",
		category: CategoryCPU,
	}

	checker2 := &mockChecker{
		name:     "test_check_2",
		category: CategoryMemory,
	}

	t.Run("register single checker", func(t *testing.T) {
		runner.RegisterChecker(checker1)

		if len(runner.checks) != 1 {
			t.Errorf("Expected 1 checker, got %d", len(runner.checks))
		}
	})

	t.Run("register multiple checkers", func(t *testing.T) {
		runner.RegisterChecker(checker2)

		if len(runner.checks) != 2 {
			t.Errorf("Expected 2 checkers, got %d", len(runner.checks))
		}
	})
}

func TestRunner_RegisterCheckers(t *testing.T) {
	runner := NewRunner(DefaultConfig(), nil, &mockExecutor{}, logrus.New())

	checkers := []Checker{
		&mockChecker{name: "check1", category: CategoryCPU},
		&mockChecker{name: "check2", category: CategoryMemory},
		&mockChecker{name: "check3", category: CategoryDisk},
	}

	runner.RegisterCheckers(checkers...)

	if len(runner.checks) != 3 {
		t.Errorf("Expected 3 checkers, got %d", len(runner.checks))
	}
}

func TestRunner_RunAll_Sequential(t *testing.T) {
	config := DefaultConfig()
	config.Parallel = false // Force sequential execution

	executor := &mockExecutor{}
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce test output noise

	runner := NewRunner(config, DefaultThresholds(), executor, logger)

	// Register test checkers
	checker1 := &mockChecker{
		name:     "cpu_check",
		category: CategoryCPU,
		runFunc: func(ctx context.Context, executor CommandExecutor) (*CheckResult, error) {
			return &CheckResult{
				Name:      "cpu_check",
				Category:  CategoryCPU,
				Status:    StatusCompleted,
				Severity:  SeverityOK,
				Message:   "CPU OK",
				Timestamp: time.Now(),
			}, nil
		},
	}

	checker2 := &mockChecker{
		name:     "memory_check",
		category: CategoryMemory,
		runFunc: func(ctx context.Context, executor CommandExecutor) (*CheckResult, error) {
			return &CheckResult{
				Name:      "memory_check",
				Category:  CategoryMemory,
				Status:    StatusCompleted,
				Severity:  SeverityWarning,
				Message:   "Memory high",
				Timestamp: time.Now(),
			}, nil
		},
	}

	runner.RegisterCheckers(checker1, checker2)

	ctx := context.Background()
	report, err := runner.RunAll(ctx)

	if err != nil {
		t.Fatalf("RunAll() error = %v, want nil", err)
	}

	if report == nil {
		t.Fatal("RunAll() returned nil report")
	}

	if len(report.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(report.Results))
	}

	if report.Summary.TotalChecks != 2 {
		t.Errorf("Summary.TotalChecks = %d, want 2", report.Summary.TotalChecks)
	}

	if report.Summary.OKCount != 1 {
		t.Errorf("Summary.OKCount = %d, want 1", report.Summary.OKCount)
	}

	if report.Summary.WarningCount != 1 {
		t.Errorf("Summary.WarningCount = %d, want 1", report.Summary.WarningCount)
	}
}

func TestRunner_RunAll_Parallel(t *testing.T) {
	config := DefaultConfig()
	config.Parallel = true
	config.MaxConcurrent = 2

	executor := &mockExecutor{}
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	runner := NewRunner(config, DefaultThresholds(), executor, logger)

	// Register multiple checkers
	for i := 0; i < 5; i++ {
		checker := &mockChecker{
			name:     fmt.Sprintf("check_%d", i),
			category: CategoryCPU,
			runFunc: func(ctx context.Context, executor CommandExecutor) (*CheckResult, error) {
				time.Sleep(10 * time.Millisecond) // Simulate work
				return &CheckResult{
					Name:      "test_check",
					Category:  CategoryCPU,
					Status:    StatusCompleted,
					Severity:  SeverityOK,
					Message:   "Check OK",
					Timestamp: time.Now(),
				}, nil
			},
		}
		runner.RegisterChecker(checker)
	}

	ctx := context.Background()
	report, err := runner.RunAll(ctx)

	if err != nil {
		t.Fatalf("RunAll() error = %v, want nil", err)
	}

	if len(report.Results) != 5 {
		t.Errorf("Expected 5 results, got %d", len(report.Results))
	}

	if report.Summary.TotalChecks != 5 {
		t.Errorf("Summary.TotalChecks = %d, want 5", report.Summary.TotalChecks)
	}

	// Parallel execution should be faster than sequential
	// With 5 checks at 10ms each, parallel (max 2) should take ~30ms
	// Sequential would take ~50ms
	if report.Duration > 45*time.Millisecond {
		t.Errorf("Parallel execution took too long: %v", report.Duration)
	}
}

func TestRunner_RunAll_NoChecks(t *testing.T) {
	runner := NewRunner(DefaultConfig(), nil, &mockExecutor{}, logrus.New())

	ctx := context.Background()
	report, err := runner.RunAll(ctx)

	if err == nil {
		t.Error("RunAll() error = nil, want error for no checks")
	}

	if report != nil {
		t.Error("RunAll() report should be nil when error occurs")
	}

	if err.Error() != "no checks registered" {
		t.Errorf("Error message = %q, want 'no checks registered'", err.Error())
	}
}

func TestRunner_RunAll_WithFailure(t *testing.T) {
	config := DefaultConfig()
	config.RetryFailedChecks = false // Disable retry for this test

	executor := &mockExecutor{}
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	runner := NewRunner(config, DefaultThresholds(), executor, logger)

	failingChecker := &mockChecker{
		name:     "failing_check",
		category: CategoryCPU,
		runFunc: func(ctx context.Context, executor CommandExecutor) (*CheckResult, error) {
			return nil, errors.New("check execution failed")
		},
	}

	runner.RegisterChecker(failingChecker)

	ctx := context.Background()
	report, err := runner.RunAll(ctx)

	if err != nil {
		t.Fatalf("RunAll() error = %v, want nil (errors in checks don't fail RunAll)", err)
	}

	if len(report.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(report.Results))
	}

	result := report.Results[0]
	if result.Status != StatusFailed {
		t.Errorf("Result.Status = %v, want %v", result.Status, StatusFailed)
	}

	if result.Severity != SeverityError {
		t.Errorf("Result.Severity = %v, want %v", result.Severity, SeverityError)
	}

	if report.Summary.ErrorCount != 1 {
		t.Errorf("Summary.ErrorCount = %d, want 1", report.Summary.ErrorCount)
	}
}

func TestRunner_RunAll_WithRetry(t *testing.T) {
	config := DefaultConfig()
	config.RetryFailedChecks = true

	executor := &mockExecutor{}
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	runner := NewRunner(config, DefaultThresholds(), executor, logger)

	// Checker that fails once then succeeds
	attemptCount := 0
	retryChecker := &mockChecker{
		name:     "retry_check",
		category: CategoryCPU,
		runFunc: func(ctx context.Context, executor CommandExecutor) (*CheckResult, error) {
			attemptCount++
			if attemptCount == 1 {
				return nil, errors.New("first attempt failed")
			}
			return &CheckResult{
				Name:      "retry_check",
				Category:  CategoryCPU,
				Status:    StatusCompleted,
				Severity:  SeverityOK,
				Message:   "Check succeeded on retry",
				Timestamp: time.Now(),
			}, nil
		},
	}

	runner.RegisterChecker(retryChecker)

	ctx := context.Background()
	report, err := runner.RunAll(ctx)

	if err != nil {
		t.Fatalf("RunAll() error = %v, want nil", err)
	}

	if attemptCount != 2 {
		t.Errorf("Expected 2 attempts (1 initial + 1 retry), got %d", attemptCount)
	}

	if len(report.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(report.Results))
	}

	result := report.Results[0]
	if result.Status != StatusCompleted {
		t.Errorf("Result.Status = %v, want %v", result.Status, StatusCompleted)
	}

	if result.Severity != SeverityOK {
		t.Errorf("Result.Severity = %v, want %v", result.Severity, SeverityOK)
	}
}

func TestRunner_RunAll_WithTimeout(t *testing.T) {
	config := DefaultConfig()
	config.CheckTimeout = 50 * time.Millisecond
	config.RetryFailedChecks = false

	executor := &mockExecutor{}
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	runner := NewRunner(config, DefaultThresholds(), executor, logger)

	slowChecker := &mockChecker{
		name:     "slow_check",
		category: CategoryCPU,
		runFunc: func(ctx context.Context, executor CommandExecutor) (*CheckResult, error) {
			select {
			case <-time.After(200 * time.Millisecond):
				return &CheckResult{
					Name:     "slow_check",
					Category: CategoryCPU,
					Status:   StatusCompleted,
					Severity: SeverityOK,
					Message:  "Should not reach here",
				}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}

	runner.RegisterChecker(slowChecker)

	ctx := context.Background()
	report, err := runner.RunAll(ctx)

	if err != nil {
		t.Fatalf("RunAll() error = %v, want nil", err)
	}

	if len(report.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(report.Results))
	}

	result := report.Results[0]
	if result.Status != StatusFailed {
		t.Errorf("Result.Status = %v, want %v (timeout should cause failure)", result.Status, StatusFailed)
	}

	if result.Severity != SeverityError {
		t.Errorf("Result.Severity = %v, want %v", result.Severity, SeverityError)
	}
}

func TestRunner_GetRegisteredChecks(t *testing.T) {
	runner := NewRunner(DefaultConfig(), nil, &mockExecutor{}, logrus.New())

	checkers := []Checker{
		&mockChecker{name: "check1", category: CategoryCPU},
		&mockChecker{name: "check2", category: CategoryMemory},
	}

	runner.RegisterCheckers(checkers...)

	registered := runner.GetRegisteredChecks()

	if len(registered) != 2 {
		t.Errorf("Expected 2 registered checks, got %d", len(registered))
	}

	// Verify names
	names := make(map[string]bool)
	for _, c := range registered {
		names[c.Name()] = true
	}

	if !names["check1"] || !names["check2"] {
		t.Error("Expected checks not found in registered checkers")
	}
}

func TestRunner_SetLogger(t *testing.T) {
	runner := NewRunner(DefaultConfig(), nil, &mockExecutor{}, logrus.New())

	newLogger := logrus.New()
	newLogger.SetLevel(logrus.DebugLevel)

	runner.SetLogger(newLogger)

	if runner.logger != newLogger {
		t.Error("Logger was not updated")
	}

	if runner.logger.Level != logrus.DebugLevel {
		t.Errorf("Logger level = %v, want %v", runner.logger.Level, logrus.DebugLevel)
	}
}

func TestRunner_EnabledChecks(t *testing.T) {
	config := DefaultConfig()
	config.EnabledChecks = []string{"cpu_check", "memory_check"}

	executor := &mockExecutor{}
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	runner := NewRunner(config, DefaultThresholds(), executor, logger)

	// Register 3 checkers, but only 2 should run
	runner.RegisterCheckers(
		&mockChecker{name: "cpu_check", category: CategoryCPU},
		&mockChecker{name: "memory_check", category: CategoryMemory},
		&mockChecker{name: "disk_check", category: CategoryDisk},
	)

	ctx := context.Background()
	report, err := runner.RunAll(ctx)

	if err != nil {
		t.Fatalf("RunAll() error = %v, want nil", err)
	}

	if len(report.Results) != 2 {
		t.Errorf("Expected 2 results (only enabled checks), got %d", len(report.Results))
	}

	// Verify only enabled checks ran
	names := make(map[string]bool)
	for _, r := range report.Results {
		names[r.Name] = true
	}

	if !names["cpu_check"] || !names["memory_check"] {
		t.Error("Expected enabled checks not found")
	}

	if names["disk_check"] {
		t.Error("Disabled check 'disk_check' should not have run")
	}
}

func TestRunner_ConcurrentSafety(t *testing.T) {
	runner := NewRunner(DefaultConfig(), nil, &mockExecutor{}, logrus.New())

	// Test concurrent RegisterChecker calls
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			checker := &mockChecker{
				name:     fmt.Sprintf("check_%d", idx),
				category: CategoryCPU,
			}
			runner.RegisterChecker(checker)
		}(i)
	}

	wg.Wait()

	if len(runner.checks) != 10 {
		t.Errorf("Expected 10 checkers after concurrent registration, got %d", len(runner.checks))
	}
}
