package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestJobsHandler tests require a database connection.
// These tests focus on HTTP request handling and validation logic.

func TestJobsHandler_Get_InvalidID(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewJobsHandler(nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/invalid-id", nil)
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

func TestJobsHandler_Get_MissingID(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewJobsHandler(nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/", nil)
	rec := httptest.NewRecorder()

	// Set up chi context without ID parameter
	rctx := chi.NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute
	handler.Get(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestJobsHandler_Delete_InvalidID(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewJobsHandler(nil, logger)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/invalid-id", nil)
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

func TestJobsHandler_Delete_MissingID(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewJobsHandler(nil, logger)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/", nil)
	rec := httptest.NewRecorder()

	// Set up chi context without ID parameter
	rctx := chi.NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute
	handler.Delete(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedLimit  int
		expectedOffset int
	}{
		{
			name:           "No parameters - defaults",
			url:            "/api/v1/jobs",
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "Valid limit and offset",
			url:            "/api/v1/jobs?limit=10&offset=20",
			expectedLimit:  10,
			expectedOffset: 20,
		},
		{
			name:           "Limit exceeds max - capped at 100",
			url:            "/api/v1/jobs?limit=150",
			expectedLimit:  50, // Falls back to default when > 100
			expectedOffset: 0,
		},
		{
			name:           "Negative limit - uses default",
			url:            "/api/v1/jobs?limit=-5",
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "Negative offset - uses default",
			url:            "/api/v1/jobs?offset=-10",
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "Invalid limit format - uses default",
			url:            "/api/v1/jobs?limit=abc",
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "Valid offset only",
			url:            "/api/v1/jobs?offset=100",
			expectedLimit:  50,
			expectedOffset: 100,
		},
		{
			name:           "Limit at max (100)",
			url:            "/api/v1/jobs?limit=100",
			expectedLimit:  100,
			expectedOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			limit, offset := parsePagination(req)
			assert.Equal(t, tt.expectedLimit, limit, "Limit mismatch")
			assert.Equal(t, tt.expectedOffset, offset, "Offset mismatch")
		})
	}
}
