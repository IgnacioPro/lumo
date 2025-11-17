package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/remediation"
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

	fixCmd.Flags().BoolP("auto-approve", "y", false, "Auto-approve safe operations")
	fixCmd.Flags().StringSliceP("skip", "s", []string{}, "Skip categories (service, disk, process)")
	fixCmd.Flags().String("audit-log", "", "Audit log path (default: ~/.lumo/remediation-audit.log)")
	fixCmd.Flags().BoolP("list-actions", "l", false, "List suggested actions without executing")
	fixCmd.Flags().Bool("dry-run", false, "Simulate actions without executing them")
}

func runFix(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	_ , err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	autoApprove, _ := cmd.Flags().GetBool("auto-approve")
	skipCategories, _ := cmd.Flags().GetStringSlice("skip")
	auditLogPath, _ := cmd.Flags().GetString("audit-log")
	listOnly, _ := cmd.Flags().GetBool("list-actions")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if auditLogPath == "" {
		homeDir, _ := os.UserHomeDir()
		auditLogPath = filepath.Join(homeDir, ".lumo", "remediation-audit.log")
	}

	log.Info("Starting auto-remediation system")



	// For now, create an empty report - full integration will be added later
	report := &diagnostics.Report{
		Results: []*diagnostics.CheckResult{},
	}

	fmt.Println("\n⚠️  Full diagnostic integration coming soon!")
	fmt.Println("For now, the remediation system is ready with the following capabilities:")
	fmt.Println("  ✓ Service restart actions")
	fmt.Println("  ✓ Disk cleanup actions")
	fmt.Println("  ✓ Process management actions")
	fmt.Println("  ✓ Risk-based approval workflow")
	fmt.Println("  ✓ Audit logging")
	fmt.Println("  ✓ Rollback support\n")

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

	executor := diagnostics.NewLocalExecutor()
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
