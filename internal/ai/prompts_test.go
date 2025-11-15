package ai

import (
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

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr))))
}
