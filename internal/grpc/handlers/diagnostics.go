package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/diagnostics/checkers"
)

// JobRepository defines the interface for job data operations
type JobRepository interface {
	Create(ctx context.Context, job *models.Job) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Job, error)
	List(ctx context.Context, opts repository.ListOptions) ([]*models.Job, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.JobStatus) error
	UpdateResult(ctx context.Context, id uuid.UUID, result []byte) error
	UpdateError(ctx context.Context, id uuid.UUID, errorMsg string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// DiagnosticsHandler implements the DiagnosticsService gRPC service
type DiagnosticsHandler struct {
	lumov1.UnimplementedDiagnosticsServiceServer

	jobRepo JobRepository
	cfg     *config.Config
	logger  *logrus.Logger
}

// NewDiagnosticsHandler creates a new diagnostics service handler
func NewDiagnosticsHandler(jobRepo JobRepository, cfg *config.Config, logger *logrus.Logger) *DiagnosticsHandler {
	return &DiagnosticsHandler{
		jobRepo: jobRepo,
		cfg:     cfg,
		logger:  logger,
	}
}

// RunDiagnostics initiates a new diagnostic run
func (h *DiagnosticsHandler) RunDiagnostics(ctx context.Context, req *lumov1.RunDiagnosticsRequest) (*lumov1.RunDiagnosticsResponse, error) {
	// Validate request
	if req.Target == "" {
		return nil, status.Error(codes.InvalidArgument, "target is required")
	}

	// Create job in database
	job := &models.Job{
		ID:     uuid.New(),
		Type:   models.JobTypeDiagnostic,
		Status: models.JobStatusPending,
		Target: req.Target,
		Metadata: map[string]interface{}{
			"checks":      req.Checks,
			"analyze":     req.Analyze,
			"format":      req.Format.String(),
			"focus_areas": req.FocusAreas,
		},
	}

	if err := h.jobRepo.Create(ctx, job); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create job: %v", err)
	}

	// Execute diagnostics asynchronously in background
	// Use a 30-minute timeout to match HTTP handler
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	go func() {
		defer cancel()
		h.executeDiagnostics(ctx, job, req)
	}()

	return &lumov1.RunDiagnosticsResponse{
		JobId:     job.ID.String(),
		Status:    toProtoJobStatus(job.Status),
		CreatedAt: timestamppb.New(job.CreatedAt),
	}, nil
}

// GetDiagnosticsResult retrieves the result of a diagnostic job
func (h *DiagnosticsHandler) GetDiagnosticsResult(ctx context.Context, req *lumov1.GetDiagnosticsResultRequest) (*lumov1.GetDiagnosticsResultResponse, error) {
	jobID, err := uuid.Parse(req.JobId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid job ID")
	}

	job, err := h.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job not found: %v", err)
	}

	resp := &lumov1.GetDiagnosticsResultResponse{
		JobId:     job.ID.String(),
		Status:    toProtoJobStatus(job.Status),
		Type:      toProtoJobType(job.Type),
		Target:    job.Target,
		CreatedAt: timestamppb.New(job.CreatedAt),
	}

	if job.StartedAt != nil {
		resp.StartedAt = timestamppb.New(*job.StartedAt)
	}
	if job.CompletedAt != nil {
		resp.CompletedAt = timestamppb.New(*job.CompletedAt)
	}

	// Parse and include result if job completed
	if job.Status == models.JobStatusCompleted && job.Result != nil {
		result, err := h.unmarshalDiagnosticsResult(job.Result)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse result: %v", err)
		}
		resp.Result = result
	}

	if job.Error != nil && *job.Error != "" {
		resp.Error = *job.Error
	}

	return resp, nil
}

// StreamDiagnostics streams diagnostic progress in real-time
func (h *DiagnosticsHandler) StreamDiagnostics(req *lumov1.StreamDiagnosticsRequest, stream lumov1.DiagnosticsService_StreamDiagnosticsServer) error {
	// Send initial event
	if err := stream.Send(&lumov1.DiagnosticsEvent{
		Type:     lumov1.DiagnosticsEvent_EVENT_TYPE_STARTED,
		JobId:    uuid.New().String(),
		Progress: 0,
	}); err != nil {
		return fmt.Errorf("failed to send start event: %w", err)
	}

	// Send completion event
	if err := stream.Send(&lumov1.DiagnosticsEvent{
		Type:     lumov1.DiagnosticsEvent_EVENT_TYPE_COMPLETED,
		Progress: 100,
	}); err != nil {
		return fmt.Errorf("failed to send completion event: %w", err)
	}

	return nil
}

// ListDiagnostics lists diagnostic jobs with optional filtering
func (h *DiagnosticsHandler) ListDiagnostics(ctx context.Context, req *lumov1.ListDiagnosticsRequest) (*lumov1.ListDiagnosticsResponse, error) {
	// Set defaults
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}
	offset := int(req.Offset)

	// Build list options
	opts := repository.ListOptions{
		Limit:  limit,
		Offset: offset,
	}

	// Add optional filters
	if req.Status != lumov1.JobStatus_JOB_STATUS_UNSPECIFIED {
		jobStatus := toModelJobStatusFromProto(req.Status)
		opts.Status = &jobStatus
	}

	if req.Target != "" {
		opts.Target = req.Target
	}

	// List jobs from database
	jobs, totalCount, err := h.jobRepo.List(ctx, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list jobs: %v", err)
	}

	// Convert to proto
	protoJobs := make([]*lumov1.JobSummary, len(jobs))
	for i, job := range jobs {
		protoJobs[i] = &lumov1.JobSummary{
			JobId:     job.ID.String(),
			Type:      toProtoJobType(job.Type),
			Status:    toProtoJobStatus(job.Status),
			Target:    job.Target,
			CreatedAt: timestamppb.New(job.CreatedAt),
		}
		if job.CompletedAt != nil {
			protoJobs[i].CompletedAt = timestamppb.New(*job.CompletedAt)
		}
	}

	return &lumov1.ListDiagnosticsResponse{
		Jobs:       protoJobs,
		TotalCount: int32(totalCount),
	}, nil
}

// Helper functions

func toProtoJobStatus(status models.JobStatus) lumov1.JobStatus {
	switch status {
	case models.JobStatusPending:
		return lumov1.JobStatus_JOB_STATUS_PENDING
	case models.JobStatusRunning:
		return lumov1.JobStatus_JOB_STATUS_RUNNING
	case models.JobStatusCompleted:
		return lumov1.JobStatus_JOB_STATUS_COMPLETED
	case models.JobStatusFailed:
		return lumov1.JobStatus_JOB_STATUS_FAILED
	case models.JobStatusCancelled:
		return lumov1.JobStatus_JOB_STATUS_CANCELLED
	default:
		return lumov1.JobStatus_JOB_STATUS_UNSPECIFIED
	}
}

func toProtoJobType(jobType models.JobType) lumov1.JobType {
	switch jobType {
	case models.JobTypeDiagnostic:
		return lumov1.JobType_JOB_TYPE_DIAGNOSTIC
	case models.JobTypeRemediation:
		return lumov1.JobType_JOB_TYPE_REMEDIATION
	default:
		return lumov1.JobType_JOB_TYPE_UNSPECIFIED
	}
}

func toModelJobStatusFromProto(status lumov1.JobStatus) models.JobStatus {
	switch status {
	case lumov1.JobStatus_JOB_STATUS_PENDING:
		return models.JobStatusPending
	case lumov1.JobStatus_JOB_STATUS_RUNNING:
		return models.JobStatusRunning
	case lumov1.JobStatus_JOB_STATUS_COMPLETED:
		return models.JobStatusCompleted
	case lumov1.JobStatus_JOB_STATUS_FAILED:
		return models.JobStatusFailed
	case lumov1.JobStatus_JOB_STATUS_CANCELLED:
		return models.JobStatusCancelled
	default:
		return models.JobStatusPending
	}
}

func (h *DiagnosticsHandler) executeDiagnostics(ctx context.Context, job *models.Job, req *lumov1.RunDiagnosticsRequest) {
	h.logger.WithFields(logrus.Fields{
		"job_id": job.ID,
		"target": req.Target,
		"checks": req.Checks,
	}).Info("Starting async diagnostics execution")

	// Update status to running
	if err := h.jobRepo.UpdateStatus(ctx, job.ID, models.JobStatusRunning); err != nil {
		h.logger.WithError(err).Error("Failed to update job status to running")
		return
	}

	// Execute diagnostics and handle result
	if err := h.runDiagnosticsExecution(ctx, job, req); err != nil {
		h.logger.WithError(err).Error("Diagnostics execution failed")
		if updateErr := h.jobRepo.UpdateError(ctx, job.ID, err.Error()); updateErr != nil {
			h.logger.WithError(updateErr).Error("Failed to update job error")
		}
		if statusErr := h.jobRepo.UpdateStatus(ctx, job.ID, models.JobStatusFailed); statusErr != nil {
			h.logger.WithError(statusErr).Error("Failed to update job status to failed")
		}
		return
	}

	// Mark as completed
	if err := h.jobRepo.UpdateStatus(ctx, job.ID, models.JobStatusCompleted); err != nil {
		h.logger.WithError(err).Error("Failed to update job status to completed")
	}

	h.logger.WithField("job_id", job.ID).Info("Async diagnostics execution completed")
}

func (h *DiagnosticsHandler) runDiagnosticsExecution(ctx context.Context, job *models.Job, req *lumov1.RunDiagnosticsRequest) error {
	// Create command executor (always use local for now - SSH support can be added later)
	executor := diagnostics.NewLocalExecutor()

	// Create diagnostic runner
	diagConfig := diagnostics.DefaultConfig()
	if len(req.Checks) > 0 {
		diagConfig.EnabledChecks = req.Checks
	}

	thresholds := diagnostics.DefaultThresholds()
	runner := diagnostics.NewRunner(diagConfig, thresholds, executor, h.logger)

	// Register checkers
	checkersToRegister := []diagnostics.Checker{
		// Core system checkers
		checkers.NewCPUChecker(thresholds.CPU),
		checkers.NewMemoryChecker(thresholds.Memory),
		checkers.NewDiskChecker(thresholds.Disk),
		checkers.NewProcessChecker(thresholds.Process),
		checkers.NewServiceChecker([]string{}),
		checkers.NewNetworkChecker(thresholds.Network, h.cfg.Diagnostics.Network.Targets),
		// Security checkers
		checkers.NewPatchChecker(),
		checkers.NewPortsChecker(h.cfg.Diagnostics.Security.PortCheck.WhitelistedPorts),
		checkers.NewSSHSecurityChecker(),
		checkers.NewAuthFailuresChecker(
			h.cfg.Diagnostics.Security.AuthFailureCheck.LookbackHours,
			h.cfg.Diagnostics.Security.AuthFailureCheck.FailureThreshold,
		),
		// Proxmox checker (auto-skips if not installed)
		checkers.NewProxmoxChecker(false, false, false, false, false, false, false),
	}

	// Add Kubernetes checker if enabled
	if h.cfg.Diagnostics.Kubernetes.Enabled {
		checkersToRegister = append(checkersToRegister, checkers.NewKubernetesChecker(h.cfg.Diagnostics.Kubernetes, h.logger))
	}

	runner.RegisterCheckers(checkersToRegister...)

	// Run diagnostics with timeout
	execCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	report, err := runner.RunAll(execCtx)
	if err != nil {
		return fmt.Errorf("failed to run diagnostics: %w", err)
	}

	// Convert report to JSON and store in job result
	resultJSON, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := h.jobRepo.UpdateResult(ctx, job.ID, resultJSON); err != nil {
		return fmt.Errorf("failed to update job result: %w", err)
	}

	return nil
}

func (h *DiagnosticsHandler) unmarshalDiagnosticsResult(resultJSON []byte) (*lumov1.DiagnosticsResult, error) {
	// Unmarshal JSON into diagnostics.Report
	var report diagnostics.Report
	if err := json.Unmarshal(resultJSON, &report); err != nil {
		return nil, fmt.Errorf("failed to unmarshal report: %w", err)
	}

	// Convert to proto message
	protoResult := &lumov1.DiagnosticsResult{
		Timestamp:  timestamppb.New(report.Timestamp),
		DurationMs: report.Duration.Milliseconds(),
		Summary: &lumov1.ReportSummary{
			TotalChecks: int32(report.Summary.TotalChecks),
			Passed:      int32(report.Summary.OKCount),
			Failed:      int32(report.Summary.CriticalCount + report.Summary.ErrorCount),
			Warnings:    int32(report.Summary.WarningCount),
			Skipped:     0, // Not tracked in diagnostics.ReportSummary
		},
		Results: make([]*lumov1.CheckResult, len(report.Results)),
	}

	// Convert check results
	for i, result := range report.Results {
		protoCheckResult := &lumov1.CheckResult{
			Name:       result.Name,
			Category:   string(result.Category),
			Status:     toProtoCheckStatus(result.Status),
			Severity:   toProtoSeverity(result.Severity),
			Message:    result.Message,
			Timestamp:  timestamppb.New(result.Timestamp),
			DurationMs: result.Duration.Milliseconds(),
		}

		if result.Error != "" {
			protoCheckResult.Error = result.Error
		}

		// Convert metrics
		protoCheckResult.Metrics = make([]*lumov1.Metric, len(result.Metrics))
		for j, metric := range result.Metrics {
			protoCheckResult.Metrics[j] = &lumov1.Metric{
				Name:  metric.Name,
				Value: metric.Value,
				Unit:  metric.Unit,
			}
		}

		protoResult.Results[i] = protoCheckResult
	}

	return protoResult, nil
}

func toProtoCheckStatus(status diagnostics.CheckStatus) lumov1.CheckStatus {
	switch status {
	case diagnostics.StatusCompleted:
		return lumov1.CheckStatus_CHECK_STATUS_PASS
	case diagnostics.StatusFailed:
		return lumov1.CheckStatus_CHECK_STATUS_FAIL
	case diagnostics.StatusSkipped:
		return lumov1.CheckStatus_CHECK_STATUS_SKIP
	case diagnostics.StatusTimeout:
		return lumov1.CheckStatus_CHECK_STATUS_FAIL // Timeout is treated as a failure
	default:
		return lumov1.CheckStatus_CHECK_STATUS_UNSPECIFIED
	}
}

func toProtoSeverity(severity diagnostics.Severity) lumov1.Severity {
	switch severity {
	case diagnostics.SeverityOK:
		return lumov1.Severity_SEVERITY_INFO // OK maps to INFO
	case diagnostics.SeverityInfo:
		return lumov1.Severity_SEVERITY_INFO
	case diagnostics.SeverityWarning:
		return lumov1.Severity_SEVERITY_MEDIUM // Warning maps to MEDIUM
	case diagnostics.SeverityCritical:
		return lumov1.Severity_SEVERITY_CRITICAL
	default:
		return lumov1.Severity_SEVERITY_UNSPECIFIED
	}
}
