package ai

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestParseProviderType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ProviderType
		wantErr bool
	}{
		{
			name:    "anthropic",
			input:   "anthropic",
			want:    ProviderAnthropic,
			wantErr: false,
		},
		{
			name:    "claude alias",
			input:   "claude",
			want:    ProviderAnthropic,
			wantErr: false,
		},
		{
			name:    "openai",
			input:   "openai",
			want:    ProviderOpenAI,
			wantErr: false,
		},
		{
			name:    "gpt alias",
			input:   "gpt",
			want:    ProviderOpenAI,
			wantErr: false,
		},
		{
			name:    "ollama",
			input:   "ollama",
			want:    ProviderOllama,
			wantErr: false,
		},
		{
			name:    "local alias",
			input:   "local",
			want:    ProviderOllama,
			wantErr: false,
		},
		{
			name:    "gemini",
			input:   "gemini",
			want:    ProviderGemini,
			wantErr: false,
		},
		{
			name:    "google alias",
			input:   "google",
			want:    ProviderGemini,
			wantErr: false,
		},
		{
			name:    "case insensitive - uppercase",
			input:   "ANTHROPIC",
			want:    ProviderAnthropic,
			wantErr: false,
		},
		{
			name:    "case insensitive - mixed",
			input:   "OpenAI",
			want:    ProviderOpenAI,
			wantErr: false,
		},
		{
			name:    "unknown provider",
			input:   "unknown",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseProviderType(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProviderType(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("ParseProviderType(%v) = %v, want %v", tt.input, got, tt.want)
			}

			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), "unknown provider") {
					t.Errorf("Expected error to mention unknown provider, got: %v", err)
				}
			}
		})
	}
}

func TestValidateProviderType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid - anthropic", "anthropic", true},
		{"valid - openai", "openai", true},
		{"valid - ollama", "ollama", true},
		{"valid - gemini", "gemini", true},
		{"valid alias - claude", "claude", true},
		{"valid alias - gpt", "gpt", true},
		{"valid alias - local", "local", true},
		{"valid alias - google", "google", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
		{"random string", "xyz123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateProviderType(tt.input)
			if got != tt.want {
				t.Errorf("ValidateProviderType(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSupportedProviders(t *testing.T) {
	providers := SupportedProviders()

	if len(providers) == 0 {
		t.Error("SupportedProviders() returned empty list")
	}

	// Check that all expected providers are present
	expectedProviders := map[ProviderType]bool{
		ProviderAnthropic: true,
		ProviderOpenAI:    true,
		ProviderOllama:    true,
		ProviderGemini:    true,
	}

	for _, p := range providers {
		if !expectedProviders[p] {
			t.Errorf("Unexpected provider in list: %v", p)
		}
		delete(expectedProviders, p)
	}

	if len(expectedProviders) > 0 {
		t.Errorf("Missing providers: %v", expectedProviders)
	}
}

func TestDefaultModelForProvider(t *testing.T) {
	tests := []struct {
		name         string
		providerType ProviderType
		wantEmpty    bool
	}{
		{
			name:         "anthropic has default model",
			providerType: ProviderAnthropic,
			wantEmpty:    false,
		},
		{
			name:         "openai has default model",
			providerType: ProviderOpenAI,
			wantEmpty:    false,
		},
		{
			name:         "ollama has default model",
			providerType: ProviderOllama,
			wantEmpty:    false,
		},
		{
			name:         "gemini has default model",
			providerType: ProviderGemini,
			wantEmpty:    false,
		},
		{
			name:         "unknown provider returns empty",
			providerType: ProviderType("unknown"),
			wantEmpty:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultModelForProvider(tt.providerType)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("DefaultModelForProvider(%v) = %v, want empty string", tt.providerType, got)
				}
			} else {
				if got == "" {
					t.Errorf("DefaultModelForProvider(%v) returned empty, want non-empty", tt.providerType)
				}
			}
		})
	}
}

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name         string
		providerType ProviderType
		config       *ProviderConfig
		wantErr      bool
	}{
		{
			name:         "anthropic with valid config",
			providerType: ProviderAnthropic,
			config: &ProviderConfig{
				APIKey:      "test-key",
				Model:       "test-model",
				Temperature: 1.0,
				MaxTokens:   1000,
			},
			wantErr: false,
		},
		{
			name:         "openai with valid config",
			providerType: ProviderOpenAI,
			config: &ProviderConfig{
				APIKey:      "test-key",
				Model:       "test-model",
				Temperature: 1.0,
				MaxTokens:   1000,
			},
			wantErr: false,
		},
		{
			name:         "ollama with valid config",
			providerType: ProviderOllama,
			config: &ProviderConfig{
				Model:       "test-model",
				Temperature: 1.0,
				MaxTokens:   1000,
			},
			wantErr: false,
		},
		{
			name:         "gemini with valid config",
			providerType: ProviderGemini,
			config: &ProviderConfig{
				APIKey:      "test-key",
				Model:       "test-model",
				Temperature: 1.0,
				MaxTokens:   1000,
			},
			wantErr: false,
		},
		{
			name:         "unsupported provider",
			providerType: ProviderType("unsupported"),
			config:       &ProviderConfig{},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logrus.New()
			log.SetOutput(logrus.StandardLogger().Out) // Suppress logs during tests

			provider, err := NewProvider(tt.providerType, tt.config, log)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if provider == nil {
					t.Error("NewProvider() returned nil provider without error")
				}

				// Verify provider name is set
				if provider.Name() == "" {
					t.Error("Provider.Name() returned empty string")
				}
			}

			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), "unsupported provider") {
					t.Errorf("Expected error about unsupported provider, got: %v", err)
				}
			}
		})
	}
}

func TestNewProvider_SetsConfigName(t *testing.T) {
	config := &ProviderConfig{
		APIKey:      "test-key",
		Model:       "test-model",
		Temperature: 1.0,
		MaxTokens:   1000,
	}

	log := logrus.New()
	provider, err := NewProvider(ProviderAnthropic, config, log)

	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	if provider == nil {
		t.Fatal("NewProvider() returned nil")
	}

	// Config name should be set to provider type
	if config.Name == "" {
		t.Error("NewProvider() did not set config.Name")
	}

	if config.Name != string(ProviderAnthropic) {
		t.Errorf("config.Name = %v, want %v", config.Name, ProviderAnthropic)
	}
}

func TestNewProvider_NilLogger(t *testing.T) {
	config := &ProviderConfig{
		APIKey:      "test-key",
		Model:       "test-model",
		Temperature: 1.0,
		MaxTokens:   1000,
	}

	// Should not panic with nil logger
	provider, err := NewProvider(ProviderAnthropic, config, nil)

	if err != nil {
		t.Fatalf("NewProvider() with nil logger error = %v", err)
	}

	if provider == nil {
		t.Error("NewProvider() with nil logger returned nil provider")
	}
}

func TestError_Error(t *testing.T) {
	tests := []struct {
		name        string
		aiErr       *Error
		wantContain []string
	}{
		{
			name: "error with provider",
			aiErr: &Error{
				Op:       "analyze",
				Provider: "anthropic",
				Err:      fmt.Errorf("api error"),
			},
			wantContain: []string{"analyze", "[anthropic]", "api error"},
		},
		{
			name: "error without provider",
			aiErr: &Error{
				Op:  "validate",
				Err: fmt.Errorf("validation failed"),
			},
			wantContain: []string{"validate", "validation failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.aiErr.Error()

			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("Error() = %v, should contain %v", got, want)
				}
			}
		})
	}
}

func TestHealthStatus_Constants(t *testing.T) {
	// Verify health status constants are defined
	statuses := []HealthStatus{
		HealthHealthy,
		HealthDegraded,
		HealthCritical,
		HealthUnknown,
	}

	for _, status := range statuses {
		if string(status) == "" {
			t.Errorf("HealthStatus constant has empty string value")
		}
	}
}

func TestPriority_Constants(t *testing.T) {
	// Verify priority constants are defined
	priorities := []Priority{
		PriorityCritical,
		PriorityHigh,
		PriorityMedium,
		PriorityLow,
	}

	for _, priority := range priorities {
		if string(priority) == "" {
			t.Errorf("Priority constant has empty string value")
		}
	}
}

func TestRiskLevel_Constants(t *testing.T) {
	// Verify risk level constants are defined
	risks := []RiskLevel{
		RiskSafe,
		RiskLow,
		RiskModerate,
		RiskHigh,
		RiskCritical,
	}

	for _, risk := range risks {
		if string(risk) == "" {
			t.Errorf("RiskLevel constant has empty string value")
		}
	}
}
