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
	// GeminiAPIURL is the default Google Gemini API endpoint
	GeminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models"

	// DefaultGeminiModel is the default Gemini model
	DefaultGeminiModel = "gemini-2.0-flash-exp"
)

// GeminiProvider implements the Provider interface for Google Gemini.
type GeminiProvider struct {
	config *ProviderConfig
	client *http.Client
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

	return &GeminiProvider{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		log: log,
	}, nil
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// Analyze analyzes diagnostic results using Gemini.
func (p *GeminiProvider) Analyze(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error) {
	start := time.Now()

	// Build prompts
	pb := NewPromptBuilder()
	systemPrompt := pb.BuildSystemPrompt()
	userPrompt, err := pb.BuildAnalysisPrompt(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build analysis prompt: %w", err)
	}

	// Build request body using typed struct
	requestBody := geminiRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{
						Text: systemPrompt + "\n\n" + userPrompt,
					},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:     p.config.Temperature,
			MaxOutputTokens: p.config.MaxTokens,
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	p.log.WithFields(logrus.Fields{
		"model":       p.config.Model,
		"temperature": p.config.Temperature,
		"max_tokens":  p.config.MaxTokens,
	}).Debug("Sending request to Gemini API")

	// Construct URL with model and API key
	url := fmt.Sprintf("%s/%s:generateContent?key=%s",
		p.config.Endpoint,
		p.config.Model,
		p.config.APIKey,
	)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &Error{
			Op:        "api_call",
			Provider:  p.Name(),
			Err:       err,
			Retryable: true,
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		retryable := resp.StatusCode >= 500 || resp.StatusCode == 429
		return nil, &Error{
			Op:        "api_call",
			Provider:  p.Name(),
			Err:       fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody)),
			Retryable: retryable,
		}
	}

	// Parse response using typed struct
	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		p.log.WithField("response_body", string(respBody)).Error("Failed to parse Gemini response JSON")
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Enhanced error handling for empty responses
	if len(geminiResp.Candidates) == 0 {
		previewLen := 500
		if len(respBody) < previewLen {
			previewLen = len(respBody)
		}
		p.log.WithFields(logrus.Fields{
			"response_preview": string(respBody[:previewLen]),
		}).Error("Gemini returned zero candidates - response may have been blocked or filtered")
		return nil, fmt.Errorf("no candidates in response - check API quota, content filters, or response body in logs")
	}

	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		finishReason := geminiResp.Candidates[0].FinishReason
		previewLen := 500
		if len(respBody) < previewLen {
			previewLen = len(respBody)
		}
		p.log.WithFields(logrus.Fields{
			"finish_reason":    finishReason,
			"candidates_count": len(geminiResp.Candidates),
			"response_preview": string(respBody[:previewLen]),
		}).Error("Gemini candidate has zero content parts")
		return nil, fmt.Errorf("no content parts in response (finish_reason: %s) - check logs for details", finishReason)
	}

	content := geminiResp.Candidates[0].Content.Parts[0].Text

	// Log warning if content is suspiciously short
	if len(content) < 50 {
		p.log.WithFields(logrus.Fields{
			"content_length": len(content),
			"content":        content,
		}).Warn("Gemini returned unusually short content")
	}

	// Check if Gemini used thinking tokens (Gemini 2.5 Flash extended thinking)
	thinkingTokensUsed := 0
	if totalTokens := geminiResp.UsageMetadata.TotalTokenCount; totalTokens > 0 {
		// thinking_tokens = total - prompt - candidates
		thinkingTokensUsed = totalTokens - geminiResp.UsageMetadata.PromptTokenCount - geminiResp.UsageMetadata.CandidatesTokenCount
	}

	logFields := logrus.Fields{
		"duration":      time.Since(start),
		"input_tokens":  geminiResp.UsageMetadata.PromptTokenCount,
		"output_tokens": geminiResp.UsageMetadata.CandidatesTokenCount,
		"total_tokens":  geminiResp.UsageMetadata.TotalTokenCount,
	}

	if thinkingTokensUsed > 0 {
		logFields["thinking_tokens"] = thinkingTokensUsed
		logFields["note"] = "Gemini 2.5 Flash used extended thinking mode"
	}

	p.log.WithFields(logFields).Debug("Received response from Gemini API")

	// Parse AI response
	analysis, err := ParseAnalysisResponse(content, p.Name(), p.config.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to parse analysis response: %w", err)
	}

	// Add token usage
	analysis.TokensUsed = &TokenUsage{
		InputTokens:  geminiResp.UsageMetadata.PromptTokenCount,
		OutputTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
		TotalTokens:  geminiResp.UsageMetadata.TotalTokenCount,
	}

	return analysis, nil
}

// AnalyzeStream analyzes diagnostic results using Gemini with streaming.
func (p *GeminiProvider) AnalyzeStream(ctx context.Context, req *AnalysisRequest) (<-chan StreamChunk, error) {
	chunkChan := make(chan StreamChunk, 10)

	go func() {
		defer close(chunkChan)

		// Build prompts
		pb := NewPromptBuilder()
		systemPrompt := pb.BuildSystemPrompt()
		userPrompt, err := pb.BuildAnalysisPrompt(req)
		if err != nil {
			chunkChan <- StreamChunk{
				Error: fmt.Errorf("failed to build analysis prompt: %w", err),
			}
			return
		}

		// Build request body with streaming enabled using typed struct
		requestBody := geminiRequest{
			Contents: []geminiContent{
				{
					Role: "user",
					Parts: []geminiPart{
						{
							Text: systemPrompt + "\n\n" + userPrompt,
						},
					},
				},
			},
			GenerationConfig: geminiGenerationConfig{
				Temperature:     p.config.Temperature,
				MaxOutputTokens: p.config.MaxTokens,
			},
		}

		bodyBytes, err := json.Marshal(requestBody)
		if err != nil {
			chunkChan <- StreamChunk{
				Error: fmt.Errorf("failed to marshal request: %w", err),
			}
			return
		}

		// Construct URL with streaming endpoint
		url := fmt.Sprintf("%s/%s:streamGenerateContent?key=%s&alt=sse",
			p.config.Endpoint,
			p.config.Model,
			p.config.APIKey,
		)

		// Create HTTP request
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			chunkChan <- StreamChunk{
				Error: fmt.Errorf("failed to create request: %w", err),
			}
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")

		// Send request
		resp, err := p.client.Do(httpReq)
		if err != nil {
			chunkChan <- StreamChunk{
				Error: &Error{
					Op:        "stream_call",
					Provider:  p.Name(),
					Err:       err,
					Retryable: true,
				},
			}
			return
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			retryable := resp.StatusCode >= 500 || resp.StatusCode == 429
			chunkChan <- StreamChunk{
				Error: &Error{
					Op:        "stream_call",
					Provider:  p.Name(),
					Err:       fmt.Errorf("API error %d: %s", resp.StatusCode, string(body)),
					Retryable: retryable,
				},
			}
			return
		}

		// Process SSE stream
		var fullContent strings.Builder
		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Text()

			// SSE format: "data: {...}"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			// Parse the JSON chunk using typed struct
			var chunkData geminiStreamChunk
			if err := json.Unmarshal([]byte(data), &chunkData); err != nil {
				p.log.WithError(err).Warn("Failed to parse stream chunk")
				continue
			}

			if len(chunkData.Candidates) > 0 && len(chunkData.Candidates[0].Content.Parts) > 0 {
				text := chunkData.Candidates[0].Content.Parts[0].Text
				fullContent.WriteString(text)

				chunkChan <- StreamChunk{
					Content: text,
					Done:    false,
				}
			}
		}

		if err := scanner.Err(); err != nil {
			chunkChan <- StreamChunk{
				Error: fmt.Errorf("stream error: %w", err),
			}
			return
		}

		// Send final chunk with complete content
		chunkChan <- StreamChunk{
			Content: fullContent.String(),
			Done:    true,
		}
	}()

	return chunkChan, nil
}

// Health checks if the Gemini API is accessible.
func (p *GeminiProvider) Health(ctx context.Context) error {
	// Construct a simple health check URL
	url := fmt.Sprintf("%s/%s?key=%s",
		p.config.Endpoint,
		p.config.Model,
		p.config.APIKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusOK {
		// Model exists (404 for GET on generateContent endpoint is expected, 200 means model info)
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, string(body))
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
