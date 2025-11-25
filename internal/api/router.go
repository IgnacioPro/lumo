package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ignacio/lumo/internal/ai"
	"github.com/ignacio/lumo/internal/api/auth"
	"github.com/ignacio/lumo/internal/api/handlers"
	apimiddleware "github.com/ignacio/lumo/internal/api/middleware"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database"
	"github.com/ignacio/lumo/internal/database/repository"
	"github.com/ignacio/lumo/internal/notifications"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// expandEnvVar expands environment variables in the format ${VAR} or $VAR
// If the value doesn't contain a variable reference, it's returned as-is
func expandEnvVar(value string) string {
	if value == "" {
		return value
	}

	// Handle ${VAR} syntax
	if strings.Contains(value, "${") && strings.Contains(value, "}") {
		return os.ExpandEnv(value)
	}

	// Handle $VAR syntax (but not if it's just a literal string starting with $)
	if strings.HasPrefix(value, "$") && !strings.Contains(value, "/") {
		varName := strings.TrimPrefix(value, "$")
		if envVal := os.Getenv(varName); envVal != "" {
			return envVal
		}
	}

	return value
}

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
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "lumo-api")
	})
	r.Use(apimiddleware.Logger(logger))
	r.Use(apimiddleware.CORS(cfg.API.AllowedOrigins))
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// Per-IP rate limiting (applied to all requests)
	r.Use(rateLimiter.PerIPMiddleware())

	r.Use(middleware.Compress(5))

	// Initialize repositories
	jobRepo := repository.NewJobRepository(db.DB)
	apiKeyRepo := repository.NewAPIKeyRepository(db.DB)
	agentRepo := repository.NewAgentRepository(db.DB)
	approvalRepo := repository.NewApprovalRepository(db.DB)
	eventRepo := repository.NewEventRepository(db.DB)

	// Initialize AI provider if enabled
	var aiProvider ai.Provider
	aiEnabled := cfg.AI.Enabled
	if aiEnabled {
		providerConfig := &ai.ProviderConfig{
			APIKey:      cfg.AI.GetAPIKeyForProvider(cfg.AI.Provider),
			Model:       cfg.AI.Model,
			Endpoint:    cfg.AI.Endpoint,
			Timeout:     cfg.AI.Timeout,
			MaxRetries:  cfg.AI.MaxRetries,
			Temperature: cfg.AI.Temperature,
			MaxTokens:   cfg.AI.MaxTokens,
		}
		var err error
		aiProvider, err = ai.NewProvider(ai.ProviderType(cfg.AI.Provider), providerConfig, logger)
		if err != nil {
			logger.WithError(err).Warn("Failed to initialize AI provider, continuing without AI")
			aiEnabled = false
		}
	}

	// Initialize notification providers
	var notifiers []notifications.Notifier
	notifEnabled := cfg.Notifications.Enabled && len(cfg.Notifications.Notifiers) > 0
	if notifEnabled {
		for _, cfgNotifier := range cfg.Notifications.Notifiers {
			if !cfgNotifier.Enabled {
				continue
			}
			notifCfg := &notifications.NotifierConfig{
				Name:         cfgNotifier.Name,
				Type:         notifications.NotifierType(cfgNotifier.Type),
				Enabled:      cfgNotifier.Enabled,
				WebhookURL:   expandEnvVar(cfgNotifier.WebhookURL),
				BotToken:     expandEnvVar(cfgNotifier.BotToken),
				ChatID:       expandEnvVar(cfgNotifier.ChatID),
				Headers:      cfgNotifier.Headers,
				Method:       cfgNotifier.Method,
				SMTPHost:     cfgNotifier.SMTPHost,
				SMTPPort:     cfgNotifier.SMTPPort,
				SMTPUsername: cfgNotifier.SMTPUser,
				SMTPPassword: expandEnvVar(cfgNotifier.SMTPPass),
				From:         cfgNotifier.From,
				To:           cfgNotifier.To,
			}
			notifier, err := notifications.NewNotifier(notifCfg, logger)
			if err != nil {
				logger.WithError(err).WithField("provider", cfgNotifier.Type).Warn("Failed to initialize notifier")
				continue
			}
			notifiers = append(notifiers, notifier)
			logger.WithFields(logrus.Fields{
				"provider": notifier.Name(),
				"type":     cfgNotifier.Type,
			}).Info("Notification provider initialized successfully")
		}
		if len(notifiers) == 0 {
			logger.Warn("No notification providers initialized")
			notifEnabled = false
		} else {
			logger.WithField("count", len(notifiers)).Info("Notification system enabled")
		}
	}

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db, logger)
	authHandler := handlers.NewAuthHandler(apiKeyRepo, jwtManager, logger)
	diagnosticsHandler := handlers.NewDiagnosticsHandler(jobRepo, cfg, logger)
	remediationHandler := handlers.NewRemediationHandler(jobRepo, approvalRepo, cfg, logger)
	jobsHandler := handlers.NewJobsHandler(jobRepo, logger)
	agentsHandler := handlers.NewAgentsHandler(agentRepo, logger)
	approvalsHandler := handlers.NewApprovalsHandler(approvalRepo, logger)
	eventsHandler := handlers.NewEventsHandler(eventRepo, agentRepo, aiProvider, notifiers, logger, aiEnabled, notifEnabled)

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
			r.Put("/agents/{id}", agentsHandler.Update)
			r.Delete("/agents/{id}", agentsHandler.Delete)

			// Approval endpoints
			r.Get("/approvals", approvalsHandler.List)
			r.Get("/approvals/stats", approvalsHandler.Stats)
			r.Get("/approvals/{id}", approvalsHandler.Get)
			r.Put("/approvals/{id}/approve", approvalsHandler.Approve)
			r.Put("/approvals/{id}/reject", approvalsHandler.Reject)

			// Event endpoints
			r.Post("/events", eventsHandler.SubmitEvents)
			r.Get("/events", eventsHandler.ListEvents)
			r.Get("/events/{id}", eventsHandler.GetEvent)
		})
	})

	return r
}
