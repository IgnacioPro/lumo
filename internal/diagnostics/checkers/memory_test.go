package checkers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// mockCommandExecutor implements CommandExecutor interface for testing
type mockCommandExecutor struct {
	commands map[string]mockCommandResult
}

type mockCommandResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func newMockExecutor() *mockCommandExecutor {
	return &mockCommandExecutor{
		commands: make(map[string]mockCommandResult),
	}
}

func (m *mockCommandExecutor) setCommand(cmd string, stdout, stderr string, exitCode int, err error) {
	m.commands[cmd] = mockCommandResult{
		stdout:   stdout,
		stderr:   stderr,
		exitCode: exitCode,
		err:      err,
	}
}

func (m *mockCommandExecutor) Execute(command string, timeout time.Duration) (string, string, int, error) {
	result, exists := m.commands[command]
	if !exists {
		return "", "command not found", 1, nil
	}
	return result.stdout, result.stderr, result.exitCode, result.err
}

func (m *mockCommandExecutor) ExecuteWithContext(ctx context.Context, command string) (string, string, int, error) {
	result, exists := m.commands[command]
	if !exists {
		return "", "command not found", 1, nil
	}
	return result.stdout, result.stderr, result.exitCode, result.err
}

func TestMemoryChecker_parseProcessLine(t *testing.T) {
	m := &MemoryChecker{}

	tests := []struct {
		name    string
		line    string
		wantOK  bool
		wantPID int
		wantMem float64
		wantCmd string
	}{
		{
			name:    "valid process line",
			line:    "user     12345  0.1  2.5  123456  78901 ?        S    10:00   0:02 /usr/bin/someapp --flag value",
			wantOK:  true,
			wantPID: 12345,
			wantMem: 2.5,
			wantCmd: "/usr/bin/someapp --flag value",
		},
		{
			name:   "header line",
			line:   "USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND",
			wantOK: false,
		},
		{
			name:   "incomplete line",
			line:   "user 123 0.1",
			wantOK: false,
		},
		{
			name:   "invalid PID",
			line:   "user     abc  0.1  2.5  123456  78901 ?        S    10:00   0:02 /usr/bin/app",
			wantOK: false,
		},
		{
			name:   "negative PID",
			line:   "user     -123  0.1  2.5  123456  78901 ?        S    10:00   0:02 /usr/bin/app",
			wantOK: false,
		},
		{
			name:   "invalid memory percent",
			line:   "user     123  0.1  abc  123456  78901 ?        S    10:00   0:02 /usr/bin/app",
			wantOK: false,
		},
		{
			name:   "memory percent over 100",
			line:   "user     123  0.1  150.0  123456  78901 ?        S    10:00   0:02 /usr/bin/app",
			wantOK: false,
		},
		{
			name:   "invalid RSS",
			line:   "user     123  0.1  2.5  123456  abc ?        S    10:00   0:02 /usr/bin/app",
			wantOK: false,
		},
		{
			name:    "long command truncation",
			line:    "user     123  0.1  2.5  123456  78901 ?        S    10:00   0:02 " + strings.Repeat("a", 100),
			wantOK:  true,
			wantPID: 123,
			wantMem: 2.5,
			wantCmd: strings.Repeat("a", 57) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer, ok := m.parseProcessLine(tt.line)

			if ok != tt.wantOK {
				t.Errorf("parseProcessLine() ok = %v, want %v", ok, tt.wantOK)
				return
			}

			if !tt.wantOK {
				return // Skip further checks for invalid cases
			}

			if consumer.PID != tt.wantPID {
				t.Errorf("parseProcessLine() PID = %v, want %v", consumer.PID, tt.wantPID)
			}

			if consumer.Percent != tt.wantMem {
				t.Errorf("parseProcessLine() Percent = %v, want %v", consumer.Percent, tt.wantMem)
			}

			if consumer.Command != tt.wantCmd {
				t.Errorf("parseProcessLine() Command = %v, want %v", consumer.Command, tt.wantCmd)
			}
		})
	}
}

func TestMemoryChecker_isHeaderLine(t *testing.T) {
	m := &MemoryChecker{}

	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "ps header line",
			line: "USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND",
			want: true,
		},
		{
			name: "process line",
			line: "user     12345  0.1  2.5  123456  78901 ?        S    10:00   0:02 /usr/bin/app",
			want: false,
		},
		{
			name: "empty line",
			line: "",
			want: false,
		},
		{
			name: "partial header",
			line: "USER PID COMMAND",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.isHeaderLine(tt.line); got != tt.want {
				t.Errorf("isHeaderLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryChecker_convertRSSToBytes(t *testing.T) {
	m := &MemoryChecker{}

	tests := []struct {
		name     string
		rssValue uint64
		want     uint64
	}{
		{
			name:     "small value in KB",
			rssValue: 1024,
			want:     1024 * 1024, // Convert from KB to bytes
		},
		{
			name:     "large value already in bytes",
			rssValue: 100 * 1024 * 1024 * 1024, // 100GB
			want:     100 * 1024 * 1024 * 1024, // Keep as bytes
		},
		{
			name:     "threshold boundary",
			rssValue: rssKBThreshold,
			want:     rssKBThreshold * 1024, // Convert from KB (function uses > not >=)
		},
		{
			name:     "just under threshold",
			rssValue: rssKBThreshold - 1,
			want:     (rssKBThreshold - 1) * 1024, // Convert from KB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.convertRSSToBytes(tt.rssValue); got != tt.want {
				t.Errorf("convertRSSToBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryChecker_sanitizeCommand(t *testing.T) {
	m := &MemoryChecker{}

	tests := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "normal command",
			command: "/usr/bin/app --flag value",
			want:    "/usr/bin/app --flag value",
		},
		{
			name:    "command with null bytes",
			command: "/usr/bin/app\x00--flag",
			want:    "/usr/bin/app--flag",
		},
		{
			name:    "command with newlines",
			command: "/usr/bin/app\n--flag\rvalue",
			want:    "/usr/bin/app --flag value",
		},
		{
			name:    "long command truncation",
			command: strings.Repeat("a", 100),
			want:    strings.Repeat("a", 57) + "...",
		},
		{
			name:    "exact length command",
			command: strings.Repeat("a", maxCommandLength),
			want:    strings.Repeat("a", maxCommandLength),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.sanitizeCommand(tt.command); got != tt.want {
				t.Errorf("sanitizeCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryChecker_parseProcessList(t *testing.T) {
	m := &MemoryChecker{}

	psOutput := `USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root         1  0.0  0.1 123456  1024 ?        Ss   00:00   0:01 /sbin/init
user      1234  1.2  5.5 456789  4096 ?        S    00:01   0:05 /usr/bin/bigapp
user      5678  0.5  2.1 234567  2048 ?        S    00:02   0:03 /usr/bin/smallapp
`

	consumers := m.parseProcessList(psOutput)

	if len(consumers) != 3 {
		t.Errorf("parseProcessList() returned %d consumers, want 3", len(consumers))
		return
	}

	// Check first consumer (init)
	if consumers[0].PID != 1 {
		t.Errorf("First consumer PID = %d, want 1", consumers[0].PID)
	}
	if consumers[0].Percent != 0.1 {
		t.Errorf("First consumer Percent = %f, want 0.1", consumers[0].Percent)
	}
	if consumers[0].Command != "/sbin/init" {
		t.Errorf("First consumer Command = %s, want /sbin/init", consumers[0].Command)
	}

	// Check memory calculations
	expectedBytes := uint64(1024 * 1024) // 1024 KB -> bytes
	if consumers[0].MemoryBytes != expectedBytes {
		t.Errorf("First consumer MemoryBytes = %d, want %d", consumers[0].MemoryBytes, expectedBytes)
	}

	expectedMB := float64(expectedBytes) / (1024 * 1024)
	if consumers[0].MemoryMB != expectedMB {
		t.Errorf("First consumer MemoryMB = %f, want %f", consumers[0].MemoryMB, expectedMB)
	}
}

func TestMemoryChecker_executeProcessList(t *testing.T) {
	m := &MemoryChecker{}

	tests := []struct {
		name           string
		linuxOutput    string
		linuxExitCode  int
		macosOutput    string
		macosExitCode  int
		expectError    bool
		expectedOutput string
	}{
		{
			name:           "Linux ps succeeds",
			linuxOutput:    "USER PID %MEM\nuser 123 1.0\n",
			linuxExitCode:  0,
			expectError:    false,
			expectedOutput: "USER PID %MEM\nuser 123 1.0\n",
		},
		{
			name:           "Linux ps fails, macOS succeeds",
			linuxOutput:    "",
			linuxExitCode:  1,
			macosOutput:    "USER PID %MEM\nuser 456 2.0\n",
			macosExitCode:  0,
			expectError:    false,
			expectedOutput: "USER PID %MEM\nuser 456 2.0\n",
		},
		{
			name:          "Both commands fail",
			linuxOutput:   "",
			linuxExitCode: 1,
			macosOutput:   "",
			macosExitCode: 1,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := newMockExecutor()
			executor.setCommand("ps aux --sort=-%mem", tt.linuxOutput, "", tt.linuxExitCode, nil)
			executor.setCommand("ps aux -m", tt.macosOutput, "", tt.macosExitCode, nil)

			ctx := context.Background()
			output, err := m.executeProcessList(ctx, executor)

			if tt.expectError {
				if err == nil {
					t.Errorf("executeProcessList() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("executeProcessList() unexpected error: %v", err)
					return
				}
				if output != tt.expectedOutput {
					t.Errorf("executeProcessList() output = %q, want %q", output, tt.expectedOutput)
				}
			}
		})
	}
}

func TestMemoryChecker_getTopMemoryConsumers(t *testing.T) {
	thresholds := diagnostics.MemoryThresholds{
		UsageWarn: 80.0,
		SwapWarn:  50.0,
	}
	m := NewMemoryChecker(thresholds)

	psOutput := `USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
user      1234  1.2  5.5 456789  4096 ?        S    00:01   0:05 /usr/bin/bigapp
user      5678  0.5  2.1 234567  2048 ?        S    00:02   0:03 /usr/bin/smallapp
`

	executor := newMockExecutor()
	executor.setCommand("ps aux --sort=-%mem", psOutput, "", 0, nil)

	ctx := context.Background()
	consumers, err := m.getTopMemoryConsumers(ctx, executor, 8*1024*1024*1024) // 8GB total

	if err != nil {
		t.Errorf("getTopMemoryConsumers() unexpected error: %v", err)
		return
	}

	if len(consumers) != 2 {
		t.Errorf("getTopMemoryConsumers() returned %d consumers, want 2", len(consumers))
		return
	}

	// Check first consumer
	if consumers[0].PID != 1234 {
		t.Errorf("First consumer PID = %d, want 1234", consumers[0].PID)
	}
	if consumers[0].Command != "/usr/bin/bigapp" {
		t.Errorf("First consumer Command = %s, want /usr/bin/bigapp", consumers[0].Command)
	}

	// Check memory values are reasonable
	if consumers[0].MemoryBytes == 0 {
		t.Errorf("First consumer MemoryBytes should not be zero")
	}
	if consumers[0].MemoryMB <= 0 {
		t.Errorf("First consumer MemoryMB should be positive, got %f", consumers[0].MemoryMB)
	}
}

func TestMemoryChecker_Run(t *testing.T) {
	thresholds := diagnostics.MemoryThresholds{
		UsageWarn: 80.0,
		SwapWarn:  50.0,
	}
	m := NewMemoryChecker(thresholds)

	// Mock the memory info commands - simple Linux /proc/meminfo
	meminfoOutput := `MemTotal:        8000000 kB
MemFree:         2000000 kB
MemAvailable:    3000000 kB
SwapTotal:       2000000 kB
SwapFree:        1500000 kB
`

	psOutput := `USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
user      1234  1.2  5.5 456789  4096 ?        S    00:01   0:05 /usr/bin/app
`

	executor := newMockExecutor()
	executor.setCommand("cat /proc/meminfo 2>/dev/null || free -b 2>/dev/null", meminfoOutput, "", 0, nil)
	executor.setCommand("ps aux --sort=-%mem", psOutput, "", 0, nil)

	ctx := context.Background()
	result, err := m.Run(ctx, executor)

	if err != nil {
		t.Errorf("Run() unexpected error: %v", err)
		return
	}

	if result == nil {
		t.Errorf("Run() returned nil result")
		return
	}

	// Check basic result structure
	if result.Name != "memory_check" {
		t.Errorf("Result Name = %s, want memory_check", result.Name)
	}
	if result.Category != diagnostics.CategoryMemory {
		t.Errorf("Result Category = %v, want %v", result.Category, diagnostics.CategoryMemory)
	}
	if result.Status != diagnostics.StatusCompleted {
		t.Errorf("Result Status = %v, want %v", result.Status, diagnostics.StatusCompleted)
	}

	// Check that data fields exist
	if _, exists := result.Data["total_bytes"]; !exists {
		t.Errorf("Result missing total_bytes data field")
	}
	if _, exists := result.Data["used_percent"]; !exists {
		t.Errorf("Result missing used_percent data field")
	}
	if _, exists := result.Data["top_consumers"]; !exists {
		t.Errorf("Result missing top_consumers data field")
	}

	// Check metrics
	if len(result.Metrics) == 0 {
		t.Errorf("Result should have at least one metric")
	}

	// Check message is not empty
	if result.Message == "" {
		t.Errorf("Result Message should not be empty")
	}

	// Check duration is reasonable
	if result.Duration == 0 {
		t.Errorf("Result Duration should not be zero")
	}
}

// Test the edge case where max consumers limit is respected
func TestMemoryChecker_parseProcessList_MaxConsumers(t *testing.T) {
	m := &MemoryChecker{}

	// Create output with more than maxConsumers (10) processes
	var lines []string
	lines = append(lines, "USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND")

	// Add 15 process lines
	for i := 1; i <= 15; i++ {
		lines = append(lines,
			"user      1234  1.2  5.5 456789  4096 ?        S    00:01   0:05 /usr/bin/app"+
				strings.Repeat("x", i)) // Make each command unique
	}

	psOutput := strings.Join(lines, "\n")
	consumers := m.parseProcessList(psOutput)

	// Should only return maxConsumers (10) processes
	if len(consumers) != maxConsumers {
		t.Errorf("parseProcessList() returned %d consumers, want %d (maxConsumers)",
			len(consumers), maxConsumers)
	}
}

func TestMemoryChecker_Name(t *testing.T) {
	checker := &MemoryChecker{}
	want := "memory_check"
	got := checker.Name()

	if got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestMemoryChecker_Category(t *testing.T) {
	checker := &MemoryChecker{}
	want := diagnostics.CategoryMemory
	got := checker.Category()

	if got != want {
		t.Errorf("Category() = %v, want %v", got, want)
	}
}

func TestMemoryChecker_Description(t *testing.T) {
	checker := &MemoryChecker{}
	got := checker.Description()

	if got == "" {
		t.Error("Description() returned empty string")
	}

	if !strings.Contains(got, "memory") {
		t.Errorf("Description() = %q, expected to contain 'memory'", got)
	}
}

func TestMemoryChecker_RequiresRoot(t *testing.T) {
	checker := &MemoryChecker{}
	got := checker.RequiresRoot()

	// Memory checks should not require root
	if got {
		t.Error("RequiresRoot() = true, want false")
	}
}
