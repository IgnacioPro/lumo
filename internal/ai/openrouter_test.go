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

func TestNewOpenRouterProvider(t *testing.T) {
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
				APIKey:      "sk-or-test123",
				Model:       "anthropic/claude-sonnet-4.5",
				Timeout:     60 * time.Second,
				MaxTokens:   2048,
				Temperature: 0.7,
			},
			wantErr: false,
		},
		{
			name: "valid config with defaults",
			config: &ProviderConfig{
				APIKey: "sk-or-test123",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: &ProviderConfig{
				Model: "anthropic/claude-sonnet-4.5",
			},
			wantErr: true,
			errMsg:  "API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenRouterProvider(tt.config, log)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewOpenRouterProvider() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("NewOpenRouterProvider() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("NewOpenRouterProvider() unexpected error = %v", err)
				return
			}

			if provider == nil {
				t.Fatal("NewOpenRouterProvider() returned nil provider")
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
			if provider.config.Model != tt.config.Model && tt.config.Model != "" {
				t.Errorf("Model = %q, want %q", provider.config.Model, tt.config.Model)
			}
		})
	}
}

func TestOpenRouterProvider_Name(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	provider, err := NewOpenRouterProvider(&ProviderConfig{
		APIKey: "test-key",
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if got := provider.Name(); got != "openrouter" {
		t.Errorf("Name() = %q, want %q", got, "openrouter")
	}
}

func TestOpenRouterProvider_Analyze(t *testing.T) {
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

		// Verify OpenRouter-specific headers
		if referer := r.Header.Get("HTTP-Referer"); referer != OpenRouterAppURL {
			t.Errorf("HTTP-Referer = %q, want %q", referer, OpenRouterAppURL)
		}

		if title := r.Header.Get("X-Title"); title != OpenRouterAppName {
			t.Errorf("X-Title = %q, want %q", title, OpenRouterAppName)
		}

		// Send mock response
		resp := openrouterResponse{
			ID:      "gen-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "anthropic/claude-sonnet-4.5",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role      string `json:"role"`
					Content   string `json:"content"`
					Refusal   string `json:"refusal,omitempty"`
					Reasoning string `json:"reasoning,omitempty"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role      string `json:"role"`
						Content   string `json:"content"`
						Refusal   string `json:"refusal,omitempty"`
						Reasoning string `json:"reasoning,omitempty"`
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

	provider, err := NewOpenRouterProvider(&ProviderConfig{
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

	if analysis.Provider != "openrouter" {
		t.Errorf("Provider = %q, want %q", analysis.Provider, "openrouter")
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

func TestOpenRouterProvider_Analyze_EmptyContent(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send response with empty content
		resp := openrouterResponse{
			ID:      "gen-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "anthropic/claude-sonnet-4.5",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role      string `json:"role"`
					Content   string `json:"content"`
					Refusal   string `json:"refusal,omitempty"`
					Reasoning string `json:"reasoning,omitempty"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role      string `json:"role"`
						Content   string `json:"content"`
						Refusal   string `json:"refusal,omitempty"`
						Reasoning string `json:"reasoning,omitempty"`
					}{
						Role:    "assistant",
						Content: "",
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
				CompletionTokens: 0,
				TotalTokens:      120,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOpenRouterProvider(&ProviderConfig{
		APIKey: "test-key",
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

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
	_, err = provider.Analyze(ctx, req)

	if err == nil {
		t.Error("Analyze() expected error for empty content, got nil")
	}

	if !strings.Contains(err.Error(), "empty content") {
		t.Errorf("Analyze() error = %q, want error containing 'empty content'", err.Error())
	}
}

func TestOpenRouterProvider_Analyze_Refusal(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send response with refusal
		resp := openrouterResponse{
			ID:      "gen-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "anthropic/claude-sonnet-4.5",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role      string `json:"role"`
					Content   string `json:"content"`
					Refusal   string `json:"refusal,omitempty"`
					Reasoning string `json:"reasoning,omitempty"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role      string `json:"role"`
						Content   string `json:"content"`
						Refusal   string `json:"refusal,omitempty"`
						Reasoning string `json:"reasoning,omitempty"`
					}{
						Role:    "assistant",
						Content: "",
						Refusal: "I cannot assist with that request",
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
				CompletionTokens: 0,
				TotalTokens:      120,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOpenRouterProvider(&ProviderConfig{
		APIKey: "test-key",
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

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
	_, err = provider.Analyze(ctx, req)

	if err == nil {
		t.Error("Analyze() expected error for refusal, got nil")
	}

	if !strings.Contains(err.Error(), "content policy refusal") {
		t.Errorf("Analyze() error = %q, want error containing 'content policy refusal'", err.Error())
	}
}

func TestOpenRouterProvider_Health(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "healthy",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.statusCode == http.StatusOK {
					resp := openrouterResponse{
						ID:      "gen-health",
						Object:  "chat.completion",
						Created: time.Now().Unix(),
						Model:   "anthropic/claude-sonnet-4.5",
						Choices: []struct {
							Index   int `json:"index"`
							Message struct {
								Role      string `json:"role"`
								Content   string `json:"content"`
								Refusal   string `json:"refusal,omitempty"`
								Reasoning string `json:"reasoning,omitempty"`
							} `json:"message"`
							FinishReason string `json:"finish_reason"`
						}{
							{
								Index: 0,
								Message: struct {
									Role      string `json:"role"`
									Content   string `json:"content"`
									Refusal   string `json:"refusal,omitempty"`
									Reasoning string `json:"reasoning,omitempty"`
								}{
									Role:    "assistant",
									Content: "test",
								},
								FinishReason: "stop",
							},
						},
						Usage: struct {
							PromptTokens     int `json:"prompt_tokens"`
							CompletionTokens int `json:"completion_tokens"`
							TotalTokens      int `json:"total_tokens"`
						}{
							PromptTokens:     5,
							CompletionTokens: 5,
							TotalTokens:      10,
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(resp)
				} else {
					w.WriteHeader(tt.statusCode)
					w.Write([]byte(`{"error": "server error"}`))
				}
			}))
			defer server.Close()

			provider, err := NewOpenRouterProvider(&ProviderConfig{
				APIKey: "test-key",
			}, log)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

			provider.config.Endpoint = server.URL
			provider.client = server.Client()

			ctx := context.Background()
			err = provider.Health(ctx)

			if tt.wantErr && err == nil {
				t.Error("Health() expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Health() unexpected error = %v", err)
			}
		})
	}
}
