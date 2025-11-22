package diagnostics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// CheckCategory represents the category of a diagnostic check
type CheckCategory string

const (
	CategoryCPU            CheckCategory = "cpu"
	CategoryMemory         CheckCategory = "memory"
	CategoryDisk           CheckCategory = "disk"
	CategoryProcess        CheckCategory = "process"
	CategoryLog            CheckCategory = "logs"
	CategoryNetwork        CheckCategory = "network"
	CategoryService        CheckCategory = "service"
	CategorySecurity       CheckCategory = "security"
	CategoryKubernetes     CheckCategory = "kubernetes"
	CategoryVirtualization CheckCategory = "virtualization"
)

// CheckStatus represents the execution status of a check
type CheckStatus string

const (
	StatusCompleted CheckStatus = "completed"
	StatusFailed    CheckStatus = "failed"
	StatusSkipped   CheckStatus = "skipped"
	StatusTimeout   CheckStatus = "timeout"
)

// Checker performs a single diagnostic check
type Checker interface {
	// Name returns the unique name of this checker
	Name() string

	// Category returns the category this checker belongs to
	Category() CheckCategory

	// Description returns a human-readable description of what this checker does
	Description() string

	// Run executes the check and returns the result
	Run(ctx context.Context, executor CommandExecutor) (*CheckResult, error)

	// RequiresRoot returns true if this check needs root/sudo privileges
	RequiresRoot() bool
}

// CommandExecutor defines the interface for executing commands on remote systems
type CommandExecutor interface {
	// Execute runs a command and returns the result
	Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)

	// ExecuteWithContext runs a command with context support
	ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error)
}

// Config contains configuration for the diagnostic system
type Config struct {
	// EnabledChecks lists specific checks to run (empty = all)
	EnabledChecks []string

	// Parallel enables parallel check execution
	Parallel bool

	// CheckTimeout is the timeout for individual checks
	CheckTimeout time.Duration

	// MaxConcurrent is the maximum number of concurrent checks (0 = no limit)
	MaxConcurrent int

	// RetryFailedChecks enables retrying failed checks once
	RetryFailedChecks bool

	// Platform settings
	Platform PlatformConfig
}

// PlatformConfig contains platform-specific settings
type PlatformConfig struct {
	// AutoDetect enables automatic OS detection
	AutoDetect bool

	// ForceOS overrides OS detection (linux, darwin, freebsd, etc.)
	ForceOS string
}

// Runner orchestrates the execution of diagnostic checks
type Runner struct {
	checks     []Checker
	config     *Config
	thresholds *ThresholdConfig
	executor   CommandExecutor
	logger     *logrus.Logger
	mu         sync.RWMutex
}

// NewRunner creates a new diagnostic runner
func NewRunner(config *Config, thresholds *ThresholdConfig, executor CommandExecutor, logger *logrus.Logger) *Runner {
	if config == nil {
		config = DefaultConfig()
	}

	if logger == nil {
		logger = logrus.New()
	}

	return &Runner{
		config:     config,
		thresholds: thresholds,
		executor:   executor,
		logger:     logger,
		checks:     []Checker{},
	}
}

// DefaultConfig returns default diagnostic configuration
func DefaultConfig() *Config {
	return &Config{
		EnabledChecks:     []string{},
		Parallel:          true,
		CheckTimeout:      30 * time.Second,
		MaxConcurrent:     4,
		RetryFailedChecks: true,
		Platform: PlatformConfig{
			AutoDetect: true,
			ForceOS:    "",
		},
	}
}

// RegisterChecker adds a checker to the runner
func (r *Runner) RegisterChecker(checker Checker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks = append(r.checks, checker)
}

// RegisterCheckers adds multiple checkers to the runner
func (r *Runner) RegisterCheckers(checkers ...Checker) {
	for _, checker := range checkers {
		r.RegisterChecker(checker)
	}
}

// RunAll executes all registered checks and returns a report
func (r *Runner) RunAll(ctx context.Context) (*Report, error) {
	// Create tracing span
	tracer := otel.Tracer("lumo.diagnostics")
	ctx, span := tracer.Start(ctx, "RunAll")
	defer span.End()

	r.mu.RLock()
	checksToRun := r.getChecksToRun()
	r.mu.RUnlock()

	if len(checksToRun) == 0 {
		err := fmt.Errorf("no checks registered")
		span.RecordError(err)
		span.SetStatus(codes.Error, "No checks registered")
		return nil, err
	}

	// Set span attributes
	span.SetAttributes(
		attribute.Int("diagnostics.checks_to_run", len(checksToRun)),
		attribute.Bool("diagnostics.parallel", r.config.Parallel),
		attribute.Int("diagnostics.max_concurrent", r.config.MaxConcurrent),
	)

	r.logger.Infof("Running %d diagnostic checks (parallel: %v)", len(checksToRun), r.config.Parallel)

	startTime := time.Now()

	var results []*CheckResult

	if r.config.Parallel {
		results = r.runParallel(ctx, checksToRun)
	} else {
		results = r.runSequential(ctx, checksToRun)
	}

	duration := time.Since(startTime)

	report := &Report{
		Timestamp: startTime,
		Duration:  duration,
		Results:   results,
		Summary:   generateSummary(results),
	}

	// Add summary to span attributes
	span.SetAttributes(
		attribute.Int("diagnostics.total_checks", report.Summary.TotalChecks),
		attribute.Int("diagnostics.ok_count", report.Summary.OKCount),
		attribute.Int("diagnostics.warning_count", report.Summary.WarningCount),
		attribute.Int("diagnostics.critical_count", report.Summary.CriticalCount),
		attribute.Int("diagnostics.error_count", report.Summary.ErrorCount),
		attribute.String("diagnostics.duration", duration.String()),
	)
	span.SetStatus(codes.Ok, "Diagnostics completed successfully")

	r.logger.Infof("Diagnostics completed in %v: %d checks, %d OK, %d warnings, %d critical, %d errors",
		duration,
		report.Summary.TotalChecks,
		report.Summary.OKCount,
		report.Summary.WarningCount,
		report.Summary.CriticalCount,
		report.Summary.ErrorCount)

	return report, nil
}

// runSequential runs checks one after another
func (r *Runner) runSequential(ctx context.Context, checks []Checker) []*CheckResult {
	results := make([]*CheckResult, 0, len(checks))

	for _, checker := range checks {
		result := r.runSingleCheck(ctx, checker)
		results = append(results, result)
	}

	return results
}

// runParallel runs checks concurrently
func (r *Runner) runParallel(ctx context.Context, checks []Checker) []*CheckResult {
	resultsChan := make(chan *CheckResult, len(checks))
	semaphore := make(chan struct{}, r.config.MaxConcurrent)

	var wg sync.WaitGroup

	for _, checker := range checks {
		wg.Add(1)

		go func(c Checker) {
			defer wg.Done()

			// Acquire semaphore
			if r.config.MaxConcurrent > 0 {
				semaphore <- struct{}{}
				defer func() { <-semaphore }()
			}

			result := r.runSingleCheck(ctx, c)
			resultsChan <- result
		}(checker)
	}

	// Wait for all checks to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	results := make([]*CheckResult, 0, len(checks))
	for result := range resultsChan {
		results = append(results, result)
	}

	return results
}

// runSingleCheck executes a single check with timeout and retry logic
func (r *Runner) runSingleCheck(ctx context.Context, checker Checker) *CheckResult {
	// Create tracing span for individual check
	tracer := otel.Tracer("lumo.diagnostics")
	ctx, span := tracer.Start(ctx, "runSingleCheck")
	defer span.End()

	// Set check metadata in span
	span.SetAttributes(
		attribute.String("check.name", checker.Name()),
		attribute.String("check.category", string(checker.Category())),
		attribute.Bool("check.requires_root", checker.RequiresRoot()),
	)

	// Create check-specific context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, r.config.CheckTimeout)
	defer cancel()

	r.logger.Debugf("Running check: %s (%s)", checker.Name(), checker.Category())

	startTime := time.Now()

	// Run the check
	result, err := checker.Run(checkCtx, r.executor)

	if err != nil {
		// Check failed
		r.logger.Warnf("Check %s failed: %v", checker.Name(), err)

		// Retry if configured
		if r.config.RetryFailedChecks {
			r.logger.Debugf("Retrying check: %s", checker.Name())

			retryCtx, retryCancel := context.WithTimeout(ctx, r.config.CheckTimeout)
			defer retryCancel()

			retryResult, retryErr := checker.Run(retryCtx, r.executor)
			if retryErr == nil {
				r.logger.Infof("Check %s succeeded on retry", checker.Name())
				result = retryResult
				err = nil
			}
		}

		// If still failed, create error result
		if err != nil {
			result = &CheckResult{
				Name:      checker.Name(),
				Category:  checker.Category(),
				Status:    StatusFailed,
				Severity:  SeverityError,
				Message:   fmt.Sprintf("Check failed: %v", err),
				Timestamp: startTime,
				Duration:  time.Since(startTime),
				Error:     err.Error(),
			}
		}
	}

	// Evaluate thresholds and set severity
	if result.Status == StatusCompleted && result.Severity == "" {
		result.Severity = r.evaluateSeverity(result)
	}

	// Update span with check result
	span.SetAttributes(
		attribute.String("check.status", string(result.Status)),
		attribute.String("check.severity", string(result.Severity)),
		attribute.String("check.duration", result.Duration.String()),
	)

	if result.Status == StatusFailed || result.Severity == SeverityError {
		span.SetStatus(codes.Error, fmt.Sprintf("Check failed: %s", result.Message))
	} else {
		span.SetStatus(codes.Ok, "Check completed")
	}

	r.logger.Debugf("Check %s completed with severity: %s", checker.Name(), result.Severity)

	return result
}

// getChecksToRun filters checks based on configuration
func (r *Runner) getChecksToRun() []Checker {
	// If no specific checks configured, run all
	if len(r.config.EnabledChecks) == 0 {
		return r.checks
	}

	// Build map of enabled checks
	enabledMap := make(map[string]bool)
	for _, name := range r.config.EnabledChecks {
		enabledMap[name] = true
	}

	// Filter checks
	filtered := make([]Checker, 0)
	for _, checker := range r.checks {
		if enabledMap[string(checker.Category())] || enabledMap[checker.Name()] {
			filtered = append(filtered, checker)
		}
	}

	return filtered
}

// evaluateSeverity determines the severity level based on thresholds
func (r *Runner) evaluateSeverity(result *CheckResult) Severity {
	if r.thresholds == nil {
		// No thresholds configured, everything is OK if completed
		return SeverityOK
	}

	// Evaluate thresholds based on metrics
	return EvaluateThresholds(result, r.thresholds)
}

// GetRegisteredChecks returns the list of registered checkers
func (r *Runner) GetRegisteredChecks() []Checker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	checks := make([]Checker, len(r.checks))
	copy(checks, r.checks)
	return checks
}

// SetLogger sets a custom logger
func (r *Runner) SetLogger(logger *logrus.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if logger != nil {
		r.logger = logger
	}
}
