package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestApprovalsHandler_Get_InvalidID(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewApprovalsHandler(nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/approvals/invalid-id", nil)
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

func TestApprovalsHandler_Approve_InvalidID(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewApprovalsHandler(nil, logger)

	reqBody := ApprovalDecisionRequest{Reason: "Test approval"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/approvals/invalid-id/approve", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// Set up chi context with invalid UUID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute
	handler.Approve(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestApprovalsHandler_Approve_NoAuthentication(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewApprovalsHandler(nil, logger)

	reqBody := ApprovalDecisionRequest{Reason: "Test approval"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/approvals/550e8400-e29b-41d4-a716-446655440000/approve", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// Set up chi context with valid UUID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute without API key in context
	handler.Approve(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestApprovalsHandler_Reject_InvalidID(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewApprovalsHandler(nil, logger)

	reqBody := ApprovalDecisionRequest{Reason: "Test rejection"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/approvals/invalid-id/reject", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// Set up chi context with invalid UUID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute
	handler.Reject(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestApprovalsHandler_Reject_NoAuthentication(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewApprovalsHandler(nil, logger)

	reqBody := ApprovalDecisionRequest{Reason: "Test rejection"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/approvals/550e8400-e29b-41d4-a716-446655440000/reject", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// Set up chi context with valid UUID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Execute without API key in context
	handler.Reject(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
