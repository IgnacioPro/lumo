package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start Lumo API server",
	Long: `Start the Lumo API server for remote access.

The API server provides REST endpoints for:
  - POST /connect - Establish SSH connections
  - POST /diagnose - Run diagnostics
  - POST /fix - Execute remediation
  - GET /reports - Retrieve reports
  - GET /status - Agent status
  - WebSocket /logs - Stream logs in real-time

Authentication is required for all endpoints.`,
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		host, _ := cmd.Flags().GetString("host")

		log.Infof("Starting API server on %s:%d...", host, port)
		fmt.Println("⚠️  Not implemented yet - Coming in Phase 7")
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Add flags specific to serve command
	serveCmd.Flags().IntP("port", "p", 8080, "API server port")
	serveCmd.Flags().StringP("host", "H", "0.0.0.0", "API server host")
	serveCmd.Flags().Bool("tls", false, "Enable TLS/HTTPS")
	serveCmd.Flags().String("cert", "", "Path to TLS certificate")
	serveCmd.Flags().String("key", "", "Path to TLS private key")
}
