package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete Lumo configuration
type Config struct {
	SSH         SSHConfig         `mapstructure:"ssh"`
	AI          AIConfig          `mapstructure:"ai"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	API         APIConfig         `mapstructure:"api"`
	Diagnostics DiagnosticsConfig `mapstructure:"diagnostics"`
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
	Provider        string            `mapstructure:"provider"`
	APIKey          string            `mapstructure:"api_key"`
	Model           string            `mapstructure:"model"`
	Models          map[string]string `mapstructure:"models"`   // Per-provider model overrides
	Endpoint        string            `mapstructure:"endpoint"` // Custom endpoint (optional)
	Timeout         time.Duration     `mapstructure:"timeout"`
	MaxRetries      int               `mapstructure:"max_retries"`
	Temperature     float64           `mapstructure:"temperature"`      // 0.0-1.0 (default 1.0)
	MaxTokens       int               `mapstructure:"max_tokens"`       // Maximum response tokens
	ReasoningEffort string            `mapstructure:"reasoning_effort"` // For OpenAI reasoning models: "low", "medium", "high"
	Enabled         bool              `mapstructure:"enabled"`          // Enable/disable AI analysis
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

// DiagnosticsConfig contains diagnostic settings
type DiagnosticsConfig struct {
	Network    NetworkConfig    `mapstructure:"network"`
	Security   SecurityConfig   `mapstructure:"security"`
	Kubernetes KubernetesConfig `mapstructure:"kubernetes"`
}

// NetworkConfig contains network diagnostic settings
type NetworkConfig struct {
	Targets []NetworkTarget `mapstructure:"targets"`
}

// NetworkTarget represents a network endpoint to test connectivity to
type NetworkTarget struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`     // Port number (0 for ICMP-only)
	Protocol string `mapstructure:"protocol"` // "tcp" or "icmp"
}

// SecurityConfig contains security diagnostic settings
type SecurityConfig struct {
	PortCheck        PortCheckConfig        `mapstructure:"port_check"`
	AuthFailureCheck AuthFailureCheckConfig `mapstructure:"auth_failure_check"`
}

// PortCheckConfig contains port scanning configuration
type PortCheckConfig struct {
	WhitelistedPorts []int `mapstructure:"whitelisted_ports"`
}

// AuthFailureCheckConfig contains auth failure checking configuration
type AuthFailureCheckConfig struct {
	LookbackHours    int `mapstructure:"lookback_hours"`
	FailureThreshold int `mapstructure:"failure_threshold"`
}

// KubernetesConfig contains Kubernetes diagnostic settings
type KubernetesConfig struct {
	Enabled           bool     `mapstructure:"enabled"`             // Enable Kubernetes diagnostics
	KubeconfigPath    string   `mapstructure:"kubeconfig_path"`     // Path to kubeconfig file (empty = default)
	Context           string   `mapstructure:"context"`             // Kubernetes context to use (empty = current)
	Namespaces        []string `mapstructure:"namespaces"`          // Namespaces to check (empty = all)
	CheckNodes        bool     `mapstructure:"check_nodes"`         // Check node health
	CheckPods         bool     `mapstructure:"check_pods"`          // Check pod status
	CheckDeployments  bool     `mapstructure:"check_deployments"`   // Check deployment health
	CheckStatefulSets bool     `mapstructure:"check_statefulsets"`  // Check StatefulSet health
	CheckDaemonSets   bool     `mapstructure:"check_daemonsets"`    // Check DaemonSet health
	CheckServices     bool     `mapstructure:"check_services"`      // Check service endpoints
	CheckPVCs         bool     `mapstructure:"check_pvcs"`          // Check PersistentVolumeClaims
	CheckEvents       bool     `mapstructure:"check_events"`        // Check recent events
	EventLookbackMins int      `mapstructure:"event_lookback_mins"` // How far back to look for events
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
				"anthropic":  "claude-sonnet-4-5-20250929",
				"openai":     "gpt-4-turbo-preview",
				"ollama":     "llama3.1:8b",
				"gemini":     "gemini-2.0-flash-exp",
				"openrouter": "anthropic/claude-sonnet-4.5",
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
		Diagnostics: DiagnosticsConfig{
			Network: NetworkConfig{
				Targets: []NetworkTarget{
					{
						Host:     "8.8.8.8",
						Port:     0,
						Protocol: "icmp",
					},
					{
						Host:     "google.com",
						Port:     0,
						Protocol: "icmp",
					},
				},
			},
			Security: SecurityConfig{
				PortCheck: PortCheckConfig{
					WhitelistedPorts: []int{22, 80, 443, 3000, 8080},
				},
				AuthFailureCheck: AuthFailureCheckConfig{
					LookbackHours:    1,
					FailureThreshold: 20,
				},
			},
			Kubernetes: KubernetesConfig{
				Enabled:           false,      // Disabled by default, enable via config or --checks kubernetes
				KubeconfigPath:    "",         // Use default ~/.kube/config
				Context:           "",         // Use current context
				Namespaces:        []string{}, // All namespaces
				CheckNodes:        true,
				CheckPods:         true,
				CheckDeployments:  true,
				CheckStatefulSets: true,
				CheckDaemonSets:   true,
				CheckServices:     true,
				CheckPVCs:         true,
				CheckEvents:       true,
				EventLookbackMins: 30, // Look back 30 minutes for events
			},
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
			"local":     true, // Alias for ollama
			"google":    true, // Alias for gemini
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

		// Validate reasoning effort (if specified)
		if c.AI.ReasoningEffort != "" {
			validEfforts := map[string]bool{
				"low":    true,
				"medium": true,
				"high":   true,
			}
			if !validEfforts[c.AI.ReasoningEffort] {
				return fmt.Errorf("AI reasoning_effort must be 'low', 'medium', or 'high', got: %s", c.AI.ReasoningEffort)
			}
		}

		// Validate API key for cloud providers
		needsAPIKey := c.AI.Provider == "anthropic" || c.AI.Provider == "openai" || c.AI.Provider == "gemini" || c.AI.Provider == "google" || c.AI.Provider == "openrouter"
		if needsAPIKey && c.AI.GetAPIKeyForProvider(c.AI.Provider) == "" {
			providerName := c.AI.Provider
			if providerName == "google" {
				providerName = "gemini"
			}
			envVar := fmt.Sprintf("LUMO_%s_API_KEY", map[string]string{
				"anthropic": "ANTHROPIC",
				"openai":    "OPENAI",
				"gemini":    "GEMINI",
			}[providerName])
			return fmt.Errorf("AI provider %s requires API key (set via %s or LUMO_AI_API_KEY environment variable)", c.AI.Provider, envVar)
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

	// Diagnostics validation
	for i, target := range c.Diagnostics.Network.Targets {
		if target.Host == "" {
			return fmt.Errorf("diagnostics.network.targets[%d]: host cannot be empty", i)
		}

		// Validate protocol
		validProtocols := map[string]bool{
			"tcp":  true,
			"icmp": true,
			"":     true, // Empty defaults to icmp
		}
		if !validProtocols[target.Protocol] {
			return fmt.Errorf("diagnostics.network.targets[%d]: invalid protocol %q (must be tcp or icmp)", i, target.Protocol)
		}

		// Validate port
		if target.Port < 0 || target.Port > 65535 {
			return fmt.Errorf("diagnostics.network.targets[%d]: invalid port %d (must be 0-65535)", i, target.Port)
		}

		// TCP targets must have a port
		if target.Protocol == "tcp" && target.Port == 0 {
			return fmt.Errorf("diagnostics.network.targets[%d]: TCP targets must specify a port", i)
		}
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

// GetAPIKeyForProvider returns the API key for a specific provider.
// It checks provider-specific environment variables first, then falls back to generic LUMO_AI_API_KEY.
// Provider-specific environment variables:
//   - LUMO_ANTHROPIC_API_KEY
//   - LUMO_OPENAI_API_KEY
//   - LUMO_GEMINI_API_KEY
//   - LUMO_OLLAMA_API_KEY (optional, usually not needed)
//   - LUMO_OPENROUTER_API_KEY
func (c *AIConfig) GetAPIKeyForProvider(provider string) string {
	// Normalize provider name (handle aliases)
	normalizedProvider := provider
	switch provider {
	case "google":
		normalizedProvider = "gemini"
	case "local":
		normalizedProvider = "ollama"
	}

	// Check provider-specific environment variable first
	// Viper expects key names without the prefix when using SetEnvPrefix("LUMO")
	providerEnvVarKey := map[string]string{
		"anthropic":  "anthropic_api_key",
		"openai":     "openai_api_key",
		"gemini":     "gemini_api_key",
		"ollama":     "ollama_api_key",
		"openrouter": "openrouter_api_key",
	}[normalizedProvider]

	if key := viper.GetString(providerEnvVarKey); key != "" {
		return key
	}

	// Fall back to generic LUMO_AI_API_KEY
	if key := viper.GetString("ai_api_key"); key != "" {
		return key
	}

	// Finally, fall back to config file value
	if c.APIKey != "" {
		return c.APIKey
	}

	return viper.GetString("ai.api_key")
}
