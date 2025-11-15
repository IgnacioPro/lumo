package ssh

import (
	"errors"
	"testing"
)

func TestSSHError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *SSHError
		want string
	}{
		{
			name: "with underlying error",
			err: &SSHError{
				Op:      "connect",
				Message: "connection failed",
				Err:     errors.New("network unreachable"),
			},
			want: "connect: connection failed: network unreachable",
		},
		{
			name: "without underlying error",
			err: &SSHError{
				Op:      "execute",
				Message: "command not found",
				Err:     nil,
			},
			want: "execute: command not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("SSHError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSSHError_Unwrap(t *testing.T) {
	underlying := errors.New("test error")
	err := &SSHError{
		Op:      "test",
		Message: "test message",
		Err:     underlying,
	}

	if unwrapped := err.Unwrap(); unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}

func TestConnectionError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ConnectionError
		want string
	}{
		{
			name: "with underlying error",
			err: &ConnectionError{
				Host:    "example.com",
				Port:    22,
				User:    "admin",
				Message: "timeout",
				Err:     errors.New("i/o timeout"),
			},
			want: "connection failed to admin@example.com:22: timeout: i/o timeout",
		},
		{
			name: "without underlying error",
			err: &ConnectionError{
				Host:    "test.local",
				Port:    2222,
				User:    "user",
				Message: "refused",
				Err:     nil,
			},
			want: "connection failed to user@test.local:2222: refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("ConnectionError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConnectionError_IsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  *ConnectionError
		want bool
	}{
		{
			name: "retryable - connection refused",
			err: &ConnectionError{
				Host: "example.com",
				Port: 22,
				User: "admin",
				Err:  errors.New("connection refused"),
			},
			want: true,
		},
		{
			name: "retryable - connection timeout",
			err: &ConnectionError{
				Host: "example.com",
				Port: 22,
				User: "admin",
				Err:  errors.New("connection timeout"),
			},
			want: true,
		},
		{
			name: "retryable - i/o timeout",
			err: &ConnectionError{
				Host: "example.com",
				Port: 22,
				User: "admin",
				Err:  errors.New("i/o timeout"),
			},
			want: true,
		},
		{
			name: "retryable - network unreachable",
			err: &ConnectionError{
				Host: "example.com",
				Port: 22,
				User: "admin",
				Err:  errors.New("network is unreachable"),
			},
			want: true,
		},
		{
			name: "not retryable - auth failed",
			err: &ConnectionError{
				Host: "example.com",
				Port: 22,
				User: "admin",
				Err:  errors.New("authentication failed"),
			},
			want: false,
		},
		{
			name: "not retryable - nil error",
			err: &ConnectionError{
				Host: "example.com",
				Port: 22,
				User: "admin",
				Err:  nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.IsRetryable(); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthenticationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *AuthenticationError
		want string
	}{
		{
			name: "with suggestion and error",
			err: &AuthenticationError{
				Host:       "example.com",
				User:       "admin",
				Method:     AuthMethodKey,
				Message:    "invalid key",
				Err:        errors.New("permission denied"),
				Suggestion: "check key permissions",
			},
			want: "authentication failed for admin@example.com using private-key: invalid key: permission denied (suggestion: check key permissions)",
		},
		{
			name: "without error",
			err: &AuthenticationError{
				Host:       "example.com",
				User:       "admin",
				Method:     AuthMethodPassword,
				Message:    "wrong password",
				Err:        nil,
				Suggestion: "verify password",
			},
			want: "authentication failed for admin@example.com using password: wrong password (suggestion: verify password)",
		},
		{
			name: "without suggestion",
			err: &AuthenticationError{
				Host:    "example.com",
				User:    "admin",
				Method:  AuthMethodAgent,
				Message: "no agent",
				Err:     errors.New("agent not available"),
			},
			want: "authentication failed for admin@example.com using ssh-agent: no agent: agent not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("AuthenticationError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *CommandError
		want string
	}{
		{
			name: "full error",
			err: &CommandError{
				Command:  "ls /nonexistent",
				ExitCode: 2,
				Message:  "directory not found",
				Err:      errors.New("exit status 2"),
			},
			want: "command failed: ls /nonexistent (exit code: 2): directory not found: exit status 2",
		},
		{
			name: "without underlying error",
			err: &CommandError{
				Command:  "echo test",
				ExitCode: 1,
				Message:  "failed",
			},
			want: "command failed: echo test (exit code: 1): failed",
		},
		{
			name: "without message",
			err: &CommandError{
				Command:  "pwd",
				ExitCode: 0,
			},
			want: "command failed: pwd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("CommandError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandError_Output(t *testing.T) {
	tests := []struct {
		name string
		err  *CommandError
		want string
	}{
		{
			name: "both stdout and stderr",
			err: &CommandError{
				Stdout: "output line",
				Stderr: "error line",
			},
			want: "stdout:\noutput line\nstderr:\nerror line\n",
		},
		{
			name: "stdout only",
			err: &CommandError{
				Stdout: "only output",
				Stderr: "",
			},
			want: "stdout:\nonly output\n",
		},
		{
			name: "stderr only",
			err: &CommandError{
				Stdout: "",
				Stderr: "only error",
			},
			want: "stderr:\nonly error\n",
		},
		{
			name: "empty output",
			err: &CommandError{
				Stdout: "",
				Stderr: "",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Output(); got != tt.want {
				t.Errorf("Output() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTimeoutError_Error(t *testing.T) {
	err := &TimeoutError{
		Operation: "command execution",
		Timeout:   "30s",
		Message:   "command did not complete",
	}

	want := "timeout: command execution exceeded 30s: command did not complete"
	if got := err.Error(); got != want {
		t.Errorf("TimeoutError.Error() = %v, want %v", got, want)
	}
}

func TestHealthCheckError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *HealthCheckError
		want string
	}{
		{
			name: "with underlying error",
			err: &HealthCheckError{
				Host:    "example.com",
				Message: "ping failed",
				Err:     errors.New("timeout"),
			},
			want: "health check failed for example.com: ping failed: timeout",
		},
		{
			name: "without underlying error",
			err: &HealthCheckError{
				Host:    "test.local",
				Message: "connection lost",
				Err:     nil,
			},
			want: "health check failed for test.local: connection lost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("HealthCheckError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConnectionError(t *testing.T) {
	connErr := &ConnectionError{Host: "test", Port: 22, User: "user"}
	regularErr := errors.New("regular error")

	if !IsConnectionError(connErr) {
		t.Error("IsConnectionError returned false for ConnectionError")
	}

	if IsConnectionError(regularErr) {
		t.Error("IsConnectionError returned true for regular error")
	}
}

func TestIsAuthenticationError(t *testing.T) {
	authErr := &AuthenticationError{Host: "test", User: "user", Method: AuthMethodKey}
	regularErr := errors.New("regular error")

	if !IsAuthenticationError(authErr) {
		t.Error("IsAuthenticationError returned false for AuthenticationError")
	}

	if IsAuthenticationError(regularErr) {
		t.Error("IsAuthenticationError returned true for regular error")
	}
}

func TestIsCommandError(t *testing.T) {
	cmdErr := &CommandError{Command: "test"}
	regularErr := errors.New("regular error")

	if !IsCommandError(cmdErr) {
		t.Error("IsCommandError returned false for CommandError")
	}

	if IsCommandError(regularErr) {
		t.Error("IsCommandError returned true for regular error")
	}
}

func TestIsTimeoutError(t *testing.T) {
	timeoutErr := &TimeoutError{Operation: "test", Timeout: "30s"}
	regularErr := errors.New("regular error")

	if !IsTimeoutError(timeoutErr) {
		t.Error("IsTimeoutError returned false for TimeoutError")
	}

	if IsTimeoutError(regularErr) {
		t.Error("IsTimeoutError returned true for regular error")
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "retryable connection error",
			err: &ConnectionError{
				Err: errors.New("connection refused"),
			},
			want: true,
		},
		{
			name: "timeout error",
			err:  &TimeoutError{Operation: "test"},
			want: true,
		},
		{
			name: "authentication error",
			err:  &AuthenticationError{Host: "test", User: "user"},
			want: false,
		},
		{
			name: "command error",
			err:  &CommandError{Command: "test"},
			want: false,
		},
		{
			name: "non-retryable connection error",
			err: &ConnectionError{
				Err: errors.New("authentication failed"),
			},
			want: false,
		},
		{
			name: "regular error",
			err:  errors.New("test error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewConnectionError(t *testing.T) {
	underlying := errors.New("test error")
	err := NewConnectionError("example.com", 22, "admin", "connection failed", underlying)

	if err.Host != "example.com" {
		t.Errorf("Host = %s, want example.com", err.Host)
	}
	if err.Port != 22 {
		t.Errorf("Port = %d, want 22", err.Port)
	}
	if err.User != "admin" {
		t.Errorf("User = %s, want admin", err.User)
	}
	if err.Message != "connection failed" {
		t.Errorf("Message = %s, want connection failed", err.Message)
	}
	if err.Err != underlying {
		t.Errorf("Err = %v, want %v", err.Err, underlying)
	}
}

func TestNewAuthenticationError(t *testing.T) {
	tests := []struct {
		name           string
		method         AuthMethod
		wantSuggestion string
	}{
		{
			name:           "agent method",
			method:         AuthMethodAgent,
			wantSuggestion: "ensure SSH agent is running and has keys loaded (ssh-add -l)",
		},
		{
			name:           "key method",
			method:         AuthMethodKey,
			wantSuggestion: "verify the key file exists and has correct permissions (chmod 600)",
		},
		{
			name:           "password method",
			method:         AuthMethodPassword,
			wantSuggestion: "verify the password is correct",
		},
		{
			name:           "interactive method",
			method:         AuthMethodInteractive,
			wantSuggestion: "ensure keyboard-interactive authentication is enabled on the server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewAuthenticationError("example.com", "admin", tt.method, "auth failed", nil)

			if err.Host != "example.com" {
				t.Errorf("Host = %s, want example.com", err.Host)
			}
			if err.User != "admin" {
				t.Errorf("User = %s, want admin", err.User)
			}
			if err.Method != tt.method {
				t.Errorf("Method = %v, want %v", err.Method, tt.method)
			}
			if err.Suggestion != tt.wantSuggestion {
				t.Errorf("Suggestion = %s, want %s", err.Suggestion, tt.wantSuggestion)
			}
		})
	}
}

func TestNewCommandError(t *testing.T) {
	underlying := errors.New("test error")
	err := NewCommandError("ls -la", "command failed", 1, "stdout", "stderr", underlying)

	if err.Command != "ls -la" {
		t.Errorf("Command = %s, want ls -la", err.Command)
	}
	if err.Message != "command failed" {
		t.Errorf("Message = %s, want command failed", err.Message)
	}
	if err.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", err.ExitCode)
	}
	if err.Stdout != "stdout" {
		t.Errorf("Stdout = %s, want stdout", err.Stdout)
	}
	if err.Stderr != "stderr" {
		t.Errorf("Stderr = %s, want stderr", err.Stderr)
	}
	if err.Err != underlying {
		t.Errorf("Err = %v, want %v", err.Err, underlying)
	}
}

func TestNewTimeoutError(t *testing.T) {
	err := NewTimeoutError("test operation", "30s", "operation timed out")

	if err.Operation != "test operation" {
		t.Errorf("Operation = %s, want test operation", err.Operation)
	}
	if err.Timeout != "30s" {
		t.Errorf("Timeout = %s, want 30s", err.Timeout)
	}
	if err.Message != "operation timed out" {
		t.Errorf("Message = %s, want operation timed out", err.Message)
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{"exact match", "connection refused", "connection refused", true},
		{"substring at start", "connection refused here", "connection", true},
		{"substring in middle", "the connection refused here", "connection", true},
		{"substring at end", "connection", "connection", true},
		{"not found", "network error", "timeout", false},
		{"empty substring", "test", "", true},
		{"empty string", "", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contains(tt.s, tt.substr); got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}
