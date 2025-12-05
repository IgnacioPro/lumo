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

	var ts string
	var err error
	var updated bool

	// Check if we should update an existing message
	if eventID, ok := notification.Fields["event_id"]; ok && eventID != "" {
		s.mu.RLock()
		existingTS, exists := s.messageThreads[eventID]
		s.mu.RUnlock()

		if exists {
			// Try to update the existing message
			ts, err = s.updateMessage(ctx, existingTS, msg)
			if err == nil {
				updated = true
			} else {
				// If update fails (e.g. message deleted), log it and fall back to new message
				s.log.WithError(err).WithField("event_id", eventID).Warn("failed to update slack message, sending new one")
			}
		}
	}

	// If not updated (or update failed), send a new message
	if !updated {
		ts, err = s.postMessage(ctx, msg)
		if err != nil {
			return fmt.Errorf("failed to post initial message: %w", err)
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
	}).Debug("initial notification sent successfully (bot token)")

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

	return nil
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
// The main card is kept concise; full AI analysis goes to thread (bot mode) or is truncated (webhook).
func (s *SlackNotifier) buildBlocks(notification *Notification) []map[string]interface{} {
	// Check if this is an incident notification and use the new card layout
	if s.isIncidentNotification(notification) {
		return s.buildIncidentBlocks(notification)
	}

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

	// Add fields from notification.Fields (truncate values for conciseness)
	if len(notification.Fields) > 0 {
		fields := []map[string]interface{}{}
		for key, value := range notification.Fields {
			// Skip internal fields (prefixed with _)
			if strings.HasPrefix(key, "_") {
				continue
			}
			truncatedValue := truncateText(value, maxFieldValueLength)
			fields = append(fields, map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s*\n%s", key, truncatedValue),
			})
		}

		if len(fields) > 0 {
			blocks = append(blocks, map[string]interface{}{
				"type":   "section",
				"fields": fields,
			})
		}
	}

	// AI Analysis section: show only snippet in main card, full analysis goes to thread
	if sections["ai_snippet"] != "" {
		blocks = append(blocks, map[string]interface{}{
			"type": "divider",
		})

		// Build AI analysis snippet with CTA to view full analysis
		aiText := fmt.Sprintf("*:robot_face: AI Analysis Preview*\n%s", sections["ai_snippet"])

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

// Truncation constants for concise Slack messages
const (
	maxDetailsLength        = 450 // Main details truncation limit
	maxFieldValueLength     = 140 // Field value truncation limit
	maxAISnippetLength      = 300 // AI analysis snippet for main card
	maxAISnippetLines       = 3   // Max bullet lines in AI snippet
	maxWordBoundaryLookback = 50  // How far back to look for word boundary when truncating
	aiContinuationMessage   = "_…more in thread/analysis page_"
)

// truncateText truncates text to maxLength, appending ellipsis if truncated.
func truncateText(text string, maxLength int) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxLength {
		return text
	}
	// Find a good break point (space or newline) near maxLength
	breakPoint := maxLength
	for i := maxLength - 1; i > maxLength-maxWordBoundaryLookback && i > 0; i-- {
		if text[i] == ' ' || text[i] == '\n' {
			breakPoint = i
			break
		}
	}
	return strings.TrimSpace(text[:breakPoint]) + "…"
}

// extractAISnippet extracts a short snippet from AI analysis for the main card.
// Returns the first 3 bullet-ish lines or up to maxAISnippetLength chars.
func extractAISnippet(aiAnalysis string) string {
	lines := strings.Split(aiAnalysis, "\n")
	var snippetLines []string
	var snippetLen int

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Calculate actual length after joining (newlines between lines, not after)
		newlineLen := 0
		if len(snippetLines) > 0 {
			newlineLen = 1 // Only add newline cost if there are previous lines
		}
		// Check if adding this line would exceed our limits
		if len(snippetLines) >= maxAISnippetLines || snippetLen+newlineLen+len(trimmed) > maxAISnippetLength {
			break
		}
		snippetLines = append(snippetLines, trimmed)
		snippetLen += newlineLen + len(trimmed)
	}

	if len(snippetLines) == 0 {
		return truncateText(aiAnalysis, maxAISnippetLength)
	}

	snippet := strings.Join(snippetLines, "\n")
	// Add continuation indicator if there's more content
	if len(snippet) < len(strings.TrimSpace(aiAnalysis)) {
		snippet += "\n" + aiContinuationMessage
	}
	return snippet
}

// parseMessageSections splits the notification message into structured sections.
// Returns:
//   - "details": truncated details for main card
//   - "ai_analysis": full AI analysis for thread
//   - "ai_snippet": short AI snippet for main card
//   - "ai_truncated": "true" if AI was present
func (s *SlackNotifier) parseMessageSections(message string) map[string]string {
	sections := make(map[string]string)

	// Look for AI Analysis section marker
	if idx := strings.Index(message, "*AI Analysis:*"); idx != -1 {
		details := strings.TrimSpace(message[:idx])
		sections["details"] = truncateText(details, maxDetailsLength)

		aiAnalysis := strings.TrimSpace(message[idx+len("*AI Analysis:*"):])
		aiAnalysis = strings.TrimPrefix(aiAnalysis, "\n")

		// Store full AI analysis for thread
		sections["ai_analysis"] = aiAnalysis
		// Extract snippet for main card
		sections["ai_snippet"] = extractAISnippet(aiAnalysis)
		sections["ai_truncated"] = "true"
	} else {
		sections["details"] = truncateText(message, maxDetailsLength)
		sections["ai_truncated"] = "false"
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

// ============================================================================
// Block Kit Helpers for clean payload construction
// ============================================================================

// slackBlockMrkdwn creates a mrkdwn text object
func slackBlockMrkdwn(text string) map[string]interface{} {
	return map[string]interface{}{
		"type": "mrkdwn",
		"text": text,
	}
}

// slackBlockPlainText creates a plain_text object
func slackBlockPlainText(text string, emoji bool) map[string]interface{} {
	return map[string]interface{}{
		"type":  "plain_text",
		"text":  text,
		"emoji": emoji,
	}
}

// slackBlockButton creates a button element for actions
func slackBlockButton(text, actionID, value string, style string) map[string]interface{} {
	btn := map[string]interface{}{
		"type":      "button",
		"text":      slackBlockPlainText(text, true),
		"action_id": actionID,
		"value":     value,
	}
	if style != "" {
		btn["style"] = style
	}
	return btn
}

// slackBlockURLButton creates a button element with a URL
func slackBlockURLButton(text, url, style string) map[string]interface{} {
	btn := map[string]interface{}{
		"type": "button",
		"text": slackBlockPlainText(text, true),
		"url":  url,
	}
	if style != "" {
		btn["style"] = style
	}
	return btn
}

// slackBlockOverflow creates an overflow menu with options
func slackBlockOverflow(actionID string, options []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":      "overflow",
		"action_id": actionID,
		"options":   options,
	}
}

// slackBlockOverflowOption creates an option for overflow menu
func slackBlockOverflowOption(text, value string) map[string]interface{} {
	return map[string]interface{}{
		"text":  slackBlockPlainText(text, true),
		"value": value,
	}
}

// slackBlockHeader creates a header block
func slackBlockHeader(text string) map[string]interface{} {
	return map[string]interface{}{
		"type": "header",
		"text": slackBlockPlainText(text, true),
	}
}

// slackBlockSection creates a section block with text
func slackBlockSection(text string) map[string]interface{} {
	return map[string]interface{}{
		"type": "section",
		"text": slackBlockMrkdwn(text),
	}
}

// slackBlockSectionWithFields creates a section with field columns
func slackBlockSectionWithFields(fields []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":   "section",
		"fields": fields,
	}
}

// slackBlockContext creates a context block with elements
func slackBlockContext(elements []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":     "context",
		"elements": elements,
	}
}

// slackBlockDivider creates a divider block
func slackBlockDivider() map[string]interface{} {
	return map[string]interface{}{
		"type": "divider",
	}
}

// slackBlockActions creates an actions block with elements
func slackBlockActions(elements []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":     "actions",
		"elements": elements,
	}
}

// ============================================================================
// Incident Notification Helpers
// ============================================================================

// isIncidentNotification checks if this is an incident notification that should use
// the new card layout with action buttons
func (s *SlackNotifier) isIncidentNotification(notification *Notification) bool {
	// Check for incident_id field which is the primary indicator
	if _, ok := notification.Fields["incident_id"]; ok {
		return true
	}
	// Also check for fix_hash as a secondary indicator
	if _, ok := notification.Fields["fix_hash"]; ok {
		return true
	}
	return false
}

// getFieldSafe retrieves a field value safely, returning empty string if not present
func getFieldSafe(fields map[string]string, key string) string {
	if fields == nil {
		return ""
	}
	return fields[key]
}

// buildIncidentBlocks creates Block Kit blocks for incident notifications with
// action buttons for human-in-the-loop controls
func (s *SlackNotifier) buildIncidentBlocks(notification *Notification) []map[string]interface{} {
	blocks := []map[string]interface{}{}

	// Extract incident fields
	incidentID := getFieldSafe(notification.Fields, "incident_id")
	fixHash := getFieldSafe(notification.Fields, "fix_hash")
	hypothesis := getFieldSafe(notification.Fields, "hypothesis")
	confidence := getFieldSafe(notification.Fields, "confidence")
	recentChange := getFieldSafe(notification.Fields, "recent_change")
	signals := getFieldSafe(notification.Fields, "signals")
	proposedFix := getFieldSafe(notification.Fields, "proposed_fix")
	rollback := getFieldSafe(notification.Fields, "rollback")
	impact := getFieldSafe(notification.Fields, "impact")
	runbookURL := getFieldSafe(notification.Fields, "runbook_url")
	auditURL := getFieldSafe(notification.Fields, "audit_url")
	timeWindow := getFieldSafe(notification.Fields, "time_window")

	// 1. Header block with severity emoji and title
	blocks = append(blocks, slackBlockHeader(fmt.Sprintf("%s %s", notification.Level.Icon(), notification.Title)))

	// 2. Summary section - concise and personable
	summaryText := notification.Message
	if len(summaryText) > maxDetailsLength {
		summaryText = truncateText(summaryText, maxDetailsLength)
	}
	blocks = append(blocks, slackBlockSection(summaryText))

	// 3. Context block (severity, time window, incident ID)
	contextElements := []map[string]interface{}{
		slackBlockMrkdwn(fmt.Sprintf("*Severity:* %s `%s`", notification.Level.Icon(), notification.Level)),
	}
	if timeWindow != "" {
		contextElements = append(contextElements, slackBlockMrkdwn(fmt.Sprintf("*Window:* %s", timeWindow)))
	} else {
		contextElements = append(contextElements, slackBlockMrkdwn(fmt.Sprintf("*Time:* <!date^%d^{date_num} {time_secs}|%s>",
			notification.Timestamp.Unix(),
			notification.Timestamp.Format("2006-01-02 15:04:05"))))
	}
	if incidentID != "" {
		contextElements = append(contextElements, slackBlockMrkdwn(fmt.Sprintf("*Incident:* `%s`", truncateText(incidentID, 12))))
	}
	blocks = append(blocks, slackBlockContext(contextElements))

	// 4. Impact section (if available)
	if impact != "" {
		blocks = append(blocks, slackBlockDivider())
		blocks = append(blocks, slackBlockSection(fmt.Sprintf("*💥 Impact*\n%s", truncateText(impact, maxFieldValueLength*2))))
	}

	// 5. Hypothesis with confidence (if available)
	if hypothesis != "" {
		blocks = append(blocks, slackBlockDivider())
		hypothesisText := fmt.Sprintf("*🎯 Hypothesis*\n%s", truncateText(hypothesis, maxFieldValueLength*2))
		if confidence != "" {
			hypothesisText += fmt.Sprintf("\n_Confidence: %s_", confidence)
		}
		blocks = append(blocks, slackBlockSection(hypothesisText))
	}

	// 6. Recent change (if available) - important for incident correlation
	if recentChange != "" {
		blocks = append(blocks, slackBlockSection(fmt.Sprintf("*🔄 Recent Change*\n%s", truncateText(recentChange, maxFieldValueLength*2))))
	}

	// 7. Signals section (if available)
	if signals != "" {
		blocks = append(blocks, slackBlockSection(fmt.Sprintf("*📊 Signals*\n%s", truncateText(signals, maxFieldValueLength*2))))
	}

	// 8. Proposed fix with rollback (if available) - the actionable part
	if proposedFix != "" {
		blocks = append(blocks, slackBlockDivider())
		fixText := fmt.Sprintf("*🔧 Proposed Fix (Dry-Run)*\n```%s```", truncateText(proposedFix, 400))
		if rollback != "" {
			fixText += fmt.Sprintf("\n*↩️ Rollback:* `%s`", truncateText(rollback, 100))
		}
		blocks = append(blocks, slackBlockSection(fixText))
	}

	// 9. Action buttons section
	actionValue := fmt.Sprintf("%s|%s", incidentID, fixHash)

	actionElements := []map[string]interface{}{}

	// Approve fix button (primary)
	if proposedFix != "" {
		actionElements = append(actionElements, slackBlockButton(
			"✅ Approve Fix",
			"lumo_approve_fix",
			actionValue,
			"primary",
		))
	}

	// More context button (links to runbook or audit)
	if runbookURL != "" {
		actionElements = append(actionElements, slackBlockURLButton(
			"📖 Runbook",
			runbookURL,
			"",
		))
	} else if auditURL != "" {
		actionElements = append(actionElements, slackBlockURLButton(
			"📋 View Details",
			auditURL,
			"",
		))
	} else if s.apiBaseURL != "" && incidentID != "" {
		// Default to incident analysis page
		actionElements = append(actionElements, slackBlockURLButton(
			"📋 More Context",
			fmt.Sprintf("%s/api/v1/incidents/%s/analysis", s.apiBaseURL, incidentID),
			"",
		))
	}

	// Snooze overflow menu
	snoozeOptions := []map[string]interface{}{
		slackBlockOverflowOption("⏰ Snooze 15m", fmt.Sprintf("snooze_15|%s", actionValue)),
		slackBlockOverflowOption("⏰ Snooze 30m", fmt.Sprintf("snooze_30|%s", actionValue)),
		slackBlockOverflowOption("⏰ Snooze 60m", fmt.Sprintf("snooze_60|%s", actionValue)),
	}
	actionElements = append(actionElements, slackBlockOverflow("lumo_snooze", snoozeOptions))

	// Decline button
	actionElements = append(actionElements, slackBlockButton(
		"❌ Decline",
		"lumo_decline",
		actionValue,
		"danger",
	))

	if len(actionElements) > 0 {
		blocks = append(blocks, slackBlockActions(actionElements))
	}

	// 10. Add visible fields from notification.Fields (skip internal keys)
	visibleFields := []map[string]interface{}{}
	// Define the set of known incident fields to skip (they're already shown above)
	knownIncidentFields := map[string]bool{
		"incident_id": true, "fix_hash": true, "hypothesis": true, "confidence": true,
		"recent_change": true, "signals": true, "proposed_fix": true, "rollback": true,
		"impact": true, "runbook_url": true, "audit_url": true, "time_window": true,
		"event_id": true,
	}
	for key, value := range notification.Fields {
		// Skip internal fields (prefixed with _) and known incident fields
		if strings.HasPrefix(key, "_") || knownIncidentFields[key] {
			continue
		}
		visibleFields = append(visibleFields, slackBlockMrkdwn(fmt.Sprintf("*%s*\n%s", key, truncateText(value, maxFieldValueLength))))
	}
	if len(visibleFields) > 0 {
		blocks = append(blocks, slackBlockSectionWithFields(visibleFields))
	}

	// 11. Footer with branding
	blocks = append(blocks, slackBlockContext([]map[string]interface{}{
		slackBlockMrkdwn(":zap: *Lumo* | Intelligent SRE Automation"),
	}))

	return blocks
}
