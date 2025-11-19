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

// SlackProvider implements the Provider interface for Slack notifications
type SlackProvider struct {
	webhookURL string
	channel    string
	username   string
	iconEmoji  string
	logger     *logrus.Logger
}

// SlackConfig contains configuration for the Slack provider
type SlackConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	WebhookURL string `mapstructure:"webhook_url"`
	Channel    string `mapstructure:"channel"`
	Username   string `mapstructure:"username"`
	IconEmoji  string `mapstructure:"icon_emoji"`
}

// slackMessage represents the JSON payload sent to Slack
type slackMessage struct {
	Channel   string       `json:"channel,omitempty"`
	Username  string       `json:"username,omitempty"`
	IconEmoji string       `json:"icon_emoji,omitempty"`
	Blocks    []slackBlock `json:"blocks"`
}

// slackBlock represents a Slack Block Kit block
type slackBlock struct {
	Type      string               `json:"type"`
	Text      *slackText           `json:"text,omitempty"`
	Fields    []slackText          `json:"fields,omitempty"`
	Accessory *slackBlockAccessory `json:"accessory,omitempty"`
	Elements  []slackElement       `json:"elements,omitempty"`
}

// slackText represents text with formatting
type slackText struct {
	Type  string `json:"type"` // plain_text or mrkdwn
	Text  string `json:"text"`
	Emoji *bool  `json:"emoji,omitempty"`
}

// slackBlockAccessory represents an accessory element in a block
type slackBlockAccessory struct {
	Type     string `json:"type"`
	ImageURL string `json:"image_url,omitempty"`
	AltText  string `json:"alt_text,omitempty"`
}

// slackElement represents an element in a context block
type slackElement struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	AltText  string `json:"alt_text,omitempty"`
}

// NewSlackProvider creates a new Slack notification provider
func NewSlackProvider(config *SlackConfig, logger *logrus.Logger) (*SlackProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.WebhookURL == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}

	if logger == nil {
		logger = logrus.New()
	}

	// Set defaults
	username := config.Username
	if username == "" {
		username = "Lumo Bot"
	}

	iconEmoji := config.IconEmoji
	if iconEmoji == "" {
		iconEmoji = ":robot_face:"
	}

	return &SlackProvider{
		webhookURL: config.WebhookURL,
		channel:    config.Channel,
		username:   username,
		iconEmoji:  iconEmoji,
		logger:     logger,
	}, nil
}

// Name returns the provider name
func (s *SlackProvider) Name() string {
	return "slack"
}

// Send sends a notification to Slack
func (s *SlackProvider) Send(ctx context.Context, msg *notifications.Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	payload := s.formatMessage(msg)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewReader(jsonData))
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

	if resp.StatusCode == 429 {
		return fmt.Errorf("rate limited by Slack")
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Slack API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Health checks if the Slack provider is properly configured
func (s *SlackProvider) Health(ctx context.Context) error {
	if s.webhookURL == "" {
		return fmt.Errorf("webhook URL is not configured")
	}

	// Slack doesn't provide a health check endpoint, so we just verify the URL format
	if len(s.webhookURL) < 10 || s.webhookURL[:8] != "https://" {
		return fmt.Errorf("invalid webhook URL format")
	}

	return nil
}

// formatMessage converts a notification message to Slack Block Kit format
func (s *SlackProvider) formatMessage(msg *notifications.Message) *slackMessage {
	blocks := make([]slackBlock, 0)

	// Header block with emoji and title
	emoji := msg.SeverityEmoji()
	headerText := fmt.Sprintf("%s %s", emoji, msg.Title)

	blocks = append(blocks, slackBlock{
		Type: "header",
		Text: &slackText{
			Type:  "plain_text",
			Text:  headerText,
			Emoji: boolPtr(true),
		},
	})

	// Body section
	blocks = append(blocks, slackBlock{
		Type: "section",
		Text: &slackText{
			Type: "mrkdwn",
			Text: msg.Body,
		},
	})

	// Severity indicator with color-coded emoji
	severityText := fmt.Sprintf("*Severity:* %s %s", emoji, string(msg.Severity))
	blocks = append(blocks, slackBlock{
		Type: "section",
		Text: &slackText{
			Type: "mrkdwn",
			Text: severityText,
		},
	})

	// Host information (if available)
	if msg.HostInfo != nil {
		fields := []slackText{}

		if msg.HostInfo.Hostname != "" {
			fields = append(fields, slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*Hostname:*\n%s", msg.HostInfo.Hostname),
			})
		}

		if msg.HostInfo.IP != "" {
			fields = append(fields, slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*IP:*\n%s", msg.HostInfo.IP),
			})
		}

		if msg.HostInfo.Platform != "" {
			fields = append(fields, slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*Platform:*\n%s", msg.HostInfo.Platform),
			})
		}

		if msg.HostInfo.OS != "" {
			fields = append(fields, slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*OS:*\n%s", msg.HostInfo.OS),
			})
		}

		if len(fields) > 0 {
			blocks = append(blocks, slackBlock{
				Type:   "section",
				Fields: fields,
			})
		}
	}

	// Additional fields
	if len(msg.Fields) > 0 {
		fields := make([]slackText, 0, len(msg.Fields))
		for _, field := range msg.Fields {
			fields = append(fields, slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*%s:*\n%s", field.Name, field.Value),
			})
		}

		blocks = append(blocks, slackBlock{
			Type:   "section",
			Fields: fields,
		})
	}

	// Divider
	blocks = append(blocks, slackBlock{
		Type: "divider",
	})

	// Context footer
	footerText := fmt.Sprintf("🤖 Automated alert from Lumo | Source: %s | %s",
		msg.Source,
		msg.Timestamp.Format("2006-01-02 15:04:05 MST"))

	blocks = append(blocks, slackBlock{
		Type: "context",
		Elements: []slackElement{
			{
				Type: "mrkdwn",
				Text: footerText,
			},
		},
	})

	slackMsg := &slackMessage{
		Username:  s.username,
		IconEmoji: s.iconEmoji,
		Blocks:    blocks,
	}

	if s.channel != "" {
		slackMsg.Channel = s.channel
	}

	return slackMsg
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}
