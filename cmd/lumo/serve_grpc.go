package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database"
	"github.com/ignacio/lumo/internal/database/repository"
	grpchandlers "github.com/ignacio/lumo/internal/grpc/handlers"
	grpcserver "github.com/ignacio/lumo/internal/grpc/server"
)

var serveGRPCCmd = &cobra.Command{
	Use:   "serve-grpc",
	Short: "Start the Lumo gRPC API server",
	Long: `Start the Lumo gRPC API server with all services.

This command starts the gRPC server with support for:
- Diagnostics: Run and retrieve diagnostic results
- Agents: Register and manage agents
- Health: Health checks for Kubernetes probes

The server supports TLS/mTLS for secure communication and JWT authentication.`,
	RunE: runServeGRPC,
}

func init() {
	rootCmd.AddCommand(serveGRPCCmd)

	serveGRPCCmd.Flags().Int("port", 9443, "gRPC server port")
	serveGRPCCmd.Flags().Bool("tls", false, "Enable TLS (requires cert and key files)")
	serveGRPCCmd.Flags().Bool("mtls", false, "Enable mutual TLS (requires client certificates)")
	serveGRPCCmd.Flags().String("cert", "deployments/certs/server.crt", "TLS certificate file")
	serveGRPCCmd.Flags().String("key", "deployments/certs/server.key", "TLS key file")
	serveGRPCCmd.Flags().String("ca-file", "deployments/certs/ca.crt", "CA certificate for mTLS client verification")
	serveGRPCCmd.Flags().Int("max-msg-size", 10*1024*1024, "Max message size in bytes (default 10MB)")
}

func runServeGRPC(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get flags
	port, _ := cmd.Flags().GetInt("port")
	tlsEnabled, _ := cmd.Flags().GetBool("tls")
	mtlsEnabled, _ := cmd.Flags().GetBool("mtls")
	certFile, _ := cmd.Flags().GetString("cert")
	keyFile, _ := cmd.Flags().GetString("key")
	caFile, _ := cmd.Flags().GetString("ca-file")
	maxMsgSize, _ := cmd.Flags().GetInt("max-msg-size")

	// Validate TLS/mTLS flags
	if mtlsEnabled && !tlsEnabled {
		log.Warn("mTLS enabled, automatically enabling TLS")
		tlsEnabled = true
	}

	log.WithFields(map[string]interface{}{
		"port":         port,
		"tls_enabled":  tlsEnabled,
		"mtls_enabled": mtlsEnabled,
	}).Info("Starting gRPC server")

	// Initialize database
	var db *database.DB
	if cfg.Database.Enabled {
		log.Info("Connecting to database...")
		db, err = database.NewPostgresDB(&cfg.Database)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()

		// Run migrations
		log.Info("Running database migrations...")
		if err := database.RunMigrations(cfg.Database.URL); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
		log.Info("Database migrations completed")
	} else {
		log.Warn("Database disabled - some features will not be available")
	}

	// Initialize repositories
	var jobRepo repository.JobRepository
	var agentRepo repository.AgentRepository

	if db != nil {
		jobRepo = repository.NewJobRepository(db.DB)
		agentRepo = repository.NewAgentRepository(db.DB)
	}

	// Create gRPC server
	grpcServer, err := grpcserver.NewServer(cfg, grpcserver.ServerOptions{
		Port:        port,
		TLSEnabled:  tlsEnabled,
		MTLSEnabled: mtlsEnabled,
		CertFile:    certFile,
		KeyFile:     keyFile,
		CAFile:      caFile,
		MaxMsgSize:  maxMsgSize,
	}, log)
	if err != nil {
		return fmt.Errorf("failed to create gRPC server: %w", err)
	}

	// Register service handlers
	if db != nil {
		diagnosticsHandler := grpchandlers.NewDiagnosticsHandler(jobRepo)
		lumov1.RegisterDiagnosticsServiceServer(grpcServer.GetGRPCServer(), diagnosticsHandler)
		log.Info("Registered DiagnosticsService")

		agentsHandler := grpchandlers.NewAgentsHandler(agentRepo)
		lumov1.RegisterAgentsServiceServer(grpcServer.GetGRPCServer(), agentsHandler)
		log.Info("Registered AgentsService")
	} else {
		log.Warn("Skipping DiagnosticsService and AgentsService (database disabled)")
	}

	// Health service always available
	healthHandler := grpchandlers.NewHealthHandler(cfg, db.DB)
	lumov1.RegisterHealthServiceServer(grpcServer.GetGRPCServer(), healthHandler)
	log.Info("Registered HealthService")

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- grpcServer.Start()
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return fmt.Errorf("gRPC server error: %w", err)
	case sig := <-sigChan:
		log.WithField("signal", sig).Info("Received shutdown signal")

		// Graceful shutdown with 30s timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := grpcServer.GracefulStop(ctx); err != nil {
			log.WithError(err).Warn("Graceful shutdown failed")
			return err
		}

		log.Info("gRPC server shutdown complete")
		return nil
	}
}
