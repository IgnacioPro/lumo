package formatters

import (
	"strings"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestNewTextFormatter(t *testing.T) {
	formatter := NewTextFormatter(true, true)
	if formatter == nil {
		t.Fatal("NewTextFormatter returned nil")
	}
	if !formatter.colorEnabled {
		t.Error("Expected colorEnabled to be true")
	}
	if !formatter.verbose {
		t.Error("Expected verbose to be true")
	}
}

func TestFormatReport(t *testing.T) {
	report := &diagnostics.Report{
		Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Duration:  100 * time.Millisecond,
		Results: []*diagnostics.CheckResult{
			{
				Name:      "cpu_check",
				Category:  diagnostics.CategoryCPU,
				Severity:  diagnostics.SeverityOK,
				Status:    diagnostics.StatusCompleted,
				Message:   "CPU usage is normal",
				Timestamp: time.Now(),
				Duration:  50 * time.Millisecond,
			},
		},
		Summary: diagnostics.ReportSummary{
			TotalChecks: 1,
			OKCount:     1,
		},
	}

	formatter := NewTextFormatter(false, false) // No color, not verbose
	output := formatter.FormatReport(report)

	// Verify header is present
	if !strings.Contains(output, "LUMO DIAGNOSTIC REPORT") {
		t.Error("Expected report header")
	}

	// Verify timestamp
	if !strings.Contains(output, "2025-01-01T12:00:00Z") {
		t.Error("Expected timestamp in output")
	}

	// Verify duration
	if !strings.Contains(output, "100ms") {
		t.Error("Expected duration in output")
	}

	// Verify category header
	if !strings.Contains(output, "CPU CHECKS") {
		t.Error("Expected CPU category header")
	}

	// Verify check name
	if !strings.Contains(output, "cpu_check") {
		t.Error("Expected check name in output")
	}

	// Verify message
	if !strings.Contains(output, "CPU usage is normal") {
		t.Error("Expected check message in output")
	}

	// Verify summary
	if !strings.Contains(output, "SUMMARY") {
		t.Error("Expected summary section")
	}
}

func TestFormatReportWithColor(t *testing.T) {
	report := &diagnostics.Report{
		Timestamp: time.Now(),
		Duration:  100 * time.Millisecond,
		Results: []*diagnostics.CheckResult{
			{
				Name:     "test_check",
				Category: diagnostics.CategoryMemory,
				Severity: diagnostics.SeverityWarning,
				Status:   diagnostics.StatusCompleted,
				Message:  "Memory usage high",
			},
		},
		Summary: diagnostics.ReportSummary{
			TotalChecks:  1,
			WarningCount: 1,
		},
	}

	formatter := NewTextFormatter(true, false) // Color enabled
	output := formatter.FormatReport(report)

	// Should contain ANSI color codes
	if !strings.Contains(output, "\033[") {
		t.Error("Expected ANSI color codes in output")
	}
}

func TestFormatResult(t *testing.T) {
	result := &diagnostics.CheckResult{
		Name:     "test_check",
		Category: diagnostics.CategoryCPU,
		Severity: diagnostics.SeverityOK,
		Status:   diagnostics.StatusCompleted,
		Message:  "All good",
		Duration: 25 * time.Millisecond,
	}

	formatter := NewTextFormatter(false, false)
	output := formatter.FormatResult(result)

	if !strings.Contains(output, "test_check") {
		t.Error("Expected check name in output")
	}
	if !strings.Contains(output, "All good") {
		t.Error("Expected message in output")
	}
}

func TestFormatResultWithError(t *testing.T) {
	result := &diagnostics.CheckResult{
		Name:     "failed_check",
		Category: diagnostics.CategoryDisk,
		Severity: diagnostics.SeverityError,
		Status:   diagnostics.StatusFailed,
		Error:    "Connection timeout",
	}

	formatter := NewTextFormatter(false, false)
	output := formatter.FormatResult(result)

	if !strings.Contains(output, "failed_check") {
		t.Error("Expected check name")
	}
	if !strings.Contains(output, "Error: Connection timeout") {
		t.Error("Expected error message")
	}
	if !strings.Contains(output, "[FAILED]") {
		t.Error("Expected failed status badge")
	}
}

func TestFormatResultVerbose(t *testing.T) {
	result := &diagnostics.CheckResult{
		Name:     "cpu_check",
		Category: diagnostics.CategoryCPU,
		Severity: diagnostics.SeverityOK,
		Status:   diagnostics.StatusCompleted,
		Duration: 30 * time.Millisecond,
		Metrics: []diagnostics.Metric{
			{
				Name:  "cpu_usage",
				Value: 45.5,
				Unit:  "percent",
			},
		},
	}

	formatter := NewTextFormatter(false, true) // Verbose mode
	output := formatter.FormatResult(result)

	// Should include metrics
	if !strings.Contains(output, "Metrics:") {
		t.Error("Expected metrics section in verbose mode")
	}
	if !strings.Contains(output, "cpu_usage") {
		t.Error("Expected metric name")
	}
	if !strings.Contains(output, "45.50") {
		t.Error("Expected metric value")
	}

	// Should include duration
	if !strings.Contains(output, "Duration: 30ms") {
		t.Error("Expected duration in verbose mode")
	}
}

func TestFormatMetric(t *testing.T) {
	formatter := NewTextFormatter(false, false)

	tests := []struct {
		name   string
		metric diagnostics.Metric
		want   []string
	}{
		{
			name: "metric without threshold",
			metric: diagnostics.Metric{
				Name:  "cpu_usage",
				Value: 50.0,
				Unit:  "percent",
			},
			want: []string{"cpu_usage", "50.00 percent"},
		},
		{
			name: "metric below max threshold",
			metric: diagnostics.Metric{
				Name:          "memory_usage",
				Value:         70.0,
				Unit:          "percent",
				Threshold:     85.0,
				ThresholdType: diagnostics.ThresholdTypeMax,
			},
			want: []string{"memory_usage", "70.00 percent", "threshold: 85.00", "[OK]"},
		},
		{
			name: "metric exceeds max threshold",
			metric: diagnostics.Metric{
				Name:          "disk_usage",
				Value:         95.0,
				Unit:          "percent",
				Threshold:     85.0,
				ThresholdType: diagnostics.ThresholdTypeMax,
			},
			want: []string{"disk_usage", "95.00 percent", "threshold: 85.00", "[WARN]"},
		},
		{
			name: "metric above min threshold",
			metric: diagnostics.Metric{
				Name:          "free_space",
				Value:         100.0,
				Unit:          "GB",
				Threshold:     50.0,
				ThresholdType: diagnostics.ThresholdTypeMin,
			},
			want: []string{"free_space", "100.00 GB", "threshold: 50.00", "[OK]"},
		},
		{
			name: "metric below min threshold",
			metric: diagnostics.Metric{
				Name:          "free_space",
				Value:         30.0,
				Unit:          "GB",
				Threshold:     50.0,
				ThresholdType: diagnostics.ThresholdTypeMin,
			},
			want: []string{"free_space", "30.00 GB", "threshold: 50.00", "[WARN]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatter.formatMetric(tt.metric)
			for _, expected := range tt.want {
				if !strings.Contains(output, expected) {
					t.Errorf("formatMetric() output missing %q\nGot: %s", expected, output)
				}
			}
		})
	}
}

func TestGetOverallStatus(t *testing.T) {
	formatter := NewTextFormatter(false, false)

	tests := []struct {
		name    string
		summary diagnostics.ReportSummary
		want    string
	}{
		{
			name:    "all healthy",
			summary: diagnostics.ReportSummary{OKCount: 5},
			want:    "HEALTHY",
		},
		{
			name:    "informational",
			summary: diagnostics.ReportSummary{InfoCount: 1},
			want:    "INFORMATIONAL",
		},
		{
			name:    "has warnings",
			summary: diagnostics.ReportSummary{WarningCount: 1},
			want:    "WARNING",
		},
		{
			name:    "has critical",
			summary: diagnostics.ReportSummary{CriticalCount: 1},
			want:    "CRITICAL",
		},
		{
			name:    "has errors",
			summary: diagnostics.ReportSummary{ErrorCount: 1},
			want:    "FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &diagnostics.Report{Summary: tt.summary}
			got := formatter.getOverallStatus(report)
			if got != tt.want {
				t.Errorf("getOverallStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSeverityIcon(t *testing.T) {
	formatter := NewTextFormatter(false, false) // No color

	tests := []struct {
		severity diagnostics.Severity
		want     string
	}{
		{diagnostics.SeverityOK, "✓"},
		{diagnostics.SeverityInfo, "ℹ"},
		{diagnostics.SeverityWarning, "⚠"},
		{diagnostics.SeverityCritical, "✗"},
		{diagnostics.SeverityError, "✗"},
		{diagnostics.Severity("unknown"), "?"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			got := formatter.getSeverityIcon(tt.severity)
			if !strings.Contains(got, tt.want) {
				t.Errorf("getSeverityIcon() = %v, want to contain %v", got, tt.want)
			}
		})
	}
}

func TestGetSeverityColor(t *testing.T) {
	formatter := NewTextFormatter(false, false)

	tests := []struct {
		severity diagnostics.Severity
		want     string
	}{
		{diagnostics.SeverityOK, colorGreen},
		{diagnostics.SeverityInfo, colorBlue},
		{diagnostics.SeverityWarning, colorYellow},
		{diagnostics.SeverityCritical, colorRed},
		{diagnostics.SeverityError, colorRed},
		{diagnostics.Severity("unknown"), colorReset},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			got := formatter.getSeverityColor(tt.severity)
			if got != tt.want {
				t.Errorf("getSeverityColor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestColorize(t *testing.T) {
	tests := []struct {
		name         string
		colorEnabled bool
		text         string
		color        string
		wantContains []string
	}{
		{
			name:         "color enabled",
			colorEnabled: true,
			text:         "hello",
			color:        colorRed,
			wantContains: []string{colorRed, "hello", colorReset},
		},
		{
			name:         "color disabled",
			colorEnabled: false,
			text:         "hello",
			color:        colorRed,
			wantContains: []string{"hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewTextFormatter(tt.colorEnabled, false)
			got := formatter.colorize(tt.text, tt.color)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("colorize() = %v, want to contain %v", got, want)
				}
			}

			if !tt.colorEnabled {
				// Should not contain color codes when disabled
				if strings.Contains(got, "\033[") {
					t.Error("Expected no color codes when color disabled")
				}
			}
		})
	}
}

func TestGroupResultsByCategory(t *testing.T) {
	formatter := NewTextFormatter(false, false)

	results := []*diagnostics.CheckResult{
		{Category: diagnostics.CategoryCPU, Name: "cpu1"},
		{Category: diagnostics.CategoryMemory, Name: "mem1"},
		{Category: diagnostics.CategoryCPU, Name: "cpu2"},
		{Category: diagnostics.CategoryDisk, Name: "disk1"},
	}

	grouped := formatter.groupResultsByCategory(results)

	if len(grouped[diagnostics.CategoryCPU]) != 2 {
		t.Errorf("Expected 2 CPU results, got %d", len(grouped[diagnostics.CategoryCPU]))
	}
	if len(grouped[diagnostics.CategoryMemory]) != 1 {
		t.Errorf("Expected 1 Memory result, got %d", len(grouped[diagnostics.CategoryMemory]))
	}
	if len(grouped[diagnostics.CategoryDisk]) != 1 {
		t.Errorf("Expected 1 Disk result, got %d", len(grouped[diagnostics.CategoryDisk]))
	}
	if len(grouped[diagnostics.CategoryNetwork]) != 0 {
		t.Errorf("Expected 0 Network results, got %d", len(grouped[diagnostics.CategoryNetwork]))
	}
}

func TestFormatSummary(t *testing.T) {
	formatter := NewTextFormatter(false, false)

	report := &diagnostics.Report{
		Summary: diagnostics.ReportSummary{
			TotalChecks:   10,
			OKCount:       5,
			InfoCount:     1,
			WarningCount:  2,
			CriticalCount: 1,
			ErrorCount:    1,
		},
	}

	output := formatter.formatSummary(report)

	// Verify summary header
	if !strings.Contains(output, "SUMMARY") {
		t.Error("Expected SUMMARY header")
	}

	// Verify counts
	if !strings.Contains(output, "Total Checks:   10") {
		t.Error("Expected total checks count")
	}
	if !strings.Contains(output, "OK:           5") {
		t.Error("Expected OK count")
	}
	if !strings.Contains(output, "Info:         1") {
		t.Error("Expected Info count")
	}
	if !strings.Contains(output, "Warnings:     2") {
		t.Error("Expected Warnings count")
	}
	if !strings.Contains(output, "Critical:     1") {
		t.Error("Expected Critical count")
	}
	if !strings.Contains(output, "Errors:       1") {
		t.Error("Expected Errors count")
	}
}

func TestFormatSummaryOnlyOK(t *testing.T) {
	formatter := NewTextFormatter(false, false)

	report := &diagnostics.Report{
		Summary: diagnostics.ReportSummary{
			TotalChecks: 5,
			OKCount:     5,
		},
	}

	output := formatter.formatSummary(report)

	// Should not show zero counts
	if strings.Contains(output, "Info:") {
		t.Error("Should not show Info when count is 0")
	}
	if strings.Contains(output, "Warnings:") {
		t.Error("Should not show Warnings when count is 0")
	}
	if strings.Contains(output, "Critical:") {
		t.Error("Should not show Critical when count is 0")
	}
	if strings.Contains(output, "Errors:") {
		t.Error("Should not show Errors when count is 0")
	}
}

func TestFormatCategoryHeader(t *testing.T) {
	formatter := NewTextFormatter(false, false)

	tests := []struct {
		category diagnostics.CheckCategory
		want     string
	}{
		{diagnostics.CategoryCPU, "CPU CHECKS"},
		{diagnostics.CategoryMemory, "MEMORY CHECKS"},
		{diagnostics.CategoryDisk, "DISK CHECKS"},
		{diagnostics.CategoryNetwork, "NETWORK CHECKS"},
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			got := formatter.formatCategoryHeader(tt.category)
			if !strings.Contains(got, tt.want) {
				t.Errorf("formatCategoryHeader() = %v, want to contain %v", got, tt.want)
			}
			// Should contain separator line
			if !strings.Contains(got, "---") {
				t.Error("Expected separator line")
			}
		})
	}
}
