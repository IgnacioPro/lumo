package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
	"github.com/sirupsen/logrus"
)

// JobsHandler handles job-related requests
type JobsHandler struct {
	jobRepo *repository.JobRepository
	logger  *logrus.Logger
}

// NewJobsHandler creates a new jobs handler
func NewJobsHandler(jobRepo *repository.JobRepository, logger *logrus.Logger) *JobsHandler {
	return &JobsHandler{
		jobRepo: jobRepo,
		logger:  logger,
	}
}

// JobListResponse represents a paginated list of jobs
type JobListResponse struct {
	Jobs   []*models.Job `json:"jobs"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

// List handles GET /api/v1/jobs
func (h *JobsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	opts := repository.ListOptions{
		Limit:     50,
		Offset:    0,
		SortBy:    "created_at",
		SortOrder: "DESC",
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			opts.Limit = limit
		}
	}

	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}

	// Parse status filter
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status := models.JobStatus(statusStr)
		opts.Status = &status
	}

	// Parse type filter
	if typeStr := r.URL.Query().Get("type"); typeStr != "" {
		jobType := models.JobType(typeStr)
		opts.Type = &jobType
	}

	// Parse target filter
	if target := r.URL.Query().Get("target"); target != "" {
		opts.Target = target
	}

	// Parse sort parameters
	if sortBy := r.URL.Query().Get("sort_by"); sortBy != "" {
		opts.SortBy = sortBy
	}
	if sortOrder := r.URL.Query().Get("sort_order"); sortOrder != "" {
		opts.SortOrder = sortOrder
	}

	// Fetch jobs from database
	jobs, total, err := h.jobRepo.List(r.Context(), opts)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list jobs")
		response.InternalServerError(w, "Failed to retrieve jobs")
		return
	}

	// Build response
	resp := JobListResponse{
		Jobs:   jobs,
		Total:  total,
		Limit:  opts.Limit,
		Offset: opts.Offset,
	}

	response.Success(w, resp)
}

// Get handles GET /api/v1/jobs/:id
func (h *JobsHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from URL
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		response.BadRequest(w, "Job ID is required")
		return
	}

	// Parse UUID
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Invalid job ID format")
		return
	}

	// Fetch job from database
	job, err := h.jobRepo.GetByID(r.Context(), id)
	if err != nil {
		h.logger.WithError(err).WithField("job_id", id).Warn("Job not found")
		response.NotFound(w, "Job not found")
		return
	}

	response.Success(w, job)
}

// Delete handles DELETE /api/v1/jobs/:id
func (h *JobsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from URL
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		response.BadRequest(w, "Job ID is required")
		return
	}

	// Parse UUID
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Invalid job ID format")
		return
	}

	// Delete job from database
	if err := h.jobRepo.Delete(r.Context(), id); err != nil {
		h.logger.WithError(err).WithField("job_id", id).Error("Failed to delete job")
		response.InternalServerError(w, "Failed to delete job")
		return
	}

	h.logger.WithField("job_id", id).Info("Job deleted")
	response.NoContent(w)
}
