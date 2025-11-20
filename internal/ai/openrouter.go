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
	// OpenRouterAPIURL is the default OpenRouter API endpoint
	OpenRouterAPIURL = "https://openrouter.ai/api/v1/chat/completions"

	// DefaultOpenRouterModel is the default model for OpenRouter
	// Using Claude Sonnet 4.5 as it's available on OpenRouter and provides good performance
	DefaultOpenRouterModel = "anthropic/claude-sonnet-4.5"

	// OpenRouterAppName is the application name for tracking
	OpenRouterAppName = "Lumo"

	// OpenRouterAppURL is the application URL for tracking
	OpenRouterAppURL = "https://github.com/ignacio/lumo"
)

// OpenRouterProvider implements the Provider interface for OpenRouter.
// OpenRouter provides unified access to multiple LLM providers through a single API.
type OpenRouterProvider struct {
	*BaseProvider
	config *ProviderConfig
	log    *logrus.Logger
}

// NewOpenRouterProvider creates a new OpenRouter provider.
func NewOpenRouterProvider(config *ProviderConfig, log *logrus.Logger) (*OpenRouterProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("openrouter API key is required")
	}

	if config.Model == "" {
		config.Model = DefaultOpenRouterModel
	}

	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}

	if config.MaxTokens == 0 {
		config.MaxTokens = 4096
	}

	if config.Temperature == 0 {
		// Default to 1.0 for consistency with other providers
		config.Temperature = 1.0
	}

	if config.Endpoint == "" {
		config.Endpoint = OpenRouterAPIURL
	}

	// Validate endpoint for security (allow localhost for testing)
	if err := ValidateEndpoint(config.Endpoint, false); err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	// Create adapter
	adapter := &openrouterAdapter{
		config: config,
	}

	// Create base provider
	base := NewBaseProvider(adapter, log)

	return &OpenRouterProvider{
		BaseProvider: base,
		config:       config,
		log:          log,
	}, nil
}

// Health checks if the OpenRouter API is accessible.
func (p *OpenRouterProvider) Health(ctx context.Context) error {
	// Simple health check: send minimal request
	adapter := p.adapter.(*openrouterAdapter)
	req := &openrouterRequest{
		Model:     adapter.config.Model,
		MaxTokens: 10,
		Messages: []openrouterMessage{
			{Role: "user", Content: "test"},
		},
	}

	// Execute request
	resp, err := p.httpClient.Do(ctx, RequestOptions{
		Method:       "POST",
		URL:          adapter.GetEndpoint(),
		Body:         req,
		Headers:      adapter.BuildHeaders(),
		ProviderName: p.Name(),
	})
	if err != nil {
		return &Error{
			Op:        "health_check",
			Provider:  p.Name(),
			Err:       err,
			Retryable: true,
		}
	}

	if len(resp.Body) == 0 {
		return &Error{
			Op:        "health_check",
			Provider:  p.Name(),
			Err:       fmt.Errorf("empty response body"),
			Retryable: true,
		}
	}

	return nil
}

// AnalyzeStream analyzes with streaming response.
func (p *OpenRouterProvider) AnalyzeStream(ctx context.Context, req *AnalysisRequest) (<-chan StreamChunk, error) {
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
	adapter := p.adapter.(*openrouterAdapter)
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

		// Use streaming parser (OpenAI-compatible)
		parser := &openrouterStreamParser{}
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

// openrouterAdapter implements ProviderAdapter for OpenRouter.
type openrouterAdapter struct {
	config *ProviderConfig
}

func (a *openrouterAdapter) Name() string {
	return "openrouter"
}

func (a *openrouterAdapter) BuildRequest(systemPrompt, userPrompt string, stream bool) (interface{}, error) {
	req := &openrouterRequest{
		Model:       a.config.Model,
		MaxTokens:   a.config.MaxTokens,
		Temperature: a.config.Temperature,
		Messages: []openrouterMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
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

func (a *openrouterAdapter) ParseResponse(body []byte) (string, *TokenUsage, error) {
	var resp openrouterResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal response: %w (body preview: %s)",
			err, truncateString(string(body), 200))
	}

	// Extract content from choices
	if len(resp.Choices) == 0 {
		return "", nil, fmt.Errorf("no choices in response")
	}

	content := resp.Choices[0].Message.Content
	refusal := resp.Choices[0].Message.Refusal

	// Check for refusal (content policy violations)
	if refusal != "" {
		return "", nil, fmt.Errorf("content policy refusal: %s", refusal)
	}

	// For reasoning models (like DeepSeek R1), content may be in the reasoning field
	if content == "" && resp.Choices[0].Message.Reasoning != "" {
		content = resp.Choices[0].Message.Reasoning
	}

	// Handle empty content
	if content == "" {
		return "", nil, fmt.Errorf("empty content in response")
	}

	// Build usage info
	usage := &TokenUsage{
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		TotalTokens:  resp.Usage.TotalTokens,
	}

	return content, usage, nil
}

func (a *openrouterAdapter) BuildHeaders() map[string]string {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + a.config.APIKey,
		// OpenRouter-specific headers for tracking and analytics
		"HTTP-Referer": OpenRouterAppURL,
		"X-Title":      OpenRouterAppName,
	}

	// Add custom headers
	for k, v := range a.config.CustomHeaders {
		headers[k] = v
	}

	return headers
}

func (a *openrouterAdapter) GetEndpoint() string {
	return a.config.Endpoint
}

func (a *openrouterAdapter) GetConfig() *ProviderConfig {
	return a.config
}

// openrouterStreamParser parses OpenRouter SSE streams (OpenAI-compatible).
type openrouterStreamParser struct{}

func (p *openrouterStreamParser) ParseLine(line string) (string, bool, error) {
	// Check for SSE "data: " prefix
	if !strings.HasPrefix(line, "data: ") {
		return "", false, nil
	}

	data := strings.TrimPrefix(line, "data: ")

	// Check for [DONE] marker
	if data == "[DONE]" {
		return "", true, nil
	}

	// Parse JSON event
	var event openrouterStreamEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return "", false, fmt.Errorf("failed to parse stream event: %w", err)
	}

	// Extract content delta
	if len(event.Choices) > 0 {
		delta := event.Choices[0].Delta.Content
		if delta != "" {
			return delta, false, nil
		}

		// Check for finish
		if event.Choices[0].FinishReason != "" {
			return "", true, nil
		}
	}

	return "", false, nil
}

// OpenRouter API request/response types
// These follow the OpenAI-compatible format

type openrouterRequest struct {
	Model       string              `json:"model"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	Messages    []openrouterMessage `json:"messages"`
	Stream      bool                `json:"stream,omitempty"`
}

type openrouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openrouterResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			Refusal   string `json:"refusal,omitempty"`   // OpenAI-compatible content policy field
			Reasoning string `json:"reasoning,omitempty"` // Reasoning models (DeepSeek R1, etc.)
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openrouterStreamEvent struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
			Refusal string `json:"refusal,omitempty"` // OpenAI-compatible content policy field
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}
