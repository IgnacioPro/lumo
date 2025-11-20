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
	"github.com/ignacio/lumo/internal/observability"
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
	if jwtSecret == "" {
		logger.Warn("No JWT secret configured - JWT authentication will be disabled")
		// Generate a temporary secret for development (NOT for production!)
		tempSecret, _ := auth.GenerateSecureSecret(32)
		jwtSecret = tempSecret
		logger.Warn("Generated temporary JWT secret for development - DO NOT use in production!")
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

	// Initialize tracing (basic setup)
	shutdownTracer := observability.InitTracer("lumo-api")
	defer func() {
		if err := shutdownTracer(context.Background()); err != nil {
			logger.WithError(err).Error("Failed to shutdown tracer")
		}
	}()

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
