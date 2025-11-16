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

func TestNewGeminiProvider(t *testing.T) {
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
				APIKey:      "test-api-key",
				Model:       "gemini-2.0-flash-exp",
				Timeout:     60 * time.Second,
				MaxTokens:   2048,
				Temperature: 0.7,
			},
			wantErr: false,
		},
		{
			name: "valid config with defaults",
			config: &ProviderConfig{
				APIKey: "test-api-key",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: &ProviderConfig{
				Model: "gemini-pro",
			},
			wantErr: true,
			errMsg:  "API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewGeminiProvider(tt.config, log)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewGeminiProvider() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("NewGeminiProvider() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("NewGeminiProvider() unexpected error = %v", err)
				return
			}

			if provider == nil {
				t.Fatal("NewGeminiProvider() returned nil provider")
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

func TestGeminiProvider_Name(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	provider, err := NewGeminiProvider(&ProviderConfig{
		APIKey: "test-key",
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if got := provider.Name(); got != "gemini" {
		t.Errorf("Name() = %q, want %q", got, "gemini")
	}
}

func TestGeminiProvider_Analyze(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Check API key in URL
		if !strings.Contains(r.URL.RawQuery, "key=") {
			t.Error("API key should be in URL query parameters")
		}

		// Send mock response
		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{
							{
								"text": `{
									"summary": "Gemini analysis complete",
									"overall_health": "healthy",
									"confidence": 0.91,
									"findings": [],
									"recommendations": []
								}`,
							},
						},
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     200,
				"candidatesTokenCount": 150,
				"totalTokenCount":      350,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewGeminiProvider(&ProviderConfig{
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

	if analysis.Provider != "gemini" {
		t.Errorf("Provider = %q, want %q", analysis.Provider, "gemini")
	}

	if analysis.Summary == "" {
		t.Error("Summary should not be empty")
	}

	// Verify token usage
	if analysis.TokensUsed == nil {
		t.Fatal("TokensUsed should not be nil")
	}

	if analysis.TokensUsed.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200", analysis.TokensUsed.InputTokens)
	}

	if analysis.TokensUsed.OutputTokens != 150 {
		t.Errorf("OutputTokens = %d, want 150", analysis.TokensUsed.OutputTokens)
	}

	if analysis.TokensUsed.TotalTokens != 350 {
		t.Errorf("TotalTokens = %d, want 350", analysis.TokensUsed.TotalTokens)
	}
}
