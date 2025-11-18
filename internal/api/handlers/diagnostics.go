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
	"github.com/ignacio/lumo/internal/ssh"
	"github.com/sirupsen/logrus"
)

// DiagnosticsHandler handles diagnostic requests
type DiagnosticsHandler struct {
	jobRepo *repository.JobRepository
	config  *config.Config
	logger  *logrus.Logger
}

// NewDiagnosticsHandler creates a new diagnostics handler
func NewDiagnosticsHandler(jobRepo *repository.JobRepository, cfg *config.Config, logger *logrus.Logger) *DiagnosticsHandler {
	return &DiagnosticsHandler{
		jobRepo: jobRepo,
		config:  cfg,
		logger:  logger,
	}
}

// DiagnosticRequest represents a diagnostic request
type DiagnosticRequest struct {
	Target   string   `json:"target"`
	Checks   []string `json:"checks,omitempty"`
	Format   string   `json:"format,omitempty"`
	Analyze  bool     `json:"analyze,omitempty"`
	DryRun   bool     `json:"dry_run,omitempty"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	KeyPath  string   `json:"key_path,omitempty"`
}

// DiagnosticResponse represents the immediate response to a diagnostic request
type DiagnosticResponse struct {
	JobID     string    `json:"job_id"`
	Status    string    `json:"status"`
	Target    string    `json:"target"`
	CreatedAt time.Time `json:"created_at"`
}

// Run handles POST /api/v1/diagnostics
func (h *DiagnosticsHandler) Run(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req DiagnosticRequest
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
		Type:      models.JobTypeDiagnostic,
		Status:    models.JobStatusPending,
		Target:    req.Target,
		CreatedBy: apiKey.Name,
		Metadata: models.JSONB{
			"checks":   req.Checks,
			"format":   req.Format,
			"analyze":  req.Analyze,
			"dry_run":  req.DryRun,
			"username": req.Username,
		},
	}

	// Save job to database
	if err := h.jobRepo.Create(r.Context(), job); err != nil {
		h.logger.WithError(err).Error("Failed to create job")
		response.InternalServerError(w, "Failed to create diagnostic job")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"job_id": job.ID,
		"target": req.Target,
	}).Info("Diagnostic job created")

	// Execute diagnostics asynchronously
	go h.executeDiagnostics(context.Background(), job, req)

	// Return immediate response
	resp := DiagnosticResponse{
		JobID:     job.ID.String(),
		Status:    string(job.Status),
		Target:    job.Target,
		CreatedAt: job.CreatedAt,
	}

	response.Created(w, resp)
}

// executeDiagnostics runs the diagnostic checks asynchronously
func (h *DiagnosticsHandler) executeDiagnostics(ctx context.Context, job *models.Job, req DiagnosticRequest) {
	// Update job status to running
	if err := h.jobRepo.UpdateStatus(ctx, job.ID, models.JobStatusRunning); err != nil {
		h.logger.WithError(err).Error("Failed to update job status to running")
		return
	}

	h.logger.WithField("job_id", job.ID).Info("Starting diagnostic execution")

	// Determine if target is localhost
	isLocalhost := h.isLocalhost(req.Target)

	var executor diagnostics.CommandExecutor

	if isLocalhost {
		// Use local executor
		executor = diagnostics.NewLocalExecutor()
		h.logger.WithField("job_id", job.ID).Info("Using local executor")
	} else {
		// Create SSH client configuration
		sshClientConfig := ssh.NewClientConfig(h.config.SSH)

		// Override with request parameters
		if req.KeyPath != "" {
			if err := sshClientConfig.SetKeyPath(req.KeyPath); err != nil {
				h.logger.WithError(err).Error("Invalid key path")
				h.jobRepo.UpdateError(ctx, job.ID, fmt.Sprintf("Invalid key path: %v", err))
				return
			}
		}

		// Set password if provided
		if req.Password != "" {
			sshClientConfig.Password = req.Password
		}

		// Create SSH client
		sshClient, err := ssh.NewClient(sshClientConfig, h.logger)
		if err != nil {
			h.logger.WithError(err).Error("Failed to create SSH client")
			h.jobRepo.UpdateError(ctx, job.ID, fmt.Sprintf("Failed to create SSH client: %v", err))
			return
		}

		// Connect to remote host
		port := h.config.SSH.Port
		username := req.Username
		if username == "" {
			username = "root" // Default username
		}

		if err := sshClient.Connect(req.Target, port, username); err != nil {
			h.logger.WithError(err).Error("Failed to connect via SSH")
			h.jobRepo.UpdateError(ctx, job.ID, fmt.Sprintf("Failed to connect: %v", err))
			return
		}
		defer func() {
			_ = sshClient.Disconnect()
		}()

		executor = diagnostics.NewSSHExecutor(sshClient)
		h.logger.WithField("job_id", job.ID).Info("Using SSH executor")
	}

	// Create diagnostic runner
	diagConfig := diagnostics.DefaultConfig()
	if len(req.Checks) > 0 {
		diagConfig.EnabledChecks = req.Checks
	}

	runner := diagnostics.NewRunner(diagConfig, nil, executor, h.logger)

	// Register all checkers
	h.registerCheckers(runner)

	// Run diagnostics
	report, err := runner.RunAll(ctx)
	if err != nil {
		h.logger.WithError(err).Error("Diagnostic execution failed")
		h.jobRepo.UpdateError(ctx, job.ID, fmt.Sprintf("Execution failed: %v", err))
		return
	}

	// Serialize result
	resultJSON, err := json.Marshal(report)
	if err != nil {
		h.logger.WithError(err).Error("Failed to marshal diagnostic result")
		h.jobRepo.UpdateError(ctx, job.ID, fmt.Sprintf("Failed to serialize result: %v", err))
		return
	}

	// Update job with result
	if err := h.jobRepo.UpdateResult(ctx, job.ID, resultJSON); err != nil {
		h.logger.WithError(err).Error("Failed to update job result")
		return
	}

	// Update status to completed
	if err := h.jobRepo.UpdateStatus(ctx, job.ID, models.JobStatusCompleted); err != nil {
		h.logger.WithError(err).Error("Failed to update job status to completed")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"job_id":       job.ID,
		"total_checks": report.Summary.TotalChecks,
		"duration":     report.Duration,
	}).Info("Diagnostic execution completed")
}

// registerCheckers registers all available diagnostic checkers
func (h *DiagnosticsHandler) registerCheckers(runner *diagnostics.Runner) {
	// Note: In a real implementation, you would import and register all checkers
	// For now, this is a placeholder that shows the pattern
	// Example:
	// runner.RegisterChecker(checkers.NewCPUChecker())
	// runner.RegisterChecker(checkers.NewMemoryChecker())
	// etc.

	h.logger.Debug("Checkers registered")
}

// isLocalhost checks if the given hostname refers to the local machine
func (h *DiagnosticsHandler) isLocalhost(hostname string) bool {
	// Normalize hostname to lowercase for comparison
	hostname = strings.ToLower(strings.TrimSpace(hostname))

	// Common localhost patterns
	localhostPatterns := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"0.0.0.0",
		"::",
	}

	for _, pattern := range localhostPatterns {
		if hostname == pattern {
			return true
		}
	}

	return false
}
