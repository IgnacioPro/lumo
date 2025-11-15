package ai

import (
	"fmt"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestParseAnalysisResponse_ValidJSON(t *testing.T) {
	jsonResponse := `{
		"summary": "System is healthy",
		"overall_health": "healthy",
		"confidence": 0.95,
		"findings": [
			{
				"category": "performance",
				"severity": "low",
				"title": "CPU usage normal",
				"description": "CPU load is within normal parameters",
				"evidence": {"cpu_load": 0.45},
				"related_checks": ["cpu"]
			}
		],
		"recommendations": [
			{
				"title": "Monitor trends",
				"description": "Continue monitoring CPU trends",
				"priority": "low",
				"risk": "low",
				"estimated_time": "5m",
				"commands": ["uptime"]
			}
		]
	}`

	response, err := ParseAnalysisResponse(jsonResponse, "test", "test-model")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response.Summary != "System is healthy" {
		t.Errorf("Expected summary 'System is healthy', got '%s'", response.Summary)
	}

	if response.OverallHealth != "healthy" {
		t.Errorf("Expected overall_health 'healthy', got '%s'", response.OverallHealth)
	}

	if response.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", response.Confidence)
	}

	if len(response.Findings) != 1 {
		t.Fatalf("Expected 1 finding, got %d", len(response.Findings))
	}

	finding := response.Findings[0]
	if finding.Category != "performance" {
		t.Errorf("Expected category 'performance', got '%s'", finding.Category)
	}

	// Note: "low" severity gets normalized to "info" by parseSeverity()
	if finding.Severity != "info" {
		t.Errorf("Expected severity 'info', got '%s'", finding.Severity)
	}

	if len(response.Recommendations) != 1 {
		t.Fatalf("Expected 1 recommendation, got %d", len(response.Recommendations))
	}

	rec := response.Recommendations[0]
	if rec.Priority != "low" {
		t.Errorf("Expected priority 'low', got '%s'", rec.Priority)
	}
}

func TestParseAnalysisResponse_JSONInMarkdownCodeBlock(t *testing.T) {
	markdownResponse := "```json\n" + `{
		"summary": "Test",
		"overall_health": "healthy",
		"confidence": 0.8,
		"findings": [],
		"recommendations": []
	}` + "\n```"

	response, err := ParseAnalysisResponse(markdownResponse, "test", "test-model")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response.Summary != "Test" {
		t.Errorf("Expected summary 'Test', got '%s'", response.Summary)
	}

	if response.OverallHealth != "healthy" {
		t.Errorf("Expected overall_health 'healthy', got '%s'", response.OverallHealth)
	}
}

func TestParseAnalysisResponse_InvalidJSON(t *testing.T) {
	invalidJSON := `{"summary": "Test", "invalid json here`

	_, err := ParseAnalysisResponse(invalidJSON, "test", "test-model")
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

func TestParseAnalysisResponse_EmptyString(t *testing.T) {
	_, err := ParseAnalysisResponse("", "test", "test-model")
	if err == nil {
		t.Fatal("Expected error for empty string, got nil")
	}
}

func TestFormatMetricValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "float64 with decimals",
			value:    12.456,
			expected: "12.46",
		},
		{
			name:     "float64 whole number",
			value:    100.0,
			expected: "100.00",
		},
		{
			name:     "string value",
			value:    "active",
			expected: "active",
		},
		{
			name:     "int value",
			value:    42,
			expected: "42",
		},
		{
			name:     "bool value",
			value:    true,
			expected: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMetricValue(tt.value)
			if result != tt.expected {
				t.Errorf("formatMetricValue(%v) = %s, want %s", tt.value, result, tt.expected)
			}
		})
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	builder := NewPromptBuilder()
	prompt := builder.BuildSystemPrompt()

	if prompt == "" {
		t.Error("Expected non-empty system prompt")
	}

	// Check for key phrases that should be in the system prompt
	keyPhrases := []string{
		"SRE",
		"DevOps",
		"diagnostic",
		"JSON",
	}

	for _, phrase := range keyPhrases {
		if !contains(prompt, phrase) {
			t.Errorf("Expected system prompt to contain '%s'", phrase)
		}
	}
}

func TestBuildAnalysisPrompt(t *testing.T) {
	builder := NewPromptBuilder()

	// Create a minimal diagnostic report
	report := &diagnostics.Report{
		Timestamp: time.Now(),
		Results:   []*diagnostics.CheckResult{},
		Summary: diagnostics.ReportSummary{
			TotalChecks: 1,
			OKCount:     1,
		},
	}

	// Create a minimal analysis request
	req := &AnalysisRequest{
		Report: report,
		SystemInfo: SystemInfo{
			Hostname: "test-server",
			Platform: "linux",
		},
		Focus: []string{"cpu", "memory"},
	}

	prompt, err := builder.BuildAnalysisPrompt(req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if prompt == "" {
		t.Error("Expected non-empty analysis prompt")
	}

	// Check for key elements
	if !contains(prompt, "test-server") {
		t.Error("Expected prompt to contain hostname")
	}

	if !contains(prompt, "cpu") {
		t.Error("Expected prompt to contain focus area 'cpu'")
	}
}

func TestPromptBuilder_WithThinking(t *testing.T) {
	builder := NewPromptBuilder()

	// Default should have thinking enabled
	if !builder.includeThinking {
		t.Error("Expected includeThinking to be true by default")
	}

	// Test disabling thinking
	builder.WithThinking(false)
	if builder.includeThinking {
		t.Error("Expected includeThinking to be false after WithThinking(false)")
	}

	// Test enabling thinking
	builder.WithThinking(true)
	if !builder.includeThinking {
		t.Error("Expected includeThinking to be true after WithThinking(true)")
	}

	// Test method chaining
	result := builder.WithThinking(false)
	if result != builder {
		t.Error("WithThinking should return the same builder for chaining")
	}
}

func TestPromptBuilder_WithFocus(t *testing.T) {
	builder := NewPromptBuilder()

	// Test setting focus areas
	builder.WithFocus("cpu", "memory")
	if len(builder.focusAreas) != 2 {
		t.Errorf("Expected 2 focus areas, got %d", len(builder.focusAreas))
	}
	if builder.focusAreas[0] != "cpu" {
		t.Errorf("Expected first focus area to be 'cpu', got '%s'", builder.focusAreas[0])
	}
	if builder.focusAreas[1] != "memory" {
		t.Errorf("Expected second focus area to be 'memory', got '%s'", builder.focusAreas[1])
	}

	// Test method chaining
	result := builder.WithFocus("disk")
	if result != builder {
		t.Error("WithFocus should return the same builder for chaining")
	}

	// Test empty focus areas
	builder2 := NewPromptBuilder().WithFocus()
	if len(builder2.focusAreas) != 0 {
		t.Errorf("Expected 0 focus areas, got %d", len(builder2.focusAreas))
	}
}

func TestPromptBuilder_Chaining(t *testing.T) {
	builder := NewPromptBuilder().
		WithThinking(false).
		WithFocus("cpu", "memory")

	if builder.includeThinking {
		t.Error("Expected includeThinking to be false")
	}

	if len(builder.focusAreas) != 2 {
		t.Errorf("Expected 2 focus areas, got %d", len(builder.focusAreas))
	}
}

func TestFormatCheckResult(t *testing.T) {
	builder := NewPromptBuilder()
	result := &diagnostics.CheckResult{
		Name:     "cpu_check",
		Category: diagnostics.CategoryCPU,
		Status:   diagnostics.StatusCompleted,
		Severity: diagnostics.SeverityWarning,
		Message:  "CPU load is high",
		Data: map[string]interface{}{
			"cpu_count": 8,
			"load_avg":  4.5,
		},
		Metrics: []diagnostics.Metric{
			{
				Name:          "cpu_usage",
				Value:         85.5,
				Unit:          "%",
				Threshold:     80.0,
				ThresholdType: diagnostics.ThresholdTypeMax,
			},
		},
	}

	formatted := builder.formatCheckResult(result)

	// Check that the formatted string contains expected elements
	if !contains(formatted, "cpu_check") {
		t.Error("Expected formatted result to contain check name")
	}
	if !contains(formatted, "CPU load is high") {
		t.Error("Expected formatted result to contain message")
	}
	if !contains(formatted, "warning") {
		t.Error("Expected formatted result to contain severity")
	}
	if !contains(formatted, "cpu_usage") {
		t.Error("Expected formatted result to contain metric name")
	}
}

func TestFormatSystemInfo(t *testing.T) {
	builder := NewPromptBuilder()
	systemInfo := SystemInfo{
		Hostname:      "test-server",
		Platform:      "linux",
		Architecture:  "amd64",
		KernelVersion: "5.10.0",
	}

	formatted := builder.formatSystemInfo(systemInfo)

	// Check that all system info fields are included
	if !contains(formatted, "test-server") {
		t.Error("Expected formatted info to contain hostname")
	}
	if !contains(formatted, "linux") {
		t.Error("Expected formatted info to contain platform")
	}
	if !contains(formatted, "amd64") {
		t.Error("Expected formatted info to contain architecture")
	}
	if !contains(formatted, "5.10.0") {
		t.Error("Expected formatted info to contain kernel version")
	}
}

func TestFormatSystemInfo_PartialData(t *testing.T) {
	builder := NewPromptBuilder()
	systemInfo := SystemInfo{
		Hostname: "minimal-server",
		Platform: "linux",
		// Other fields empty
	}

	formatted := builder.formatSystemInfo(systemInfo)

	// Should still work with partial data
	if !contains(formatted, "minimal-server") {
		t.Error("Expected formatted info to contain hostname")
	}
	if !contains(formatted, "linux") {
		t.Error("Expected formatted info to contain platform")
	}
}

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected diagnostics.Severity
	}{
		{"lowercase info", "info", diagnostics.SeverityInfo},
		{"uppercase INFO", "INFO", diagnostics.SeverityInfo},
		{"lowercase warning", "warning", diagnostics.SeverityWarning},
		{"uppercase WARNING", "WARNING", diagnostics.SeverityWarning},
		{"lowercase error", "error", diagnostics.SeverityError},
		{"uppercase ERROR", "ERROR", diagnostics.SeverityError},
		{"lowercase critical", "critical", diagnostics.SeverityCritical},
		{"uppercase CRITICAL", "CRITICAL", diagnostics.SeverityCritical},
		{"unknown maps to info", "unknown", diagnostics.SeverityInfo},
		{"empty string maps to info", "", diagnostics.SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSeverity(tt.input)
			if result != tt.expected {
				t.Errorf("parseSeverity(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatMetricValue_AdditionalCases(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "negative float",
			value:    -5.123,
			expected: "-5.12",
		},
		{
			name:     "zero value",
			value:    0.0,
			expected: "0.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMetricValue(tt.value)
			if result != tt.expected {
				t.Errorf("formatMetricValue(%v) = %s, want %s", tt.value, result, tt.expected)
			}
		})
	}
}

func TestAIError_Unwrap(t *testing.T) {
	innerErr := fmt.Errorf("inner error")
	aiErr := &Error{
		Op:       "analyze",
		Provider: "test",
		Err:      innerErr,
	}

	unwrapped := aiErr.Unwrap()
	if unwrapped != innerErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, innerErr)
	}

	// Test Error without wrapped error
	aiErr2 := &Error{
		Op:       "analyze",
		Provider: "test",
	}
	if aiErr2.Unwrap() != nil {
		t.Error("Unwrap() should return nil when no error is wrapped")
	}
}

func TestBuildAnalysisPrompt_WithSelectedChecks(t *testing.T) {
	builder := NewPromptBuilder()

	report := &diagnostics.Report{
		Timestamp: time.Now(),
		Results:   []*diagnostics.CheckResult{},
		Summary: diagnostics.ReportSummary{
			TotalChecks: 2,
			OKCount:     2,
		},
	}

	req := &AnalysisRequest{
		Report: report,
		SystemInfo: SystemInfo{
			Hostname: "test-server",
			Platform: "linux",
		},
		SelectedChecks: []string{"cpu", "memory"},
	}

	prompt, err := builder.BuildAnalysisPrompt(req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should mention selected checks
	if !contains(prompt, "Selected Checks") {
		t.Error("Expected prompt to contain 'Selected Checks' section")
	}
	if !contains(prompt, "cpu") {
		t.Error("Expected prompt to contain 'cpu' in selected checks")
	}
	if !contains(prompt, "memory") {
		t.Error("Expected prompt to contain 'memory' in selected checks")
	}
}

func TestBuildAnalysisPrompt_WithResults(t *testing.T) {
	builder := NewPromptBuilder()

	results := []*diagnostics.CheckResult{
		{
			Name:     "cpu_check",
			Category: diagnostics.CategoryCPU,
			Status:   diagnostics.StatusCompleted,
			Severity: diagnostics.SeverityInfo,
			Message:  "CPU is OK",
			Metrics: []diagnostics.Metric{
				{
					Name:  "usage",
					Value: 45.0,
					Unit:  "%",
				},
			},
		},
	}

	report := &diagnostics.Report{
		Timestamp: time.Now(),
		Results:   results,
		Summary: diagnostics.ReportSummary{
			TotalChecks: 1,
			OKCount:     1,
		},
	}

	req := &AnalysisRequest{
		Report: report,
		SystemInfo: SystemInfo{
			Hostname: "test-server",
			Platform: "linux",
		},
	}

	prompt, err := builder.BuildAnalysisPrompt(req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should contain result information
	if !contains(prompt, "cpu_check") {
		t.Error("Expected prompt to contain check result")
	}
	if !contains(prompt, "CPU is OK") {
		t.Error("Expected prompt to contain check message")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr))))
}
