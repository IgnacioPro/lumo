package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ignacio/lumo/internal/api/auth"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database"
	"github.com/sirupsen/logrus"
)

// Server represents the HTTP API server
type Server struct {
	config     *config.Config
	db         *database.DB
	httpServer *http.Server
	logger     *logrus.Logger
}

// NewServer creates a new API server instance
func NewServer(cfg *config.Config, db *database.DB, logger *logrus.Logger) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if logger == nil {
		logger = logrus.New()
	}

	// Initialize JWT manager
	jwtSecret := cfg.API.JWTSecret
	if jwtSecret == "" {
		// Try environment variable
		jwtSecret = os.Getenv("LUMO_API_JWT_SECRET")
	}

	// Detect production environment
	isProduction := detectProductionMode(cfg)

	if jwtSecret == "" {
		if isProduction {
			// FAIL FAST in production - never use temporary secrets
			return nil, fmt.Errorf("CRITICAL: JWT secret not configured in production. Set LUMO_API_JWT_SECRET environment variable or config.api.jwt_secret")
		}

		// Development mode: generate temporary secret with clear warnings
		logger.Warn("⚠️  NO JWT SECRET - Generating temporary secret for DEVELOPMENT ONLY")
		tempSecret, err := auth.GenerateSecureSecret(32)
		if err != nil {
			return nil, fmt.Errorf("failed to generate temporary JWT secret: %w", err)
		}
		jwtSecret = tempSecret
		logger.Warn("⚠️  TEMPORARY JWT SECRET - Multi-instance deployments will NOT work")
		logger.Warn("⚠️  Sessions will RESET on server restart")
		logger.Warn("⚠️  Set LUMO_API_JWT_SECRET before deploying to production")
	} else {
		// Secret is configured - validate length
		if len(jwtSecret) < 32 {
			return nil, fmt.Errorf("JWT secret too short (minimum 32 characters, got %d)", len(jwtSecret))
		}
		if isProduction {
			logger.Info("JWT secret configured ✓ (production mode)")
		} else {
			logger.Info("JWT secret configured ✓ (development mode)")
		}
	}

	jwtExpiration := cfg.API.JWTExpiration
	if jwtExpiration == 0 {
		jwtExpiration = 24 * time.Hour // Default to 24 hours
	}

	jwtIssuer := cfg.API.JWTIssuer
	if jwtIssuer == "" {
		jwtIssuer = "lumo-api"
	}

	jwtManager, err := auth.NewJWTManager(jwtSecret, jwtExpiration, jwtIssuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT manager: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"jwt_expiration": jwtExpiration,
		"jwt_issuer":     jwtIssuer,
	}).Info("JWT authentication enabled")

	// Create router
	router := NewRouter(db, cfg, jwtManager, logger)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.API.Host, cfg.API.Port),
		Handler:      router,
		ReadTimeout:  cfg.API.ReadTimeout,
		WriteTimeout: cfg.API.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	server := &Server{
		config:     cfg,
		db:         db,
		httpServer: httpServer,
		logger:     logger,
	}

	return server, nil
}

// Start starts the HTTP server with graceful shutdown support
func (s *Server) Start() error {
	// Channel to listen for errors coming from the listener
	serverErrors := make(chan error, 1)

	// Start the server
	go func() {
		s.logger.WithFields(logrus.Fields{
			"addr": s.httpServer.Addr,
			"tls":  s.config.API.TLS,
		}).Info("Starting HTTP server")

		if s.config.API.TLS {
			serverErrors <- s.httpServer.ListenAndServeTLS(s.config.API.CertFile, s.config.API.KeyFile)
		} else {
			serverErrors <- s.httpServer.ListenAndServe()
		}
	}()

	// Channel to listen for interrupt signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or an error
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		s.logger.WithField("signal", sig.String()).Info("Received shutdown signal")

		// Give outstanding requests a deadline for completion
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Gracefully shutdown the server
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.WithError(err).Error("Graceful shutdown failed")

			// Force shutdown
			if err := s.httpServer.Close(); err != nil {
				return fmt.Errorf("could not stop server: %w", err)
			}
		}

		s.logger.Info("Server stopped gracefully")
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")
	return s.httpServer.Shutdown(ctx)
}

// detectProductionMode determines if the server is running in production
// Precedence: LUMO_ENVIRONMENT env var > config.Environment > auto-detection heuristics
func detectProductionMode(cfg *config.Config) bool {
	// Method 1: Check explicit LUMO_ENVIRONMENT variable (highest priority)
	if env := os.Getenv("LUMO_ENVIRONMENT"); env != "" {
		return env == "production" || env == "prod"
	}

	// Method 2: Check config.Environment field (second priority)
	if cfg.Environment != "" {
		return cfg.Environment == "production" || cfg.Environment == "prod"
	}

	// Method 3: Auto-detection heuristics (lowest priority)

	// Check if TLS is enabled (production typically uses TLS)
	if cfg.API.TLS {
		return true
	}

	// Check if binding to public interface (not localhost)
	if cfg.API.Host != "localhost" && cfg.API.Host != "127.0.0.1" && cfg.API.Host != "::1" && cfg.API.Host != "" {
		return true
	}

	// Check Kubernetes environment
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	// Default: assume development if no indicators
	return false
}
