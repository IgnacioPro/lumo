package remediation

import (
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		desc     string
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: "'hello'",
			desc:     "basic alphanumeric string",
		},
		{
			name:     "string with spaces",
			input:    "hello world",
			expected: "'hello world'",
			desc:     "string containing spaces",
		},
		{
			name:     "string with single quote",
			input:    "it's",
			expected: "'it'\\''s'",
			desc:     "string containing single quote",
		},
		{
			name:     "string with multiple single quotes",
			input:    "a'b'c",
			expected: "'a'\\''b'\\''c'",
			desc:     "string with multiple single quotes",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "''",
			desc:     "empty string",
		},
		{
			name:     "command injection attempt - semicolon",
			input:    "nginx; rm -rf /",
			expected: "'nginx; rm -rf /'",
			desc:     "command injection with semicolon",
		},
		{
			name:     "command injection attempt - backticks",
			input:    "`whoami`",
			expected: "'`whoami`'",
			desc:     "command substitution with backticks",
		},
		{
			name:     "command injection attempt - dollar",
			input:    "$(cat /etc/passwd)",
			expected: "'$(cat /etc/passwd)'",
			desc:     "command substitution with $(...)",
		},
		{
			name:     "command injection attempt - pipe",
			input:    "nginx | cat /etc/passwd",
			expected: "'nginx | cat /etc/passwd'",
			desc:     "command injection with pipe",
		},
		{
			name:     "command injection attempt - ampersand",
			input:    "nginx && cat /etc/shadow",
			expected: "'nginx && cat /etc/shadow'",
			desc:     "command injection with &&",
		},
		{
			name:     "variable expansion attempt",
			input:    "$HOME",
			expected: "'$HOME'",
			desc:     "variable expansion attempt",
		},
		{
			name:     "glob pattern",
			input:    "*.conf",
			expected: "'*.conf'",
			desc:     "glob pattern",
		},
		{
			name:     "redirect attempt",
			input:    "> /etc/passwd",
			expected: "'> /etc/passwd'",
			desc:     "output redirection attempt",
		},
		{
			name:     "newline injection",
			input:    "nginx\nrm -rf /",
			expected: "'nginx\nrm -rf /'",
			desc:     "newline injection attempt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shellQuote(tt.input)
			if result != tt.expected {
				t.Errorf("shellQuote(%q) = %q, expected %q (%s)", tt.input, result, tt.expected, tt.desc)
			}

			// Verify that the quoted string doesn't contain unescaped dangerous characters
			// (except within the quotes themselves)
			if strings.Contains(tt.input, ";") || strings.Contains(tt.input, "|") ||
				strings.Contains(tt.input, "&") || strings.Contains(tt.input, "`") {
				// The result should wrap these in quotes, making them literals
				if !strings.HasPrefix(result, "'") || !strings.HasSuffix(result, "'") {
					t.Errorf("shellQuote(%q) didn't properly wrap dangerous characters: %q", tt.input, result)
				}
			}
		})
	}
}

func TestIsValidServiceName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		desc     string
	}{
		{
			name:     "valid simple name",
			input:    "nginx",
			expected: true,
			desc:     "simple service name",
		},
		{
			name:     "valid name with extension",
			input:    "nginx.service",
			expected: true,
			desc:     "service name with .service extension",
		},
		{
			name:     "valid name with dash",
			input:    "docker-compose",
			expected: true,
			desc:     "service name with dash",
		},
		{
			name:     "valid name with underscore",
			input:    "my_service",
			expected: true,
			desc:     "service name with underscore",
		},
		{
			name:     "valid name with dots",
			input:    "com.example.service",
			expected: true,
			desc:     "service name with dots",
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
			desc:     "empty service name",
		},
		{
			name:     "command injection - semicolon",
			input:    "nginx; rm -rf /",
			expected: false,
			desc:     "service name with command injection",
		},
		{
			name:     "command injection - pipe",
			input:    "nginx | cat /etc/passwd",
			expected: false,
			desc:     "service name with pipe",
		},
		{
			name:     "command injection - ampersand",
			input:    "nginx && whoami",
			expected: false,
			desc:     "service name with ampersand",
		},
		{
			name:     "command injection - backtick",
			input:    "nginx`whoami`",
			expected: false,
			desc:     "service name with backticks",
		},
		{
			name:     "command injection - dollar",
			input:    "nginx$(whoami)",
			expected: false,
			desc:     "service name with command substitution",
		},
		{
			name:     "command injection - redirect",
			input:    "nginx > /tmp/out",
			expected: false,
			desc:     "service name with redirect",
		},
		{
			name:     "too long",
			input:    strings.Repeat("a", 257),
			expected: false,
			desc:     "service name exceeding 256 chars",
		},
		{
			name:     "max length",
			input:    strings.Repeat("a", 256),
			expected: true,
			desc:     "service name at 256 char limit",
		},
		{
			name:     "newline injection",
			input:    "nginx\nrm -rf /",
			expected: false,
			desc:     "service name with newline",
		},
		{
			name:     "tab injection",
			input:    "nginx\trm -rf /",
			expected: false,
			desc:     "service name with tab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidServiceName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidServiceName(%q) = %v, expected %v (%s)", tt.input, result, tt.expected, tt.desc)
			}
		})
	}
}

func TestIsValidProcessPattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		desc     string
	}{
		{
			name:     "valid simple pattern",
			input:    "nginx",
			expected: true,
			desc:     "simple process name",
		},
		{
			name:     "valid path pattern",
			input:    "/usr/bin/nginx",
			expected: true,
			desc:     "full path process pattern",
		},
		{
			name:     "valid pattern with dash",
			input:    "node-app",
			expected: true,
			desc:     "process pattern with dash",
		},
		{
			name:     "valid pattern with underscore",
			input:    "my_process",
			expected: true,
			desc:     "process pattern with underscore",
		},
		{
			name:     "valid pattern with dots",
			input:    "app.js",
			expected: true,
			desc:     "process pattern with dots",
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
			desc:     "empty process pattern",
		},
		{
			name:     "command injection - semicolon",
			input:    "nginx; rm -rf /",
			expected: false,
			desc:     "process pattern with command injection",
		},
		{
			name:     "command injection - pipe",
			input:    "nginx | cat /etc/passwd",
			expected: false,
			desc:     "process pattern with pipe",
		},
		{
			name:     "command injection - single quote escape",
			input:    "nginx' && cat /etc/shadow || pgrep -f '",
			expected: false,
			desc:     "process pattern with quote escape injection",
		},
		{
			name:     "command injection - backtick",
			input:    "nginx`whoami`",
			expected: false,
			desc:     "process pattern with backticks",
		},
		{
			name:     "command injection - dollar",
			input:    "nginx$(whoami)",
			expected: false,
			desc:     "process pattern with command substitution",
		},
		{
			name:     "too long",
			input:    strings.Repeat("a", 1025),
			expected: false,
			desc:     "process pattern exceeding 1024 chars",
		},
		{
			name:     "max length",
			input:    strings.Repeat("a", 1024),
			expected: true,
			desc:     "process pattern at 1024 char limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidProcessPattern(tt.input)
			if result != tt.expected {
				t.Errorf("isValidProcessPattern(%q) = %v, expected %v (%s)", tt.input, result, tt.expected, tt.desc)
			}
		})
	}
}

func TestValidateServiceName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		desc        string
	}{
		{
			name:        "valid service name",
			input:       "nginx",
			expectError: false,
			desc:        "valid service name should not error",
		},
		{
			name:        "invalid service name with injection",
			input:       "nginx; rm -rf /",
			expectError: true,
			desc:        "service name with command injection should error",
		},
		{
			name:        "empty service name",
			input:       "",
			expectError: true,
			desc:        "empty service name should error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServiceName(tt.input)
			if tt.expectError && err == nil {
				t.Errorf("validateServiceName(%q) expected error but got none (%s)", tt.input, tt.desc)
			}
			if !tt.expectError && err != nil {
				t.Errorf("validateServiceName(%q) unexpected error: %v (%s)", tt.input, err, tt.desc)
			}
		})
	}
}

func TestValidateProcessPattern(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		desc        string
	}{
		{
			name:        "valid process pattern",
			input:       "nginx",
			expectError: false,
			desc:        "valid process pattern should not error",
		},
		{
			name:        "invalid process pattern with injection",
			input:       "nginx' && cat /etc/shadow",
			expectError: true,
			desc:        "process pattern with command injection should error",
		},
		{
			name:        "empty process pattern",
			input:       "",
			expectError: true,
			desc:        "empty process pattern should error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProcessPattern(tt.input)
			if tt.expectError && err == nil {
				t.Errorf("validateProcessPattern(%q) expected error but got none (%s)", tt.input, tt.desc)
			}
			if !tt.expectError && err != nil {
				t.Errorf("validateProcessPattern(%q) unexpected error: %v (%s)", tt.input, err, tt.desc)
			}
		})
	}
}
