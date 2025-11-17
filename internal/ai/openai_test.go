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
		_ = json.NewEncoder(w).Encode(resp)
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

func TestOpenAIProvider_Health(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "healthy service",
			statusCode: http.StatusOK,
			response: openaiResponse{
				ID:      "chatcmpl-health",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4",
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
							Role:    "assistant",
							Content: "OK",
						},
						FinishReason: "stop",
					},
				},
				Usage: struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				}{
					PromptTokens:     10,
					CompletionTokens: 1,
					TotalTokens:      11,
				},
			},
			wantErr: false,
		},
		{
			name:       "service unavailable",
			statusCode: http.StatusServiceUnavailable,
			wantErr:    true,
			errMsg:     "503",
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
			errMsg:     "401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.response != nil {
					_ = json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			provider, err := NewOpenAIProvider(&ProviderConfig{
				APIKey: "test-key",
			}, log)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

			provider.config.Endpoint = server.URL + "/v1/models"
			provider.client = server.Client()

			ctx := context.Background()
			err = provider.Health(ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Health() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Health() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Health() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestOpenAIProvider_Analyze_ErrorHandling(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "network error - 500",
			statusCode: http.StatusInternalServerError,
			response:   map[string]string{"error": "internal error"},
			wantErr:    true,
			errMsg:     "500",
		},
		{
			name:       "unauthorized - 401",
			statusCode: http.StatusUnauthorized,
			response:   map[string]string{"error": "invalid API key"},
			wantErr:    true,
			errMsg:     "401",
		},
		{
			name:       "rate limited - 429",
			statusCode: http.StatusTooManyRequests,
			response:   map[string]string{"error": "rate limit exceeded"},
			wantErr:    true,
			errMsg:     "429",
		},
		{
			name:       "malformed JSON response",
			statusCode: http.StatusOK,
			response:   "not valid json",
			wantErr:    true,
			errMsg:     "failed to decode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if str, ok := tt.response.(string); ok {
					w.Write([]byte(str))
				} else {
					_ = json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			provider, err := NewOpenAIProvider(&ProviderConfig{
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

			if !tt.wantErr {
				if err != nil {
					t.Errorf("Analyze() unexpected error = %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("Analyze() expected error containing %q, got nil", tt.errMsg)
				return
			}

			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Analyze() error = %q, want error containing %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestOpenAIProvider_Analyze_ContextCancellation(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(&ProviderConfig{
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

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = provider.Analyze(ctx, req)

	if err == nil {
		t.Error("Analyze() expected context cancellation error, got nil")
		return
	}

	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Analyze() error = %q, want context cancellation error", err.Error())
	}
}
