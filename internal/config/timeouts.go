package config

import "time"

// Timeout constants for various operations across the codebase.
// Centralized here for consistency and easy tuning.
const (
	// DefaultHTTPTimeout is the default timeout for HTTP requests.
	DefaultHTTPTimeout = 30 * time.Second

	// HealthCheckTimeout is the timeout for health check operations.
	HealthCheckTimeout = 5 * time.Second

	// AIAnalysisTimeout is the timeout for AI analysis operations.
	AIAnalysisTimeout = 60 * time.Second

	// DiagnosticsTimeout is the timeout for diagnostic check operations.
	DiagnosticsTimeout = 2 * time.Minute

	// SSHConnectionTimeout is the default timeout for SSH connections.
	SSHConnectionTimeout = 30 * time.Second

	// DatabaseConnectionTimeout is the timeout for database connections.
	DatabaseConnectionTimeout = 10 * time.Second

	// GracefulShutdownTimeout is the timeout for graceful server shutdown.
	GracefulShutdownTimeout = 30 * time.Second

	// AsyncProcessingTimeout is the timeout for async event processing.
	AsyncProcessingTimeout = 5 * time.Minute
)
