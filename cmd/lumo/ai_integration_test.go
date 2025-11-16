package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/diagnostics"
)

// TestRunAIAnalysis_Ollama tests AI analysis with mock Ollama API
//
// Note: We use Ollama for integration testing because it allows localhost endpoints,
// unlike Anthropic/OpenAI which enforce strict endpoint validation for security.
// The HTTP provider details are tested separately in internal/ai package tests.
func TestRunAIAnalysis_Ollama(t *testing.T) {
	// Create mock Ollama server
	mockResponse := map[string]interface{}{
		"model":     "llama3.1:8b",
		"created_at": time.Now().Format(time.RFC3339),
		"message": map[string]interface{}{
			"role": "assistant",
			"content": `{
				"summary": "Local analysis complete",
				"overall_health": "healthy",
				"confidence": 0.85,
				"findings": [],
				"recommendations": []
			}`,
		},
		"done": true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	// Create config with mock endpoint
	cfg := config.DefaultConfig()
	cfg.AI.Provider = "ollama"
	cfg.AI.Endpoint = server.URL
	cfg.AI.Timeout = 5 * time.Second

	// Create mock diagnostic report
	report := &diagnostics.Report{
		Timestamp: time.Now(),
		Duration:  100 * time.Millisecond,
		Results:   []*diagnostics.CheckResult{},
	}

	// Run AI analysis
	analysis, err := runAIAnalysis(cfg, report, "test-host", []string{}, []string{})

	if err != nil {
		t.Errorf("runAIAnalysis() error = %v, want nil", err)
	}

	if analysis == nil {
		t.Fatal("runAIAnalysis() returned nil analysis")
	}

	if analysis.Provider != "ollama" {
		t.Errorf("Analysis provider = %s, want ollama", analysis.Provider)
	}
}

// TestRunAIAnalysis_InvalidProvider tests error handling for invalid provider
func TestRunAIAnalysis_InvalidProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AI.Provider = "invalid-provider"

	report := &diagnostics.Report{
		Timestamp: time.Now(),
		Duration:  100 * time.Millisecond,
		Results:   []*diagnostics.CheckResult{},
	}

	_, err := runAIAnalysis(cfg, report, "test-host", []string{}, []string{})

	if err == nil {
		t.Error("runAIAnalysis() with invalid provider should return error")
	}
}

// TestRunAIAnalysis_WithFocusAreas tests AI analysis with focus areas using Ollama
func TestRunAIAnalysis_WithFocusAreas(t *testing.T) {
	// Create mock Ollama server that captures the request
	var receivedRequest map[string]interface{}

	mockResponse := map[string]interface{}{
		"model":      "llama3.1:8b",
		"created_at": time.Now().Format(time.RFC3339),
		"message": map[string]interface{}{
			"role": "assistant",
			"content": `{
				"summary": "Focused analysis on CPU and memory",
				"overall_health": "healthy",
				"confidence": 0.92,
				"findings": [],
				"recommendations": []
			}`,
		},
		"done": true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture request body
		json.NewDecoder(r.Body).Decode(&receivedRequest)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.AI.Provider = "ollama"
	cfg.AI.Endpoint = server.URL
	cfg.AI.Timeout = 5 * time.Second

	report := &diagnostics.Report{
		Timestamp: time.Now(),
		Duration:  100 * time.Millisecond,
		Results:   []*diagnostics.CheckResult{},
	}

	// Run with focus areas
	focusAreas := []string{"cpu", "memory"}
	analysis, err := runAIAnalysis(cfg, report, "test-host", []string{}, focusAreas)

	if err != nil {
		t.Errorf("runAIAnalysis() error = %v, want nil", err)
	}

	if analysis == nil {
		t.Fatal("runAIAnalysis() returned nil analysis")
	}

	// Verify the request was received
	// (Focus areas are embedded in the prompt sent to the provider)
	if receivedRequest == nil {
		t.Error("Server did not receive request")
	}
}

// Note: Provider-level Health() tests are in internal/ai package tests
// These cmd/lumo integration tests focus on the runAIAnalysis() orchestration logic
