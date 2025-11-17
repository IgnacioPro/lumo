package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// TestRunDiagnostics_Localhost tests the complete localhost diagnostic flow
func TestRunDiagnostics_Localhost(t *testing.T) {
	// Save original stdout and restore after test
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	// Create command with localhost argument
	cmd := &cobra.Command{}
	cmd.Flags().Int("port", 22, "SSH port")
	cmd.Flags().String("identity", "", "Identity file")
	cmd.Flags().StringSlice("checks", []string{}, "Checks to run")
	cmd.Flags().String("format", "text", "Output format")
	cmd.Flags().Bool("no-color", true, "Disable color")
	cmd.Flags().Bool("analyze", false, "Enable AI analysis")
	cmd.Flags().StringSlice("focus", []string{}, "Focus areas")

	// Suppress log output during test
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	// Run diagnostics for localhost
	err := runDiagnostics(cmd, []string{"localhost"})

	// Close write end and read captured output
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify no error
	if err != nil {
		t.Errorf("runDiagnostics() error = %v, want nil", err)
	}

	// Verify output contains diagnostic results
	if !strings.Contains(output, "Diagnostic Report") && !strings.Contains(output, "DIAGNOSTIC REPORT") {
		t.Errorf("Output missing diagnostic report header:\n%s", output)
	}

	// Should contain at least some check results
	checksFound := strings.Contains(output, "cpu") ||
		strings.Contains(output, "memory") ||
		strings.Contains(output, "disk")

	if !checksFound {
		t.Errorf("Output missing expected check results:\n%s", output)
	}
}

// TestRunDiagnostics_LocalhostWithFormat tests different output formats
func TestRunDiagnostics_LocalhostWithFormat(t *testing.T) {
	tests := []struct {
		name             string
		format           string
		expectedInOutput string
	}{
		{
			name:             "json format",
			format:           "json",
			expectedInOutput: `"results"`,
		},
		{
			name:             "text format",
			format:           "text",
			expectedInOutput: "DIAGNOSTIC",
		},
		{
			name:             "toon format",
			format:           "toon",
			expectedInOutput: "results",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Suppress logs
			log.SetOutput(io.Discard)
			defer log.SetOutput(os.Stderr)

			// Create command
			cmd := &cobra.Command{}
			cmd.Flags().Int("port", 22, "SSH port")
			cmd.Flags().String("identity", "", "Identity file")
			cmd.Flags().StringSlice("checks", []string{}, "Checks to run")
			cmd.Flags().String("format", tt.format, "Output format")
			cmd.Flags().Bool("no-color", true, "Disable color")
			cmd.Flags().Bool("analyze", false, "Enable AI analysis")
			cmd.Flags().StringSlice("focus", []string{}, "Focus areas")

			// Run diagnostics
			err := runDiagnostics(cmd, []string{"localhost"})

			// Restore stdout and capture output
			_ = w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			if err != nil {
				t.Errorf("runDiagnostics() error = %v, want nil", err)
			}

			if !strings.Contains(output, tt.expectedInOutput) {
				t.Errorf("Output missing expected content %q:\n%s", tt.expectedInOutput, output)
			}
		})
	}
}

// TestRunDiagnostics_LocalhostWithSpecificChecks tests filtering to specific checks
func TestRunDiagnostics_LocalhostWithSpecificChecks(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Suppress logs
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	// Create command with specific checks
	cmd := &cobra.Command{}
	cmd.Flags().Int("port", 22, "SSH port")
	cmd.Flags().String("identity", "", "Identity file")
	cmd.Flags().StringSlice("checks", []string{"cpu", "memory"}, "Checks to run")
	cmd.Flags().String("format", "text", "Output format")
	cmd.Flags().Bool("no-color", true, "Disable color")
	cmd.Flags().Bool("analyze", false, "Enable AI analysis")
	cmd.Flags().StringSlice("focus", []string{}, "Focus areas")

	// Run diagnostics
	err := runDiagnostics(cmd, []string{"localhost"})

	// Restore stdout and capture output
	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("runDiagnostics() error = %v, want nil", err)
	}

	// Should contain CPU and memory checks
	if !strings.Contains(strings.ToLower(output), "cpu") {
		t.Error("Output missing CPU check results")
	}

	if !strings.Contains(strings.ToLower(output), "memory") {
		t.Error("Output missing memory check results")
	}
}

// TestRunDiagnostics_NoArgs tests default behavior (should default to localhost)
func TestRunDiagnostics_NoArgs(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Suppress logs
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	// Create command with no args
	cmd := &cobra.Command{}
	cmd.Flags().Int("port", 22, "SSH port")
	cmd.Flags().String("identity", "", "Identity file")
	cmd.Flags().StringSlice("checks", []string{}, "Checks to run")
	cmd.Flags().String("format", "text", "Output format")
	cmd.Flags().Bool("no-color", true, "Disable color")
	cmd.Flags().Bool("analyze", false, "Enable AI analysis")
	cmd.Flags().StringSlice("focus", []string{}, "Focus areas")

	// Run with empty args (should default to localhost)
	err := runDiagnostics(cmd, []string{})

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Errorf("runDiagnostics() error = %v, want nil (should default to localhost)", err)
	}
}

// TestRunDiagnostics_UserHostFormat tests user@host argument parsing
func TestRunDiagnostics_UserHostFormat(t *testing.T) {
	tests := []struct {
		name    string
		hostArg string
		wantErr bool
	}{
		{
			name:    "localhost",
			hostArg: "localhost",
			wantErr: false,
		},
		{
			name:    "127.0.0.1",
			hostArg: "127.0.0.1",
			wantErr: false,
		},
		{
			name:    "user@localhost",
			hostArg: "user@localhost",
			wantErr: false,
		},
		{
			name:    "IPv6 localhost",
			hostArg: "::1",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Suppress logs
			oldLogOutput := log.Out
			log.SetOutput(io.Discard)
			defer log.SetOutput(oldLogOutput)

			// Create command
			cmd := &cobra.Command{}
			cmd.Flags().Int("port", 22, "SSH port")
			cmd.Flags().String("identity", "", "Identity file")
			cmd.Flags().StringSlice("checks", []string{}, "Checks to run")
			cmd.Flags().String("format", "text", "Output format")
			cmd.Flags().Bool("no-color", true, "Disable color")
			cmd.Flags().Bool("analyze", false, "Enable AI analysis")
			cmd.Flags().StringSlice("focus", []string{}, "Focus areas")

			// Run diagnostics
			err := runDiagnostics(cmd, []string{tt.hostArg})

			// Restore stdout
			_ = w.Close()
			os.Stdout = oldStdout
			_, _ = io.Copy(io.Discard, r) // Drain pipe

			if (err != nil) != tt.wantErr {
				t.Errorf("runDiagnostics() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// mockDiagnosticExecutor is a test executor that returns predictable results
type mockDiagnosticExecutor struct{}

func (m *mockDiagnosticExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	return m.ExecuteWithContext(context.Background(), command)
}

func (m *mockDiagnosticExecutor) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	// Return mock data based on command
	if strings.Contains(command, "nproc") || strings.Contains(command, "sysctl -n hw.ncpu") {
		return "4", "", 0, nil
	}
	if strings.Contains(command, "uptime") {
		return "load average: 0.5, 0.3, 0.2", "", 0, nil
	}
	if strings.Contains(command, "free") || strings.Contains(command, "vm_stat") {
		return "MemTotal: 8192 MB\nMemAvailable: 4096 MB", "", 0, nil
	}
	if strings.Contains(command, "df") {
		return "/dev/sda1 100G 50G 50G 50% /", "", 0, nil
	}
	return "", "", 0, nil
}

// TestDiagnosticRunner_WithMockExecutor tests the runner with controlled executor
func TestDiagnosticRunner_WithMockExecutor(t *testing.T) {
	// This tests the integration of runner + checkers with predictable executor
	executor := &mockDiagnosticExecutor{}

	config := diagnostics.DefaultConfig()
	thresholds := diagnostics.DefaultThresholds()

	// Create logger
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	runner := diagnostics.NewRunner(config, thresholds, executor, logger)

	// Register checkers (just CPU for quick test)
	runner.RegisterChecker(&diagnostics.MockChecker{
		CheckName:     "test_check",
		CheckCategory: diagnostics.CategoryCPU,
		CheckResult: &diagnostics.CheckResult{
			Name:     "test_check",
			Category: diagnostics.CategoryCPU,
			Severity: diagnostics.SeverityOK,
			Message:  "Test passed",
			Data:     map[string]interface{}{"test": true},
		},
	})

	// Run diagnostics
	ctx := context.Background()
	report, err := runner.RunAll(ctx)

	if err != nil {
		t.Errorf("RunAll() error = %v, want nil", err)
	}

	if report == nil {
		t.Fatal("RunAll() returned nil report")
	}

	if len(report.Results) == 0 {
		t.Error("RunAll() returned empty results")
	}
}

// TestIsLocalhost_Integration tests localhost detection in real scenarios
func TestIsLocalhost_Integration(t *testing.T) {
	tests := []struct {
		hostname string
		want     bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"0.0.0.0", true},
		{"", true},
		{"example.com", false},
		{"192.168.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			got := isLocalhost(tt.hostname)
			if got != tt.want {
				t.Errorf("isLocalhost(%q) = %v, want %v", tt.hostname, got, tt.want)
			}
		})
	}
}
