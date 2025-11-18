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
		_ = json.NewEncoder(w).Encode(resp)
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

func TestAnthropicProvider_Health(t *testing.T) {
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
			response: map[string]interface{}{
				"status": "ok",
			},
			wantErr: false,
		},
		{
			name:       "service unavailable",
			statusCode: http.StatusServiceUnavailable,
			response: map[string]interface{}{
				"error": "service down",
			},
			wantErr: true,
			errMsg:  "503",
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

			provider, err := NewAnthropicProvider(&ProviderConfig{
				APIKey: "test-key",
			}, log)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

			provider.config.Endpoint = server.URL
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

func TestAnthropicProvider_Analyze_ErrorHandling(t *testing.T) {
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

			provider, err := NewAnthropicProvider(&ProviderConfig{
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

func TestAnthropicProvider_Analyze_ContextCancellation(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	// Create a server that delays response
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider, err := NewAnthropicProvider(&ProviderConfig{
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

	// Create context that cancels immediately
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = provider.Analyze(ctx, req)

	if err == nil {
		t.Error("Analyze() expected context cancellation error, got nil")
		return
	}

	// Should get context deadline exceeded or context canceled
	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Analyze() error = %q, want context cancellation error", err.Error())
	}
}

func TestAnthropicProvider_AnalyzeStream(t *testing.T) {
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
				// Verify request has stream=true
				if r.Method != "POST" {
					t.Errorf("Expected POST request, got %s", r.Method)
				}

				// Send SSE-formatted streaming response
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)

				// Send multiple content chunks
				chunks := []string{
					`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"System "}}`,
					`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"is "}}`,
					`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"healthy"}}`,
					`data: {"type":"message_stop"}`,
				}

				for _, chunk := range chunks {
					_, _ = w.Write([]byte(chunk + "\n\n"))
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
					time.Sleep(10 * time.Millisecond)
				}
			},
			expectChunks: 4, // 3 text deltas + 1 final done
			expectError:  false,
			expectDone:   true,
		},
		{
			name: "streaming with [DONE] marker",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)

				chunks := []string{
					`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Test content"}}`,
					`data: [DONE]`,
				}

				for _, chunk := range chunks {
					_, _ = w.Write([]byte(chunk + "\n\n"))
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
				}
			},
			expectChunks: 1,
			expectError:  false,
			expectDone:   false, // [DONE] breaks without final chunk
		},
		{
			name: "http error status code",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error": "bad request"}`))
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

			provider, err := NewAnthropicProvider(&ProviderConfig{
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

func TestAnthropicProvider_AnalyzeStream_ContextCancellation(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	// Create server that streams slowly
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Stream slowly to allow cancellation
		for i := 0; i < 10; i++ {
			_, _ = w.Write([]byte(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"chunk"}}` + "\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(500 * time.Millisecond)
		}
	}))
	defer server.Close()

	provider, err := NewAnthropicProvider(&ProviderConfig{
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

	// Create context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ch, err := provider.AnalyzeStream(ctx, req)
	if err != nil {
		t.Fatalf("AnalyzeStream() unexpected error: %v", err)
	}

	// Stream should end due to context cancellation
	chunks := 0
	for range ch {
		chunks++
	}

	// Should receive at least one chunk before cancellation
	if chunks == 0 {
		t.Error("AnalyzeStream() expected at least one chunk before cancellation")
	}
}

func TestAnthropicProvider_CustomHeaders(t *testing.T) {
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
					Text: `{"summary": "OK", "overall_health": "healthy", "confidence": 0.9, "findings": [], "recommendations": []}`,
				},
			},
			Model: "claude-sonnet-4-5-20250929",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{
				InputTokens:  10,
				OutputTokens: 10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewAnthropicProvider(&ProviderConfig{
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

// testLogWriter writes log output to test output
type testLogWriter struct {
	t *testing.T
}

func (w testLogWriter) Write(p []byte) (n int, err error) {
	w.t.Log(string(p))
	return len(p), nil
}
