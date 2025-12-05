package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

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
	slackAPIURL    string // Slack API URL (default: https://slack.com/api)

	// messageThreads stores the timestamp of the last message sent for a given entity ID
	// This allows us to update existing messages instead of sending new ones
	messageThreads map[string]string
	mu             sync.RWMutex
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
		slackAPIURL:    "https://slack.com/api",
		messageThreads: make(map[string]string),
	}, nil
}

// Name returns the notifier name.
func (s *SlackNotifier) Name() string {
	return s.config.Name
}

// Send sends a notification to Slack.
func (s *SlackNotifier) Send(ctx context.Context, notification *Notification) (string, error) {
	// Wrap execution in circuit breaker
	result, err := s.circuitBreaker.Execute(func() (interface{}, error) {
		return s.sendInternal(ctx, notification)
	})
	if err != nil {
		return "", err
	}
	if id, ok := result.(string); ok {
		return id, nil
	}
	return "", nil
}

func (s *SlackNotifier) sendInternal(ctx context.Context, notification *Notification) (string, error) {
	// Check if we're using Bot Token (supports threading) or Webhook (legacy)
	useBotToken := s.config.SlackBotToken != "" && s.config.SlackChannel != ""

	if useBotToken {
		return s.sendWithBotToken(ctx, notification)
	}
	return "", s.sendWithWebhook(ctx, notification)
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
func (s *SlackNotifier) sendWithBotToken(ctx context.Context, notification *Notification) (string, error) {
	// Send initial summary message
	msg := s.buildMessage(notification)
	msg.Channel = s.config.SlackChannel

	var ts string
	var err error
	var updated bool

	// Check if we should update an existing message or reply to a thread
	var existingTS string
	var exists bool

	// 1. Check explicit ThreadTS from notification (highest priority)
	if notification.ThreadTS != "" {
		existingTS = notification.ThreadTS
		exists = true
	} else if eventID, ok := notification.Fields["event_id"]; ok && eventID != "" {
		// 2. Check internal map for event_id
		s.mu.RLock()
		existingTS, exists = s.messageThreads[eventID]
		s.mu.RUnlock()
	}

	// Handle Threaded Reply
	if notification.IsReply && exists {
		msg.ThreadTS = existingTS
		// For replies, we usually want to broadcast to channel if critical, but for now just thread
		ts, err = s.postMessage(ctx, msg)
		if err != nil {
			return "", fmt.Errorf("failed to post reply: %w", err)
		}
		// Return the thread parent TS, not the reply TS, to keep context?
		// Or return the reply TS? The interface returns "string" which is usually the ID of the sent message.
		return ts, nil
	}

	// Handle Update or New Message
	if exists {
		// Try to update the existing message
		ts, err = s.updateMessage(ctx, existingTS, msg)
		if err == nil {
			updated = true
		} else {
			// If update fails (e.g. message deleted), log it and fall back to new message
			s.log.WithError(err).WithField("ts", existingTS).Warn("failed to update slack message, sending new one")
		}
	}

	// If not updated (or update failed), send a new message
	if !updated {
		ts, err = s.postMessage(ctx, msg)
		if err != nil {
			return "", fmt.Errorf("failed to post initial message: %w", err)
		}

		// Store the timestamp if we have an event ID
		if eventID, ok := notification.Fields["event_id"]; ok && eventID != "" {
			s.mu.Lock()
			s.messageThreads[eventID] = ts
			s.mu.Unlock()
		}
	}

	s.log.WithFields(logrus.Fields{
		"notifier": s.Name(),
		"level":    notification.Level,
		"title":    notification.Title,
		"ts":       ts,
		"updated":  updated,
		"is_reply": notification.IsReply,
	}).Debug("notification sent successfully (bot token)")

	// If there's AI analysis in the message, send it as a threaded reply
	sections := s.parseMessageSections(notification.Message)
	if sections["ai_analysis"] != "" {
		if err := s.sendAIAnalysisThread(ctx, ts, sections["ai_analysis"]); err != nil {
			s.log.WithError(err).Warn("Failed to send AI analysis thread")
			// Don't fail the whole notification if thread fails
		}
	}

	// If there's a postmortem, send it as threaded replies (splitting if needed)
	if notification.Postmortem != "" {
		if err := s.sendPostmortemThread(ctx, ts, notification.Postmortem); err != nil {
			s.log.WithError(err).Warn("Failed to send postmortem thread")
			// Don't fail the whole notification if thread fails
		}
	}

	return ts, nil
}

// sendPostmortemThread sends the postmortem as threaded replies, splitting into multiple messages if needed
func (s *SlackNotifier) sendPostmortemThread(ctx context.Context, threadTS string, postmortem string) error {
	// Slack has a 3000 character limit per block text, but we use a safer limit
	const maxChunkSize = 2800

	// Split postmortem into chunks by sections (prefer splitting at ## headers)
	chunks := s.splitPostmortemIntoChunks(postmortem, maxChunkSize)

	s.log.WithFields(logrus.Fields{
		"thread_ts":   threadTS,
		"total_chars": len(postmortem),
		"num_chunks":  len(chunks),
	}).Debug("Sending postmortem as threaded replies")

	for i, chunk := range chunks {
		var headerText string
		if i == 0 {
			headerText = "📋 Incident Postmortem"
		} else {
			headerText = fmt.Sprintf("📋 Postmortem (continued %d/%d)", i+1, len(chunks))
		}

		threadMsg := &slackMessage{
			Channel:  s.config.SlackChannel,
			ThreadTS: threadTS,
			Text:     headerText,
			Blocks: []map[string]interface{}{
				{
					"type": "header",
					"text": map[string]interface{}{
						"type":  "plain_text",
						"text":  headerText,
						"emoji": true,
					},
				},
				{
					"type": "section",
					"text": map[string]interface{}{
						"type": "mrkdwn",
						"text": chunk,
					},
				},
			},
		}

		if _, err := s.postMessage(ctx, threadMsg); err != nil {
			return fmt.Errorf("failed to send postmortem chunk %d: %w", i+1, err)
		}
	}

	return nil
}

// splitPostmortemIntoChunks splits the postmortem into chunks, preferring to split at section boundaries
func (s *SlackNotifier) splitPostmortemIntoChunks(postmortem string, maxSize int) []string {
	if len(postmortem) <= maxSize {
		return []string{postmortem}
	}

	var chunks []string
	lines := strings.Split(postmortem, "\n")
	var currentChunk strings.Builder

	for _, line := range lines {
		// Check if adding this line would exceed the limit
		if currentChunk.Len()+len(line)+1 > maxSize {
			// Save current chunk if it has content
			if currentChunk.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
				currentChunk.Reset()
			}

			// If a single line is longer than maxSize, split it
			if len(line) > maxSize {
				for len(line) > maxSize {
					chunks = append(chunks, line[:maxSize])
					line = line[maxSize:]
				}
				if len(line) > 0 {
					currentChunk.WriteString(line)
					currentChunk.WriteString("\n")
				}
				continue
			}
		}

		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")

		// Prefer to split at section headers (## )
		if strings.HasPrefix(line, "## ") && currentChunk.Len() > maxSize/2 {
			// Save current chunk before the header and start new chunk with header
			content := currentChunk.String()
			headerIdx := strings.LastIndex(content, line)
			if headerIdx > 0 {
				chunks = append(chunks, strings.TrimSpace(content[:headerIdx]))
				currentChunk.Reset()
				currentChunk.WriteString(line)
				currentChunk.WriteString("\n")
			}
		}
	}

	// Don't forget the last chunk
	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}
func (s *SlackNotifier) postMessage(ctx context.Context, msg *slackMessage) (string, error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat.postMessage", s.slackAPIURL), bytes.NewReader(payload))
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

// updateMessage updates an existing Slack message
func (s *SlackNotifier) updateMessage(ctx context.Context, ts string, msg *slackMessage) (string, error) {
	msg.ThreadTS = ts // Keep the timestamp for update
	payload, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}

	// Add ts to the payload for update
	// Note: slackMessage struct doesn't have TS field at top level for update, so we might need a wrapper or map
	// but let's check the API. chat.update requires "channel", "ts", "text"/"blocks".
	// Our slackMessage has Channel. We need to inject TS.
	var updatePayload map[string]interface{}
	if err := json.Unmarshal(payload, &updatePayload); err != nil {
		return "", err
	}
	updatePayload["ts"] = ts

	finalPayload, err := json.Marshal(updatePayload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat.update", s.slackAPIURL), bytes.NewReader(finalPayload))
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

	// Add recommended action buttons if available
	if len(notification.Actions) > 0 {
		blocks = append(blocks, map[string]interface{}{
			"type": "divider",
		})

		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": "*🛠 Recommended Actions*",
			},
		})

		// Build action buttons (max 5 per Slack limit)
		actionElements := []map[string]interface{}{}
		maxActions := len(notification.Actions)
		if maxActions > 5 {
			maxActions = 5
		}

		for i := 0; i < maxActions; i++ {
			action := notification.Actions[i]
			buttonStyle := "primary"
			if action.Destructive {
				buttonStyle = "danger"
			}

			button := map[string]interface{}{
				"type": "button",
				"text": map[string]interface{}{
					"type":  "plain_text",
					"text":  fmt.Sprintf("%d. %s", i+1, truncateString(action.Title, 30)),
					"emoji": true,
				},
				"style": buttonStyle,
			}

			// If URL is provided, use it. Otherwise, show command as tooltip
			if action.URL != "" {
				button["url"] = action.URL
			} else if action.Command != "" {
				// Use action_id to identify this for potential future interaction handling
				button["action_id"] = fmt.Sprintf("action_%d", i)
				// Add confirm dialog for destructive actions
				if action.Destructive {
					button["confirm"] = map[string]interface{}{
						"title": map[string]interface{}{
							"type": "plain_text",
							"text": "⚠️ Confirm Action",
						},
						"text": map[string]interface{}{
							"type": "mrkdwn",
							"text": fmt.Sprintf("This action may be destructive:\n```%s```\nProceed?", action.Command),
						},
						"confirm": map[string]interface{}{
							"type": "plain_text",
							"text": "Execute",
						},
						"deny": map[string]interface{}{
							"type": "plain_text",
							"text": "Cancel",
						},
					}
				}
			}

			actionElements = append(actionElements, button)
		}

		if len(actionElements) > 0 {
			blocks = append(blocks, map[string]interface{}{
				"type":     "actions",
				"elements": actionElements,
			})
		}

		// Show commands as text for copy-paste
		if len(notification.Actions) > 0 && notification.Actions[0].Command != "" {
			var cmdText strings.Builder
			cmdText.WriteString("*Commands (copy & paste):*\n```\n")
			for i, action := range notification.Actions {
				if i >= 3 {
					cmdText.WriteString("# ... more commands in full analysis\n")
					break
				}
				if action.Command != "" {
					cmdText.WriteString(fmt.Sprintf("# %s\n%s\n\n", action.Title, action.Command))
				}
			}
			cmdText.WriteString("```")

			blocks = append(blocks, map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": cmdText.String(),
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

// truncateString truncates a string to max length
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
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
