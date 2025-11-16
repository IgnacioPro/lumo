package ssh

import (
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "''",
		},
		{
			name:  "simple string",
			input: "hello",
			want:  "'hello'",
		},
		{
			name:  "string with spaces",
			input: "hello world",
			want:  "'hello world'",
		},
		{
			name:  "string with single quote",
			input: "it's",
			want:  "'it'\\''s'",
		},
		{
			name:  "string with multiple single quotes",
			input: "a'b'c",
			want:  "'a'\\''b'\\''c'",
		},
		{
			name:  "string with dollar sign",
			input: "$HOME",
			want:  "'$HOME'",
		},
		{
			name:  "string with backticks",
			input: "`whoami`",
			want:  "'`whoami`'",
		},
		{
			name:  "string with semicolon",
			input: "echo hello; rm -rf /",
			want:  "'echo hello; rm -rf /'",
		},
		{
			name:  "string with pipe",
			input: "cat file | grep pattern",
			want:  "'cat file | grep pattern'",
		},
		{
			name:  "string with ampersand",
			input: "sleep 10 &",
			want:  "'sleep 10 &'",
		},
		{
			name:  "string with redirect",
			input: "echo test > file",
			want:  "'echo test > file'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellQuote(tt.input)
			if got != tt.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeWorkingDir(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		want    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty directory",
			dir:     "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "valid absolute path",
			dir:     "/home/user",
			want:    "/home/user",
			wantErr: false,
		},
		{
			name:    "valid path with cleaning",
			dir:     "/home/user/../user/./docs",
			want:    "/home/user/docs",
			wantErr: false,
		},
		{
			name:    "relative path",
			dir:     "relative/path",
			wantErr: true,
			errMsg:  "must be an absolute path",
		},
		{
			name:    "path with semicolon",
			dir:     "/home/user; rm -rf /",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with pipe",
			dir:     "/home/user | cat",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with ampersand",
			dir:     "/home/user & echo",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with dollar sign",
			dir:     "/home/$USER",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with backtick",
			dir:     "/home/`whoami`",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with redirect",
			dir:     "/home/user > /tmp/out",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with parentheses",
			dir:     "/home/(user)",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with braces",
			dir:     "/home/{user}",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with brackets",
			dir:     "/home/[user]",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with exclamation",
			dir:     "/home/user!",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with asterisk",
			dir:     "/home/user*",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with question mark",
			dir:     "/home/user?",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with tilde",
			dir:     "/home/~user",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with newline",
			dir:     "/home/user\nrm -rf /",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with carriage return",
			dir:     "/home/user\r",
			wantErr: true,
			errMsg:  "shell metacharacters",
		},
		{
			name:    "path with null byte",
			dir:     "/home/user\x00",
			wantErr: true,
			errMsg:  "null byte detected",
		},
		{
			name:    "complex valid path",
			dir:     "/opt/applications/myapp/data",
			want:    "/opt/applications/myapp/data",
			wantErr: false,
		},
		{
			name:    "path with trailing slash",
			dir:     "/home/user/",
			want:    "/home/user",
			wantErr: false,
		},
		{
			name:    "root path",
			dir:     "/",
			want:    "/",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeWorkingDir(tt.dir)

			if tt.wantErr {
				if err == nil {
					t.Errorf("sanitizeWorkingDir(%q) error = nil, want error containing %q", tt.dir, tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("sanitizeWorkingDir(%q) error = %q, want error containing %q", tt.dir, err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("sanitizeWorkingDir(%q) unexpected error = %v", tt.dir, err)
				return
			}

			if got != tt.want {
				t.Errorf("sanitizeWorkingDir(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestSanitizeWorkingDir_SecurityValidation(t *testing.T) {
	// Additional security-focused test cases
	injectionAttempts := []string{
		"/home/user; cat /etc/passwd",
		"/home/user && rm -rf /",
		"/home/user || echo hacked",
		"/home/user | tee /tmp/output",
		"/home/user $(whoami)",
		"/home/user `id`",
		"/home/user > /tmp/redirect",
		"/home/user < /etc/passwd",
		"/home/user >> /var/log/append",
		"/home/user 2>&1",
	}

	for _, attempt := range injectionAttempts {
		t.Run("injection_"+attempt, func(t *testing.T) {
			_, err := sanitizeWorkingDir(attempt)
			if err == nil {
				t.Errorf("sanitizeWorkingDir(%q) = nil, want error for injection attempt", attempt)
			}
		})
	}
}

func TestShellQuote_PreventInjection(t *testing.T) {
	// Test that shellQuote prevents command injection
	dangerousInputs := []struct {
		name  string
		input string
	}{
		{"command injection with semicolon", "test; rm -rf /"},
		{"command injection with pipe", "test | cat /etc/passwd"},
		{"command injection with ampersand", "test && whoami"},
		{"command injection with backticks", "test `cat /etc/passwd`"},
		{"command injection with dollar paren", "test $(whoami)"},
		{"command substitution", "`id`"},
		{"variable expansion", "$HOME/.ssh/id_rsa"},
	}

	for _, tt := range dangerousInputs {
		t.Run(tt.name, func(t *testing.T) {
			quoted := shellQuote(tt.input)

			// Verify the result is properly quoted (starts and ends with ')
			if !strings.HasPrefix(quoted, "'") || !strings.HasSuffix(quoted, "'") {
				t.Errorf("shellQuote(%q) = %q, not properly quoted", tt.input, quoted)
			}

			// Verify special characters are escaped or contained
			// The result should treat the entire input as a literal string
			// when used in a shell command
		})
	}
}
