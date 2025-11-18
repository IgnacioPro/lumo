package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// DiagnosticDuration tracks the duration of diagnostic checks
	DiagnosticDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lumo_diagnostic_duration_seconds",
			Help:    "Duration of diagnostic checks in seconds",
			Buckets: prometheus.DefBuckets, // 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
		},
		[]string{"check", "status"},
	)

	// DiagnosticTotal counts total diagnostic checks run
	DiagnosticTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_diagnostic_total",
			Help: "Total number of diagnostic checks run",
		},
		[]string{"check", "status"},
	)

	// SSHConnectionAttempts tracks SSH connection attempts
	SSHConnectionAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_ssh_connection_attempts_total",
			Help: "Total SSH connection attempts",
		},
		[]string{"host", "result"}, // result: success, failed, timeout
	)

	// SSHConnectionDuration tracks SSH connection establishment time
	SSHConnectionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lumo_ssh_connection_duration_seconds",
			Help:    "SSH connection establishment duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"host"},
	)

	// AIAPICallDuration tracks AI API call duration
	AIAPICallDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lumo_ai_api_duration_seconds",
			Help:    "AI API call duration in seconds",
			Buckets: []float64{0.5, 1, 2, 5, 10, 30, 60, 120},
		},
		[]string{"provider", "model", "status"}, // status: success, error, timeout
	)

	// AIAPICallTotal counts total AI API calls
	AIAPICallTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_ai_api_calls_total",
			Help: "Total number of AI API calls",
		},
		[]string{"provider", "model", "status"},
	)

	// AITokensUsed tracks token usage for AI providers
	AITokensUsed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_ai_tokens_used_total",
			Help: "Total number of AI tokens used",
		},
		[]string{"provider", "model", "type"}, // type: input, output
	)

	// RemediationActionsTotal counts remediation actions
	RemediationActionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_remediation_actions_total",
			Help: "Total number of remediation actions attempted",
		},
		[]string{"action_type", "risk_level", "status"}, // status: approved, rejected, success, failed
	)

	// RemediationDuration tracks remediation execution time
	RemediationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lumo_remediation_duration_seconds",
			Help:    "Remediation action execution duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"action_type"},
	)

	// SessionsActive tracks active diagnostic sessions
	SessionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lumo_sessions_active",
			Help: "Number of currently active diagnostic sessions",
		},
	)

	// SessionsTotal counts total sessions started
	SessionsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "lumo_sessions_total",
			Help: "Total number of diagnostic sessions started",
		},
	)

	// CommandExecutionDuration tracks command execution time
	CommandExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lumo_command_execution_duration_seconds",
			Help:    "Command execution duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10},
		},
		[]string{"executor_type"}, // executor_type: ssh, local
	)

	// ErrorsTotal counts errors by category
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_errors_total",
			Help: "Total number of errors by category",
		},
		[]string{"category", "severity"}, // category: ssh, ai, diagnostic, remediation; severity: warning, error, critical
	)
)

// RecordSessionStart increments active sessions and total sessions
func RecordSessionStart() {
	SessionsActive.Inc()
	SessionsTotal.Inc()
}

// RecordSessionEnd decrements active sessions
func RecordSessionEnd() {
	SessionsActive.Dec()
}
