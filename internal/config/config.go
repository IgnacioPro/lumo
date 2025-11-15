package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete Lumo configuration
type Config struct {
	SSH     SSHConfig     `mapstructure:"ssh"`
	AI      AIConfig      `mapstructure:"ai"`
	Logging LoggingConfig `mapstructure:"logging"`
	API     APIConfig     `mapstructure:"api"`
}

// SSHConfig contains SSH connection settings
type SSHConfig struct {
	Timeout               time.Duration `mapstructure:"timeout"`
	Port                  int           `mapstructure:"port"`
	KeepAlive             time.Duration `mapstructure:"keepalive"`
	MaxRetries            int           `mapstructure:"max_retries"`
	RetryInterval         time.Duration `mapstructure:"retry_interval"`
	KnownHostsPath        string        `mapstructure:"known_hosts_path"`
	StrictHostKeyChecking bool          `mapstructure:"strict_host_key_checking"`
	PreferredAuthMethods  []string      `mapstructure:"preferred_auth_methods"`
	CommandTimeout        time.Duration `mapstructure:"command_timeout"`
	DefaultKeyPath        string        `mapstructure:"default_key_path"`
}

// AIConfig contains AI provider settings
type AIConfig struct {
	Provider    string            `mapstructure:"provider"`
	APIKey      string            `mapstructure:"api_key"`
	Model       string            `mapstructure:"model"`
	Models      map[string]string `mapstructure:"models"`   // Per-provider model overrides
	Endpoint    string            `mapstructure:"endpoint"` // Custom endpoint (optional)
	Timeout     time.Duration     `mapstructure:"timeout"`
	MaxRetries  int               `mapstructure:"max_retries"`
	Temperature float64           `mapstructure:"temperature"` // 0.0-1.0 (default 1.0)
	MaxTokens   int               `mapstructure:"max_tokens"`  // Maximum response tokens
	Enabled     bool              `mapstructure:"enabled"`     // Enable/disable AI analysis
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// APIConfig contains API server settings
type APIConfig struct {
	Port           int           `mapstructure:"port"`
	Host           string        `mapstructure:"host"`
	TLS            bool          `mapstructure:"tls"`
	CertFile       string        `mapstructure:"cert_file"`
	KeyFile        string        `mapstructure:"key_file"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	MaxConnections int           `mapstructure:"max_connections"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		SSH: SSHConfig{
			Timeout:               30 * time.Second,
			Port:                  22,
			KeepAlive:             30 * time.Second,
			MaxRetries:            3,
			RetryInterval:         5 * time.Second,
			KnownHostsPath:        "", // Will use default ~/.ssh/known_hosts
			StrictHostKeyChecking: false,
			PreferredAuthMethods:  []string{"agent", "key", "password", "interactive"},
			CommandTimeout:        5 * time.Minute,
			DefaultKeyPath:        "", // Will auto-discover in ~/.ssh/
		},
		AI: AIConfig{
			Provider: "anthropic",
			APIKey:   "", // Set via LUMO_AI_API_KEY environment variable
			Model:    "", // Will use provider-specific default
			Models: map[string]string{
				"anthropic": "claude-sonnet-4-5-20250929",
				"openai":    "gpt-4-turbo-preview",
				"ollama":    "llama3.1:8b",
				"gemini":    "gemini-2.0-flash-exp",
			},
			Endpoint:    "", // Will use provider-specific default
			Timeout:     120 * time.Second,
			MaxRetries:  3,
			Temperature: 1.0,
			MaxTokens:   4096,
			Enabled:     true,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
		API: APIConfig{
			Port:           8080,
			Host:           "0.0.0.0",
			TLS:            false,
			ReadTimeout:    15 * time.Second,
			WriteTimeout:   15 * time.Second,
			MaxConnections: 100,
		},
	}
}

// Load reads the configuration from viper and returns a Config struct
func Load() (*Config, error) {
	cfg := DefaultConfig()

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// SSH validation
	if c.SSH.Port < 1 || c.SSH.Port > 65535 {
		return fmt.Errorf("invalid SSH port: %d", c.SSH.Port)
	}
	if c.SSH.Timeout < 0 {
		return fmt.Errorf("SSH timeout must be positive")
	}

	// AI validation
	if c.AI.Enabled {
		validProviders := map[string]bool{
			"anthropic": true,
			"openai":    true,
			"ollama":    true,
			"gemini":    true,
			"local":     true,  // Alias for ollama
			"google":    true,  // Alias for gemini
		}
		if !validProviders[c.AI.Provider] {
			return fmt.Errorf("unsupported AI provider: %s (supported: anthropic, openai, ollama, gemini)", c.AI.Provider)
		}

		// Validate temperature range
		if c.AI.Temperature < 0 || c.AI.Temperature > 1.0 {
			return fmt.Errorf("AI temperature must be between 0.0 and 1.0, got: %.2f", c.AI.Temperature)
		}

		// Validate max tokens
		if c.AI.MaxTokens < 1 {
			return fmt.Errorf("AI max_tokens must be positive, got: %d", c.AI.MaxTokens)
		}

		// Validate API key for cloud providers
		if (c.AI.Provider == "anthropic" || c.AI.Provider == "openai") && c.AI.APIKey == "" {
			return fmt.Errorf("AI provider %s requires api_key (set via LUMO_AI_API_KEY environment variable)", c.AI.Provider)
		}
	}

	// Logging validation
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	validFormats := map[string]bool{
		"text": true,
		"json": true,
	}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}

	// API validation
	if c.API.Port < 1 || c.API.Port > 65535 {
		return fmt.Errorf("invalid API port: %d", c.API.Port)
	}
	if c.API.TLS && (c.API.CertFile == "" || c.API.KeyFile == "") {
		return fmt.Errorf("TLS enabled but cert_file or key_file not specified")
	}

	return nil
}

// GetModelForProvider returns the model to use for a specific provider.
// It checks the Model field first, then falls back to the Models map, then provider defaults.
func (c *AIConfig) GetModelForProvider(provider string) string {
	// Use explicitly set model if available
	if c.Model != "" {
		return c.Model
	}

	// Fall back to provider-specific model from map
	if model, ok := c.Models[provider]; ok && model != "" {
		return model
	}

	// Will use provider's default model
	return ""
}
