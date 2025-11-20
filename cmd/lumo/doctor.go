package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/doctor"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate Lumo configuration and dependencies",
	Long: `The doctor command runs health checks to validate your Lumo installation.

It checks:
  - Configuration file exists and is readable
  - AI provider is configured and valid
  - API keys are set and working
  - RAG system is properly configured (if enabled)
  - System dependencies are available
  - Updates are available

This is useful for troubleshooting setup issues before running diagnostics.`,
	Example: `  # Run all health checks
  lumo doctor

  # Run with verbose output
  lumo doctor -v`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Print header
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║               Lumo Health Check                           ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Load configuration (don't fail if it doesn't exist)
	cfg, err := config.Load()
	if err != nil {
		log.Debugf("Failed to load config (continuing with defaults): %v", err)
		// Use default config
		cfg = &config.Config{
			AI: config.AIConfig{
				Provider: os.Getenv("LUMO_AI_PROVIDER"),
			},
			RAG: config.RAGConfig{
				Enabled: os.Getenv("LUMO_RAG_ENABLED") == "true",
			},
		}
	}

	// Create doctor and add checks
	d := doctor.New(log)

	// Core checks
	d.AddCheck(doctor.NewConfigFileCheck(cfg, log))
	d.AddCheck(doctor.NewAIProviderCheck(cfg, log))
	d.AddCheck(doctor.NewAIKeyCheck(cfg, log))
	d.AddCheck(doctor.NewRAGCheck(cfg, log))
	d.AddCheck(doctor.NewDependencyCheck(cfg, log))
	d.AddCheck(doctor.NewVersionCheck(version, log))

	// Run all checks
	log.Debug("Running health checks...")
	start := time.Now()
	results := d.RunAll(ctx)
	duration := time.Since(start)

	// Print results
	fmt.Println("Running diagnostics...")
	fmt.Println()

	for _, result := range results {
		fmt.Println(doctor.FormatResult(result))
	}

	// Print summary
	fmt.Println()
	fmt.Println("───────────────────────────────────────────────────────────────")

	ok, warnings, errors, skipped := doctor.Summary(results)
	totalChecks := ok + warnings + errors

	fmt.Printf("Health Check Summary (%.2fs)\n", duration.Seconds())
	fmt.Printf("  Total Checks: %d\n", totalChecks)

	if ok > 0 {
		fmt.Printf("  \033[32m✓\033[0m OK:       %d\n", ok)
	}
	if warnings > 0 {
		fmt.Printf("  \033[33m⚠\033[0m Warnings: %d\n", warnings)
	}
	if errors > 0 {
		fmt.Printf("  \033[31m✗\033[0m Errors:   %d\n", errors)
	}
	if skipped > 0 {
		fmt.Printf("  \033[90m○\033[0m Skipped:  %d\n", skipped)
	}

	fmt.Println()

	// Overall status
	if doctor.IsHealthy(results) {
		if warnings > 0 {
			fmt.Println("\033[33m⚠ Lumo is working but has some warnings\033[0m")
			fmt.Println("  Review warnings above for potential improvements")
		} else {
			fmt.Println("\033[32m✓ Lumo is healthy and ready to use!\033[0m")
		}
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  • Run a local diagnostic: lumo diagnose localhost")
		fmt.Println("  • View examples: lumo examples")
		fmt.Println("  • Read docs: https://github.com/ignacio/lumo")
	} else {
		fmt.Println("\033[31m✗ Lumo has configuration issues\033[0m")
		fmt.Println("  Fix errors above before running diagnostics")
		fmt.Println()
		fmt.Println("Quick fixes:")
		fmt.Println("  • Run setup wizard: lumo init")
		fmt.Println("  • Check documentation: https://github.com/ignacio/lumo")
		fmt.Println("  • View examples: lumo examples")
		return fmt.Errorf("health check failed with %d error(s)", errors)
	}

	fmt.Println()
	return nil
}
