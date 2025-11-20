package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPClient(t *testing.T) {
	log := logrus.New()
	timeout := 30 * time.Second

	client := NewHTTPClient(timeout, log)

	assert.NotNil(t, client)
	assert.NotNil(t, client.client)
	assert.Equal(t, timeout, client.client.Timeout)
	assert.Equal(t, log, client.log)
}

func TestHTTPClient_Do_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-value", r.Header.Get("X-Test-Header"))

		// Read and verify body
		body, _ := io.ReadAll(r.Body)
		var reqData map[string]string
		_ = json.Unmarshal(body, &reqData)
		assert.Equal(t, "bar", reqData["foo"])

		// Send response
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result": "success"}`))
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(30*time.Second, logrus.New())

	// Execute request
	resp, err := client.Do(context.Background(), RequestOptions{
		Method: "POST",
		URL:    server.URL,
		Body: map[string]string{
			"foo": "bar",
		},
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"X-Test-Header": "test-value",
		},
		ProviderName: "test",
	})

	// Verify
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"result": "success"}`, string(resp.Body))
	assert.GreaterOrEqual(t, resp.Duration.Nanoseconds(), int64(0))
}

func TestHTTPClient_Do_WithoutBody(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(30*time.Second, logrus.New())

	// Execute request
	resp, err := client.Do(context.Background(), RequestOptions{
		Method:       "GET",
		URL:          server.URL,
		ProviderName: "test",
	})

	// Verify
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"status": "ok"}`, string(resp.Body))
}

func TestHTTPClient_Do_CustomExpectedStatus(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"created": true}`))
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(30*time.Second, logrus.New())

	// Execute request with custom expected status
	resp, err := client.Do(context.Background(), RequestOptions{
		Method:         "POST",
		URL:            server.URL,
		Body:           map[string]string{"test": "data"},
		ExpectedStatus: http.StatusCreated,
		ProviderName:   "test",
	})

	// Verify
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestHTTPClient_Do_UnexpectedStatus(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(30*time.Second, logrus.New())

	// Execute request
	resp, err := client.Do(context.Background(), RequestOptions{
		Method:       "POST",
		URL:          server.URL,
		Body:         map[string]string{"test": "data"},
		ProviderName: "test",
	})

	// Verify error
	require.Error(t, err)
	assert.Nil(t, resp)

	// Check error details
	aiErr, ok := err.(*Error)
	require.True(t, ok, "expected *Error type")
	assert.Equal(t, "test", aiErr.Provider)
	assert.Equal(t, "http_request", aiErr.Op)
	assert.False(t, aiErr.Retryable, "4xx errors should not be retryable")
	assert.Contains(t, aiErr.Error(), "unexpected status code 400")
}

func TestHTTPClient_Do_ServerError_Retryable(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(30*time.Second, logrus.New())

	// Execute request
	resp, err := client.Do(context.Background(), RequestOptions{
		Method:       "POST",
		URL:          server.URL,
		Body:         map[string]string{"test": "data"},
		ProviderName: "test",
	})

	// Verify error
	require.Error(t, err)
	assert.Nil(t, resp)

	// Check error is retryable
	aiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.True(t, aiErr.Retryable, "5xx errors should be retryable")
	assert.Contains(t, aiErr.Error(), "unexpected status code 500")
}

func TestHTTPClient_Do_RateLimit_Retryable(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": "rate limit exceeded"}`))
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(30*time.Second, logrus.New())

	// Execute request
	resp, err := client.Do(context.Background(), RequestOptions{
		Method:       "POST",
		URL:          server.URL,
		Body:         map[string]string{"test": "data"},
		ProviderName: "test",
	})

	// Verify error
	require.Error(t, err)
	assert.Nil(t, resp)

	// Check error is retryable
	aiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.True(t, aiErr.Retryable, "429 rate limit errors should be retryable")
}

func TestHTTPClient_Do_ContextCanceled(t *testing.T) {
	// Create test server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(30*time.Second, logrus.New())

	// Create context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Execute request
	resp, err := client.Do(ctx, RequestOptions{
		Method:       "POST",
		URL:          server.URL,
		Body:         map[string]string{"test": "data"},
		ProviderName: "test",
	})

	// Verify error
	require.Error(t, err)
	assert.Nil(t, resp)

	// Check error is retryable
	aiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.True(t, aiErr.Retryable, "context errors should be retryable")
}

func TestHTTPClient_DoStreaming_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

		// Send streaming response
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		_, _ = w.Write([]byte("data: chunk1\n\n"))
		flusher.Flush()

		_, _ = w.Write([]byte("data: chunk2\n\n"))
		flusher.Flush()

		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(30*time.Second, logrus.New())

	// Execute streaming request
	resp, err := client.DoStreaming(context.Background(), RequestOptions{
		Method: "POST",
		URL:    server.URL,
		Body: map[string]string{
			"stream": "true",
		},
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		ProviderName: "test",
	})

	// Verify
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Read stream
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Contains(t, string(body), "chunk1")
	assert.Contains(t, string(body), "chunk2")
}

func TestHTTPClient_DoStreaming_UnexpectedStatus(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	// Create client
	client := NewHTTPClient(30*time.Second, logrus.New())

	// Execute streaming request
	resp, err := client.DoStreaming(context.Background(), RequestOptions{
		Method:       "POST",
		URL:          server.URL,
		Body:         map[string]string{"test": "data"},
		ProviderName: "test",
	})

	// Verify error
	require.Error(t, err)
	assert.Nil(t, resp)

	// Check error details
	aiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, "test", aiErr.Provider)
	assert.Equal(t, "http_streaming_request", aiErr.Op)
	assert.Contains(t, aiErr.Error(), "unexpected status code 400")
}

func TestHTTPClient_UnmarshalResponse_Success(t *testing.T) {
	type TestResponse struct {
		Message string `json:"message"`
		Count   int    `json:"count"`
	}

	body := []byte(`{"message": "hello", "count": 42}`)

	var result TestResponse
	err := UnmarshalResponse(body, &result)

	require.NoError(t, err)
	assert.Equal(t, "hello", result.Message)
	assert.Equal(t, 42, result.Count)
}

func TestHTTPClient_UnmarshalResponse_InvalidJSON(t *testing.T) {
	body := []byte(`{"invalid json`)

	var result map[string]interface{}
	err := UnmarshalResponse(body, &result)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal response")
}

func TestHTTPClient_Do_MarshalError(t *testing.T) {
	// Create client
	client := NewHTTPClient(30*time.Second, logrus.New())

	// Try to marshal an unmarshalable value (channel)
	ch := make(chan int)
	resp, err := client.Do(context.Background(), RequestOptions{
		Method:       "POST",
		URL:          "http://example.com",
		Body:         ch, // Channels can't be marshaled to JSON
		ProviderName: "test",
	})

	// Verify error
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to marshal request body")
}
