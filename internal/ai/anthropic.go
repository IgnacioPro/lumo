package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	config *ProviderConfig
	client *http.Client
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

	return &AnthropicProvider{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		log: log,
	}, nil
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Analyze analyzes diagnostic results using Claude.
func (p *AnthropicProvider) Analyze(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error) {
	start := time.Now()

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

	p.log.WithFields(logrus.Fields{
		"model":        p.config.Model,
		"system_chars": len(systemPrompt),
		"user_chars":   len(userPrompt),
	}).Debug("Built analysis prompts")

	// Make API request
	apiReq := p.buildRequest(systemPrompt, userPrompt)
	respContent, usage, err := p.callAPI(ctx, apiReq)
	if err != nil {
		return nil, err
	}

	// Parse response
	response, err := ParseAnalysisResponse(respContent, p.Name(), p.config.Model)
	if err != nil {
		return nil, &Error{
			Op:       "parse_response",
			Provider: p.Name(),
			Err:      err,
		}
	}

	// Set metadata
	response.Timestamp = time.Now()
	response.Duration = time.Since(start)
	response.TokensUsed = usage

	p.log.WithFields(logrus.Fields{
		"findings":        len(response.Findings),
		"recommendations": len(response.Recommendations),
		"health":          response.OverallHealth,
		"duration":        response.Duration,
		"tokens":          usage.TotalTokens,
	}).Info("Analysis complete")

	return response, nil
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

	// Create streaming request
	apiReq := p.buildRequest(systemPrompt, userPrompt)
	apiReq.Stream = true

	ch := make(chan StreamChunk, 10)

	go func() {
		defer close(ch)

		if err := p.streamAPI(ctx, apiReq, ch); err != nil {
			ch <- StreamChunk{
				Type:  ChunkError,
				Error: err,
				Done:  true,
			}
		}
	}()

	return ch, nil
}

// Health checks if the Anthropic API is accessible.
func (p *AnthropicProvider) Health(ctx context.Context) error {
	// Simple health check: send a minimal request
	req := &anthropicRequest{
		Model:     p.config.Model,
		MaxTokens: 10,
		Messages: []anthropicMessage{
			{Role: "user", Content: "test"},
		},
	}

	_, _, err := p.callAPI(ctx, req)
	if err != nil {
		return &Error{
			Op:        "health_check",
			Provider:  p.Name(),
			Err:       err,
			Retryable: true,
		}
	}

	return nil
}

// buildRequest constructs an Anthropic API request.
func (p *AnthropicProvider) buildRequest(systemPrompt, userPrompt string) *anthropicRequest {
	return &anthropicRequest{
		Model:       p.config.Model,
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
		System:      systemPrompt,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
	}
}

// callAPI makes a non-streaming API call.
func (p *AnthropicProvider) callAPI(ctx context.Context, req *anthropicRequest) (string, *TokenUsage, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", nil, &Error{
			Op:        "api_call",
			Provider:  p.Name(),
			Err:       err,
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, &Error{
			Op:        "api_call",
			Provider:  p.Name(),
			Err:       fmt.Errorf("API error %d: %s", resp.StatusCode, string(body)),
			Retryable: resp.StatusCode >= 500,
		}
	}

	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract content
	var content strings.Builder
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	// Build usage info
	usage := &TokenUsage{
		InputTokens:  apiResp.Usage.InputTokens,
		OutputTokens: apiResp.Usage.OutputTokens,
		TotalTokens:  apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
	}

	return content.String(), usage, nil
}

// streamAPI makes a streaming API call.
func (p *AnthropicProvider) streamAPI(ctx context.Context, req *anthropicRequest, ch chan<- StreamChunk) error {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return &Error{
			Op:        "stream_call",
			Provider:  p.Name(),
			Err:       err,
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &Error{
			Op:        "stream_call",
			Provider:  p.Name(),
			Err:       fmt.Errorf("API error %d: %s", resp.StatusCode, string(body)),
			Retryable: resp.StatusCode >= 500,
		}
	}

	// Read streaming response
	scanner := bufio.NewScanner(resp.Body)
	var fullContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse SSE format: "data: {json}"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// End of stream marker
		if data == "[DONE]" {
			break
		}

		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			p.log.WithError(err).Warn("Failed to parse stream event")
			continue
		}

		// Handle different event types
		switch event.Type {
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				fullContent.WriteString(event.Delta.Text)
				ch <- StreamChunk{
					Type:    ChunkSummary,
					Content: event.Delta.Text,
				}
			}

		case "message_stop":
			// Final chunk with complete content
			ch <- StreamChunk{
				Type:    ChunkSummary,
				Content: fullContent.String(),
				Done:    true,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read error: %w", err)
	}

	return nil
}

// setHeaders sets required Anthropic API headers.
func (p *AnthropicProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.config.APIKey)
	req.Header.Set("anthropic-version", AnthropicAPIVersion)

	// Add custom headers
	for k, v := range p.config.CustomHeaders {
		req.Header.Set(k, v)
	}
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
