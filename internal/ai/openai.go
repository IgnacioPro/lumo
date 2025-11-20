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
	// OpenAIAPIURL is the default OpenAI API endpoint
	OpenAIAPIURL = "https://api.openai.com/v1/chat/completions"

	// DefaultOpenAIModel is the default GPT model
	DefaultOpenAIModel = "gpt-4-turbo-preview"
)

// OpenAIProvider implements the Provider interface for OpenAI GPT.
type OpenAIProvider struct {
	*BaseProvider
	config *ProviderConfig
	log    *logrus.Logger
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(config *ProviderConfig, log *logrus.Logger) (*OpenAIProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("openai API key is required")
	}

	if config.Model == "" {
		config.Model = DefaultOpenAIModel
	}

	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}

	if config.MaxTokens == 0 {
		config.MaxTokens = 4096
	}

	if config.Temperature == 0 {
		// Default to 1.0 for consistency with Anthropic and more creative responses
		config.Temperature = 1.0
	}

	if config.Endpoint == "" {
		config.Endpoint = OpenAIAPIURL
	}

	// Validate endpoint for security (allow localhost for testing)
	if err := ValidateEndpoint(config.Endpoint, false); err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	// Create adapter
	adapter := &openaiAdapter{
		config: config,
		log:    log,
	}

	// Create base provider
	base := NewBaseProvider(adapter, log)

	return &OpenAIProvider{
		BaseProvider: base,
		config:       config,
		log:          log,
	}, nil
}

// Health checks if the OpenAI API is accessible.
func (p *OpenAIProvider) Health(ctx context.Context) error {
	// Override to use higher token limit for reasoning models
	// Note: Use 1000 tokens to accommodate reasoning models (e.g., o1, o3, gpt-5-nano)
	// which use significant tokens for internal reasoning before generating content.
	adapter := p.adapter.(*openaiAdapter)
	req, err := adapter.buildHealthRequest()
	if err != nil {
		return &Error{
			Op:        "health_check",
			Provider:  p.Name(),
			Err:       fmt.Errorf("failed to build health check request: %w", err),
			Retryable: false,
		}
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
func (p *OpenAIProvider) AnalyzeStream(ctx context.Context, req *AnalysisRequest) (<-chan StreamChunk, error) {
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
	adapter := p.adapter.(*openaiAdapter)
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
		parser := &openaiStreamParser{}
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

// openaiAdapter implements ProviderAdapter for OpenAI.
type openaiAdapter struct {
	config *ProviderConfig
	log    *logrus.Logger
}

func (a *openaiAdapter) Name() string {
	return "openai"
}

func (a *openaiAdapter) BuildRequest(systemPrompt, userPrompt string, stream bool) (interface{}, error) {
	req := &openaiRequest{
		Model:               a.config.Model,
		MaxCompletionTokens: a.config.MaxTokens,
		Temperature:         a.config.Temperature,
		Messages: []openaiMessage{
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

	// Add reasoning_effort if configured (for reasoning models like o1, o3, gpt-5-nano)
	if a.config.ReasoningEffort != "" {
		req.ReasoningEffort = a.config.ReasoningEffort
		a.log.WithFields(logrus.Fields{
			"model":            a.config.Model,
			"reasoning_effort": a.config.ReasoningEffort,
		}).Debug("Using reasoning effort configuration")
	}

	if stream {
		req.Stream = true
	}

	return req, nil
}

func (a *openaiAdapter) buildHealthRequest() (interface{}, error) {
	return &openaiRequest{
		Model:               a.config.Model,
		MaxCompletionTokens: 1000, // Higher limit for reasoning models
		Messages: []openaiMessage{
			{Role: "user", Content: "Respond with 'OK'"},
		},
	}, nil
}

func (a *openaiAdapter) ParseResponse(body []byte) (string, *TokenUsage, error) {
	var resp openaiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal response: %w (body preview: %s)",
			err, truncateString(string(body), 200))
	}

	if len(resp.Choices) == 0 {
		return "", nil, fmt.Errorf("no response choices returned")
	}

	choice := resp.Choices[0]
	content := choice.Message.Content
	refusal := choice.Message.Refusal

	// Handle refusal field (OpenAI content policy)
	if refusal != "" {
		return "", nil, fmt.Errorf("OpenAI refused request: %s", refusal)
	}

	// Handle empty content (can occur with reasoning models that exhaust tokens on reasoning)
	if content == "" {
		if choice.FinishReason == "length" {
			return "", nil, fmt.Errorf("OpenAI returned empty content: reasoning model exhausted token limit (model: %s, tokens used: %d). Consider increasing max_tokens or using a model with higher limits",
				resp.Model, resp.Usage.CompletionTokens)
		}
		return "", nil, fmt.Errorf("OpenAI returned empty content (model: %s, finish_reason: %s, tokens: %d)",
			resp.Model, choice.FinishReason, resp.Usage.CompletionTokens)
	}

	// Build usage info
	usage := &TokenUsage{
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		TotalTokens:  resp.Usage.TotalTokens,
	}

	return content, usage, nil
}

func (a *openaiAdapter) BuildHeaders() map[string]string {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + a.config.APIKey,
	}

	// Add custom headers
	for k, v := range a.config.CustomHeaders {
		headers[k] = v
	}

	return headers
}

func (a *openaiAdapter) GetEndpoint() string {
	return a.config.Endpoint
}

func (a *openaiAdapter) GetConfig() *ProviderConfig {
	return a.config
}

// openaiStreamParser parses OpenAI SSE streams.
type openaiStreamParser struct{}

func (p *openaiStreamParser) ParseLine(line string) (string, bool, error) {
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
	var event openaiStreamEvent
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

// OpenAI API request/response types

type openaiRequest struct {
	Model               string          `json:"model"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Temperature         float64         `json:"temperature,omitempty"`
	Messages            []openaiMessage `json:"messages"`
	Stream              bool            `json:"stream,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"` // For reasoning models: "low", "medium", "high"
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
			Refusal string `json:"refusal,omitempty"` // Added for OpenAI content policy
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openaiStreamEvent struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
			Refusal string `json:"refusal,omitempty"` // Added for OpenAI content policy
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}
