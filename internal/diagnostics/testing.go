package diagnostics

import "context"

// MockChecker is a test double for the Checker interface
type MockChecker struct {
	CheckName     string
	CheckCategory CheckCategory
	CheckResult   *CheckResult
	CheckError    error
	RunFunc       func(ctx context.Context, executor CommandExecutor) (*CheckResult, error)
}

// Name returns the checker name
func (m *MockChecker) Name() string {
	if m.CheckName != "" {
		return m.CheckName
	}
	return "mock_checker"
}

// Category returns the checker category
func (m *MockChecker) Category() CheckCategory {
	if m.CheckCategory != "" {
		return m.CheckCategory
	}
	return CategoryCPU
}

// Description returns the checker description
func (m *MockChecker) Description() string {
	return "Mock checker for testing"
}

// RequiresRoot returns whether the checker requires root privileges
func (m *MockChecker) RequiresRoot() bool {
	return false
}

// Run executes the check
func (m *MockChecker) Run(ctx context.Context, executor CommandExecutor) (*CheckResult, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, executor)
	}

	if m.CheckError != nil {
		return nil, m.CheckError
	}

	if m.CheckResult != nil {
		return m.CheckResult, nil
	}

	// Default result
	return &CheckResult{
		Name:     m.Name(),
		Category: m.Category(),
		Severity: SeverityOK,
		Message:  "Mock check passed",
		Data: map[string]interface{}{
			"mock": true,
		},
	}, nil
}
