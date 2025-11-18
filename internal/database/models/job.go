package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JobType represents the type of job
type JobType string

const (
	JobTypeDiagnostic   JobType = "diagnostic"
	JobTypeRemediation  JobType = "remediation"
)

// JobStatus represents the current status of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// Job represents a diagnostic or remediation job
type Job struct {
	ID          uuid.UUID       `json:"id"`
	Type        JobType         `json:"type"`
	Status      JobStatus       `json:"status"`
	Target      string          `json:"target"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	CreatedBy   string          `json:"created_by,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       *string         `json:"error,omitempty"`
	Metadata    JSONB           `json:"metadata,omitempty"`
}

// JSONB is a helper type for PostgreSQL JSONB columns
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan JSONB: expected []byte, got %T", value)
	}

	if len(bytes) == 0 {
		*j = make(map[string]interface{})
		return nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return fmt.Errorf("failed to unmarshal JSONB: %w", err)
	}

	*j = data
	return nil
}

// Duration returns the job duration if started
func (j *Job) Duration() *time.Duration {
	if j.StartedAt == nil {
		return nil
	}

	endTime := time.Now()
	if j.CompletedAt != nil {
		endTime = *j.CompletedAt
	}

	duration := endTime.Sub(*j.StartedAt)
	return &duration
}

// IsComplete returns true if the job has finished (completed, failed, or cancelled)
func (j *Job) IsComplete() bool {
	return j.Status == JobStatusCompleted ||
		j.Status == JobStatusFailed ||
		j.Status == JobStatusCancelled
}

// IsRunning returns true if the job is currently running
func (j *Job) IsRunning() bool {
	return j.Status == JobStatusRunning
}

// IsPending returns true if the job is pending
func (j *Job) IsPending() bool {
	return j.Status == JobStatusPending
}
