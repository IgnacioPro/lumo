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
	// OpenAIAPIURL is the default OpenAI API endpoint
	OpenAIAPIURL = "https://api.openai.com/v1/chat/completions"

	// DefaultOpenAIModel is the default GPT model
	DefaultOpenAIModel = "gpt-4-turbo-preview"
)

// OpenAIProvider implements the Provider interface for OpenAI GPT.
type OpenAIProvider struct {
	config *ProviderConfig
	client *http.Client
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
		config.Temperature = 0.7
	}

	if config.Endpoint == "" {
		config.Endpoint = OpenAIAPIURL
	}

	return &OpenAIProvider{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		log: log,
	}, nil
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Analyze analyzes diagnostic results using GPT.
func (p *OpenAIProvider) Analyze(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error) {
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

// Health checks if the OpenAI API is accessible.
func (p *OpenAIProvider) Health(ctx context.Context) error {
	// Simple health check: send a minimal request
	req := &openaiRequest{
		Model:      p.config.Model,
		MaxTokens:  10,
		Messages: []openaiMessage{
			{Role: "user", Content: "test"},
		},
	}

	_, _, err := p.callAPI(ctx, req)
	if err != nil {
		return &Error{
			Op:       "health_check",
			Provider: p.Name(),
			Err:      err,
			Retryable: true,
		}
	}

	return nil
}

// buildRequest constructs an OpenAI API request.
func (p *OpenAIProvider) buildRequest(systemPrompt, userPrompt string) *openaiRequest {
	return &openaiRequest{
		Model:       p.config.Model,
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
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
}

// callAPI makes a non-streaming API call.
func (p *OpenAIProvider) callAPI(ctx context.Context, req *openaiRequest) (string, *TokenUsage, error) {
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

	var apiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return "", nil, fmt.Errorf("no response choices returned")
	}

	// Build usage info
	usage := &TokenUsage{
		InputTokens:  apiResp.Usage.PromptTokens,
		OutputTokens: apiResp.Usage.CompletionTokens,
		TotalTokens:  apiResp.Usage.TotalTokens,
	}

	return apiResp.Choices[0].Message.Content, usage, nil
}

// streamAPI makes a streaming API call.
func (p *OpenAIProvider) streamAPI(ctx context.Context, req *openaiRequest, ch chan<- StreamChunk) error {
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

		var event openaiStreamEvent
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

// setHeaders sets required OpenAI API headers.
func (p *OpenAIProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	// Add custom headers
	for k, v := range p.config.CustomHeaders {
		req.Header.Set(k, v)
	}
}

// OpenAI API request/response types

type openaiRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Messages    []openaiMessage `json:"messages"`
	Stream      bool            `json:"stream,omitempty"`
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
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}
