package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/reliability"
)

// SlackNotifier sends notifications via Slack webhooks.
type SlackNotifier struct {
	config         *NotifierConfig
	log            *logrus.Logger
	client         *http.Client
	circuitBreaker *reliability.CircuitBreaker
}

// slackMessage represents a Slack message payload.
type slackMessage struct {
	Text        string                   `json:"text,omitempty"`
	Attachments []slackAttachment        `json:"attachments,omitempty"`
	Blocks      []map[string]interface{} `json:"blocks,omitempty"`
}

// slackAttachment represents a Slack message attachment.
type slackAttachment struct {
	Color  string       `json:"color,omitempty"`
	Title  string       `json:"title,omitempty"`
	Text   string       `json:"text,omitempty"`
	Fields []slackField `json:"fields,omitempty"`
	Footer string       `json:"footer,omitempty"`
	Ts     int64        `json:"ts,omitempty"`
}

// slackField represents a field in a Slack attachment.
type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlackNotifier creates a new Slack notifier.
func NewSlackNotifier(config *NotifierConfig, log *logrus.Logger) (*SlackNotifier, error) {
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("slack webhook URL is required")
	}

	return &SlackNotifier{
		config:         config,
		log:            log,
		client:         NewHTTPClientFromSeconds(config.Timeout),
		circuitBreaker: reliability.NewCircuitBreaker(fmt.Sprintf("slack-%s", config.Name)),
	}, nil
}

// Name returns the notifier name.
func (s *SlackNotifier) Name() string {
	return s.config.Name
}

// Send sends a notification to Slack.
func (s *SlackNotifier) Send(ctx context.Context, notification *Notification) error {
	// Wrap execution in circuit breaker
	_, err := s.circuitBreaker.Execute(func() (interface{}, error) {
		return nil, s.sendInternal(ctx, notification)
	})
	return err
}

func (s *SlackNotifier) sendInternal(ctx context.Context, notification *Notification) error {
	// Build Slack message
	msg := s.buildMessage(notification)

	// Marshal to JSON
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", s.config.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack notification: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned non-OK status: %d", resp.StatusCode)
	}

	s.log.WithFields(logrus.Fields{
		"notifier": s.Name(),
		"level":    notification.Level,
		"title":    notification.Title,
	}).Debug("notification sent successfully")

	return nil
}

// Health checks if the Slack webhook is configured.
func (s *SlackNotifier) Health(ctx context.Context) error {
	if s.config.WebhookURL == "" {
		return fmt.Errorf("slack webhook URL not configured")
	}
	return nil
}

// buildMessage builds a Slack message from a notification.
func (s *SlackNotifier) buildMessage(notification *Notification) *slackMessage {
	// Build fields from notification fields
	var fields []slackField
	for key, value := range notification.Fields {
		fields = append(fields, slackField{
			Title: key,
			Value: value,
			Short: true,
		})
	}

	// Create attachment
	attachment := slackAttachment{
		Color:  notification.Level.Color(),
		Title:  notification.Title,
		Text:   notification.Message,
		Fields: fields,
		Footer: "Lumo",
		Ts:     notification.Timestamp.Unix(),
	}

	// Create message
	return &slackMessage{
		Text:        fmt.Sprintf("%s %s", notification.Level.Icon(), notification.Title),
		Attachments: []slackAttachment{attachment},
	}
}
