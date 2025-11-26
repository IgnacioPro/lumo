package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/api/response"
)

// Recovery is a middleware that recovers from panics and returns a 500 error
func Recovery(logger *logrus.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Log the panic with stack trace
					logger.WithFields(logrus.Fields{
						"error":      err,
						"method":     r.Method,
						"path":       r.URL.Path,
						"stacktrace": string(debug.Stack()),
					}).Error("Panic recovered")

					// Return 500 error to client
					response.InternalServerError(w, fmt.Sprintf("Internal server error: %v", err))
				}
			}()

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
