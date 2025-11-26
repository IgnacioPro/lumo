package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/ignacio/lumo/internal/ai"
	"github.com/ignacio/lumo/internal/config"
)

var askCmd = &cobra.Command{
	Use:   "ask [query]",
	Short: "Ask Lumo to perform a task using natural language",
	Long: `Ask Lumo to perform a task using natural language.
Lumo will interpret your request and propose a CLI command to execute.

Examples:
  lumo ask "check cpu usage"
  lumo ask "why is nginx failing"
  lumo ask "show me network stats"
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.Join(args, " ")
		if err := runAsk(cmd, query); err != nil {
			log.Errorf("Ask failed: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(askCmd)
	askCmd.Flags().BoolP("yes", "y", false, "Automatically execute the proposed command without confirmation")
}

// validateProposedCommand performs comprehensive safety checks on the AI-generated command
func validateProposedCommand(command string) error {
	// Safety Check 1: Must start with "lumo"
	if !strings.HasPrefix(command, "lumo ") {
		return fmt.Errorf("AI proposed an unsafe or invalid command (must start with 'lumo'): %s", command)
	}

	// Safety Check 2: Block recursive ask commands
	parts := strings.Fields(command)
	if len(parts) >= 2 && parts[1] == "ask" {
		return fmt.Errorf("recursive 'lumo ask' commands are not allowed for security reasons")
	}

	// Safety Check 3: Comprehensive shell operator blocking
	// Aligned with internal/ssh/session.go:430 sanitizeWorkingDir
	// Block shell operators, subshells, wildcards, and control characters
	dangerousChars := ";|&$`<>(){}[]!*?~\n\r"
	if strings.ContainsAny(command, dangerousChars) {
		return fmt.Errorf("AI proposed a command with unsafe shell characters: %s", command)
	}

	// Safety Check 4: Null bytes
	if strings.Contains(command, "\x00") {
		return fmt.Errorf("AI proposed a command with null bytes")
	}

	return nil
}

func runAsk(cmd *cobra.Command, query string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Warnf("Failed to load config, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	if !cfg.AI.Enabled {
		return fmt.Errorf("AI is disabled in configuration. Please enable it to use 'lumo ask'")
	}

	// Initialize AI provider
	providerType, err := ai.ParseProviderType(cfg.AI.Provider)
	if err != nil {
		return fmt.Errorf("invalid AI provider: %w", err)
	}

	providerConfig := &ai.ProviderConfig{
		Name:            string(providerType),
		APIKey:          cfg.AI.GetAPIKeyForProvider(cfg.AI.Provider),
		Model:           cfg.AI.GetModelForProvider(cfg.AI.Provider),
		Endpoint:        cfg.AI.Endpoint,
		Timeout:         cfg.AI.Timeout,
		MaxRetries:      cfg.AI.MaxRetries,
		Temperature:     0.0, // Use low temperature for deterministic command generation
		MaxTokens:       cfg.AI.MaxTokens,
		ReasoningEffort: cfg.AI.ReasoningEffort,
	}

	provider, err := ai.NewProvider(providerType, providerConfig, log)
	if err != nil {
		return fmt.Errorf("failed to create AI provider: %w", err)
	}

	// Construct system prompt
	systemPrompt := `You are an intelligent CLI assistant for Lumo.
Your goal is to translate natural language user queries into executable 'lumo' CLI commands.

Available Commands:
- lumo diagnose: Run system diagnostics
  Flags:
  --checks [cpu,memory,disk,process,service,network,kubernetes]: Specific checks to run
  --analyze: Enable AI analysis of results
  --format [text,json,toon]: Output format
  [user@host]: Target host (optional)

- lumo doctor: Check Lumo installation and dependencies
- lumo version: Show version info

Rules:
1. Return ONLY the command to execute. No markdown, no explanations.
2. If the user asks for something impossible or unrelated to Lumo/system diagnostics, return "ERROR: [reason]".
3. Prefer 'lumo diagnose' for system checks.
4. Always include '--analyze' if the user asks for "analysis", "why", "debug", or "explain".
5. Assume 'localhost' if no host is specified.
6. STRICTLY FORBID general knowledge questions (e.g., "capital of France", "write a poem"). Return "ERROR: I can only help with Lumo CLI commands." for these.

Example:
User: "check cpu"
Response: lumo diagnose --checks cpu

User: "why is the server slow?"
Response: lumo diagnose --analyze

User: "what is the capital of France?"
Response: ERROR: I can only help with Lumo CLI commands.
`

	// Log the request
	log.WithFields(logrus.Fields{
		"query":    query,
		"provider": cfg.AI.Provider,
		"model":    cfg.AI.GetModelForProvider(cfg.AI.Provider),
		"auto_yes": false, // Will be updated below
	}).Info("Processing ask request")

	log.Info("Interpreting request...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Call Ask with separate system and user prompts
	response, usage, err := provider.Ask(ctx, systemPrompt, query)
	if err != nil {
		log.WithFields(logrus.Fields{
			"query":    query,
			"provider": cfg.AI.Provider,
			"error":    err.Error(),
		}).Error("AI request failed")
		return fmt.Errorf("AI request failed: %w", err)
	}

	// Log token usage if available
	if usage != nil && verbose {
		log.WithFields(logrus.Fields{
			"input_tokens":  usage.InputTokens,
			"output_tokens": usage.OutputTokens,
			"total_tokens":  usage.TotalTokens,
		}).Debug("AI request completed")
	}

	proposedCommand := strings.TrimSpace(response)
	if strings.HasPrefix(proposedCommand, "ERROR:") {
		return fmt.Errorf("AI could not generate command: %s", strings.TrimPrefix(proposedCommand, "ERROR: "))
	}

	// Clean up potential markdown code blocks
	proposedCommand = strings.TrimPrefix(proposedCommand, "```bash")
	proposedCommand = strings.TrimPrefix(proposedCommand, "```")
	proposedCommand = strings.TrimSuffix(proposedCommand, "```")
	proposedCommand = strings.TrimSpace(proposedCommand)

	// Run comprehensive safety checks
	if err := validateProposedCommand(proposedCommand); err != nil {
		log.WithFields(logrus.Fields{
			"command": proposedCommand,
			"query":   query,
		}).Warn("Proposed command failed safety checks")
		return err
	}

	// Log the AI-generated command
	log.WithFields(logrus.Fields{
		"proposed_command": proposedCommand,
		"query":            query,
	}).Info("AI generated command proposal")

	fmt.Printf("\nProposed Command: \033[1;36m%s\033[0m\n", proposedCommand)

	// Check for auto-execute flag
	autoYes, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return fmt.Errorf("failed to get auto-execute flag: %w", err)
	}

	if !autoYes {
		fmt.Print("Execute this command? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}

		input = strings.TrimSpace(strings.ToLower(input))

		// Only proceed on explicit yes
		if input != "y" && input != "yes" {
			if input == "" {
				fmt.Println("No confirmation received. Aborted.")
			} else {
				fmt.Println("Aborted.")
			}
			log.WithFields(logrus.Fields{
				"command": proposedCommand,
				"query":   query,
			}).Info("User rejected command execution")
			return nil
		}
	}

	// Log user approval
	log.WithFields(logrus.Fields{
		"command": proposedCommand,
		"auto":    autoYes,
		"query":   query,
	}).Info("Command execution approved")

	// Check for dry-run mode
	if dryRun {
		fmt.Printf("\n[DRY RUN] Would execute: %s\n", proposedCommand)
		log.WithField("command", proposedCommand).Info("Dry-run mode: skipping execution")
		return nil
	}

	// Execute command
	fmt.Println()
	parts := strings.Fields(proposedCommand)
	if len(parts) == 0 {
		return fmt.Errorf("empty command generated")
	}

	// We execute the binary itself with the arguments
	// Skip "lumo" if it's the first argument, as we are already running lumo
	args := parts
	if len(args) > 0 && args[0] == "lumo" {
		args = args[1:]
	}

	// For safety, we re-enter the root command execution rather than spawning a subprocess
	// This ensures we use the same process context and don't rely on PATH
	rootCmd.SetArgs(args)
	err = rootCmd.Execute()

	// Log execution result
	if err != nil {
		log.WithFields(logrus.Fields{
			"command": proposedCommand,
			"status":  "failed",
			"error":   err.Error(),
		}).Error("AI-generated command execution failed")
	} else {
		log.WithFields(logrus.Fields{
			"command": proposedCommand,
			"status":  "completed",
		}).Info("AI-generated command executed successfully")
	}

	return err
}
