package main

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/diagnostics/checkers"
	"github.com/ignacio/lumo/internal/remediation"
	"github.com/ignacio/lumo/internal/ssh"
	"github.com/spf13/cobra"
)

var fixCmd = &cobra.Command{
	Use:   "fix [user@]hostname",
	Short: "Execute auto-remediation for detected issues",
	Long: `Execute auto-remediation actions for detected issues.

Critical operations will require human approval before execution.
Safe operations can be auto-approved based on policy configuration.

Example actions:
  - Restart failed services
  - Clean up disk space (logs, cache)
  - Kill problematic processes

Examples:
  lumo fix localhost                          # Analyze localhost and suggest fixes
  lumo fix user@server.com --auto-approve     # Auto-approve safe operations
  lumo fix user@server.com --skip service     # Skip service-related fixes`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runFix(cmd, args); err != nil {
			log.Errorf("Remediation failed: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(fixCmd)

	// SSH connection flags (same as diagnose command)
	fixCmd.Flags().IntP("port", "p", 22, "SSH port")
	fixCmd.Flags().StringP("identity", "i", "", "SSH private key file")
	fixCmd.Flags().DurationP("timeout", "t", 30*time.Second, "Connection timeout")

	// Remediation flags
	fixCmd.Flags().BoolP("auto-approve", "y", false, "Auto-approve safe operations")
	fixCmd.Flags().StringSliceP("skip", "s", []string{}, "Skip categories (service, disk, process)")
	fixCmd.Flags().String("audit-log", "", "Audit log path (default: ~/.lumo/remediation-audit.log)")
	fixCmd.Flags().BoolP("list-actions", "l", false, "List suggested actions without executing")
	fixCmd.Flags().Bool("dry-run", false, "Simulate actions without executing them")
}

func runFix(cmd *cobra.Command, args []string) error {
	// Default to localhost if no host argument provided
	hostArg := "localhost"
	if len(args) > 0 {
		hostArg = args[0]
	}

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
	autoApprove, _ := cmd.Flags().GetBool("auto-approve")
	skipCategories, _ := cmd.Flags().GetStringSlice("skip")
	auditLogPath, _ := cmd.Flags().GetString("audit-log")
	listOnly, _ := cmd.Flags().GetBool("list-actions")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Default to current user if not specified
	if username == "" {
		currentUser, err := user.Current()
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}
		username = currentUser.Username
	}

	// Check if this is a localhost execution (no SSH needed)
	isLocal := isLocalhost(hostname)

	if isLocal {
		log.Info("Running diagnostics locally (no SSH connection needed)")
	} else {
		log.Infof("Starting diagnostics for %s@%s", username, hostname)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Warnf("Failed to load config, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	// Set audit log path
	if auditLogPath == "" {
		homeDir, _ := os.UserHomeDir()
		auditLogPath = filepath.Join(homeDir, ".lumo", "remediation-audit.log")
	}

	log.Info("Starting auto-remediation system")

	// Create command executor (local or SSH)
	var executor diagnostics.CommandExecutor
	if isLocal {
		// Use local executor - no SSH needed
		executor = diagnostics.NewLocalExecutor()
		log.Debug("Using local command executor")
	} else {
		// Create SSH client configuration
		sshClientConfig := ssh.NewClientConfig(cfg.SSH)

		// Override with command-line flags
		if identityFile != "" {
			if err := sshClientConfig.SetKeyPath(identityFile); err != nil {
				return fmt.Errorf("invalid key path: %w", err)
			}
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

		// Create SSH executor
		executor = diagnostics.NewSSHExecutor(sshClient)
	}

	// Create diagnostic runner
	diagConfig := diagnostics.DefaultConfig()
	thresholds := diagnostics.DefaultThresholds()
	runner := diagnostics.NewRunner(diagConfig, thresholds, executor, log)

	// Register checkers (same as diagnose command)
	log.Debug("Registering diagnostic checkers")

	checkersToRegister := []diagnostics.Checker{
		// Core system checkers
		checkers.NewCPUChecker(thresholds.CPU),
		checkers.NewMemoryChecker(thresholds.Memory),
		checkers.NewDiskChecker(thresholds.Disk),
		checkers.NewProcessChecker(thresholds.Process),
		checkers.NewServiceChecker([]string{}), // Empty list = check all services
		checkers.NewNetworkChecker(thresholds.Network, cfg.Diagnostics.Network.Targets),
		// Security checkers
		checkers.NewPatchChecker(),
		checkers.NewPortsChecker(cfg.Diagnostics.Security.PortCheck.WhitelistedPorts),
		checkers.NewSSHSecurityChecker(),
		checkers.NewAuthFailuresChecker(
			cfg.Diagnostics.Security.AuthFailureCheck.LookbackHours,
			cfg.Diagnostics.Security.AuthFailureCheck.FailureThreshold,
		),
		// Proxmox checker (auto-skips if not installed)
		checkers.NewProxmoxChecker(false, false, false, false, false, false, false),
	}

	// Add Kubernetes checker if enabled in config
	if cfg.Diagnostics.Kubernetes.Enabled {
		log.Debug("Kubernetes diagnostics enabled")
		checkersToRegister = append(checkersToRegister, checkers.NewKubernetesChecker(cfg.Diagnostics.Kubernetes, log))
	}

	runner.RegisterCheckers(checkersToRegister...)

	// Run diagnostics to detect issues
	log.Info("Running diagnostic checks to detect issues...")
	ctx, cancel := context.WithTimeout(getRootContext(), 2*time.Minute)
	defer cancel()

	report, err := runner.RunAll(ctx)
	if err != nil {
		return fmt.Errorf("diagnostic run failed: %w", err)
	}

	log.Infof("Diagnostics complete - found %d check results", len(report.Results))

	// Generate remediation plan from diagnostic report
	registry := remediation.GetDefaultRegistry(log)
	suggester := remediation.NewSuggester(registry, log)

	plan, err := suggester.SuggestFromReport(report)
	if err != nil {
		return fmt.Errorf("failed to generate remediation plan: %w", err)
	}

	if len(skipCategories) > 0 {
		for _, cat := range skipCategories {
			plan.SkipCategories = append(plan.SkipCategories, remediation.ActionCategory(cat))
		}
	}

	plan.DryRun = dryRun
	plan.AutoApprove = autoApprove

	filteredActions := plan.FilteredActions()

	if len(filteredActions) == 0 {
		fmt.Println("✅ No issues requiring remediation detected")
		return nil
	}

	displayRemediationPlan(plan)

	if listOnly {
		return nil
	}

	auditor, err := remediation.NewAuditor(auditLogPath, log)
	if err != nil {
		return fmt.Errorf("failed to initialize audit log: %w", err)
	}
	defer auditor.Close()

	approver := remediation.NewApprover(log)

	// Use the same executor we created earlier (local or SSH)
	remediationExecutor := remediation.NewExecutor(executor, auditor, approver, log, dryRun, autoApprove)

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Starting Remediation Execution")
	fmt.Println(strings.Repeat("=", 70))

	executionReport, err := remediationExecutor.ExecutePlan(ctx, plan)
	if err != nil {
		return fmt.Errorf("remediation execution failed: %w", err)
	}

	displayExecutionReport(executionReport)

	return nil
}

func displayRemediationPlan(plan *remediation.RemediationPlan) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Remediation Plan")
	fmt.Println(strings.Repeat("=", 70))

	if plan.DryRun {
		fmt.Println("🔍 DRY-RUN MODE: Actions will be simulated, not executed")
	}
	if plan.AutoApprove {
		fmt.Println("⚡ AUTO-APPROVE: Safe actions will be automatically approved")
	}
	if len(plan.SkipCategories) > 0 {
		fmt.Printf("⏭️  SKIPPING: %v\n", plan.SkipCategories)
	}

	fmt.Println()

	filteredActions := plan.FilteredActions()
	fmt.Printf("Total suggested actions: %d\n\n", len(filteredActions))

	for i, action := range filteredActions {
		fmt.Printf("%d. [%s] %s\n", i+1, formatRiskBadge(action.Risk()), action.Name())
		fmt.Printf("   Category: %s\n", action.Category())
		fmt.Printf("   Description: %s\n", action.Description())
		fmt.Printf("   Impact: %s\n", action.EstimateImpact())
		fmt.Printf("   Reversible: %v\n", action.IsReversible())
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 70))
}

func displayExecutionReport(report *remediation.ExecutionReport) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Remediation Execution Report")
	fmt.Println(strings.Repeat("=", 70))

	fmt.Printf("Duration: %s\n", report.Duration)
	fmt.Printf("Total Actions: %d\n", report.TotalActions)
	fmt.Printf("Executed: %d\n", report.Executed)
	fmt.Printf("✅ Succeeded: %d\n", report.Succeeded)
	fmt.Printf("❌ Failed: %d\n", report.Failed)
	fmt.Printf("⏭️  Skipped: %d\n", report.Skipped)
	fmt.Printf("🚫 Rejected: %d\n", report.Rejected)
	fmt.Printf("↩️  Rolled Back: %d\n", report.RolledBack)

	if report.DryRun {
		fmt.Println("\n🔍 This was a DRY-RUN - no actual changes were made")
	}

	fmt.Println("\n" + strings.Repeat("-", 70))
	fmt.Println("Action Results:")
	fmt.Println(strings.Repeat("-", 70))

	for i, result := range report.Results {
		fmt.Printf("\n%d. %s\n", i+1, result.ActionID)
		fmt.Printf("   Status: %s\n", formatStatusBadge(result.Status))
		fmt.Printf("   Message: %s\n", result.Message)
		fmt.Printf("   Duration: %s\n", result.Duration)

		if len(result.ChangesApplied) > 0 {
			fmt.Println("   Changes:")
			for _, change := range result.ChangesApplied {
				fmt.Printf("     - %s\n", change)
			}
		}

		if result.Error != "" {
			fmt.Printf("   Error: %s\n", result.Error)
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
}

func formatRiskBadge(risk remediation.RiskLevel) string {
	switch risk {
	case remediation.RiskSafe:
		return "✅ SAFE"
	case remediation.RiskModerate:
		return "⚠️  MODERATE"
	case remediation.RiskCritical:
		return "⛔ CRITICAL"
	default:
		return string(risk)
	}
}

func formatStatusBadge(status remediation.ActionStatus) string {
	switch status {
	case remediation.StatusSuccess:
		return "✅ SUCCESS"
	case remediation.StatusFailed:
		return "❌ FAILED"
	case remediation.StatusSkipped:
		return "⏭️  SKIPPED"
	case remediation.StatusRejected:
		return "🚫 REJECTED"
	case remediation.StatusRolledBack:
		return "↩️  ROLLED BACK"
	default:
		return string(status)
	}
}
