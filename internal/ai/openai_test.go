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
					_, _ = w.Write([]byte(str))
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

func TestOpenAIProvider_AnalyzeStream(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	tests := []struct {
		name         string
		serverFunc   func(w http.ResponseWriter, r *http.Request)
		expectChunks int
		expectError  bool
		expectDone   bool
	}{
		{
			name: "successful streaming with multiple chunks",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)

				chunks := []string{
					`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"System "},"finish_reason":null}]}`,
					`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"is "},"finish_reason":null}]}`,
					`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"healthy"},"finish_reason":null}]}`,
					`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
				}

				for _, chunk := range chunks {
					_, _ = w.Write([]byte(chunk + "\n\n"))
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
					time.Sleep(10 * time.Millisecond)
				}
			},
			expectChunks: 4, // 3 content deltas + 1 finish
			expectError:  false,
			expectDone:   true,
		},
		{
			name: "streaming with [DONE] marker",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)

				chunks := []string{
					`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Test"},"finish_reason":null}]}`,
					`data: [DONE]`,
				}

				for _, chunk := range chunks {
					_, _ = w.Write([]byte(chunk + "\n\n"))
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
				}
			},
			expectChunks: 2, // 1 content + 1 done
			expectError:  false,
			expectDone:   true,
		},
		{
			name: "http error status code",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
			},
			expectChunks: 1, // Error chunk
			expectError:  true,
			expectDone:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(tt.serverFunc))
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
			ch, err := provider.AnalyzeStream(ctx, req)

			if err != nil {
				t.Fatalf("AnalyzeStream() unexpected error creating stream: %v", err)
			}

			// Collect all chunks
			var chunks []StreamChunk
			for chunk := range ch {
				chunks = append(chunks, chunk)
			}

			if len(chunks) < tt.expectChunks {
				t.Errorf("AnalyzeStream() got %d chunks, want at least %d", len(chunks), tt.expectChunks)
			}

			// Check for error chunk if expected
			if tt.expectError {
				foundError := false
				for _, chunk := range chunks {
					if chunk.Error != nil {
						foundError = true
						break
					}
				}
				if !foundError {
					t.Error("AnalyzeStream() expected error chunk, got none")
				}
			}

			// Check for done chunk if expected
			if tt.expectDone {
				foundDone := false
				for _, chunk := range chunks {
					if chunk.Done {
						foundDone = true
						break
					}
				}
				if !foundDone {
					t.Error("AnalyzeStream() expected done chunk, got none")
				}
			}
		})
	}
}

func TestOpenAIProvider_BuildRequest_ReasoningEffort(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	tests := []struct {
		name            string
		reasoningEffort string
		wantInRequest   bool
	}{
		{
			name:            "with reasoning effort low",
			reasoningEffort: "low",
			wantInRequest:   true,
		},
		{
			name:            "with reasoning effort medium",
			reasoningEffort: "medium",
			wantInRequest:   true,
		},
		{
			name:            "with reasoning effort high",
			reasoningEffort: "high",
			wantInRequest:   true,
		},
		{
			name:            "without reasoning effort",
			reasoningEffort: "",
			wantInRequest:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenAIProvider(&ProviderConfig{
				APIKey:          "test-key",
				ReasoningEffort: tt.reasoningEffort,
			}, log)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

			req := provider.buildRequest("system prompt", "user prompt")

			if tt.wantInRequest {
				if req.ReasoningEffort != tt.reasoningEffort {
					t.Errorf("buildRequest() ReasoningEffort = %q, want %q", req.ReasoningEffort, tt.reasoningEffort)
				}
			} else {
				if req.ReasoningEffort != "" {
					t.Errorf("buildRequest() ReasoningEffort = %q, want empty", req.ReasoningEffort)
				}
			}
		})
	}
}

func TestOpenAIProvider_CustomHeaders(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	customHeaderKey := "X-Custom-Header"
	customHeaderValue := "test-value"

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify custom header is present
		if got := r.Header.Get(customHeaderKey); got != customHeaderValue {
			t.Errorf("Custom header %s = %q, want %q", customHeaderKey, got, customHeaderValue)
		}

		// Send valid response
		resp := openaiResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1694268190,
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
						Role:    "assistant",
						Content: `{"summary": "OK", "overall_health": "healthy", "confidence": 0.9, "findings": [], "recommendations": []}`,
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
				CompletionTokens: 10,
				TotalTokens:      20,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(&ProviderConfig{
		APIKey: "test-key",
		CustomHeaders: map[string]string{
			customHeaderKey: customHeaderValue,
		},
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

	if err != nil {
		t.Errorf("Analyze() with custom headers error = %v", err)
	}
}
