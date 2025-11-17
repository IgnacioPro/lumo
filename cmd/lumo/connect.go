package main

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/ssh"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect <host>",
	Short: "Connect to a remote server via SSH",
	Long: `Connect to a remote server via SSH.

Supports multiple authentication methods:
  - SSH agent (recommended)
  - Private key files (~/.ssh/id_ed25519, id_rsa, etc.)
  - Password authentication
  - Keyboard-interactive authentication

Examples:
  lumo connect user@example.com
  lumo connect 192.168.1.10 -u root
  lumo connect example.com -u admin -p 2222
  lumo connect user@example.com -k ~/.ssh/my_key`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runConnect(cmd, args); err != nil {
			log.Errorf("Connection failed: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)

	// Add flags specific to connect command
	connectCmd.Flags().IntP("port", "p", 22, "SSH port")
	connectCmd.Flags().StringP("user", "u", "", "SSH user")
	connectCmd.Flags().StringP("key", "k", "", "Path to SSH private key")
	// NOTE: Password flag removed for security - password prompt will be used if needed
	connectCmd.Flags().Bool("test", true, "Run a test command after connecting")
}

func runConnect(cmd *cobra.Command, args []string) error {
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
	userFlag, _ := cmd.Flags().GetString("user")
	keyPath, _ := cmd.Flags().GetString("key")
	runTest, _ := cmd.Flags().GetBool("test")

	// User from flag takes precedence
	if userFlag != "" {
		username = userFlag
	}

	// Default to current user if not specified
	if username == "" {
		currentUser, err := user.Current()
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}
		username = currentUser.Username
	}

	log.Infof("Connecting to %s@%s:%d...", username, hostname, port)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Warnf("Failed to load config, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	// Create SSH client config
	sshClientConfig := ssh.NewClientConfig(cfg.SSH)

	// Override with command-line flags
	if keyPath != "" {
		if err := sshClientConfig.SetKeyPath(keyPath); err != nil {
			return fmt.Errorf("invalid key path: %w", err)
		}
	}

	// Password authentication will use secure prompting via SSH auth methods
	// No password flag for security - prevents exposure in process lists and history

	// Create SSH client
	sshClient, err := ssh.NewClient(sshClientConfig, log)
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	// Connect to the server
	if err := sshClient.Connect(hostname, port, username); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() {
		_ = sshClient.Disconnect() // Error logged internally
	}()

	// Display connection info
	connInfo := sshClient.GetConnectionInfo()
	log.Infof("✓ Connected successfully!")
	log.Infof("  Host: %s:%d", connInfo.Host, connInfo.Port)
	log.Infof("  User: %s", connInfo.User)
	log.Infof("  Auth method: %s", connInfo.AuthMethodUsed)
	log.Infof("  Connected at: %s", connInfo.ConnectedAt.Format(time.RFC3339))

	// Run a test command if enabled
	if runTest {
		log.Info("\nRunning test commands...")

		testCommands := []string{
			"hostname",
			"whoami",
			"uname -a",
		}

		for _, testCmd := range testCommands {
			log.Debugf("Executing: %s", testCmd)

			result, err := sshClient.Execute(testCmd, &ssh.CommandOptions{
				Timeout: 10 * time.Second,
			})

			if err != nil {
				log.Warnf("  Test command '%s' failed: %v", testCmd, err)
				continue
			}

			if result.Success() {
				output := strings.TrimSpace(result.Stdout)
				log.Infof("  %s: %s", testCmd, output)
			}
		}
	}

	// Check connection health
	log.Info("\nChecking connection health...")
	if err := sshClient.CheckHealth(); err != nil {
		log.Warnf("Health check failed: %v", err)
	} else {
		log.Info("  ✓ Connection is healthy")
	}

	if !dryRun {
		log.Info("\n✓ Connection test completed successfully!")
		fmt.Println("\nConnection verified. You can now use other Lumo commands with this configuration.")
	}

	return nil
}
