package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/api/middleware"
	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/diagnostics/checkers"
	"github.com/ignacio/lumo/internal/remediation"
	"github.com/ignacio/lumo/internal/ssh"
	"github.com/sirupsen/logrus"
)

// RemediationHandler handles remediation requests
type RemediationHandler struct {
	jobRepo *repository.JobRepository
	config  *config.Config
	logger  *logrus.Logger
}

// NewRemediationHandler creates a new remediation handler
func NewRemediationHandler(jobRepo *repository.JobRepository, cfg *config.Config, logger *logrus.Logger) *RemediationHandler {
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
	job.Status = models.JobStatusRunning
	job.StartedAt = timePtr(time.Now())
	if err := h.jobRepo.Update(ctx, job); err != nil {
		h.logger.WithError(err).Error("Failed to update job status")
		return
	}

	// Execute remediation
	result, err := h.performRemediation(ctx, req)

	// Update job with results
	job.CompletedAt = timePtr(time.Now())
	if err != nil {
		job.Status = models.JobStatusFailed
		job.Error = err.Error()
		h.logger.WithError(err).WithField("job_id", job.ID).Error("Remediation failed")
	} else {
		job.Status = models.JobStatusCompleted
		job.Result = models.JSONB(result)
		h.logger.WithField("job_id", job.ID).Info("Remediation completed successfully")
	}

	if err := h.jobRepo.Update(ctx, job); err != nil {
		h.logger.WithError(err).Error("Failed to update job with results")
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
	if isLocal {
		executor = diagnostics.NewLocalExecutor()
		h.logger.Debug("Using local command executor for remediation")
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
		defer sshClient.Disconnect()

		executor = sshClient
	}

	// Run diagnostics first to identify issues
	h.logger.Info("Running diagnostics before remediation")
	runner := diagnostics.NewRunner(executor, h.logger)

	// Register all checkers
	registerAllCheckers(runner)

	// Run diagnostics
	results := runner.RunAll(ctx)
	if len(results) == 0 {
		return nil, fmt.Errorf("no diagnostic results available")
	}

	// Generate remediation suggestions
	suggester := remediation.NewSuggester(h.logger)
	plan := suggester.GeneratePlan(results)

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

	// Create approver (always auto-approve for API requests or use request setting)
	approver := remediation.NewApprover(req.AutoApprove, h.logger)

	// Create executor
	remediationExecutor := remediation.NewExecutor(executor, auditor, approver, h.logger, req.DryRun, req.AutoApprove)

	// Execute plan
	report, err := remediationExecutor.ExecutePlan(ctx, plan)
	if err != nil {
		return nil, fmt.Errorf("failed to execute remediation plan: %w", err)
	}

	// Format response
	result := map[string]interface{}{
		"start_time":    report.StartTime,
		"end_time":      report.EndTime,
		"duration_ms":   report.Duration.Milliseconds(),
		"total_actions": report.TotalActions,
		"executed":      report.Executed,
		"succeeded":     report.Succeeded,
		"failed":        report.Failed,
		"skipped":       report.Skipped,
		"rejected":      report.Rejected,
		"rolled_back":   report.RolledBack,
		"dry_run":       report.DryRun,
		"results":       report.Results,
	}

	return result, nil
}

// registerAllCheckers registers all available checkers
func registerAllCheckers(runner *diagnostics.Runner) {
	runner.RegisterChecker("cpu", checkers.NewCPUChecker())
	runner.RegisterChecker("memory", checkers.NewMemoryChecker())
	runner.RegisterChecker("disk", checkers.NewDiskChecker())
	runner.RegisterChecker("process", checkers.NewProcessChecker())
	runner.RegisterChecker("service", checkers.NewServiceChecker())
	runner.RegisterChecker("network", checkers.NewNetworkChecker())
	runner.RegisterChecker("patch", checkers.NewPatchChecker())
	runner.RegisterChecker("ports", checkers.NewOpenPortsChecker())
	runner.RegisterChecker("ssh_security", checkers.NewSSHSecurityChecker())
	runner.RegisterChecker("auth_failures", checkers.NewAuthFailuresChecker())
}

// isLocalhost checks if the hostname refers to the local machine
func isLocalhost(hostname string) bool {
	hostname = strings.ToLower(hostname)
	return hostname == "localhost" ||
		hostname == "127.0.0.1" ||
		hostname == "::1" ||
		hostname == "0.0.0.0"
}

// timePtr returns a pointer to the given time
func timePtr(t time.Time) *time.Time {
	return &t
}
