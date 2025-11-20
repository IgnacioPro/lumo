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
		_ = json.NewEncoder(w).Encode(resp)
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

func TestGeminiProvider_Health(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "healthy service",
			statusCode: http.StatusOK,
			wantErr:    false,
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
				if tt.statusCode == http.StatusOK {
					_, _ = w.Write([]byte(`{"name": "models/gemini-pro"}`))
				}
			}))
			defer server.Close()

			provider, err := NewGeminiProvider(&ProviderConfig{
				APIKey: "test-key",
			}, log)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

			provider.config.Endpoint = server.URL
			provider.httpClient.SetClient(server.Client())

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

func TestGeminiProvider_Analyze_ErrorHandling(t *testing.T) {
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

			provider, err := NewGeminiProvider(&ProviderConfig{
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

func TestGeminiProvider_Analyze_ContextCancellation(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider, err := NewGeminiProvider(&ProviderConfig{
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

func TestGeminiProvider_AnalyzeStream(t *testing.T) {
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

				// Gemini uses SSE format like Anthropic
				chunks := []string{
					`data: {"candidates":[{"content":{"parts":[{"text":"System "}]}}]}`,
					`data: {"candidates":[{"content":{"parts":[{"text":"is "}]}}]}`,
					`data: {"candidates":[{"content":{"parts":[{"text":"healthy"}]}}]}`,
					`data: [DONE]`,
				}

				for _, chunk := range chunks {
					_, _ = w.Write([]byte(chunk + "\n\n"))
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
					time.Sleep(10 * time.Millisecond)
				}
			},
			expectChunks: 4, // 3 content chunks + 1 done
			expectError:  false,
			expectDone:   true,
		},
		{
			name: "streaming with empty content",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)

				chunks := []string{
					`data: {"candidates":[{"content":{"parts":[{"text":"Test"}]}}]}`,
					`data: [DONE]`,
				}

				for _, chunk := range chunks {
					_, _ = w.Write([]byte(chunk + "\n\n"))
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
				}
			},
			expectChunks: 2,
			expectError:  false,
			expectDone:   true,
		},
		{
			name: "http error status code",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error": {"message": "API key invalid"}}`))
			},
			expectChunks: 1, // Error chunk
			expectError:  true,
			expectDone:   false, // No done chunk on error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(tt.serverFunc))
			defer server.Close()

			provider, err := NewGeminiProvider(&ProviderConfig{
				APIKey: "test-key",
			}, log)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

			// Override endpoint to point to test server
			// Gemini URL format includes model, so we need to strip that
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

// ========== Streaming Tests ==========

func TestGeminiProvider_AnalyzeStream_Success(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify API key in query
		if !strings.Contains(r.URL.RawQuery, "key=test-key") {
			t.Error("Missing API key in query")
		}

		w.Header().Set("Content-Type", "text/event-stream")

		// Gemini streaming response in SSE format
		responses := []string{
			`data: {"candidates":[{"content":{"parts":[{"text":"System "}],"role":"model"}}]}`,
			`data: {"candidates":[{"content":{"parts":[{"text":"is "}],"role":"model"}}]}`,
			`data: {"candidates":[{"content":{"parts":[{"text":"healthy"}],"role":"model"},"finishReason":"STOP"}]}`,
			`data: [DONE]`,
		}

		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	provider, err := NewGeminiProvider(&ProviderConfig{
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
	var content strings.Builder

	for chunk := range ch {
		chunks = append(chunks, chunk)
		if !chunk.Done && chunk.Content != "" {
			content.WriteString(chunk.Content)
		}
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks")
	}

	if content.String() != "System is healthy" {
		t.Errorf("Expected 'System is healthy', got '%s'", content.String())
	}
}

func TestGeminiProvider_AnalyzeStream_ErrorResponse(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"code": 400, "message": "Invalid request"}}`))
	}))
	defer server.Close()

	provider, err := NewGeminiProvider(&ProviderConfig{
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
		if chunk.Error != nil {
			receivedError = true
		}
	}

	if !receivedError {
		t.Error("Expected error chunk")
	}
}
