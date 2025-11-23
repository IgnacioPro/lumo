package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ignacio/lumo/internal/api/auth"
	"github.com/ignacio/lumo/internal/api/middleware"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestAuthHandler_GenerateToken_InvalidJSON(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAuthHandler(nil, nil, logger)

	// Create invalid JSON request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()

	// Execute
	handler.GenerateToken(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthHandler_GenerateToken_MissingAPIKey(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAuthHandler(nil, nil, logger)

	// Create request without API key
	reqBody := TokenRequest{}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// Execute
	handler.GenerateToken(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthHandler_RefreshToken_NoToken(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAuthHandler(nil, nil, logger)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	rec := httptest.NewRecorder()

	// Execute (without JWT claims in context)
	handler.RefreshToken(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthHandler_RefreshToken_MissingAuthHeader(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAuthHandler(nil, nil, logger)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	rec := httptest.NewRecorder()

	// Add mock JWT claims to context
	claims := &auth.Claims{
		UserID:   uuid.New().String(),
		Username: "test-user",
	}
	ctx := context.WithValue(req.Context(), middleware.JWTClaimsKey, claims)
	req = req.WithContext(ctx)

	// Execute without Authorization header
	handler.RefreshToken(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthHandler_RefreshToken_InvalidAuthHeader(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAuthHandler(nil, nil, logger)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	rec := httptest.NewRecorder()

	// Add mock JWT claims to context
	claims := &auth.Claims{
		UserID:   uuid.New().String(),
		Username: "test-user",
	}
	ctx := context.WithValue(req.Context(), middleware.JWTClaimsKey, claims)
	req = req.WithContext(ctx)

	// Execute
	handler.RefreshToken(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthHandler_ValidateToken_NoToken(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewAuthHandler(nil, nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/validate", nil)
	rec := httptest.NewRecorder()

	// Execute without JWT claims in context
	handler.ValidateToken(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

