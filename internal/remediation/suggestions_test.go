package remediation

import (
	"strings"
	"testing"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/sirupsen/logrus"
)

func TestSuggestServiceActions(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	registry := GetDefaultRegistry(logger)
	suggester := NewSuggester(registry, logger)

	tests := []struct {
		name           string
		result         *diagnostics.CheckResult
		expectedCount  int
		expectedAction string
	}{
		{
			name: "failed services with string slice",
			result: &diagnostics.CheckResult{
				Name:     "service_check",
				Severity: diagnostics.SeverityWarning,
				Message:  "3 services failed: nginx, postgresql, redis",
				Data: map[string]interface{}{
					"failed_service_names": []string{"nginx", "postgresql", "redis"},
				},
			},
			expectedCount:  3,
			expectedAction: "service.restart",
		},
		{
			name: "failed services with interface slice",
			result: &diagnostics.CheckResult{
				Name:     "service_check",
				Severity: diagnostics.SeverityCritical,
				Message:  "2 services failed: mysql, apache2",
				Data: map[string]interface{}{
					"failed_service_names": []interface{}{"mysql", "apache2"},
				},
			},
			expectedCount:  2,
			expectedAction: "service.restart",
		},
		{
			name: "no failed services",
			result: &diagnostics.CheckResult{
				Name:     "service_check",
				Severity: diagnostics.SeverityOK,
				Message:  "All services running normally",
				Data:     map[string]interface{}{},
			},
			expectedCount: 0,
		},
		{
			name: "empty failed services list",
			result: &diagnostics.CheckResult{
				Name:     "service_check",
				Severity: diagnostics.SeverityOK,
				Message:  "All services running normally",
				Data: map[string]interface{}{
					"failed_service_names": []string{},
				},
			},
			expectedCount: 0,
		},
		{
			name: "missing data field",
			result: &diagnostics.CheckResult{
				Name:     "service_check",
				Severity: diagnostics.SeverityWarning,
				Message:  "Some services failed",
				Data:     nil,
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := suggester.suggestServiceActions(tt.result)

			if len(actions) != tt.expectedCount {
				t.Errorf("Expected %d actions, got %d", tt.expectedCount, len(actions))
			}

			if tt.expectedCount > 0 && len(actions) > 0 {
				// Check that the action ID starts with the expected prefix
				if !strings.HasPrefix(actions[0].ID(), tt.expectedAction) {
					t.Errorf("Expected action ID to start with %s, got %s", tt.expectedAction, actions[0].ID())
				}
			}

			if tt.expectedCount > 0 && len(actions) > 0 {
				// Check that the action ID starts with the expected prefix
				if !strings.HasPrefix(actions[0].ID(), tt.expectedAction) {
					t.Errorf("Expected action ID to start with %s, got %s", tt.expectedAction, actions[0].ID())
				}
			}
		})
	}
}

func TestSuggestFromReport(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	registry := GetDefaultRegistry(logger)
	suggester := NewSuggester(registry, logger)

	report := &diagnostics.Report{
		Results: []*diagnostics.CheckResult{
			{
				Name:     "service_check",
				Severity: diagnostics.SeverityWarning,
				Message:  "Failed services detected",
				Data: map[string]interface{}{
					"failed_service_names": []string{"nginx", "redis"},
				},
			},
			{
				Name:     "cpu_check",
				Severity: diagnostics.SeverityOK,
				Message:  "CPU usage normal",
			},
			{
				Name:     "disk_check",
				Severity: diagnostics.SeverityCritical,
				Message:  "Disk usage at 92%",
			},
		},
	}

	plan, err := suggester.SuggestFromReport(report)
	if err != nil {
		t.Fatalf("SuggestFromReport failed: %v", err)
	}

	// Should have actions from service check (2 services) and disk check
	if len(plan.Actions) < 2 {
		t.Errorf("Expected at least 2 actions (2 service restarts), got %d", len(plan.Actions))
	}

	// Verify we have service restart actions
	serviceRestartCount := 0
	for _, action := range plan.Actions {
		if strings.HasPrefix(action.ID(), "service.restart") {
			serviceRestartCount++
		}
	}

	if serviceRestartCount != 2 {
		t.Errorf("Expected 2 service.restart actions, got %d", serviceRestartCount)
	}
}

func TestSuggestForCheckResult_SeverityFiltering(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	registry := GetDefaultRegistry(logger)
	suggester := NewSuggester(registry, logger)

	tests := []struct {
		name          string
		severity      diagnostics.Severity
		expectActions bool
	}{
		{
			name:          "OK severity - no actions",
			severity:      diagnostics.SeverityOK,
			expectActions: false,
		},
		{
			name:          "Warning severity - has actions",
			severity:      diagnostics.SeverityWarning,
			expectActions: true,
		},
		{
			name:          "Critical severity - has actions",
			severity:      diagnostics.SeverityCritical,
			expectActions: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &diagnostics.CheckResult{
				Name:     "service_check",
				Severity: tt.severity,
				Message:  "Test message",
				Data: map[string]interface{}{
					"failed_service_names": []string{"test-service"},
				},
			}

			actions := suggester.suggestForCheckResult(result)

			hasActions := len(actions) > 0
			if hasActions != tt.expectActions {
				t.Errorf("Expected actions: %v, got: %v (count: %d)", tt.expectActions, hasActions, len(actions))
			}
		})
	}
}

func TestSuggestDiskActions(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	registry := GetDefaultRegistry(logger)
	suggester := NewSuggester(registry, logger)

	tests := []struct {
		name          string
		result        *diagnostics.CheckResult
		minActions    int
		expectCleaner bool
	}{
		{
			name: "critical disk usage 92%",
			result: &diagnostics.CheckResult{
				Name:     "disk_check",
				Severity: diagnostics.SeverityCritical,
				Message:  "Disk usage at 92%",
			},
			minActions:    3, // Should suggest aggressive cleanup
			expectCleaner: true,
		},
		{
			name: "warning disk usage 75%",
			result: &diagnostics.CheckResult{
				Name:     "disk_check",
				Severity: diagnostics.SeverityWarning,
				Message:  "Disk usage at 75%",
			},
			minActions:    1, // Should suggest conservative cleanup
			expectCleaner: true,
		},
		{
			name: "ok disk usage",
			result: &diagnostics.CheckResult{
				Name:     "disk_check",
				Severity: diagnostics.SeverityOK,
				Message:  "Disk usage at 45%",
			},
			minActions:    0, // No actions needed
			expectCleaner: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := suggester.suggestDiskActions(tt.result)

			if len(actions) < tt.minActions {
				t.Errorf("Expected at least %d actions, got %d", tt.minActions, len(actions))
			}

			hasCleaner := false
			for _, action := range actions {
				if action.Category() == CategoryDisk {
					hasCleaner = true
					break
				}
			}

			if hasCleaner != tt.expectCleaner {
				t.Errorf("Expected disk cleaner: %v, got: %v", tt.expectCleaner, hasCleaner)
			}
		})
	}
}

func TestSuggestProcessActions(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	registry := GetDefaultRegistry(logger)
	suggester := NewSuggester(registry, logger)

	t.Run("detects zombie processes but does not suggest actions", func(t *testing.T) {
		result := &diagnostics.CheckResult{
			Name:     "process_check",
			Severity: diagnostics.SeverityWarning,
			Message:  "Found 5 zombie processes",
		}

		actions := suggester.suggestProcessActions(result)

		if len(actions) != 0 {
			t.Errorf("suggestProcessActions() returned %d actions, want 0 (zombies require manual intervention)", len(actions))
		}
	})

	t.Run("detects many zombies but does not suggest actions", func(t *testing.T) {
		result := &diagnostics.CheckResult{
			Name:     "process_check",
			Severity: diagnostics.SeverityCritical,
			Message:  "Found 15 zombies processes",
		}

		actions := suggester.suggestProcessActions(result)

		if len(actions) != 0 {
			t.Errorf("suggestProcessActions() returned %d actions, want 0 (many zombies still require manual intervention)", len(actions))
		}
	})

	t.Run("detects high process count but does not suggest actions", func(t *testing.T) {
		result := &diagnostics.CheckResult{
			Name:     "process_check",
			Severity: diagnostics.SeverityWarning,
			Message:  "High process count: 2500 processes running",
		}

		actions := suggester.suggestProcessActions(result)

		if len(actions) != 0 {
			t.Errorf("suggestProcessActions() returned %d actions, want 0 (high process count requires manual analysis)", len(actions))
		}
	})

	t.Run("no process issues returns empty", func(t *testing.T) {
		result := &diagnostics.CheckResult{
			Name:     "process_check",
			Severity: diagnostics.SeverityOK,
			Message:  "Process count normal: 150 processes",
		}

		actions := suggester.suggestProcessActions(result)

		if len(actions) != 0 {
			t.Errorf("suggestProcessActions() returned %d actions, want 0", len(actions))
		}
	})
}

func TestSuggestMemoryActions(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	registry := GetDefaultRegistry(logger)
	suggester := NewSuggester(registry, logger)

	t.Run("suggests cache cleanup for critical memory issues", func(t *testing.T) {
		result := &diagnostics.CheckResult{
			Name:     "memory_check",
			Severity: diagnostics.SeverityCritical,
			Message:  "Critical memory pressure: 95% used",
		}

		actions := suggester.suggestMemoryActions(result)

		if len(actions) == 0 {
			t.Fatal("suggestMemoryActions() returned no actions, want cache cleanup suggestion")
		}

		if actions[0].Category() != CategoryDisk {
			t.Errorf("suggestMemoryActions() action category = %v, want %v", actions[0].Category(), CategoryDisk)
		}
	})

	t.Run("no suggestions for warning level memory issues", func(t *testing.T) {
		result := &diagnostics.CheckResult{
			Name:     "memory_check",
			Severity: diagnostics.SeverityWarning,
			Message:  "Memory usage at 75%",
		}

		actions := suggester.suggestMemoryActions(result)

		if len(actions) != 0 {
			t.Errorf("suggestMemoryActions() returned %d actions, want 0 for warning level", len(actions))
		}
	})

	t.Run("no suggestions for non-memory issues", func(t *testing.T) {
		result := &diagnostics.CheckResult{
			Name:     "cpu_check",
			Severity: diagnostics.SeverityCritical,
			Message:  "High CPU usage detected",
		}

		actions := suggester.suggestMemoryActions(result)

		if len(actions) != 0 {
			t.Errorf("suggestMemoryActions() returned %d actions, want 0 for non-memory issues", len(actions))
		}
	})
}

func TestSuggestForSeverity(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	registry := GetDefaultRegistry(logger)
	suggester := NewSuggester(registry, logger)

	t.Run("suggests cleanup for multiple critical issues", func(t *testing.T) {
		report := &diagnostics.Report{
			Results: []*diagnostics.CheckResult{
				{Name: "disk_check", Severity: diagnostics.SeverityCritical, Message: "Disk 90% full"},
				{Name: "memory_check", Severity: diagnostics.SeverityCritical, Message: "Memory 95% used"},
				{Name: "cpu_check", Severity: diagnostics.SeverityCritical, Message: "CPU at 100%"},
				{Name: "network_check", Severity: diagnostics.SeverityOK, Message: "Network OK"},
			},
		}

		actions := suggester.SuggestForSeverity(report, diagnostics.SeverityCritical)

		// Should suggest cleanup actions for multiple critical issues
		if len(actions) == 0 {
			t.Error("SuggestForSeverity() returned no actions for multiple critical issues")
		}
	})

	t.Run("no suggestions for few critical issues", func(t *testing.T) {
		report := &diagnostics.Report{
			Results: []*diagnostics.CheckResult{
				{Name: "disk_check", Severity: diagnostics.SeverityCritical, Message: "Disk 90% full"},
				{Name: "memory_check", Severity: diagnostics.SeverityWarning, Message: "Memory 75% used"},
				{Name: "cpu_check", Severity: diagnostics.SeverityOK, Message: "CPU normal"},
			},
		}

		actions := suggester.SuggestForSeverity(report, diagnostics.SeverityCritical)

		// With only 1 critical issue, should not suggest based on severity alone
		if len(actions) != 0 {
			t.Errorf("SuggestForSeverity() returned %d actions, want 0 for single critical issue", len(actions))
		}
	})

	t.Run("no suggestions for all OK report", func(t *testing.T) {
		report := &diagnostics.Report{
			Results: []*diagnostics.CheckResult{
				{Name: "disk_check", Severity: diagnostics.SeverityOK, Message: "Disk OK"},
				{Name: "memory_check", Severity: diagnostics.SeverityOK, Message: "Memory OK"},
				{Name: "cpu_check", Severity: diagnostics.SeverityOK, Message: "CPU OK"},
			},
		}

		actions := suggester.SuggestForSeverity(report, diagnostics.SeverityWarning)

		if len(actions) != 0 {
			t.Errorf("SuggestForSeverity() returned %d actions, want 0 for all-OK report", len(actions))
		}
	})
}

func TestExplainSuggestion(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("generates explanation for action suggestion", func(t *testing.T) {
		action := NewCleanLogsAction(30, false, logger)
		checkResult := &diagnostics.CheckResult{
			Name:     "disk_check",
			Severity: diagnostics.SeverityCritical,
			Message:  "Disk space critical: 95% full",
		}

		explanation := ExplainSuggestion(action, checkResult)

		if !strings.Contains(explanation, action.Name()) {
			t.Errorf("ExplainSuggestion() does not contain action name: %s", action.Name())
		}

		if !strings.Contains(explanation, checkResult.Name) {
			t.Errorf("ExplainSuggestion() does not contain check name: %s", checkResult.Name)
		}

		if !strings.Contains(explanation, string(checkResult.Severity)) {
			t.Errorf("ExplainSuggestion() does not contain severity: %s", checkResult.Severity)
		}

		if !strings.Contains(explanation, checkResult.Message) {
			t.Errorf("ExplainSuggestion() does not contain message: %s", checkResult.Message)
		}

		if !strings.Contains(explanation, action.EstimateImpact()) {
			t.Errorf("ExplainSuggestion() does not contain impact estimation")
		}
	})

	t.Run("formats explanation with proper structure", func(t *testing.T) {
		action := NewRestartServiceAction("nginx", logger)
		checkResult := &diagnostics.CheckResult{
			Name:     "service_check",
			Severity: diagnostics.SeverityWarning,
			Message:  "Service nginx has failed",
		}

		explanation := ExplainSuggestion(action, checkResult)

		// Check for expected structure keywords
		if !strings.Contains(explanation, "Action") {
			t.Error("ExplainSuggestion() missing 'Action' keyword")
		}

		if !strings.Contains(explanation, "Check:") {
			t.Error("ExplainSuggestion() missing 'Check:' section")
		}

		if !strings.Contains(explanation, "Severity:") {
			t.Error("ExplainSuggestion() missing 'Severity:' section")
		}

		if !strings.Contains(explanation, "Issue:") {
			t.Error("ExplainSuggestion() missing 'Issue:' section")
		}

		if !strings.Contains(explanation, "Expected Impact:") {
			t.Error("ExplainSuggestion() missing 'Expected Impact:' section")
		}
	})
}
