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
	AutoApprove    bool     `json:"auto_approve,omitempty"` // Auto-approve safe operations
	SkipCategories []string `json:"skip_categories,omitempty"`
	DryRun         bool     `json:"dry_run,omitempty"`
	Username       string   `json:"username,omitempty"`
	Password       string   `json:"password,omitempty"`
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
	go h.executeRemediation(context.Background(), job, &req)

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
			return nil, fmt.Errorf("failed to create SSH client: %w", err)
		}

		// Connect
		port := h.config.SSH.Port
		if port == 0 {
			port = 22
		}
		if err := sshClient.Connect(hostname, port, username); err != nil {
			return nil, fmt.Errorf("failed to connect: %w", err)
		}

		// Use diagnostics.NewSSHExecutor to wrap the SSH client
		executor = diagnostics.NewSSHExecutor(sshClient)
		cleanup = func() { sshClient.Disconnect() }
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
		return nil, fmt.Errorf("failed to run diagnostics: %w", err)
	}
	if len(diagReport.Results) == 0 {
		return nil, fmt.Errorf("no diagnostic results available")
	}

	// Create remediation registry
	registry := remediation.NewRegistry(h.logger)

	// Generate remediation suggestions
	suggester := remediation.NewSuggester(registry, h.logger)
	plan, err := suggester.SuggestFromReport(diagReport)
	if err != nil {
		return nil, fmt.Errorf("failed to generate remediation plan: %w", err)
	}

	// Configure plan based on request
	plan.DryRun = req.DryRun
	plan.AutoApprove = req.AutoApprove

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
	defer auditor.Close()

	// Create approver
	approver := remediation.NewApprover(h.logger)

	// Create executor
	remediationExecutor := remediation.NewExecutor(executor, auditor, approver, h.logger, req.DryRun, req.AutoApprove)

	// Execute plan
	execReport, err := remediationExecutor.ExecutePlan(ctx, plan)
	if err != nil {
		return nil, fmt.Errorf("failed to execute remediation plan: %w", err)
	}

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
