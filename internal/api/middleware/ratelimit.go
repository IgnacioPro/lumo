package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"
	"github.com/sirupsen/logrus"
)

// RateLimiter provides rate limiting middleware for API endpoints
type RateLimiter struct {
	enabled         bool
	requestsPerMin  int
	requestsPerHour int
	burstSize       int
	logger          *logrus.Logger
	perIPLimiter    func(http.Handler) http.Handler
}

// NewRateLimiter creates a new rate limiter middleware
func NewRateLimiter(enabled bool, requestsPerMin, requestsPerHour, burstSize int, logger *logrus.Logger) *RateLimiter {
	if logger == nil {
		logger = logrus.New()
	}

	rl := &RateLimiter{
		enabled:         enabled,
		requestsPerMin:  requestsPerMin,
		requestsPerHour: requestsPerHour,
		burstSize:       burstSize,
		logger:          logger,
	}

	if enabled {
		// Per-IP rate limiter (for all requests)
		// Uses sliding window counter with IP address as key
		rl.perIPLimiter = httprate.Limit(
			requestsPerMin,                          // Max requests per window
			time.Minute,                             // Window duration
			httprate.WithKeyFuncs(httprate.KeyByIP), // Key by IP address
			httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
				logger.WithFields(logrus.Fields{
					"ip":     r.RemoteAddr,
					"method": r.Method,
					"path":   r.URL.Path,
					"limit":  requestsPerMin,
				}).Warn("Rate limit exceeded (per-IP)")

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-RateLimit-Limit", "60")
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"Rate limit exceeded. Please try again later.","retry_after_seconds":60}`))
			}),
		)

		logger.WithFields(logrus.Fields{
			"requests_per_min":  requestsPerMin,
			"requests_per_hour": requestsPerHour,
			"burst_size":        burstSize,
		}).Info("Rate limiting enabled")
	} else {
		logger.Info("Rate limiting disabled")
	}

	return rl
}

// PerIPMiddleware returns a middleware that enforces per-IP rate limiting
func (rl *RateLimiter) PerIPMiddleware() func(http.Handler) http.Handler {
	if !rl.enabled || rl.perIPLimiter == nil {
		// Passthrough if disabled
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return rl.perIPLimiter
}

// PerUserMiddleware returns a middleware that enforces per-user rate limiting
// for authenticated endpoints (by API key or JWT user ID)
func (rl *RateLimiter) PerUserMiddleware() func(http.Handler) http.Handler {
	if !rl.enabled {
		// Passthrough if disabled
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// Per-user rate limiter (for authenticated requests)
	// Uses API key or JWT user ID as key
	return httprate.Limit(
		rl.requestsPerHour, // Max requests per hour
		time.Hour,          // Window duration
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			// Try to extract user identifier from context
			// Priority: API key > JWT user ID > IP address (fallback)

			// Check for API key in context (set by APIKeyAuth middleware)
			if apiKey, ok := GetAPIKeyFromContext(r.Context()); ok && apiKey != nil {
				return "apikey:" + apiKey.ID.String(), nil
			}

			// Check for JWT user ID in context (set by JWT middleware)
			if userID := r.Context().Value("user_id"); userID != nil {
				if id, ok := userID.(string); ok && id != "" {
					return "user:" + id, nil
				}
			}

			// Fallback to IP address if no user identifier found
			// This shouldn't happen for authenticated endpoints, but provides safety
			ip, _ := httprate.KeyByIP(r)
			return "ip:" + ip, nil
		}),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			userKey := "unknown"
			if apiKey, ok := GetAPIKeyFromContext(r.Context()); ok && apiKey != nil {
				// Only log first 8 chars of ID for security
				keyID := apiKey.ID.String()
				if len(keyID) > 8 {
					userKey = keyID[:8] + "..."
				} else {
					userKey = keyID
				}
			}

			rl.logger.WithFields(logrus.Fields{
				"user_key": userKey,
				"method":   r.Method,
				"path":     r.URL.Path,
				"limit":    rl.requestsPerHour,
			}).Warn("Rate limit exceeded (per-user)")

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", "3600")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"Rate limit exceeded. Please try again in 1 hour.","retry_after_seconds":3600}`))
		}),
	)
}
