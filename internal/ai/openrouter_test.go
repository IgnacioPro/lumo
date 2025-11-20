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
		_ = json.NewEncoder(w).Encode(resp)
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
	provider.httpClient.SetClient(server.Client())

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
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOpenRouterProvider(&ProviderConfig{
		APIKey: "test-key",
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	provider.config.Endpoint = server.URL
	provider.httpClient.SetClient(server.Client())

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
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOpenRouterProvider(&ProviderConfig{
		APIKey: "test-key",
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	provider.config.Endpoint = server.URL
	provider.httpClient.SetClient(server.Client())

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
					_ = json.NewEncoder(w).Encode(resp)
				} else {
					w.WriteHeader(tt.statusCode)
					_, _ = w.Write([]byte(`{"error": "server error"}`))
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
			provider.httpClient.SetClient(server.Client())

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

func TestOpenRouterProvider_Analyze_ErrorHandling(t *testing.T) {
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
			errMsg:     "failed to unmarshal",
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

			provider, err := NewOpenRouterProvider(&ProviderConfig{
				APIKey: "test-key",
			}, log)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

			provider.config.Endpoint = server.URL
			provider.httpClient.SetClient(server.Client())

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

func TestOpenRouterProvider_Analyze_ContextCancellation(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider, err := NewOpenRouterProvider(&ProviderConfig{
		APIKey: "test-key",
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	provider.config.Endpoint = server.URL
	provider.httpClient.SetClient(server.Client())

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

func TestOpenRouterProvider_AnalyzeStream(t *testing.T) {
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

				// OpenRouter uses OpenAI-compatible SSE format
				chunks := []string{
					`data: {"id":"gen-123","object":"chat.completion.chunk","created":1694268190,"model":"anthropic/claude-sonnet-4.5","choices":[{"index":0,"delta":{"content":"System "},"finish_reason":null}]}`,
					`data: {"id":"gen-123","object":"chat.completion.chunk","created":1694268190,"model":"anthropic/claude-sonnet-4.5","choices":[{"index":0,"delta":{"content":"is "},"finish_reason":null}]}`,
					`data: {"id":"gen-123","object":"chat.completion.chunk","created":1694268190,"model":"anthropic/claude-sonnet-4.5","choices":[{"index":0,"delta":{"content":"healthy"},"finish_reason":null}]}`,
					`data: {"id":"gen-123","object":"chat.completion.chunk","created":1694268190,"model":"anthropic/claude-sonnet-4.5","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
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
					`data: {"id":"gen-123","object":"chat.completion.chunk","created":1694268190,"model":"anthropic/claude-sonnet-4.5","choices":[{"index":0,"delta":{"content":"Test"},"finish_reason":null}]}`,
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

			provider, err := NewOpenRouterProvider(&ProviderConfig{
				APIKey: "test-key",
			}, log)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

			provider.config.Endpoint = server.URL
			provider.httpClient.SetClient(server.Client())

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

func TestOpenRouterProvider_CustomHeaders(t *testing.T) {
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
		resp := openrouterResponse{
			ID:      "gen-123",
			Model:   "anthropic/claude-sonnet-4.5",
			Object:  "chat.completion",
			Created: 1694268190,
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

	provider, err := NewOpenRouterProvider(&ProviderConfig{
		APIKey: "test-key",
		CustomHeaders: map[string]string{
			customHeaderKey: customHeaderValue,
		},
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	provider.config.Endpoint = server.URL
	provider.httpClient.SetClient(server.Client())

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

// ========== Streaming Tests ==========

func TestOpenRouterProvider_AnalyzeStream_Success(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("Missing Authorization header")
		}

		w.Header().Set("Content-Type", "text/event-stream")

		// OpenRouter uses OpenAI-compatible format
		events := []string{
			`data: {"id":"gen-123","choices":[{"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`data: {"id":"gen-123","choices":[{"delta":{"content":"System "},"finish_reason":null}]}`,
			`data: {"id":"gen-123","choices":[{"delta":{"content":"is "},"finish_reason":null}]}`,
			`data: {"id":"gen-123","choices":[{"delta":{"content":"healthy"},"finish_reason":null}]}`,
			`data: {"id":"gen-123","choices":[{"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}

		for _, event := range events {
			_, _ = w.Write([]byte(event + "\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(10 * time.Millisecond)
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
	provider.httpClient.SetClient(server.Client())

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
		t.Fatalf("AnalyzeStream() error = %v", err)
	}

	var chunks []StreamChunk
	var foundDone bool
	var content strings.Builder

	for chunk := range ch {
		chunks = append(chunks, chunk)
		if chunk.Done {
			foundDone = true
		}
		if !chunk.Done && chunk.Content != "" {
			content.WriteString(chunk.Content)
		}
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks")
	}

	if !foundDone {
		t.Error("Expected Done marker")
	}

	if content.String() != "System is healthy" {
		t.Errorf("Expected 'System is healthy', got '%s'", content.String())
	}
}

func TestOpenRouterProvider_AnalyzeStream_ErrorResponse(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	provider, err := NewOpenRouterProvider(&ProviderConfig{
		APIKey: "test-key",
	}, log)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	provider.config.Endpoint = server.URL
	provider.httpClient.SetClient(server.Client())

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
		t.Fatalf("AnalyzeStream() error = %v", err)
	}

	var receivedError bool
	for chunk := range ch {
		if chunk.Type == ChunkError && chunk.Error != nil {
			receivedError = true
		}
	}

	if !receivedError {
		t.Error("Expected error chunk")
	}
}
