package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ignacio/lumo/internal/api/auth"
	"github.com/ignacio/lumo/internal/api/response"
	"github.com/sirupsen/logrus"
)

// jwtContextKey is the context key for JWT claims
type jwtContextKey string

const (
	// JWTClaimsKey is the context key for storing JWT claims
	JWTClaimsKey jwtContextKey = "jwt_claims"
)

// JWTAuth is middleware that validates JWT tokens
func JWTAuth(jwtManager *auth.JWTManager, logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Debug("Missing Authorization header")
				response.Unauthorized(w, "Missing authorization token")
				return
			}

			// Check for Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				logger.Debug("Invalid Authorization header format")
				response.Unauthorized(w, "Invalid authorization header format")
				return
			}

			tokenString := parts[1]

			// Validate token
			claims, err := jwtManager.ValidateToken(tokenString)
			if err != nil {
				logger.WithError(err).Debug("Invalid JWT token")
				response.Unauthorized(w, "Invalid or expired token")
				return
			}

			// Add claims to request context
			ctx := SetJWTClaimsInContext(r.Context(), claims)
			r = r.WithContext(ctx)

			logger.WithFields(logrus.Fields{
				"user_id":  claims.UserID,
				"username": claims.Username,
				"scopes":   claims.Scopes,
			}).Debug("JWT authentication successful")

			next.ServeHTTP(w, r)
		})
	}
}

// OptionalJWTAuth is middleware that validates JWT tokens but allows requests without them
// Useful for endpoints that support both authenticated and unauthenticated access
func OptionalJWTAuth(jwtManager *auth.JWTManager, logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// No token provided, continue without authentication
				next.ServeHTTP(w, r)
				return
			}

			// Parse token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				// Invalid format but continue anyway
				next.ServeHTTP(w, r)
				return
			}

			tokenString := parts[1]

			// Validate token
			claims, err := jwtManager.ValidateToken(tokenString)
			if err != nil {
				// Invalid token but continue anyway
				logger.WithError(err).Debug("Invalid JWT token (optional auth)")
				next.ServeHTTP(w, r)
				return
			}

			// Add claims to request context
			ctx := SetJWTClaimsInContext(r.Context(), claims)
			r = r.WithContext(ctx)

			logger.WithFields(logrus.Fields{
				"user_id":  claims.UserID,
				"username": claims.Username,
			}).Debug("Optional JWT authentication successful")

			next.ServeHTTP(w, r)
		})
	}
}

// RequireScope is middleware that checks if the JWT claims include a required scope
func RequireScope(requiredScope string, logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetJWTClaimsFromContext(r.Context())
			if !ok {
				logger.Debug("No JWT claims in context")
				response.Forbidden(w, "Access denied")
				return
			}

			// Check if required scope is present
			hasScope := false
			for _, scope := range claims.Scopes {
				if scope == requiredScope {
					hasScope = true
					break
				}
			}

			if !hasScope {
				logger.WithFields(logrus.Fields{
					"user_id":        claims.UserID,
					"required_scope": requiredScope,
					"user_scopes":    claims.Scopes,
				}).Debug("User missing required scope")
				response.Forbidden(w, "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SetJWTClaimsInContext adds JWT claims to the request context
func SetJWTClaimsInContext(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, JWTClaimsKey, claims)
}

// GetJWTClaimsFromContext retrieves JWT claims from the request context
func GetJWTClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(JWTClaimsKey).(*auth.Claims)
	return claims, ok
}
