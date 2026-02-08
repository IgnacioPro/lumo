package api

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/ignacio/lumo/internal/ai"
	"github.com/ignacio/lumo/internal/api/auth"
	"github.com/ignacio/lumo/internal/api/handlers"
	apimiddleware "github.com/ignacio/lumo/internal/api/middleware"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/correlation"
	"github.com/ignacio/lumo/internal/database"
	"github.com/ignacio/lumo/internal/database/repository"
	"github.com/ignacio/lumo/internal/notifications"
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
	r.Use(apimiddleware.RequestSizeLimit(5 * 1024 * 1024)) // 5 MB limit to prevent DoS
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
				Headers:      cfgNotifier.Headers,
				Method:       cfgNotifier.Method,
				SMTPHost:     cfgNotifier.SMTPHost,
				SMTPPort:     cfgNotifier.SMTPPort,
				SMTPUsername: cfgNotifier.SMTPUser,
				SMTPPassword: expandEnvVar(cfgNotifier.SMTPPass),
				From:         cfgNotifier.From,
				To:           cfgNotifier.To,
				UseTLS:       cfgNotifier.UseTLS,
				Timeout:      cfgNotifier.Timeout,
			}

			// Map bot_token and chat_id based on notifier type
			botToken := expandEnvVar(cfgNotifier.BotToken)
			chatID := expandEnvVar(cfgNotifier.ChatID)

			switch cfgNotifier.Type {
			case "slack":
				notifCfg.SlackBotToken = botToken
				notifCfg.SlackChannel = chatID
			case "telegram":
				notifCfg.TelegramBotToken = botToken
				notifCfg.TelegramChatID = chatID
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

	// Initialize correlation engine (Phase 19) with Realtime Manager (Phase 20)
	incidentRepo := correlation.NewInMemoryIncidentRepository(logger)
	var correlationEngine *correlation.Engine
	var realtimeManager *correlation.RealtimeIncidentManager
	var incidentNotifier *correlation.IncidentNotifierImpl

	logger.WithFields(logrus.Fields{
		"notif_enabled":  notifEnabled,
		"notifier_count": len(notifiers),
	}).Info("Checking correlation engine conditions")

	// Only enable correlation if notifications are available
	if notifEnabled && len(notifiers) > 0 {
		// Build API base URL from config
		apiBaseURL := cfg.API.BaseURL
		if apiBaseURL == "" {
			apiBaseURL = expandEnvVar("${LUMO_API_BASE_URL}")
		}

		incidentNotifier = correlation.NewIncidentNotifier(notifiers, apiBaseURL, logger)

		// Initialize AI analyzer only if AI is enabled and provider is available
		// We explicitly use the AIAnalyzer interface type to avoid passing a typed nil
		var aiAnalyzer correlation.AIAnalyzer
		if aiEnabled && aiProvider != nil {
			aiAnalyzer = correlation.NewAIIncidentAnalyzer(aiProvider, logger)
		}

		// Create the correlation engine (Phase 19 - still used for rules/categories)
		correlationEngine = correlation.NewEngine(
			nil, // Use default config
			nil, // Context gatherer (not in API server - only in agent)
			aiAnalyzer,
			incidentNotifier,
			incidentRepo,
			logger,
		)

		// Create the realtime incident manager (Phase 20)
		engineConfig := correlation.DefaultEngineConfig()
		// Configure critical labels for immediate notification
		engineConfig.CriticalLabels = map[string]string{
			"critical": "true",
		}
		// Configure intervals
		engineConfig.HealthCheckInterval = 30 * time.Second
		engineConfig.ProgressNotificationInterval = 45 * time.Second

		realtimeManager = correlation.NewRealtimeIncidentManager(
			engineConfig,
			incidentRepo,
			nil, // Context gatherer (not in API server - only in agent)
			aiAnalyzer,
			incidentNotifier,
			&correlation.NoOpHealthChecker{}, // API server doesn't have K8s client
			logger,
		)

		// Start both engines
		if err := correlationEngine.Start(); err != nil {
			logger.WithError(err).Error("Failed to start correlation engine")
		} else {
			logger.Info("Correlation engine started for incident management")
		}

		if err := realtimeManager.Start(); err != nil {
			logger.WithError(err).Error("Failed to start realtime incident manager")
		} else {
			logger.Info("Realtime incident manager started (Phase 20)")
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

	// Wire correlation engine to events handler
	if correlationEngine != nil {
		eventsHandler.SetCorrelationEngine(correlationEngine)
	}

	// Wire realtime incident manager to events handler (Phase 20)
	if realtimeManager != nil {
		eventsHandler.SetRealtimeManager(realtimeManager)
	}

	// Initialize incidents handler
	incidentsHandler := handlers.NewIncidentsHandler(correlationEngine, incidentRepo, logger)

	// Initialize Slack interaction handler
	// The signing secret should be configured via environment variable for security
	slackSigningSecret := expandEnvVar("${SLACK_SIGNING_SECRET}")
	slackInteractionHandler := handlers.NewSlackInteractionHandler(incidentRepo, correlationEngine, slackSigningSecret, logger)

	// Prometheus metrics endpoint (public, no auth required)
	r.Handle("/metrics", promhttp.Handler())

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public endpoints (no authentication required)
		r.Get("/health", healthHandler.Health)
		r.Get("/ready", healthHandler.Ready)
		r.Get("/live", healthHandler.Live)

		// Public event analysis view (shareable HTML page)
		r.Get("/events/{id}/analysis", eventsHandler.GetEventAnalysisHTML)

		// Public incident analysis view (shareable HTML page)
		r.Get("/incidents/{id}/analysis", incidentsHandler.GetIncidentAnalysis)

		// Slack interactions endpoint (must be public for Slack to call it)
		// Uses Slack signature verification instead of API keys
		r.With(slackInteractionHandler.VerifySlackSignature).Post("/slack/interactions", slackInteractionHandler.HandleInteraction)

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

			// Incident endpoints (Phase 19)
			r.Get("/incidents", incidentsHandler.ListIncidents)
			r.Get("/incidents/open", incidentsHandler.GetOpenIncidents)
			r.Get("/incidents/stats", incidentsHandler.GetIncidentStats)
			r.Get("/incidents/{id}", incidentsHandler.GetIncident)
		})
	})

	return r
}
