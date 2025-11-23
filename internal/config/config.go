package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete Lumo configuration
type Config struct {
	Environment   string              `mapstructure:"environment"` // Deployment environment: development, staging, production
	SSH           SSHConfig           `mapstructure:"ssh"`
	AI            AIConfig            `mapstructure:"ai"`
	Logging       LoggingConfig       `mapstructure:"logging"`
	API           APIConfig           `mapstructure:"api"`
	Diagnostics   DiagnosticsConfig   `mapstructure:"diagnostics"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Cache         CacheConfig         `mapstructure:"cache"`
	Agent         AgentConfig         `mapstructure:"agent"`
	Notifications NotificationsConfig `mapstructure:"notifications"`
	RAG           RAGConfig           `mapstructure:"rag"`
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
	JWTSecret      string        `mapstructure:"jwt_secret"`     // JWT signing secret (prefer LUMO_API_JWT_SECRET env var)
	JWTExpiration  time.Duration `mapstructure:"jwt_expiration"` // JWT token expiration (default: 24h)
	JWTIssuer      string        `mapstructure:"jwt_issuer"`     // JWT issuer (default: lumo-api)

	// Rate limiting settings
	RateLimitEnabled         bool     `mapstructure:"rate_limit_enabled"`           // Enable rate limiting
	RateLimitRequestsPerMin  int      `mapstructure:"rate_limit_requests_per_min"`  // Per-IP rate limit (requests per minute)
	RateLimitRequestsPerHour int      `mapstructure:"rate_limit_requests_per_hour"` // Per-user rate limit (requests per hour)
	RateLimitBurstSize       int      `mapstructure:"rate_limit_burst_size"`        // Burst size for rate limiter
	AllowedOrigins           []string `mapstructure:"allowed_origins"`              // CORS allowed origins
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

// DatabaseConfig contains database connection settings
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Name            string        `mapstructure:"name"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"` // Prefer LUMO_DATABASE_PASSWORD env var
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxConnections  int           `mapstructure:"max_connections"`
	MaxIdle         int           `mapstructure:"max_idle"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// CacheConfig contains cache (Redis) settings
type CacheConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	RedisURL   string        `mapstructure:"redis_url"`
	Password   string        `mapstructure:"password"` // Prefer LUMO_CACHE_PASSWORD env var
	MaxRetries int           `mapstructure:"max_retries"`
	PoolSize   int           `mapstructure:"pool_size"`
	TTL        time.Duration `mapstructure:"ttl"`
}

// AgentConfig contains agent daemon settings
type AgentConfig struct {
	Mode             string        `mapstructure:"mode"`               // scheduled|on-demand|continuous|hybrid
	Schedule         string        `mapstructure:"schedule"`           // Cron expression for scheduled mode
	APIEndpoint      string        `mapstructure:"api_endpoint"`       // Lumo API server endpoint
	Token            string        `mapstructure:"token"`              // JWT authentication token (prefer env var)
	TLSEnabled       bool          `mapstructure:"tls_enabled"`        // Enable TLS for API communication
	TLSInsecure      bool          `mapstructure:"tls_insecure"`       // Skip TLS verification (dev only)
	TLSCertFile      string        `mapstructure:"tls_cert_file"`      // Client certificate for mTLS
	TLSKeyFile       string        `mapstructure:"tls_key_file"`       // Client private key for mTLS
	TLSCAFile        string        `mapstructure:"tls_ca_file"`        // CA certificate for server verification
	EnabledChecks    []string      `mapstructure:"enabled_checks"`     // List of enabled diagnostic checks
	ReportFormat     string        `mapstructure:"report_format"`      // Report format (text|json|toon)
	OfflineMode      bool          `mapstructure:"offline_mode"`       // Continue without API availability
	CachePath        string        `mapstructure:"cache_path"`         // Path for local cache storage
	CacheMaxSize     int64         `mapstructure:"cache_max_size"`     // Maximum cache size in bytes
	CacheTTL         time.Duration `mapstructure:"cache_ttl"`          // Cache entry TTL
	HealthCheckPort  int           `mapstructure:"health_check_port"`  // Port for health check endpoints
	MetricsPort      int           `mapstructure:"metrics_port"`       // Port for Prometheus metrics
	HeartbeatSeconds int           `mapstructure:"heartbeat_seconds"`  // Heartbeat interval in seconds
	RetryMaxAttempts int           `mapstructure:"retry_max_attempts"` // Max retry attempts for API calls
	RetryBaseDelay   time.Duration `mapstructure:"retry_base_delay"`   // Base delay for exponential backoff

	// Kubernetes-specific settings
	Kubernetes KubernetesAgentConfig `mapstructure:"kubernetes"`
}

// KubernetesAgentConfig contains Kubernetes-specific agent settings
type KubernetesAgentConfig struct {
	Enabled   bool   `mapstructure:"enabled"`   // Enable Kubernetes-specific features
	Scope     string `mapstructure:"scope"`     // node|cluster - determines agent type
	Cluster   string `mapstructure:"cluster"`   // Kubernetes cluster name
	Namespace string `mapstructure:"namespace"` // Agent namespace
	NodeName  string `mapstructure:"node_name"` // Node name (for DaemonSet agents)
	PodName   string `mapstructure:"pod_name"`  // Pod name
}

// NotificationsConfig contains notification system settings
type NotificationsConfig struct {
	Enabled   bool             `mapstructure:"enabled"`   // Enable notifications
	Notifiers []NotifierConfig `mapstructure:"notifiers"` // List of configured notifiers
}

// NotifierConfig contains configuration for a notification provider
type NotifierConfig struct {
	Name       string            `mapstructure:"name"`        // User-friendly name
	Type       string            `mapstructure:"type"`        // slack|telegram|webhook|email
	Enabled    bool              `mapstructure:"enabled"`     // Enable this notifier
	WebhookURL string            `mapstructure:"webhook_url"` // Slack/Webhook URL
	BotToken   string            `mapstructure:"bot_token"`   // Telegram bot token
	ChatID     string            `mapstructure:"chat_id"`     // Telegram chat ID
	Headers    map[string]string `mapstructure:"headers"`     // Custom HTTP headers (webhook)
	Method     string            `mapstructure:"method"`      // HTTP method (webhook)
	SMTPHost   string            `mapstructure:"smtp_host"`   // SMTP server host
	SMTPPort   int               `mapstructure:"smtp_port"`   // SMTP server port
	SMTPUser   string            `mapstructure:"smtp_user"`   // SMTP username
	SMTPPass   string            `mapstructure:"smtp_pass"`   // SMTP password (prefer env var)
	From       string            `mapstructure:"from"`        // Email sender
	To         []string          `mapstructure:"to"`          // Email recipients
	UseTLS     bool              `mapstructure:"use_tls"`     // Use TLS (email)
	Timeout    int               `mapstructure:"timeout"`     // Request timeout in seconds
}

// RAGConfig contains RAG (Retrieval Augmented Generation) system settings
type RAGConfig struct {
	Enabled           bool    `mapstructure:"enabled"`            // Enable RAG system
	StoragePath       string  `mapstructure:"storage_path"`       // ./data/rag/embeddings
	EmbeddingProvider string  `mapstructure:"embedding_provider"` // openai, anthropic, local
	EmbeddingModel    string  `mapstructure:"embedding_model"`    // text-embedding-3-small
	MaxDocuments      int     `mapstructure:"max_documents"`      // 10000
	SimilarityK       int     `mapstructure:"similarity_k"`       // 5 (top K results)
	MinScore          float32 `mapstructure:"min_score"`          // 0.7 (minimum similarity)
	IngestionMode     string  `mapstructure:"ingestion_mode"`     // realtime, batch, hybrid
	BatchInterval     int     `mapstructure:"batch_interval"`     // 300 seconds
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Environment: "development", // Default to development (can be overridden by config file or LUMO_ENVIRONMENT env var)
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
			Port:                     8080,
			Host:                     "0.0.0.0",
			TLS:                      false,
			ReadTimeout:              15 * time.Second,
			WriteTimeout:             15 * time.Second,
			MaxConnections:           100,
			RateLimitEnabled:         true, // Enable by default for security
			RateLimitRequestsPerMin:  60,   // 60 req/min per IP (1 req/sec average)
			RateLimitRequestsPerHour: 3600, // 3600 req/hour per user (1 req/sec average)
			RateLimitBurstSize:       10,   // Allow bursts of 10 requests
			AllowedOrigins:           []string{"*"},
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
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			Name:            "lumo",
			User:            "lumo",
			Password:        "lumo_dev", // Set via LUMO_DATABASE_PASSWORD env var
			SSLMode:         "disable",
			MaxConnections:  50,               // Adaptive default for medium deployments (100-500 agents)
			MaxIdle:         12,               // 25% of MaxConnections (keep warm connections)
			ConnMaxLifetime: 30 * time.Minute, // Increased from 5m to reduce connection churn
		},
		Cache: CacheConfig{
			Enabled:    false, // Disabled by default
			RedisURL:   "redis://localhost:6379/0",
			Password:   "", // Set via LUMO_CACHE_PASSWORD env var
			MaxRetries: 3,
			PoolSize:   10,
			TTL:        1 * time.Hour,
		},
		Agent: AgentConfig{
			Mode:             "hybrid",                                                           // Hybrid mode (scheduled + on-demand)
			Schedule:         "*/5 * * * *",                                                      // Every 5 minutes
			APIEndpoint:      "http://localhost:8080",                                            // Default to local API
			Token:            "",                                                                 // Set via LUMO_AGENT_TOKEN env var
			TLSEnabled:       true,                                                               // Enable TLS by default
			TLSInsecure:      false,                                                              // Verify TLS certificates
			EnabledChecks:    []string{"cpu", "memory", "disk", "process", "service", "network"}, // Core checks
			ReportFormat:     "toon",                                                             // TOON for efficiency
			OfflineMode:      true,                                                               // Continue when API unavailable
			CachePath:        "/var/lib/lumo-agent/cache",                                        // Default cache path
			CacheMaxSize:     1024 * 1024 * 1024,                                                 // 1 GB
			CacheTTL:         24 * time.Hour,                                                     // 24 hours
			HealthCheckPort:  8080,                                                               // Health check port
			MetricsPort:      9090,                                                               // Prometheus metrics port
			HeartbeatSeconds: 30,                                                                 // Heartbeat every 30 seconds
			RetryMaxAttempts: 4,                                                                  // 4 retry attempts with backoff
			RetryBaseDelay:   2 * time.Second,                                                    // Start with 2s delay
			Kubernetes: KubernetesAgentConfig{
				Enabled:   false,  // Disabled by default
				Scope:     "node", // node|cluster
				Cluster:   "",
				Namespace: "",
				NodeName:  "",
				PodName:   "",
			},
		},
		Notifications: NotificationsConfig{
			Enabled:   false,              // Disabled by default
			Notifiers: []NotifierConfig{}, // No notifiers configured by default
		},
		RAG: RAGConfig{
			Enabled:           false,                    // Disabled by default
			StoragePath:       "./data/rag/embeddings",  // Local storage path
			EmbeddingProvider: "openai",                 // OpenAI for embeddings
			EmbeddingModel:    "text-embedding-3-small", // Efficient embedding model
			MaxDocuments:      10000,                    // Maximum documents to store
			SimilarityK:       5,                        // Return top 5 matches
			MinScore:          0.7,                      // Minimum similarity threshold
			IngestionMode:     "hybrid",                 // Hybrid ingestion (realtime for critical, batch for low-priority)
			BatchInterval:     300,                      // 5 minutes batch interval
		},
	}
}

// Load reads the configuration from viper and returns a Config struct
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Configure environment variable handling
	viper.SetEnvPrefix("LUMO")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Explicitly bind keys to ensure AutomaticEnv works for them
	// This is necessary because we're using a struct for defaults, not viper.SetDefault
	viper.SetDefault("api.rate_limit_enabled", true)
	viper.SetDefault("api.rate_limit_requests_per_min", 60)
	viper.SetDefault("api.rate_limit_requests_per_hour", 3600)
	viper.SetDefault("api.rate_limit_burst_size", 10)

	// Database config binding (required for K8s deployment)
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.name", "lumo")
	viper.SetDefault("database.user", "lumo")
	viper.SetDefault("database.password", "")
	viper.SetDefault("database.sslmode", "disable")

	// AI config binding (required for K8s deployment)
	viper.SetDefault("ai.provider", "anthropic")
	viper.SetDefault("ai.enabled", true)
	viper.SetDefault("anthropic_api_key", "")
	viper.SetDefault("openai_api_key", "")
	viper.SetDefault("gemini_api_key", "")
	viper.SetDefault("ollama_api_key", "")
	viper.SetDefault("openrouter_api_key", "")
	viper.SetDefault("ai_api_key", "")

	// Agent config binding (required for K8s deployment)
	viper.SetDefault("agent.cache_path", "/var/lib/lumo-agent/cache")
	viper.SetDefault("agent.cache_max_size", 1073741824)
	viper.SetDefault("agent.cache_ttl", "24h")

	// Kubernetes agent metadata binding (for K8s deployments)
	// These map to POD_NAME, NODE_NAME, POD_NAMESPACE env vars set by K8s
	viper.SetDefault("agent.kubernetes.cluster", "")
	viper.SetDefault("agent.kubernetes.namespace", "")
	viper.SetDefault("agent.kubernetes.node_name", "")
	viper.SetDefault("agent.kubernetes.pod_name", "")

	// Kubernetes diagnostics config - bind LUMO_AGENT_KUBERNETES_* to diagnostics.kubernetes.*
	// This allows agents to use LUMO_AGENT_KUBERNETES_ENABLED instead of LUMO_DIAGNOSTICS_KUBERNETES_ENABLED
	// Note: viper.BindEnv with multiple env vars checks them in order (first found wins)
	_ = viper.BindEnv("diagnostics.kubernetes.enabled", "LUMO_AGENT_KUBERNETES_ENABLED", "LUMO_DIAGNOSTICS_KUBERNETES_ENABLED")
	viper.SetDefault("diagnostics.kubernetes.enabled", false)

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Manual parsing of comma-separated LUMO_AGENT_ENABLED_CHECKS environment variable
	// Viper's AutomaticEnv() doesn't handle comma-separated strings → slices
	if enabledChecksEnv := os.Getenv("LUMO_AGENT_ENABLED_CHECKS"); enabledChecksEnv != "" {
		cfg.Agent.EnabledChecks = strings.Split(enabledChecksEnv, ",")
		// Trim whitespace from each check name
		for i := range cfg.Agent.EnabledChecks {
			cfg.Agent.EnabledChecks[i] = strings.TrimSpace(cfg.Agent.EnabledChecks[i])
		}
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
			"anthropic":  true,
			"openai":     true,
			"ollama":     true,
			"gemini":     true,
			"openrouter": true,
			"local":      true, // Alias for ollama
			"google":     true, // Alias for gemini
		}
		if !validProviders[c.AI.Provider] {
			return fmt.Errorf("unsupported AI provider: %s (supported: anthropic, openai, ollama, gemini, openrouter)", c.AI.Provider)
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
				"anthropic":  "ANTHROPIC",
				"openai":     "OPENAI",
				"gemini":     "GEMINI",
				"openrouter": "OPENROUTER",
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

	// Database validation
	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", c.Database.Port)
	}
	if c.Database.Host == "" {
		return fmt.Errorf("database host cannot be empty")
	}
	if c.Database.Name == "" {
		return fmt.Errorf("database name cannot be empty")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database user cannot be empty")
	}
	validSSLModes := map[string]bool{
		"disable":     true,
		"require":     true,
		"verify-ca":   true,
		"verify-full": true,
	}
	if !validSSLModes[c.Database.SSLMode] {
		return fmt.Errorf("invalid database ssl_mode: %s (must be disable, require, verify-ca, or verify-full)", c.Database.SSLMode)
	}

	// Cache validation (only if enabled)
	if c.Cache.Enabled {
		if c.Cache.RedisURL == "" {
			return fmt.Errorf("cache is enabled but redis_url is empty")
		}
		if c.Cache.PoolSize < 1 {
			return fmt.Errorf("cache pool_size must be positive, got: %d", c.Cache.PoolSize)
		}
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

	// Agent validation
	if c.Agent.Mode != "" {
		validModes := map[string]bool{
			"scheduled":  true,
			"on-demand":  true,
			"continuous": true,
			"hybrid":     true,
		}
		if !validModes[c.Agent.Mode] {
			return fmt.Errorf("invalid agent mode: %s (must be scheduled, on-demand, continuous, or hybrid)", c.Agent.Mode)
		}
	}

	if c.Agent.ReportFormat != "" {
		validFormats := map[string]bool{
			"text": true,
			"json": true,
			"toon": true,
		}
		if !validFormats[c.Agent.ReportFormat] {
			return fmt.Errorf("invalid agent report format: %s (must be text, json, or toon)", c.Agent.ReportFormat)
		}
	}

	if c.Agent.HealthCheckPort < 0 || c.Agent.HealthCheckPort > 65535 {
		return fmt.Errorf("invalid agent health check port: %d", c.Agent.HealthCheckPort)
	}

	if c.Agent.MetricsPort < 0 || c.Agent.MetricsPort > 65535 {
		return fmt.Errorf("invalid agent metrics port: %d", c.Agent.MetricsPort)
	}

	if c.Agent.Kubernetes.Enabled && c.Agent.Kubernetes.Scope != "" {
		validScopes := map[string]bool{
			"node":    true,
			"cluster": true,
		}
		if !validScopes[c.Agent.Kubernetes.Scope] {
			return fmt.Errorf("invalid agent kubernetes scope: %s (must be node or cluster)", c.Agent.Kubernetes.Scope)
		}
	}

	// Notifications validation (only if enabled)
	if c.Notifications.Enabled {
		for i, notifier := range c.Notifications.Notifiers {
			// Validate notifier type
			validTypes := map[string]bool{
				"slack":    true,
				"telegram": true,
				"webhook":  true,
				"email":    true,
			}
			if !validTypes[notifier.Type] {
				return fmt.Errorf("notifications.notifiers[%d]: invalid type %s (must be slack, telegram, webhook, or email)", i, notifier.Type)
			}

			// Skip validation if notifier is disabled
			if !notifier.Enabled {
				continue
			}

			// Type-specific validation
			switch notifier.Type {
			case "slack":
				if notifier.WebhookURL == "" {
					return fmt.Errorf("notifications.notifiers[%d]: slack requires webhook_url", i)
				}
			case "telegram":
				if notifier.BotToken == "" {
					return fmt.Errorf("notifications.notifiers[%d]: telegram requires bot_token", i)
				}
				if notifier.ChatID == "" {
					return fmt.Errorf("notifications.notifiers[%d]: telegram requires chat_id", i)
				}
			case "webhook":
				if notifier.WebhookURL == "" {
					return fmt.Errorf("notifications.notifiers[%d]: webhook requires webhook_url", i)
				}
			case "email":
				if notifier.SMTPHost == "" {
					return fmt.Errorf("notifications.notifiers[%d]: email requires smtp_host", i)
				}
				if notifier.From == "" {
					return fmt.Errorf("notifications.notifiers[%d]: email requires from address", i)
				}
				if len(notifier.To) == 0 {
					return fmt.Errorf("notifications.notifiers[%d]: email requires at least one recipient", i)
				}
			}
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
