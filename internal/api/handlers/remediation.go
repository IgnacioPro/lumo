package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ignacio/lumo/internal/api/middleware"
	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/diagnostics/checkers"
	"github.com/ignacio/lumo/internal/remediation"
	"github.com/ignacio/lumo/internal/ssh"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// JobRepository defines the interface for job storage operations
type JobRepository interface {
	Create(ctx context.Context, job *models.Job) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.JobStatus) error
	UpdateResult(ctx context.Context, id uuid.UUID, result []byte) error
	UpdateError(ctx context.Context, id uuid.UUID, errorMsg string) error
}

// RemediationHandler handles remediation requests
type RemediationHandler struct {
	jobRepo JobRepository
	config  *config.Config
	logger  *logrus.Logger
}

// NewRemediationHandler creates a new remediation handler
func NewRemediationHandler(jobRepo JobRepository, cfg *config.Config, logger *logrus.Logger) *RemediationHandler {
	return &RemediationHandler{
		jobRepo: jobRepo,
		config:  cfg,
		logger:  logger,
	}
}

// RemediationRequest represents a remediation request
type RemediationRequest struct {
	Target         string   `json:"target"`
	Actions        []string `json:"actions,omitempty"`      // Specific action IDs to execute
	AutoApprove    bool     `json:"auto_approve,omitempty"` // DEPRECATED: Ignored for security reasons (prevents approval bypass)
	SkipCategories []string `json:"skip_categories,omitempty"`
	DryRun         bool     `json:"dry_run,omitempty"`
	Username       string   `json:"username,omitempty"`
	Password       string   `json:"password,omitempty"` // Never stored in database
	KeyPath        string   `json:"key_path,omitempty"`
}

// RemediationResponse represents the immediate response to a remediation request
type RemediationResponse struct {
	JobID     string    `json:"job_id"`
	Status    string    `json:"status"`
	Target    string    `json:"target"`
	CreatedAt time.Time `json:"created_at"`
}

// Run handles POST /api/v1/remediation
func (h *RemediationHandler) Run(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req RemediationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	// Validate request
	if req.Target == "" {
		response.BadRequest(w, "Target is required")
		return
	}

	// Validate SSH target to prevent SSRF attacks (unless it's localhost)
	if !isLocalhost(req.Target) {
		if err := ssh.ValidateSSHTarget(req.Target); err != nil {
			response.BadRequest(w, fmt.Sprintf("Invalid target: %v", err))
			return
		}
	}

	// Get authenticated API key from context
	apiKey, ok := middleware.GetAPIKeyFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "Authentication required")
		return
	}

	// Create job record
	job := &models.Job{
		Type:      models.JobTypeRemediation,
		Status:    models.JobStatusPending,
		Target:    req.Target,
		CreatedBy: apiKey.Name,
		Metadata: models.JSONB{
			"actions":         req.Actions,
			"auto_approve":    req.AutoApprove,
			"skip_categories": req.SkipCategories,
			"dry_run":         req.DryRun,
			"username":        req.Username,
		},
	}

	// Save job to database
	if err := h.jobRepo.Create(r.Context(), job); err != nil {
		h.logger.WithError(err).Error("Failed to create job")
		response.InternalServerError(w, "Failed to create remediation job")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"job_id": job.ID,
		"target": req.Target,
		"type":   job.Type,
	}).Info("Remediation job created")

	// Execute remediation asynchronously
	// Use a long timeout (1 hour) to allow for complex remediation tasks
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	go func() {
		defer cancel()
		h.executeRemediation(ctx, job, &req)
	}()

	// Return immediate response
	resp := RemediationResponse{
		JobID:     job.ID.String(),
		Status:    string(job.Status),
		Target:    job.Target,
		CreatedAt: job.CreatedAt,
	}

	response.Created(w, resp)
}

// executeRemediation runs the remediation in the background
func (h *RemediationHandler) executeRemediation(ctx context.Context, job *models.Job, req *RemediationRequest) {
	// Update job status to running
	if err := h.jobRepo.UpdateStatus(ctx, job.ID, models.JobStatusRunning); err != nil {
		h.logger.WithError(err).Error("Failed to update job status")
		return
	}

	// Execute remediation
	result, err := h.performRemediation(ctx, req)

	// Update job with results
	if err != nil {
		h.logger.WithError(err).WithField("job_id", job.ID).Error("Remediation failed")
		if updateErr := h.jobRepo.UpdateStatus(ctx, job.ID, models.JobStatusFailed); updateErr != nil {
			h.logger.WithError(updateErr).Error("Failed to update job status to failed")
		}
		if updateErr := h.jobRepo.UpdateError(ctx, job.ID, err.Error()); updateErr != nil {
			h.logger.WithError(updateErr).Error("Failed to update job error")
		}
	} else {
		h.logger.WithField("job_id", job.ID).Info("Remediation completed successfully")
		if updateErr := h.jobRepo.UpdateStatus(ctx, job.ID, models.JobStatusCompleted); updateErr != nil {
			h.logger.WithError(updateErr).Error("Failed to update job status to completed")
		}
		// Marshal result to JSON
		resultJSON, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			h.logger.WithError(marshalErr).Error("Failed to marshal remediation result")
		} else {
			if updateErr := h.jobRepo.UpdateResult(ctx, job.ID, resultJSON); updateErr != nil {
				h.logger.WithError(updateErr).Error("Failed to update job result")
			}
		}
	}
}

// performRemediation executes the actual remediation logic
func (h *RemediationHandler) performRemediation(ctx context.Context, req *RemediationRequest) (map[string]interface{}, error) {
	// Create tracing span
	tracer := otel.Tracer("lumo.api.handlers")
	ctx, span := tracer.Start(ctx, "performRemediation")
	defer span.End()

	// Set span attributes
	span.SetAttributes(
		attribute.String("remediation.target", req.Target),
		attribute.Bool("remediation.dry_run", req.DryRun),
		attribute.Int("remediation.actions_count", len(req.Actions)),
		attribute.Int("remediation.skip_categories_count", len(req.SkipCategories)),
	)

	// Parse host argument
	var username, hostname string
	if strings.Contains(req.Target, "@") {
		parts := strings.SplitN(req.Target, "@", 2)
		username = parts[0]
		hostname = parts[1]
	} else {
		hostname = req.Target
		username = req.Username
		if username == "" {
			username = "root" // Default username for API requests
		}
	}

	// Check if localhost
	isLocal := isLocalhost(hostname)
	span.SetAttributes(attribute.Bool("remediation.is_localhost", isLocal))

	// Create command executor
	var executor diagnostics.CommandExecutor
	var cleanup func()
	if isLocal {
		executor = diagnostics.NewLocalExecutor()
		h.logger.Debug("Using local command executor for remediation")
		cleanup = func() {} // No cleanup needed for local executor
	} else {
		// Create SSH client
		sshClientConfig := ssh.NewClientConfig(h.config.SSH)
		if req.KeyPath != "" {
			if err := sshClientConfig.SetKeyPath(req.KeyPath); err != nil {
				return nil, fmt.Errorf("invalid key path: %w", err)
			}
		}

		sshClient, err := ssh.NewClient(sshClientConfig, h.logger)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to create SSH client")
			return nil, fmt.Errorf("failed to create SSH client: %w", err)
		}

		// Connect
		port := h.config.SSH.Port
		if port == 0 {
			port = 22
		}
		if err := sshClient.Connect(hostname, port, username); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to connect via SSH")
			return nil, fmt.Errorf("failed to connect: %w", err)
		}

		// Use diagnostics.NewSSHExecutor to wrap the SSH client
		executor = diagnostics.NewSSHExecutor(sshClient)
		cleanup = func() { _ = sshClient.Disconnect() }
	}
	defer cleanup()

	// Run diagnostics first to identify issues
	h.logger.Info("Running diagnostics before remediation")

	// Create diagnostics config with defaults
	diagConfig := diagnostics.DefaultConfig()
	diagConfig.EnabledChecks = []string{"cpu", "memory", "disk", "process", "service", "network"}

	// Create default thresholds
	thresholds := diagnostics.DefaultThresholds()

	// Create runner
	runner := diagnostics.NewRunner(diagConfig, thresholds, executor, h.logger)

	// Register all checkers
	registerAllCheckers(runner, thresholds)

	// Run diagnostics
	diagReport, err := runner.RunAll(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to run diagnostics")
		return nil, fmt.Errorf("failed to run diagnostics: %w", err)
	}
	if len(diagReport.Results) == 0 {
		err := fmt.Errorf("no diagnostic results available")
		span.RecordError(err)
		span.SetStatus(codes.Error, "No diagnostic results available")
		return nil, err
	}

	// Create remediation registry
	registry := remediation.NewRegistry(h.logger)

	// Generate remediation suggestions
	suggester := remediation.NewSuggester(registry, h.logger)
	plan, err := suggester.SuggestFromReport(diagReport)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to generate remediation plan")
		return nil, fmt.Errorf("failed to generate remediation plan: %w", err)
	}

	// Configure plan based on request
	plan.DryRun = req.DryRun
	// SECURITY: Never allow user-controlled auto_approve to prevent bypassing human-in-the-loop approval
	// Auto-approval should only be configured server-side based on action risk levels and organizational policy
	// plan.AutoApprove = req.AutoApprove  // REMOVED - security vulnerability

	// Parse skip categories
	for _, cat := range req.SkipCategories {
		switch strings.ToLower(cat) {
		case "service":
			plan.SkipCategories = append(plan.SkipCategories, remediation.CategoryService)
		case "disk":
			plan.SkipCategories = append(plan.SkipCategories, remediation.CategoryDisk)
		case "process":
			plan.SkipCategories = append(plan.SkipCategories, remediation.CategoryProcess)
		case "network":
			plan.SkipCategories = append(plan.SkipCategories, remediation.CategoryNetwork)
		case "system":
			plan.SkipCategories = append(plan.SkipCategories, remediation.CategorySystem)
		case "security":
			plan.SkipCategories = append(plan.SkipCategories, remediation.CategorySecurity)
		}
	}

	// Create auditor
	auditor, err := remediation.NewAuditor("/tmp/remediation-audit.log", h.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create auditor: %w", err)
	}
	defer func() { _ = auditor.Close() }()

	// Create approver
	approver := remediation.NewApprover(h.logger)

	// Create executor
	remediationExecutor := remediation.NewExecutor(executor, auditor, approver, h.logger, req.DryRun, req.AutoApprove)

	// Execute plan
	execReport, err := remediationExecutor.ExecutePlan(ctx, plan)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to execute remediation plan")
		return nil, fmt.Errorf("failed to execute remediation plan: %w", err)
	}

	// Set span attributes with execution results
	span.SetAttributes(
		attribute.Int("remediation.total_actions", execReport.TotalActions),
		attribute.Int("remediation.executed", execReport.Executed),
		attribute.Int("remediation.succeeded", execReport.Succeeded),
		attribute.Int("remediation.failed", execReport.Failed),
		attribute.Int("remediation.skipped", execReport.Skipped),
		attribute.Int("remediation.rejected", execReport.Rejected),
		attribute.Int("remediation.rolled_back", execReport.RolledBack),
		attribute.String("remediation.duration", execReport.Duration.String()),
	)
	span.SetStatus(codes.Ok, "Remediation completed successfully")

	// Format response
	result := map[string]interface{}{
		"start_time":    execReport.StartTime,
		"end_time":      execReport.EndTime,
		"duration_ms":   execReport.Duration.Milliseconds(),
		"total_actions": execReport.TotalActions,
		"executed":      execReport.Executed,
		"succeeded":     execReport.Succeeded,
		"failed":        execReport.Failed,
		"skipped":       execReport.Skipped,
		"rejected":      execReport.Rejected,
		"rolled_back":   execReport.RolledBack,
		"dry_run":       execReport.DryRun,
		"results":       execReport.Results,
	}

	return result, nil
}

// registerAllCheckers registers all available checkers
func registerAllCheckers(runner *diagnostics.Runner, thresholds *diagnostics.ThresholdConfig) {
	runner.RegisterChecker(checkers.NewCPUChecker(thresholds.CPU))
	runner.RegisterChecker(checkers.NewMemoryChecker(thresholds.Memory))
	runner.RegisterChecker(checkers.NewDiskChecker(thresholds.Disk))
	runner.RegisterChecker(checkers.NewProcessChecker(thresholds.Process))
	runner.RegisterChecker(checkers.NewServiceChecker([]string{}))
	runner.RegisterChecker(checkers.NewNetworkChecker(thresholds.Network, nil))
	runner.RegisterChecker(checkers.NewPatchChecker())
	runner.RegisterChecker(checkers.NewPortsChecker([]int{}))
	runner.RegisterChecker(checkers.NewSSHSecurityChecker())
	runner.RegisterChecker(checkers.NewAuthFailuresChecker(24, 20))
}

// isLocalhost checks if the hostname refers to the local machine
func isLocalhost(hostname string) bool {
	hostname = strings.ToLower(hostname)
	return hostname == "localhost" ||
		hostname == "127.0.0.1" ||
		hostname == "::1" ||
		hostname == "0.0.0.0"
}
