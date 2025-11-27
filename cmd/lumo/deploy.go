package main

import (
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy Lumo to various platforms",
	Long: `Deploy Lumo components to various platforms including Kubernetes, cloud providers, and more.

Examples:
  # Deploy to Kubernetes cluster
  lumo deploy kubernetes --db-password $DB_PASS --api-jwt-secret $JWT_SECRET --agent-token $TOKEN

  # Deploy to local Kind cluster for testing
  lumo deploy kubernetes --kind

  # Deploy agent only (infrastructure exists)
  lumo deploy kubernetes --agent-only --db-host postgres.db.svc --redis-host redis.cache.svc
`,
}

func init() {
	// Register subcommands
	deployCmd.AddCommand(newDeployKubernetesCmd())

	// Register with root command
	rootCmd.AddCommand(deployCmd)
}
