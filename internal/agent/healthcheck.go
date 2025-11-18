package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HealthStatus represents the health status of the agent
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck provides HTTP endpoints for health monitoring
type HealthCheck struct {
	port    int
	server  *http.Server
	logger  *logrus.Logger
	agent   *Agent
	mu      sync.RWMutex
	status  HealthStatus
	lastRun time.Time
	errors  []string
}

// NewHealthCheck creates a new health check server
func NewHealthCheck(port int, agent *Agent, logger *logrus.Logger) *HealthCheck {
	hc := &HealthCheck{
		port:   port,
		logger: logger,
		agent:  agent,
		status: HealthStatusHealthy,
	}

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", hc.healthHandler)
	mux.HandleFunc("/ready", hc.readyHandler)
	mux.HandleFunc("/live", hc.liveHandler)
	mux.HandleFunc("/status", hc.statusHandler)

	hc.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return hc
}

// Start starts the health check server
func (hc *HealthCheck) Start() error {
	hc.logger.WithField("port", hc.port).Info("Starting health check server")

	go func() {
		if err := hc.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			hc.logger.WithError(err).Error("Health check server failed")
		}
	}()

	return nil
}

// Stop stops the health check server
func (hc *HealthCheck) Stop() error {
	hc.logger.Info("Stopping health check server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := hc.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown health check server: %w", err)
	}

	return nil
}

// UpdateStatus updates the health status
func (hc *HealthCheck) UpdateStatus(status HealthStatus, errors []string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.status = status
	hc.lastRun = time.Now()
	hc.errors = errors
}

// healthHandler handles GET /health
func (hc *HealthCheck) healthHandler(w http.ResponseWriter, r *http.Request) {
	hc.mu.RLock()
	status := hc.status
	hc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if status == HealthStatusHealthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	response := map[string]interface{}{
		"status": status,
		"time":   time.Now().Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		hc.logger.WithError(err).Error("Failed to encode health response")
	}
}

// readyHandler handles GET /ready (readiness probe)
func (hc *HealthCheck) readyHandler(w http.ResponseWriter, r *http.Request) {
	hc.mu.RLock()
	status := hc.status
	lastRun := hc.lastRun
	hc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	// Consider ready if healthy and has run recently (within 10 minutes)
	ready := status == HealthStatusHealthy && time.Since(lastRun) < 10*time.Minute

	if ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	response := map[string]interface{}{
		"ready":    ready,
		"status":   status,
		"last_run": lastRun.Format(time.RFC3339),
		"time":     time.Now().Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		hc.logger.WithError(err).Error("Failed to encode ready response")
	}
}

// liveHandler handles GET /live (liveness probe)
func (hc *HealthCheck) liveHandler(w http.ResponseWriter, r *http.Request) {
	// Agent is alive if the HTTP server is responding
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"alive": true,
		"time":  time.Now().Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		hc.logger.WithError(err).Error("Failed to encode live response")
	}
}

// statusHandler handles GET /status (detailed status)
func (hc *HealthCheck) statusHandler(w http.ResponseWriter, r *http.Request) {
	hc.mu.RLock()
	status := hc.status
	lastRun := hc.lastRun
	errors := make([]string, len(hc.errors))
	copy(errors, hc.errors)
	hc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Get agent info
	var agentInfo map[string]interface{}
	if hc.agent != nil {
		agentInfo = map[string]interface{}{
			"id":       hc.agent.ID,
			"hostname": hc.agent.Hostname,
			"mode":     hc.agent.Mode,
		}
	}

	response := map[string]interface{}{
		"status":   status,
		"last_run": lastRun.Format(time.RFC3339),
		"uptime":   time.Since(hc.agent.StartTime).String(),
		"errors":   errors,
		"agent":    agentInfo,
		"time":     time.Now().Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		hc.logger.WithError(err).Error("Failed to encode status response")
	}
}
