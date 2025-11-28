package messaging

import (
	"testing"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil config",
			config:      nil,
			wantErr:     true,
			errContains: "config is nil",
		},
		{
			name: "nats provider",
			config: &Config{
				Provider: "nats",
				Brokers:  []string{"localhost:4222"},
			},
			wantErr: false,
		},
		{
			name: "kafka provider",
			config: &Config{
				Provider: "kafka",
				Brokers:  []string{"localhost:9092"},
			},
			wantErr: false,
		},
		{
			name: "rabbitmq provider",
			config: &Config{
				Provider: "rabbitmq",
				Brokers:  []string{"localhost:5672"},
			},
			wantErr: false,
		},
		{
			name: "redis provider",
			config: &Config{
				Provider: "redis",
				Brokers:  []string{"localhost:6379"},
			},
			wantErr: false,
		},
		{
			name: "unsupported provider",
			config: &Config{
				Provider: "unsupported",
			},
			wantErr:     true,
			errContains: "unsupported messaging provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewProvider() expected error, got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewProvider() error = %v, want to contain %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("NewProvider() unexpected error: %v", err)
				return
			}

			if provider == nil {
				t.Errorf("NewProvider() returned nil provider")
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || len(s) > len(substr) && s[1:len(substr)+1] == substr)
}
