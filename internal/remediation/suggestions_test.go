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
