package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Run diagnostic commands on connected servers",
	Long: `Run diagnostic commands to check system health including:
  - CPU usage
  - Memory usage
  - Disk space
  - Process status
  - Log analysis
  - Network connectivity

The AI agent will analyze the results and identify issues.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Running diagnostics...")
		fmt.Println("⚠️  Not implemented yet - Coming in Phase 3")
	},
}

func init() {
	rootCmd.AddCommand(diagnoseCmd)

	// Add flags specific to diagnose command
	diagnoseCmd.Flags().StringSliceP("checks", "c", []string{}, "Specific checks to run (cpu, memory, disk, processes, logs, network)")
	diagnoseCmd.Flags().BoolP("all", "a", true, "Run all diagnostic checks")
}
