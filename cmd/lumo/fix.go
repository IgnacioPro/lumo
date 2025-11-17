package main

import (
	"bufio"
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
	"github.com/ignacio/lumo/internal/remediation"
	"github.com/ignacio/lumo/internal/ssh"
)

var fixCmd = &cobra.Command{
	Use:   "fix [[user@]host]",
	Short: "Execute auto-remediation for detected issues",
	Long: `Execute auto-remediation actions for detected issues.

First runs diagnostics to identify problems, then generates remediation actions.
Critical operations will require human approval before execution.
Safe operations can be auto-approved based on policy configuration.

Example actions:
  - Restart failed services
  - Clean up disk space (logs, cache)
  - Kill problematic processes
  - Apply security patches

Examples:
  lumo fix                           # Fix localhost issues with interactive approval
  lumo fix user@example.com          # Fix remote host via SSH
  lumo fix --dry-run                 # Show what would be done without executing
  lumo fix -y                        # Auto-approve safe operations
  lumo fix --checks service,disk     # Only check specific issues`,
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

	// SSH connection flags
	fixCmd.Flags().IntP("port", "p", 22, "SSH port")
	fixCmd.Flags().StringP("identity", "i", "", "SSH private key file")
	fixCmd.Flags().DurationP("timeout", "t", 30*time.Second, "Connection timeout")

	// Diagnostic flags
	fixCmd.Flags().StringSliceP("checks", "c", []string{}, "Specific checks to run (cpu, memory, disk, process, service, network, etc.)")

	// Remediation flags
	fixCmd.Flags().BoolP("dry-run", "d", false, "Show what would be done without executing")
	fixCmd.Flags().BoolP("auto-approve", "y", false, "Auto-approve safe operations (still requires approval for critical/moderate changes)")
	fixCmd.Flags().StringSliceP("skip", "s", []string{}, "Skip specific remediation action types")
	fixCmd.Flags().BoolP("no-color", "n", false, "Disable colored output")
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
	checksFilter, _ := cmd.Flags().GetStringSlice("checks")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	autoApprove, _ := cmd.Flags().GetBool("auto-approve")
	noColor, _ := cmd.Flags().GetBool("no-color")

	if dryRun {
		log.Info("Dry-run mode enabled - no changes will be made")
	}

	log.Info("Starting auto-remediation system")

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
	if len(checksFilter) > 0 {
		diagConfig.EnabledChecks = checksFilter
	}

	thresholds := diagnostics.DefaultThresholds()
	runner := diagnostics.NewRunner(diagConfig, thresholds, executor, log)

	// Register checkers
	log.Debug("Registering diagnostic checkers")

	checkersToRegister := []diagnostics.Checker{
		// Core system checkers
		checkers.NewCPUChecker(thresholds.CPU),
		checkers.NewMemoryChecker(thresholds.Memory),
		checkers.NewDiskChecker(thresholds.Disk),
		checkers.NewProcessChecker(thresholds.Process),
		checkers.NewServiceChecker([]string{}),
		checkers.NewNetworkChecker(thresholds.Network, cfg.Diagnostics.Network.Targets),
		// Security checkers
		checkers.NewPatchChecker(),
		checkers.NewPortsChecker(cfg.Diagnostics.Security.PortCheck.WhitelistedPorts),
		checkers.NewSSHSecurityChecker(),
		checkers.NewAuthFailuresChecker(
			cfg.Diagnostics.Security.AuthFailureCheck.LookbackHours,
			cfg.Diagnostics.Security.AuthFailureCheck.FailureThreshold,
		),
		// Proxmox virtualization checker
		checkers.NewProxmoxChecker(false, false, false, false, false, false, false),
	}

	// Add Kubernetes checker if enabled
	if cfg.Diagnostics.Kubernetes.Enabled {
		log.Debug("Kubernetes diagnostics enabled")
		checkersToRegister = append(checkersToRegister, checkers.NewKubernetesChecker(cfg.Diagnostics.Kubernetes, log))
	}

	runner.RegisterCheckers(checkersToRegister...)

	// Run diagnostics
	log.Info("Running diagnostic checks...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	report, err := runner.RunAll(ctx)
	if err != nil {
		return fmt.Errorf("diagnostic run failed: %w", err)
	}

	// Display diagnostic results
	formatter := formatters.NewTextFormatter(!noColor, verbose)
	output := formatter.FormatReport(report)
	fmt.Println(output)

	// Generate remediation plan
	log.Info("Generating remediation plan...")
	mapper := remediation.NewMapper()
	plan := mapper.MapFromDiagnostics(report, hostname, username)

	if plan.TotalActions == 0 {
		log.Info("No remediation actions needed")
		return nil
	}

	// Display plan summary
	displayRemediationPlan(plan)

	// Create approval system
	approver := remediation.NewInteractiveApprover(interactivePrompt)

	// Ask for plan approval
	approved, err := approver.ApprovePlan(plan, autoApprove)
	if err != nil {
		return fmt.Errorf("approval error: %w", err)
	}

	if !approved {
		log.Info("Remediation plan rejected by user")
		return nil
	}

	// Create remediation executor
	var remediationExecutor remediation.Executor
	if isLocal {
		// For local execution, we need to adapt the diagnostics executor to remediation executor
		remediationExecutor = remediation.NewLocalExecutor(executor, dryRun)
	} else {
		// For SSH, we also need to adapt
		remediationExecutor = remediation.NewLocalExecutor(executor, dryRun)
	}

	// Create audit logger
	auditLog := remediation.NewAuditLogger("")

	// Create planner and execute
	planner := remediation.NewPlanner(remediationExecutor, approver, auditLog, dryRun)

	log.Info("Executing remediation plan...")
	results, err := planner.ExecutePlan(ctx, plan, autoApprove)
	if err != nil {
		return fmt.Errorf("remediation execution failed: %w", err)
	}

	// Display results
	displayRemediationResults(results, !noColor)

	// Summary
	successCount := 0
	failedCount := 0
	for _, result := range results {
		if result.Status == remediation.StatusSuccess {
			successCount++
		} else if result.Status == remediation.StatusFailed {
			failedCount++
		}
	}

	fmt.Printf("\n════════════════════════════════════════\n")
	fmt.Printf("REMEDIATION COMPLETE\n")
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", failedCount)
	if dryRun {
		fmt.Printf("(DRY-RUN - No changes were actually made)\n")
	}
	fmt.Printf("════════════════════════════════════════\n")

	log.Info("Remediation completed")
	return nil
}

// displayRemediationPlan shows a summary of the remediation plan
func displayRemediationPlan(plan *remediation.Plan) {
	fmt.Printf("\n════════════════════════════════════════\n")
	fmt.Printf("REMEDIATION PLAN\n")
	fmt.Printf("════════════════════════════════════════\n")
	fmt.Printf("Host: %s@%s\n", plan.Username, plan.Hostname)
	fmt.Printf("Total Actions: %d\n", plan.TotalActions)
	fmt.Printf("  Safe (auto-approvable): %d\n", plan.SafeActions)
	fmt.Printf("  Moderate (needs review): %d\n", plan.ModerateActions)
	fmt.Printf("  Critical (explicit approval required): %d\n", plan.CriticalActions)
	fmt.Printf("Estimated Duration: %v\n", plan.EstimatedDuration)
	fmt.Printf("════════════════════════════════════════\n\n")

	// Group by risk level
	fmt.Println("PLANNED ACTIONS:")
	for _, action := range plan.Actions {
		riskIcon := "✓"
		if action.Risk == remediation.RiskModerate {
			riskIcon = "⚠"
		} else if action.Risk == remediation.RiskCritical {
			riskIcon = "🔴"
		}
		fmt.Printf("%s [%s] %s\n", riskIcon, action.Risk, action.Title)
		fmt.Printf("   From: %s\n", action.SourceCheck)
		fmt.Printf("   Metric: %s\n\n", action.Metric)
	}
}

// displayRemediationResults shows the results of remediation execution
func displayRemediationResults(results []*remediation.Result, colored bool) {
	fmt.Printf("\n════════════════════════════════════════\n")
	fmt.Printf("EXECUTION RESULTS\n")
	fmt.Printf("════════════════════════════════════════\n")

	for _, result := range results {
		statusIcon := "✓"
		if result.Status == remediation.StatusFailed {
			statusIcon = "✗"
		} else if result.Status == remediation.StatusRejected {
			statusIcon = "⊘"
		}

		fmt.Printf("%s [%s] Action %s\n", statusIcon, result.Status, result.ActionID[:8])
		if result.Output != "" {
			fmt.Printf("   Output:\n%s\n", indentText(result.Output, 6))
		}
		if result.Error != "" {
			fmt.Printf("   Error: %s\n", result.Error)
		}
		if result.VerificationOutput != "" {
			fmt.Printf("   Verification: %s\n", indentText(result.VerificationOutput, 6))
		}
		fmt.Printf("   Duration: %v\n\n", result.Duration)
	}
}

// interactivePrompt prompts the user for approval
func interactivePrompt(prompt string) (bool, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "yes" || response == "y", nil
}

// indentText adds indentation to multi-line text
func indentText(text string, spaces int) string {
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}
