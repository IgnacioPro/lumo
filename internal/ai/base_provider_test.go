package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAdapter is a mock implementation of ProviderAdapter for testing
type mockAdapter struct {
	name            string
	config          *ProviderConfig
	endpoint        string
	buildReqErr     error
	parseRespErr    error
	parseRespResult string
	usage           *TokenUsage
}

func (m *mockAdapter) Name() string {
	return m.name
}

func (m *mockAdapter) BuildRequest(systemPrompt, userPrompt string, stream bool) (interface{}, error) {
	if m.buildReqErr != nil {
		return nil, m.buildReqErr
	}

	return map[string]interface{}{
		"system": systemPrompt,
		"user":   userPrompt,
		"stream": stream,
	}, nil
}

func (m *mockAdapter) ParseResponse(body []byte) (string, *TokenUsage, error) {
	if m.parseRespErr != nil {
		return "", nil, m.parseRespErr
	}

	if m.parseRespResult != "" {
		return m.parseRespResult, m.usage, nil
	}

	// Default: parse as generic response
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", nil, err
	}

	content, ok := resp["content"].(string)
	if !ok {
		return "", nil, nil
	}

	return content, m.usage, nil
}

func (m *mockAdapter) BuildHeaders() map[string]string {
	return map[string]string{
		"Content-Type":   "application/json",
		"Authorization":  "Bearer " + m.config.APIKey,
		"X-Test-Provider": m.name,
	}
}

func (m *mockAdapter) GetEndpoint() string {
	return m.endpoint
}

func (m *mockAdapter) GetConfig() *ProviderConfig {
	return m.config
}

func TestNewBaseProvider(t *testing.T) {
	adapter := &mockAdapter{
		name: "test-provider",
		config: &ProviderConfig{
			Name:    "test",
			APIKey:  "test-key",
			Timeout: 30 * time.Second,
		},
	}

	log := logrus.New()
	provider := NewBaseProvider(adapter, log)

	assert.NotNil(t, provider)
	assert.Equal(t, adapter, provider.adapter)
	assert.NotNil(t, provider.httpClient)
	assert.Equal(t, log, provider.log)
}

func TestBaseProvider_Name(t *testing.T) {
	adapter := &mockAdapter{
		name: "test-provider",
		config: &ProviderConfig{
			Timeout: 30 * time.Second,
		},
	}

	provider := NewBaseProvider(adapter, logrus.New())
	assert.Equal(t, "test-provider", provider.Name())
}

func TestBaseProvider_Analyze_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "test-provider", r.Header.Get("X-Test-Provider"))

		// Send mock AI response
		w.WriteHeader(http.StatusOK)
		response := `# System Health Analysis

## Summary
System is healthy.

## Findings
- **HIGH**: CPU usage at 85%
- **MEDIUM**: Memory usage at 60%

## Recommendations
1. Monitor CPU usage
2. Consider adding more memory

## Overall Health
GOOD`
		_, _ = w.Write([]byte(fmt.Sprintf(`{"content": %q}`, response)))
	}))
	defer server.Close()

	// Create adapter
	adapter := &mockAdapter{
		name:     "test-provider",
		endpoint: server.URL,
		config: &ProviderConfig{
			Name:    "test",
			APIKey:  "test-key",
			Model:   "test-model",
			Timeout: 30 * time.Second,
		},
		usage: &TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
	}

	// Create provider
	provider := NewBaseProvider(adapter, logrus.New())

	// Execute analysis
	req := &AnalysisRequest{
		DiagnosticData: "test data",
		Target:         "localhost",
	}

	resp, err := provider.Analyze(context.Background(), req)

	// Verify
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-provider", resp.Provider)
	assert.Equal(t, "test-model", resp.Model)
	assert.Equal(t, 2, len(resp.Findings))
	assert.Equal(t, 2, len(resp.Recommendations))
	assert.Equal(t, "GOOD", string(resp.OverallHealth))
	assert.NotZero(t, resp.Duration)
	assert.Equal(t, 150, resp.TokensUsed.TotalTokens)
}

func TestBaseProvider_Analyze_BuildPromptError(t *testing.T) {
	adapter := &mockAdapter{
		name: "test-provider",
		config: &ProviderConfig{
			Timeout: 30 * time.Second,
		},
	}

	provider := NewBaseProvider(adapter, logrus.New())

	// Invalid request (empty diagnostic data will cause prompt build error)
	req := &AnalysisRequest{}

	resp, err := provider.Analyze(context.Background(), req)

	// Verify error
	require.Error(t, err)
	assert.Nil(t, resp)

	aiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, "build_prompt", aiErr.Op)
	assert.Equal(t, "test-provider", aiErr.Provider)
}

func TestBaseProvider_Analyze_BuildRequestError(t *testing.T) {
	adapter := &mockAdapter{
		name:        "test-provider",
		buildReqErr: assert.AnError,
		config: &ProviderConfig{
			Timeout: 30 * time.Second,
		},
	}

	provider := NewBaseProvider(adapter, logrus.New())

	req := &AnalysisRequest{
		DiagnosticData: "test data",
		Target:         "localhost",
	}

	resp, err := provider.Analyze(context.Background(), req)

	// Verify error
	require.Error(t, err)
	assert.Nil(t, resp)

	aiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, "build_request", aiErr.Op)
	assert.Equal(t, "test-provider", aiErr.Provider)
}

func TestBaseProvider_Analyze_HTTPError(t *testing.T) {
	// Create test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	adapter := &mockAdapter{
		name:     "test-provider",
		endpoint: server.URL,
		config: &ProviderConfig{
			APIKey:  "test-key",
			Timeout: 30 * time.Second,
		},
	}

	provider := NewBaseProvider(adapter, logrus.New())

	req := &AnalysisRequest{
		DiagnosticData: "test data",
		Target:         "localhost",
	}

	resp, err := provider.Analyze(context.Background(), req)

	// Verify error
	require.Error(t, err)
	assert.Nil(t, resp)

	aiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, "test-provider", aiErr.Provider)
	assert.True(t, aiErr.Retryable) // 500 errors are retryable
}

func TestBaseProvider_Analyze_ParseResponseError(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content": "valid json"}`))
	}))
	defer server.Close()

	adapter := &mockAdapter{
		name:         "test-provider",
		endpoint:     server.URL,
		parseRespErr: assert.AnError,
		config: &ProviderConfig{
			APIKey:  "test-key",
			Timeout: 30 * time.Second,
		},
	}

	provider := NewBaseProvider(adapter, logrus.New())

	req := &AnalysisRequest{
		DiagnosticData: "test data",
		Target:         "localhost",
	}

	resp, err := provider.Analyze(context.Background(), req)

	// Verify error
	require.Error(t, err)
	assert.Nil(t, resp)

	aiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, "parse_response", aiErr.Op)
	assert.Equal(t, "test-provider", aiErr.Provider)
}

func TestBaseProvider_Analyze_WithFocus(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body to verify focus areas
		var reqBody map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&reqBody)

		userPrompt, _ := reqBody["user"].(string)
		// Verify focus areas are in the prompt
		assert.Contains(t, userPrompt, "cpu")
		assert.Contains(t, userPrompt, "memory")

		w.WriteHeader(http.StatusOK)
		response := `# Analysis\n\n## Summary\nFocused on CPU and memory.\n\n## Overall Health\nGOOD`
		_, _ = w.Write([]byte(fmt.Sprintf(`{"content": %q}`, response)))
	}))
	defer server.Close()

	adapter := &mockAdapter{
		name:     "test-provider",
		endpoint: server.URL,
		config: &ProviderConfig{
			APIKey:  "test-key",
			Model:   "test-model",
			Timeout: 30 * time.Second,
		},
	}

	provider := NewBaseProvider(adapter, logrus.New())

	req := &AnalysisRequest{
		DiagnosticData: "test data",
		Target:         "localhost",
		Focus:          []string{"cpu", "memory"},
	}

	resp, err := provider.Analyze(context.Background(), req)

	// Verify
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestBaseProvider_AnalyzeStream(t *testing.T) {
	adapter := &mockAdapter{
		name:     "test-provider",
		endpoint: "http://example.com",
		config: &ProviderConfig{
			APIKey:  "test-key",
			Timeout: 30 * time.Second,
		},
	}

	provider := NewBaseProvider(adapter, logrus.New())

	req := &AnalysisRequest{
		DiagnosticData: "test data",
		Target:         "localhost",
	}

	ch, err := provider.AnalyzeStream(context.Background(), req)

	// Verify channel was returned
	require.NoError(t, err)
	assert.NotNil(t, ch)

	// Read from channel (should get error for now since streaming not implemented)
	chunk := <-ch
	assert.True(t, chunk.Done)
	assert.NotNil(t, chunk.Error)
	assert.Contains(t, chunk.Error.Error(), "streaming implementation pending")
}

func TestBaseProvider_Health_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content": "OK"}`))
	}))
	defer server.Close()

	adapter := &mockAdapter{
		name:     "test-provider",
		endpoint: server.URL,
		config: &ProviderConfig{
			APIKey:  "test-key",
			Timeout: 30 * time.Second,
		},
	}

	provider := NewBaseProvider(adapter, logrus.New())

	err := provider.Health(context.Background())

	// Verify
	assert.NoError(t, err)
}

func TestBaseProvider_Health_EmptyResponse(t *testing.T) {
	// Create test server that returns empty body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Empty body
	}))
	defer server.Close()

	adapter := &mockAdapter{
		name:     "test-provider",
		endpoint: server.URL,
		config: &ProviderConfig{
			APIKey:  "test-key",
			Timeout: 30 * time.Second,
		},
	}

	provider := NewBaseProvider(adapter, logrus.New())

	err := provider.Health(context.Background())

	// Verify error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response body")
}

func TestBaseProvider_Health_HTTPError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error": "service unavailable"}`))
	}))
	defer server.Close()

	adapter := &mockAdapter{
		name:     "test-provider",
		endpoint: server.URL,
		config: &ProviderConfig{
			APIKey:  "test-key",
			Timeout: 30 * time.Second,
		},
	}

	provider := NewBaseProvider(adapter, logrus.New())

	err := provider.Health(context.Background())

	// Verify error
	require.Error(t, err)

	aiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, "health_check", aiErr.Op)
	assert.True(t, aiErr.Retryable)
}
