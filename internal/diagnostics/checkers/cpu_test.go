package checkers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// mockExecutor implements diagnostics.CommandExecutor for testing
type mockExecutor struct {
	responses map[string]mockResponse
}

type mockResponse struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func (m *mockExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return m.ExecuteWithContext(ctx, command)
}

func (m *mockExecutor) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	// Find matching response based on command pattern
	for pattern, resp := range m.responses {
		if strings.Contains(command, pattern) {
			return resp.stdout, resp.stderr, resp.exitCode, resp.err
		}
	}
	return "", "", 1, nil
}

func TestNewCPUChecker(t *testing.T) {
	thresholds := diagnostics.CPUThresholds{
		UsageWarn:     80.0,
		UsageCritical: 95.0,
	}

	checker := NewCPUChecker(thresholds)
	if checker == nil {
		t.Fatal("NewCPUChecker returned nil")
	}
	if checker.thresholds.UsageWarn != 80.0 {
		t.Errorf("Expected UsageWarn 80.0, got %v", checker.thresholds.UsageWarn)
	}
}

func TestCPUChecker_Name(t *testing.T) {
	checker := NewCPUChecker(diagnostics.CPUThresholds{})
	if got := checker.Name(); got != "cpu_check" {
		t.Errorf("Name() = %v, want cpu_check", got)
	}
}

func TestCPUChecker_Category(t *testing.T) {
	checker := NewCPUChecker(diagnostics.CPUThresholds{})
	if got := checker.Category(); got != diagnostics.CategoryCPU {
		t.Errorf("Category() = %v, want %v", got, diagnostics.CategoryCPU)
	}
}

func TestCPUChecker_Description(t *testing.T) {
	checker := NewCPUChecker(diagnostics.CPUThresholds{})
	desc := checker.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if !strings.Contains(strings.ToLower(desc), "cpu") {
		t.Error("Description should mention CPU")
	}
}

func TestCPUChecker_RequiresRoot(t *testing.T) {
	checker := NewCPUChecker(diagnostics.CPUThresholds{})
	if checker.RequiresRoot() {
		t.Error("CPUChecker should not require root")
	}
}

func TestCPUChecker_Run(t *testing.T) {
	thresholds := diagnostics.CPUThresholds{
		UsageWarn:     80.0,
		UsageCritical: 95.0,
		LoadWarn:      1.5,
		LoadCritical:  2.0,
	}

	executor := &mockExecutor{
		responses: map[string]mockResponse{
			"nproc": {
				stdout:   "8\n",
				exitCode: 0,
			},
			"loadavg": {
				stdout:   "1.23 1.45 1.67\n",
				exitCode: 0,
			},
			"top": {
				stdout:   "Cpu(s): 45.2%us\n",
				exitCode: 0,
			},
		},
	}

	checker := NewCPUChecker(thresholds)
	result, err := checker.Run(context.Background(), executor)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Name != "cpu_check" {
		t.Errorf("Result.Name = %v, want cpu_check", result.Name)
	}

	if result.Category != diagnostics.CategoryCPU {
		t.Errorf("Result.Category = %v, want %v", result.Category, diagnostics.CategoryCPU)
	}

	if result.Status != diagnostics.StatusCompleted {
		t.Errorf("Result.Status = %v, want %v", result.Status, diagnostics.StatusCompleted)
	}

	// Check data fields
	if cpuCount, ok := result.GetDataFloat("cpu_count"); !ok || cpuCount != 8.0 {
		t.Errorf("Expected cpu_count=8, got %v (ok=%v)", cpuCount, ok)
	}

	if load1, ok := result.GetDataFloat("load_1min"); !ok || load1 != 1.23 {
		t.Errorf("Expected load_1min=1.23, got %v (ok=%v)", load1, ok)
	}

	if load5, ok := result.GetDataFloat("load_5min"); !ok || load5 != 1.45 {
		t.Errorf("Expected load_5min=1.45, got %v (ok=%v)", load5, ok)
	}

	if load15, ok := result.GetDataFloat("load_15min"); !ok || load15 != 1.67 {
		t.Errorf("Expected load_15min=1.67, got %v (ok=%v)", load15, ok)
	}

	// Check load per CPU calculation
	loadPerCPU, ok := result.GetDataFloat("load_per_cpu_1min")
	if !ok {
		t.Error("Expected load_per_cpu_1min to be present")
	}
	expectedLoadPerCPU := 1.23 / 8.0
	if loadPerCPU != expectedLoadPerCPU {
		t.Errorf("Expected load_per_cpu_1min=%.4f, got %.4f", expectedLoadPerCPU, loadPerCPU)
	}

	// Check message
	if result.Message == "" {
		t.Error("Expected non-empty message")
	}

	// Check metrics
	if len(result.Metrics) == 0 {
		t.Error("Expected metrics to be present")
	}
}

func TestCPUChecker_GetCPUCount(t *testing.T) {
	tests := []struct {
		name     string
		stdout   string
		exitCode int
		want     int
		wantErr  bool
	}{
		{
			name:     "nproc success",
			stdout:   "8\n",
			exitCode: 0,
			want:     8,
			wantErr:  false,
		},
		{
			name:     "single core",
			stdout:   "1\n",
			exitCode: 0,
			want:     1,
			wantErr:  false,
		},
		{
			name:     "many cores",
			stdout:   "64\n",
			exitCode: 0,
			want:     64,
			wantErr:  false,
		},
		{
			name:     "with whitespace",
			stdout:   "  12  \n",
			exitCode: 0,
			want:     12,
			wantErr:  false,
		},
		{
			name:     "macOS sysctl fallback",
			stdout:   "4\n",
			exitCode: 0,
			want:     4,
			wantErr:  false,
		},
		{
			name:     "very high core count",
			stdout:   "128\n",
			exitCode: 0,
			want:     128,
			wantErr:  false,
		},
		{
			name:     "invalid output - non-numeric",
			stdout:   "invalid\n",
			exitCode: 0,
			want:     1,
			wantErr:  true,
		},
		{
			name:     "empty output",
			stdout:   "\n",
			exitCode: 0,
			want:     1,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				responses: map[string]mockResponse{
					"nproc": {
						stdout:   tt.stdout,
						exitCode: tt.exitCode,
					},
				},
			}

			checker := NewCPUChecker(diagnostics.CPUThresholds{})
			got, err := checker.getCPUCount(context.Background(), executor)

			if (err != nil) != tt.wantErr {
				t.Errorf("getCPUCount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("getCPUCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCPUChecker_GetLoadAverage(t *testing.T) {
	tests := []struct {
		name      string
		stdout    string
		exitCode  int
		wantLoad1 float64
		wantLoad5 float64
		wantLoad15 float64
		wantErr   bool
	}{
		{
			name:       "Linux /proc/loadavg",
			stdout:     "1.23 1.45 1.67 2/345 12345\n",
			exitCode:   0,
			wantLoad1:  1.23,
			wantLoad5:  1.45,
			wantLoad15: 1.67,
			wantErr:    false,
		},
		{
			name:       "macOS uptime format",
			stdout:     "1.50 2.00 2.50\n",
			exitCode:   0,
			wantLoad1:  1.50,
			wantLoad5:  2.00,
			wantLoad15: 2.50,
			wantErr:    false,
		},
		{
			name:       "high load",
			stdout:     "8.99 7.50 6.25\n",
			exitCode:   0,
			wantLoad1:  8.99,
			wantLoad5:  7.50,
			wantLoad15: 6.25,
			wantErr:    false,
		},
		{
			name:       "zero load",
			stdout:     "0.00 0.00 0.00\n",
			exitCode:   0,
			wantLoad1:  0.00,
			wantLoad5:  0.00,
			wantLoad15: 0.00,
			wantErr:    false,
		},
		{
			name:     "invalid format - too few fields",
			stdout:   "1.23 1.45\n",
			exitCode: 0,
			wantErr:  true,
		},
		{
			name:     "invalid format - non-numeric load1",
			stdout:   "abc 1.45 1.67\n",
			exitCode: 0,
			wantErr:  true,
		},
		{
			name:     "invalid format - non-numeric load5",
			stdout:   "1.23 xyz 1.67\n",
			exitCode: 0,
			wantErr:  true,
		},
		{
			name:     "invalid format - non-numeric load15",
			stdout:   "1.23 1.45 def\n",
			exitCode: 0,
			wantErr:  true,
		},
		{
			name:     "empty output",
			stdout:   "",
			exitCode: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				responses: map[string]mockResponse{
					"loadavg": {
						stdout:   tt.stdout,
						exitCode: tt.exitCode,
					},
				},
			}

			checker := NewCPUChecker(diagnostics.CPUThresholds{})
			load1, load5, load15, err := checker.getLoadAverage(context.Background(), executor)

			if (err != nil) != tt.wantErr {
				t.Errorf("getLoadAverage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if load1 != tt.wantLoad1 {
					t.Errorf("load1 = %v, want %v", load1, tt.wantLoad1)
				}
				if load5 != tt.wantLoad5 {
					t.Errorf("load5 = %v, want %v", load5, tt.wantLoad5)
				}
				if load15 != tt.wantLoad15 {
					t.Errorf("load15 = %v, want %v", load15, tt.wantLoad15)
				}
			}
		})
	}
}

func TestCPUChecker_GetCPUUsage(t *testing.T) {
	tests := []struct {
		name     string
		stdout   string
		exitCode int
		want     float64
		wantErr  bool
	}{
		{
			name:     "Linux top format",
			stdout:   "45.2\n",
			exitCode: 0,
			want:     45.2,
			wantErr:  false,
		},
		{
			name:     "macOS top format",
			stdout:   "23.5\n",
			exitCode: 0,
			want:     23.5,
			wantErr:  false,
		},
		{
			name:     "high usage",
			stdout:   "98.7\n",
			exitCode: 0,
			want:     98.7,
			wantErr:  false,
		},
		{
			name:     "low usage",
			stdout:   "1.2\n",
			exitCode: 0,
			want:     1.2,
			wantErr:  false,
		},
		{
			name:     "zero usage",
			stdout:   "0.0\n",
			exitCode: 0,
			want:     0.0,
			wantErr:  false,
		},
		{
			name:     "with whitespace",
			stdout:   "  56.3  \n",
			exitCode: 0,
			want:     56.3,
			wantErr:  false,
		},
		{
			name:     "command failure - returns 0",
			stdout:   "",
			exitCode: 1,
			want:     0.0,
			wantErr:  false,
		},
		{
			name:     "invalid number - returns 0",
			stdout:   "invalid\n",
			exitCode: 0,
			want:     0.0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				responses: map[string]mockResponse{
					"top": {
						stdout:   tt.stdout,
						exitCode: tt.exitCode,
					},
				},
			}

			checker := NewCPUChecker(diagnostics.CPUThresholds{})
			got, err := checker.getCPUUsage(context.Background(), executor)

			if (err != nil) != tt.wantErr {
				t.Errorf("getCPUUsage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("getCPUUsage() = %v, want %v", got, tt.want)
			}
		})
	}
}
