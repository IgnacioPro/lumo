package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	initForce  bool
	initPreset string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Lumo configuration interactively",
	Long: `Initialize Lumo configuration with an interactive wizard.

This command helps you set up Lumo by walking through essential
configuration options. It will:
  - Choose an AI provider and set up credentials
  - Configure basic diagnostic options
  - Set up logging preferences
  - Create a minimal, working configuration file

The configuration will be saved to $HOME/.lumo/config.yaml by default.`,
	Example: `  # Interactive setup
  lumo init

  # Use a preset configuration
  lumo init --preset local

  # Force overwrite existing config
  lumo init --force

Available presets:
  local      - Local-only diagnostics (no SSH, no agent)
  agent      - Agent mode configuration
  production - Production-ready setup with all features`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "overwrite existing configuration")
	initCmd.Flags().StringVarP(&initPreset, "preset", "p", "", "use a preset configuration (local|agent|production)")
}

type InitConfig struct {
	// Essential settings
	AIProvider   string
	AIAPIKey     string
	LogLevel     string
	OutputFormat string

	// Optional features
	EnableSSH           bool
	EnableAgent         bool
	EnableNotifications bool
}

func runInit(cmd *cobra.Command, args []string) error {
	// Print banner
	fmt.Println()
	fmt.Println("╦  ╦ ╦╔╦╗╔═╗")
	fmt.Println("║  ║ ║║║║║ ║")
	fmt.Println("╩═╝╚═╝╩ ╩╚═╝")
	fmt.Println()
	fmt.Println("Welcome to Lumo Setup Wizard!")
	fmt.Println("==============================")
	fmt.Println()

	// Determine config path
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to determine config path: %w", err)
	}

	// Check if config exists
	if !initForce && fileExists(configPath) {
		log.Warnf("Configuration file already exists: %s", configPath)
		overwrite := false
		prompt := &survey.Confirm{
			Message: "Do you want to overwrite it?",
			Default: false,
		}
		if err := survey.AskOne(prompt, &overwrite); err != nil {
			return err
		}
		if !overwrite {
			fmt.Println("\nSetup cancelled. Use --force to overwrite existing configuration.")
			return nil
		}
	}

	// Use preset or interactive setup
	var config InitConfig
	if initPreset != "" {
		config, err = loadPreset(initPreset)
		if err != nil {
			return fmt.Errorf("failed to load preset: %w", err)
		}
		fmt.Printf("Using preset: %s\n\n", initPreset)
	} else {
		config, err = interactiveSetup()
		if err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}
	}

	// Generate and save configuration
	yamlConfig := generateYAMLConfig(config)

	// Create config directory
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config file
	if err := os.WriteFile(configPath, []byte(yamlConfig), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Success message
	fmt.Println()
	fmt.Println("✅ Configuration saved successfully!")
	fmt.Printf("   Location: %s\n", configPath)
	fmt.Println()

	// Show next steps
	showNextSteps(config)

	return nil
}

func getConfigPath() (string, error) {
	if cfgFile != "" {
		return cfgFile, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".lumo", "config.yaml"), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func interactiveSetup() (InitConfig, error) {
	var config InitConfig

	fmt.Println("Let's set up your Lumo configuration!")

	// AI Provider selection
	aiProvider := ""
	promptProvider := &survey.Select{
		Message: "Choose an AI provider:",
		Options: []string{
			"anthropic (Claude - Recommended)",
			"openai (GPT)",
			"ollama (Local)",
			"gemini (Google)",
			"openrouter (Multi-model)",
			"none (Skip AI features)",
		},
		Default: "anthropic (Claude - Recommended)",
	}
	if err := survey.AskOne(promptProvider, &aiProvider); err != nil {
		return config, err
	}
	config.AIProvider = strings.Split(aiProvider, " ")[0]

	// API Key (if needed)
	if config.AIProvider != "none" && config.AIProvider != "ollama" {
		fmt.Println()
		fmt.Printf("You selected: %s\n", config.AIProvider)
		fmt.Println("You'll need an API key. Get one from:")

		switch config.AIProvider {
		case "anthropic":
			fmt.Println("  → https://console.anthropic.com/")
		case "openai":
			fmt.Println("  → https://platform.openai.com/api-keys")
		case "gemini":
			fmt.Println("  → https://makersuite.google.com/app/apikey")
		case "openrouter":
			fmt.Println("  → https://openrouter.ai/keys")
		}
		fmt.Println()

		apiKey := ""
		promptKey := &survey.Password{
			Message: fmt.Sprintf("Enter your %s API key:", strings.ToUpper(config.AIProvider)),
		}
		if err := survey.AskOne(promptKey, &apiKey, survey.WithValidator(survey.Required)); err != nil {
			return config, err
		}
		config.AIAPIKey = strings.TrimSpace(apiKey)

		// Validate API key format
		if !validateAPIKey(config.AIProvider, config.AIAPIKey) {
			log.Warn("API key format looks incorrect, but continuing anyway...")
		}
	}

	// Log level
	fmt.Println()
	logLevel := ""
	promptLog := &survey.Select{
		Message: "Choose logging level:",
		Options: []string{"info", "debug", "warn", "error"},
		Default: "info",
	}
	if err := survey.AskOne(promptLog, &logLevel); err != nil {
		return config, err
	}
	config.LogLevel = logLevel

	// Output format
	fmt.Println()
	outputFormat := ""
	promptFormat := &survey.Select{
		Message: "Choose default output format:",
		Options: []string{
			"text (Human-readable)",
			"json (Machine-readable)",
			"toon (AI-optimized, 30-60% token reduction)",
		},
		Default: "text (Human-readable)",
	}
	if err := survey.AskOne(promptFormat, &outputFormat); err != nil {
		return config, err
	}
	config.OutputFormat = strings.Split(outputFormat, " ")[0]

	// Optional features
	fmt.Println()
	fmt.Println("Optional Features:")
	fmt.Println("------------------")

	promptSSH := &survey.Confirm{
		Message: "Enable SSH for remote diagnostics?",
		Default: true,
	}
	if err := survey.AskOne(promptSSH, &config.EnableSSH); err != nil {
		return config, err
	}

	promptAgent := &survey.Confirm{
		Message: "Configure as agent mode? (for K8s/VM deployments)",
		Default: false,
	}
	if err := survey.AskOne(promptAgent, &config.EnableAgent); err != nil {
		return config, err
	}

	promptNotifications := &survey.Confirm{
		Message: "Enable notifications? (Slack, Telegram, etc.)",
		Default: false,
	}
	if err := survey.AskOne(promptNotifications, &config.EnableNotifications); err != nil {
		return config, err
	}

	return config, nil
}

func validateAPIKey(provider, key string) bool {
	if key == "" {
		return false
	}

	switch provider {
	case "anthropic":
		return strings.HasPrefix(key, "sk-ant-")
	case "openai":
		return strings.HasPrefix(key, "sk-")
	case "openrouter":
		return strings.HasPrefix(key, "sk-or-")
	default:
		return len(key) > 10 // Basic length check
	}
}

func loadPreset(preset string) (InitConfig, error) {
	presets := map[string]InitConfig{
		"local": {
			AIProvider:          "anthropic",
			LogLevel:            "info",
			OutputFormat:        "text",
			EnableSSH:           false,
			EnableAgent:         false,
			EnableNotifications: false,
		},
		"agent": {
			AIProvider:          "anthropic",
			LogLevel:            "info",
			OutputFormat:        "toon",
			EnableSSH:           false,
			EnableAgent:         true,
			EnableNotifications: true,
		},
		"production": {
			AIProvider:          "anthropic",
			LogLevel:            "warn",
			OutputFormat:        "json",
			EnableSSH:           true,
			EnableAgent:         true,
			EnableNotifications: true,
		},
	}

	config, ok := presets[preset]
	if !ok {
		return InitConfig{}, fmt.Errorf("unknown preset: %s (available: local, agent, production)", preset)
	}

	// Still need API key for presets
	if config.AIProvider != "none" && config.AIProvider != "ollama" {
		fmt.Printf("Preset '%s' requires an API key for %s\n", preset, config.AIProvider)
		apiKey := ""
		prompt := &survey.Password{
			Message: fmt.Sprintf("Enter your %s API key:", strings.ToUpper(config.AIProvider)),
		}
		if err := survey.AskOne(prompt, &apiKey, survey.WithValidator(survey.Required)); err != nil {
			return config, err
		}
		config.AIAPIKey = strings.TrimSpace(apiKey)
	}

	return config, nil
}

func generateYAMLConfig(config InitConfig) string {
	// Create a structured config
	type AIConfig struct {
		Provider string `yaml:"provider"`
		Model    string `yaml:"model,omitempty"`
	}

	type LogConfig struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	}

	type DiagConfig struct {
		OutputFormat  string   `yaml:"output_format"`
		EnabledChecks []string `yaml:"enabled_checks"`
	}

	type AgentConfig struct {
		Mode        string `yaml:"mode,omitempty"`
		Schedule    string `yaml:"schedule,omitempty"`
		APIEndpoint string `yaml:"api_endpoint,omitempty"`
	}

	type NotifConfig struct {
		Enabled   bool     `yaml:"enabled"`
		Providers []string `yaml:"providers,omitempty"`
	}

	type Config struct {
		AI            AIConfig     `yaml:"ai"`
		Logging       LogConfig    `yaml:"logging"`
		Diagnostics   DiagConfig   `yaml:"diagnostics"`
		Agent         *AgentConfig `yaml:"agent,omitempty"`
		Notifications *NotifConfig `yaml:"notifications,omitempty"`
	}

	// Build config
	cfg := Config{
		AI: AIConfig{
			Provider: config.AIProvider,
		},
		Logging: LogConfig{
			Level:  config.LogLevel,
			Format: "text",
		},
		Diagnostics: DiagConfig{
			OutputFormat:  config.OutputFormat,
			EnabledChecks: []string{"cpu", "memory", "disk", "process", "service", "network"},
		},
	}

	// Add model default for known providers
	switch config.AIProvider {
	case "anthropic":
		cfg.AI.Model = "claude-3-5-sonnet-20241022"
	case "openai":
		cfg.AI.Model = "gpt-4o"
	case "gemini":
		cfg.AI.Model = "gemini-2.0-flash-exp"
	case "ollama":
		cfg.AI.Model = "llama3.2"
	}

	// Add agent config if enabled
	if config.EnableAgent {
		cfg.Agent = &AgentConfig{
			Mode:     "hybrid",
			Schedule: "*/5 * * * *",
		}
	}

	// Add notifications config if enabled
	if config.EnableNotifications {
		cfg.Notifications = &NotifConfig{
			Enabled:   true,
			Providers: []string{},
		}
	}

	// Marshal to YAML
	data, _ := yaml.Marshal(&cfg)

	// Add header comment
	header := `# Lumo Configuration File
# Generated by: lumo init
# Documentation: https://github.com/ignacio/lumo
#
# IMPORTANT: API keys should be set via environment variables for security
# Example: export LUMO_` + strings.ToUpper(config.AIProvider) + `_API_KEY=your-key-here
#
# You can also use the generic LUMO_AI_API_KEY for any provider
# For more options, see: configs/config.example.yaml
`

	// Add footer with environment variable instructions
	footer := `
# Environment Variables (RECOMMENDED)
# -----------------------------------
# Set your API key securely via environment variable:
#   export LUMO_` + strings.ToUpper(config.AIProvider) + `_API_KEY=` + getMaskedKey(config.AIAPIKey) + `
#
# Other useful environment variables:
#   LUMO_AI_PROVIDER           - Override AI provider
#   LUMO_LOG_LEVEL             - Override log level
#   LUMO_DIAGNOSTICS_FORMAT    - Override output format
#
# For a complete list of options and examples, see:
#   https://github.com/ignacio/lumo/blob/main/configs/config.example.yaml
`

	return header + "\n" + string(data) + footer
}

func getMaskedKey(key string) string {
	if key == "" {
		return "your-api-key-here"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func showNextSteps(config InitConfig) {
	fmt.Println("Next Steps:")
	fmt.Println("-----------")

	// Set environment variable
	if config.AIAPIKey != "" {
		envVar := fmt.Sprintf("LUMO_%s_API_KEY", strings.ToUpper(config.AIProvider))
		fmt.Printf("1. Set your API key (IMPORTANT):\n")
		fmt.Printf("   export %s='%s'\n", envVar, config.AIAPIKey)
		fmt.Println()
		fmt.Println("   Or add to your shell config (~/.bashrc or ~/.zshrc):")
		fmt.Printf("   echo 'export %s=%s' >> ~/.bashrc\n", envVar, getMaskedKey(config.AIAPIKey))
		fmt.Println()
	}

	// Try local diagnostic
	fmt.Println("2. Try a local diagnostic:")
	fmt.Println("   lumo diagnose localhost")
	fmt.Println()

	// Try with AI analysis
	if config.AIProvider != "none" {
		fmt.Println("3. Try with AI analysis:")
		fmt.Println("   lumo diagnose localhost --analyze")
		fmt.Println()
	}

	// SSH example
	if config.EnableSSH {
		fmt.Println("4. Diagnose a remote server:")
		fmt.Println("   lumo diagnose user@server.example.com")
		fmt.Println()
	}

	// Agent mode
	if config.EnableAgent {
		fmt.Println("4. Deploy agent (see deployment docs):")
		fmt.Println("   kubectl apply -f deployments/kubernetes/")
		fmt.Println("   # or")
		fmt.Println("   ./deployments/systemd/install.sh")
		fmt.Println()
	}

	// Documentation
	fmt.Println("📚 Documentation & Help:")
	fmt.Println("   lumo --help              - Command help")
	fmt.Println("   lumo examples            - Usage examples")
	fmt.Println("   https://github.com/ignacio/lumo - Full documentation")
	fmt.Println()

	// Tip
	fmt.Println("💡 Tip: You can modify the config file at any time")
	fmt.Println("   See configs/config.example.yaml for all available options")
	fmt.Println()
}
