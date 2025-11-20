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
	// DefaultOllamaURL is the default Ollama API endpoint
	DefaultOllamaURL = "http://localhost:11434/api/chat"

	// DefaultOllamaModel is the default local model
	DefaultOllamaModel = "llama3.1:8b"
)

// OllamaProvider implements the Provider interface for Ollama (local models).
type OllamaProvider struct {
	*BaseProvider
	config *ProviderConfig
	log    *logrus.Logger
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(config *ProviderConfig, log *logrus.Logger) (*OllamaProvider, error) {
	if config.Model == "" {
		config.Model = DefaultOllamaModel
	}

	if config.Timeout == 0 {
		config.Timeout = 300 * time.Second // Local models can be slower
	}

	if config.Temperature == 0 {
		// Default to 1.0 for consistency with Anthropic and more creative responses
		config.Temperature = 1.0
	}

	if config.Endpoint == "" {
		config.Endpoint = DefaultOllamaURL
	}

	// Validate endpoint for security (allow localhost since Ollama runs locally)
	if err := ValidateEndpoint(config.Endpoint, true); err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	// Create adapter
	adapter := &ollamaAdapter{
		config: config,
	}

	// Create base provider
	base := NewBaseProvider(adapter, log)

	return &OllamaProvider{
		BaseProvider: base,
		config:       config,
		log:          log,
	}, nil
}

// Health checks if the Ollama service is accessible.
func (p *OllamaProvider) Health(ctx context.Context) error {
	// Check if Ollama is running by hitting the health endpoint
	healthURL := strings.Replace(p.config.Endpoint, "/api/chat", "/api/tags", 1)

	resp, err := p.httpClient.Do(ctx, RequestOptions{
		Method:       "GET",
		URL:          healthURL,
		Headers:      nil,
		ProviderName: p.Name(),
	})
	if err != nil {
		return &Error{
			Op:        "health_check",
			Provider:  p.Name(),
			Err:       fmt.Errorf("ollama service not accessible: %w", err),
			Retryable: true,
		}
	}

	if len(resp.Body) == 0 {
		return &Error{
			Op:        "health_check",
			Provider:  p.Name(),
			Err:       fmt.Errorf("empty response from ollama"),
			Retryable: true,
		}
	}

	return nil
}

// AnalyzeStream analyzes with streaming response.
func (p *OllamaProvider) AnalyzeStream(ctx context.Context, req *AnalysisRequest) (<-chan StreamChunk, error) {
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
	adapter := p.adapter.(*ollamaAdapter)
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

		// Use JSON line parser (not SSE)
		parser := &ollamaStreamParser{}
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

// ollamaAdapter implements ProviderAdapter for Ollama.
type ollamaAdapter struct {
	config *ProviderConfig
}

func (a *ollamaAdapter) Name() string {
	return "ollama"
}

func (a *ollamaAdapter) BuildRequest(systemPrompt, userPrompt string, stream bool) (interface{}, error) {
	req := &ollamaRequest{
		Model: a.config.Model,
		Messages: []ollamaMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Options: ollamaOptions{
			Temperature: a.config.Temperature,
			NumPredict:  a.config.MaxTokens,
		},
	}

	if stream {
		req.Stream = true
	}

	return req, nil
}

func (a *ollamaAdapter) ParseResponse(body []byte) (string, *TokenUsage, error) {
	var resp ollamaResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	content := resp.Message.Content

	// Ollama doesn't provide token usage
	return content, nil, nil
}

func (a *ollamaAdapter) BuildHeaders() map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Add custom headers
	for k, v := range a.config.CustomHeaders {
		headers[k] = v
	}

	return headers
}

func (a *ollamaAdapter) GetEndpoint() string {
	return a.config.Endpoint
}

func (a *ollamaAdapter) GetConfig() *ProviderConfig {
	return a.config
}

// ollamaStreamParser parses Ollama JSON-per-line streams.
type ollamaStreamParser struct{}

func (p *ollamaStreamParser) ParseLine(line string) (string, bool, error) {
	// Ollama uses JSON-per-line, not SSE
	var event ollamaStreamEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return "", false, fmt.Errorf("failed to parse JSON line: %w", err)
	}

	// Extract content
	content := event.Message.Content

	// Check if done
	done := event.Done

	return content, done, nil
}

// Ollama API request/response types

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream,omitempty"`
	Options  ollamaOptions   `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

type ollamaStreamEvent struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}
