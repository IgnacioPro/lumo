package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
)

// Logger is a middleware that logs HTTP requests
func Logger(logger *logrus.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			start := time.Now()

			defer func() {
				logger.WithFields(logrus.Fields{
					"method":     r.Method,
					"path":       r.URL.Path,
					"query":      r.URL.RawQuery,
					"status":     ww.Status(),
					"bytes":      ww.BytesWritten(),
					"duration":   time.Since(start).Milliseconds(),
					"remote_addr": r.RemoteAddr,
					"user_agent": r.UserAgent(),
				}).Info("HTTP request")
			}()

			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}
