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

func TestNewAnthropicProvider(t *testing.T) {
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
				APIKey:      "sk-ant-test123",
				Model:       "claude-sonnet-4-5-20250929",
				Timeout:     60 * time.Second,
				MaxTokens:   2048,
				Temperature: 0.7,
			},
			wantErr: false,
		},
		{
			name: "valid config with defaults",
			config: &ProviderConfig{
				APIKey: "sk-ant-test123",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: &ProviderConfig{
				Model: "claude-sonnet-4-5-20250929",
			},
			wantErr: true,
			errMsg:  "API key is required",
		},
		{
			name: "invalid endpoint",
			config: &ProviderConfig{
				APIKey:   "sk-ant-test123",
				Endpoint: "not-a-url",
			},
			wantErr: true,
			errMsg:  "invalid endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewAnthropicProvider(tt.config, log)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewAnthropicProvider() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("NewAnthropicProvider() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("NewAnthropicProvider() unexpected error = %v", err)
				return
			}

			if provider == nil {
				t.Fatal("NewAnthropicProvider() returned nil provider")
			}

			// Verify defaults were applied
			if provider.config.Model == "" {
				t.Error("Model should have default value")
			}
			if provider.config.Timeout == 0 {
				t.Error("Timeout should have default value")
			}
			if provider.config.MaxTokens == 0 {
				t.Error("MaxTokens should have default value")
			}
			if provider.config.Temperature == 0 {
				t.Error("Temperature should have default value")
			}
		})
	}
}

func TestAnthropicProvider_Name(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	provider, err := NewAnthropicProvider(&ProviderConfig{
		APIKey: "test-key",
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if got := provider.Name(); got != "anthropic" {
		t.Errorf("Name() = %q, want %q", got, "anthropic")
	}
}

func TestAnthropicProvider_Analyze(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	// Create mock server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") == "" {
			t.Error("Missing x-api-key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("Missing anthropic-version header")
		}

		// Send mock response
		resp := anthropicResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{
					Type: "text",
					Text: `{
						"summary": "System is healthy",
						"overall_health": "healthy",
						"confidence": 0.95,
						"findings": [],
						"recommendations": []
					}`,
				},
			},
			Model:      "claude-sonnet-4-5-20250929",
			StopReason: "end_turn",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{
				InputTokens:  150,
				OutputTokens: 200,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create provider with fake endpoint, then override
	provider, err := NewAnthropicProvider(&ProviderConfig{
		APIKey: "test-key",
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Override endpoint and client for testing
	provider.config.Endpoint = server.URL
	provider.client = server.Client()

	// Create analysis request
	req := &AnalysisRequest{
		Report: &diagnostics.Report{
			Timestamp: time.Now(),
			Duration:  100 * time.Millisecond,
			Results:   []*diagnostics.CheckResult{},
		},
		SystemInfo: SystemInfo{Hostname: "test-host"},
	}

	// Run analysis
	ctx := context.Background()
	analysis, err := provider.Analyze(ctx, req)

	if err != nil {
		t.Errorf("Analyze() error = %v, want nil", err)
	}

	if analysis == nil {
		t.Fatal("Analyze() returned nil analysis")
	}

	// Verify response fields
	if analysis.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", analysis.Provider, "anthropic")
	}

	if analysis.Summary == "" {
		t.Error("Summary should not be empty")
	}

	if analysis.TokensUsed == nil {
		t.Fatal("TokensUsed should not be nil")
	}

	if analysis.TokensUsed.InputTokens != 150 {
		t.Errorf("InputTokens = %d, want 150", analysis.TokensUsed.InputTokens)
	}

	if analysis.TokensUsed.OutputTokens != 200 {
		t.Errorf("OutputTokens = %d, want 200", analysis.TokensUsed.OutputTokens)
	}

	if analysis.TokensUsed.TotalTokens != 350 {
		t.Errorf("TotalTokens = %d, want 350", analysis.TokensUsed.TotalTokens)
	}
}

// testLogWriter writes log output to test output
type testLogWriter struct {
	t *testing.T
}

func (w testLogWriter) Write(p []byte) (n int, err error) {
	w.t.Log(string(p))
	return len(p), nil
}
