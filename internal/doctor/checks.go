package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/config"
	"github.com/sirupsen/logrus"
)

// ConfigFileCheck verifies config file exists and is readable
type ConfigFileCheck struct {
	cfg *config.Config
	log *logrus.Logger
}

func NewConfigFileCheck(cfg *config.Config, log *logrus.Logger) *ConfigFileCheck {
	return &ConfigFileCheck{cfg: cfg, log: log}
}

func (c *ConfigFileCheck) Name() string {
	return "Configuration File"
}

func (c *ConfigFileCheck) Run(ctx context.Context) CheckResult {
	// Check if config file exists
	configPath := c.findConfigPath()

	if configPath == "" {
		return CheckResult{
			Name:        c.Name(),
			Status:      StatusWarning,
			Message:     "No configuration file found",
			Remediation: "Run: lumo init",
		}
	}

	// Check if file is readable
	if _, err := os.Stat(configPath); err != nil {
		return CheckResult{
			Name:        c.Name(),
			Status:      StatusError,
			Message:     fmt.Sprintf("Configuration file not readable: %s", configPath),
			Remediation: fmt.Sprintf("Check file permissions: chmod 600 %s", configPath),
			Error:       err,
		}
	}

	return CheckResult{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: fmt.Sprintf("Configuration file exists: %s", configPath),
	}
}

func (c *ConfigFileCheck) findConfigPath() string {
	// Check flag, then current dir, then home dir
	paths := []string{
		"./config.yaml",
		filepath.Join(os.Getenv("HOME"), ".lumo", "config.yaml"),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// AIProviderCheck verifies AI provider configuration
type AIProviderCheck struct {
	cfg *config.Config
	log *logrus.Logger
}

func NewAIProviderCheck(cfg *config.Config, log *logrus.Logger) *AIProviderCheck {
	return &AIProviderCheck{cfg: cfg, log: log}
}

func (c *AIProviderCheck) Name() string {
	return "AI Provider Configuration"
}

func (c *AIProviderCheck) Run(ctx context.Context) CheckResult {
	provider := c.cfg.AI.Provider
	if provider == "" {
		return CheckResult{
			Name:        c.Name(),
			Status:      StatusWarning,
			Message:     "No AI provider configured",
			Remediation: "Set LUMO_AI_PROVIDER environment variable or run: lumo init",
		}
	}

	validProviders := []string{"anthropic", "openai", "ollama", "gemini", "openrouter"}
	valid := false
	for _, p := range validProviders {
		if provider == p {
			valid = true
			break
		}
	}

	if !valid {
		return CheckResult{
			Name:        c.Name(),
			Status:      StatusError,
			Message:     fmt.Sprintf("Invalid AI provider: %s", provider),
			Remediation: fmt.Sprintf("Valid providers: %s", strings.Join(validProviders, ", ")),
		}
	}

	return CheckResult{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: fmt.Sprintf("AI Provider: %s", provider),
	}
}

// AIKeyCheck tests AI API key with actual request
type AIKeyCheck struct {
	cfg *config.Config
	log *logrus.Logger
}

func NewAIKeyCheck(cfg *config.Config, log *logrus.Logger) *AIKeyCheck {
	return &AIKeyCheck{cfg: cfg, log: log}
}

func (c *AIKeyCheck) Name() string {
	return "AI API Key"
}

func (c *AIKeyCheck) Run(ctx context.Context) CheckResult {
	provider := c.cfg.AI.Provider
	if provider == "" {
		return CheckResult{
			Name:    c.Name(),
			Status:  StatusSkipped,
			Message: "Skipped (no provider configured)",
		}
	}

	// Skip Ollama (local) - no API key needed
	if provider == "ollama" {
		return CheckResult{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: "Not required for Ollama (local provider)",
		}
	}

	// Get API key for provider
	apiKey := c.getAPIKey(provider)
	if apiKey == "" {
		return CheckResult{
			Name:    c.Name(),
			Status:  StatusError,
			Message: fmt.Sprintf("No API key found for %s", provider),
			Remediation: fmt.Sprintf("Set LUMO_%s_API_KEY environment variable or run: lumo init",
				strings.ToUpper(provider)),
		}
	}

	// Test API key with a simple health check request
	if err := c.testAPIKey(ctx, provider, apiKey); err != nil {
		return CheckResult{
			Name:    c.Name(),
			Status:  StatusError,
			Message: fmt.Sprintf("API key validation failed: %s", err.Error()),
			Remediation: fmt.Sprintf("Check your LUMO_%s_API_KEY environment variable",
				strings.ToUpper(provider)),
			Error: err,
		}
	}

	return CheckResult{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: fmt.Sprintf("API key validated successfully (%s)", provider),
	}
}

func (c *AIKeyCheck) getAPIKey(provider string) string {
	return c.cfg.AI.GetAPIKeyForProvider(provider)
}

func (c *AIKeyCheck) testAPIKey(ctx context.Context, provider, apiKey string) error {
	// Use a lightweight test request for each provider
	timeout := 10 * time.Second
	client := &http.Client{Timeout: timeout}

	switch provider {
	case "anthropic":
		return c.testAnthropic(ctx, client, apiKey)
	case "openai":
		return c.testOpenAI(ctx, client, apiKey)
	case "gemini":
		return c.testGemini(ctx, client, apiKey)
	case "openrouter":
		return c.testOpenRouter(ctx, client, apiKey)
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}
}

func (c *AIKeyCheck) testAnthropic(ctx context.Context, client *http.Client, apiKey string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.anthropic.com/v1/messages", nil)
	if err != nil {
		return err
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 400 = valid key but bad request (expected for GET on messages endpoint)
	// 401 = invalid key
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("invalid API key (HTTP %d)", resp.StatusCode)
	}

	return nil
}

func (c *AIKeyCheck) testOpenAI(ctx context.Context, client *http.Client, apiKey string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/v1/models", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("invalid API key (HTTP %d)", resp.StatusCode)
	}

	return nil
}

func (c *AIKeyCheck) testGemini(ctx context.Context, client *http.Client, apiKey string) error {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models?key=%s", apiKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 400 || resp.StatusCode == 403 {
		return fmt.Errorf("invalid API key (HTTP %d)", resp.StatusCode)
	}

	return nil
}

func (c *AIKeyCheck) testOpenRouter(ctx context.Context, client *http.Client, apiKey string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("invalid API key (HTTP %d)", resp.StatusCode)
	}

	return nil
}

// RAGCheck verifies RAG configuration
type RAGCheck struct {
	cfg *config.Config
	log *logrus.Logger
}

func NewRAGCheck(cfg *config.Config, log *logrus.Logger) *RAGCheck {
	return &RAGCheck{cfg: cfg, log: log}
}

func (c *RAGCheck) Name() string {
	return "RAG System"
}

func (c *RAGCheck) Run(ctx context.Context) CheckResult {
	if !c.cfg.RAG.Enabled {
		return CheckResult{
			Name:    c.Name(),
			Status:  StatusSkipped,
			Message: "RAG system disabled",
		}
	}

	// Check storage path
	storagePath := c.cfg.RAG.StoragePath
	if storagePath == "" {
		storagePath = "./data/rag/embeddings"
	}

	// Check if storage directory exists or can be created
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return CheckResult{
			Name:        c.Name(),
			Status:      StatusError,
			Message:     fmt.Sprintf("Cannot create storage directory: %s", storagePath),
			Remediation: fmt.Sprintf("Check directory permissions: %s", filepath.Dir(storagePath)),
			Error:       err,
		}
	}

	// Check embedding provider API key
	embeddingProvider := c.cfg.RAG.EmbeddingProvider
	if embeddingProvider == "" {
		embeddingProvider = "openai"
	}

	if embeddingProvider == "openai" && c.cfg.AI.GetAPIKeyForProvider("openai") == "" {
		return CheckResult{
			Name:        c.Name(),
			Status:      StatusError,
			Message:     "RAG enabled but no OpenAI API key for embeddings",
			Remediation: "Set LUMO_OPENAI_API_KEY environment variable",
		}
	}

	return CheckResult{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: fmt.Sprintf("RAG system configured (storage: %s, embeddings: %s)", storagePath, embeddingProvider),
	}
}

// VersionCheck checks for updates
type VersionCheck struct {
	currentVersion string
	log            *logrus.Logger
}

func NewVersionCheck(currentVersion string, log *logrus.Logger) *VersionCheck {
	return &VersionCheck{currentVersion: currentVersion, log: log}
}

func (c *VersionCheck) Name() string {
	return "Version Check"
}

func (c *VersionCheck) Run(ctx context.Context) CheckResult {
	latestVersion, err := c.getLatestVersion(ctx)
	if err != nil {
		c.log.Debugf("Failed to check for updates: %v", err)
		return CheckResult{
			Name:    c.Name(),
			Status:  StatusSkipped,
			Message: "Could not check for updates (network issue)",
		}
	}

	currentVer := strings.TrimPrefix(c.currentVersion, "v")
	latestVer := strings.TrimPrefix(latestVersion, "v")

	if currentVer == latestVer {
		return CheckResult{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: fmt.Sprintf("Up to date (v%s)", currentVer),
		}
	}

	return CheckResult{
		Name:    c.Name(),
		Status:  StatusWarning,
		Message: fmt.Sprintf("Update available: v%s → v%s", currentVer, latestVer),
		Remediation: "Visit: https://github.com/ignacio/lumo/releases/latest",
	}
}

func (c *VersionCheck) getLatestVersion(ctx context.Context) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://api.github.com/repos/ignacio/lumo/releases/latest", nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.Unmarshal(body, &release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

// DependencyCheck verifies required tools are available
type DependencyCheck struct {
	cfg *config.Config
	log *logrus.Logger
}

func NewDependencyCheck(cfg *config.Config, log *logrus.Logger) *DependencyCheck {
	return &DependencyCheck{cfg: cfg, log: log}
}

func (c *DependencyCheck) Name() string {
	return "System Dependencies"
}

func (c *DependencyCheck) Run(ctx context.Context) CheckResult {
	warnings := []string{}

	// Check for common diagnostic tools
	tools := map[string]string{
		"ps":      "Process monitoring",
		"df":      "Disk usage",
		"free":    "Memory monitoring (Linux)",
		"top":     "System monitoring",
		"netstat": "Network statistics",
	}

	missing := []string{}
	for tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}

	if len(missing) > 0 {
		warnings = append(warnings, fmt.Sprintf("Missing tools: %s", strings.Join(missing, ", ")))
	}

	if len(warnings) > 0 {
		return CheckResult{
			Name:        c.Name(),
			Status:      StatusWarning,
			Message:     "Some diagnostic tools are missing",
			Remediation: fmt.Sprintf("Install missing tools: %s", strings.Join(missing, ", ")),
		}
	}

	return CheckResult{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: "All common diagnostic tools available",
	}
}
