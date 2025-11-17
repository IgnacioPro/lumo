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

func TestNewOpenAIProvider(t *testing.T) {
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
				APIKey:      "sk-test123",
				Model:       "gpt-4-turbo-preview",
				Timeout:     60 * time.Second,
				MaxTokens:   2048,
				Temperature: 0.7,
			},
			wantErr: false,
		},
		{
			name: "valid config with defaults",
			config: &ProviderConfig{
				APIKey: "sk-test123",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: &ProviderConfig{
				Model: "gpt-4",
			},
			wantErr: true,
			errMsg:  "API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenAIProvider(tt.config, log)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewOpenAIProvider() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("NewOpenAIProvider() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("NewOpenAIProvider() unexpected error = %v", err)
				return
			}

			if provider == nil {
				t.Fatal("NewOpenAIProvider() returned nil provider")
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

func TestOpenAIProvider_Name(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	provider, err := NewOpenAIProvider(&ProviderConfig{
		APIKey: "test-key",
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if got := provider.Name(); got != "openai" {
		t.Errorf("Name() = %q, want %q", got, "openai")
	}
}

func TestOpenAIProvider_Analyze(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Error("Missing or invalid Authorization header")
		}

		// Send mock response
		resp := openaiResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4-turbo-preview",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
					Refusal string `json:"refusal,omitempty"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
						Refusal string `json:"refusal,omitempty"`
					}{
						Role: "assistant",
						Content: `{
							"summary": "System is healthy",
							"overall_health": "healthy",
							"confidence": 0.92,
							"findings": [],
							"recommendations": []
						}`,
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     120,
				CompletionTokens: 180,
				TotalTokens:      300,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(&ProviderConfig{
		APIKey: "test-key",
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Override for testing
	provider.config.Endpoint = server.URL
	provider.client = server.Client()

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

	if analysis.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", analysis.Provider, "openai")
	}

	if analysis.TokensUsed == nil {
		t.Fatal("TokensUsed should not be nil")
	}

	if analysis.TokensUsed.InputTokens != 120 {
		t.Errorf("InputTokens = %d, want 120", analysis.TokensUsed.InputTokens)
	}

	if analysis.TokensUsed.OutputTokens != 180 {
		t.Errorf("OutputTokens = %d, want 180", analysis.TokensUsed.OutputTokens)
	}

	if analysis.TokensUsed.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", analysis.TokensUsed.TotalTokens)
	}
}
