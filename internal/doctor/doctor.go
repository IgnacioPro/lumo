package doctor

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// CheckStatus represents the result of a health check
type CheckStatus string

const (
	StatusOK      CheckStatus = "ok"
	StatusWarning CheckStatus = "warning"
	StatusError   CheckStatus = "error"
	StatusSkipped CheckStatus = "skipped"
)

// CheckResult represents the result of a single health check
type CheckResult struct {
	Name        string
	Status      CheckStatus
	Message     string
	Remediation string // Suggestion on how to fix
	Duration    time.Duration
	Error       error
}

// Check is the interface for health checks
type Check interface {
	Name() string
	Run(ctx context.Context) CheckResult
}

// Doctor runs health checks and reports results
type Doctor struct {
	checks []Check
	log    *logrus.Logger
}

// New creates a new Doctor instance
func New(log *logrus.Logger) *Doctor {
	return &Doctor{
		checks: []Check{},
		log:    log,
	}
}

// AddCheck adds a health check to the doctor
func (d *Doctor) AddCheck(check Check) {
	d.checks = append(d.checks, check)
}

// RunAll executes all health checks
func (d *Doctor) RunAll(ctx context.Context) []CheckResult {
	results := make([]CheckResult, 0, len(d.checks))

	for _, check := range d.checks {
		d.log.Debugf("Running check: %s", check.Name())
		start := time.Now()

		result := check.Run(ctx)
		result.Duration = time.Since(start)

		results = append(results, result)
		d.log.Debugf("Check %s completed: %s (%.2fs)",
			check.Name(), result.Status, result.Duration.Seconds())
	}

	return results
}

// Summary returns overall health summary
func Summary(results []CheckResult) (ok, warnings, errors, skipped int) {
	for _, r := range results {
		switch r.Status {
		case StatusOK:
			ok++
		case StatusWarning:
			warnings++
		case StatusError:
			errors++
		case StatusSkipped:
			skipped++
		}
	}
	return
}

// IsHealthy returns true if no errors were found
func IsHealthy(results []CheckResult) bool {
	for _, r := range results {
		if r.Status == StatusError {
			return false
		}
	}
	return true
}

// FormatResult formats a single check result with colors and symbols
func FormatResult(r CheckResult) string {
	var symbol, color string

	switch r.Status {
	case StatusOK:
		symbol = "✓"
		color = "\033[32m" // Green
	case StatusWarning:
		symbol = "⚠"
		color = "\033[33m" // Yellow
	case StatusError:
		symbol = "✗"
		color = "\033[31m" // Red
	case StatusSkipped:
		symbol = "○"
		color = "\033[90m" // Gray
	}

	reset := "\033[0m"

	msg := fmt.Sprintf("%s%s%s %s", color, symbol, reset, r.Message)

	if r.Remediation != "" && (r.Status == StatusError || r.Status == StatusWarning) {
		msg += fmt.Sprintf("\n  → %s", r.Remediation)
	}

	return msg
}
