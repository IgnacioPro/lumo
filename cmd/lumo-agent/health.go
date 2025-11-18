package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check agent health status",
	Long: `Query the agent's health check endpoint to verify it's running correctly.
This command connects to the agent's health check port and retrieves status information.`,
	RunE: checkHealth,
}

var (
	healthPort     int
	healthEndpoint string
)

func init() {
	rootCmd.AddCommand(healthCmd)

	healthCmd.Flags().IntVarP(&healthPort, "port", "p", 8080, "health check port")
	healthCmd.Flags().StringVarP(&healthEndpoint, "endpoint", "e", "/status", "health check endpoint (/health, /ready, /live, /status)")
}

func checkHealth(cmd *cobra.Command, args []string) error {
	url := fmt.Sprintf("http://localhost:%d%s", healthPort, healthEndpoint)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to agent: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Pretty print JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		// Not JSON, just print raw response
		fmt.Println(string(body))
		return nil
	}

	prettyJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Println(string(body))
		return nil
	}

	fmt.Println(string(prettyJSON))

	// Exit with error code if not healthy
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent is not healthy (status: %d)", resp.StatusCode)
	}

	return nil
}
