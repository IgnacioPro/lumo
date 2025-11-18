package remediation

import (
	"context"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/sirupsen/logrus"
)

func TestActionStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status ActionStatus
		want   string
	}{
		{"pending", StatusPending, "pending"},
		{"approved", StatusApproved, "approved"},
		{"success", StatusSuccess, "success"},
		{"failed", StatusFailed, "failed"},
		{"rejected", StatusRejected, "rejected"},
		{"rolled_back", StatusRolledBack, "rolled_back"},
		{"skipped", StatusSkipped, "skipped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("ActionStatus.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestActionCategory_String(t *testing.T) {
	tests := []struct {
		name     string
		category ActionCategory
		want     string
	}{
		{"service", CategoryService, "service"},
		{"disk", CategoryDisk, "disk"},
		{"process", CategoryProcess, "process"},
		{"network", CategoryNetwork, "network"},
		{"system", CategorySystem, "system"},
		{"security", CategorySecurity, "security"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.category.String(); got != tt.want {
				t.Errorf("ActionCategory.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestActionResult_Succeeded(t *testing.T) {
	tests := []struct {
		name   string
		status ActionStatus
		want   bool
	}{
		{"success status returns true", StatusSuccess, true},
		{"failed status returns false", StatusFailed, false},
		{"pending status returns false", StatusPending, false},
		{"approved status returns false", StatusApproved, false},
		{"rejected status returns false", StatusRejected, false},
		{"rolled_back status returns false", StatusRolledBack, false},
		{"skipped status returns false", StatusSkipped, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ActionResult{Status: tt.status}
			if got := result.Succeeded(); got != tt.want {
				t.Errorf("ActionResult.Succeeded() = %v, want %v (status=%q)", got, tt.want, tt.status)
			}
		})
	}
}

func TestActionResult_Failed(t *testing.T) {
	tests := []struct {
		name   string
		status ActionStatus
		want   bool
	}{
		{"failed status returns true", StatusFailed, true},
		{"success status returns false", StatusSuccess, false},
		{"pending status returns false", StatusPending, false},
		{"approved status returns false", StatusApproved, false},
		{"rejected status returns false", StatusRejected, false},
		{"rolled_back status returns false", StatusRolledBack, false},
		{"skipped status returns false", StatusSkipped, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ActionResult{Status: tt.status}
			if got := result.Failed(); got != tt.want {
				t.Errorf("ActionResult.Failed() = %v, want %v (status=%q)", got, tt.want, tt.status)
			}
		})
	}
}

func TestActionResult_WasRolledBack(t *testing.T) {
	tests := []struct {
		name   string
		status ActionStatus
		want   bool
	}{
		{"rolled_back status returns true", StatusRolledBack, true},
		{"success status returns false", StatusSuccess, false},
		{"failed status returns false", StatusFailed, false},
		{"pending status returns false", StatusPending, false},
		{"approved status returns false", StatusApproved, false},
		{"rejected status returns false", StatusRejected, false},
		{"skipped status returns false", StatusSkipped, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ActionResult{Status: tt.status}
			if got := result.WasRolledBack(); got != tt.want {
				t.Errorf("ActionResult.WasRolledBack() = %v, want %v (status=%q)", got, tt.want, tt.status)
			}
		})
	}
}

func TestRemediationPlan_FilteredActions(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&testLogWriter{t})

	// Create mock actions for testing
	action1 := &mockAction{category: CategoryService}
	action2 := &mockAction{category: CategoryDisk}
	action3 := &mockAction{category: CategoryProcess}
	action4 := &mockAction{category: CategoryService}

	t.Run("no skip categories returns all actions", func(t *testing.T) {
		plan := NewRemediationPlan(logger)
		plan.AddAction(action1)
		plan.AddAction(action2)
		plan.AddAction(action3)
		plan.AddAction(action4)

		filtered := plan.FilteredActions()

		if len(filtered) != 4 {
			t.Errorf("FilteredActions() returned %d actions, want 4", len(filtered))
		}
	})

	t.Run("skip one category", func(t *testing.T) {
		plan := NewRemediationPlan(logger)
		plan.AddAction(action1)
		plan.AddAction(action2)
		plan.AddAction(action3)
		plan.AddAction(action4)
		plan.SkipCategories = []ActionCategory{CategoryService}

		filtered := plan.FilteredActions()

		// Should exclude action1 and action4 (both CategoryService)
		if len(filtered) != 2 {
			t.Errorf("FilteredActions() returned %d actions, want 2", len(filtered))
		}

		// Verify remaining actions are not CategoryService
		for _, action := range filtered {
			if action.Category() == CategoryService {
				t.Error("FilteredActions() should not include CategoryService actions")
			}
		}
	})

	t.Run("skip multiple categories", func(t *testing.T) {
		plan := NewRemediationPlan(logger)
		plan.AddAction(action1)
		plan.AddAction(action2)
		plan.AddAction(action3)
		plan.AddAction(action4)
		plan.SkipCategories = []ActionCategory{CategoryService, CategoryDisk}

		filtered := plan.FilteredActions()

		// Should only include action3 (CategoryProcess)
		if len(filtered) != 1 {
			t.Errorf("FilteredActions() returned %d actions, want 1", len(filtered))
		}

		if filtered[0].Category() != CategoryProcess {
			t.Errorf("FilteredActions()[0].Category() = %q, want %q", filtered[0].Category(), CategoryProcess)
		}
	})

	t.Run("skip all categories returns empty", func(t *testing.T) {
		plan := NewRemediationPlan(logger)
		plan.AddAction(action1)
		plan.AddAction(action2)
		plan.SkipCategories = []ActionCategory{CategoryService, CategoryDisk, CategoryProcess}

		filtered := plan.FilteredActions()

		if len(filtered) != 0 {
			t.Errorf("FilteredActions() returned %d actions, want 0", len(filtered))
		}
	})

	t.Run("empty plan returns empty", func(t *testing.T) {
		plan := NewRemediationPlan(logger)
		plan.SkipCategories = []ActionCategory{CategoryService}

		filtered := plan.FilteredActions()

		if len(filtered) != 0 {
			t.Errorf("FilteredActions() returned %d actions, want 0", len(filtered))
		}
	})
}

func TestBaseAction_IsReversible(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&testLogWriter{t})

	t.Run("reversible action", func(t *testing.T) {
		action := NewBaseAction("test-1", "Test", "Description", CategoryDisk, RiskSafe, true, "Low", logger)
		if !action.IsReversible() {
			t.Error("IsReversible() = false, want true")
		}
	})

	t.Run("non-reversible action", func(t *testing.T) {
		action := NewBaseAction("test-2", "Test", "Description", CategoryDisk, RiskCritical, false, "High", logger)
		if action.IsReversible() {
			t.Error("IsReversible() = true, want false")
		}
	})
}

func TestBaseAction_EstimateImpact(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&testLogWriter{t})

	tests := []struct {
		name   string
		impact string
	}{
		{"low impact", "Low"},
		{"medium impact", "Medium"},
		{"high impact", "High"},
		{"custom impact", "Frees up to 5GB of disk space"},
		{"empty impact", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := NewBaseAction("test", "Test", "Description", CategoryDisk, RiskSafe, true, tt.impact, logger)
			if got := action.EstimateImpact(); got != tt.impact {
				t.Errorf("EstimateImpact() = %q, want %q", got, tt.impact)
			}
		})
	}
}

func TestBaseAction_Validate(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&testLogWriter{t})
	action := NewBaseAction("test", "Test", "Description", CategoryDisk, RiskSafe, true, "Low", logger)
	ctx := context.Background()

	t.Run("valid executor passes validation", func(t *testing.T) {
		executor := &mockExecutor{}
		err := action.Validate(ctx, executor)
		if err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("nil executor fails validation", func(t *testing.T) {
		err := action.Validate(ctx, nil)
		if err == nil {
			t.Error("Validate() expected error with nil executor, got nil")
		}
		if err.Error() != "executor is nil" {
			t.Errorf("Validate() error = %q, want 'executor is nil'", err.Error())
		}
	})
}

func TestBaseAction_Execute(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&testLogWriter{t})
	action := NewBaseAction("test-action", "Test", "Description", CategoryDisk, RiskSafe, true, "Low", logger)
	ctx := context.Background()
	executor := &mockExecutor{}

	t.Run("base execute returns not implemented error", func(t *testing.T) {
		result, err := action.Execute(ctx, executor)

		if err == nil {
			t.Error("Execute() expected error, got nil")
		}

		if result != nil {
			t.Errorf("Execute() result = %v, want nil", result)
		}

		expectedMsg := "Execute not implemented for test-action"
		if err.Error() != expectedMsg {
			t.Errorf("Execute() error = %q, want %q", err.Error(), expectedMsg)
		}
	})
}

// Mock action for testing
type mockAction struct {
	category ActionCategory
}

func (m *mockAction) ID() string                       { return "mock-action" }
func (m *mockAction) Name() string                     { return "Mock Action" }
func (m *mockAction) Description() string              { return "Mock description" }
func (m *mockAction) Category() ActionCategory         { return m.category }
func (m *mockAction) Risk() RiskLevel                  { return RiskSafe }
func (m *mockAction) IsReversible() bool               { return true }
func (m *mockAction) EstimateImpact() string           { return "None" }
func (m *mockAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	return nil
}
func (m *mockAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	return &ActionResult{Status: StatusSuccess}, nil
}
func (m *mockAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	return nil
}

// Mock executor for testing
type mockExecutor struct{}

func (m *mockExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	return "", "", 0, nil
}

func (m *mockExecutor) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	return "", "", 0, nil
}

// Test log writer
type testLogWriter struct {
	t *testing.T
}

func (w *testLogWriter) Write(p []byte) (n int, err error) {
	w.t.Log(string(p))
	return len(p), nil
}
