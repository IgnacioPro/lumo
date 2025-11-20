package handlers

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
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
}

// NewDiagnosticsHandler creates a new diagnostics service handler
func NewDiagnosticsHandler(jobRepo JobRepository) *DiagnosticsHandler {
	return &DiagnosticsHandler{
		jobRepo: jobRepo,
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

	// TODO: Execute diagnostics asynchronously in background
	// go h.executeDiagnostics(context.Background(), job, req)

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

	// TODO: Parse and include result if job completed
	// if job.Status == models.JobStatusCompleted && job.Result != nil {
	//     result, err := h.unmarshalDiagnosticsResult(job.Result)
	//     if err != nil {
	//         return nil, status.Errorf(codes.Internal, "failed to parse result: %v", err)
	//     }
	//     resp.Result = result
	// }

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
		return err
	}

	// TODO: Execute diagnostics with progress reporting
	// For now, just send a completion event
	if err := stream.Send(&lumov1.DiagnosticsEvent{
		Type:     lumov1.DiagnosticsEvent_EVENT_TYPE_COMPLETED,
		Progress: 100,
	}); err != nil {
		return err
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
