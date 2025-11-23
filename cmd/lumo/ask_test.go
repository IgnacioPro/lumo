package main

import (
	"strings"
	"testing"
)

func TestValidateProposedCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantErr     bool
		errContains string
	}{
		// Valid commands
		{
			name:    "valid lumo diagnose command",
			command: "lumo diagnose --checks cpu",
			wantErr: false,
		},
		{
			name:    "valid lumo doctor command",
			command: "lumo doctor",
			wantErr: false,
		},
		{
			name:    "valid lumo version command",
			command: "lumo version",
			wantErr: false,
		},
		{
			name:    "valid diagnose with analyze flag",
			command: "lumo diagnose --analyze",
			wantErr: false,
		},
		{
			name:    "valid diagnose with multiple flags",
			command: "lumo diagnose --checks cpu,memory --analyze --format json",
			wantErr: false,
		},

		// Invalid: Missing "lumo" prefix
		{
			name:        "command without lumo prefix",
			command:     "diagnose --checks cpu",
			wantErr:     true,
			errContains: "must start with 'lumo'",
		},
		{
			name:        "arbitrary shell command",
			command:     "rm -rf /",
			wantErr:     true,
			errContains: "must start with 'lumo'",
		},
		{
			name:        "empty command",
			command:     "",
			wantErr:     true,
			errContains: "must start with 'lumo'",
		},

		// Invalid: Recursive ask commands
		{
			name:        "recursive ask command",
			command:     "lumo ask 'check cpu'",
			wantErr:     true,
			errContains: "recursive",
		},
		{
			name:        "recursive ask with flags",
			command:     "lumo ask --yes 'run diagnostics'",
			wantErr:     true,
			errContains: "recursive",
		},

		// Invalid: Shell operators
		{
			name:        "pipe operator",
			command:     "lumo diagnose | grep error",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "semicolon separator",
			command:     "lumo diagnose; rm -rf /",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "ampersand background",
			command:     "lumo diagnose &",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "dollar variable",
			command:     "lumo diagnose $HOME",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "backtick command substitution",
			command:     "lumo diagnose `whoami`",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "redirect output",
			command:     "lumo diagnose > file.txt",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "redirect input",
			command:     "lumo diagnose < file.txt",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "parentheses subshell",
			command:     "lumo diagnose (date)",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "braces command grouping",
			command:     "lumo diagnose {date}",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "square brackets",
			command:     "lumo diagnose [test]",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "exclamation history expansion",
			command:     "lumo diagnose !!",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "asterisk wildcard",
			command:     "lumo diagnose *",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "question mark wildcard",
			command:     "lumo diagnose test?",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "tilde home directory",
			command:     "lumo diagnose ~/test",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "newline injection",
			command:     "lumo diagnose\nrm -rf /",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},
		{
			name:        "carriage return injection",
			command:     "lumo diagnose\rrm -rf /",
			wantErr:     true,
			errContains: "unsafe shell characters",
		},

		// Invalid: Null bytes
		{
			name:        "null byte injection",
			command:     "lumo diagnose\x00rm -rf /",
			wantErr:     true,
			errContains: "null bytes",
		},

		// Edge cases
		{
			name:    "lumo with trailing space",
			command: "lumo diagnose ",
			wantErr: false,
		},
		{
			name:    "lumo with multiple spaces",
			command: "lumo  diagnose  --checks  cpu",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProposedCommand(tt.command)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateProposedCommand() error = nil, wantErr = true")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("validateProposedCommand() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("validateProposedCommand() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestValidateProposedCommand_AllDangerousChars ensures all dangerous characters are blocked
func TestValidateProposedCommand_AllDangerousChars(t *testing.T) {
	dangerousChars := ";|&$`<>(){}[]!*?~\n\r"

	for _, char := range dangerousChars {
		t.Run(string(char), func(t *testing.T) {
			command := "lumo diagnose " + string(char)
			err := validateProposedCommand(command)

			if err == nil {
				t.Errorf("validateProposedCommand() should reject character %q but accepted it", char)
			}

			if !strings.Contains(err.Error(), "unsafe shell characters") {
				t.Errorf("validateProposedCommand() error = %v, want error containing 'unsafe shell characters'", err)
			}
		})
	}
}

// TestValidateProposedCommand_SecurityBoundaries tests security boundary conditions
func TestValidateProposedCommand_SecurityBoundaries(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantErr     bool
		errContains string
		description string
	}{
		{
			name:        "double recursive ask",
			command:     "lumo ask ask 'check cpu'",
			wantErr:     true,
			errContains: "recursive",
			description: "Should block even double-nested ask commands",
		},
		{
			name:        "ask in middle of command",
			command:     "lumo diagnose ask --checks cpu",
			wantErr:     false,
			description: "Should allow 'ask' as a flag value (not a subcommand)",
		},
		{
			name:        "multiple operators",
			command:     "lumo diagnose && echo test || rm file",
			wantErr:     true,
			errContains: "unsafe shell characters",
			description: "Should block multiple shell operators",
		},
		{
			name:        "encoded newline attempt",
			command:     "lumo diagnose\\nrm -rf /",
			wantErr:     false,
			description: "Should allow escaped newline literals (\\n as text, not actual newline)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProposedCommand(tt.command)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateProposedCommand() error = nil, wantErr = true for case: %s", tt.description)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("validateProposedCommand() error = %v, want error containing %q for case: %s", err, tt.errContains, tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("validateProposedCommand() unexpected error = %v for case: %s", err, tt.description)
				}
			}
		})
	}
}
