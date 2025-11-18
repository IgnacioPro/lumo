package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ignacio/lumo/internal/api/handlers"
	apimiddleware "github.com/ignacio/lumo/internal/api/middleware"
	"github.com/ignacio/lumo/internal/database"
	"github.com/sirupsen/logrus"
)

// NewRouter creates and configures the HTTP router
func NewRouter(db *database.DB, logger *logrus.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(apimiddleware.Recovery(logger))
	r.Use(apimiddleware.Logger(logger))
	r.Use(apimiddleware.CORS())
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Compress(5))

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db, logger)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public endpoints (no authentication required)
		r.Get("/health", healthHandler.Health)
		r.Get("/ready", healthHandler.Ready)
		r.Get("/live", healthHandler.Live)

		// Authenticated endpoints (require API key)
		// TODO: Add API key authentication middleware
		// r.Group(func(r chi.Router) {
		// 	r.Use(apimiddleware.APIKeyAuth(db, logger))
		//
		// 	// Diagnostic endpoints
		// 	r.Post("/diagnostics", diagnosticsHandler.Run)
		//
		// 	// Remediation endpoints
		// 	r.Post("/remediation", remediationHandler.Run)
		//
		// 	// Job endpoints
		// 	r.Get("/jobs", jobsHandler.List)
		// 	r.Get("/jobs/{id}", jobsHandler.Get)
		//
		// 	// Status endpoint
		// 	r.Get("/status", statusHandler.Status)
		// })
	})

	return r
}
