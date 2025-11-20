package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// HTTPClient handles common HTTP operations for AI providers.
// It centralizes request/response handling, logging, and error management.
type HTTPClient struct {
	client *http.Client
	log    *logrus.Logger
}

// NewHTTPClient creates a new HTTP client with the specified timeout.
func NewHTTPClient(timeout time.Duration, log *logrus.Logger) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
		log: log,
	}
}

// RequestOptions configures an HTTP request.
type RequestOptions struct {
	// Method is the HTTP method (GET, POST, etc.)
	Method string

	// URL is the full request URL
	URL string

	// Body is the request body (will be JSON-marshaled if not nil)
	Body interface{}

	// Headers to set on the request
	Headers map[string]string

	// ExpectedStatus is the expected successful status code (default: 200)
	ExpectedStatus int

	// ProviderName for error reporting
	ProviderName string
}

// Response contains the parsed HTTP response.
type Response struct {
	// StatusCode is the HTTP status code
	StatusCode int

	// Headers contains the response headers
	Headers http.Header

	// Body contains the raw response body
	Body []byte

	// Duration is the time taken for the request
	Duration time.Duration
}

// Do executes an HTTP request with common error handling and logging.
func (c *HTTPClient) Do(ctx context.Context, opts RequestOptions) (*Response, error) {
	start := time.Now()

	// Set default expected status
	if opts.ExpectedStatus == 0 {
		opts.ExpectedStatus = http.StatusOK
	}

	// Marshal body if provided
	var bodyReader io.Reader
	if opts.Body != nil {
		bodyBytes, err := json.Marshal(opts.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)

		c.log.WithFields(logrus.Fields{
			"url":         opts.URL,
			"method":      opts.Method,
			"body_length": len(bodyBytes),
		}).Debug("Marshaled request body")
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, opts.Method, opts.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	c.log.WithFields(logrus.Fields{
		"url":    opts.URL,
		"method": opts.Method,
	}).Debug("Sending HTTP request")

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &Error{
			Op:        "http_request",
			Provider:  opts.ProviderName,
			Err:       err,
			Retryable: true, // Network errors are retryable
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	duration := time.Since(start)

	c.log.WithFields(logrus.Fields{
		"status_code":  resp.StatusCode,
		"content_type": resp.Header.Get("Content-Type"),
		"body_length":  len(body),
		"duration_ms":  duration.Milliseconds(),
	}).Debug("Received HTTP response")

	// Check status code
	if resp.StatusCode != opts.ExpectedStatus {
		c.log.WithFields(logrus.Fields{
			"status_code": resp.StatusCode,
			"body":        truncateString(string(body), 500),
		}).Error("HTTP request returned unexpected status code")

		return nil, &Error{
			Op:        "http_request",
			Provider:  opts.ProviderName,
			Err:       fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body)),
			Retryable: resp.StatusCode >= 500 || resp.StatusCode == 429, // 5xx and rate limits are retryable
		}
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
		Duration:   duration,
	}, nil
}

// DoStreaming executes an HTTP request for streaming responses.
// Returns the raw http.Response for the caller to process the stream.
// The caller is responsible for closing the response body.
func (c *HTTPClient) DoStreaming(ctx context.Context, opts RequestOptions) (*http.Response, error) {
	// Marshal body if provided
	var bodyReader io.Reader
	if opts.Body != nil {
		bodyBytes, err := json.Marshal(opts.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, opts.Method, opts.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	c.log.WithFields(logrus.Fields{
		"url":    opts.URL,
		"method": opts.Method,
	}).Debug("Sending streaming HTTP request")

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &Error{
			Op:        "http_streaming_request",
			Provider:  opts.ProviderName,
			Err:       err,
			Retryable: true,
		}
	}

	// Set default expected status
	expectedStatus := opts.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = http.StatusOK
	}

	// Check status code (for streaming, we check before processing the stream)
	if resp.StatusCode != expectedStatus {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		c.log.WithFields(logrus.Fields{
			"status_code": resp.StatusCode,
			"body":        string(body),
		}).Error("Streaming HTTP request returned unexpected status code")

		return nil, &Error{
			Op:        "http_streaming_request",
			Provider:  opts.ProviderName,
			Err:       fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body)),
			Retryable: resp.StatusCode >= 500 || resp.StatusCode == 429,
		}
	}

	return resp, nil
}

// UnmarshalResponse is a helper to unmarshal a response body into a target struct.
func UnmarshalResponse(body []byte, target interface{}) error {
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w (body preview: %s)",
			err, truncateString(string(body), 200))
	}
	return nil
}
