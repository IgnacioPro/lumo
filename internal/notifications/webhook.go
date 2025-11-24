package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/reliability"
	"github.com/sirupsen/logrus"
)

// WebhookNotifier sends notifications via generic webhooks (Discord, Teams, Mattermost, etc.).
type WebhookNotifier struct {
	config         *NotifierConfig
	log            *logrus.Logger
	client         *http.Client
	circuitBreaker *reliability.CircuitBreaker
}

// webhookMessage represents a generic webhook message payload.
// This format is compatible with Discord, Microsoft Teams, Mattermost, and others.
type webhookMessage struct {
	Content    string           `json:"content,omitempty"`    // Plain text (Discord, Mattermost)
	Username   string           `json:"username,omitempty"`   // Bot name override
	Embeds     []webhookEmbed   `json:"embeds,omitempty"`     // Rich embeds (Discord)
	Text       string           `json:"text,omitempty"`       // Plain text (Teams)
	Title      string           `json:"title,omitempty"`      // Title (Teams)
	Summary    string           `json:"summary,omitempty"`    // Summary (Teams)
	Sections   []webhookSection `json:"sections,omitempty"`   // Sections (Teams)
	ThemeColor string           `json:"themeColor,omitempty"` // Color (Teams)
}

// webhookEmbed represents a rich embed for Discord-style webhooks.
type webhookEmbed struct {
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Color       int            `json:"color,omitempty"` // Decimal color value
	Fields      []webhookField `json:"fields,omitempty"`
	Footer      *webhookFooter `json:"footer,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"` // ISO 8601 timestamp
}

// webhookField represents a field in a webhook embed.
type webhookField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// webhookFooter represents a footer in a webhook embed.
type webhookFooter struct {
	Text string `json:"text"`
}

// webhookSection represents a section for Teams-style webhooks.
type webhookSection struct {
	ActivityTitle    string         `json:"activityTitle,omitempty"`
	ActivitySubtitle string         `json:"activitySubtitle,omitempty"`
	ActivityText     string         `json:"activityText,omitempty"`
	Facts            []webhookField `json:"facts,omitempty"`
}

// NewWebhookNotifier creates a new generic webhook notifier.
func NewWebhookNotifier(config *NotifierConfig, log *logrus.Logger) (*WebhookNotifier, error) {
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}

	// Default to POST if method not specified
	if config.Method == "" {
		config.Method = "POST"
	}

	timeout := 30 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	return &WebhookNotifier{
		config: config,
		log:    log,
		client: &http.Client{
			Timeout: timeout,
		},
		circuitBreaker: reliability.NewCircuitBreaker(fmt.Sprintf("webhook-%s", config.Name)),
	}, nil
}

// Name returns the notifier name.
func (w *WebhookNotifier) Name() string {
	return w.config.Name
}

// Send sends a notification to the webhook.
func (w *WebhookNotifier) Send(ctx context.Context, notification *Notification) error {
	// Wrap execution in circuit breaker
	_, err := w.circuitBreaker.Execute(func() (interface{}, error) {
		return nil, w.sendInternal(ctx, notification)
	})
	return err
}

func (w *WebhookNotifier) sendInternal(ctx context.Context, notification *Notification) error {
	// Build webhook message based on URL patterns
	var msg interface{}

	url := strings.ToLower(w.config.WebhookURL)
	switch {
	case strings.Contains(url, "discord.com"):
		msg = w.buildDiscordMessage(notification)
	case strings.Contains(url, "webhook.office.com"):
		msg = w.buildTeamsMessage(notification)
	default:
		// Generic format (compatible with most webhooks)
		msg = w.buildGenericMessage(notification)
	}

	// Marshal to JSON
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook message: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, w.config.Method, w.config.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for key, value := range w.config.Headers {
		req.Header.Set(key, value)
	}

	// Send request
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook notification: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-success status: %d", resp.StatusCode)
	}

	w.log.WithFields(logrus.Fields{
		"notifier": w.Name(),
		"level":    notification.Level,
		"title":    notification.Title,
	}).Debug("notification sent successfully")

	return nil
}

// Health checks if the webhook is configured.
func (w *WebhookNotifier) Health(ctx context.Context) error {
	if w.config.WebhookURL == "" {
		return fmt.Errorf("webhook URL not configured")
	}
	return nil
}

// buildDiscordMessage builds a Discord-compatible message.
func (w *WebhookNotifier) buildDiscordMessage(notification *Notification) *webhookMessage {
	// Build fields
	var fields []webhookField
	for key, value := range notification.Fields {
		fields = append(fields, webhookField{
			Name:   key,
			Value:  value,
			Inline: true,
		})
	}

	// Create embed
	embed := webhookEmbed{
		Title:       notification.Title,
		Description: notification.Message,
		Color:       hexToDecimal(notification.Level.Color()),
		Fields:      fields,
		Footer: &webhookFooter{
			Text: "Lumo",
		},
		Timestamp: notification.Timestamp.Format(time.RFC3339),
	}

	return &webhookMessage{
		Content:  fmt.Sprintf("%s %s", notification.Level.Icon(), notification.Title),
		Username: "Lumo",
		Embeds:   []webhookEmbed{embed},
	}
}

// buildTeamsMessage builds a Microsoft Teams-compatible message.
func (w *WebhookNotifier) buildTeamsMessage(notification *Notification) *webhookMessage {
	// Build facts
	var facts []webhookField
	for key, value := range notification.Fields {
		facts = append(facts, webhookField{
			Name:  key,
			Value: value,
		})
	}

	// Create section
	section := webhookSection{
		ActivityTitle: notification.Title,
		ActivityText:  notification.Message,
		Facts:         facts,
	}

	return &webhookMessage{
		Title:      notification.Title,
		Summary:    notification.Title,
		Text:       notification.Message,
		Sections:   []webhookSection{section},
		ThemeColor: strings.TrimPrefix(notification.Level.Color(), "#"),
	}
}

// buildGenericMessage builds a generic message format.
func (w *WebhookNotifier) buildGenericMessage(notification *Notification) map[string]interface{} {
	msg := map[string]interface{}{
		"title":     notification.Title,
		"message":   notification.Message,
		"level":     notification.Level.String(),
		"timestamp": notification.Timestamp.Format(time.RFC3339),
	}

	if len(notification.Fields) > 0 {
		msg["fields"] = notification.Fields
	}

	if len(notification.Tags) > 0 {
		msg["tags"] = notification.Tags
	}

	return msg
}

// hexToDecimal converts a hex color string to decimal.
func hexToDecimal(hex string) int {
	hex = strings.TrimPrefix(hex, "#")
	var value int
	_, _ = fmt.Sscanf(hex, "%x", &value)
	return value
}
