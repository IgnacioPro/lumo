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

func TestOllamaProvider_Health(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_, _ = w.Write([]byte(`{"models": []}`))
				}
			}))
			defer server.Close()

			provider, err := NewOllamaProvider(&ProviderConfig{
				Endpoint: server.URL,
			}, log)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

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

func TestOllamaProvider_Analyze_ErrorHandling(t *testing.T) {
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
			name:       "malformed JSON response",
			statusCode: http.StatusOK,
			response:   "not valid json",
			wantErr:    true,
			errMsg:     "failed to unmarshal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if str, ok := tt.response.(string); ok {
					_, _ = w.Write([]byte(str))
				} else {
					_ = json.NewEncoder(w).Encode(tt.response)
				}
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

func TestOllamaProvider_Analyze_ContextCancellation(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
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

func TestOllamaProvider_AnalyzeStream(t *testing.T) {
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
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				// Ollama sends newline-delimited JSON (not SSE)
				chunks := []string{
					`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"System "},"done":false}`,
					`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"is "},"done":false}`,
					`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"healthy"},"done":false}`,
					`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true}`,
				}

				for _, chunk := range chunks {
					_, _ = w.Write([]byte(chunk + "\n"))
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
			name: "streaming with done flag",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				chunks := []string{
					`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Test"},"done":false}`,
					`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true}`,
				}

				for _, chunk := range chunks {
					_, _ = w.Write([]byte(chunk + "\n"))
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
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error": "Internal server error"}`))
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

			provider, err := NewOllamaProvider(&ProviderConfig{}, log)
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

// ========== Streaming Tests ==========

func TestOllamaProvider_AnalyzeStream_Success(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")

		// Ollama uses newline-delimited JSON
		responses := []string{
			`{"model":"llama2","created_at":"2023-01-01T00:00:00Z","message":{"role":"assistant","content":"System "},"done":false}`,
			`{"model":"llama2","created_at":"2023-01-01T00:00:00Z","message":{"role":"assistant","content":"is "},"done":false}`,
			`{"model":"llama2","created_at":"2023-01-01T00:00:00Z","message":{"role":"assistant","content":"healthy"},"done":false}`,
			`{"model":"llama2","created_at":"2023-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true}`,
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

	provider, err := NewOllamaProvider(&ProviderConfig{
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
		t.Fatal("Expected to receive chunks")
	}

	if !foundDone {
		t.Error("Expected Done marker")
	}

	if content.String() != "System is healthy" {
		t.Errorf("Expected 'System is healthy', got '%s'", content.String())
	}
}

func TestOllamaProvider_AnalyzeStream_ErrorResponse(t *testing.T) {
	log := logrus.New()
	log.SetOutput(testLogWriter{t})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "model not found"}`))
	}))
	defer server.Close()

	provider, err := NewOllamaProvider(&ProviderConfig{
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
