package helm

import (
	"fmt"
	"strings"
	"time"
)

// DeploymentConfig holds all configuration for a Lumo deployment
type DeploymentConfig struct {
	// Cluster access
	Kubeconfig string
	Context    string
	Namespace  string

	// Component selection
	AgentOnly bool
	SkipAgent bool

	// Kind testing
	UseKind     bool
	KindCluster string

	// Database config
	DBPassword string
	DBHost     string // Empty = use embedded PostgreSQL
	DBPort     int

	// Redis config
	RedisHost string // Empty = use embedded Redis
	RedisPort int

	// API config
	APIJWTSecret string
	APIImage     string

	// Agent config
	AgentToken    string
	AgentImage    string
	AgentReplicas int

	// AI provider
	AIProvider   string
	AnthropicKey string
	OpenAIKey    string
	GeminiKey    string

	// Deployment options
	Timeout time.Duration
	Wait    bool
	DryRun  bool
	Verbose bool

	// Helm
	HelmValues string
	HelmSets   []string
}

// Validate checks that required configuration is provided
func (d *DeploymentConfig) Validate() error {
	// Required secrets validation
	if d.DBPassword == "" && d.DBHost == "" {
		return fmt.Errorf("--db-password required (or set LUMO_DB_PASSWORD)")
	}

	if d.APIJWTSecret == "" {
		return fmt.Errorf("--api-jwt-secret required (or set LUMO_API_JWT_SECRET)")
	}

	if d.AgentToken == "" {
		return fmt.Errorf("--agent-token required (or set LUMO_AGENT_TOKEN)")
	}

	// AI provider validation (at least one key required unless using Ollama)
	if d.AnthropicKey == "" && d.OpenAIKey == "" && d.GeminiKey == "" && d.AIProvider != "ollama" {
		return fmt.Errorf("at least one AI provider key required (--anthropic-key, --openai-key, or --gemini-key), or use --ai-provider=ollama")
	}

	// Namespace validation
	if d.Namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}

	return nil
}

// GenerateHelmValues builds the values map for Helm deployment
func GenerateHelmValues(config *DeploymentConfig) (map[string]interface{}, error) {
	values := map[string]interface{}{
		"global": map[string]interface{}{
			"namespace":        config.Namespace,
			"databasePassword": config.DBPassword,
			// External PostgreSQL (if provided)
			"externalPostgresql": map[string]interface{}{
				"enabled": config.DBHost != "",
				"host":    config.DBHost,
				"port":    config.DBPort,
			},
			// External Redis (if provided)
			"externalRedis": map[string]interface{}{
				"enabled": config.RedisHost != "",
				"host":    config.RedisHost,
				"port":    config.RedisPort,
			},
			// AI provider
			"ai": map[string]interface{}{
				"provider": config.AIProvider,
				"enabled":  true,
				"keys": map[string]interface{}{
					"anthropic": config.AnthropicKey,
					"openai":    config.OpenAIKey,
					"gemini":    config.GeminiKey,
				},
			},
		},

		// PostgreSQL configuration
		"postgresql": map[string]interface{}{
			"enabled": config.DBHost == "", // Disable if using external DB
			"auth": map[string]interface{}{
				"database": "lumo",
				"username": "lumo",
				"password": config.DBPassword, // Keep for backwards compat with sub-chart
			},
		},

		// Redis configuration
		"redis": map[string]interface{}{
			"enabled": config.RedisHost == "", // Disable if using external Redis
		},

		// API server
		"api": map[string]interface{}{
			"enabled": !config.SkipAgent, // Deploy unless explicitly skipped
			"image": map[string]interface{}{
				"repository": parseImageRepo(config.APIImage),
				"tag":        parseImageTag(config.APIImage),
			},
			"jwtSecret": config.APIJWTSecret,
		},

		// Agent
		"agent": map[string]interface{}{
			"enabled":  !config.AgentOnly,
			"replicas": config.AgentReplicas,
			"image": map[string]interface{}{
				"repository": parseImageRepo(config.AgentImage),
				"tag":        parseImageTag(config.AgentImage),
			},
			"token": config.AgentToken,
		},
	}

	return values, nil
}

// parseImageRepo extracts the repository from a full image reference
// Example: "ghcr.io/ignacio/lumo-api:v1.0.0" -> "ghcr.io/ignacio/lumo-api"
func parseImageRepo(image string) string {
	// Split on last colon to separate repo from tag
	parts := strings.Split(image, ":")
	if len(parts) > 1 {
		// Rejoin all but last part (handles registry with port like localhost:5000)
		return strings.Join(parts[:len(parts)-1], ":")
	}
	return image
}

// parseImageTag extracts the tag from a full image reference
// Example: "ghcr.io/ignacio/lumo-api:v1.0.0" -> "v1.0.0"
func parseImageTag(image string) string {
	parts := strings.Split(image, ":")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return "latest"
}
