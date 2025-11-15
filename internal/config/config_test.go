package config

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	// Test SSH defaults
	if cfg.SSH.Port != 22 {
		t.Errorf("SSH.Port = %v, want 22", cfg.SSH.Port)
	}
	if cfg.SSH.Timeout != 30*time.Second {
		t.Errorf("SSH.Timeout = %v, want 30s", cfg.SSH.Timeout)
	}
	if cfg.SSH.MaxRetries != 3 {
		t.Errorf("SSH.MaxRetries = %v, want 3", cfg.SSH.MaxRetries)
	}

	// Test AI defaults
	if cfg.AI.Provider != "anthropic" {
		t.Errorf("AI.Provider = %v, want anthropic", cfg.AI.Provider)
	}
	if cfg.AI.Temperature != 1.0 {
		t.Errorf("AI.Temperature = %v, want 1.0", cfg.AI.Temperature)
	}
	if cfg.AI.MaxTokens != 4096 {
		t.Errorf("AI.MaxTokens = %v, want 4096", cfg.AI.MaxTokens)
	}
	if !cfg.AI.Enabled {
		t.Error("AI.Enabled should be true by default")
	}

	// Test Logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %v, want info", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Logging.Format = %v, want text", cfg.Logging.Format)
	}

	// Test API defaults
	if cfg.API.Port != 8080 {
		t.Errorf("API.Port = %v, want 8080", cfg.API.Port)
	}
	if cfg.API.Host != "0.0.0.0" {
		t.Errorf("API.Host = %v, want 0.0.0.0", cfg.API.Host)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with API key set",
			cfg: func() *Config {
				c := DefaultConfig()
				c.AI.APIKey = "test-key"
				return c
			}(),
			wantErr: false,
		},
		{
			name: "invalid SSH port - too low",
			cfg: &Config{
				SSH: SSHConfig{Port: 0},
				AI:  AIConfig{Provider: "anthropic"},
			},
			wantErr: true,
			errMsg:  "port",
		},
		{
			name: "invalid SSH port - too high",
			cfg: &Config{
				SSH: SSHConfig{Port: 70000},
				AI:  AIConfig{Provider: "anthropic"},
			},
			wantErr: true,
			errMsg:  "port",
		},
		{
			name: "invalid AI provider",
			cfg: &Config{
				SSH:     SSHConfig{Port: 22},
				AI:      AIConfig{Provider: "invalid_provider", APIKey: "key", Enabled: true, MaxTokens: 1000, Temperature: 1.0},
				Logging: LoggingConfig{Level: "info", Format: "text"},
				API:     APIConfig{Port: 8080},
			},
			wantErr: true,
			errMsg:  "AI provider",
		},
		{
			name: "invalid log level",
			cfg: &Config{
				SSH:     SSHConfig{Port: 22},
				AI:      AIConfig{Provider: "anthropic", APIKey: "key"},
				Logging: LoggingConfig{Level: "invalid_level", Format: "text"},
				API:     APIConfig{Port: 8080},
			},
			wantErr: true,
			errMsg:  "log level",
		},
		{
			name: "invalid log format",
			cfg: &Config{
				SSH:     SSHConfig{Port: 22},
				AI:      AIConfig{Provider: "anthropic", APIKey: "key"},
				Logging: LoggingConfig{Level: "info", Format: "invalid_format"},
				API:     APIConfig{Port: 8080},
			},
			wantErr: true,
			errMsg:  "log format",
		},
		{
			name: "TLS enabled without cert",
			cfg: &Config{
				SSH:     SSHConfig{Port: 22},
				AI:      AIConfig{Provider: "anthropic", APIKey: "key"},
				Logging: LoggingConfig{Level: "info", Format: "text"},
				API:     APIConfig{Port: 8080, TLS: true, CertFile: "", KeyFile: "key.pem"},
			},
			wantErr: true,
			errMsg:  "cert_file",
		},
		{
			name: "TLS enabled without key",
			cfg: &Config{
				SSH:     SSHConfig{Port: 22},
				AI:      AIConfig{Provider: "anthropic", APIKey: "key"},
				Logging: LoggingConfig{Level: "info", Format: "text"},
				API:     APIConfig{Port: 8080, TLS: true, CertFile: "cert.pem", KeyFile: ""},
			},
			wantErr: true,
			errMsg:  "key_file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errMsg)) {
					t.Errorf("Validate() error = %v, should contain %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestGetModelForProvider(t *testing.T) {
	tests := []struct {
		name   string
		config AIConfig
		provider string
		want   string
	}{
		{
			name:     "uses explicit Model field",
			config:   AIConfig{Model: "explicit-model"},
			provider: "anthropic",
			want:     "explicit-model",
		},
		{
			name: "uses Models map",
			config: AIConfig{
				Models: map[string]string{
					"anthropic": "custom-claude",
				},
			},
			provider: "anthropic",
			want:     "custom-claude",
		},
		{
			name:     "returns empty for no configuration",
			config:   AIConfig{},
			provider: "anthropic",
			want:     "",
		},
		{
			name: "prefers Model field over Models map",
			config: AIConfig{
				Model: "explicit-model",
				Models: map[string]string{
					"anthropic": "map-model",
				},
			},
			provider: "anthropic",
			want:     "explicit-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetModelForProvider(tt.provider)
			if got != tt.want {
				t.Errorf("GetModelForProvider(%v) = %v, want %v", tt.provider, got, tt.want)
			}
		})
	}
}

// Removed TestGetModelForProvider_WithOverrides - covered by table-driven test above

func TestNetworkTarget(t *testing.T) {
	target := NetworkTarget{
		Host:     "example.com",
		Port:     80,
		Protocol: "tcp",
	}

	if target.Host != "example.com" {
		t.Errorf("Host = %v, want example.com", target.Host)
	}
	if target.Port != 80 {
		t.Errorf("Port = %v, want 80", target.Port)
	}
	if target.Protocol != "tcp" {
		t.Errorf("Protocol = %v, want tcp", target.Protocol)
	}
}
