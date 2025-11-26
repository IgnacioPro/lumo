package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ignacio/lumo/internal/config"
)

// TestNewReporter tests the Reporter constructor
func TestNewReporter(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *config.AgentConfig
		expectInsecure bool
	}{
		{
			name: "secure TLS by default",
			cfg: &config.AgentConfig{
				APIEndpoint: "https://api.lumo.example.com",
				TLSInsecure: false,
			},
			expectInsecure: false,
		},
		{
			name: "insecure TLS when configured",
			cfg: &config.AgentConfig{
				APIEndpoint: "https://api.lumo.example.com",
				TLSInsecure: true,
			},
			expectInsecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			logger.SetOutput(logrus.StandardLogger().Out)

			reporter := NewReporter(tt.cfg, logger)

			assert.NotNil(t, reporter)
			assert.NotNil(t, reporter.httpClient)
			assert.NotNil(t, reporter.logger)
			assert.Equal(t, tt.cfg, reporter.cfg)

			// Verify TLS configuration
			transport := reporter.httpClient.Transport.(*http.Transport)
			assert.Equal(t, tt.expectInsecure, transport.TLSClientConfig.InsecureSkipVerify)

			// Verify timeout
			assert.Equal(t, 30*time.Second, reporter.httpClient.Timeout)
		})
	}
}

// TestSetAgentID tests setting the agent ID
func TestSetAgentID(t *testing.T) {
	cfg := &config.AgentConfig{APIEndpoint: "https://api.example.com"}
	reporter := NewReporter(cfg, logrus.New())

	agentID := uuid.New()
	reporter.SetAgentID(agentID)

	assert.Equal(t, agentID, reporter.agentID)
}

// TestRegisterAgent tests agent registration
func TestRegisterAgent(t *testing.T) {
	tests := []struct {
		name          string
		request       RegisterAgentRequest
		serverStatus  int
		serverResp    interface{}
		expectError   bool
		errorContains string
	}{
		{
			name: "successful registration",
			request: RegisterAgentRequest{
				Name:         "test-agent",
				Hostname:     "test-host",
				Platform:     "linux",
				Architecture: "amd64",
				Version:      "1.0.0",
				Capabilities: []string{"diagnostics", "remediation"},
			},
			serverStatus: http.StatusOK,
			serverResp: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"agent_id":      uuid.New().String(),
					"name":          "test-agent",
					"hostname":      "test-host",
					"status":        "active",
					"registered_at": time.Now().Format(time.RFC3339),
				},
			},
			expectError: false,
		},
		{
			name: "registration with Kubernetes metadata",
			request: RegisterAgentRequest{
				Name:         "k8s-agent",
				Hostname:     "k8s-node-1",
				Platform:     "linux",
				Architecture: "amd64",
				Version:      "1.0.0",
				KubernetesMetadata: &KubernetesMetadata{
					Cluster:   "prod-cluster",
					Namespace: "lumo-system",
					NodeName:  "node-1",
					PodName:   "lumo-agent-abc123",
				},
			},
			serverStatus: http.StatusOK,
			serverResp: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"agent_id":      uuid.New().String(),
					"name":          "k8s-agent",
					"hostname":      "k8s-node-1",
					"status":        "active",
					"registered_at": time.Now().Format(time.RFC3339),
				},
			},
			expectError: false,
		},
		{
			name: "server error - internal server error",
			request: RegisterAgentRequest{
				Name:     "test-agent",
				Hostname: "test-host",
			},
			serverStatus:  http.StatusInternalServerError,
			serverResp:    map[string]interface{}{"error": "internal server error"},
			expectError:   true,
			errorContains: "failed to register agent",
		},
		{
			name: "client error - bad request",
			request: RegisterAgentRequest{
				Name: "", // Invalid empty name
			},
			serverStatus:  http.StatusBadRequest,
			serverResp:    map[string]interface{}{"error": "invalid request"},
			expectError:   true,
			errorContains: "API returned status 400",
		},
		{
			name: "invalid agent ID in response",
			request: RegisterAgentRequest{
				Name:     "test-agent",
				Hostname: "test-host",
			},
			serverStatus: http.StatusOK,
			serverResp: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"agent_id":      "invalid-uuid",
					"name":          "test-agent",
					"hostname":      "test-host",
					"status":        "active",
					"registered_at": time.Now().Format(time.RFC3339),
				},
			},
			expectError:   true,
			errorContains: "invalid agent ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/agents/register", r.URL.Path)
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Verify request body
				var reqBody RegisterAgentRequest
				err := json.NewDecoder(r.Body).Decode(&reqBody)
				require.NoError(t, err)
				assert.Equal(t, tt.request.Name, reqBody.Name)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.serverStatus)
				_ = json.NewEncoder(w).Encode(tt.serverResp)
			}))
			defer server.Close()

			cfg := &config.AgentConfig{
				APIEndpoint:      server.URL,
				RetryMaxAttempts: 1,
				RetryBaseDelay:   1 * time.Millisecond,
			}
			reporter := NewReporter(cfg, logrus.New())

			ctx := context.Background()
			resp, err := reporter.RegisterAgent(ctx, tt.request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.AgentID)
				assert.Equal(t, tt.request.Name, resp.Name)
				assert.NotEqual(t, uuid.Nil, reporter.agentID)
			}
		})
	}
}

// TestSendHeartbeat tests heartbeat sending
func TestSendHeartbeat(t *testing.T) {
	tests := []struct {
		name          string
		setupReporter func(*Reporter)
		serverStatus  int
		expectError   bool
		errorContains string
	}{
		{
			name: "successful heartbeat",
			setupReporter: func(r *Reporter) {
				r.SetAgentID(uuid.New())
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name: "missing agent ID",
			setupReporter: func(r *Reporter) {
				// Don't set agent ID
			},
			serverStatus:  http.StatusOK,
			expectError:   true,
			errorContains: "agent ID not set",
		},
		{
			name: "server error",
			setupReporter: func(r *Reporter) {
				r.SetAgentID(uuid.New())
			},
			serverStatus:  http.StatusInternalServerError,
			expectError:   true,
			errorContains: "failed to send heartbeat",
		},
		{
			name: "unauthorized",
			setupReporter: func(r *Reporter) {
				r.SetAgentID(uuid.New())
			},
			serverStatus:  http.StatusUnauthorized,
			expectError:   true,
			errorContains: "API returned status 401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, "/api/v1/agents/")
				assert.Contains(t, r.URL.Path, "/heartbeat")
				assert.Equal(t, "PUT", r.Method)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.serverStatus)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"data":    map[string]interface{}{"status": "ok"},
				})
			}))
			defer server.Close()

			cfg := &config.AgentConfig{
				APIEndpoint:      server.URL,
				RetryMaxAttempts: 1,
				RetryBaseDelay:   1 * time.Millisecond,
			}
			reporter := NewReporter(cfg, logrus.New())
			tt.setupReporter(reporter)

			ctx := context.Background()
			err := reporter.SendHeartbeat(ctx)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSubmitDiagnosticResult tests diagnostic result submission
func TestSubmitDiagnosticResult(t *testing.T) {
	tests := []struct {
		name          string
		result        DiagnosticResult
		serverStatus  int
		expectError   bool
		errorContains string
	}{
		{
			name: "successful submission",
			result: DiagnosticResult{
				Target:  "localhost",
				Checks:  []string{"cpu", "memory", "disk"},
				Format:  "toon",
				Analyze: true,
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name: "submission without analysis",
			result: DiagnosticResult{
				Target:  "192.168.1.100",
				Checks:  []string{"network"},
				Format:  "json",
				Analyze: false,
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name: "server error",
			result: DiagnosticResult{
				Target: "localhost",
				Checks: []string{"cpu"},
			},
			serverStatus:  http.StatusInternalServerError,
			expectError:   true,
			errorContains: "failed to submit diagnostic result",
		},
		{
			name: "bad request",
			result: DiagnosticResult{
				Target: "", // Invalid empty target
				Checks: []string{},
			},
			serverStatus:  http.StatusBadRequest,
			expectError:   true,
			errorContains: "API returned status 400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/diagnostics", r.URL.Path)
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Verify request body
				var reqBody DiagnosticResult
				err := json.NewDecoder(r.Body).Decode(&reqBody)
				require.NoError(t, err)
				assert.Equal(t, tt.result.Target, reqBody.Target)
				assert.Equal(t, tt.result.Checks, reqBody.Checks)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.serverStatus)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"data":    map[string]interface{}{"job_id": uuid.New().String()},
				})
			}))
			defer server.Close()

			cfg := &config.AgentConfig{
				APIEndpoint:      server.URL,
				RetryMaxAttempts: 1,
				RetryBaseDelay:   1 * time.Millisecond,
			}
			reporter := NewReporter(cfg, logrus.New())

			ctx := context.Background()
			err := reporter.SubmitDiagnosticResult(ctx, tt.result)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestDoWithRetry tests the retry logic
func TestDoWithRetry(t *testing.T) {
	tests := []struct {
		name           string
		maxAttempts    int
		serverBehavior func() func(http.ResponseWriter, *http.Request)
		expectError    bool
		errorContains  string
		expectAttempts int
	}{
		{
			name:        "success on first attempt",
			maxAttempts: 3,
			serverBehavior: func() func(http.ResponseWriter, *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": true,
						"data":    map[string]string{"status": "ok"},
					})
				}
			},
			expectError:    false,
			expectAttempts: 1,
		},
		{
			name:        "success after retry",
			maxAttempts: 3,
			serverBehavior: func() func(http.ResponseWriter, *http.Request) {
				attempts := 0
				return func(w http.ResponseWriter, r *http.Request) {
					attempts++
					if attempts < 2 {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": true,
						"data":    map[string]string{"status": "ok"},
					})
				}
			},
			expectError:    false,
			expectAttempts: 2,
		},
		{
			name:        "4xx error - no retry",
			maxAttempts: 3,
			serverBehavior: func() func(http.ResponseWriter, *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error": "bad request",
					})
				}
			},
			expectError:    true,
			errorContains:  "API returned status 400",
			expectAttempts: 1,
		},
		{
			name:        "5xx error - retry exhausted",
			maxAttempts: 2,
			serverBehavior: func() func(http.ResponseWriter, *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error": "internal server error",
					})
				}
			},
			expectError:    true,
			errorContains:  "request failed after 2 attempts",
			expectAttempts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempts := 0
			handler := tt.serverBehavior()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				handler(w, r)
			}))
			defer server.Close()

			cfg := &config.AgentConfig{
				APIEndpoint:      server.URL,
				RetryMaxAttempts: tt.maxAttempts,
				RetryBaseDelay:   1 * time.Millisecond,
			}
			reporter := NewReporter(cfg, logrus.New())

			ctx := context.Background()
			var respData map[string]interface{}
			err := reporter.doWithRetry(ctx, "GET", fmt.Sprintf("%s/test", server.URL), nil, &respData)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, respData)
			}

			assert.Equal(t, tt.expectAttempts, attempts)
		})
	}
}

// TestIsAvailable tests API availability checking
func TestIsAvailable(t *testing.T) {
	tests := []struct {
		name         string
		serverStatus int
		expectResult bool
	}{
		{
			name:         "API available",
			serverStatus: http.StatusOK,
			expectResult: true,
		},
		{
			name:         "API unavailable - 500",
			serverStatus: http.StatusInternalServerError,
			expectResult: false,
		},
		{
			name:         "API unavailable - 404",
			serverStatus: http.StatusNotFound,
			expectResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/health", r.URL.Path)
				assert.Equal(t, "GET", r.Method)
				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			cfg := &config.AgentConfig{APIEndpoint: server.URL}
			reporter := NewReporter(cfg, logrus.New())

			ctx := context.Background()
			available := reporter.IsAvailable(ctx)

			assert.Equal(t, tt.expectResult, available)
		})
	}
}

// TestIsAvailable_NetworkError tests API availability with network errors
func TestIsAvailable_NetworkError(t *testing.T) {
	cfg := &config.AgentConfig{
		APIEndpoint: "http://invalid-host-that-does-not-exist:12345",
	}
	reporter := NewReporter(cfg, logrus.New())

	ctx := context.Background()
	available := reporter.IsAvailable(ctx)

	assert.False(t, available)
}

// TestReporterWithAuthentication tests API calls with authentication token
func TestReporterWithAuthentication(t *testing.T) {
	expectedToken := "test-api-token"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authentication header
		token := r.Header.Get("X-API-Key")
		assert.Equal(t, expectedToken, token)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]interface{}{"status": "ok"},
		})
	}))
	defer server.Close()

	cfg := &config.AgentConfig{
		APIEndpoint:      server.URL,
		Token:            expectedToken,
		RetryMaxAttempts: 1,
		RetryBaseDelay:   1 * time.Millisecond,
	}
	reporter := NewReporter(cfg, logrus.New())
	reporter.SetAgentID(uuid.New())

	ctx := context.Background()
	err := reporter.SendHeartbeat(ctx)

	assert.NoError(t, err)
}

// TestReporterContextCancellation tests context cancellation handling
func TestReporterContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]interface{}{"status": "ok"},
		})
	}))
	defer server.Close()

	cfg := &config.AgentConfig{
		APIEndpoint:      server.URL,
		RetryMaxAttempts: 1,
		RetryBaseDelay:   1 * time.Millisecond,
	}
	reporter := NewReporter(cfg, logrus.New())
	reporter.SetAgentID(uuid.New())

	// Create context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := reporter.SendHeartbeat(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}
