package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/api/middleware"
	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
)

// ApprovalsHandler handles approval-related requests
type ApprovalsHandler struct {
	approvalRepo *repository.ApprovalRepository
	logger       *logrus.Logger
}

// NewApprovalsHandler creates a new approvals handler
func NewApprovalsHandler(approvalRepo *repository.ApprovalRepository, logger *logrus.Logger) *ApprovalsHandler {
	return &ApprovalsHandler{
		approvalRepo: approvalRepo,
		logger:       logger,
	}
}

// List handles GET /api/v1/approvals
func (h *ApprovalsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	filters := make(map[string]interface{})

	// Status filter
	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = models.ApprovalStatus(status)
	} else {
		// Default to pending approvals only
		filters["status"] = models.ApprovalStatusPending
		filters["exclude_expired"] = true
	}

	// Risk level filter
	if riskLevel := r.URL.Query().Get("risk_level"); riskLevel != "" {
		filters["risk_level"] = models.ApprovalRiskLevel(riskLevel)
	}

	// Target filter
	if target := r.URL.Query().Get("target"); target != "" {
		filters["target"] = target
	}

	// Requested by filter
	if requestedBy := r.URL.Query().Get("requested_by"); requestedBy != "" {
		filters["requested_by"] = requestedBy
	}

	// Sort parameter
	if sort := r.URL.Query().Get("sort"); sort != "" {
		filters["sort"] = sort
	}

	// Pagination
	limit, offset := parsePagination(r)
	if limit > 0 {
		filters["limit"] = limit
	}
	if offset > 0 {
		filters["offset"] = offset
	}

	// Get approvals
	approvals, err := h.approvalRepo.List(r.Context(), filters)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list approvals")
		response.InternalServerError(w, "Failed to list approvals")
		return
	}

	response.Success(w, map[string]interface{}{
		"approvals": approvals,
		"count":     len(approvals),
	})
}

// Get handles GET /api/v1/approvals/:id
func (h *ApprovalsHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Get approval ID from URL
	approvalIDStr := chi.URLParam(r, "id")
	approvalID, err := uuid.Parse(approvalIDStr)
	if err != nil {
		response.BadRequest(w, "Invalid approval ID")
		return
	}

	// Get approval
	approval, err := h.approvalRepo.GetByID(r.Context(), approvalID)
	if err != nil {
		h.logger.WithError(err).WithField("approval_id", approvalID).Error("Failed to get approval")
		response.NotFound(w, "Approval not found")
		return
	}

	response.Success(w, approval)
}

// ApprovalDecisionRequest represents an approval decision (approve/reject)
type ApprovalDecisionRequest struct {
	Reason string `json:"reason,omitempty"`
}

// Approve handles PUT /api/v1/approvals/:id/approve
func (h *ApprovalsHandler) Approve(w http.ResponseWriter, r *http.Request) {
	// Get approval ID from URL
	approvalIDStr := chi.URLParam(r, "id")
	approvalID, err := uuid.Parse(approvalIDStr)
	if err != nil {
		response.BadRequest(w, "Invalid approval ID")
		return
	}

	// Parse request
	var req ApprovalDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Reason is optional, so empty body is OK
		req.Reason = "Approved"
	}

	// Get authenticated user from context
	apiKey, ok := middleware.GetAPIKeyFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "Authentication required")
		return
	}

	reviewedBy := apiKey.Name
	if req.Reason == "" {
		req.Reason = "Approved via API"
	}

	// First, get the approval to check if it exists and is pending
	approval, err := h.approvalRepo.GetByID(r.Context(), approvalID)
	if err != nil {
		h.logger.WithError(err).WithField("approval_id", approvalID).Error("Failed to get approval")
		response.NotFound(w, "Approval not found")
		return
	}

	// Check if approval is pending
	if !approval.IsPending() {
		response.BadRequest(w, "Approval is not in pending status")
		return
	}

	// Check if expired
	if approval.IsExpired() {
		response.BadRequest(w, "Approval has expired")
		return
	}

	// Approve
	if err := h.approvalRepo.Approve(r.Context(), approvalID, reviewedBy, req.Reason); err != nil {
		h.logger.WithError(err).WithField("approval_id", approvalID).Error("Failed to approve")
		response.InternalServerError(w, "Failed to approve")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"approval_id": approvalID,
		"reviewed_by": reviewedBy,
		"action_name": approval.ActionName,
	}).Info("Approval approved")

	// Get updated approval
	updatedApproval, err := h.approvalRepo.GetByID(r.Context(), approvalID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get updated approval")
		response.InternalServerError(w, "Approval approved but failed to retrieve updated data")
		return
	}

	response.Success(w, map[string]interface{}{
		"message":  "Approval approved successfully",
		"approval": updatedApproval,
	})
}

// Reject handles PUT /api/v1/approvals/:id/reject
func (h *ApprovalsHandler) Reject(w http.ResponseWriter, r *http.Request) {
	// Get approval ID from URL
	approvalIDStr := chi.URLParam(r, "id")
	approvalID, err := uuid.Parse(approvalIDStr)
	if err != nil {
		response.BadRequest(w, "Invalid approval ID")
		return
	}

	// Parse request
	var req ApprovalDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Reason is optional, so empty body is OK
		req.Reason = "Rejected"
	}

	// Get authenticated user from context
	apiKey, ok := middleware.GetAPIKeyFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "Authentication required")
		return
	}

	reviewedBy := apiKey.Name
	if req.Reason == "" {
		req.Reason = "Rejected via API"
	}

	// First, get the approval to check if it exists and is pending
	approval, err := h.approvalRepo.GetByID(r.Context(), approvalID)
	if err != nil {
		h.logger.WithError(err).WithField("approval_id", approvalID).Error("Failed to get approval")
		response.NotFound(w, "Approval not found")
		return
	}

	// Check if approval is pending
	if !approval.IsPending() {
		response.BadRequest(w, "Approval is not in pending status")
		return
	}

	// Reject
	if err := h.approvalRepo.Reject(r.Context(), approvalID, reviewedBy, req.Reason); err != nil {
		h.logger.WithError(err).WithField("approval_id", approvalID).Error("Failed to reject")
		response.InternalServerError(w, "Failed to reject")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"approval_id": approvalID,
		"reviewed_by": reviewedBy,
		"action_name": approval.ActionName,
	}).Info("Approval rejected")

	// Get updated approval
	updatedApproval, err := h.approvalRepo.GetByID(r.Context(), approvalID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get updated approval")
		response.InternalServerError(w, "Approval rejected but failed to retrieve updated data")
		return
	}

	response.Success(w, map[string]interface{}{
		"message":  "Approval rejected successfully",
		"approval": updatedApproval,
	})
}

// Stats handles GET /api/v1/approvals/stats
func (h *ApprovalsHandler) Stats(w http.ResponseWriter, r *http.Request) {
	// Get count by status
	statusCounts, err := h.approvalRepo.CountByStatus(r.Context())
	if err != nil {
		h.logger.WithError(err).Error("Failed to get approval stats by status")
		response.InternalServerError(w, "Failed to get approval stats")
		return
	}

	// Get count by risk level (pending only)
	riskCounts, err := h.approvalRepo.CountByRiskLevel(r.Context())
	if err != nil {
		h.logger.WithError(err).Error("Failed to get approval stats by risk level")
		response.InternalServerError(w, "Failed to get approval stats")
		return
	}

	// Calculate total
	total := 0
	for _, count := range statusCounts {
		total += count
	}

	response.Success(w, map[string]interface{}{
		"total":         total,
		"by_status":     statusCounts,
		"by_risk_level": riskCounts,
		"pending":       statusCounts[models.ApprovalStatusPending],
		"approved":      statusCounts[models.ApprovalStatusApproved],
		"rejected":      statusCounts[models.ApprovalStatusRejected],
		"expired":       statusCounts[models.ApprovalStatusExpired],
	})
}
