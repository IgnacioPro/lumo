package messaging

import (
	"time"

	"github.com/ignacio/lumo/internal/config"
)

// FromAgentConfig converts config.MessagingConfig to messaging.Config
func FromAgentConfig(agentCfg *config.MessagingConfig) *Config {
	if agentCfg == nil {
		return DefaultConfig()
	}

	return &Config{
		Enabled:  agentCfg.Enabled,
		Provider: agentCfg.Provider,
		URL:      agentCfg.URL,
		Username: agentCfg.Username,
		Password: agentCfg.Password,
		Timeout:  agentCfg.Timeout,
		TLS: TLSConfig{
			Enabled:            agentCfg.TLS.Enabled,
			CertFile:           agentCfg.TLS.CertFile,
			KeyFile:            agentCfg.TLS.KeyFile,
			CAFile:             agentCfg.TLS.CAFile,
			InsecureSkipVerify: agentCfg.TLS.InsecureSkipVerify,
		},
		Retry: RetryConfig{
			MaxAttempts: agentCfg.Retry.MaxAttempts,
			BaseDelay:   agentCfg.Retry.BaseDelay,
			MaxDelay:    agentCfg.Retry.MaxDelay,
		},
		DLQ: DLQConfig{
			Enabled: agentCfg.DLQ.Enabled,
			Topic:   agentCfg.DLQ.Topic,
			MaxSize: agentCfg.DLQ.MaxSize,
		},
		Options: agentCfg.Options,
	}
}

// ToAgentConfig converts messaging.Config to config.MessagingConfig
func ToAgentConfig(msgCfg *Config) *config.MessagingConfig {
	if msgCfg == nil {
		return &config.MessagingConfig{
			Enabled: false,
		}
	}

	return &config.MessagingConfig{
		Enabled:  msgCfg.Enabled,
		Provider: msgCfg.Provider,
		URL:      msgCfg.URL,
		Username: msgCfg.Username,
		Password: msgCfg.Password,
		Timeout:  msgCfg.Timeout,
		TLS: config.MessagingTLSConfig{
			Enabled:            msgCfg.TLS.Enabled,
			CertFile:           msgCfg.TLS.CertFile,
			KeyFile:            msgCfg.TLS.KeyFile,
			CAFile:             msgCfg.TLS.CAFile,
			InsecureSkipVerify: msgCfg.TLS.InsecureSkipVerify,
		},
		Retry: config.MessagingRetryConfig{
			MaxAttempts: msgCfg.Retry.MaxAttempts,
			BaseDelay:   msgCfg.Retry.BaseDelay,
			MaxDelay:    msgCfg.Retry.MaxDelay,
		},
		DLQ: config.MessagingDLQConfig{
			Enabled: msgCfg.DLQ.Enabled,
			Topic:   msgCfg.DLQ.Topic,
			MaxSize: msgCfg.DLQ.MaxSize,
		},
		Options: msgCfg.Options,
	}
}

// WithDefaults applies default values to zero-valued fields
func (c *Config) WithDefaults() *Config {
	defaults := DefaultConfig()

	if c.Provider == "" {
		c.Provider = defaults.Provider
	}

	if c.URL == "" {
		c.URL = defaults.URL
	}

	if c.Timeout == 0 {
		c.Timeout = defaults.Timeout
	}

	if c.Retry.MaxAttempts == 0 {
		c.Retry.MaxAttempts = defaults.Retry.MaxAttempts
	}

	if c.Retry.BaseDelay == 0 {
		c.Retry.BaseDelay = defaults.Retry.BaseDelay
	}

	if c.Retry.MaxDelay == 0 {
		c.Retry.MaxDelay = defaults.Retry.MaxDelay
	}

	if c.DLQ.Topic == "" {
		c.DLQ.Topic = defaults.DLQ.Topic
	}

	if c.DLQ.MaxSize == 0 {
		c.DLQ.MaxSize = defaults.DLQ.MaxSize
	}

	if c.Options == nil {
		c.Options = make(map[string]interface{})
	}

	return c
}

// SetDefaults sets default values if not already set
func SetDefaults(cfg *config.MessagingConfig) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry.MaxAttempts = 3
	}

	if cfg.Retry.BaseDelay == 0 {
		cfg.Retry.BaseDelay = 1 * time.Second
	}

	if cfg.Retry.MaxDelay == 0 {
		cfg.Retry.MaxDelay = 30 * time.Second
	}

	if cfg.DLQ.Topic == "" {
		cfg.DLQ.Topic = "lumo.dlq"
	}

	if cfg.DLQ.MaxSize == 0 {
		cfg.DLQ.MaxSize = 10000
	}

	if cfg.Options == nil {
		cfg.Options = make(map[string]interface{})
	}
}
