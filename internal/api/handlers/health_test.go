package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestHealthHandler_Health_NoDatabaseConfigured(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs during testing

	handler := NewHealthHandler(nil, logger) // No DB configured

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	// Execute
	handler.Health(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)

	var wrappedResp struct {
		Success bool           `json:"success"`
		Data    HealthResponse `json:"data"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &wrappedResp)
	assert.NoError(t, err)
	assert.True(t, wrappedResp.Success)
	resp := wrappedResp.Data

	assert.Equal(t, "healthy", resp.Status)
	assert.NotEmpty(t, resp.Timestamp)
	assert.NotEmpty(t, resp.Uptime)
	assert.NotEmpty(t, resp.Version)
	assert.NotNil(t, resp.Services)

	// Database should be marked as not_configured
	dbStatus, exists := resp.Services["database"]
	assert.True(t, exists)
	assert.Equal(t, "not_configured", dbStatus.Status)
}

func TestHealthHandler_Ready_NoDatabaseConfigured(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewHealthHandler(nil, logger) // No database configured

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil)
	rec := httptest.NewRecorder()

	// Execute
	handler.Ready(rec, req)

	// Assert - should still be ready even without database
	assert.Equal(t, http.StatusOK, rec.Code)

	var wrappedResp struct {
		Success bool              `json:"success"`
		Data    map[string]string `json:"data"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &wrappedResp)
	assert.NoError(t, err)
	assert.True(t, wrappedResp.Success)
	assert.Equal(t, "ready", wrappedResp.Data["status"])
}

func TestHealthHandler_Live(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewHealthHandler(nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/live", nil)
	rec := httptest.NewRecorder()

	// Execute
	handler.Live(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)

	var wrappedResp struct {
		Success bool              `json:"success"`
		Data    map[string]string `json:"data"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &wrappedResp)
	assert.NoError(t, err)
	assert.True(t, wrappedResp.Success)
	assert.Equal(t, "alive", wrappedResp.Data["status"])
}

func TestHealthHandler_UptimeCalculation(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewHealthHandler(nil, logger)

	// Wait a bit to ensure uptime is measurable
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	// Execute
	handler.Health(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)

	var wrappedResp struct {
		Success bool           `json:"success"`
		Data    HealthResponse `json:"data"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &wrappedResp)
	assert.NoError(t, err)
	resp := wrappedResp.Data

	// Uptime should be a non-empty string
	assert.NotEmpty(t, resp.Uptime)
	// Timestamp should be recent
	assert.True(t, time.Since(resp.Timestamp) < time.Second)
}
