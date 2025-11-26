package middleware

import (
	"net/http"
)

const (
	// DefaultMaxBodySize is the default maximum request body size (5 MB)
	DefaultMaxBodySize = 5 * 1024 * 1024 // 5 MB
)

// RequestSizeLimit returns a middleware that limits the size of request bodies.
// This helps prevent DoS attacks via large JSON payloads.
//
// maxBytes specifies the maximum allowed request body size in bytes.
// If 0, DefaultMaxBodySize (5 MB) is used.
func RequestSizeLimit(maxBytes int64) func(next http.Handler) http.Handler {
	if maxBytes == 0 {
		maxBytes = DefaultMaxBodySize
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only limit POST, PUT, PATCH requests (methods with bodies)
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
