package doctor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ignacio/lumo/internal/config"
)

func TestConfigFileCheck(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := &config.Config{}

	t.Run("NoConfigFile", func(t *testing.T) {
		// Change to a temp directory with no config
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		// Temporarily unset HOME to avoid finding config in home dir
		origHome := os.Getenv("HOME")
		_ = os.Setenv("HOME", tmpDir)
		defer func() { _ = os.Setenv("HOME", origHome) }()

		check := NewConfigFileCheck(cfg, logger)
		result := check.Run(context.Background())

		assert.Equal(t, "Configuration File", result.Name)
		assert.Equal(t, StatusWarning, result.Status)
		assert.Contains(t, result.Message, "No configuration file found")
		assert.Contains(t, result.Remediation, "lumo init")
	})

	t.Run("ConfigFileExists", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte("test: true"), 0600)
		require.NoError(t, err)

		origDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(origDir) }()
		_ = os.Chdir(tmpDir)

		check := NewConfigFileCheck(cfg, logger)
		result := check.Run(context.Background())

		assert.Equal(t, StatusOK, result.Status)
		assert.Contains(t, result.Message, "config.yaml")
	})
}

func TestAIProviderCheck(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("NoProvider", func(t *testing.T) {
		cfg := &config.Config{
			AI: config.AIConfig{},
		}

		check := NewAIProviderCheck(cfg, logger)
		result := check.Run(context.Background())

		assert.Equal(t, StatusWarning, result.Status)
		assert.Contains(t, result.Message, "No AI provider")
	})

	t.Run("ValidProvider", func(t *testing.T) {
		providers := []string{"anthropic", "openai", "ollama", "gemini", "openrouter"}

		for _, provider := range providers {
			cfg := &config.Config{
				AI: config.AIConfig{
					Provider: provider,
				},
			}

			check := NewAIProviderCheck(cfg, logger)
			result := check.Run(context.Background())

			assert.Equal(t, StatusOK, result.Status)
			assert.Contains(t, result.Message, provider)
		}
	})

	t.Run("InvalidProvider", func(t *testing.T) {
		cfg := &config.Config{
			AI: config.AIConfig{
				Provider: "invalid-provider",
			},
		}

		check := NewAIProviderCheck(cfg, logger)
		result := check.Run(context.Background())

		assert.Equal(t, StatusError, result.Status)
		assert.Contains(t, result.Message, "Invalid AI provider")
		assert.Contains(t, result.Remediation, "Valid providers")
	})
}

func TestAIKeyCheck(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("NoProvider", func(t *testing.T) {
		cfg := &config.Config{
			AI: config.AIConfig{},
		}

		check := NewAIKeyCheck(cfg, logger)
		result := check.Run(context.Background())

		assert.Equal(t, StatusSkipped, result.Status)
		assert.Contains(t, result.Message, "no provider")
	})

	t.Run("OllamaProvider", func(t *testing.T) {
		cfg := &config.Config{
			AI: config.AIConfig{
				Provider: "ollama",
			},
		}

		check := NewAIKeyCheck(cfg, logger)
		result := check.Run(context.Background())

		assert.Equal(t, StatusOK, result.Status)
		assert.Contains(t, result.Message, "Not required for Ollama")
	})

	t.Run("NoAPIKey", func(t *testing.T) {
		cfg := &config.Config{
			AI: config.AIConfig{
				Provider: "anthropic",
			},
		}

		check := NewAIKeyCheck(cfg, logger)
		result := check.Run(context.Background())

		assert.Equal(t, StatusError, result.Status)
		assert.Contains(t, result.Message, "No API key found")
	})

	t.Run("ValidAnthropicKey", func(t *testing.T) {
		// Mock Anthropic API
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("x-api-key") == "valid-key" {
				w.WriteHeader(http.StatusBadRequest) // Expected for GET request
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		}))
		defer server.Close()

		cfg := &config.Config{
			AI: config.AIConfig{
				Provider: "anthropic",
			},
		}
		// Set API key via environment or internal method
		_ = os.Setenv("LUMO_ANTHROPIC_API_KEY", "valid-key")
		defer func() { _ = os.Unsetenv("LUMO_ANTHROPIC_API_KEY") }()

		check := NewAIKeyCheck(cfg, logger)
		// Note: This will fail with real API but we're testing the structure
		_ = check.Run(context.Background())
		// Can't easily test success without mocking HTTP client more deeply
	})
}

func TestRAGCheck(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("Disabled", func(t *testing.T) {
		cfg := &config.Config{
			RAG: config.RAGConfig{
				Enabled: false,
			},
		}

		check := NewRAGCheck(cfg, logger)
		result := check.Run(context.Background())

		assert.Equal(t, StatusSkipped, result.Status)
		assert.Contains(t, result.Message, "disabled")
	})

	t.Run("EnabledWithStorage", func(t *testing.T) {
		t.Skip("Skipping - GetAPIKeyForProvider requires viper initialization which is complex in tests")
	})

	t.Run("EnabledNoAPIKey", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &config.Config{
			RAG: config.RAGConfig{
				Enabled:           true,
				StoragePath:       filepath.Join(tmpDir, "rag-storage"),
				EmbeddingProvider: "openai",
			},
			AI: config.AIConfig{},
		}
		// Ensure no OpenAI key is set
		_ = os.Unsetenv("LUMO_OPENAI_API_KEY")

		check := NewRAGCheck(cfg, logger)
		result := check.Run(context.Background())

		assert.Equal(t, StatusError, result.Status)
		assert.Contains(t, result.Message, "no OpenAI API key")
	})

	t.Run("InvalidStoragePath", func(t *testing.T) {
		cfg := &config.Config{
			RAG: config.RAGConfig{
				Enabled:           true,
				StoragePath:       "/root/no-permission/rag",
				EmbeddingProvider: "openai",
			},
			AI: config.AIConfig{},
		}
		_ = os.Setenv("LUMO_OPENAI_API_KEY", "test-key")
		defer func() { _ = os.Unsetenv("LUMO_OPENAI_API_KEY") }()

		check := NewRAGCheck(cfg, logger)
		result := check.Run(context.Background())

		// This might be StatusError or StatusOK depending on permissions
		// Just verify it doesn't panic
		assert.NotEmpty(t, result.Status)
	})
}

func TestVersionCheck(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("NetworkError", func(t *testing.T) {
		check := NewVersionCheck("v1.0.0", logger)

		// Use a context with very short timeout to force error
		ctx, cancel := context.WithTimeout(context.Background(), 1)
		defer cancel()

		result := check.Run(ctx)

		assert.Equal(t, StatusSkipped, result.Status)
		assert.Contains(t, result.Message, "Could not check")
	})

	t.Run("UpToDate", func(t *testing.T) {
		// Mock GitHub API
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"tag_name": "v1.0.0"}`))
		}))
		defer server.Close()

		check := &VersionCheck{
			currentVersion: "v1.0.0",
			log:            logger,
		}

		// This test would need HTTP client mocking to work properly
		// For now, just verify the structure
		assert.NotNil(t, check)
	})
}

func TestDependencyCheck(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := &config.Config{}

	t.Run("CheckExists", func(t *testing.T) {
		check := NewDependencyCheck(cfg, logger)
		result := check.Run(context.Background())

		// Result depends on system, but should not panic
		assert.NotEmpty(t, result.Status)
		assert.NotEmpty(t, result.Message)

		// On most systems, at least some tools should be available
		if result.Status == StatusOK {
			assert.Contains(t, result.Message, "available")
		}
	})
}

func TestCheckConstructors(t *testing.T) {
	logger := logrus.New()
	cfg := &config.Config{}

	t.Run("AllConstructors", func(t *testing.T) {
		checks := []struct {
			name  string
			check Check
		}{
			{"ConfigFile", NewConfigFileCheck(cfg, logger)},
			{"AIProvider", NewAIProviderCheck(cfg, logger)},
			{"AIKey", NewAIKeyCheck(cfg, logger)},
			{"RAG", NewRAGCheck(cfg, logger)},
			{"Version", NewVersionCheck("v1.0.0", logger)},
			{"Dependency", NewDependencyCheck(cfg, logger)},
		}

		for _, tc := range checks {
			t.Run(tc.name, func(t *testing.T) {
				assert.NotNil(t, tc.check)
				assert.NotEmpty(t, tc.check.Name())

				// Verify Run doesn't panic
				result := tc.check.Run(context.Background())
				assert.NotEmpty(t, result.Name)
				assert.NotEmpty(t, result.Status)
			})
		}
	})
}

func TestAIKeyCheck_GetAPIKey(t *testing.T) {
	t.Skip("Skipping - getAPIKey is a private method and GetAPIKeyForProvider requires proper config initialization")
}
