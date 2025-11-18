package observability

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordSessionStart(t *testing.T) {
	// Reset metrics
	SessionsActive.Set(0)

	initialTotal := testutil.ToFloat64(SessionsTotal)
	initialActive := testutil.ToFloat64(SessionsActive)

	RecordSessionStart()

	if testutil.ToFloat64(SessionsActive) != initialActive+1 {
		t.Errorf("SessionsActive should be incremented by 1, got %f", testutil.ToFloat64(SessionsActive))
	}

	if testutil.ToFloat64(SessionsTotal) != initialTotal+1 {
		t.Errorf("SessionsTotal should be incremented by 1, got %f", testutil.ToFloat64(SessionsTotal))
	}
}

func TestRecordSessionEnd(t *testing.T) {
	// Set initial value
	SessionsActive.Set(5)
	initial := testutil.ToFloat64(SessionsActive)

	RecordSessionEnd()

	if testutil.ToFloat64(SessionsActive) != initial-1 {
		t.Errorf("SessionsActive should be decremented by 1, got %f", testutil.ToFloat64(SessionsActive))
	}
}

func TestMetricsRegistration(t *testing.T) {
	// Test that all metrics are properly registered by checking they're not nil
	metrics := []interface{}{
		DiagnosticDuration,
		DiagnosticTotal,
		SSHConnectionAttempts,
		SSHConnectionDuration,
		AIAPICallDuration,
		AIAPICallTotal,
		AITokensUsed,
		RemediationActionsTotal,
		RemediationDuration,
		SessionsActive,
		SessionsTotal,
		CommandExecutionDuration,
		ErrorsTotal,
	}

	for i, metric := range metrics {
		if metric == nil {
			t.Errorf("Metric %d should not be nil", i)
		}
	}
}

func TestDiagnosticMetrics(t *testing.T) {
	// Record a diagnostic check
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	duration := time.Since(start).Seconds()

	DiagnosticDuration.WithLabelValues("cpu", "success").Observe(duration)
	DiagnosticTotal.WithLabelValues("cpu", "success").Inc()

	// Verify the counter was incremented
	count := testutil.ToFloat64(DiagnosticTotal.WithLabelValues("cpu", "success"))
	if count < 1 {
		t.Errorf("DiagnosticTotal should be at least 1, got %f", count)
	}
}

func TestSSHMetrics(t *testing.T) {
	SSHConnectionAttempts.WithLabelValues("example.com", "success").Inc()

	count := testutil.ToFloat64(SSHConnectionAttempts.WithLabelValues("example.com", "success"))
	if count < 1 {
		t.Errorf("SSHConnectionAttempts should be at least 1, got %f", count)
	}
}

func TestAIMetrics(t *testing.T) {
	// Record AI API call
	AIAPICallTotal.WithLabelValues("anthropic", "claude-3", "success").Inc()
	AITokensUsed.WithLabelValues("anthropic", "claude-3", "input").Add(100)
	AITokensUsed.WithLabelValues("anthropic", "claude-3", "output").Add(50)

	callCount := testutil.ToFloat64(AIAPICallTotal.WithLabelValues("anthropic", "claude-3", "success"))
	if callCount < 1 {
		t.Errorf("AIAPICallTotal should be at least 1, got %f", callCount)
	}

	inputTokens := testutil.ToFloat64(AITokensUsed.WithLabelValues("anthropic", "claude-3", "input"))
	if inputTokens < 100 {
		t.Errorf("AITokensUsed (input) should be at least 100, got %f", inputTokens)
	}
}
