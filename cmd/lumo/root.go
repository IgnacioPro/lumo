package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile    string
	verbose    bool
	dryRun     bool
	log        = logrus.New()
	version    = "0.4.1"
	rootCtx    context.Context
	rootCancel context.CancelFunc
)

var rootCmd = &cobra.Command{
	Use:   "lumo",
	Short: "Lumo - Intelligent SRE/DevOps Agent",
	Long: `Lumo is an intelligent SRE/DevOps agent that helps you:
  - SSH into remote machines
  - Run diagnostic commands
  - Perform auto-remediation with human-in-the-loop approval
  - Generate detailed reports

Lumo uses AI to analyze system health and suggest or execute fixes.`,
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set up logging
		if verbose {
			log.SetLevel(logrus.DebugLevel)
			log.Debug("Verbose logging enabled")
		} else {
			log.SetLevel(logrus.InfoLevel)
		}

		if dryRun {
			log.Info("Dry-run mode enabled - no changes will be made")
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(initShutdownHandler)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.lumo/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "simulate actions without making changes")
}

// initShutdownHandler sets up graceful shutdown handling for SIGINT and SIGTERM
func initShutdownHandler() {
	// Create root context for the entire application
	rootCtx, rootCancel = context.WithCancel(context.Background())

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Infof("Received signal %v, initiating graceful shutdown...", sig)

		// Cancel the root context to signal all operations to stop
		rootCancel()

		// Give operations 10 seconds to finish gracefully
		time.Sleep(10 * time.Second)

		log.Warn("Graceful shutdown timeout exceeded, forcing exit")
		os.Exit(1)
	}()
}

// getRootContext returns the root context, falling back to Background if not initialized
// This is useful for testing where the cobra initialization doesn't happen
func getRootContext() context.Context {
	if rootCtx != nil {
		return rootCtx
	}
	return context.Background()
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			log.Warn("Unable to find home directory:", err)
			return
		}

		// Search config in home directory and current directory
		viper.AddConfigPath(home + "/.lumo")
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// Read in environment variables that match
	viper.SetEnvPrefix("LUMO")
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		log.Debug("Using config file:", viper.ConfigFileUsed())
	}
}
