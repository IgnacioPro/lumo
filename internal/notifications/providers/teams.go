package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ignacio/lumo/internal/notifications"
	"github.com/sirupsen/logrus"
)

// TeamsProvider implements the Provider interface for Microsoft Teams notifications
type TeamsProvider struct {
	webhookURL string
	logger     *logrus.Logger
}

// TeamsConfig contains configuration for the Teams provider
type TeamsConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	WebhookURL string `mapstructure:"webhook_url"`
}

// teamsMessage represents a Microsoft Teams message card
type teamsMessage struct {
	Type       string            `json:"@type"`
	Context    string            `json:"@context"`
	ThemeColor string            `json:"themeColor"`
	Summary    string            `json:"summary"`
	Sections   []teamsSection    `json:"sections"`
	PotentialAction []teamsAction `json:"potentialAction,omitempty"`
}

// teamsSection represents a section in a Teams message card
type teamsSection struct {
	ActivityTitle    string      `json:"activityTitle,omitempty"`
	ActivitySubtitle string      `json:"activitySubtitle,omitempty"`
	ActivityImage    string      `json:"activityImage,omitempty"`
	Facts            []teamsFact `json:"facts,omitempty"`
	Text             string      `json:"text,omitempty"`
	Markdown         bool        `json:"markdown,omitempty"`
}

// teamsFact represents a key-value fact in a Teams section
type teamsFact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// teamsAction represents an action button in Teams
type teamsAction struct {
	Type    string         `json:"@type"`
	Name    string         `json:"name"`
	Targets []teamsTarget  `json:"targets,omitempty"`
}

// teamsTarget represents an action target
type teamsTarget struct {
	OS  string `json:"os"`
	URI string `json:"uri"`
}

// NewTeamsProvider creates a new Microsoft Teams notification provider
func NewTeamsProvider(config *TeamsConfig, logger *logrus.Logger) (*TeamsProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.WebhookURL == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}

	if logger == nil {
		logger = logrus.New()
	}

	return &TeamsProvider{
		webhookURL: config.WebhookURL,
		logger:     logger,
	}, nil
}

// Name returns the provider name
func (t *TeamsProvider) Name() string {
	return "teams"
}

// Send sends a notification to Microsoft Teams
func (t *TeamsProvider) Send(ctx context.Context, msg *notifications.Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	payload := t.formatMessage(msg)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Teams message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.webhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Teams API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Health checks if the Teams provider is properly configured
func (t *TeamsProvider) Health(ctx context.Context) error {
	if t.webhookURL == "" {
		return fmt.Errorf("webhook URL is not configured")
	}

	// Verify URL format
	if len(t.webhookURL) < 10 || t.webhookURL[:8] != "https://" {
		return fmt.Errorf("invalid webhook URL format")
	}

	return nil
}

// formatMessage converts a notification message to Teams MessageCard format
func (t *TeamsProvider) formatMessage(msg *notifications.Message) *teamsMessage {
	card := &teamsMessage{
		Type:       "MessageCard",
		Context:    "http://schema.org/extensions",
		ThemeColor: t.getSeverityColor(msg),
		Summary:    msg.Title,
		Sections:   []teamsSection{},
	}

	// Main section with title and body
	mainSection := teamsSection{
		ActivityTitle:    fmt.Sprintf("%s %s", msg.SeverityEmoji(), msg.Title),
		ActivitySubtitle: fmt.Sprintf("Source: %s", msg.Source),
		Text:             msg.Body,
		Markdown:         true,
	}

	// Build facts
	facts := []teamsFact{
		{
			Name:  "Severity",
			Value: string(msg.Severity),
		},
		{
			Name:  "Timestamp",
			Value: msg.Timestamp.Format("2006-01-02 15:04:05 MST"),
		},
	}

	// Add host information
	if msg.HostInfo != nil {
		if msg.HostInfo.Hostname != "" {
			facts = append(facts, teamsFact{
				Name:  "Hostname",
				Value: msg.HostInfo.Hostname,
			})
		}
		if msg.HostInfo.IP != "" {
			facts = append(facts, teamsFact{
				Name:  "IP Address",
				Value: msg.HostInfo.IP,
			})
		}
		if msg.HostInfo.Platform != "" {
			facts = append(facts, teamsFact{
				Name:  "Platform",
				Value: msg.HostInfo.Platform,
			})
		}
		if msg.HostInfo.OS != "" {
			facts = append(facts, teamsFact{
				Name:  "Operating System",
				Value: msg.HostInfo.OS,
			})
		}
	}

	mainSection.Facts = facts
	card.Sections = append(card.Sections, mainSection)

	// Additional fields section
	if len(msg.Fields) > 0 {
		fieldFacts := make([]teamsFact, 0, len(msg.Fields))
		for _, field := range msg.Fields {
			fieldFacts = append(fieldFacts, teamsFact{
				Name:  field.Name,
				Value: field.Value,
			})
		}

		fieldsSection := teamsSection{
			ActivityTitle: "Additional Details",
			Facts:         fieldFacts,
		}

		card.Sections = append(card.Sections, fieldsSection)
	}

	return card
}

// getSeverityColor returns the theme color for a severity level
func (t *TeamsProvider) getSeverityColor(msg *notifications.Message) string {
	// Remove the # prefix as Teams expects it without
	color := msg.SeverityColor()
	if len(color) > 0 && color[0] == '#' {
		return color[1:]
	}
	return color
}
