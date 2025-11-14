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
	Timeout       time.Duration `mapstructure:"timeout"`
	Port          int           `mapstructure:"port"`
	KeepAlive     time.Duration `mapstructure:"keepalive"`
	MaxRetries    int           `mapstructure:"max_retries"`
	RetryInterval time.Duration `mapstructure:"retry_interval"`
}

// AIConfig contains AI provider settings
type AIConfig struct {
	Provider string            `mapstructure:"provider"`
	Models   map[string]string `mapstructure:"models"`
	Timeout  time.Duration     `mapstructure:"timeout"`
	MaxRetries int             `mapstructure:"max_retries"`
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
			Timeout:       30 * time.Second,
			Port:          22,
			KeepAlive:     30 * time.Second,
			MaxRetries:    3,
			RetryInterval: 5 * time.Second,
		},
		AI: AIConfig{
			Provider: "anthropic",
			Models: map[string]string{
				"anthropic": "claude-sonnet-4-5-20250929",
				"openai":    "gpt-4",
			},
			Timeout:    60 * time.Second,
			MaxRetries: 3,
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
	validProviders := map[string]bool{
		"anthropic": true,
		"openai":    true,
		"local":     true,
	}
	if !validProviders[c.AI.Provider] {
		return fmt.Errorf("unsupported AI provider: %s", c.AI.Provider)
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
