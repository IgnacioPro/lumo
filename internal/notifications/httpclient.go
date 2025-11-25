package notifications

import (
	"net/http"
	"time"
)

// DefaultHTTPTimeout is the default timeout for HTTP requests.
const DefaultHTTPTimeout = 30 * time.Second

// NewHTTPClient creates an HTTP client with the specified timeout.
// If timeout is 0, DefaultHTTPTimeout is used.
func NewHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultHTTPTimeout
	}
	return &http.Client{
		Timeout: timeout,
	}
}

// NewHTTPClientFromSeconds creates an HTTP client from a timeout value in seconds.
// If timeoutSeconds is 0, DefaultHTTPTimeout is used.
func NewHTTPClientFromSeconds(timeoutSeconds int) *http.Client {
	if timeoutSeconds > 0 {
		return NewHTTPClient(time.Duration(timeoutSeconds) * time.Second)
	}
	return NewHTTPClient(DefaultHTTPTimeout)
}
