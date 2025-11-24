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

// TelegramNotifier sends notifications via Telegram Bot API.
type TelegramNotifier struct {
	config         *NotifierConfig
	log            *logrus.Logger
	client         *http.Client
	circuitBreaker *reliability.CircuitBreaker
}

// telegramMessage represents a Telegram message payload.
type telegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// NewTelegramNotifier creates a new Telegram notifier.
func NewTelegramNotifier(config *NotifierConfig, log *logrus.Logger) (*TelegramNotifier, error) {
	if config.BotToken == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}
	if config.ChatID == "" {
		return nil, fmt.Errorf("telegram chat ID is required")
	}

	timeout := 30 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	return &TelegramNotifier{
		config: config,
		log:    log,
		client: &http.Client{
			Timeout: timeout,
		},
		circuitBreaker: reliability.NewCircuitBreaker(fmt.Sprintf("telegram-%s", config.Name)),
	}, nil
}

// Name returns the notifier name.
func (t *TelegramNotifier) Name() string {
	return t.config.Name
}

// Send sends a notification to Telegram.
func (t *TelegramNotifier) Send(ctx context.Context, notification *Notification) error {
	// Wrap execution in circuit breaker
	_, err := t.circuitBreaker.Execute(func() (interface{}, error) {
		return nil, t.sendInternal(ctx, notification)
	})
	return err
}

func (t *TelegramNotifier) sendInternal(ctx context.Context, notification *Notification) error {
	// Build Telegram message
	msg := t.buildMessage(notification)

	// Marshal to JSON
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram message: %w", err)
	}

	// Build API URL
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.config.BotToken)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram notification: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram returned non-OK status: %d", resp.StatusCode)
	}

	t.log.WithFields(logrus.Fields{
		"notifier": t.Name(),
		"level":    notification.Level,
		"title":    notification.Title,
	}).Debug("notification sent successfully")

	return nil
}

// Health checks if the Telegram bot is configured.
func (t *TelegramNotifier) Health(ctx context.Context) error {
	if t.config.BotToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}
	if t.config.ChatID == "" {
		return fmt.Errorf("telegram chat ID not configured")
	}
	return nil
}

// buildMessage builds a Telegram message from a notification.
func (t *TelegramNotifier) buildMessage(notification *Notification) *telegramMessage {
	var sb strings.Builder

	// Add level icon and title
	sb.WriteString(notification.Level.Icon())
	sb.WriteString(" *")
	sb.WriteString(escapeMarkdown(notification.Title))
	sb.WriteString("*\n\n")

	// Add message
	sb.WriteString(escapeMarkdown(notification.Message))

	// Add fields if present
	if len(notification.Fields) > 0 {
		sb.WriteString("\n\n")
		for key, value := range notification.Fields {
			sb.WriteString("*")
			sb.WriteString(escapeMarkdown(key))
			sb.WriteString(":* ")
			sb.WriteString(escapeMarkdown(value))
			sb.WriteString("\n")
		}
	}

	return &telegramMessage{
		ChatID:    t.config.ChatID,
		Text:      sb.String(),
		ParseMode: "Markdown",
	}
}

// escapeMarkdown escapes special characters for Telegram Markdown.
func escapeMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(text)
}
