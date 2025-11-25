package config

import (
	"fmt"
	"strings"
)

// isOneOf checks if a value is in a list of valid values.
func isOneOf(value string, validValues []string) bool {
	for _, v := range validValues {
		if value == v {
			return true
		}
	}
	return false
}

// validatePort checks if a port number is valid (1-65535).
func validatePort(port int, name string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid %s port: %d (must be 1-65535)", name, port)
	}
	return nil
}

// validateRequired checks if a string value is non-empty.
func validateRequired(value, name string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s cannot be empty", name)
	}
	return nil
}

// validAIProviders is the list of supported AI providers.
var validAIProviders = []string{
	"anthropic",
	"openai",
	"ollama",
	"gemini",
	"openrouter",
	"local",  // Alias for ollama
	"google", // Alias for gemini
}

// validLogLevels is the list of supported log levels.
var validLogLevels = []string{"debug", "info", "warn", "error"}

// validLogFormats is the list of supported log formats.
var validLogFormats = []string{"text", "json"}

// validSSLModes is the list of supported database SSL modes.
var validSSLModes = []string{"disable", "require", "verify-ca", "verify-full"}

// validReasoningEfforts is the list of supported AI reasoning effort levels.
var validReasoningEfforts = []string{"low", "medium", "high"}
