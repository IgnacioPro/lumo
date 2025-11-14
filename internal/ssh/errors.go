package ssh

import (
	"errors"
	"fmt"
)

// SSHError is the base error type for SSH-related errors
type SSHError struct {
	Op      string // Operation that failed
	Message string // Error message
	Err     error  // Underlying error
}

// Error implements the error interface
func (e *SSHError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// Unwrap returns the underlying error
func (e *SSHError) Unwrap() error {
	return e.Err
}

// ConnectionError represents connection-related errors
type ConnectionError struct {
	Host    string
	Port    int
	User    string
	Message string
	Err     error
}

// Error implements the error interface
func (e *ConnectionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("connection failed to %s@%s:%d: %s: %v", e.User, e.Host, e.Port, e.Message, e.Err)
	}
	return fmt.Sprintf("connection failed to %s@%s:%d: %s", e.User, e.Host, e.Port, e.Message)
}

// Unwrap returns the underlying error
func (e *ConnectionError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the connection error is transient and can be retried
func (e *ConnectionError) IsRetryable() bool {
	// Network errors, timeouts, and connection refused are retryable
	// Authentication errors and invalid hosts are not retryable
	if e.Err == nil {
		return false
	}

	errStr := e.Err.Error()
	// Check for retryable error patterns
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"i/o timeout",
		"network is unreachable",
		"no route to host",
		"temporary failure",
	}

	for _, pattern := range retryablePatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// AuthenticationError represents authentication-related errors
type AuthenticationError struct {
	Host       string
	User       string
	Method     AuthMethod
	Message    string
	Err        error
	Suggestion string // Helpful suggestion for the user
}

// Error implements the error interface
func (e *AuthenticationError) Error() string {
	msg := fmt.Sprintf("authentication failed for %s@%s using %s: %s", e.User, e.Host, e.Method, e.Message)
	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}
	if e.Suggestion != "" {
		msg += fmt.Sprintf(" (suggestion: %s)", e.Suggestion)
	}
	return msg
}

// Unwrap returns the underlying error
func (e *AuthenticationError) Unwrap() error {
	return e.Err
}

// CommandError represents command execution errors
type CommandError struct {
	Command  string
	Stdout   string
	Stderr   string
	ExitCode int
	Message  string
	Err      error
}

// Error implements the error interface
func (e *CommandError) Error() string {
	msg := fmt.Sprintf("command failed: %s", e.Command)
	if e.ExitCode != 0 {
		msg += fmt.Sprintf(" (exit code: %d)", e.ExitCode)
	}
	if e.Message != "" {
		msg += fmt.Sprintf(": %s", e.Message)
	}
	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}
	return msg
}

// Unwrap returns the underlying error
func (e *CommandError) Unwrap() error {
	return e.Err
}

// Output returns the combined output with a clear format
func (e *CommandError) Output() string {
	result := ""
	if e.Stdout != "" {
		result += fmt.Sprintf("stdout:\n%s\n", e.Stdout)
	}
	if e.Stderr != "" {
		result += fmt.Sprintf("stderr:\n%s\n", e.Stderr)
	}
	return result
}

// TimeoutError represents timeout errors
type TimeoutError struct {
	Operation string
	Timeout   string
	Message   string
}

// Error implements the error interface
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timeout: %s exceeded %s: %s", e.Operation, e.Timeout, e.Message)
}

// HealthCheckError represents health check failures
type HealthCheckError struct {
	Host    string
	Message string
	Err     error
}

// Error implements the error interface
func (e *HealthCheckError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("health check failed for %s: %s: %v", e.Host, e.Message, e.Err)
	}
	return fmt.Sprintf("health check failed for %s: %s", e.Host, e.Message)
}

// Unwrap returns the underlying error
func (e *HealthCheckError) Unwrap() error {
	return e.Err
}

// Helper functions for error type checking

// IsConnectionError checks if the error is a ConnectionError
func IsConnectionError(err error) bool {
	var connErr *ConnectionError
	return errors.As(err, &connErr)
}

// IsAuthenticationError checks if the error is an AuthenticationError
func IsAuthenticationError(err error) bool {
	var authErr *AuthenticationError
	return errors.As(err, &authErr)
}

// IsCommandError checks if the error is a CommandError
func IsCommandError(err error) bool {
	var cmdErr *CommandError
	return errors.As(err, &cmdErr)
}

// IsTimeoutError checks if the error is a TimeoutError
func IsTimeoutError(err error) bool {
	var timeoutErr *TimeoutError
	return errors.As(err, &timeoutErr)
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	// Check if it's a connection error and retryable
	var connErr *ConnectionError
	if errors.As(err, &connErr) {
		return connErr.IsRetryable()
	}

	// Timeout errors are generally retryable
	if IsTimeoutError(err) {
		return true
	}

	// Authentication errors are not retryable
	if IsAuthenticationError(err) {
		return false
	}

	// Command errors are not retryable by default
	if IsCommandError(err) {
		return false
	}

	return false
}

// NewConnectionError creates a new ConnectionError
func NewConnectionError(host string, port int, user, message string, err error) *ConnectionError {
	return &ConnectionError{
		Host:    host,
		Port:    port,
		User:    user,
		Message: message,
		Err:     err,
	}
}

// NewAuthenticationError creates a new AuthenticationError
func NewAuthenticationError(host, user string, method AuthMethod, message string, err error) *AuthenticationError {
	authErr := &AuthenticationError{
		Host:    host,
		User:    user,
		Method:  method,
		Message: message,
		Err:     err,
	}

	// Add helpful suggestions based on auth method
	switch method {
	case AuthMethodAgent:
		authErr.Suggestion = "ensure SSH agent is running and has keys loaded (ssh-add -l)"
	case AuthMethodKey:
		authErr.Suggestion = "verify the key file exists and has correct permissions (chmod 600)"
	case AuthMethodPassword:
		authErr.Suggestion = "verify the password is correct"
	case AuthMethodInteractive:
		authErr.Suggestion = "ensure keyboard-interactive authentication is enabled on the server"
	}

	return authErr
}

// NewCommandError creates a new CommandError
func NewCommandError(command, message string, exitCode int, stdout, stderr string, err error) *CommandError {
	return &CommandError{
		Command:  command,
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
		Message:  message,
		Err:      err,
	}
}

// NewTimeoutError creates a new TimeoutError
func NewTimeoutError(operation, timeout, message string) *TimeoutError {
	return &TimeoutError{
		Operation: operation,
		Timeout:   timeout,
		Message:   message,
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
