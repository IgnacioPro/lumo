package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ignacio/lumo/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestDiagnosticsHandler_Run_InvalidJSON(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	handler := NewDiagnosticsHandler(nil, cfg, logger)

	// Create invalid JSON request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/diagnostics", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()

	// Execute
	handler.Run(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDiagnosticsHandler_Run_MissingTarget(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	handler := NewDiagnosticsHandler(nil, cfg, logger)

	// Create request without target
	reqBody := DiagnosticRequest{}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/diagnostics", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// Execute
	handler.Run(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDiagnosticsHandler_Run_InvalidTarget(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	handler := NewDiagnosticsHandler(nil, cfg, logger)

	// Create request with invalid target (SSRF attempt)
	reqBody := DiagnosticRequest{
		Target: "http://169.254.169.254/latest/meta-data/",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/diagnostics", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// Execute
	handler.Run(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDiagnosticsHandler_IsLocalhost(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		expected bool
	}{
		{
			name:     "localhost",
			target:   "localhost",
			expected: true,
		},
		{
			name:     "127.0.0.1",
			target:   "127.0.0.1",
			expected: true,
		},
		{
			name:     "::1",
			target:   "::1",
			expected: true,
		},
		{
			name:     "0.0.0.0",
			target:   "0.0.0.0",
			expected: true,
		},
		{
			name:     "LOCALHOST uppercase",
			target:   "LOCALHOST",
			expected: true,
		},
		{
			name:     "example.com",
			target:   "example.com",
			expected: false,
		},
		{
			name:     "192.168.1.1",
			target:   "192.168.1.1",
			expected: false,
		},
		{
			name:     "localhost with port - not matched",
			target:   "localhost:8080",
			expected: false,
		},
		{
			name:     "127.0.0.1 with port - not matched",
			target:   "127.0.0.1:22",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLocalhost(tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewDiagnosticsHandler(t *testing.T) {
	// Setup
	logger := logrus.New()
	cfg := &config.Config{}

	// Execute
	handler := NewDiagnosticsHandler(nil, cfg, logger)

	// Assert
	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
	assert.Equal(t, logger, handler.logger)
}
