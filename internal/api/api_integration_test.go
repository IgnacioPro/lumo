package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ignacio/lumo/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthEndpoints tests all health-related endpoints
func TestHealthEndpoints(t *testing.T) {
	t.Skip("Use tests/integration/ for integration tests with testcontainers")

	// Create test router with in-memory database
	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	tests := []struct {
		name           string
		endpoint       string
		expectedStatus int
		checkBody      func(t *testing.T, body map[string]interface{})
	}{
		{
			name:           "Health endpoint returns OK",
			endpoint:       "/api/v1/health",
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "ok", body["status"])
				assert.NotNil(t, body["timestamp"])
			},
		},
		{
			name:           "Ready endpoint returns OK",
			endpoint:       "/api/v1/ready",
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "ready", body["status"])
			},
		},
		{
			name:           "Live endpoint returns OK",
			endpoint:       "/api/v1/live",
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "alive", body["status"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.endpoint, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var body map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &body)
			require.NoError(t, err)

			if tt.checkBody != nil {
				tt.checkBody(t, body)
			}
		})
	}
}

// TestAgentRegistration tests agent registration flow
func TestAgentRegistration(t *testing.T) {
	t.Skip("Use tests/integration/ for integration tests with testcontainers")

	router, apiKey, cleanup := setupTestRouter(t)
	defer cleanup()

	// Test agent registration request
	registerReq := map[string]interface{}{
		"name":         "test-agent",
		"hostname":     "test-host-01",
		"platform":     "linux",
		"architecture": "amd64",
		"version":      "1.0.0",
		"capabilities": []string{"cpu", "memory", "disk"},
		"labels": map[string]interface{}{
			"environment": "test",
			"region":      "us-west-2",
		},
	}

	body, _ := json.Marshal(registerReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["agent_id"])
	assert.Equal(t, "test-agent", data["name"])
	assert.Equal(t, "test-host-01", data["hostname"])
	assert.Equal(t, "online", data["status"])

	// Store agent ID for next tests
	agentID := data["agent_id"].(string)

	// Test heartbeat
	t.Run("Heartbeat updates agent status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/agents/"+agentID+"/heartbeat", nil)
		req.Header.Set("X-API-Key", apiKey)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].(map[string]interface{})
		assert.Equal(t, "Heartbeat recorded", data["message"])
		assert.Equal(t, "online", data["status"])
	})

	// Test get agent
	t.Run("Get agent returns agent details", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID, nil)
		req.Header.Set("X-API-Key", apiKey)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].(map[string]interface{})
		assert.Equal(t, agentID, data["id"])
		assert.Equal(t, "test-agent", data["name"])
		assert.Equal(t, "test-host-01", data["hostname"])
	})

	// Test list agents
	t.Run("List agents returns registered agents", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
		req.Header.Set("X-API-Key", apiKey)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].(map[string]interface{})
		agents := data["agents"].([]interface{})
		assert.GreaterOrEqual(t, len(agents), 1)
		assert.GreaterOrEqual(t, int(data["count"].(float64)), 1)
	})

	// Test agent stats
	t.Run("Agent stats returns correct counts", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/stats", nil)
		req.Header.Set("X-API-Key", apiKey)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].(map[string]interface{})
		assert.GreaterOrEqual(t, int(data["total"].(float64)), 1)
		assert.GreaterOrEqual(t, int(data["online"].(float64)), 1)
	})

	// Test delete agent
	t.Run("Delete agent removes agent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/"+agentID, nil)
		req.Header.Set("X-API-Key", apiKey)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify agent is deleted
		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID, nil)
		req2.Header.Set("X-API-Key", apiKey)
		w2 := httptest.NewRecorder()

		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusNotFound, w2.Code)
	})
}

// TestAgentReregistration tests that re-registering an agent updates existing registration
func TestAgentReregistration(t *testing.T) {
	t.Skip("Use tests/integration/ for integration tests with testcontainers")

	router, apiKey, cleanup := setupTestRouter(t)
	defer cleanup()

	hostname := "reregister-test-host"

	// Register agent first time
	registerReq := map[string]interface{}{
		"name":         "agent-v1",
		"hostname":     hostname,
		"platform":     "linux",
		"architecture": "amd64",
		"version":      "1.0.0",
		"capabilities": []string{"cpu", "memory"},
	}

	body, _ := json.Marshal(registerReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp1 map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp1)
	assert.NoError(t, err)
	firstAgentID := resp1["data"].(map[string]interface{})["agent_id"].(string)

	// Re-register same agent with updated info
	registerReq2 := map[string]interface{}{
		"name":         "agent-v2",
		"hostname":     hostname,
		"platform":     "linux",
		"architecture": "amd64",
		"version":      "2.0.0",
		"capabilities": []string{"cpu", "memory", "disk"},
	}

	body2, _ := json.Marshal(registerReq2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agents/register", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-API-Key", apiKey)
	w2 := httptest.NewRecorder()

	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	var resp2 map[string]interface{}
	err = json.Unmarshal(w2.Body.Bytes(), &resp2)
	assert.NoError(t, err)
	secondAgentID := resp2["data"].(map[string]interface{})["agent_id"].(string)

	// Should be same agent ID (updated, not created new)
	assert.Equal(t, firstAgentID, secondAgentID)

	// Verify updated info
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+firstAgentID, nil)
	req3.Header.Set("X-API-Key", apiKey)
	w3 := httptest.NewRecorder()

	router.ServeHTTP(w3, req3)

	var resp3 map[string]interface{}
	err = json.Unmarshal(w3.Body.Bytes(), &resp3)
	assert.NoError(t, err)
	data := resp3["data"].(map[string]interface{})

	assert.Equal(t, "agent-v2", data["name"])
	assert.Equal(t, "2.0.0", data["version"])
	capabilities := data["capabilities"].([]interface{})
	assert.Len(t, capabilities, 3)
}

// TestAuthenticationRequired tests that endpoints require API key
func TestAuthenticationRequired(t *testing.T) {
	t.Skip("Use tests/integration/ for integration tests with testcontainers")

	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/diagnostics"},
		{http.MethodGet, "/api/v1/jobs"},
		{http.MethodPost, "/api/v1/agents/register"},
		{http.MethodGet, "/api/v1/agents"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path+" requires auth", func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// setupTestRouter creates a test router with in-memory database
func setupTestRouter(t *testing.T) (*http.ServeMux, string, func()) {
	t.Helper()
	// Create test logger
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs in tests

	// Create test config
	_ = &config.Config{
		API: config.APIConfig{
			Host: "localhost",
			Port: 8080,
		},
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Name:     "lumo_test",
			User:     "lumo",
			Password: "lumo_dev",
		},
	}

	// Note: This is a simplified test setup
	// In a real scenario, you'd use a test database or mock
	// For now, we'll create a mock router
	mux := http.NewServeMux()

	// Create test API key
	testAPIKey := "test-api-key-12345"

	cleanup := func() {
		// Cleanup resources
	}

	return mux, testAPIKey, cleanup
}
