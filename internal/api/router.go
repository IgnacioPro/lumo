package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ignacio/lumo/internal/api/auth"
	"github.com/ignacio/lumo/internal/api/handlers"
	apimiddleware "github.com/ignacio/lumo/internal/api/middleware"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database"
	"github.com/ignacio/lumo/internal/database/repository"
	"github.com/sirupsen/logrus"
)

// NewRouter creates and configures the HTTP router
func NewRouter(db *database.DB, cfg *config.Config, jwtManager *auth.JWTManager, logger *logrus.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Initialize rate limiter
	rateLimiter := apimiddleware.NewRateLimiter(
		cfg.API.RateLimitEnabled,
		cfg.API.RateLimitRequestsPerMin,
		cfg.API.RateLimitRequestsPerHour,
		cfg.API.RateLimitBurstSize,
		logger,
	)

	// Global middleware
	r.Use(apimiddleware.Recovery(logger))
	r.Use(apimiddleware.Logger(logger))
	r.Use(apimiddleware.CORS())
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// Per-IP rate limiting (applied to all requests)
	r.Use(rateLimiter.PerIPMiddleware())

	r.Use(middleware.Compress(5))

	// Initialize repositories
	jobRepo := repository.NewJobRepository(db.DB)
	apiKeyRepo := repository.NewAPIKeyRepository(db.DB)
	agentRepo := repository.NewAgentRepository(db.DB)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db, logger)
	authHandler := handlers.NewAuthHandler(apiKeyRepo, jwtManager, logger)
	diagnosticsHandler := handlers.NewDiagnosticsHandler(jobRepo, cfg, logger)
	remediationHandler := handlers.NewRemediationHandler(jobRepo, cfg, logger)
	jobsHandler := handlers.NewJobsHandler(jobRepo, logger)
	agentsHandler := handlers.NewAgentsHandler(agentRepo, logger)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public endpoints (no authentication required)
		r.Get("/health", healthHandler.Health)
		r.Get("/ready", healthHandler.Ready)
		r.Get("/live", healthHandler.Live)

		// Auth endpoints (public - used to obtain JWT tokens)
		r.Post("/auth/token", authHandler.GenerateToken)

		// JWT-authenticated endpoints (require JWT token)
		r.Group(func(r chi.Router) {
			r.Use(apimiddleware.JWTAuth(jwtManager, logger))

			// Token management
			r.Post("/auth/refresh", authHandler.RefreshToken)
			r.Get("/auth/validate", authHandler.ValidateToken)
		})

		// Authenticated endpoints (require API key OR JWT token)
		r.Group(func(r chi.Router) {
			// Support both API key and JWT authentication
			r.Use(apimiddleware.APIKeyAuth(apiKeyRepo, logger))

			// Per-user rate limiting (stricter than per-IP)
			r.Use(rateLimiter.PerUserMiddleware())

			// Diagnostic endpoints
			r.Post("/diagnostics", diagnosticsHandler.Run)

			// Remediation endpoints
			r.Post("/remediation", remediationHandler.Run)

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
		})
	})

	return r
}
