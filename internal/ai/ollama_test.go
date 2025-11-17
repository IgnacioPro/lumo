package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/sirupsen/logrus"
)

func TestNewOllamaProvider(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	tests := []struct {
		name    string
		config  *ProviderConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with all fields",
			config: &ProviderConfig{
				Model:       "llama3.1:8b",
				Endpoint:    "http://localhost:11434/api/chat",
				Timeout:     120 * time.Second,
				MaxTokens:   2048,
				Temperature: 0.8,
			},
			wantErr: false,
		},
		{
			name:    "valid config with defaults",
			config:  &ProviderConfig{},
			wantErr: false,
		},
		{
			name: "custom model",
			config: &ProviderConfig{
				Model: "codellama:13b",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOllamaProvider(tt.config, log)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewOllamaProvider() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("NewOllamaProvider() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("NewOllamaProvider() unexpected error = %v", err)
				return
			}

			if provider == nil {
				t.Fatal("NewOllamaProvider() returned nil provider")
			}

			// Verify defaults
			if provider.config.Model == "" {
				t.Error("Model should have default value")
			}
			if provider.config.Timeout == 0 {
				t.Error("Timeout should have default value")
			}
			if provider.config.Temperature == 0 {
				t.Error("Temperature should have a value")
			}
		})
	}
}

func TestOllamaProvider_Name(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	provider, err := NewOllamaProvider(&ProviderConfig{}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if got := provider.Name(); got != "ollama" {
		t.Errorf("Name() = %q, want %q", got, "ollama")
	}
}

func TestOllamaProvider_Analyze(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	// Ollama allows localhost, so use regular HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type: application/json")
		}

		// Parse request
		var req ollamaRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.Model == "" {
			t.Error("Model should be set in request")
		}

		if len(req.Messages) < 2 {
			t.Error("Expected system and user messages")
		}

		// Send mock response
		resp := ollamaResponse{
			Model:     "llama3.1:8b",
			CreatedAt: time.Now().Format(time.RFC3339),
			Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{
				Role: "assistant",
				Content: `{
					"summary": "Local system analysis complete",
					"overall_health": "healthy",
					"confidence": 0.88,
					"findings": [],
					"recommendations": []
				}`,
			},
			Done: true,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOllamaProvider(&ProviderConfig{
		Endpoint: server.URL,
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	req := &AnalysisRequest{
		Report: &diagnostics.Report{
			Timestamp: time.Now(),
			Duration:  100 * time.Millisecond,
			Results:   []*diagnostics.CheckResult{},
		},
		SystemInfo: SystemInfo{Hostname: "test-host"},
	}

	ctx := context.Background()
	analysis, err := provider.Analyze(ctx, req)

	if err != nil {
		t.Errorf("Analyze() error = %v, want nil", err)
	}

	if analysis == nil {
		t.Fatal("Analyze() returned nil analysis")
	}

	if analysis.Provider != "ollama" {
		t.Errorf("Provider = %q, want %q", analysis.Provider, "ollama")
	}

	if analysis.Summary == "" {
		t.Error("Summary should not be empty")
	}

	// Ollama doesn't provide token usage, so it should be nil
	if analysis.TokensUsed != nil {
		t.Error("Ollama should not provide token usage")
	}
}

func TestOllamaProvider_NoAPIKeyRequired(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	// Ollama doesn't require API key
	provider, err := NewOllamaProvider(&ProviderConfig{
		APIKey: "", // Empty API key should be fine
	}, log)

	if err != nil {
		t.Errorf("NewOllamaProvider() should not require API key, got error: %v", err)
	}

	if provider == nil {
		t.Error("Provider should be created without API key")
	}
}
