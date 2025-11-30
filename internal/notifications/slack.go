package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/reliability"
)

// SlackNotifier sends notifications via Slack webhooks.
type SlackNotifier struct {
	config         *NotifierConfig
	log            *logrus.Logger
	client         *http.Client
	circuitBreaker *reliability.CircuitBreaker
	apiBaseURL     string // Base URL for viewing full analysis
}

// slackMessage represents a Slack message payload using Block Kit.
type slackMessage struct {
	Text        string                   `json:"text,omitempty"`
	Attachments []slackAttachment        `json:"attachments,omitempty"`
	Blocks      []map[string]interface{} `json:"blocks,omitempty"`
	Unfurl      bool                     `json:"unfurl_links,omitempty"`
	Channel     string                   `json:"channel,omitempty"`   // For API messages
	ThreadTS    string                   `json:"thread_ts,omitempty"` // For threaded replies
}

// slackResponse represents a Slack API response
type slackResponse struct {
	OK      bool   `json:"ok"`
	TS      string `json:"ts"` // Message timestamp
	Channel string `json:"channel"`
	Error   string `json:"error,omitempty"`
}

// slackAttachment represents a Slack message attachment (legacy fallback).
type slackAttachment struct {
	Color  string                   `json:"color,omitempty"`
	Title  string                   `json:"title,omitempty"`
	Text   string                   `json:"text,omitempty"`
	Fields []slackField             `json:"fields,omitempty"`
	Footer string                   `json:"footer,omitempty"`
	Ts     int64                    `json:"ts,omitempty"`
	Blocks []map[string]interface{} `json:"blocks,omitempty"`
}

// slackField represents a field in a Slack attachment.
type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlackNotifier creates a new Slack notifier.
func NewSlackNotifier(config *NotifierConfig, log *logrus.Logger) (*SlackNotifier, error) {
	// Either webhook URL or bot token + channel is required
	if config.WebhookURL == "" && (config.SlackBotToken == "" || config.SlackChannel == "") {
		return nil, fmt.Errorf("slack requires either webhook_url OR (bot_token + channel)")
	}

	// Default API base URL (can be overridden via config)
	apiBaseURL := "http://localhost:8080"
	if config.APIBaseURL != "" {
		apiBaseURL = config.APIBaseURL
	}

	return &SlackNotifier{
		config:         config,
		log:            log,
		client:         NewHTTPClientFromSeconds(config.Timeout),
		circuitBreaker: reliability.NewCircuitBreaker(fmt.Sprintf("slack-%s", config.Name)),
		apiBaseURL:     apiBaseURL,
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
	// Check if we're using Bot Token (supports threading) or Webhook (legacy)
	useBotToken := s.config.SlackBotToken != "" && s.config.SlackChannel != ""

	if useBotToken {
		return s.sendWithBotToken(ctx, notification)
	}
	return s.sendWithWebhook(ctx, notification)
}

// sendWithWebhook sends via incoming webhook (legacy, no threading support)
func (s *SlackNotifier) sendWithWebhook(ctx context.Context, notification *Notification) error {
	msg := s.buildMessage(notification)
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.config.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned non-OK status: %d", resp.StatusCode)
	}

	s.log.WithFields(logrus.Fields{
		"notifier": s.Name(),
		"level":    notification.Level,
		"title":    notification.Title,
	}).Debug("notification sent successfully (webhook)")

	return nil
}

// sendWithBotToken sends via Slack Web API (supports threading)
func (s *SlackNotifier) sendWithBotToken(ctx context.Context, notification *Notification) error {
	// Send initial summary message
	msg := s.buildMessage(notification)
	msg.Channel = s.config.SlackChannel

	ts, err := s.postMessage(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to post initial message: %w", err)
	}

	s.log.WithFields(logrus.Fields{
		"notifier": s.Name(),
		"level":    notification.Level,
		"title":    notification.Title,
		"ts":       ts,
	}).Debug("initial notification sent successfully (bot token)")

	// If there's AI analysis, send it as a threaded reply
	sections := s.parseMessageSections(notification.Message)
	if sections["ai_analysis"] != "" {
		if err := s.sendAIAnalysisThread(ctx, ts, sections["ai_analysis"]); err != nil {
			s.log.WithError(err).Warn("Failed to send AI analysis thread")
			// Don't fail the whole notification if thread fails
		}
	}

	return nil
}

// postMessage posts a message to Slack Web API and returns the message timestamp
func (s *SlackNotifier) postMessage(ctx context.Context, msg *slackMessage) (string, error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://slack.com/api/chat.postMessage", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.SlackBotToken))

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var slackResp slackResponse
	if err := json.NewDecoder(resp.Body).Decode(&slackResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if !slackResp.OK {
		return "", fmt.Errorf("slack API error: %s", slackResp.Error)
	}

	return slackResp.TS, nil
}

// sendAIAnalysisThread sends the full AI analysis as a threaded reply
func (s *SlackNotifier) sendAIAnalysisThread(ctx context.Context, threadTS string, aiAnalysis string) error {
	// Build a simpler message for the thread with the full AI analysis
	threadMsg := &slackMessage{
		Channel:  s.config.SlackChannel,
		ThreadTS: threadTS,
		Text:     "🤖 *Full AI Analysis*",
		Blocks: []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]interface{}{
					"type":  "plain_text",
					"text":  "🤖 Complete AI-Powered Analysis",
					"emoji": true,
				},
			},
			{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": aiAnalysis,
				},
			},
		},
	}

	_, err := s.postMessage(ctx, threadMsg)
	return err
}

// Health checks if the Slack webhook is configured.
func (s *SlackNotifier) Health(ctx context.Context) error {
	useBotToken := s.config.SlackBotToken != "" && s.config.SlackChannel != ""
	if !useBotToken && s.config.WebhookURL == "" {
		return fmt.Errorf("slack not configured (need webhook_url OR bot_token+channel)")
	}
	return nil
}

// buildMessage builds a Slack message from a notification using Block Kit for enhanced UX.
func (s *SlackNotifier) buildMessage(notification *Notification) *slackMessage {
	blocks := s.buildBlocks(notification)

	// Fallback text for notifications preview
	fallbackText := fmt.Sprintf("%s %s", notification.Level.Icon(), notification.Title)

	// Legacy attachment for color bar (Block Kit doesn't support colored borders)
	attachment := slackAttachment{
		Color:  notification.Level.Color(),
		Blocks: blocks,
	}

	return &slackMessage{
		Text:        fallbackText,
		Attachments: []slackAttachment{attachment},
		Unfurl:      false,
	}
}

// buildBlocks creates Block Kit blocks for rich Slack message formatting.
func (s *SlackNotifier) buildBlocks(notification *Notification) []map[string]interface{} {
	blocks := []map[string]interface{}{}

	// Header block with severity emoji and title
	blocks = append(blocks, map[string]interface{}{
		"type": "header",
		"text": map[string]interface{}{
			"type":  "plain_text",
			"text":  fmt.Sprintf("%s %s", notification.Level.Icon(), notification.Title),
			"emoji": true,
		},
	})

	// Context block with timestamp and tags
	contextElements := []map[string]interface{}{
		{
			"type": "mrkdwn",
			"text": fmt.Sprintf("*Severity:* %s `%s`", notification.Level.Icon(), notification.Level),
		},
		{
			"type": "mrkdwn",
			"text": fmt.Sprintf("*Time:* <!date^%d^{date_num} {time_secs}|%s>",
				notification.Timestamp.Unix(),
				notification.Timestamp.Format("2006-01-02 15:04:05")),
		},
	}

	// Add tags if available
	if len(notification.Tags) > 0 {
		tagText := ""
		for _, tag := range notification.Tags {
			tagText += fmt.Sprintf("`%s` ", tag)
		}
		contextElements = append(contextElements, map[string]interface{}{
			"type": "mrkdwn",
			"text": tagText,
		})
	}

	blocks = append(blocks, map[string]interface{}{
		"type":     "context",
		"elements": contextElements,
	})

	// Main message section
	messageText := notification.Message

	// Parse message to extract structured sections (AI analysis, etc.)
	sections := s.parseMessageSections(messageText)

	if sections["details"] != "" {
		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": sections["details"],
			},
		})
	}

	// Add fields from notification.Fields
	if len(notification.Fields) > 0 {
		fields := []map[string]interface{}{}
		for key, value := range notification.Fields {
			fields = append(fields, map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s*\n%s", key, value),
			})
		}

		blocks = append(blocks, map[string]interface{}{
			"type":   "section",
			"fields": fields,
		})
	}

	// AI Analysis section (if present in message)
	if sections["ai_analysis"] != "" {
		blocks = append(blocks, map[string]interface{}{
			"type": "divider",
		})

		// Build AI analysis text with truncation notice
		aiText := fmt.Sprintf("*:robot_face: AI-Powered Analysis*\n%s", sections["ai_analysis"])
		if sections["ai_truncated"] == "true" {
			aiText += "\n\n_Analysis truncated for display. Click below to view full analysis._"
		}

		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": aiText,
			},
		})

		// Add "View Full Analysis" button if we have the event URL
		if eventURL, ok := notification.Fields["_event_url"]; ok {
			fullURL := s.apiBaseURL + eventURL
			blocks = append(blocks, map[string]interface{}{
				"type": "actions",
				"elements": []map[string]interface{}{
					{
						"type": "button",
						"text": map[string]interface{}{
							"type": "plain_text",
							"text": "📊 View Full Analysis",
						},
						"url":   fullURL,
						"style": "primary",
					},
				},
			})
		}
	}

	// Footer with branding
	blocks = append(blocks, map[string]interface{}{
		"type": "context",
		"elements": []map[string]interface{}{
			{
				"type": "mrkdwn",
				"text": fmt.Sprintf(":zap: *Lumo* | Intelligent SRE Automation | <%s|View Details>",
					s.getDashboardURL(notification)),
			},
		},
	})

	return blocks
}

// parseMessageSections splits the notification message into structured sections.
func (s *SlackNotifier) parseMessageSections(message string) map[string]string {
	sections := make(map[string]string)

	// Look for AI Analysis section marker
	if idx := strings.Index(message, "*AI Analysis:*"); idx != -1 {
		sections["details"] = strings.TrimSpace(message[:idx])
		aiAnalysis := strings.TrimSpace(message[idx+len("*AI Analysis:*"):])
		aiAnalysis = strings.TrimPrefix(aiAnalysis, "\n")

		// Truncate AI analysis if it's too long for Slack (max 3000 chars per block)
		// We use 1500 chars to be safe and leave room for formatting
		const maxAILength = 1500
		if len(aiAnalysis) > maxAILength {
			sections["ai_analysis"] = aiAnalysis[:maxAILength] + "..."
			sections["ai_truncated"] = "true"
		} else {
			sections["ai_analysis"] = aiAnalysis
			sections["ai_truncated"] = "false"
		}
	} else {
		sections["details"] = message
	}

	return sections
}

// getDashboardURL generates a URL to the dashboard or event details.
// TODO: Make this configurable via NotifierConfig
func (s *SlackNotifier) getDashboardURL(notification *Notification) string {
	// For now, return a placeholder. In production, this would link to:
	// - Kubernetes dashboard
	// - Lumo web UI (when implemented)
	// - Cloud provider console
	return "#"
}
