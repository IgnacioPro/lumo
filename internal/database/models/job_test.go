package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobType(t *testing.T) {
	assert.Equal(t, JobType("diagnostic"), JobTypeDiagnostic)
	assert.Equal(t, JobType("remediation"), JobTypeRemediation)
}

func TestJobStatus(t *testing.T) {
	assert.Equal(t, JobStatus("pending"), JobStatusPending)
	assert.Equal(t, JobStatus("running"), JobStatusRunning)
	assert.Equal(t, JobStatus("completed"), JobStatusCompleted)
	assert.Equal(t, JobStatus("failed"), JobStatusFailed)
}

func TestJob_Validation(t *testing.T) {
	t.Run("ValidJob", func(t *testing.T) {
		job := &Job{
			Type:      JobTypeDiagnostic,
			Status:    JobStatusPending,
			Target:    "localhost",
			CreatedBy: "test-user",
		}

		assert.NotEmpty(t, job.Type)
		assert.NotEmpty(t, job.Status)
		assert.NotEmpty(t, job.Target)
	})

	t.Run("WithMetadata", func(t *testing.T) {
		metadata := JSONB{
			"key1": "value1",
			"key2": float64(123),
			"key3": true,
		}

		job := &Job{
			Type:      JobTypeDiagnostic,
			Status:    JobStatusPending,
			Target:    "localhost",
			Metadata:  metadata,
			CreatedBy: "test-user",
		}

		assert.Equal(t, "value1", job.Metadata["key1"])
		assert.Equal(t, float64(123), job.Metadata["key2"])
		assert.Equal(t, true, job.Metadata["key3"])
	})
}

func TestJSONB_MarshalUnmarshal(t *testing.T) {
	original := JSONB{
		"string": "value",
		"number": 42,
		"bool":   true,
		"nested": map[string]interface{}{
			"inner": "data",
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back
	var decoded JSONB
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "value", decoded["string"])
	assert.Equal(t, float64(42), decoded["number"])
	assert.Equal(t, true, decoded["bool"])

	nested, ok := decoded["nested"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "data", nested["inner"])
}

func TestJob_StatusTransitions(t *testing.T) {
	job := &Job{
		ID:        uuid.New(),
		Type:      JobTypeDiagnostic,
		Status:    JobStatusPending,
		Target:    "localhost",
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
	}

	// Pending -> Running
	job.Status = JobStatusRunning
	job.StartedAt = timePtr(time.Now())
	assert.Equal(t, JobStatusRunning, job.Status)
	assert.NotNil(t, job.StartedAt)

	// Running -> Completed
	job.Status = JobStatusCompleted
	job.CompletedAt = timePtr(time.Now())
	assert.Equal(t, JobStatusCompleted, job.Status)
	assert.NotNil(t, job.CompletedAt)
}

func TestJob_FailedStatus(t *testing.T) {
	job := &Job{
		ID:        uuid.New(),
		Type:      JobTypeDiagnostic,
		Status:    JobStatusPending,
		Target:    "localhost",
		CreatedBy: "test-user",
		CreatedAt: time.Now(),
	}

	// Set to failed with error
	job.Status = JobStatusFailed
	job.Error = strPtr("Connection timeout")
	job.CompletedAt = timePtr(time.Now())

	assert.Equal(t, JobStatusFailed, job.Status)
	assert.NotNil(t, job.Error)
	assert.Equal(t, "Connection timeout", *job.Error)
}

func TestJob_WithResult(t *testing.T) {
	result := []byte(`{"test": "data"}`)

	job := &Job{
		ID:          uuid.New(),
		Type:        JobTypeDiagnostic,
		Status:      JobStatusCompleted,
		Target:      "localhost",
		Result:      result,
		CreatedBy:   "test-user",
		CreatedAt:   time.Now(),
		CompletedAt: timePtr(time.Now()),
	}

	assert.NotNil(t, job.Result)
	assert.Contains(t, string(job.Result), "test")

	// Verify it's valid JSON
	var decoded map[string]interface{}
	err := json.Unmarshal(job.Result, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "data", decoded["test"])
}

func TestJob_Duration(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-5 * time.Minute)

	job := &Job{
		CreatedAt:   earlier,
		StartedAt:   timePtr(earlier.Add(1 * time.Minute)),
		CompletedAt: timePtr(now),
	}

	// Calculate duration manually
	if job.CompletedAt != nil && job.StartedAt != nil {
		duration := job.CompletedAt.Sub(*job.StartedAt)
		assert.GreaterOrEqual(t, duration, 3*time.Minute)
		assert.LessOrEqual(t, duration, 5*time.Minute)
	}
}

// Helper functions
func timePtr(t time.Time) *time.Time {
	return &t
}

func strPtr(s string) *string {
	return &s
}
