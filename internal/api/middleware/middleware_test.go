package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/ignacio/lumo/internal/database/models"
)

// testContextKey is a custom type for test context keys to avoid collisions
type testContextKey string

const userIDContextKey testContextKey = "user_id"

// Test Rate Limiter

func TestNewRateLimiter(t *testing.T) {
	tests := []struct {
		name            string
		enabled         bool
		requestsPerMin  int
		requestsPerHour int
		burstSize       int
		logger          *logrus.Logger
	}{
		{
			name:            "enabled with logger",
			enabled:         true,
			requestsPerMin:  60,
			requestsPerHour: 3600,
			burstSize:       10,
			logger:          logrus.New(),
		},
		{
			name:            "disabled with logger",
			enabled:         false,
			requestsPerMin:  60,
			requestsPerHour: 3600,
			burstSize:       10,
			logger:          logrus.New(),
		},
		{
			name:            "enabled with nil logger",
			enabled:         true,
			requestsPerMin:  60,
			requestsPerHour: 3600,
			burstSize:       10,
			logger:          nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.enabled, tt.requestsPerMin, tt.requestsPerHour, tt.burstSize, tt.logger)

			assert.NotNil(t, rl)
			assert.Equal(t, tt.enabled, rl.enabled)
			assert.Equal(t, tt.requestsPerMin, rl.requestsPerMin)
			assert.Equal(t, tt.requestsPerHour, rl.requestsPerHour)
			assert.Equal(t, tt.burstSize, rl.burstSize)
			assert.NotNil(t, rl.logger)

			if tt.enabled {
				assert.NotNil(t, rl.perIPLimiter, "perIPLimiter should be set when enabled")
			}
		})
	}
}

func TestRateLimiter_PerIPMiddleware(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{
			name:     "enabled returns limiting middleware",
			enabled:  true,
			expected: true,
		},
		{
			name:     "disabled returns passthrough",
			enabled:  false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.enabled, 60, 3600, 10, logrus.New())
			middleware := rl.PerIPMiddleware()

			assert.NotNil(t, middleware)

			// Test that middleware can wrap a handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			wrapped := middleware(handler)
			assert.NotNil(t, wrapped)

			// Execute wrapped handler
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, req)

			// Should pass through on first request
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestRateLimiter_PerUserMiddleware(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		setupCtx func(*http.Request) *http.Request
	}{
		{
			name:    "enabled with API key in context",
			enabled: true,
			setupCtx: func(r *http.Request) *http.Request {
				apiKey := &models.APIKey{
					ID:   uuid.New(),
					Name: "test-key",
				}
				ctx := SetAPIKeyInContext(r.Context(), apiKey)
				return r.WithContext(ctx)
			},
		},
		{
			name:    "enabled with user ID in context",
			enabled: true,
			setupCtx: func(r *http.Request) *http.Request {
				ctx := context.WithValue(r.Context(), userIDContextKey, "test-user-123")
				return r.WithContext(ctx)
			},
		},
		{
			name:    "enabled without user context (fallback to IP)",
			enabled: true,
			setupCtx: func(r *http.Request) *http.Request {
				return r
			},
		},
		{
			name:    "disabled returns passthrough",
			enabled: false,
			setupCtx: func(r *http.Request) *http.Request {
				return r
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.enabled, 60, 100, 10, logrus.New())
			middleware := rl.PerUserMiddleware()

			assert.NotNil(t, middleware)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			wrapped := middleware(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			req = tt.setupCtx(req)

			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, req)

			// Should pass through on first request
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// Test API Key Auth

func TestExtractAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected string
	}{
		{
			name: "X-API-Key header",
			headers: map[string]string{
				"X-API-Key": "test-api-key-123",
			},
			expected: "test-api-key-123",
		},
		{
			name: "Authorization Bearer header",
			headers: map[string]string{
				"Authorization": "Bearer test-api-key-456",
			},
			expected: "test-api-key-456",
		},
		{
			name: "Authorization Bearer with extra spaces",
			headers: map[string]string{
				"Authorization": "Bearer   test-api-key-789  ",
			},
			expected: "test-api-key-789",
		},
		{
			name: "X-API-Key takes precedence",
			headers: map[string]string{
				"X-API-Key":     "priority-key",
				"Authorization": "Bearer fallback-key",
			},
			expected: "priority-key",
		},
		{
			name: "Authorization with wrong scheme",
			headers: map[string]string{
				"Authorization": "Basic dGVzdDp0ZXN0",
			},
			expected: "",
		},
		{
			name:     "no headers",
			headers:  map[string]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := extractAPIKey(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAPIKeyFromContext(t *testing.T) {
	t.Run("key exists in context", func(t *testing.T) {
		expectedKey := &models.APIKey{
			ID:   uuid.New(),
			Name: "test-key",
		}

		ctx := SetAPIKeyInContext(context.Background(), expectedKey)
		key, ok := GetAPIKeyFromContext(ctx)

		assert.True(t, ok)
		assert.Equal(t, expectedKey, key)
	})

	t.Run("key does not exist in context", func(t *testing.T) {
		ctx := context.Background()
		key, ok := GetAPIKeyFromContext(ctx)

		assert.False(t, ok)
		assert.Nil(t, key)
	})

	t.Run("wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), APIKeyContextKey, "not-an-api-key")
		key, ok := GetAPIKeyFromContext(ctx)

		assert.False(t, ok)
		assert.Nil(t, key)
	})
}

func TestSetAPIKeyInContext(t *testing.T) {
	t.Run("set and retrieve key", func(t *testing.T) {
		apiKey := &models.APIKey{
			ID:   uuid.New(),
			Name: "test-key",
		}

		ctx := SetAPIKeyInContext(context.Background(), apiKey)
		retrieved, ok := GetAPIKeyFromContext(ctx)

		assert.True(t, ok)
		assert.Equal(t, apiKey, retrieved)
	})

	t.Run("set nil key", func(t *testing.T) {
		ctx := SetAPIKeyInContext(context.Background(), nil)
		retrieved, ok := GetAPIKeyFromContext(ctx)

		assert.True(t, ok)
		assert.Nil(t, retrieved)
	})
}

// Note: Full APIKeyAuth middleware tests require database integration
// and are covered in tests/integration/. Here we test the helper functions
// that can be tested in isolation.

func TestRequireScope(t *testing.T) {
	tests := []struct {
		name               string
		requiredScope      string
		apiKey             *models.APIKey
		expectedStatusCode int
		expectNext         bool
	}{
		{
			name:          "has required scope",
			requiredScope: "read",
			apiKey: &models.APIKey{
				ID:     uuid.New(),
				Name:   "test-key",
				Scopes: []string{"read", "write"},
			},
			expectedStatusCode: http.StatusOK,
			expectNext:         true,
		},
		{
			name:          "does not have required scope",
			requiredScope: "admin",
			apiKey: &models.APIKey{
				ID:     uuid.New(),
				Name:   "test-key",
				Scopes: []string{"read", "write"},
			},
			expectedStatusCode: http.StatusForbidden,
			expectNext:         false,
		},
		{
			name:               "no API key in context",
			requiredScope:      "read",
			apiKey:             nil,
			expectedStatusCode: http.StatusUnauthorized,
			expectNext:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := RequireScope(tt.requiredScope)

			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(next)

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.apiKey != nil {
				ctx := SetAPIKeyInContext(req.Context(), tt.apiKey)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatusCode, w.Code)
			assert.Equal(t, tt.expectNext, nextCalled)
		})
	}
}

// Test CORS

func TestCORS(t *testing.T) {
	t.Run("returns middleware with custom origins", func(t *testing.T) {
		corsMiddleware := CORS([]string{"https://example.com"})
		assert.NotNil(t, corsMiddleware)
	})

	t.Run("returns middleware with default origins when empty", func(t *testing.T) {
		corsMiddleware := CORS([]string{})
		assert.NotNil(t, corsMiddleware)
	})

	t.Run("returns middleware with default origins when nil", func(t *testing.T) {
		corsMiddleware := CORS(nil)
		assert.NotNil(t, corsMiddleware)
	})
}

// Note: Full CORS behavior tests require testing the go-chi/cors library
// and are covered in integration tests. Here we verify that the middleware
// is properly created with different configurations.
