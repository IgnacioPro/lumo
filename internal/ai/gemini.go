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
	// GeminiAPIURL is the default Google Gemini API endpoint
	GeminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models"

	// DefaultGeminiModel is the default Gemini model
	DefaultGeminiModel = "gemini-2.0-flash-exp"
)

// GeminiProvider implements the Provider interface for Google Gemini.
type GeminiProvider struct {
	*BaseProvider
	config *ProviderConfig
	log    *logrus.Logger
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(config *ProviderConfig, log *logrus.Logger) (*GeminiProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("gemini API key is required")
	}

	if config.Model == "" {
		config.Model = DefaultGeminiModel
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
		config.Endpoint = GeminiAPIURL
	}

	// Validate endpoint for security (allow localhost for testing)
	if err := ValidateEndpoint(config.Endpoint, false); err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	// Create adapter
	adapter := &geminiAdapter{
		config: config,
		log:    log,
	}

	// Create base provider
	base := NewBaseProvider(adapter, log)

	return &GeminiProvider{
		BaseProvider: base,
		config:       config,
		log:          log,
	}, nil
}

// Health checks if the Gemini API is accessible.
func (p *GeminiProvider) Health(ctx context.Context) error {
	// Gemini health check: GET model info
	adapter := p.adapter.(*geminiAdapter)
	url := fmt.Sprintf("%s/%s?key=%s", adapter.GetEndpoint(), adapter.config.Model, adapter.config.APIKey)

	resp, err := p.httpClient.Do(ctx, RequestOptions{
		Method:       "GET",
		URL:          url,
		Headers:      nil,
		ProviderName: p.Name(),
	})

	// Accept both 200 (model info) and 404 (expected for GET on generateContent endpoint)
	if err == nil {
		if len(resp.Body) > 0 {
			return nil
		}
		return &Error{
			Op:        "health_check",
			Provider:  p.Name(),
			Err:       fmt.Errorf("unexpected empty response"),
			Retryable: true,
		}
	}

	// Check if it's just a 404 (which is OK for Gemini)
	if strings.Contains(err.Error(), "404") {
		return nil
	}

	return &Error{
		Op:        "health_check",
		Provider:  p.Name(),
		Err:       err,
		Retryable: true,
	}
}

// AnalyzeStream analyzes with streaming response.
func (p *GeminiProvider) AnalyzeStream(ctx context.Context, req *AnalysisRequest) (<-chan StreamChunk, error) {
	// Build prompts
	pb := NewPromptBuilder()
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
	adapter := p.adapter.(*geminiAdapter)
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

		// Build streaming URL with alt=sse parameter
		streamURL := fmt.Sprintf("%s/%s:streamGenerateContent?key=%s&alt=sse",
			adapter.GetEndpoint(),
			adapter.config.Model,
			adapter.config.APIKey)

		// Execute streaming HTTP request
		resp, err := p.httpClient.DoStreaming(ctx, RequestOptions{
			Method:       "POST",
			URL:          streamURL,
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
		parser := &geminiStreamParser{}
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

// geminiAdapter implements ProviderAdapter for Gemini.
type geminiAdapter struct {
	config *ProviderConfig
	log    *logrus.Logger
}

func (a *geminiAdapter) Name() string {
	return "gemini"
}

func (a *geminiAdapter) BuildRequest(systemPrompt, userPrompt string, stream bool) (interface{}, error) {
	// Gemini combines system and user prompts
	combinedPrompt := systemPrompt + "\n\n" + userPrompt

	return &geminiRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: combinedPrompt},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:     a.config.Temperature,
			MaxOutputTokens: a.config.MaxTokens,
		},
	}, nil
}

func (a *geminiAdapter) ParseResponse(body []byte) (string, *TokenUsage, error) {
	var resp geminiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal response: %w (body preview: %s)",
			err, truncateString(string(body), 200))
	}

	// Enhanced error handling for empty responses
	if len(resp.Candidates) == 0 {
		return "", nil, fmt.Errorf("no candidates in response - check API quota, content filters, or response body in logs")
	}

	if len(resp.Candidates[0].Content.Parts) == 0 {
		finishReason := resp.Candidates[0].FinishReason
		return "", nil, fmt.Errorf("no content parts in response (finish_reason: %s) - check logs for details", finishReason)
	}

	content := resp.Candidates[0].Content.Parts[0].Text

	// Log warning if content is suspiciously short
	if len(content) < 50 {
		a.log.WithFields(logrus.Fields{
			"content_length": len(content),
			"content":        content,
		}).Warn("Gemini returned unusually short content")
	}

	// Check if Gemini used thinking tokens (Gemini 2.5 Flash extended thinking)
	thinkingTokensUsed := 0
	if totalTokens := resp.UsageMetadata.TotalTokenCount; totalTokens > 0 {
		// thinking_tokens = total - prompt - candidates
		thinkingTokensUsed = totalTokens - resp.UsageMetadata.PromptTokenCount - resp.UsageMetadata.CandidatesTokenCount
	}

	if thinkingTokensUsed > 0 {
		a.log.WithFields(logrus.Fields{
			"thinking_tokens": thinkingTokensUsed,
			"note":            "Gemini 2.5 Flash used extended thinking mode",
		}).Debug("Extended thinking mode detected")
	}

	// Build usage info
	usage := &TokenUsage{
		InputTokens:  resp.UsageMetadata.PromptTokenCount,
		OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
		TotalTokens:  resp.UsageMetadata.TotalTokenCount,
	}

	return content, usage, nil
}

func (a *geminiAdapter) BuildHeaders() map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Add custom headers
	for k, v := range a.config.CustomHeaders {
		headers[k] = v
	}

	return headers
}

func (a *geminiAdapter) GetEndpoint() string {
	// Gemini uses API key in URL, not in endpoint
	return fmt.Sprintf("%s/%s:generateContent?key=%s",
		a.config.Endpoint,
		a.config.Model,
		a.config.APIKey)
}

func (a *geminiAdapter) GetConfig() *ProviderConfig {
	return a.config
}

// geminiStreamParser parses Gemini SSE streams.
type geminiStreamParser struct{}

func (p *geminiStreamParser) ParseLine(line string) (string, bool, error) {
	// Check for SSE "data: " prefix
	if !strings.HasPrefix(line, "data: ") {
		return "", false, nil
	}

	data := strings.TrimPrefix(line, "data: ")

	// Check for [DONE] marker
	if data == "[DONE]" {
		return "", true, nil
	}

	// Parse JSON chunk
	var chunk geminiStreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return "", false, fmt.Errorf("failed to parse stream chunk: %w", err)
	}

	if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
		text := chunk.Candidates[0].Content.Parts[0].Text
		return text, false, nil
	}

	return "", false, nil
}

// Gemini API request/response types

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

type geminiStreamChunk struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}
