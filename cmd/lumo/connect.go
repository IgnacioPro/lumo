package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect <host>",
	Short: "Connect to a remote server via SSH",
	Long: `Connect to a remote server via SSH.

Example:
  lumo connect user@example.com
  lumo connect 192.168.1.10`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		host := args[0]
		log.Infof("Connecting to %s...", host)
		fmt.Println("⚠️  Not implemented yet - Coming in Phase 2")
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)

	// Add flags specific to connect command
	connectCmd.Flags().IntP("port", "p", 22, "SSH port")
	connectCmd.Flags().StringP("user", "u", "", "SSH user")
	connectCmd.Flags().StringP("key", "k", "", "Path to SSH private key")
}
