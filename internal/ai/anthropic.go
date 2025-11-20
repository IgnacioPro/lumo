package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// AnthropicAPIURL is the default Anthropic API endpoint
	AnthropicAPIURL = "https://api.anthropic.com/v1/messages"

	// AnthropicAPIVersion is the required API version header
	AnthropicAPIVersion = "2023-06-01"

	// DefaultAnthropicModel is the default Claude model
	DefaultAnthropicModel = "claude-sonnet-4-5-20250929"
)

// AnthropicProvider implements the Provider interface for Anthropic Claude.
type AnthropicProvider struct {
	*BaseProvider
	config *ProviderConfig
	log    *logrus.Logger
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(config *ProviderConfig, log *logrus.Logger) (*AnthropicProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("anthropic API key is required")
	}

	if config.Model == "" {
		config.Model = DefaultAnthropicModel
	}

	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}

	if config.MaxTokens == 0 {
		config.MaxTokens = 4096
	}

	if config.Temperature == 0 {
		config.Temperature = 1.0
	}

	if config.Endpoint == "" {
		config.Endpoint = AnthropicAPIURL
	}

	// Validate endpoint for security (allow localhost for testing)
	if err := ValidateEndpoint(config.Endpoint, false); err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	// Create adapter
	adapter := &anthropicAdapter{
		config: config,
	}

	// Create base provider
	base := NewBaseProvider(adapter, log)

	return &AnthropicProvider{
		BaseProvider: base,
		config:       config,
		log:          log,
	}, nil
}

// AnalyzeStream analyzes with streaming response.
func (p *AnthropicProvider) AnalyzeStream(ctx context.Context, req *AnalysisRequest) (<-chan StreamChunk, error) {
	// Build prompts
	pb := NewPromptBuilder()
	if len(req.Focus) > 0 {
		pb.WithFocus(req.Focus...)
	}

	systemPrompt := pb.BuildSystemPrompt()
	userPrompt, err := pb.BuildAnalysisPrompt(req)
	if err != nil {
		return nil, &Error{
			Op:       "build_prompt",
			Provider: p.Name(),
			Err:      err,
		}
	}

	// Build streaming request
	adapter := p.adapter.(*anthropicAdapter)
	apiReq, err := adapter.BuildRequest(systemPrompt, userPrompt, true)
	if err != nil {
		return nil, &Error{
			Op:       "build_request",
			Provider: p.Name(),
			Err:      err,
		}
	}

	ch := make(chan StreamChunk, 10)

	go func() {
		defer close(ch)

		// Execute streaming HTTP request
		resp, err := p.httpClient.DoStreaming(ctx, RequestOptions{
			Method:       "POST",
			URL:          adapter.GetEndpoint(),
			Body:         apiReq,
			Headers:      adapter.BuildHeaders(),
			ProviderName: p.Name(),
		})
		if err != nil {
			ch <- StreamChunk{
				Type:  ChunkError,
				Error: err,
				Done:  true,
			}
			return
		}

		// Use streaming parser
		parser := &anthropicStreamParser{}
		if err := StreamToChannel(resp.Body, parser, ch, p.log); err != nil {
			ch <- StreamChunk{
				Type:  ChunkError,
				Error: err,
				Done:  true,
			}
		}
	}()

	return ch, nil
}

// anthropicAdapter implements ProviderAdapter for Anthropic.
type anthropicAdapter struct {
	config *ProviderConfig
}

func (a *anthropicAdapter) Name() string {
	return "anthropic"
}

func (a *anthropicAdapter) BuildRequest(systemPrompt, userPrompt string, stream bool) (interface{}, error) {
	req := &anthropicRequest{
		Model:       a.config.Model,
		MaxTokens:   a.config.MaxTokens,
		Temperature: a.config.Temperature,
		System:      systemPrompt,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
	}

	if stream {
		req.Stream = true
	}

	return req, nil
}

func (a *anthropicAdapter) ParseResponse(body []byte) (string, *TokenUsage, error) {
	var resp anthropicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal response: %w (body preview: %s)",
			err, truncateString(string(body), 200))
	}

	// Extract content from content blocks
	var content strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	// Build usage info
	usage := &TokenUsage{
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
	}

	return content.String(), usage, nil
}

func (a *anthropicAdapter) BuildHeaders() map[string]string {
	headers := map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         a.config.APIKey,
		"anthropic-version": AnthropicAPIVersion,
	}

	// Add custom headers
	for k, v := range a.config.CustomHeaders {
		headers[k] = v
	}

	return headers
}

func (a *anthropicAdapter) GetEndpoint() string {
	return a.config.Endpoint
}

func (a *anthropicAdapter) GetConfig() *ProviderConfig {
	return a.config
}

// anthropicStreamParser parses Anthropic SSE streams.
type anthropicStreamParser struct{}

func (p *anthropicStreamParser) ParseLine(line string) (string, bool, error) {
	// Check for SSE "data: " prefix
	if !strings.HasPrefix(line, "data: ") {
		return "", false, nil
	}

	data := strings.TrimPrefix(line, "data: ")

	// Parse JSON event
	var event anthropicStreamEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return "", false, fmt.Errorf("failed to parse stream event: %w", err)
	}

	// Handle different event types
	switch event.Type {
	case "content_block_delta":
		if event.Delta.Type == "text_delta" {
			return event.Delta.Text, false, nil
		}

	case "message_stop":
		return "", true, nil
	}

	return "", false, nil
}

// Anthropic API request/response types

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
}
