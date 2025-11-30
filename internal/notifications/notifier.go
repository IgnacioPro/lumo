// Package notifications provides notification capabilities for Lumo.
// It supports multiple providers (Slack, Telegram, Email, Webhooks)
// with a unified interface for sending alerts and updates.
package notifications

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// NotifierType represents supported notification provider types.
type NotifierType string

const (
	NotifierSlack    NotifierType = "slack"
	NotifierTelegram NotifierType = "telegram"
	NotifierWebhook  NotifierType = "webhook"
	NotifierEmail    NotifierType = "email"
)

// Notifier represents a notification provider that can send messages.
// Implementations must be safe for concurrent use.
type Notifier interface {
	// Name returns the notifier name (e.g., "slack", "telegram", "email")
	Name() string

	// Send sends a notification.
	// The context can be used for cancellation and timeout control.
	Send(ctx context.Context, notification *Notification) error

	// Health checks if the notifier is available and configured correctly.
	Health(ctx context.Context) error
}

// NotifierConfig contains configuration for a notification provider.
type NotifierConfig struct {
	// Name is a user-friendly name for this notifier instance
	Name string

	// Type is the notifier type (slack, telegram, webhook, email)
	Type NotifierType

	// Enabled controls whether this notifier is active
	Enabled bool

	// Slack-specific configuration
	WebhookURL    string // Used by Slack webhook (legacy)
	SlackBotToken string // Slack Bot Token for API access (enables threading)
	SlackChannel  string // Slack channel ID (required for Bot Token mode)

	// Telegram-specific configuration
	TelegramBotToken string // Telegram bot token
	TelegramChatID   string // Telegram chat ID

	// Webhook-specific configuration
	Headers map[string]string // Custom HTTP headers
	Method  string            // HTTP method (default: POST)

	// Email-specific configuration
	SMTPHost     string   // SMTP server host
	SMTPPort     int      // SMTP server port
	SMTPUsername string   // SMTP authentication username
	SMTPPassword string   // SMTP authentication password
	From         string   // Email sender address
	To           []string // Email recipient addresses
	UseTLS       bool     // Use TLS connection

	// API base URL for "View Full Analysis" links (e.g., "https://lumo.example.com")
	APIBaseURL string

	// Timeout for notification requests (default: 30s)
	Timeout int
}

// NewNotifier creates a new notification provider based on configuration.
func NewNotifier(config *NotifierConfig, log *logrus.Logger) (Notifier, error) {
	if log == nil {
		log = logrus.New()
	}

	if !config.Enabled {
		return nil, fmt.Errorf("notifier %s is disabled", config.Name)
	}

	// Set notifier name if not already set
	if config.Name == "" {
		config.Name = string(config.Type)
	}

	switch config.Type {
	case NotifierSlack:
		return NewSlackNotifier(config, log)

	case NotifierTelegram:
		return NewTelegramNotifier(config, log)

	case NotifierWebhook:
		return NewWebhookNotifier(config, log)

	case NotifierEmail:
		return NewEmailNotifier(config, log)

	default:
		return nil, fmt.Errorf("unsupported notifier type: %s", config.Type)
	}
}

// ParseNotifierType parses a notifier type string.
func ParseNotifierType(s string) (NotifierType, error) {
	switch strings.ToLower(s) {
	case "slack":
		return NotifierSlack, nil
	case "telegram":
		return NotifierTelegram, nil
	case "webhook":
		return NotifierWebhook, nil
	case "email":
		return NotifierEmail, nil
	default:
		return "", fmt.Errorf("unknown notifier type: %s (supported: slack, telegram, webhook, email)", s)
	}
}

// ValidateNotifierType checks if a notifier type is supported.
func ValidateNotifierType(s string) bool {
	_, err := ParseNotifierType(s)
	return err == nil
}

// SupportedNotifiers returns a list of all supported notifier types.
func SupportedNotifiers() []NotifierType {
	return []NotifierType{
		NotifierSlack,
		NotifierTelegram,
		NotifierWebhook,
		NotifierEmail,
	}
}
