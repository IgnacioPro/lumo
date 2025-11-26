package models

import (
	"time"

	"github.com/google/uuid"
)

// ApprovalStatus represents the status of an approval request
type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
	ApprovalStatusExpired  ApprovalStatus = "expired"
)

// ApprovalRiskLevel represents the risk level of an action
type ApprovalRiskLevel string

const (
	ApprovalRiskSafe     ApprovalRiskLevel = "safe"
	ApprovalRiskModerate ApprovalRiskLevel = "moderate"
	ApprovalRiskCritical ApprovalRiskLevel = "critical"
)

// Approval represents a pending remediation action approval request
type Approval struct {
	ID              uuid.UUID         `json:"id"`
	JobID           uuid.UUID         `json:"job_id"`
	ActionID        string            `json:"action_id"`
	ActionName      string            `json:"action_name"`
	ActionCategory  string            `json:"action_category"`
	Description     string            `json:"description"`
	RiskLevel       ApprovalRiskLevel `json:"risk_level"`
	IsReversible    bool              `json:"is_reversible"`
	EstimatedImpact string            `json:"estimated_impact"`
	Target          string            `json:"target"`
	Status          ApprovalStatus    `json:"status"`
	RequestedBy     string            `json:"requested_by"`
	RequestedAt     time.Time         `json:"requested_at"`
	ReviewedBy      *string           `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time        `json:"reviewed_at,omitempty"`
	Reason          *string           `json:"reason,omitempty"`
	ExpiresAt       *time.Time        `json:"expires_at,omitempty"`
	Metadata        JSONB             `json:"metadata,omitempty"`
}

// IsPending returns true if the approval is pending
func (a *Approval) IsPending() bool {
	// Check if expired
	if a.ExpiresAt != nil && time.Now().After(*a.ExpiresAt) {
		return false
	}
	return a.Status == ApprovalStatusPending
}

// IsExpired returns true if the approval has expired
func (a *Approval) IsExpired() bool {
	if a.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*a.ExpiresAt)
}

// IsApproved returns true if the approval has been approved
func (a *Approval) IsApproved() bool {
	return a.Status == ApprovalStatusApproved
}

// IsRejected returns true if the approval has been rejected
func (a *Approval) IsRejected() bool {
	return a.Status == ApprovalStatusRejected
}

// IsResolved returns true if the approval has been approved, rejected, or expired
func (a *Approval) IsResolved() bool {
	return a.Status == ApprovalStatusApproved ||
		a.Status == ApprovalStatusRejected ||
		a.Status == ApprovalStatusExpired
}
