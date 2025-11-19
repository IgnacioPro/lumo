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

// WebhookProvider implements the Provider interface for generic webhook notifications
type WebhookProvider struct {
	url     string
	method  string
	headers map[string]string
	logger  *logrus.Logger
}

// WebhookConfig contains configuration for the Webhook provider
type WebhookConfig struct {
	Enabled bool              `mapstructure:"enabled"`
	URL     string            `mapstructure:"url"`
	Method  string            `mapstructure:"method"`
	Headers map[string]string `mapstructure:"headers"`
}

// webhookPayload represents the JSON payload sent to the webhook
type webhookPayload struct {
	Severity  string                  `json:"severity"`
	Title     string                  `json:"title"`
	Body      string                  `json:"body"`
	Fields    []notifications.Field   `json:"fields,omitempty"`
	Timestamp int64                   `json:"timestamp"`
	Source    string                  `json:"source"`
	HostInfo  *notifications.HostInfo `json:"host_info,omitempty"`
	Tags      []string                `json:"tags,omitempty"`
	Metadata  map[string]interface{}  `json:"metadata,omitempty"`
}

// NewWebhookProvider creates a new generic webhook notification provider
func NewWebhookProvider(config *WebhookConfig, logger *logrus.Logger) (*WebhookProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.URL == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}

	if logger == nil {
		logger = logrus.New()
	}

	method := config.Method
	if method == "" {
		method = "POST"
	}

	headers := config.Headers
	if headers == nil {
		headers = make(map[string]string)
	}

	return &WebhookProvider{
		url:     config.URL,
		method:  method,
		headers: headers,
		logger:  logger,
	}, nil
}

// Name returns the provider name
func (w *WebhookProvider) Name() string {
	return "webhook"
}

// Send sends a notification to the webhook endpoint
func (w *WebhookProvider) Send(ctx context.Context, msg *notifications.Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	payload := &webhookPayload{
		Severity:  string(msg.Severity),
		Title:     msg.Title,
		Body:      msg.Body,
		Fields:    msg.Fields,
		Timestamp: msg.Timestamp.Unix(),
		Source:    msg.Source,
		HostInfo:  msg.HostInfo,
		Tags:      msg.Tags,
		Metadata:  msg.Metadata,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, w.method, w.url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add custom headers
	for key, value := range w.headers {
		req.Header.Set(key, value)
	}

	// Set content type if not already set
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned error: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Health checks if the webhook provider is properly configured
func (w *WebhookProvider) Health(ctx context.Context) error {
	if w.url == "" {
		return fmt.Errorf("webhook URL is not configured")
	}

	// Verify URL format (basic validation)
	if len(w.url) < 8 {
		return fmt.Errorf("invalid webhook URL")
	}

	return nil
}
