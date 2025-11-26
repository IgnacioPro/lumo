package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// APIKeyContextKey is the context key for the authenticated API key
	APIKeyContextKey contextKey = "api_key"
)

// APIKeyAuth is a middleware that validates API keys
func APIKeyAuth(apiKeyRepo *repository.APIKeyRepository, logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from header
			apiKey := extractAPIKey(r)
			if apiKey == "" {
				logger.Warn("API key missing in request")
				response.Unauthorized(w, "API key required")
				return
			}

			// Validate the API key
			key, err := apiKeyRepo.ValidateAndGet(r.Context(), apiKey)
			if err != nil {
				logger.WithError(err).Warn("Invalid API key")
				response.Unauthorized(w, "Invalid or expired API key")
				return
			}

			// Check if key is valid (not revoked, not expired)
			if !key.IsValid() {
				logger.WithField("key_id", key.ID).Warn("Revoked or expired API key")
				response.Unauthorized(w, "API key is revoked or expired")
				return
			}

			// Add API key to request context
			ctx := context.WithValue(r.Context(), APIKeyContextKey, key)

			// Log successful authentication
			logger.WithFields(logrus.Fields{
				"key_id":   key.ID,
				"key_name": key.Name,
				"path":     r.URL.Path,
			}).Debug("API key authenticated")

			// Call next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope is a middleware that checks if the API key has a specific scope
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get API key from context
			key, ok := r.Context().Value(APIKeyContextKey).(*models.APIKey)
			if !ok || key == nil {
				response.Unauthorized(w, "Authentication required")
				return
			}

			// Check if key has the required scope
			if !key.HasScope(scope) {
				response.Forbidden(w, "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractAPIKey extracts the API key from the request
// Supports both "X-API-Key" header and "Authorization: Bearer" header
func extractAPIKey(r *http.Request) string {
	// Try X-API-Key header first
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}

	// Try Authorization header with Bearer scheme
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Format: "Bearer <api-key>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return strings.TrimSpace(parts[1])
		}
	}

	return ""
}

// GetAPIKeyFromContext retrieves the authenticated API key from the request context
func GetAPIKeyFromContext(ctx context.Context) (*models.APIKey, bool) {
	key, ok := ctx.Value(APIKeyContextKey).(*models.APIKey)
	return key, ok
}

// SetAPIKeyInContext sets the API key in the request context (used for testing)
func SetAPIKeyInContext(ctx context.Context, key *models.APIKey) context.Context {
	return context.WithValue(ctx, APIKeyContextKey, key)
}
