package main

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/diagnostics/checkers"
	"github.com/ignacio/lumo/internal/diagnostics/formatters"
	"github.com/ignacio/lumo/internal/ssh"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose [user@]host",
	Short: "Run diagnostic commands on remote servers",
	Long: `Run diagnostic commands to check system health including:
  - CPU usage and load average
  - Memory and swap usage
  - Disk space and inode usage
  - Process counts, zombies, and resource consumers
  - Log analysis (coming soon)
  - Network connectivity (coming soon)

The command connects to the remote server via SSH, runs diagnostic checks,
and displays the results in a formatted report.

Examples:
  lumo diagnose user@example.com
  lumo diagnose root@192.168.1.10 --port 2222
  lumo diagnose admin@server --checks cpu,memory,disk,process
  lumo diagnose user@host --format json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDiagnostics(cmd, args); err != nil {
			log.Errorf("Diagnostics failed: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(diagnoseCmd)

	// SSH connection flags
	diagnoseCmd.Flags().IntP("port", "p", 22, "SSH port")
	diagnoseCmd.Flags().StringP("identity", "i", "", "SSH private key file")
	diagnoseCmd.Flags().StringP("password", "P", "", "SSH password (not recommended, use key-based auth)")
	diagnoseCmd.Flags().DurationP("timeout", "t", 30*time.Second, "Connection timeout")

	// Diagnostic flags
	diagnoseCmd.Flags().StringSliceP("checks", "c", []string{}, "Specific checks to run (cpu, memory, disk, process)")
	diagnoseCmd.Flags().BoolP("all", "a", true, "Run all available diagnostic checks")
	diagnoseCmd.Flags().StringP("format", "f", "text", "Output format (text, json)")
	diagnoseCmd.Flags().BoolP("no-color", "n", false, "Disable colored output")
}

func runDiagnostics(cmd *cobra.Command, args []string) error {
	hostArg := args[0]

	// Parse host argument (supports user@host format)
	var username, hostname string
	if strings.Contains(hostArg, "@") {
		parts := strings.SplitN(hostArg, "@", 2)
		username = parts[0]
		hostname = parts[1]
	} else {
		hostname = hostArg
	}

	// Get flags
	port, _ := cmd.Flags().GetInt("port")
	identityFile, _ := cmd.Flags().GetString("identity")
	password, _ := cmd.Flags().GetString("password")
	checksFilter, _ := cmd.Flags().GetStringSlice("checks")
	format, _ := cmd.Flags().GetString("format")
	noColor, _ := cmd.Flags().GetBool("no-color")

	// Default to current user if not specified
	if username == "" {
		currentUser, err := user.Current()
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}
		username = currentUser.Username
	}

	log.Infof("Starting diagnostics for %s@%s", username, hostname)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Warnf("Failed to load config, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	// Create SSH client configuration
	sshClientConfig := ssh.NewClientConfig(cfg.SSH)

	// Override with command-line flags
	if identityFile != "" {
		if err := sshClientConfig.SetKeyPath(identityFile); err != nil {
			return fmt.Errorf("invalid key path: %w", err)
		}
	}

	if password != "" {
		log.Warn("Using password from command line is not secure!")
		sshClientConfig.SetPassword(password)
	}

	// Create SSH client
	log.Debug("Creating SSH client")
	sshClient, err := ssh.NewClient(sshClientConfig, log)
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	// Connect
	log.Infof("Connecting to %s@%s:%d", username, hostname, port)
	if err := sshClient.Connect(hostname, port, username); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Disconnect()

	log.Info("Connected successfully")

	// Create command executor
	executor := diagnostics.NewSSHExecutor(sshClient)

	// Create diagnostic runner
	diagConfig := diagnostics.DefaultConfig()
	// If specific checks are requested, use only those (overrides --all)
	if len(checksFilter) > 0 {
		diagConfig.EnabledChecks = checksFilter
	}

	thresholds := diagnostics.DefaultThresholds()
	runner := diagnostics.NewRunner(diagConfig, thresholds, executor, log)

	// Register checkers (Phase 3.3: CPU, Memory, Disk, Process)
	log.Debug("Registering diagnostic checkers")
	runner.RegisterCheckers(
		checkers.NewCPUChecker(thresholds.CPU),
		checkers.NewMemoryChecker(thresholds.Memory),
		checkers.NewDiskChecker(thresholds.Disk),
		checkers.NewProcessChecker(thresholds.Process),
	)

	// Run diagnostics
	log.Info("Running diagnostic checks...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	report, err := runner.RunAll(ctx)
	if err != nil {
		return fmt.Errorf("diagnostic run failed: %w", err)
	}

	// Format and display results
	if format == "json" {
		jsonOutput, err := report.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
		fmt.Println(jsonOutput)
	} else {
		// Text format
		formatter := formatters.NewTextFormatter(!noColor, verbose)
		output := formatter.FormatReport(report)
		fmt.Println(output)
	}

	// Exit with error code if critical issues found
	if report.HasCriticalIssues() {
		log.Warn("Critical issues detected")
		return nil // Don't return error, just log warning
	}

	log.Info("Diagnostics completed successfully")
	return nil
}
