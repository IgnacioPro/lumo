package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ignacio/lumo/internal/database/models"
)

func newTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	return logger
}

func TestEventsHandler_SubmitEvents_InvalidJSON(t *testing.T) {
	logger := newTestLogger()
	handler := NewEventsHandler(nil, nil, nil, nil, logger, false, false)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()

	handler.SubmitEvents(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEventsHandler_SubmitEvents_EmptyEvents(t *testing.T) {
	logger := newTestLogger()
	handler := NewEventsHandler(nil, nil, nil, nil, logger, false, false)

	reqBody := models.SubmitEventRequest{
		Events: []models.EventSubmission{},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.SubmitEvents(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEventsHandler_SubmitEvents_TooManyEvents(t *testing.T) {
	logger := newTestLogger()
	handler := NewEventsHandler(nil, nil, nil, nil, logger, false, false)

	// Create 101 events (max is 100)
	events := make([]models.EventSubmission, 101)
	for i := range events {
		events[i] = models.EventSubmission{
			EventType:      "test-event",
			Severity:       models.EventSeverityLow,
			ResourceKind:   "Pod",
			ResourceName:   "test-pod",
			Message:        "Test message",
			EventTimestamp: time.Now(),
		}
	}

	reqBody := models.SubmitEventRequest{Events: events}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.SubmitEvents(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEventsHandler_SubmitEvents_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		event   models.EventSubmission
		wantErr string
	}{
		{
			name: "Missing event_type",
			event: models.EventSubmission{
				Severity:       models.EventSeverityHigh,
				ResourceKind:   "Pod",
				ResourceName:   "test-pod",
				Message:        "Test message",
				EventTimestamp: time.Now(),
			},
			wantErr: "event_type is required",
		},
		{
			name: "Missing severity",
			event: models.EventSubmission{
				EventType:      "crash-loop-backoff",
				ResourceKind:   "Pod",
				ResourceName:   "test-pod",
				Message:        "Test message",
				EventTimestamp: time.Now(),
			},
			wantErr: "severity is required",
		},
		{
			name: "Missing resource_kind",
			event: models.EventSubmission{
				EventType:      "crash-loop-backoff",
				Severity:       models.EventSeverityHigh,
				ResourceName:   "test-pod",
				Message:        "Test message",
				EventTimestamp: time.Now(),
			},
			wantErr: "resource_kind is required",
		},
		{
			name: "Missing resource_name",
			event: models.EventSubmission{
				EventType:      "crash-loop-backoff",
				Severity:       models.EventSeverityHigh,
				ResourceKind:   "Pod",
				Message:        "Test message",
				EventTimestamp: time.Now(),
			},
			wantErr: "resource_name is required",
		},
		{
			name: "Missing message",
			event: models.EventSubmission{
				EventType:      "crash-loop-backoff",
				Severity:       models.EventSeverityHigh,
				ResourceKind:   "Pod",
				ResourceName:   "test-pod",
				EventTimestamp: time.Now(),
			},
			wantErr: "message is required",
		},
		{
			name: "Missing event_timestamp",
			event: models.EventSubmission{
				EventType:    "crash-loop-backoff",
				Severity:     models.EventSeverityHigh,
				ResourceKind: "Pod",
				ResourceName: "test-pod",
				Message:      "Test message",
			},
			wantErr: "event_timestamp is required",
		},
		{
			name: "Invalid severity",
			event: models.EventSubmission{
				EventType:      "crash-loop-backoff",
				Severity:       models.EventSeverity("invalid"),
				ResourceKind:   "Pod",
				ResourceName:   "test-pod",
				Message:        "Test message",
				EventTimestamp: time.Now(),
			},
			wantErr: "invalid severity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := newTestLogger()
			handler := NewEventsHandler(nil, nil, nil, nil, logger, false, false)

			err := handler.validateEventSubmission(&tt.event)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestEventsHandler_ValidateEventSubmission_Success(t *testing.T) {
	logger := newTestLogger()
	handler := NewEventsHandler(nil, nil, nil, nil, logger, false, false)

	validEvent := &models.EventSubmission{
		EventType:      "crash-loop-backoff",
		Severity:       models.EventSeverityHigh,
		ResourceKind:   "Pod",
		ResourceName:   "test-pod",
		Message:        "Pod is in CrashLoopBackOff state",
		EventTimestamp: time.Now(),
	}

	err := handler.validateEventSubmission(validEvent)
	assert.NoError(t, err)
}

func TestEventsHandler_ValidateEventSubmission_AllSeverities(t *testing.T) {
	logger := newTestLogger()
	handler := NewEventsHandler(nil, nil, nil, nil, logger, false, false)

	severities := []models.EventSeverity{
		models.EventSeverityLow,
		models.EventSeverityMedium,
		models.EventSeverityHigh,
		models.EventSeverityCritical,
	}

	for _, severity := range severities {
		t.Run(string(severity), func(t *testing.T) {
			event := &models.EventSubmission{
				EventType:      "test-event",
				Severity:       severity,
				ResourceKind:   "Pod",
				ResourceName:   "test-pod",
				Message:        "Test message",
				EventTimestamp: time.Now(),
			}

			err := handler.validateEventSubmission(event)
			assert.NoError(t, err)
		})
	}
}

func TestEventsHandler_GetEvent_InvalidID(t *testing.T) {
	logger := newTestLogger()
	handler := NewEventsHandler(nil, nil, nil, nil, logger, false, false)

	// Create router with URL parameter
	r := chi.NewRouter()
	r.Get("/api/v1/events/{id}", handler.GetEvent)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/invalid-uuid", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEventsHandler_ListEvents_Success(t *testing.T) {
	// Skip this test as it requires a real database connection
	// The handler expects a non-nil repository
	t.Skip("Requires database connection - covered by integration tests")
}

func TestEventsHandler_ListEvents_WithFilters(t *testing.T) {
	// Skip this test as it requires a real database connection
	t.Skip("Requires database connection - covered by integration tests")
}

func TestEventsHandler_BuildNotificationMessage(t *testing.T) {
	logger := newTestLogger()
	handler := NewEventsHandler(nil, nil, nil, nil, logger, false, false)

	namespace := "default"
	event := &models.Event{
		ID:             uuid.New(),
		EventType:      "crash-loop-backoff",
		Severity:       models.EventSeverityCritical,
		ResourceKind:   "Pod",
		ResourceName:   "my-app-pod",
		Namespace:      &namespace,
		Message:        "Pod is in CrashLoopBackOff state after 5 restarts",
		EventTimestamp: time.Now(),
	}

	notification := handler.buildNotificationMessage(event)

	assert.Contains(t, notification.Title, "crash-loop-backoff")
	assert.Contains(t, notification.Title, "🔴") // Critical severity emoji
	assert.Contains(t, notification.Message, "critical")
	assert.Contains(t, notification.Message, "Pod/my-app-pod")
	assert.Contains(t, notification.Message, "default")
}

func TestEventsHandler_BuildNotificationMessage_AllSeverities(t *testing.T) {
	logger := newTestLogger()
	handler := NewEventsHandler(nil, nil, nil, nil, logger, false, false)

	tests := []struct {
		severity models.EventSeverity
		emoji    string
	}{
		{models.EventSeverityCritical, "🔴"},
		{models.EventSeverityHigh, "🟠"},
		{models.EventSeverityMedium, "🟡"},
		{models.EventSeverityLow, "🔵"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			event := &models.Event{
				ID:             uuid.New(),
				EventType:      "test-event",
				Severity:       tt.severity,
				ResourceKind:   "Pod",
				ResourceName:   "test-pod",
				Message:        "Test message",
				EventTimestamp: time.Now(),
			}

			notification := handler.buildNotificationMessage(event)
			assert.Contains(t, notification.Title, tt.emoji)
		})
	}
}

func TestEventsHandler_BuildNotificationMessage_WithAIAnalysis(t *testing.T) {
	logger := newTestLogger()
	handler := NewEventsHandler(nil, nil, nil, nil, logger, false, false)

	analysis := "The pod is experiencing OOM kills due to memory limits being too low."
	event := &models.Event{
		ID:             uuid.New(),
		EventType:      "oom-killed",
		Severity:       models.EventSeverityCritical,
		ResourceKind:   "Pod",
		ResourceName:   "memory-hog",
		Message:        "Container killed due to OOM",
		EventTimestamp: time.Now(),
		AIAnalysis:     &analysis,
	}

	notification := handler.buildNotificationMessage(event)
	assert.Contains(t, notification.Message, "AI Analysis")
	assert.Contains(t, notification.Message, "OOM kills")
}

func TestEventsHandler_GetAgentIDFromContext(t *testing.T) {
	logger := newTestLogger()
	handler := NewEventsHandler(nil, nil, nil, nil, logger, false, false)

	t.Run("No agent ID in context", func(t *testing.T) {
		ctx := context.Background()
		_, err := handler.getAgentIDFromContext(ctx)
		assert.Error(t, err)
	})

	t.Run("Agent ID as UUID in context", func(t *testing.T) {
		expectedID := uuid.New()
		// Use the same key type as the handler expects
		ctx := context.WithValue(context.Background(), "agent_id", expectedID) //nolint:staticcheck
		id, err := handler.getAgentIDFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expectedID, id)
	})

	t.Run("Agent ID as string in context", func(t *testing.T) {
		expectedID := uuid.New()
		// Use the same key type as the handler expects
		ctx := context.WithValue(context.Background(), "agent_id", expectedID.String()) //nolint:staticcheck
		id, err := handler.getAgentIDFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expectedID, id)
	})
}

func TestStringOrEmpty(t *testing.T) {
	t.Run("Nil string returns N/A", func(t *testing.T) {
		result := stringOrEmpty(nil)
		assert.Equal(t, "N/A", result)
	})

	t.Run("Non-nil string returns value", func(t *testing.T) {
		value := "test-namespace"
		result := stringOrEmpty(&value)
		assert.Equal(t, "test-namespace", result)
	})

	t.Run("Empty string returns empty", func(t *testing.T) {
		value := ""
		result := stringOrEmpty(&value)
		assert.Equal(t, "", result)
	})
}
