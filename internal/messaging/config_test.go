package messaging

import (
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/config"
)

func TestFromAgentConfig(t *testing.T) {
	agentCfg := &config.MessagingConfig{
		Enabled:  true,
		Provider: "kafka",
		URL:      "localhost:9092",
		Username: "user",
		Password: "pass",
		Timeout:  45 * time.Second,
		TLS: config.MessagingTLSConfig{
			Enabled:  true,
			CertFile: "/path/to/cert.pem",
			KeyFile:  "/path/to/key.pem",
		},
		Retry: config.MessagingRetryConfig{
			MaxAttempts: 5,
			BaseDelay:   2 * time.Second,
			MaxDelay:    60 * time.Second,
		},
		DLQ: config.MessagingDLQConfig{
			Enabled: true,
			Topic:   "custom.dlq",
			MaxSize: 5000,
		},
	}

	msgCfg := FromAgentConfig(agentCfg)

	if msgCfg.Enabled != agentCfg.Enabled {
		t.Errorf("Expected Enabled %v, got %v", agentCfg.Enabled, msgCfg.Enabled)
	}

	if msgCfg.Provider != agentCfg.Provider {
		t.Errorf("Expected Provider %s, got %s", agentCfg.Provider, msgCfg.Provider)
	}

	if msgCfg.URL != agentCfg.URL {
		t.Errorf("Expected URL %s, got %s", agentCfg.URL, msgCfg.URL)
	}

	if msgCfg.Timeout != agentCfg.Timeout {
		t.Errorf("Expected Timeout %v, got %v", agentCfg.Timeout, msgCfg.Timeout)
	}

	if msgCfg.TLS.Enabled != agentCfg.TLS.Enabled {
		t.Errorf("Expected TLS.Enabled %v, got %v", agentCfg.TLS.Enabled, msgCfg.TLS.Enabled)
	}

	if msgCfg.Retry.MaxAttempts != agentCfg.Retry.MaxAttempts {
		t.Errorf("Expected Retry.MaxAttempts %d, got %d", agentCfg.Retry.MaxAttempts, msgCfg.Retry.MaxAttempts)
	}
}

func TestFromAgentConfigNil(t *testing.T) {
	msgCfg := FromAgentConfig(nil)

	if msgCfg == nil {
		t.Error("FromAgentConfig(nil) should return default config, not nil")
	}

	defaults := DefaultConfig()
	if msgCfg.Provider != defaults.Provider {
		t.Errorf("Expected default provider %s, got %s", defaults.Provider, msgCfg.Provider)
	}
}

func TestToAgentConfig(t *testing.T) {
	msgCfg := &Config{
		Enabled:  true,
		Provider: "rabbitmq",
		URL:      "amqp://localhost:5672",
		Username: "guest",
		Password: "guest",
		Timeout:  20 * time.Second,
		TLS: TLSConfig{
			Enabled:  false,
			CertFile: "",
			KeyFile:  "",
		},
		Retry: RetryConfig{
			MaxAttempts: 4,
			BaseDelay:   1500 * time.Millisecond,
			MaxDelay:    45 * time.Second,
		},
		DLQ: DLQConfig{
			Enabled: false,
			Topic:   "lumo.dlq",
			MaxSize: 8000,
		},
	}

	agentCfg := ToAgentConfig(msgCfg)

	if agentCfg.Enabled != msgCfg.Enabled {
		t.Errorf("Expected Enabled %v, got %v", msgCfg.Enabled, agentCfg.Enabled)
	}

	if agentCfg.Provider != msgCfg.Provider {
		t.Errorf("Expected Provider %s, got %s", msgCfg.Provider, agentCfg.Provider)
	}

	if agentCfg.Retry.BaseDelay != msgCfg.Retry.BaseDelay {
		t.Errorf("Expected BaseDelay %v, got %v", msgCfg.Retry.BaseDelay, agentCfg.Retry.BaseDelay)
	}
}

func TestToAgentConfigNil(t *testing.T) {
	agentCfg := ToAgentConfig(nil)

	if agentCfg == nil {
		t.Error("ToAgentConfig(nil) should return config, not nil")
	}

	if agentCfg.Enabled {
		t.Error("ToAgentConfig(nil) should return disabled config")
	}
}

func TestWithDefaults(t *testing.T) {
	cfg := &Config{
		Enabled:  true,
		Provider: "", // Empty, should get default
		URL:      "", // Empty, should get default
	}

	cfg.WithDefaults()

	if cfg.Provider == "" {
		t.Error("Provider should be set to default")
	}

	if cfg.URL == "" {
		t.Error("URL should be set to default")
	}

	if cfg.Timeout == 0 {
		t.Error("Timeout should be set to default")
	}

	if cfg.Retry.MaxAttempts == 0 {
		t.Error("Retry.MaxAttempts should be set to default")
	}
}

func TestSetDefaults(t *testing.T) {
	cfg := &config.MessagingConfig{
		Enabled:  true,
		Provider: "nats",
		URL:      "nats://localhost:4222",
	}

	SetDefaults(cfg)

	if cfg.Timeout == 0 {
		t.Error("Timeout should be set by SetDefaults")
	}

	if cfg.Retry.MaxAttempts == 0 {
		t.Error("Retry.MaxAttempts should be set by SetDefaults")
	}

	if cfg.Retry.BaseDelay == 0 {
		t.Error("Retry.BaseDelay should be set by SetDefaults")
	}

	if cfg.DLQ.Topic == "" {
		t.Error("DLQ.Topic should be set by SetDefaults")
	}

	if cfg.Options == nil {
		t.Error("Options should be initialized by SetDefaults")
	}
}
