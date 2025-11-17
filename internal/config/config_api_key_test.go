package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestGetAPIKeyForProvider(t *testing.T) {
	// Save original env vars
	origAnthropicKey := os.Getenv("LUMO_ANTHROPIC_API_KEY")
	origOpenAIKey := os.Getenv("LUMO_OPENAI_API_KEY")
	origGeminiKey := os.Getenv("LUMO_GEMINI_API_KEY")
	origGenericKey := os.Getenv("LUMO_AI_API_KEY")

	// Clean up after test
	defer func() {
		_ = os.Setenv("LUMO_ANTHROPIC_API_KEY", origAnthropicKey)
		_ = os.Setenv("LUMO_OPENAI_API_KEY", origOpenAIKey)
		_ = os.Setenv("LUMO_GEMINI_API_KEY", origGeminiKey)
		_ = os.Setenv("LUMO_AI_API_KEY", origGenericKey)
		viper.Reset()
	}()

	tests := []struct {
		name                 string
		provider             string
		anthropicEnv         string
		openaiEnv            string
		geminiEnv            string
		genericEnv           string
		configAPIKey         string
		expectedKey          string
		shouldUseProviderKey bool
	}{
		{
			name:                 "anthropic with provider-specific key",
			provider:             "anthropic",
			anthropicEnv:         "sk-ant-provider-specific",
			genericEnv:           "sk-generic",
			expectedKey:          "sk-ant-provider-specific",
			shouldUseProviderKey: true,
		},
		{
			name:        "anthropic falls back to generic key",
			provider:    "anthropic",
			genericEnv:  "sk-generic",
			expectedKey: "sk-generic",
		},
		{
			name:                 "openai with provider-specific key",
			provider:             "openai",
			openaiEnv:            "sk-openai-specific",
			genericEnv:           "sk-generic",
			expectedKey:          "sk-openai-specific",
			shouldUseProviderKey: true,
		},
		{
			name:        "openai falls back to generic key",
			provider:    "openai",
			genericEnv:  "sk-generic",
			expectedKey: "sk-generic",
		},
		{
			name:                 "gemini with provider-specific key",
			provider:             "gemini",
			geminiEnv:            "gemini-key-specific",
			genericEnv:           "sk-generic",
			expectedKey:          "gemini-key-specific",
			shouldUseProviderKey: true,
		},
		{
			name:        "gemini falls back to generic key",
			provider:    "gemini",
			genericEnv:  "sk-generic",
			expectedKey: "sk-generic",
		},
		{
			name:        "google alias uses gemini key",
			provider:    "google",
			geminiEnv:   "gemini-key",
			expectedKey: "gemini-key",
		},
		{
			name:         "config file key is used when no env vars",
			provider:     "anthropic",
			configAPIKey: "sk-from-config",
			expectedKey:  "sk-from-config",
		},
		{
			name:        "ollama doesn't require key",
			provider:    "ollama",
			expectedKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper for each test
			viper.Reset()
			viper.AutomaticEnv()
			viper.SetEnvPrefix("LUMO")

			// Set environment variables
			if tt.anthropicEnv != "" {
				_ = os.Setenv("LUMO_ANTHROPIC_API_KEY", tt.anthropicEnv)
			} else {
				_ = os.Unsetenv("LUMO_ANTHROPIC_API_KEY")
			}

			if tt.openaiEnv != "" {
				_ = os.Setenv("LUMO_OPENAI_API_KEY", tt.openaiEnv)
			} else {
				_ = os.Unsetenv("LUMO_OPENAI_API_KEY")
			}

			if tt.geminiEnv != "" {
				_ = os.Setenv("LUMO_GEMINI_API_KEY", tt.geminiEnv)
			} else {
				_ = os.Unsetenv("LUMO_GEMINI_API_KEY")
			}

			if tt.genericEnv != "" {
				_ = os.Setenv("LUMO_AI_API_KEY", tt.genericEnv)
			} else {
				_ = os.Unsetenv("LUMO_AI_API_KEY")
			}

			// Create config
			cfg := &AIConfig{
				Provider: tt.provider,
				APIKey:   tt.configAPIKey,
			}

			// Get API key
			result := cfg.GetAPIKeyForProvider(tt.provider)

			// Verify result
			if result != tt.expectedKey {
				t.Errorf("GetAPIKeyForProvider(%q) = %q, want %q", tt.provider, result, tt.expectedKey)
			}

			// Verify provider-specific key takes precedence
			if tt.shouldUseProviderKey && result == tt.genericEnv {
				t.Errorf("Expected provider-specific key to take precedence over generic key")
			}
		})
	}
}

func TestProviderSwitchingWithoutChangingKeys(t *testing.T) {
	// Save original env vars
	origAnthropicKey := os.Getenv("LUMO_ANTHROPIC_API_KEY")
	origOpenAIKey := os.Getenv("LUMO_OPENAI_API_KEY")
	origGeminiKey := os.Getenv("LUMO_GEMINI_API_KEY")

	// Clean up after test
	defer func() {
		_ = os.Setenv("LUMO_ANTHROPIC_API_KEY", origAnthropicKey)
		_ = os.Setenv("LUMO_OPENAI_API_KEY", origOpenAIKey)
		_ = os.Setenv("LUMO_GEMINI_API_KEY", origGeminiKey)
		viper.Reset()
	}()

	// Set up all provider keys
	_ = os.Setenv("LUMO_ANTHROPIC_API_KEY", "sk-ant-key")
	_ = os.Setenv("LUMO_OPENAI_API_KEY", "sk-openai-key")
	_ = os.Setenv("LUMO_GEMINI_API_KEY", "gemini-key")

	viper.Reset()
	viper.AutomaticEnv()
	viper.SetEnvPrefix("LUMO")

	// Create config
	cfg := &AIConfig{}

	// Test switching providers - should automatically use correct key
	providers := []struct {
		name        string
		expectedKey string
	}{
		{"anthropic", "sk-ant-key"},
		{"openai", "sk-openai-key"},
		{"gemini", "gemini-key"},
		{"google", "gemini-key"}, // Alias
	}

	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			cfg.Provider = p.name
			key := cfg.GetAPIKeyForProvider(p.name)

			if key != p.expectedKey {
				t.Errorf("Provider %q: got key %q, want %q", p.name, key, p.expectedKey)
			}
		})
	}
}
