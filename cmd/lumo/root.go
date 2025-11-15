package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
	dryRun  bool
	log     = logrus.New()
	version = "0.1.0"
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

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.lumo/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "simulate actions without making changes")
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
