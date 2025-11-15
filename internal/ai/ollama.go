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
	// DefaultOllamaURL is the default Ollama API endpoint
	DefaultOllamaURL = "http://localhost:11434/api/chat"

	// DefaultOllamaModel is the default local model
	DefaultOllamaModel = "llama3.1:8b"
)

// OllamaProvider implements the Provider interface for Ollama (local models).
type OllamaProvider struct {
	config *ProviderConfig
	client *http.Client
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

	return &OllamaProvider{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		log: log,
	}, nil
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Analyze analyzes diagnostic results using a local Ollama model.
func (p *OllamaProvider) Analyze(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error) {
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
		"endpoint":     p.config.Endpoint,
		"system_chars": len(systemPrompt),
		"user_chars":   len(userPrompt),
	}).Debug("Built analysis prompts")

	// Make API request
	apiReq := p.buildRequest(systemPrompt, userPrompt)
	respContent, err := p.callAPI(ctx, apiReq)
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

	// Set metadata (Ollama doesn't provide token usage)
	response.Timestamp = time.Now()
	response.Duration = time.Since(start)

	p.log.WithFields(logrus.Fields{
		"findings":        len(response.Findings),
		"recommendations": len(response.Recommendations),
		"health":          response.OverallHealth,
		"duration":        response.Duration,
	}).Info("Analysis complete")

	return response, nil
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

// Health checks if the Ollama service is accessible.
func (p *OllamaProvider) Health(ctx context.Context) error {
	// Check if Ollama is running by hitting the health endpoint
	healthURL := strings.Replace(p.config.Endpoint, "/api/chat", "/api/tags", 1)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return &Error{
			Op:        "health_check",
			Provider:  p.Name(),
			Err:       fmt.Errorf("ollama service not accessible: %w", err),
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &Error{
			Op:        "health_check",
			Provider:  p.Name(),
			Err:       fmt.Errorf("ollama service returned status %d", resp.StatusCode),
			Retryable: true,
		}
	}

	return nil
}

// buildRequest constructs an Ollama API request.
func (p *OllamaProvider) buildRequest(systemPrompt, userPrompt string) *ollamaRequest {
	return &ollamaRequest{
		Model: p.config.Model,
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
			Temperature: p.config.Temperature,
			NumPredict:  p.config.MaxTokens,
		},
	}
}

// callAPI makes a non-streaming API call.
func (p *OllamaProvider) callAPI(ctx context.Context, req *ollamaRequest) (string, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", &Error{
			Op:        "api_call",
			Provider:  p.Name(),
			Err:       err,
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", &Error{
			Op:        "api_call",
			Provider:  p.Name(),
			Err:       fmt.Errorf("API error %d: %s", resp.StatusCode, string(body)),
			Retryable: resp.StatusCode >= 500,
		}
	}

	var apiResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return apiResp.Message.Content, nil
}

// streamAPI makes a streaming API call.
func (p *OllamaProvider) streamAPI(ctx context.Context, req *ollamaRequest, ch chan<- StreamChunk) error {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

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

	// Read streaming response (Ollama sends JSON objects line by line)
	scanner := bufio.NewScanner(resp.Body)
	var fullContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		var event ollamaStreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			p.log.WithError(err).Warn("Failed to parse stream event")
			continue
		}

		// Append content
		if event.Message.Content != "" {
			fullContent.WriteString(event.Message.Content)
			ch <- StreamChunk{
				Type:    ChunkSummary,
				Content: event.Message.Content,
			}
		}

		// Check if done
		if event.Done {
			ch <- StreamChunk{
				Type:    ChunkSummary,
				Content: fullContent.String(),
				Done:    true,
			}
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read error: %w", err)
	}

	return nil
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
