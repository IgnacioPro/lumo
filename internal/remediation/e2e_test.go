package remediation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/diagnostics/checkers"
)

// mockExecutorWithFailedServices simulates a system with failed services
type mockExecutorWithFailedServices struct{}

func (m *mockExecutorWithFailedServices) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	// Detect service manager
	if command == "command -v systemctl >/dev/null 2>&1" {
		return "", "", 0, nil // systemd exists
	}
	if command == "systemctl is-system-running >/dev/null 2>&1 || systemctl list-units >/dev/null 2>&1" {
		return "", "", 0, nil // systemd is running
	}

	// Return mock systemd output with failed services
	if command == "systemctl list-units --type=service --all --no-pager --no-legend --plain" {
		return `nginx.service                loaded    failed     failed    Nginx web server
postgresql.service          loaded    failed     failed    PostgreSQL database server
redis.service               loaded    active     running   Redis in-memory data store
docker.service              loaded    active     running   Docker Application Container Engine
ssh.service                 loaded    active     running   OpenSSH server daemon
apache2.service             loaded    failed     failed    Apache HTTP Server`, "", 0, nil
	}

	return "", "", 0, nil
}

// Implement Execute method for interface compatibility
func (m *mockExecutorWithFailedServices) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	return m.ExecuteWithContext(context.Background(), command)
}

// TestEndToEndServiceRemediation tests the complete flow:
// 1. Diagnostics detects failed services
// 2. Remediation suggester reads from result.Data
// 3. Remediation actions are generated correctly
func TestEndToEndServiceRemediation(t *testing.T) {
	// Setup logging
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Step 1: Run diagnostics with mock executor
	t.Log("Step 1: Running service diagnostics...")
	serviceChecker := checkers.NewServiceChecker(nil)
	mockExec := &mockExecutorWithFailedServices{}

	ctx := context.Background()
	result, err := serviceChecker.Run(ctx, mockExec)
	if err != nil {
		t.Fatalf("Service check failed: %v", err)
	}

	// Set severity based on threshold (normally done by runner)
	// The service checker has a threshold of 1.0 for failed_services metric
	// Since we have 3 failed services, this should be a warning
	result.Severity = diagnostics.SeverityWarning

	// Verify diagnostics detected failed services
	failedCountValue, ok := result.GetDataValue("failed_services")
	if !ok {
		t.Fatal("failed_services not set in result data")
	}

	failedCount := failedCountValue.(int)
	if failedCount != 3 {
		t.Errorf("Expected 3 failed services, got %v", failedCount)
	}

	failedNames, ok := result.GetDataValue("failed_service_names")
	if !ok {
		t.Fatal("failed_service_names not set in result data")
	}

	failedSlice, ok := failedNames.([]string)
	if !ok {
		t.Fatalf("failed_service_names is not []string, got %T", failedNames)
	}

	expectedFailed := []string{"nginx", "postgresql", "apache2"}
	if len(failedSlice) != len(expectedFailed) {
		t.Errorf("Expected %d failed services, got %d", len(expectedFailed), len(failedSlice))
	}

	t.Logf("✅ Diagnostics detected failed services: %v", failedSlice)

	// Step 2: Create remediation suggester
	t.Log("Step 2: Generating remediation suggestions...")
	registry := NewRegistry(logger)

	suggester := NewSuggester(registry, logger)

	// Step 3: Generate suggestions from diagnostic report
	report := &diagnostics.Report{
		Results: []*diagnostics.CheckResult{result},
		Summary: diagnostics.ReportSummary{
			TotalChecks:   1,
			WarningCount:  1,
			CriticalCount: 0,
		},
	}

	plan, err := suggester.SuggestFromReport(report)
	if err != nil {
		t.Fatalf("Failed to generate remediation plan: %v", err)
	}

	// Step 4: Verify remediation actions were generated
	t.Log("Step 3: Verifying remediation actions...")
	if len(plan.Actions) == 0 {
		t.Fatal("No remediation actions generated!")
	}

	t.Logf("✅ Generated %d remediation actions", len(plan.Actions))

	// Verify we have restart actions for each failed service
	serviceRestartCount := 0
	for _, action := range plan.Actions {
		if strings.HasPrefix(action.ID(), "service.restart.") {
			serviceRestartCount++
			t.Logf("   - Restart action: %s (risk: %s)",
				action.Name(), action.Risk())
		}
	}

	if serviceRestartCount != 3 {
		t.Errorf("Expected 3 service restart actions, got %d", serviceRestartCount)
	}

	t.Log("✅ End-to-end test PASSED: Full flow from diagnostics → remediation working correctly")
}

// TestEndToEndDiskRemediation tests disk space remediation
func TestEndToEndDiskRemediation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create a critical disk space result
	result := &diagnostics.CheckResult{
		Name:      "disk_check",
		Category:  diagnostics.CategoryDisk,
		Severity:  diagnostics.SeverityCritical,
		Message:   "Disk usage critical: 92% on /",
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
	}

	result.AddMetric(diagnostics.Metric{
		Name:          "disk_usage_root",
		Value:         92.0,
		Unit:          "percent",
		Threshold:     90.0,
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	// Generate remediation suggestions
	registry := NewRegistry(logger)
	suggester := NewSuggester(registry, logger)

	report := &diagnostics.Report{
		Results: []*diagnostics.CheckResult{result},
		Summary: diagnostics.ReportSummary{
			TotalChecks:   1,
			CriticalCount: 1,
		},
	}

	plan, err := suggester.SuggestFromReport(report)
	if err != nil {
		t.Fatalf("Failed to generate remediation plan: %v", err)
	}

	// Verify disk cleanup actions were generated
	if len(plan.Actions) == 0 {
		t.Fatal("No remediation actions generated for critical disk usage!")
	}

	t.Logf("✅ Generated %d disk cleanup actions for 92%% usage", len(plan.Actions))

	// Verify we have various cleanup actions
	cleanupTypes := make(map[string]int)
	for _, action := range plan.Actions {
		cleanupTypes[action.ID()]++
		t.Logf("   - %s: %s (risk: %s)", action.ID(), action.Description(), action.Risk())
	}

	// Should have multiple cleanup strategies
	if len(cleanupTypes) < 2 {
		t.Errorf("Expected multiple cleanup strategies, got %d", len(cleanupTypes))
	}

	t.Log("✅ Disk remediation test PASSED")
}

// TestEndToEndMultipleIssues tests handling multiple issues in one report
func TestEndToEndMultipleIssues(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create multiple check results
	serviceResult := &diagnostics.CheckResult{
		Name:      "service_check",
		Category:  diagnostics.CategoryService,
		Severity:  diagnostics.SeverityWarning,
		Message:   "2 failed services",
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
	}
	serviceResult.SetData("failed_service_names", []string{"nginx", "postgresql"})

	diskResult := &diagnostics.CheckResult{
		Name:      "disk_check",
		Category:  diagnostics.CategoryDisk,
		Severity:  diagnostics.SeverityCritical,
		Message:   "Disk space critical: 93% on /",
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
	}
	diskResult.AddMetric(diagnostics.Metric{
		Name:          "disk_usage_root",
		Value:         93.0,
		Unit:          "percent",
		Threshold:     90.0,
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	cpuResult := &diagnostics.CheckResult{
		Name:      "cpu_check",
		Category:  diagnostics.CategoryCPU,
		Severity:  diagnostics.SeverityOK,
		Message:   "CPU usage normal",
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
	}

	// Generate remediation plan
	registry := NewRegistry(logger)
	suggester := NewSuggester(registry, logger)

	report := &diagnostics.Report{
		Results: []*diagnostics.CheckResult{serviceResult, diskResult, cpuResult},
		Summary: diagnostics.ReportSummary{
			TotalChecks:   3,
			OKCount:       1,
			WarningCount:  1,
			CriticalCount: 1,
		},
	}

	plan, err := suggester.SuggestFromReport(report)
	if err != nil {
		t.Fatalf("Failed to generate remediation plan: %v", err)
	}

	// Verify actions from multiple sources
	if len(plan.Actions) < 3 {
		t.Errorf("Expected at least 3 actions (2 service restarts + disk cleanup), got %d", len(plan.Actions))
	}

	serviceActions := 0
	diskActions := 0
	for _, action := range plan.Actions {
		actionID := action.ID()
		if strings.HasPrefix(actionID, "service.restart.") || strings.HasPrefix(actionID, "service.start.") {
			serviceActions++
		} else if strings.HasPrefix(actionID, "disk.clean_") {
			diskActions++
		}
	}

	if serviceActions != 2 {
		t.Errorf("Expected 2 service actions, got %d", serviceActions)
	}

	if diskActions == 0 {
		t.Error("Expected disk cleanup actions, got none")
	}

	t.Logf("✅ Multiple issues test PASSED: %d service actions, %d disk actions", serviceActions, diskActions)
}
