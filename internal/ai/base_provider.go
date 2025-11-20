package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// ProviderAdapter defines provider-specific operations that must be implemented
// by each AI provider. This interface allows BaseProvider to handle common logic
// while delegating provider-specific behavior to the adapter.
type ProviderAdapter interface {
	// Name returns the provider name (e.g., "anthropic", "openai")
	Name() string

	// BuildRequest constructs a provider-specific API request from prompts.
	// Returns the request object (will be JSON-marshaled by HTTPClient).
	BuildRequest(systemPrompt, userPrompt string, stream bool) (interface{}, error)

	// ParseResponse parses a provider-specific API response body.
	// Returns the content string and token usage (nil if not supported).
	ParseResponse(body []byte) (content string, usage *TokenUsage, err error)

	// BuildHeaders returns provider-specific HTTP headers.
	BuildHeaders() map[string]string

	// GetEndpoint returns the API endpoint URL.
	GetEndpoint() string

	// GetConfig returns the provider configuration.
	GetConfig() *ProviderConfig
}

// BaseProvider provides common implementation for AI providers.
// It handles prompt building, response parsing, and metadata management,
// delegating provider-specific operations to the ProviderAdapter.
type BaseProvider struct {
	adapter    ProviderAdapter
	httpClient *HTTPClient
	log        *logrus.Logger
}

// NewBaseProvider creates a new base provider with the given adapter.
func NewBaseProvider(adapter ProviderAdapter, log *logrus.Logger) *BaseProvider {
	config := adapter.GetConfig()

	return &BaseProvider{
		adapter:    adapter,
		httpClient: NewHTTPClient(config.Timeout, log),
		log:        log,
	}
}

// SetHTTPClient sets a custom HTTP client (primarily for testing).
func (p *BaseProvider) SetHTTPClient(client *HTTPClient) {
	p.httpClient = client
}

// Name returns the provider name.
func (p *BaseProvider) Name() string {
	return p.adapter.Name()
}

// Analyze analyzes diagnostic results using the AI provider.
// This method implements the common analysis workflow, delegating
// provider-specific operations to the adapter.
func (p *BaseProvider) Analyze(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error) {
	start := time.Now()

	// Build prompts (common logic)
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
		"model":        p.adapter.GetConfig().Model,
		"system_chars": len(systemPrompt),
		"user_chars":   len(userPrompt),
	}).Debug("Built analysis prompts")

	// Build provider-specific request
	apiReq, err := p.adapter.BuildRequest(systemPrompt, userPrompt, false)
	if err != nil {
		return nil, &Error{
			Op:       "build_request",
			Provider: p.Name(),
			Err:      err,
		}
	}

	// Execute HTTP request
	resp, err := p.httpClient.Do(ctx, RequestOptions{
		Method:       "POST",
		URL:          p.adapter.GetEndpoint(),
		Body:         apiReq,
		Headers:      p.adapter.BuildHeaders(),
		ProviderName: p.Name(),
	})
	if err != nil {
		return nil, err // Error already wrapped by HTTPClient
	}

	// Parse provider-specific response
	respContent, usage, err := p.adapter.ParseResponse(resp.Body)
	if err != nil {
		return nil, &Error{
			Op:       "parse_response",
			Provider: p.Name(),
			Err:      err,
		}
	}

	p.log.WithFields(logrus.Fields{
		"content_length":  len(respContent),
		"content_preview": truncateString(respContent, 300),
	}).Debug("Parsing AI response content")

	// Parse AI analysis response (common logic)
	response, err := ParseAnalysisResponse(respContent, p.Name(), p.adapter.GetConfig().Model)
	if err != nil {
		p.log.WithFields(logrus.Fields{
			"error":          err.Error(),
			"content_length": len(respContent),
		}).Error("Failed to parse analysis response")
		return nil, &Error{
			Op:       "parse_analysis",
			Provider: p.Name(),
			Err:      err,
		}
	}

	// Set metadata (common logic)
	response.Timestamp = time.Now()
	response.Duration = time.Since(start)
	response.TokensUsed = usage

	p.log.WithFields(logrus.Fields{
		"findings":        len(response.Findings),
		"recommendations": len(response.Recommendations),
		"health":          response.OverallHealth,
		"duration":        response.Duration,
	}).Info("Analysis complete")

	if usage != nil {
		p.log.WithField("tokens", usage.TotalTokens).Debug("Token usage")
	}

	return response, nil
}

// AnalyzeStream analyzes with streaming response.
// Returns a channel that receives chunks of the analysis as they arrive.
func (p *BaseProvider) AnalyzeStream(ctx context.Context, req *AnalysisRequest) (<-chan StreamChunk, error) {
	// Build prompts (common logic)
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

	// Build provider-specific streaming request
	apiReq, err := p.adapter.BuildRequest(systemPrompt, userPrompt, true)
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
			URL:          p.adapter.GetEndpoint(),
			Body:         apiReq,
			Headers:      p.adapter.BuildHeaders(),
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
		defer func() {
			_ = resp.Body.Close()
		}()

		// Parse streaming response (provider-specific, handled in next phase)
		// For now, send error indicating streaming not yet implemented
		ch <- StreamChunk{
			Type:  ChunkError,
			Error: fmt.Errorf("streaming implementation pending for %s", p.Name()),
			Done:  true,
		}
	}()

	return ch, nil
}

// Health checks if the AI provider is accessible.
// Default implementation sends a minimal request. Providers can override this.
func (p *BaseProvider) Health(ctx context.Context) error {
	// Simple health check: build minimal request
	minimalReq, err := p.adapter.BuildRequest("You are a helpful assistant.", "Respond with 'OK'", false)
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
		URL:          p.adapter.GetEndpoint(),
		Body:         minimalReq,
		Headers:      p.adapter.BuildHeaders(),
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

	// Just verify we got a response
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
