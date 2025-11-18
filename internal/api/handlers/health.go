package handlers

import (
	"net/http"
	"time"

	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/database"
	"github.com/sirupsen/logrus"
)

// HealthHandler handles health check requests
type HealthHandler struct{
	db     *database.DB
	logger *logrus.Logger
	startTime time.Time
}

// NewHealthHandler creates a new health check handler
func NewHealthHandler(db *database.DB, logger *logrus.Logger) *HealthHandler {
	return &HealthHandler{
		db:     db,
		logger: logger,
		startTime: time.Now(),
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Uptime    string                 `json:"uptime"`
	Version   string                 `json:"version"`
	Services  map[string]ServiceHealth `json:"services"`
}

// ServiceHealth represents individual service health
type ServiceHealth struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Health handles GET /api/v1/health
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	services := make(map[string]ServiceHealth)

	// Check database health
	dbStatus := ServiceHealth{Status: "healthy"}
	if h.db != nil {
		if err := h.db.Health(); err != nil {
			dbStatus.Status = "unhealthy"
			dbStatus.Message = err.Error()
			h.logger.WithError(err).Warn("Database health check failed")
		}
	} else {
		dbStatus.Status = "not_configured"
	}
	services["database"] = dbStatus

	// Determine overall status
	overallStatus := "healthy"
	for _, svc := range services {
		if svc.Status == "unhealthy" {
			overallStatus = "degraded"
			break
		}
	}

	healthResp := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Uptime:    time.Since(h.startTime).String(),
		Version:   "0.5.0", // TODO: Get from build info
		Services:  services,
	}

	// Return 200 if healthy, 503 if degraded
	if overallStatus == "healthy" {
		response.Success(w, healthResp)
	} else {
		response.JSON(w, http.StatusServiceUnavailable, healthResp)
	}
}

// Ready handles GET /api/v1/ready (readiness probe)
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	// Check if database is accessible
	if h.db != nil {
		if err := h.db.Health(); err != nil {
			response.ServiceUnavailable(w, "Database not ready")
			return
		}
	}

	response.Success(w, map[string]string{
		"status": "ready",
	})
}

// Live handles GET /api/v1/live (liveness probe)
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	response.Success(w, map[string]string{
		"status": "alive",
	})
}
