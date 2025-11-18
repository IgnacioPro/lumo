package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ignacio/lumo/internal/agent"
	"github.com/ignacio/lumo/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
	log     = logrus.New()
)

var rootCmd = &cobra.Command{
	Use:   "lumo-agent",
	Short: "Lumo Agent - Intelligent SRE/DevOps automation daemon",
	Long: `Lumo Agent is a daemon that runs on your infrastructure to perform
system diagnostics, monitoring, and auto-remediation.

The agent can operate in multiple modes:
  - scheduled: Periodic diagnostics based on cron schedule
  - on-demand: Diagnostics triggered via API calls
  - continuous: Continuous monitoring with high-frequency checks
  - hybrid: Combination of scheduled and on-demand (recommended)

The agent reports results to the Lumo API server and provides:
  - Health check endpoints for Kubernetes liveness/readiness probes
  - Prometheus metrics for monitoring
  - Local caching for offline resilience`,
	RunE: runAgent,
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.lumo/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Agent-specific flags
	rootCmd.Flags().String("mode", "", "agent mode (scheduled|on-demand|continuous|hybrid)")
	rootCmd.Flags().String("schedule", "", "cron schedule for periodic diagnostics")
	rootCmd.Flags().String("api-endpoint", "", "Lumo API server endpoint")
	rootCmd.Flags().String("token", "", "API authentication token")
	rootCmd.Flags().Int("health-port", 0, "health check server port")
	rootCmd.Flags().Int("metrics-port", 0, "metrics server port")

	// Bind flags to viper
	viper.BindPFlag("agent.mode", rootCmd.Flags().Lookup("mode"))
	viper.BindPFlag("agent.schedule", rootCmd.Flags().Lookup("schedule"))
	viper.BindPFlag("agent.api_endpoint", rootCmd.Flags().Lookup("api-endpoint"))
	viper.BindPFlag("agent.token", rootCmd.Flags().Lookup("token"))
	viper.BindPFlag("agent.health_check_port", rootCmd.Flags().Lookup("health-port"))
	viper.BindPFlag("agent.metrics_port", rootCmd.Flags().Lookup("metrics-port"))
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in home directory
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
			os.Exit(1)
		}

		// Search config in home directory with name ".lumo" (without extension)
		viper.AddConfigPath(filepath.Join(home, ".lumo"))
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Read environment variables
	viper.SetEnvPrefix("LUMO")
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		log.WithField("config", viper.ConfigFileUsed()).Info("Using config file")
	} else {
		log.Debug("No config file found, using defaults and environment variables")
	}

	// Configure logging
	if verbose {
		log.SetLevel(logrus.DebugLevel)
	} else {
		logLevel := viper.GetString("logging.level")
		level, err := logrus.ParseLevel(logLevel)
		if err != nil {
			level = logrus.InfoLevel
		}
		log.SetLevel(level)
	}

	logFormat := viper.GetString("logging.format")
	if logFormat == "json" {
		log.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}
}

func runAgent(cmd *cobra.Command, args []string) error {
	log.Info("Starting Lumo Agent")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create agent
	a, err := agent.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start agent
	if err := a.Start(ctx); err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	// Wait for shutdown signal
	sig := <-sigCh
	log.WithField("signal", sig).Info("Received shutdown signal")

	// Cancel context to stop background routines
	cancel()

	// Stop agent
	if err := a.Stop(); err != nil {
		return fmt.Errorf("failed to stop agent: %w", err)
	}

	log.Info("Agent shutdown complete")
	return nil
}
