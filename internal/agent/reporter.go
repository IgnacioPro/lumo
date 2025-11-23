package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/ignacio/lumo/internal/config"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// Reporter handles communication with the Lumo API server
type Reporter struct {
	cfg        *config.AgentConfig
	httpClient *http.Client
	logger     *logrus.Logger
	agentID    uuid.UUID
}

// NewReporter creates a new API reporter
func NewReporter(cfg *config.AgentConfig, logger *logrus.Logger) *Reporter {
	// Configure HTTP client with TLS settings
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.TLSInsecure,
		},
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: false,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return &Reporter{
		cfg:        cfg,
		httpClient: httpClient,
		logger:     logger,
	}
}

// SetAgentID sets the agent ID for this reporter
func (r *Reporter) SetAgentID(agentID uuid.UUID) {
	r.agentID = agentID
}

// RegisterAgent registers the agent with the API server
func (r *Reporter) RegisterAgent(ctx context.Context, req RegisterAgentRequest) (*RegisterAgentResponse, error) {
	// Create tracing span
	tracer := otel.Tracer("lumo.agent")
	ctx, span := tracer.Start(ctx, "RegisterAgent")
	defer span.End()

	// Set span attributes
	span.SetAttributes(
		attribute.String("agent.name", req.Name),
		attribute.String("agent.hostname", req.Hostname),
		attribute.String("agent.platform", req.Platform),
		attribute.String("agent.architecture", req.Architecture),
		attribute.String("agent.version", req.Version),
	)

	url := fmt.Sprintf("%s/api/v1/agents/register", r.cfg.APIEndpoint)

	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to marshal registration request")
		return nil, fmt.Errorf("failed to marshal registration request: %w", err)
	}

	// Perform request with retry
	var resp RegisterAgentResponse
	err = r.doWithRetry(ctx, "POST", url, body, &resp)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to register agent")
		return nil, fmt.Errorf("failed to register agent: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"agent_id": resp.AgentID,
		"hostname": resp.Hostname,
		"status":   resp.Status,
	}).Info("Agent registered successfully")

	// Parse and store agent ID
	agentID, err := uuid.Parse(resp.AgentID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid agent ID returned")
		return nil, fmt.Errorf("invalid agent ID returned: %w", err)
	}
	r.agentID = agentID

	span.SetAttributes(attribute.String("agent.id", agentID.String()))
	span.SetStatus(codes.Ok, "Agent registered successfully")

	return &resp, nil
}

// SendHeartbeat sends a heartbeat to the API server
func (r *Reporter) SendHeartbeat(ctx context.Context) error {
	// Create tracing span
	tracer := otel.Tracer("lumo.agent")
	ctx, span := tracer.Start(ctx, "SendHeartbeat")
	defer span.End()

	if r.agentID == uuid.Nil {
		err := fmt.Errorf("agent ID not set")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Agent ID not set")
		return err
	}

	span.SetAttributes(attribute.String("agent.id", r.agentID.String()))

	url := fmt.Sprintf("%s/api/v1/agents/%s/heartbeat", r.cfg.APIEndpoint, r.agentID)

	var resp map[string]interface{}
	err := r.doWithRetry(ctx, "PUT", url, nil, &resp)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to send heartbeat")
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	span.SetStatus(codes.Ok, "Heartbeat sent successfully")
	r.logger.Debug("Heartbeat sent successfully")
	return nil
}

// SubmitDiagnosticResult submits a diagnostic result to the API server
func (r *Reporter) SubmitDiagnosticResult(ctx context.Context, result DiagnosticResult) error {
	// Create tracing span
	tracer := otel.Tracer("lumo.agent")
	ctx, span := tracer.Start(ctx, "SubmitDiagnosticResult")
	defer span.End()

	// Set span attributes
	span.SetAttributes(
		attribute.String("diagnostic.target", result.Target),
		attribute.Int("diagnostic.checks_count", len(result.Checks)),
		attribute.Bool("diagnostic.analyze", result.Analyze),
	)

	url := fmt.Sprintf("%s/api/v1/diagnostics", r.cfg.APIEndpoint)

	// Marshal result
	body, err := json.Marshal(result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to marshal diagnostic result")
		return fmt.Errorf("failed to marshal diagnostic result: %w", err)
	}

	// Submit with retry
	var resp map[string]interface{}
	err = r.doWithRetry(ctx, "POST", url, body, &resp)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to submit diagnostic result")
		return fmt.Errorf("failed to submit diagnostic result: %w", err)
	}

	span.SetStatus(codes.Ok, "Diagnostic result submitted successfully")
	r.logger.WithFields(logrus.Fields{
		"target": result.Target,
		"checks": len(result.Checks),
	}).Info("Diagnostic result submitted successfully")

	return nil
}

// doWithRetry performs an HTTP request with exponential backoff retry
func (r *Reporter) doWithRetry(ctx context.Context, method, url string, body []byte, respData interface{}) error {
	var lastErr error
	baseDelay := r.cfg.RetryBaseDelay
	maxAttempts := r.cfg.RetryMaxAttempts

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Create request
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		if r.cfg.Token != "" {
			req.Header.Set("X-API-Key", r.cfg.Token)
		}

		// Perform request
		resp, err := r.httpClient.Do(req)
		if err != nil {
			lastErr = err
			r.logger.WithError(err).WithFields(logrus.Fields{
				"attempt": attempt + 1,
				"max":     maxAttempts,
			}).Warn("HTTP request failed, retrying...")

			// Wait before retry with exponential backoff
			if attempt < maxAttempts-1 {
				delay := baseDelay * time.Duration(1<<uint(attempt)) // 2s, 4s, 8s, 16s
				time.Sleep(delay)
			}
			continue
		}

		// Read response body
		defer func() {
			if err := resp.Body.Close(); err != nil {
				r.logger.WithError(err).Debug("Failed to close response body")
			}
		}()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		// Check status code
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Success - unmarshal response
			if respData != nil {
				// API wraps responses in {success: bool, data: T}
				var apiResp struct {
					Success bool            `json:"success"`
					Data    json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal(respBody, &apiResp); err != nil {
					return fmt.Errorf("failed to unmarshal API response wrapper: %w", err)
				}
				// Unmarshal the actual data
				if err := json.Unmarshal(apiResp.Data, respData); err != nil {
					return fmt.Errorf("failed to unmarshal response data: %w", err)
				}
			}
			return nil
		}

		// Handle error responses
		lastErr = fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))

		// Don't retry on client errors (4xx)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return lastErr
		}

		// Retry on server errors (5xx) with backoff
		r.logger.WithFields(logrus.Fields{
			"status":  resp.StatusCode,
			"attempt": attempt + 1,
			"max":     maxAttempts,
		}).Warn("Server error, retrying...")

		if attempt < maxAttempts-1 {
			delay := baseDelay * time.Duration(1<<uint(attempt))
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("request failed after %d attempts: %w", maxAttempts, lastErr)
}

// RegisterAgentRequest represents an agent registration request
type RegisterAgentRequest struct {
	Name               string                 `json:"name"`
	Hostname           string                 `json:"hostname"`
	IPAddress          string                 `json:"ip_address,omitempty"`
	Platform           string                 `json:"platform"`
	Architecture       string                 `json:"architecture"`
	Version            string                 `json:"version"`
	Capabilities       []string               `json:"capabilities"`
	Labels             map[string]interface{} `json:"labels,omitempty"`
	KubernetesMetadata *KubernetesMetadata    `json:"kubernetes_metadata,omitempty"`
}

// RegisterAgentResponse represents the response to an agent registration
type RegisterAgentResponse struct {
	AgentID      string `json:"agent_id"`
	Name         string `json:"name"`
	Hostname     string `json:"hostname"`
	Status       string `json:"status"`
	RegisteredAt string `json:"registered_at"`
}

// KubernetesMetadata contains Kubernetes-specific agent information
type KubernetesMetadata struct {
	Cluster   string `json:"cluster,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	NodeName  string `json:"node_name,omitempty"`
	PodName   string `json:"pod_name,omitempty"`
}

// DiagnosticResult represents a diagnostic result to submit
type DiagnosticResult struct {
	Target  string   `json:"target"`
	Checks  []string `json:"checks"`
	Format  string   `json:"format,omitempty"`
	Analyze bool     `json:"analyze,omitempty"`
}

// IsAvailable checks if the API server is reachable
func (r *Reporter) IsAvailable(ctx context.Context) bool {
	url := fmt.Sprintf("%s/api/v1/health", r.cfg.APIEndpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			r.logger.WithError(err).Debug("Failed to close response body")
		}
	}()

	return resp.StatusCode == http.StatusOK
}
