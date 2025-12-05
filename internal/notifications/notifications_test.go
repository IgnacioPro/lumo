package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestNewNotification tests notification creation.
func TestNewNotification(t *testing.T) {
	n := NewNotification("Test Title", "Test Message", LevelInfo)

	if n.Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got '%s'", n.Title)
	}
	if n.Message != "Test Message" {
		t.Errorf("expected message 'Test Message', got '%s'", n.Message)
	}
	if n.Level != LevelInfo {
		t.Errorf("expected level '%s', got '%s'", LevelInfo, n.Level)
	}
	if n.Fields == nil {
		t.Error("expected Fields to be initialized")
	}
	if n.Tags == nil {
		t.Error("expected Tags to be initialized")
	}
}

// TestNotificationWithField tests adding fields to notifications.
func TestNotificationWithField(t *testing.T) {
	n := NewNotification("Test", "Message", LevelWarning).
		WithField("key1", "value1").
		WithField("key2", "value2")

	if len(n.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(n.Fields))
	}
	if n.Fields["key1"] != "value1" {
		t.Errorf("expected key1='value1', got '%s'", n.Fields["key1"])
	}
}

// TestNotificationWithTag tests adding tags to notifications.
func TestNotificationWithTag(t *testing.T) {
	n := NewNotification("Test", "Message", LevelError).
		WithTag("tag1").
		WithTag("tag2")

	if len(n.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(n.Tags))
	}
}

// TestNotificationLevelIcon tests level icons.
func TestNotificationLevelIcon(t *testing.T) {
	tests := []struct {
		level        NotificationLevel
		expectedIcon string
	}{
		{LevelInfo, "ℹ️"},
		{LevelWarning, "⚠️"},
		{LevelError, "❌"},
		{LevelCritical, "🚨"},
		{LevelSuccess, "✅"},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			if icon := tt.level.Icon(); icon != tt.expectedIcon {
				t.Errorf("expected icon '%s', got '%s'", tt.expectedIcon, icon)
			}
		})
	}
}

// TestNotificationLevelColor tests level colors.
func TestNotificationLevelColor(t *testing.T) {
	tests := []struct {
		level         NotificationLevel
		expectedColor string
	}{
		{LevelInfo, "#0066CC"},
		{LevelWarning, "#FFA500"},
		{LevelError, "#DC143C"},
		{LevelCritical, "#8B0000"},
		{LevelSuccess, "#32CD32"},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			if color := tt.level.Color(); color != tt.expectedColor {
				t.Errorf("expected color '%s', got '%s'", tt.expectedColor, color)
			}
		})
	}
}

// TestParseNotifierType tests notifier type parsing.
func TestParseNotifierType(t *testing.T) {
	tests := []struct {
		input    string
		expected NotifierType
		hasError bool
	}{
		{"slack", NotifierSlack, false},
		{"telegram", NotifierTelegram, false},
		{"webhook", NotifierWebhook, false},
		{"email", NotifierEmail, false},
		{"SLACK", NotifierSlack, false}, // Case insensitive
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseNotifierType(tt.input)
			if tt.hasError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

// TestValidateNotifierType tests notifier type validation.
func TestValidateNotifierType(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"slack", true},
		{"telegram", true},
		{"webhook", true},
		{"email", true},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ValidateNotifierType(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestSupportedNotifiers tests supported notifiers list.
func TestSupportedNotifiers(t *testing.T) {
	notifiers := SupportedNotifiers()
	if len(notifiers) != 4 {
		t.Errorf("expected 4 supported notifiers, got %d", len(notifiers))
	}

	// Check that all expected notifiers are present
	expected := map[NotifierType]bool{
		NotifierSlack:    false,
		NotifierTelegram: false,
		NotifierWebhook:  false,
		NotifierEmail:    false,
	}

	for _, n := range notifiers {
		if _, ok := expected[n]; !ok {
			t.Errorf("unexpected notifier type: %s", n)
		}
		expected[n] = true
	}

	for k, v := range expected {
		if !v {
			t.Errorf("missing notifier type: %s", k)
		}
	}
}

// TestSlackNotifier tests Slack notification sending.
func TestSlackNotifier(t *testing.T) {
	// Create test server
	var receivedPayload slackMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("failed to unmarshal payload: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create notifier
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "test-slack",
		Type:       NotifierSlack,
		Enabled:    true,
		WebhookURL: server.URL,
		Timeout:    5,
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	// Send notification
	notification := NewNotification("Test Alert", "This is a test message", LevelWarning).
		WithField("Host", "test-server").
		WithField("Status", "degraded")

	ctx := context.Background()
	if _, err := notifier.Send(ctx, notification); err != nil {
		t.Fatalf("failed to send notification: %v", err)
	}

	// Verify payload
	if !strings.Contains(receivedPayload.Text, "Test Alert") {
		t.Error("payload should contain notification title")
	}
	if len(receivedPayload.Attachments) == 0 {
		t.Error("expected attachments in payload")
	}

	// Check that blocks are present (new Block Kit format)
	if len(receivedPayload.Attachments[0].Blocks) == 0 {
		t.Error("expected blocks in attachment (Block Kit format)")
	}

	// Verify blocks contain expected structure (header, context, sections, footer)
	blocks := receivedPayload.Attachments[0].Blocks
	hasHeader := false
	hasFields := false
	for _, block := range blocks {
		if blockType, ok := block["type"].(string); ok {
			if blockType == "header" {
				hasHeader = true
			}
			// Check for fields section
			if blockType == "section" {
				if fields, ok := block["fields"].([]interface{}); ok && len(fields) > 0 {
					hasFields = true
				}
			}
		}
	}

	if !hasHeader {
		t.Error("expected header block in Slack message")
	}
	if !hasFields {
		t.Error("expected fields section in Slack message")
	}
}

// TestTelegramNotifier tests Telegram notification sending.
func TestTelegramNotifier(t *testing.T) {
	// Create test server
	var receivedPayload telegramMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/sendMessage") {
			t.Errorf("expected /sendMessage endpoint, got %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("failed to unmarshal payload: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	// Create notifier (override API URL for testing)
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:             "test-telegram",
		Type:             NotifierTelegram,
		Enabled:          true,
		TelegramBotToken: "test-token",
		TelegramChatID:   "12345",
		Timeout:          5,
	}

	notifier, err := NewTelegramNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	// We can't easily test the actual API call without modifying the code,
	// so just verify the notifier was created correctly
	if notifier.Name() != "test-telegram" {
		t.Errorf("expected name 'test-telegram', got '%s'", notifier.Name())
	}

	// Test health check
	if err := notifier.Health(context.Background()); err != nil {
		t.Errorf("health check failed: %v", err)
	}
}

// TestWebhookNotifier tests generic webhook notification sending.
func TestWebhookNotifier(t *testing.T) {
	// Create test server
	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("failed to unmarshal payload: %v", err)
		}

		// Check custom headers
		if r.Header.Get("X-Custom-Header") != "test-value" {
			t.Errorf("expected custom header, got %s", r.Header.Get("X-Custom-Header"))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create notifier
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "test-webhook",
		Type:       NotifierWebhook,
		Enabled:    true,
		WebhookURL: server.URL,
		Method:     "POST",
		Headers: map[string]string{
			"X-Custom-Header": "test-value",
		},
		Timeout: 5,
	}

	notifier, err := NewWebhookNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	// Send notification
	notification := NewNotification("Test Alert", "This is a test message", LevelError)

	ctx := context.Background()
	if _, err := notifier.Send(ctx, notification); err != nil {
		t.Fatalf("failed to send notification: %v", err)
	}

	// Verify payload
	if receivedPayload["title"] != "Test Alert" {
		t.Errorf("expected title 'Test Alert', got '%v'", receivedPayload["title"])
	}
}

// TestEmailNotifierValidation tests email notifier validation.
func TestEmailNotifierValidation(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	tests := []struct {
		name      string
		config    *NotifierConfig
		shouldErr bool
	}{
		{
			name: "valid config",
			config: &NotifierConfig{
				Name:     "test-email",
				Type:     NotifierEmail,
				Enabled:  true,
				SMTPHost: "smtp.example.com",
				SMTPPort: 587,
				From:     "sender@example.com",
				To:       []string{"recipient@example.com"},
			},
			shouldErr: false,
		},
		{
			name: "missing SMTP host",
			config: &NotifierConfig{
				Name:    "test-email",
				Type:    NotifierEmail,
				Enabled: true,
				From:    "sender@example.com",
				To:      []string{"recipient@example.com"},
			},
			shouldErr: true,
		},
		{
			name: "missing from address",
			config: &NotifierConfig{
				Name:     "test-email",
				Type:     NotifierEmail,
				Enabled:  true,
				SMTPHost: "smtp.example.com",
				To:       []string{"recipient@example.com"},
			},
			shouldErr: true,
		},
		{
			name: "missing recipients",
			config: &NotifierConfig{
				Name:     "test-email",
				Type:     NotifierEmail,
				Enabled:  true,
				SMTPHost: "smtp.example.com",
				From:     "sender@example.com",
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEmailNotifier(tt.config, log)
			if tt.shouldErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestNewNotifierFactory tests the notifier factory function.
func TestNewNotifierFactory(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	tests := []struct {
		name      string
		config    *NotifierConfig
		shouldErr bool
	}{
		{
			name: "slack notifier",
			config: &NotifierConfig{
				Name:       "test-slack",
				Type:       NotifierSlack,
				Enabled:    true,
				WebhookURL: "https://hooks.slack.com/test",
			},
			shouldErr: false,
		},
		{
			name: "telegram notifier",
			config: &NotifierConfig{
				Name:             "test-telegram",
				Type:             NotifierTelegram,
				Enabled:          true,
				TelegramBotToken: "test-token",
				TelegramChatID:   "12345",
			},
			shouldErr: false,
		},
		{
			name: "webhook notifier",
			config: &NotifierConfig{
				Name:       "test-webhook",
				Type:       NotifierWebhook,
				Enabled:    true,
				WebhookURL: "https://example.com/webhook",
			},
			shouldErr: false,
		},
		{
			name: "email notifier",
			config: &NotifierConfig{
				Name:     "test-email",
				Type:     NotifierEmail,
				Enabled:  true,
				SMTPHost: "smtp.example.com",
				From:     "sender@example.com",
				To:       []string{"recipient@example.com"},
			},
			shouldErr: false,
		},
		{
			name: "disabled notifier",
			config: &NotifierConfig{
				Name:    "disabled",
				Type:    NotifierSlack,
				Enabled: false,
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewNotifier(tt.config, log)
			if tt.shouldErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if notifier == nil {
					t.Error("expected notifier, got nil")
				}
			}
		})
	}
}

// TestHTMLEscape tests HTML escaping function.
func TestHTMLEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "Hello World"},
		{"<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"A & B", "A &amp; B"},
		{`"quoted"`, "&quot;quoted&quot;"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := htmlEscape(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestEscapeMarkdown tests Telegram markdown escaping.
func TestEscapeMarkdown(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "Hello World"},
		{"*bold*", "\\*bold\\*"},
		{"_italic_", "\\_italic\\_"},
		{"[link](url)", "\\[link\\]\\(url\\)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeMarkdown(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestHexToDecimal tests hex color conversion.
func TestHexToDecimal(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"#FF0000", 16711680}, // Red
		{"#00FF00", 65280},    // Green
		{"#0000FF", 255},      // Blue
		{"#FFFFFF", 16777215}, // White
		{"#000000", 0},        // Black
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := hexToDecimal(tt.input)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestSlackNotifierTimeout tests timeout handling.
func TestSlackNotifierTimeout(t *testing.T) {
	// Create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "test-slack",
		Type:       NotifierSlack,
		Enabled:    true,
		WebhookURL: server.URL,
		Timeout:    1, // 1 second timeout
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	notification := NewNotification("Test", "Message", LevelInfo)

	ctx := context.Background()
	_, err = notifier.Send(ctx, notification)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// BenchmarkSlackNotifier benchmarks Slack notification sending.
func BenchmarkSlackNotifier(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "bench-slack",
		Type:       NotifierSlack,
		Enabled:    true,
		WebhookURL: server.URL,
	}

	notifier, _ := NewSlackNotifier(config, log)
	notification := NewNotification("Benchmark Test", "Performance testing", LevelInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = notifier.Send(context.Background(), notification)
	}
}
