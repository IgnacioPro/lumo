package formatters

import (
	"strings"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestNewToonFormatter(t *testing.T) {
	tests := []struct {
		name         string
		opts         []ToonFormatterOption
		wantIndent   int
		wantLengthM  bool
	}{
		{
			name:        "default options",
			opts:        nil,
			wantIndent:  2,
			wantLengthM: false,
		},
		{
			name:        "custom indent",
			opts:        []ToonFormatterOption{WithIndent(4)},
			wantIndent:  4,
			wantLengthM: false,
		},
		{
			name:        "with length marker",
			opts:        []ToonFormatterOption{WithLengthMarker()},
			wantIndent:  2,
			wantLengthM: true,
		},
		{
			name:        "all options",
			opts:        []ToonFormatterOption{WithIndent(4), WithLengthMarker()},
			wantIndent:  4,
			wantLengthM: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewToonFormatter(tt.opts...)

			if formatter.indent != tt.wantIndent {
				t.Errorf("NewToonFormatter() indent = %d, want %d", formatter.indent, tt.wantIndent)
			}

			if formatter.lengthMarker != tt.wantLengthM {
				t.Errorf("NewToonFormatter() lengthMarker = %v, want %v", formatter.lengthMarker, tt.wantLengthM)
			}
		})
	}
}

func TestToonFormatter_FormatResult(t *testing.T) {
	formatter := NewToonFormatter()

	tests := []struct {
		name     string
		result   *diagnostics.CheckResult
		wantPart []string // Strings that should appear in output
	}{
		{
			name: "simple result",
			result: &diagnostics.CheckResult{
				Name:      "cpu_check",
				Category:  diagnostics.CategoryCPU,
				Status:    diagnostics.StatusCompleted,
				Severity:  diagnostics.SeverityOK,
				Message:   "CPU usage is normal",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Duration:  100 * time.Millisecond,
			},
			wantPart: []string{
				"name: cpu_check",
				"category: cpu",
				"status: completed",
				"severity: ok",
				"message: CPU usage is normal",
			},
		},
		{
			name: "result with metrics",
			result: &diagnostics.CheckResult{
				Name:      "memory_check",
				Category:  diagnostics.CategoryMemory,
				Status:    diagnostics.StatusCompleted,
				Severity:  diagnostics.SeverityWarning,
				Message:   "Memory usage is high",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Duration:  150 * time.Millisecond,
				Metrics: []diagnostics.Metric{
					{
						Name:          "usage",
						Value:         85.5,
						Unit:          "percent",
						Threshold:     80.0,
						ThresholdType: diagnostics.ThresholdTypeMax,
					},
					{
						Name:  "available",
						Value: 2048,
						Unit:  "MB",
					},
				},
			},
			wantPart: []string{
				"name: memory_check",
				"metrics",
				"name: usage",
				"value: 85.5",
				"unit: percent",
				"threshold: 80",
			},
		},
		{
			name: "result with error",
			result: &diagnostics.CheckResult{
				Name:      "disk_check",
				Category:  diagnostics.CategoryDisk,
				Status:    diagnostics.StatusFailed,
				Severity:  diagnostics.SeverityError,
				Message:   "Failed to check disk",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Duration:  50 * time.Millisecond,
				Error:     "command execution failed",
			},
			wantPart: []string{
				"name: disk_check",
				"status: failed",
				"severity: error",
				"error: command execution failed",
			},
		},
		{
			name: "result with structured data",
			result: &diagnostics.CheckResult{
				Name:      "process_check",
				Category:  diagnostics.CategoryProcess,
				Status:    diagnostics.StatusCompleted,
				Severity:  diagnostics.SeverityInfo,
				Message:   "Process count normal",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Duration:  200 * time.Millisecond,
				Data: map[string]interface{}{
					"total_processes": 150,
					"zombie_count":    0,
				},
			},
			wantPart: []string{
				"name: process_check",
				"category: process",
				"data:",
				"total_processes: 150",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatter.FormatResult(tt.result)

			// Verify output contains expected parts
			for _, part := range tt.wantPart {
				if !strings.Contains(output, part) {
					t.Errorf("FormatResult() output missing expected part %q\nGot:\n%s", part, output)
				}
			}

			// Verify no error message in output
			if strings.Contains(output, "Error encoding") {
				t.Errorf("FormatResult() returned error: %s", output)
			}
		})
	}
}

func TestToonFormatter_FormatReport(t *testing.T) {
	formatter := NewToonFormatter()

	tests := []struct {
		name     string
		report   *diagnostics.Report
		wantPart []string
	}{
		{
			name: "empty report",
			report: &diagnostics.Report{
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Duration:  500 * time.Millisecond,
				Results:   []*diagnostics.CheckResult{},
				Summary: diagnostics.ReportSummary{
					TotalChecks:   0,
					OKCount:       0,
					InfoCount:     0,
					WarningCount:  0,
					CriticalCount: 0,
					ErrorCount:    0,
				},
			},
			wantPart: []string{
				"timestamp:",
				"duration: 500",
				"summary:",
				"total_checks: 0",
			},
		},
		{
			name: "report with multiple results",
			report: &diagnostics.Report{
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Duration:  1000 * time.Millisecond,
				Results: []*diagnostics.CheckResult{
					{
						Name:      "cpu_check",
						Category:  diagnostics.CategoryCPU,
						Status:    diagnostics.StatusCompleted,
						Severity:  diagnostics.SeverityOK,
						Message:   "CPU OK",
						Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
						Duration:  100 * time.Millisecond,
					},
					{
						Name:      "memory_check",
						Category:  diagnostics.CategoryMemory,
						Status:    diagnostics.StatusCompleted,
						Severity:  diagnostics.SeverityWarning,
						Message:   "Memory high",
						Timestamp: time.Date(2024, 1, 1, 12, 0, 1, 0, time.UTC),
						Duration:  150 * time.Millisecond,
						Metrics: []diagnostics.Metric{
							{
								Name:          "usage",
								Value:         85.0,
								Unit:          "percent",
								Threshold:     80.0,
								ThresholdType: diagnostics.ThresholdTypeMax,
							},
						},
					},
				},
				Summary: diagnostics.ReportSummary{
					TotalChecks:   2,
					OKCount:       1,
					InfoCount:     0,
					WarningCount:  1,
					CriticalCount: 0,
					ErrorCount:    0,
				},
			},
			wantPart: []string{
				"timestamp:",
				"duration: 1000",
				"summary:",
				"total_checks: 2",
				"ok_count: 1",
				"warning_count: 1",
				"results",
				"name: cpu_check",
				"name: memory_check",
			},
		},
		{
			name: "report with metadata",
			report: &diagnostics.Report{
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Duration:  500 * time.Millisecond,
				Results:   []*diagnostics.CheckResult{},
				Summary: diagnostics.ReportSummary{
					TotalChecks: 0,
				},
				Metadata: map[string]interface{}{
					"hostname": "test-server",
					"version":  "1.0.0",
				},
			},
			wantPart: []string{
				"metadata:",
				"hostname: test-server",
				"version: 1.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatter.FormatReport(tt.report)

			// Verify output contains expected parts
			for _, part := range tt.wantPart {
				if !strings.Contains(output, part) {
					t.Errorf("FormatReport() output missing expected part %q\nGot:\n%s", part, output)
				}
			}

			// Verify no error message in output
			if strings.Contains(output, "Error encoding") {
				t.Errorf("FormatReport() returned error: %s", output)
			}
		})
	}
}

func TestToonFormatter_TokenEfficiency(t *testing.T) {
	// Create a realistic diagnostic report
	report := &diagnostics.Report{
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Duration:  2000 * time.Millisecond,
		Results: []*diagnostics.CheckResult{
			{
				Name:      "cpu_check",
				Category:  diagnostics.CategoryCPU,
				Status:    diagnostics.StatusCompleted,
				Severity:  diagnostics.SeverityOK,
				Message:   "CPU usage is normal",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Duration:  100 * time.Millisecond,
				Metrics: []diagnostics.Metric{
					{Name: "load_1min", Value: 1.5, Unit: "load"},
					{Name: "load_5min", Value: 1.2, Unit: "load"},
					{Name: "load_15min", Value: 0.8, Unit: "load"},
					{Name: "usage", Value: 45.5, Unit: "percent", Threshold: 80.0, ThresholdType: diagnostics.ThresholdTypeMax},
				},
			},
			{
				Name:      "memory_check",
				Category:  diagnostics.CategoryMemory,
				Status:    diagnostics.StatusCompleted,
				Severity:  diagnostics.SeverityWarning,
				Message:   "Memory usage is high",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 1, 0, time.UTC),
				Duration:  150 * time.Millisecond,
				Metrics: []diagnostics.Metric{
					{Name: "usage", Value: 85.5, Unit: "percent", Threshold: 80.0, ThresholdType: diagnostics.ThresholdTypeMax},
					{Name: "available", Value: 2048, Unit: "MB"},
					{Name: "swap_usage", Value: 10.0, Unit: "percent"},
				},
			},
		},
		Summary: diagnostics.ReportSummary{
			TotalChecks:   2,
			OKCount:       1,
			WarningCount:  1,
			CriticalCount: 0,
			ErrorCount:    0,
		},
	}

	formatter := NewToonFormatter()
	toonOutput := formatter.FormatReport(report)

	// Compare with JSON output
	jsonOutput, err := report.ToJSON()
	if err != nil {
		t.Fatalf("Failed to create JSON output: %v", err)
	}

	toonSize := len(toonOutput)
	jsonSize := len(jsonOutput)

	// TOON should be smaller than JSON
	if toonSize >= jsonSize {
		t.Logf("TOON size: %d bytes, JSON size: %d bytes", toonSize, jsonSize)
		t.Logf("TOON output:\n%s", toonOutput)
		t.Logf("JSON output:\n%s", jsonOutput)
		t.Errorf("TOON output (%d bytes) should be smaller than JSON (%d bytes)", toonSize, jsonSize)
	}

	// Calculate reduction percentage
	reduction := float64(jsonSize-toonSize) / float64(jsonSize) * 100
	t.Logf("Token efficiency: TOON is %.1f%% smaller than JSON (%d vs %d bytes)", reduction, toonSize, jsonSize)

	// TOON should achieve at least 10% reduction (conservative estimate)
	if reduction < 10 {
		t.Errorf("Expected at least 10%% size reduction, got %.1f%%", reduction)
	}
}

func TestToonFormatter_WithLengthMarker(t *testing.T) {
	formatter := NewToonFormatter(WithLengthMarker())

	report := &diagnostics.Report{
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Duration:  500 * time.Millisecond,
		Results: []*diagnostics.CheckResult{
			{
				Name:      "test_check",
				Category:  diagnostics.CategoryCPU,
				Status:    diagnostics.StatusCompleted,
				Severity:  diagnostics.SeverityOK,
				Message:   "Test",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Duration:  100 * time.Millisecond,
			},
		},
		Summary: diagnostics.ReportSummary{
			TotalChecks: 1,
		},
	}

	output := formatter.FormatReport(report)

	// With length marker, arrays should have '#' prefix
	// Note: The actual format depends on gotoon implementation
	if !strings.Contains(output, "results") {
		t.Errorf("Expected array 'results' in output, got:\n%s", output)
	}
}

func TestToonFormatter_CustomIndent(t *testing.T) {
	tests := []struct {
		name   string
		indent int
	}{
		{"indent 2", 2},
		{"indent 4", 4},
		{"indent 8", 8},
	}

	result := &diagnostics.CheckResult{
		Name:      "test",
		Category:  diagnostics.CategoryCPU,
		Status:    diagnostics.StatusCompleted,
		Severity:  diagnostics.SeverityOK,
		Message:   "Test",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Duration:  100 * time.Millisecond,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewToonFormatter(WithIndent(tt.indent))
			output := formatter.FormatResult(result)

			// Output should be valid TOON (no error)
			if strings.Contains(output, "Error encoding") {
				t.Errorf("FormatResult() with indent %d failed: %s", tt.indent, output)
			}
		})
	}
}

func TestNewToonFormatter_Defaults(t *testing.T) {
	formatter := NewToonFormatter()

	if formatter == nil {
		t.Fatal("NewToonFormatter() returned nil")
	}

	if formatter.indent != 2 {
		t.Errorf("Default indent = %d, want 2", formatter.indent)
	}

	if formatter.lengthMarker != false {
		t.Errorf("Default lengthMarker = %v, want false", formatter.lengthMarker)
	}
}

func TestToonFormatter_ResultToMap(t *testing.T) {
	formatter := NewToonFormatter()

	tests := []struct {
		name   string
		result *diagnostics.CheckResult
		verify func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "basic fields",
			result: &diagnostics.CheckResult{
				Name:      "test_check",
				Category:  diagnostics.CategoryCPU,
				Status:    diagnostics.StatusCompleted,
				Severity:  diagnostics.SeverityOK,
				Message:   "Test message",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Duration:  100 * time.Millisecond,
			},
			verify: func(t *testing.T, data map[string]interface{}) {
				if data["name"] != "test_check" {
					t.Errorf("Expected name=test_check, got %v", data["name"])
				}
				if data["category"] != "cpu" {
					t.Errorf("Expected category=cpu, got %v", data["category"])
				}
				if data["status"] != "completed" {
					t.Errorf("Expected status=completed, got %v", data["status"])
				}
				if data["duration"] != int64(100) {
					t.Errorf("Expected duration=100, got %v", data["duration"])
				}
			},
		},
		{
			name: "with metrics",
			result: &diagnostics.CheckResult{
				Name:      "test",
				Category:  diagnostics.CategoryMemory,
				Status:    diagnostics.StatusCompleted,
				Severity:  diagnostics.SeverityOK,
				Message:   "Test",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Duration:  100 * time.Millisecond,
				Metrics: []diagnostics.Metric{
					{Name: "usage", Value: 75.5, Unit: "percent"},
				},
			},
			verify: func(t *testing.T, data map[string]interface{}) {
				metrics, ok := data["metrics"].([]map[string]interface{})
				if !ok {
					t.Errorf("Expected metrics to be []map[string]interface{}, got %T", data["metrics"])
					return
				}
				if len(metrics) != 1 {
					t.Errorf("Expected 1 metric, got %d", len(metrics))
				}
			},
		},
		{
			name: "with error",
			result: &diagnostics.CheckResult{
				Name:      "test",
				Category:  diagnostics.CategoryDisk,
				Status:    diagnostics.StatusFailed,
				Severity:  diagnostics.SeverityError,
				Message:   "Test",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Duration:  100 * time.Millisecond,
				Error:     "test error",
			},
			verify: func(t *testing.T, data map[string]interface{}) {
				if data["error"] != "test error" {
					t.Errorf("Expected error='test error', got %v", data["error"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := formatter.resultToMap(tt.result)
			tt.verify(t, data)
		})
	}
}
