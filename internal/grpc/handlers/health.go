package handlers

import (
	"context"
	"database/sql"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/version"
)

// HealthHandler implements the HealthService gRPC service
type HealthHandler struct {
	lumov1.UnimplementedHealthServiceServer

	cfg       *config.Config
	db        *sql.DB
	startTime time.Time
}

// NewHealthHandler creates a new health service handler
func NewHealthHandler(cfg *config.Config, db *sql.DB) *HealthHandler {
	return &HealthHandler{
		cfg:       cfg,
		db:        db,
		startTime: time.Now(),
	}
}

// Check performs a comprehensive health check
func (h *HealthHandler) Check(ctx context.Context, req *lumov1.HealthCheckRequest) (*lumov1.HealthCheckResponse, error) {
	resp := &lumov1.HealthCheckResponse{
		Status:     lumov1.HealthCheckResponse_SERVING_STATUS_SERVING,
		Version:    version.Version,
		Uptime:     timestamppb.New(h.startTime),
		Components: make(map[string]*lumov1.ComponentHealth),
	}

	// Check database connection
	if h.db != nil {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		if err := h.db.PingContext(ctx); err != nil {
			resp.Components["database"] = &lumov1.ComponentHealth{
				Healthy: false,
				Message: err.Error(),
			}
			resp.Status = lumov1.HealthCheckResponse_SERVING_STATUS_NOT_SERVING
		} else {
			resp.Components["database"] = &lumov1.ComponentHealth{
				Healthy: true,
				Message: "connected",
			}
		}
	}

	// Add more component checks as needed
	resp.Components["api"] = &lumov1.ComponentHealth{
		Healthy: true,
		Message: "operational",
	}

	return resp, nil
}

// Ready implements Kubernetes readiness probe
func (h *HealthHandler) Ready(ctx context.Context, req *lumov1.ReadyRequest) (*lumov1.ReadyResponse, error) {
	// Check critical dependencies (database, cache, etc.)
	ready := true
	message := "ready"

	if h.db != nil {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()

		if err := h.db.PingContext(ctx); err != nil {
			ready = false
			message = "database not ready: " + err.Error()
		}
	}

	return &lumov1.ReadyResponse{
		Ready:   ready,
		Message: message,
	}, nil
}

// Live implements Kubernetes liveness probe
func (h *HealthHandler) Live(ctx context.Context, req *lumov1.LiveRequest) (*lumov1.LiveResponse, error) {
	// Simple liveness check - if we can respond, we're alive
	return &lumov1.LiveResponse{
		Alive:     true,
		Timestamp: timestamppb.Now(),
	}, nil
}
