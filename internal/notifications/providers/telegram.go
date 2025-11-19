package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/notifications"
	"github.com/sirupsen/logrus"
)

// TelegramProvider implements the Provider interface for Telegram Bot API notifications
type TelegramProvider struct {
	botToken  string
	chatID    string
	parseMode string // "Markdown" or "HTML"
	logger    *logrus.Logger
}

// TelegramConfig contains configuration for the Telegram provider
type TelegramConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	BotToken  string `mapstructure:"bot_token"`
	ChatID    string `mapstructure:"chat_id"`
	ParseMode string `mapstructure:"parse_mode"`
}

// telegramRequest represents the API request payload
type telegramRequest struct {
	ChatID              string `json:"chat_id"`
	Text                string `json:"text"`
	ParseMode           string `json:"parse_mode,omitempty"`
	DisableNotification bool   `json:"disable_notification,omitempty"`
}

// telegramResponse represents the API response
type telegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

// NewTelegramProvider creates a new Telegram notification provider
func NewTelegramProvider(config *TelegramConfig, logger *logrus.Logger) (*TelegramProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.BotToken == "" {
		return nil, fmt.Errorf("bot token is required")
	}

	if config.ChatID == "" {
		return nil, fmt.Errorf("chat ID is required")
	}

	if logger == nil {
		logger = logrus.New()
	}

	parseMode := config.ParseMode
	if parseMode == "" {
		parseMode = "Markdown"
	}

	return &TelegramProvider{
		botToken:  config.BotToken,
		chatID:    config.ChatID,
		parseMode: parseMode,
		logger:    logger,
	}, nil
}

// Name returns the provider name
func (t *TelegramProvider) Name() string {
	return "telegram"
}

// Send sends a notification via Telegram Bot API
func (t *TelegramProvider) Send(ctx context.Context, msg *notifications.Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

	// Format message
	text := t.formatMessage(msg)

	// Build request
	request := telegramRequest{
		ChatID:    t.chatID,
		Text:      text,
		ParseMode: t.parseMode,
	}

	// Silent notifications for low-priority messages
	if msg.Severity == diagnostics.SeverityInfo || msg.Severity == diagnostics.SeverityOK {
		request.DisableNotification = true
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var telegramResp telegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !telegramResp.OK {
		return fmt.Errorf("Telegram API error: %s", telegramResp.Description)
	}

	return nil
}

// Health checks if the Telegram provider is properly configured
func (t *TelegramProvider) Health(ctx context.Context) error {
	if t.botToken == "" {
		return fmt.Errorf("bot token is not configured")
	}

	if t.chatID == "" {
		return fmt.Errorf("chat ID is not configured")
	}

	// Test the bot token by calling getMe
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", t.botToken)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	var telegramResp telegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
		return fmt.Errorf("failed to decode health check response: %w", err)
	}

	if !telegramResp.OK {
		return fmt.Errorf("invalid bot token: %s", telegramResp.Description)
	}

	return nil
}

// formatMessage formats a notification message for Telegram with Markdown
func (t *TelegramProvider) formatMessage(msg *notifications.Message) string {
	var buf bytes.Buffer

	// Title with emoji
	buf.WriteString(msg.SeverityEmoji())
	buf.WriteString(" *")
	buf.WriteString(escapeMarkdown(msg.Title))
	buf.WriteString("*\n\n")

	// Body
	buf.WriteString(escapeMarkdown(msg.Body))
	buf.WriteString("\n\n")

	// Severity
	buf.WriteString("*Severity:* ")
	buf.WriteString("`")
	buf.WriteString(string(msg.Severity))
	buf.WriteString("`\n")

	// Host information
	if msg.HostInfo != nil {
		buf.WriteString("\n*Host Information:*\n")

		if msg.HostInfo.Hostname != "" {
			buf.WriteString("• *Hostname:* `")
			buf.WriteString(msg.HostInfo.Hostname)
			buf.WriteString("`\n")
		}

		if msg.HostInfo.IP != "" {
			buf.WriteString("• *IP:* `")
			buf.WriteString(msg.HostInfo.IP)
			buf.WriteString("`\n")
		}

		if msg.HostInfo.Platform != "" {
			buf.WriteString("• *Platform:* `")
			buf.WriteString(msg.HostInfo.Platform)
			buf.WriteString("`\n")
		}

		if msg.HostInfo.OS != "" {
			buf.WriteString("• *OS:* `")
			buf.WriteString(msg.HostInfo.OS)
			buf.WriteString("`\n")
		}
	}

	// Additional fields
	if len(msg.Fields) > 0 {
		buf.WriteString("\n*Details:*\n")
		for _, field := range msg.Fields {
			buf.WriteString("• *")
			buf.WriteString(escapeMarkdown(field.Name))
			buf.WriteString(":* `")
			buf.WriteString(escapeMarkdown(field.Value))
			buf.WriteString("`\n")
		}
	}

	// Footer
	buf.WriteString("\n_🤖 Automated alert from Lumo | Source: ")
	buf.WriteString(msg.Source)
	buf.WriteString("_\n")
	buf.WriteString("_")
	buf.WriteString(msg.Timestamp.Format("2006-01-02 15:04:05 MST"))
	buf.WriteString("_")

	return buf.String()
}

// escapeMarkdown escapes special Markdown characters
func escapeMarkdown(text string) string {
	// For Telegram Markdown, we need to escape: _ * [ ] ( ) ~ ` > # + - = | { } . !
	replacer := []struct {
		old string
		new string
	}{
		{"_", "\\_"},
		{"*", "\\*"},
		{"[", "\\["},
		{"]", "\\]"},
		{"(", "\\("},
		{")", "\\)"},
		{"~", "\\~"},
		{"`", "\\`"},
		{">", "\\>"},
		{"#", "\\#"},
		{"+", "\\+"},
		{"-", "\\-"},
		{"=", "\\="},
		{"|", "\\|"},
		{"{", "\\{"},
		{"}", "\\}"},
		{".", "\\."},
		{"!", "\\!"},
	}

	result := []byte(text)
	for _, r := range replacer {
		result = bytes.ReplaceAll(result, []byte(r.old), []byte(r.new))
	}

	return string(result)
}
