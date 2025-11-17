package ai

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// ProviderType represents supported AI provider types.
type ProviderType string

const (
	ProviderAnthropic  ProviderType = "anthropic"
	ProviderOpenAI     ProviderType = "openai"
	ProviderOllama     ProviderType = "ollama"
	ProviderGemini     ProviderType = "gemini"
	ProviderOpenRouter ProviderType = "openrouter"
)

// NewProvider creates a new AI provider based on configuration.
func NewProvider(providerType ProviderType, config *ProviderConfig, log *logrus.Logger) (Provider, error) {
	if log == nil {
		log = logrus.New()
	}

	// Set provider name if not already set
	if config.Name == "" {
		config.Name = string(providerType)
	}

	switch providerType {
	case ProviderAnthropic:
		return NewAnthropicProvider(config, log)

	case ProviderOpenAI:
		return NewOpenAIProvider(config, log)

	case ProviderOllama:
		return NewOllamaProvider(config, log)

	case ProviderGemini:
		return NewGeminiProvider(config, log)

	case ProviderOpenRouter:
		return NewOpenRouterProvider(config, log)

	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

// ParseProviderType parses a provider type string.
func ParseProviderType(s string) (ProviderType, error) {
	switch strings.ToLower(s) {
	case "anthropic", "claude":
		return ProviderAnthropic, nil
	case "openai", "gpt":
		return ProviderOpenAI, nil
	case "ollama", "local":
		return ProviderOllama, nil
	case "gemini", "google":
		return ProviderGemini, nil
	case "openrouter":
		return ProviderOpenRouter, nil
	default:
		return "", fmt.Errorf("unknown provider type: %s (supported: anthropic, openai, ollama, gemini, openrouter)", s)
	}
}

// ValidateProviderType checks if a provider type is supported.
func ValidateProviderType(s string) bool {
	_, err := ParseProviderType(s)
	return err == nil
}

// SupportedProviders returns a list of all supported provider types.
func SupportedProviders() []ProviderType {
	return []ProviderType{
		ProviderAnthropic,
		ProviderOpenAI,
		ProviderOllama,
		ProviderGemini,
		ProviderOpenRouter,
	}
}

// DefaultModelForProvider returns the default model for a provider.
func DefaultModelForProvider(providerType ProviderType) string {
	switch providerType {
	case ProviderAnthropic:
		return DefaultAnthropicModel
	case ProviderOpenAI:
		return DefaultOpenAIModel
	case ProviderOllama:
		return DefaultOllamaModel
	case ProviderGemini:
		return DefaultGeminiModel
	case ProviderOpenRouter:
		return DefaultOpenRouterModel
	default:
		return ""
	}
}
