package agent

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/grpc/client"
	"github.com/ignacio/lumo/internal/version"
)

// GRPCReporter handles communication with the Lumo gRPC API server
type GRPCReporter struct {
	client  *client.Client
	cfg     *config.AgentConfig
	logger  *logrus.Logger
	agentID uuid.UUID
}

// NewGRPCReporter creates a new gRPC API reporter
func NewGRPCReporter(cfg *config.AgentConfig, logger *logrus.Logger) (*GRPCReporter, error) {
	// Create gRPC client
	grpcClient, err := client.NewClient(client.ClientOptions{
		Address:    cfg.APIEndpoint,
		TLSEnabled: cfg.TLSEnabled,
		CertFile:   cfg.TLSCertFile,
		KeyFile:    cfg.TLSKeyFile,
		CAFile:     cfg.TLSCAFile,
		Token:      cfg.Token,
		Timeout:    30 * time.Second,
		MaxMsgSize: 10 * 1024 * 1024, // 10MB
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	return &GRPCReporter{
		client: grpcClient,
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Close closes the gRPC client connection
func (r *GRPCReporter) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// SetAgentID sets the agent ID for this reporter
func (r *GRPCReporter) SetAgentID(agentID uuid.UUID) {
	r.agentID = agentID
}

// RegisterAgent registers the agent with the gRPC API server
func (r *GRPCReporter) RegisterAgent(ctx context.Context, req RegisterAgentRequest) (*RegisterAgentResponse, error) {
	// Get hostname if not provided
	hostname := req.Hostname
	if hostname == "" {
		var err error
		hostname, err = os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get hostname: %w", err)
		}
	}

	// Convert labels to string map
	labels := make(map[string]string)
	for k, v := range req.Labels {
		labels[k] = fmt.Sprintf("%v", v)
	}

	// Build gRPC request
	grpcReq := &lumov1.RegisterAgentRequest{
		Name:         req.Name,
		Hostname:     hostname,
		IpAddress:    req.IPAddress,
		Platform:     req.Platform,
		Architecture: req.Architecture,
		Version:      req.Version,
		Capabilities: req.Capabilities,
		Labels:       labels,
	}

	// Add Kubernetes metadata if available
	if req.KubernetesMetadata != nil {
		// Note: Pod labels could be populated from Kubernetes API if needed
		// For now, we use the agent's general labels as K8s labels
		grpcReq.Kubernetes = &lumov1.KubernetesMetadata{
			Namespace: req.KubernetesMetadata.Namespace,
			PodName:   req.KubernetesMetadata.PodName,
			NodeName:  req.KubernetesMetadata.NodeName,
			Labels:    labels, // Use agent labels (could be enhanced to query K8s API for pod labels)
		}
	}

	// Perform registration with retry
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var resp *lumov1.RegisterAgentResponse
	var lastErr error

	for attempt := 0; attempt < r.cfg.RetryMaxAttempts; attempt++ {
		resp, lastErr = r.client.RegisterAgent(ctxWithTimeout, grpcReq)
		if lastErr == nil {
			break
		}

		r.logger.WithError(lastErr).WithFields(logrus.Fields{
			"attempt": attempt + 1,
			"max":     r.cfg.RetryMaxAttempts,
		}).Warn("Failed to register agent, retrying...")

		if attempt < r.cfg.RetryMaxAttempts-1 {
			delay := r.cfg.RetryBaseDelay * time.Duration(1<<uint(attempt))
			time.Sleep(delay)
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to register agent after %d attempts: %w", r.cfg.RetryMaxAttempts, lastErr)
	}

	// Parse and store agent ID
	agentID, err := uuid.Parse(resp.AgentId)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID returned: %w", err)
	}
	r.agentID = agentID

	r.logger.WithFields(logrus.Fields{
		"agent_id":      resp.AgentId,
		"registered_at": resp.RegisteredAt.AsTime(),
	}).Info("Agent registered successfully via gRPC")

	return &RegisterAgentResponse{
		AgentID:      resp.AgentId,
		Hostname:     hostname,
		Status:       "online",
		RegisteredAt: resp.RegisteredAt.AsTime().Format(time.RFC3339),
	}, nil
}

// SendHeartbeat sends a heartbeat to the gRPC API server
func (r *GRPCReporter) SendHeartbeat(ctx context.Context) error {
	if r.agentID == uuid.Nil {
		return fmt.Errorf("agent ID not set")
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Build health status
	health := &lumov1.AgentHealth{
		Status: lumov1.AgentHealth_HEALTH_STATUS_HEALTHY,
	}

	var lastErr error
	for attempt := 0; attempt < r.cfg.RetryMaxAttempts; attempt++ {
		_, lastErr = r.client.SendHeartbeat(ctxWithTimeout, r.agentID.String(), health)
		if lastErr == nil {
			r.logger.Debug("Heartbeat sent successfully via gRPC")
			return nil
		}

		r.logger.WithError(lastErr).WithFields(logrus.Fields{
			"attempt": attempt + 1,
			"max":     r.cfg.RetryMaxAttempts,
		}).Warn("Failed to send heartbeat, retrying...")

		if attempt < r.cfg.RetryMaxAttempts-1 {
			delay := r.cfg.RetryBaseDelay * time.Duration(1<<uint(attempt))
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("failed to send heartbeat after %d attempts: %w", r.cfg.RetryMaxAttempts, lastErr)
}

// SubmitDiagnosticResult submits a diagnostic result to the gRPC API server
func (r *GRPCReporter) SubmitDiagnosticResult(ctx context.Context, result DiagnosticResult) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Convert output format
	var format lumov1.OutputFormat
	switch result.Format {
	case "text":
		format = lumov1.OutputFormat_OUTPUT_FORMAT_TEXT
	case "json":
		format = lumov1.OutputFormat_OUTPUT_FORMAT_JSON
	case "toon":
		format = lumov1.OutputFormat_OUTPUT_FORMAT_TOON
	default:
		format = lumov1.OutputFormat_OUTPUT_FORMAT_UNSPECIFIED
	}

	// Build gRPC request
	grpcReq := &lumov1.RunDiagnosticsRequest{
		Target:  result.Target,
		Checks:  result.Checks,
		Analyze: result.Analyze,
		Format:  format,
	}

	var lastErr error
	for attempt := 0; attempt < r.cfg.RetryMaxAttempts; attempt++ {
		resp, err := r.client.RunDiagnostics(ctxWithTimeout, grpcReq)
		if err == nil {
			r.logger.WithFields(logrus.Fields{
				"job_id": resp.JobId,
				"target": result.Target,
				"checks": len(result.Checks),
			}).Info("Diagnostic result submitted successfully via gRPC")
			return nil
		}

		lastErr = err
		r.logger.WithError(err).WithFields(logrus.Fields{
			"attempt": attempt + 1,
			"max":     r.cfg.RetryMaxAttempts,
		}).Warn("Failed to submit diagnostic result, retrying...")

		if attempt < r.cfg.RetryMaxAttempts-1 {
			delay := r.cfg.RetryBaseDelay * time.Duration(1<<uint(attempt))
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("failed to submit diagnostic result after %d attempts: %w", r.cfg.RetryMaxAttempts, lastErr)
}

// IsAvailable checks if the gRPC API server is reachable
func (r *GRPCReporter) IsAvailable(ctx context.Context) bool {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := r.client.Live(ctxWithTimeout)
	return err == nil
}

// GetAgentInfo returns information about this agent
func (r *GRPCReporter) GetAgentInfo() *lumov1.RegisterAgentRequest {
	hostname, _ := os.Hostname()

	return &lumov1.RegisterAgentRequest{
		Name:         "lumo-agent",
		Hostname:     hostname,
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		Version:      version.Version,
		Capabilities: []string{
			"cpu", "memory", "disk", "process",
			"service", "network", "security",
		},
	}
}

// StreamDiagnostics streams diagnostic events in real-time
func (r *GRPCReporter) StreamDiagnostics(ctx context.Context, target string, checks []string, handler func(*lumov1.DiagnosticsEvent) error) error {
	req := &lumov1.StreamDiagnosticsRequest{
		Target: target,
		Checks: checks,
	}

	return r.client.StreamDiagnostics(ctx, req, handler)
}

// GetDiagnosticsResult retrieves a diagnostic job result
func (r *GRPCReporter) GetDiagnosticsResult(ctx context.Context, jobID string) (*lumov1.GetDiagnosticsResultResponse, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return r.client.GetDiagnosticsResult(ctxWithTimeout, jobID)
}

// ListDiagnostics lists diagnostic jobs
func (r *GRPCReporter) ListDiagnostics(ctx context.Context, limit, offset int32) (*lumov1.ListDiagnosticsResponse, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return r.client.ListDiagnostics(ctxWithTimeout, &lumov1.ListDiagnosticsRequest{
		Limit:  limit,
		Offset: offset,
	})
}
