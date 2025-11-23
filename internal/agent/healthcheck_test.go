package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestHealthCheck_Ready(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Suppress logs during test

	t.Run("Ready immediately after initialization", func(t *testing.T) {
		// Initialize HealthCheck
		hc := NewHealthCheck(8080, nil, logger)

		// Create request
		req := httptest.NewRequest("GET", "/ready", nil)
		w := httptest.NewRecorder()

		// Call handler
		hc.readyHandler(w, req)

		// Check response
		resp := w.Result()
		defer func() {
			_ = resp.Body.Close()
		}()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]interface{}
		err := json.NewDecoder(resp.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, true, body["ready"])
	})

	t.Run("Not ready if unhealthy", func(t *testing.T) {
		hc := NewHealthCheck(8080, nil, logger)
		hc.UpdateStatus(HealthStatusUnhealthy, []string{"error"})

		req := httptest.NewRequest("GET", "/ready", nil)
		w := httptest.NewRecorder()

		hc.readyHandler(w, req)

		resp := w.Result()
		defer func() {
			_ = resp.Body.Close()
		}()

		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	})
}
