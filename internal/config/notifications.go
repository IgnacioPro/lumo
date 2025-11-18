package config

import (
	"fmt"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// NotificationConfig contains configuration for the notification system
type NotificationConfig struct {
	Enabled   bool                                `mapstructure:"enabled"`
	Routing   map[string][]string                 `mapstructure:"routing"`
	Providers NotificationProvidersConfig         `mapstructure:"providers"`
}

// NotificationProvidersConfig contains configuration for all notification providers
type NotificationProvidersConfig struct {
	Slack    *SlackNotificationConfig    `mapstructure:"slack"`
	Teams    *TeamsNotificationConfig    `mapstructure:"teams"`
	Telegram *TelegramNotificationConfig `mapstructure:"telegram"`
	Email    *EmailNotificationConfig    `mapstructure:"email"`
	Webhook  *WebhookNotificationConfig  `mapstructure:"webhook"`
}

// SlackNotificationConfig contains Slack-specific configuration
type SlackNotificationConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	WebhookURL string `mapstructure:"webhook_url"`
	Channel    string `mapstructure:"channel"`
	Username   string `mapstructure:"username"`
	IconEmoji  string `mapstructure:"icon_emoji"`
}

// TeamsNotificationConfig contains Microsoft Teams-specific configuration
type TeamsNotificationConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	WebhookURL string `mapstructure:"webhook_url"`
}

// TelegramNotificationConfig contains Telegram-specific configuration
type TelegramNotificationConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	BotToken  string `mapstructure:"bot_token"`
	ChatID    string `mapstructure:"chat_id"`
	ParseMode string `mapstructure:"parse_mode"`
}

// EmailNotificationConfig contains email-specific configuration
type EmailNotificationConfig struct {
	Enabled  bool                      `mapstructure:"enabled"`
	SMTPHost string                    `mapstructure:"smtp_host"`
	SMTPPort int                       `mapstructure:"smtp_port"`
	From     string                    `mapstructure:"from"`
	To       []string                  `mapstructure:"to"`
	Auth     EmailAuthConfig           `mapstructure:"auth"`
}

// EmailAuthConfig contains email authentication settings
type EmailAuthConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// WebhookNotificationConfig contains generic webhook configuration
type WebhookNotificationConfig struct {
	Enabled bool              `mapstructure:"enabled"`
	URL     string            `mapstructure:"url"`
	Method  string            `mapstructure:"method"`
	Headers map[string]string `mapstructure:"headers"`
}

// GetRoutingForSeverity returns the list of provider names for a given severity level
func (nc *NotificationConfig) GetRoutingForSeverity(severity diagnostics.Severity) []string {
	if nc.Routing == nil {
		return nil
	}

	return nc.Routing[string(severity)]
}

// ValidateNotifications validates the notification configuration
func (c *Config) ValidateNotifications() error {
	if !c.Notifications.Enabled {
		return nil
	}

	// Validate Slack
	if c.Notifications.Providers.Slack != nil && c.Notifications.Providers.Slack.Enabled {
		if c.Notifications.Providers.Slack.WebhookURL == "" {
			return fmt.Errorf("slack webhook_url is required when slack is enabled")
		}
	}

	// Validate Teams
	if c.Notifications.Providers.Teams != nil && c.Notifications.Providers.Teams.Enabled {
		if c.Notifications.Providers.Teams.WebhookURL == "" {
			return fmt.Errorf("teams webhook_url is required when teams is enabled")
		}
	}

	// Validate Telegram
	if c.Notifications.Providers.Telegram != nil && c.Notifications.Providers.Telegram.Enabled {
		if c.Notifications.Providers.Telegram.BotToken == "" {
			return fmt.Errorf("telegram bot_token is required when telegram is enabled")
		}
		if c.Notifications.Providers.Telegram.ChatID == "" {
			return fmt.Errorf("telegram chat_id is required when telegram is enabled")
		}
	}

	// Validate Email
	if c.Notifications.Providers.Email != nil && c.Notifications.Providers.Email.Enabled {
		if c.Notifications.Providers.Email.SMTPHost == "" {
			return fmt.Errorf("email smtp_host is required when email is enabled")
		}
		if c.Notifications.Providers.Email.From == "" {
			return fmt.Errorf("email from address is required when email is enabled")
		}
		if len(c.Notifications.Providers.Email.To) == 0 {
			return fmt.Errorf("email to addresses are required when email is enabled")
		}
	}

	// Validate Webhook
	if c.Notifications.Providers.Webhook != nil && c.Notifications.Providers.Webhook.Enabled {
		if c.Notifications.Providers.Webhook.URL == "" {
			return fmt.Errorf("webhook url is required when webhook is enabled")
		}
	}

	return nil
}

// DefaultNotificationConfig returns default notification configuration
func DefaultNotificationConfig() NotificationConfig {
	return NotificationConfig{
		Enabled: false,
		Routing: map[string][]string{
			"critical": {"slack", "email"},
			"error":    {"slack"},
			"warning":  {"slack"},
			"info":     {},
			"ok":       {},
		},
		Providers: NotificationProvidersConfig{
			Slack: &SlackNotificationConfig{
				Enabled:   false,
				Username:  "Lumo Bot",
				IconEmoji: ":robot_face:",
			},
			Teams: &TeamsNotificationConfig{
				Enabled: false,
			},
			Telegram: &TelegramNotificationConfig{
				Enabled:   false,
				ParseMode: "Markdown",
			},
			Email: &EmailNotificationConfig{
				Enabled:  false,
				SMTPPort: 587,
			},
			Webhook: &WebhookNotificationConfig{
				Enabled: false,
				Method:  "POST",
			},
		},
	}
}
