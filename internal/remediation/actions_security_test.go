package remediation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// MockExecutor for testing command injection prevention
type MockSecurityExecutor struct {
	lastCommand string
	commands    []string
}

func (m *MockSecurityExecutor) ExecuteWithContext(ctx context.Context, command string) (string, string, int, error) {
	m.lastCommand = command
	m.commands = append(m.commands, command)

	// Handle specific validation commands
	if strings.Contains(command, "which systemctl") {
		// systemctl is available
		return "/usr/bin/systemctl", "", 0, nil
	}

	if strings.Contains(command, "systemctl list-unit-files") {
		// Extract service name from command and return appropriate output
		// Command format: systemctl list-unit-files 'servicename'.service
		// For valid service names like nginx, return proper output
		// Handle both 'nginx' and 'nginx.service' variations
		if strings.Contains(command, "nginx") {
			return "nginx.service                           enabled", "", 0, nil
		}
		// For invalid/injection attempts, return empty (service not found)
		return "", "", 0, nil
	}

	return "mocked output", "", 0, nil
}

func (m *MockSecurityExecutor) Execute(command string, timeout time.Duration) (string, string, int, error) {
	return m.ExecuteWithContext(context.Background(), command)
}

func (m *MockSecurityExecutor) getLastCommand() string {
	return m.lastCommand
}

// TestServiceActionCommandInjection tests that service names are properly validated and quoted
func TestServiceActionCommandInjection(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs during tests
	ctx := context.Background()

	injectionAttempts := []struct {
		name        string
		serviceName string
		expectError bool
		desc        string
	}{
		{
			name:        "normal service name",
			serviceName: "nginx",
			expectError: false,
			desc:        "valid service name should work",
		},
		{
			name:        "service with extension",
			serviceName: "nginx.service",
			expectError: false,
			desc:        "service name with .service extension should work",
		},
		{
			name:        "command injection - semicolon",
			serviceName: "nginx; rm -rf /",
			expectError: true,
			desc:        "semicolon command injection should be blocked",
		},
		{
			name:        "command injection - pipe",
			serviceName: "nginx | cat /etc/passwd",
			expectError: true,
			desc:        "pipe command injection should be blocked",
		},
		{
			name:        "command injection - ampersand",
			serviceName: "nginx && cat /etc/shadow",
			expectError: true,
			desc:        "ampersand command injection should be blocked",
		},
		{
			name:        "command injection - backtick",
			serviceName: "nginx`whoami`",
			expectError: true,
			desc:        "backtick command substitution should be blocked",
		},
		{
			name:        "command injection - dollar paren",
			serviceName: "nginx$(whoami)",
			expectError: true,
			desc:        "dollar paren command substitution should be blocked",
		},
		{
			name:        "command injection - newline",
			serviceName: "nginx\nrm -rf /",
			expectError: true,
			desc:        "newline injection should be blocked",
		},
		{
			name:        "command injection - redirect",
			serviceName: "nginx > /etc/passwd",
			expectError: true,
			desc:        "output redirection should be blocked",
		},
	}

	for _, tt := range injectionAttempts {
		t.Run(tt.name, func(t *testing.T) {
			executor := &MockSecurityExecutor{}

			// Test RestartServiceAction
			action := NewRestartServiceAction(tt.serviceName, logger)
			err := action.Validate(ctx, executor)

			if tt.expectError && err == nil {
				t.Errorf("RestartServiceAction.Validate() with %q expected error but got none (%s)",
					tt.serviceName, tt.desc)
			}

			if !tt.expectError && err != nil {
				t.Errorf("RestartServiceAction.Validate() with %q unexpected error: %v (%s)",
					tt.serviceName, err, tt.desc)
			}

			// If validation passed (for valid names), verify the command is properly quoted
			if !tt.expectError && err == nil && len(executor.commands) > 0 {
				lastCmd := executor.getLastCommand()
				// Should not contain unquoted dangerous characters
				if strings.Contains(tt.serviceName, ";") ||
					strings.Contains(tt.serviceName, "|") ||
					strings.Contains(tt.serviceName, "&") {
					// Command should have proper quoting
					if !strings.Contains(lastCmd, "'") {
						t.Errorf("Command not properly quoted: %s", lastCmd)
					}
				}
			}
		})
	}
}

// TestProcessActionCommandInjection tests that process patterns are properly validated and quoted
func TestProcessActionCommandInjection(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	ctx := context.Background()

	injectionAttempts := []struct {
		name           string
		processPattern string
		expectError    bool
		desc           string
	}{
		{
			name:           "normal process pattern",
			processPattern: "nginx",
			expectError:    false,
			desc:           "valid process pattern should work",
		},
		{
			name:           "process with path",
			processPattern: "/usr/bin/nginx",
			expectError:    false,
			desc:           "process pattern with path should work",
		},
		{
			name:           "command injection - quote escape",
			processPattern: "nginx' && cat /etc/shadow || pgrep -f '",
			expectError:    true,
			desc:           "quote escape injection should be blocked",
		},
		{
			name:           "command injection - semicolon",
			processPattern: "nginx; rm -rf /",
			expectError:    true,
			desc:           "semicolon command injection should be blocked",
		},
		{
			name:           "command injection - pipe",
			processPattern: "nginx | cat /etc/passwd",
			expectError:    true,
			desc:           "pipe command injection should be blocked",
		},
		{
			name:           "command injection - backtick",
			processPattern: "nginx`whoami`",
			expectError:    true,
			desc:           "backtick command substitution should be blocked",
		},
		{
			name:           "command injection - dollar paren",
			processPattern: "nginx$(whoami)",
			expectError:    true,
			desc:           "dollar paren command substitution should be blocked",
		},
	}

	for _, tt := range injectionAttempts {
		t.Run(tt.name, func(t *testing.T) {
			executor := &MockSecurityExecutor{}

			// Test KillProcessByNameAction
			action := NewKillProcessByNameAction(tt.processPattern, "SIGTERM", logger)
			err := action.Validate(ctx, executor)

			// Note: Validate will fail because pgrep won't find processes in our mock
			// So we check if the error is about validation (injection blocked) vs not finding processes
			if tt.expectError {
				if err == nil {
					t.Errorf("KillProcessByNameAction.Validate() with %q expected error but got none (%s)",
						tt.processPattern, tt.desc)
				} else {
					// Check if it's a validation error (good) not a "process not found" error
					if strings.Contains(err.Error(), "invalid process pattern") {
						// This is the expected validation error
						return
					}
					// If it's "no processes found", that means validation passed (bad for injection attempts)
					if strings.Contains(err.Error(), "no processes found") && tt.expectError {
						t.Errorf("KillProcessByNameAction.Validate() with %q should have failed validation but only failed to find processes (%s)",
							tt.processPattern, tt.desc)
					}
				}
			}
		})
	}
}

// TestShellQuoteAgainstKnownAttacks tests shellQuote against known attack vectors
func TestShellQuoteAgainstKnownAttacks(t *testing.T) {
	attacks := []struct {
		name  string
		input string
		desc  string
	}{
		{
			name:  "simple command injection",
			input: "test; rm -rf /",
			desc:  "semicolon command chaining",
		},
		{
			name:  "command substitution backticks",
			input: "test`whoami`",
			desc:  "backtick command substitution",
		},
		{
			name:  "command substitution dollar paren",
			input: "test$(whoami)",
			desc:  "dollar paren command substitution",
		},
		{
			name:  "pipe to command",
			input: "test | cat /etc/passwd",
			desc:  "pipe to exfiltrate data",
		},
		{
			name:  "background execution",
			input: "test & rm -rf /",
			desc:  "background command execution",
		},
		{
			name:  "logical AND",
			input: "test && cat /etc/shadow",
			desc:  "logical AND command chaining",
		},
		{
			name:  "logical OR",
			input: "test || cat /etc/shadow",
			desc:  "logical OR command chaining",
		},
		{
			name:  "output redirection",
			input: "test > /etc/passwd",
			desc:  "output redirection attack",
		},
		{
			name:  "input redirection",
			input: "test < /etc/passwd",
			desc:  "input redirection attack",
		},
		{
			name:  "newline injection",
			input: "test\nrm -rf /",
			desc:  "newline command injection",
		},
		{
			name:  "glob expansion",
			input: "test/*",
			desc:  "glob pattern expansion",
		},
		{
			name:  "variable expansion",
			input: "test$HOME",
			desc:  "variable expansion",
		},
	}

	for _, attack := range attacks {
		t.Run(attack.name, func(t *testing.T) {
			quoted := shellQuote(attack.input)

			// Verify that the result is wrapped in single quotes
			if !strings.HasPrefix(quoted, "'") || !strings.HasSuffix(quoted, "'") {
				t.Errorf("shellQuote(%q) = %q, should be wrapped in single quotes (%s)",
					attack.input, quoted, attack.desc)
			}

			// Verify that dangerous characters are inside the quotes (neutralized)
			// The only exception is if there were single quotes in the input,
			// which should be properly escaped
			if strings.Contains(attack.input, "'") {
				// Should contain '\'' escape sequences
				if !strings.Contains(quoted, `'\''`) {
					t.Errorf("shellQuote(%q) = %q, single quotes not properly escaped (%s)",
						attack.input, quoted, attack.desc)
				}
			}

			// The dangerous characters should be present but quoted
			// (we're not removing them, just making them literals)
			t.Logf("Attack vector %q quoted to: %s", attack.input, quoted)
		})
	}
}
