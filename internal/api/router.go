package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ignacio/lumo/internal/api/handlers"
	apimiddleware "github.com/ignacio/lumo/internal/api/middleware"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database"
	"github.com/ignacio/lumo/internal/database/repository"
	"github.com/sirupsen/logrus"
)

// NewRouter creates and configures the HTTP router
func NewRouter(db *database.DB, cfg *config.Config, logger *logrus.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(apimiddleware.Recovery(logger))
	r.Use(apimiddleware.Logger(logger))
	r.Use(apimiddleware.CORS())
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Compress(5))

	// Initialize repositories
	jobRepo := repository.NewJobRepository(db.DB)
	apiKeyRepo := repository.NewAPIKeyRepository(db.DB)
	agentRepo := repository.NewAgentRepository(db.DB)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db, logger)
	diagnosticsHandler := handlers.NewDiagnosticsHandler(jobRepo, cfg, logger)
	jobsHandler := handlers.NewJobsHandler(jobRepo, logger)
	agentsHandler := handlers.NewAgentsHandler(agentRepo, logger)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public endpoints (no authentication required)
		r.Get("/health", healthHandler.Health)
		r.Get("/ready", healthHandler.Ready)
		r.Get("/live", healthHandler.Live)

		// Authenticated endpoints (require API key)
		r.Group(func(r chi.Router) {
			r.Use(apimiddleware.APIKeyAuth(apiKeyRepo, logger))

			// Diagnostic endpoints
			r.Post("/diagnostics", diagnosticsHandler.Run)

			// Job endpoints
			r.Get("/jobs", jobsHandler.List)
			r.Get("/jobs/{id}", jobsHandler.Get)
			r.Delete("/jobs/{id}", jobsHandler.Delete)

			// Agent endpoints
			r.Post("/agents/register", agentsHandler.Register)
			r.Put("/agents/{id}/heartbeat", agentsHandler.Heartbeat)
			r.Get("/agents", agentsHandler.List)
			r.Get("/agents/stats", agentsHandler.Stats)
			r.Get("/agents/{id}", agentsHandler.Get)
			r.Delete("/agents/{id}", agentsHandler.Delete)

			// TODO: Remediation endpoints (Phase 7 completion)
			// r.Post("/remediation", remediationHandler.Run)
		})
	})

	return r
}
