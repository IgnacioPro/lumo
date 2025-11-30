package models

import (
	"time"

	"github.com/google/uuid"
)

// EventSeverity represents the severity level of a Kubernetes event
type EventSeverity string

const (
	EventSeverityLow      EventSeverity = "low"
	EventSeverityMedium   EventSeverity = "medium"
	EventSeverityHigh     EventSeverity = "high"
	EventSeverityCritical EventSeverity = "critical"
)

// Event represents a Kubernetes event reported by an agent
type Event struct {
	ID                   uuid.UUID     `json:"id"`
	TenantID             uuid.UUID     `json:"tenant_id,omitempty"`
	AgentID              uuid.UUID     `json:"agent_id"`
	EventType            string        `json:"event_type"`
	Severity             EventSeverity `json:"severity"`
	ResourceKind         string        `json:"resource_kind"`
	ResourceName         string        `json:"resource_name"`
	ResourceUID          *string       `json:"resource_uid,omitempty"`
	Namespace            *string       `json:"namespace,omitempty"`
	Message              string        `json:"message"`
	Metadata             JSONB         `json:"metadata,omitempty"`
	EventTimestamp       time.Time     `json:"event_timestamp"`
	AIAnalysis           *string       `json:"ai_analysis,omitempty"`
	AIAnalyzedAt         *time.Time    `json:"ai_analyzed_at,omitempty"`
	NotificationSent     bool          `json:"notification_sent"`
	NotificationSentAt   *time.Time    `json:"notification_sent_at,omitempty"`
	NotificationChannels []string      `json:"notification_channels,omitempty"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}

// IsCritical returns true if the event is critical severity
func (e *Event) IsCritical() bool {
	return e.Severity == EventSeverityCritical
}

// IsHigh returns true if the event is high severity
func (e *Event) IsHigh() bool {
	return e.Severity == EventSeverityHigh
}

// IsMedium returns true if the event is medium severity
func (e *Event) IsMedium() bool {
	return e.Severity == EventSeverityMedium
}

// IsLow returns true if the event is low severity
func (e *Event) IsLow() bool {
	return e.Severity == EventSeverityLow
}

// IsHighPriority returns true if the event is high or critical severity
func (e *Event) IsHighPriority() bool {
	return e.IsCritical() || e.IsHigh()
}

// HasAIAnalysis returns true if AI analysis has been performed
func (e *Event) HasAIAnalysis() bool {
	return e.AIAnalysis != nil && *e.AIAnalysis != ""
}

// WasNotified returns true if notification was sent
func (e *Event) WasNotified() bool {
	return e.NotificationSent
}

// Age returns the duration since the event occurred
func (e *Event) Age() time.Duration {
	return time.Since(e.EventTimestamp)
}

// IsRecent returns true if the event occurred within the given duration
func (e *Event) IsRecent(threshold time.Duration) bool {
	return e.Age() <= threshold
}

// SubmitEventRequest represents a request to submit one or more events
type SubmitEventRequest struct {
	Events []EventSubmission `json:"events" validate:"required,dive"`
}

// EventSubmission represents a single event submission from an agent
type EventSubmission struct {
	EventType      string                 `json:"event_type" validate:"required"`
	Severity       EventSeverity          `json:"severity" validate:"required,oneof=low medium high critical"`
	ResourceKind   string                 `json:"resource_kind" validate:"required"`
	ResourceName   string                 `json:"resource_name" validate:"required"`
	ResourceUID    *string                `json:"resource_uid,omitempty"`
	Namespace      *string                `json:"namespace,omitempty"`
	Message        string                 `json:"message" validate:"required"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	EventTimestamp time.Time              `json:"event_timestamp" validate:"required"`
}

// SubmitEventResponse represents the response after submitting events
type SubmitEventResponse struct {
	Accepted int      `json:"accepted"`
	Rejected int      `json:"rejected"`
	EventIDs []string `json:"event_ids,omitempty"`
	Errors   []string `json:"errors,omitempty"`
	Message  string   `json:"message"`
}
