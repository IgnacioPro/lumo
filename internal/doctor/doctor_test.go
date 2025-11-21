package doctor

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCheck implements the Check interface for testing
type MockCheck struct {
	name   string
	result CheckResult
	delay  time.Duration
}

func (m *MockCheck) Name() string {
	return m.name
}

func (m *MockCheck) Run(ctx context.Context) CheckResult {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	return CheckResult{
		Name:        m.name,
		Status:      m.result.Status,
		Message:     m.result.Message,
		Remediation: m.result.Remediation,
		Error:       m.result.Error,
	}
}

func TestNew(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	doctor := New(logger)

	assert.NotNil(t, doctor)
	assert.NotNil(t, doctor.log)
	assert.Empty(t, doctor.checks)
}

func TestDoctor_AddCheck(t *testing.T) {
	doctor := New(logrus.New())

	check1 := &MockCheck{name: "check1"}
	check2 := &MockCheck{name: "check2"}

	doctor.AddCheck(check1)
	assert.Len(t, doctor.checks, 1)

	doctor.AddCheck(check2)
	assert.Len(t, doctor.checks, 2)
}

func TestDoctor_RunAll(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	doctor := New(logger)

	t.Run("EmptyChecks", func(t *testing.T) {
		results := doctor.RunAll(context.Background())
		assert.Empty(t, results)
	})

	t.Run("SingleCheck", func(t *testing.T) {
		d := New(logger)
		check := &MockCheck{
			name: "test-check",
			result: CheckResult{
				Status:  StatusOK,
				Message: "Test passed",
			},
		}
		d.AddCheck(check)

		results := d.RunAll(context.Background())
		require.Len(t, results, 1)
		assert.Equal(t, "test-check", results[0].Name)
		assert.Equal(t, StatusOK, results[0].Status)
		assert.Equal(t, "Test passed", results[0].Message)
		assert.Greater(t, results[0].Duration, time.Duration(0))
	})

	t.Run("MultipleChecks", func(t *testing.T) {
		d := New(logger)

		checks := []*MockCheck{
			{name: "check1", result: CheckResult{Status: StatusOK, Message: "OK"}},
			{name: "check2", result: CheckResult{Status: StatusWarning, Message: "Warning"}},
			{name: "check3", result: CheckResult{Status: StatusError, Message: "Error"}},
		}

		for _, check := range checks {
			d.AddCheck(check)
		}

		results := d.RunAll(context.Background())
		require.Len(t, results, 3)

		assert.Equal(t, StatusOK, results[0].Status)
		assert.Equal(t, StatusWarning, results[1].Status)
		assert.Equal(t, StatusError, results[2].Status)
	})

	t.Run("WithDelay", func(t *testing.T) {
		d := New(logger)
		check := &MockCheck{
			name:  "slow-check",
			delay: 50 * time.Millisecond,
			result: CheckResult{
				Status:  StatusOK,
				Message: "Completed",
			},
		}
		d.AddCheck(check)

		start := time.Now()
		results := d.RunAll(context.Background())
		elapsed := time.Since(start)

		require.Len(t, results, 1)
		assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
		assert.GreaterOrEqual(t, results[0].Duration, 50*time.Millisecond)
	})
}

func TestSummary(t *testing.T) {
	t.Run("AllOK", func(t *testing.T) {
		results := []CheckResult{
			{Status: StatusOK},
			{Status: StatusOK},
			{Status: StatusOK},
		}

		ok, warnings, errors, skipped := Summary(results)
		assert.Equal(t, 3, ok)
		assert.Equal(t, 0, warnings)
		assert.Equal(t, 0, errors)
		assert.Equal(t, 0, skipped)
	})

	t.Run("Mixed", func(t *testing.T) {
		results := []CheckResult{
			{Status: StatusOK},
			{Status: StatusOK},
			{Status: StatusWarning},
			{Status: StatusWarning},
			{Status: StatusError},
			{Status: StatusSkipped},
		}

		ok, warnings, errors, skipped := Summary(results)
		assert.Equal(t, 2, ok)
		assert.Equal(t, 2, warnings)
		assert.Equal(t, 1, errors)
		assert.Equal(t, 1, skipped)
	})

	t.Run("Empty", func(t *testing.T) {
		results := []CheckResult{}

		ok, warnings, errors, skipped := Summary(results)
		assert.Equal(t, 0, ok)
		assert.Equal(t, 0, warnings)
		assert.Equal(t, 0, errors)
		assert.Equal(t, 0, skipped)
	})
}

func TestIsHealthy(t *testing.T) {
	t.Run("Healthy", func(t *testing.T) {
		results := []CheckResult{
			{Status: StatusOK},
			{Status: StatusOK},
			{Status: StatusWarning},
			{Status: StatusSkipped},
		}

		assert.True(t, IsHealthy(results))
	})

	t.Run("Unhealthy", func(t *testing.T) {
		results := []CheckResult{
			{Status: StatusOK},
			{Status: StatusError},
			{Status: StatusWarning},
		}

		assert.False(t, IsHealthy(results))
	})

	t.Run("Empty", func(t *testing.T) {
		results := []CheckResult{}
		assert.True(t, IsHealthy(results))
	})

	t.Run("MultipleErrors", func(t *testing.T) {
		results := []CheckResult{
			{Status: StatusError},
			{Status: StatusError},
		}

		assert.False(t, IsHealthy(results))
	})
}

func TestFormatResult(t *testing.T) {
	testCases := []struct {
		name             string
		result           CheckResult
		expectedSymbol   string
		expectedContains []string
	}{
		{
			name: "OK",
			result: CheckResult{
				Name:    "Test Check",
				Status:  StatusOK,
				Message: "Everything is fine",
			},
			expectedSymbol:   "✓",
			expectedContains: []string{"Everything is fine"},
		},
		{
			name: "Warning",
			result: CheckResult{
				Name:        "Test Check",
				Status:      StatusWarning,
				Message:     "Minor issue",
				Remediation: "Fix this",
			},
			expectedSymbol:   "⚠",
			expectedContains: []string{"Minor issue", "Fix this"},
		},
		{
			name: "Error",
			result: CheckResult{
				Name:        "Test Check",
				Status:      StatusError,
				Message:     "Critical failure",
				Remediation: "Urgent action needed",
			},
			expectedSymbol:   "✗",
			expectedContains: []string{"Critical failure", "Urgent action needed"},
		},
		{
			name: "Skipped",
			result: CheckResult{
				Name:    "Test Check",
				Status:  StatusSkipped,
				Message: "Not applicable",
			},
			expectedSymbol:   "○",
			expectedContains: []string{"Not applicable"},
		},
		{
			name: "OKNoRemediation",
			result: CheckResult{
				Name:        "Test Check",
				Status:      StatusOK,
				Message:     "All good",
				Remediation: "This should not appear",
			},
			expectedSymbol:   "✓",
			expectedContains: []string{"All good"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formatted := FormatResult(tc.result)

			assert.Contains(t, formatted, tc.expectedSymbol)
			for _, expected := range tc.expectedContains {
				assert.Contains(t, formatted, expected)
			}

			// OK and Skipped should not show remediation
			if tc.result.Status == StatusOK || tc.result.Status == StatusSkipped {
				if tc.result.Remediation != "" {
					assert.NotContains(t, formatted, tc.result.Remediation)
				}
			}
		})
	}
}

func TestCheckStatus(t *testing.T) {
	// Test that status constants are correct
	assert.Equal(t, CheckStatus("ok"), StatusOK)
	assert.Equal(t, CheckStatus("warning"), StatusWarning)
	assert.Equal(t, CheckStatus("error"), StatusError)
	assert.Equal(t, CheckStatus("skipped"), StatusSkipped)
}

func TestCheckResultFields(t *testing.T) {
	// Verify CheckResult struct can hold all expected data
	err := assert.AnError
	result := CheckResult{
		Name:        "Test",
		Status:      StatusError,
		Message:     "Message",
		Remediation: "Fix it",
		Duration:    100 * time.Millisecond,
		Error:       err,
	}

	assert.Equal(t, "Test", result.Name)
	assert.Equal(t, StatusError, result.Status)
	assert.Equal(t, "Message", result.Message)
	assert.Equal(t, "Fix it", result.Remediation)
	assert.Equal(t, 100*time.Millisecond, result.Duration)
	assert.Equal(t, err, result.Error)
}
