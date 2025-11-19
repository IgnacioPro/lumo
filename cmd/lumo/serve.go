package main

import (
	"fmt"

	"github.com/ignacio/lumo/internal/api"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start Lumo API server",
	Long: `Start the Lumo API server for remote access.

The API server provides REST endpoints for:
  - GET /api/v1/health - Health check
  - GET /api/v1/ready - Readiness probe
  - GET /api/v1/live - Liveness probe
  - POST /api/v1/diagnostics - Run diagnostics
  - GET /api/v1/jobs - List jobs
  - GET /api/v1/jobs/:id - Get job status
  - POST /api/v1/agents/register - Register agent
  - PUT /api/v1/agents/:id/heartbeat - Agent heartbeat

Authentication is required for most endpoints.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Override config with command-line flags
		if cmd.Flags().Changed("port") {
			port, _ := cmd.Flags().GetInt("port")
			cfg.API.Port = port
		}
		if cmd.Flags().Changed("host") {
			host, _ := cmd.Flags().GetString("host")
			cfg.API.Host = host
		}
		if cmd.Flags().Changed("tls") {
			tls, _ := cmd.Flags().GetBool("tls")
			cfg.API.TLS = tls
		}
		if cmd.Flags().Changed("cert") {
			cert, _ := cmd.Flags().GetString("cert")
			cfg.API.CertFile = cert
		}
		if cmd.Flags().Changed("key") {
			key, _ := cmd.Flags().GetString("key")
			cfg.API.KeyFile = key
		}

		// Configure logging level from config
		switch cfg.Logging.Level {
		case "debug":
			log.SetLevel(logrus.DebugLevel)
		case "info":
			log.SetLevel(logrus.InfoLevel)
		case "warn":
			log.SetLevel(logrus.WarnLevel)
		case "error":
			log.SetLevel(logrus.ErrorLevel)
		default:
			log.SetLevel(logrus.InfoLevel)
		}

		log.WithField("version", "0.9.1").Info("Starting Lumo API Server")

		// Connect to database
		dbConfig := &database.PostgresConfig{
			Host:            cfg.Database.Host,
			Port:            cfg.Database.Port,
			Name:            cfg.Database.Name,
			User:            cfg.Database.User,
			Password:        cfg.Database.Password,
			SSLMode:         cfg.Database.SSLMode,
			MaxConnections:  cfg.Database.MaxConnections,
			MaxIdle:         cfg.Database.MaxIdle,
			ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		}

		db, err := database.NewPostgresDB(dbConfig, log)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer func() {
			if err := db.Close(); err != nil {
				log.WithError(err).Warn("Failed to close database connection")
			}
		}()

		// Run database migrations
		if err := database.RunMigrations(db.DB, log); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		// Create and start API server
		server, err := api.NewServer(cfg, db, log)
		if err != nil {
			return fmt.Errorf("failed to create server: %w", err)
		}

		log.WithField("addr", fmt.Sprintf("%s:%d", cfg.API.Host, cfg.API.Port)).Info("API server starting")

		// Start server (blocks until shutdown)
		if err := server.Start(); err != nil {
			return fmt.Errorf("server error: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Add flags specific to serve command
	serveCmd.Flags().IntP("port", "p", 8080, "API server port")
	serveCmd.Flags().StringP("host", "H", "0.0.0.0", "API server host")
	serveCmd.Flags().Bool("tls", false, "Enable TLS/HTTPS")
	serveCmd.Flags().String("cert", "", "Path to TLS certificate")
	serveCmd.Flags().String("key", "", "Path to TLS private key")
}
