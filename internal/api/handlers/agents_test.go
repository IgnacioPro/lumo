package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestAgentsHandler_Register_InvalidJSON(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAgentsHandler(nil, logger)

	// Create invalid JSON request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/register", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()

	// Execute
	handler.Register(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAgentsHandler_Register_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		request RegisterRequest
		errMsg  string
	}{
		{
			name:    "Missing name",
			request: RegisterRequest{Hostname: "test", Platform: models.AgentPlatformLinux, Architecture: "amd64", Version: "1.0.0"},
			errMsg:  "Name is required",
		},
		{
			name:    "Missing hostname",
			request: RegisterRequest{Name: "test", Platform: models.AgentPlatformLinux, Architecture: "amd64", Version: "1.0.0"},
			errMsg:  "Hostname is required",
		},
		{
			name:    "Missing platform",
			request: RegisterRequest{Name: "test", Hostname: "test", Architecture: "amd64", Version: "1.0.0"},
			errMsg:  "Platform is required",
		},
		{
			name:    "Missing architecture",
			request: RegisterRequest{Name: "test", Hostname: "test", Platform: models.AgentPlatformLinux, Version: "1.0.0"},
			errMsg:  "Architecture is required",
		},
		{
			name:    "Missing version",
			request: RegisterRequest{Name: "test", Hostname: "test", Platform: models.AgentPlatformLinux, Architecture: "amd64"},
			errMsg:  "Version is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			logger.SetLevel(logrus.FatalLevel)

			handler := NewAgentsHandler(nil, logger)

			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/register", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			handler.Register(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestAgentsHandler_Register_InvalidPlatform(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAgentsHandler(nil, logger)

	reqBody := RegisterRequest{
		Name:         "test-agent",
		Hostname:     "test-host",
		Platform:     "invalid-platform",
		Architecture: "amd64",
		Version:      "1.0.0",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// Execute
	handler.Register(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAgentsHandler_Heartbeat_InvalidID(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAgentsHandler(nil, logger)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/agents/invalid-id/heartbeat", nil)
	rec := httptest.NewRecorder()

	// Set up chi context with invalid UUID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute
	handler.Heartbeat(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAgentsHandler_Get_InvalidID(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAgentsHandler(nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/invalid-id", nil)
	rec := httptest.NewRecorder()

	// Set up chi context with invalid UUID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute
	handler.Get(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAgentsHandler_Update_InvalidID(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAgentsHandler(nil, logger)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/agents/invalid-id", nil)
	rec := httptest.NewRecorder()

	// Set up chi context with invalid UUID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute
	handler.Update(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAgentsHandler_Update_InvalidJSON(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAgentsHandler(nil, logger)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/agents/550e8400-e29b-41d4-a716-446655440000", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()

	// Set up chi context with valid UUID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute
	handler.Update(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAgentsHandler_Delete_InvalidID(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAgentsHandler(nil, logger)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/invalid-id", nil)
	rec := httptest.NewRecorder()

	// Set up chi context with invalid UUID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute
	handler.Delete(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
