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
	config *ProviderConfig
	client *http.Client
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

	return &OpenRouterProvider{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		log: log,
	}, nil
}

// Name returns the provider name.
func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

// Analyze analyzes diagnostic results using OpenRouter.
func (p *OpenRouterProvider) Analyze(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error) {
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
	p.log.WithFields(logrus.Fields{
		"content_length":  len(respContent),
		"content_preview": truncateString(respContent, 300),
	}).Debug("Parsing AI response content")

	response, err := ParseAnalysisResponse(respContent, p.Name(), p.config.Model)
	if err != nil {
		p.log.WithFields(logrus.Fields{
			"error":          err.Error(),
			"content_length": len(respContent),
		}).Error("Failed to parse analysis response")
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

// Health checks if the OpenRouter API is accessible.
func (p *OpenRouterProvider) Health(ctx context.Context) error {
	// Simple health check: send a minimal request
	req := &openrouterRequest{
		Model:     p.config.Model,
		MaxTokens: 10,
		Messages: []openrouterMessage{
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

// buildRequest constructs an OpenRouter API request.
func (p *OpenRouterProvider) buildRequest(systemPrompt, userPrompt string) *openrouterRequest {
	return &openrouterRequest{
		Model:       p.config.Model,
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
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
}

// callAPI makes a non-streaming API call.
func (p *OpenRouterProvider) callAPI(ctx context.Context, req *openrouterRequest) (string, *TokenUsage, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	p.log.WithFields(logrus.Fields{
		"endpoint": p.config.Endpoint,
		"model":    req.Model,
		"messages": len(req.Messages),
	}).Debug("Sending OpenRouter API request")

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

	// Read the entire response body for logging and parsing
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %w", err)
	}

	p.log.WithFields(logrus.Fields{
		"status_code":  resp.StatusCode,
		"content_type": resp.Header.Get("Content-Type"),
		"body_length":  len(body),
	}).Debug("Received OpenRouter API response")

	if resp.StatusCode != http.StatusOK {
		p.log.WithFields(logrus.Fields{
			"status_code": resp.StatusCode,
			"body":        string(body),
		}).Error("OpenRouter API returned non-OK status")
		return "", nil, &Error{
			Op:        "api_call",
			Provider:  p.Name(),
			Err:       fmt.Errorf("API error %d: %s", resp.StatusCode, string(body)),
			Retryable: resp.StatusCode >= 500,
		}
	}

	var apiResp openrouterResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		p.log.WithFields(logrus.Fields{
			"error":        err.Error(),
			"body_length":  len(body),
			"body_preview": truncateString(string(body), 500),
		}).Error("Failed to parse OpenRouter response JSON")
		return "", nil, fmt.Errorf("failed to decode response: %w (body preview: %s)", err, truncateString(string(body), 200))
	}

	// Extract content from choices
	if len(apiResp.Choices) == 0 {
		p.log.Error("OpenRouter response has no choices")
		return "", nil, fmt.Errorf("no choices in response")
	}

	content := apiResp.Choices[0].Message.Content

	// Check for refusal (content policy violations)
	if apiResp.Choices[0].Message.Refusal != "" {
		p.log.WithField("refusal", apiResp.Choices[0].Message.Refusal).Warn("OpenRouter refused to respond")
		return "", nil, fmt.Errorf("content policy refusal: %s", apiResp.Choices[0].Message.Refusal)
	}

	// Handle empty content
	if content == "" {
		p.log.Warn("OpenRouter returned empty content")
		return "", nil, fmt.Errorf("empty content in response")
	}

	// Build usage info
	usage := &TokenUsage{
		InputTokens:  apiResp.Usage.PromptTokens,
		OutputTokens: apiResp.Usage.CompletionTokens,
		TotalTokens:  apiResp.Usage.TotalTokens,
	}

	p.log.WithFields(logrus.Fields{
		"content_length": len(content),
		"input_tokens":   usage.InputTokens,
		"output_tokens":  usage.OutputTokens,
		"total_tokens":   usage.TotalTokens,
	}).Debug("Successfully parsed OpenRouter response")

	return content, usage, nil
}

// streamAPI makes a streaming API call.
func (p *OpenRouterProvider) streamAPI(ctx context.Context, req *openrouterRequest, ch chan<- StreamChunk) error {
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
			ch <- StreamChunk{
				Type:    ChunkSummary,
				Content: fullContent.String(),
				Done:    true,
			}
			break
		}

		var event openrouterStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			p.log.WithError(err).Warn("Failed to parse stream event")
			continue
		}

		// Extract content delta
		if len(event.Choices) > 0 {
			delta := event.Choices[0].Delta.Content
			if delta != "" {
				fullContent.WriteString(delta)
				ch <- StreamChunk{
					Type:    ChunkSummary,
					Content: delta,
				}
			}

			// Check for finish
			if event.Choices[0].FinishReason != "" {
				ch <- StreamChunk{
					Type:    ChunkSummary,
					Content: fullContent.String(),
					Done:    true,
				}
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read error: %w", err)
	}

	return nil
}

// setHeaders sets required OpenRouter API headers.
func (p *OpenRouterProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	// OpenRouter-specific headers for tracking and analytics
	req.Header.Set("HTTP-Referer", OpenRouterAppURL)
	req.Header.Set("X-Title", OpenRouterAppName)

	// Add custom headers
	for k, v := range p.config.CustomHeaders {
		req.Header.Set(k, v)
	}
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
			Role    string `json:"role"`
			Content string `json:"content"`
			Refusal string `json:"refusal,omitempty"` // OpenAI-compatible content policy field
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
