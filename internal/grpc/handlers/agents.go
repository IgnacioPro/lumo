package handlers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/database/models"
)

// AgentRepository defines the interface for agent data operations
type AgentRepository interface {
	Create(ctx context.Context, agent *models.Agent) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error)
	GetByHostname(ctx context.Context, hostname string) (*models.Agent, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*models.Agent, error)
	Update(ctx context.Context, agent *models.Agent) error
	UpdateHeartbeat(ctx context.Context, id uuid.UUID) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.AgentStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
	CountByStatus(ctx context.Context) (map[models.AgentStatus]int, error)
	MarkStaleAgentsOffline(ctx context.Context, threshold time.Duration) (int64, error)
}

// AgentsHandler implements the AgentsService gRPC service
type AgentsHandler struct {
	lumov1.UnimplementedAgentsServiceServer

	agentRepo AgentRepository
}

// NewAgentsHandler creates a new agents service handler
func NewAgentsHandler(agentRepo AgentRepository) *AgentsHandler {
	return &AgentsHandler{
		agentRepo: agentRepo,
	}
}

// RegisterAgent registers a new agent
func (h *AgentsHandler) RegisterAgent(ctx context.Context, req *lumov1.RegisterAgentRequest) (*lumov1.RegisterAgentResponse, error) {
	// Validate request
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "agent name is required")
	}
	if req.Hostname == "" {
		return nil, status.Error(codes.InvalidArgument, "hostname is required")
	}

	// Create agent model
	ipAddr := req.IpAddress
	labelsJSON, err := models.MapToJSONB(req.Labels)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert labels: %v", err)
	}

	agent := &models.Agent{
		ID:           uuid.New(),
		Name:         req.Name,
		Hostname:     req.Hostname,
		IPAddress:    &ipAddr,
		Platform:     models.AgentPlatform(req.Platform),
		Architecture: req.Architecture,
		Version:      req.Version,
		Status:       models.AgentStatusOnline,
		Capabilities: req.Capabilities,
		Labels:       labelsJSON,
	}

	// Save to database
	if err := h.agentRepo.Create(ctx, agent); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to register agent: %v", err)
	}

	return &lumov1.RegisterAgentResponse{
		AgentId:      agent.ID.String(),
		RegisteredAt: timestamppb.New(agent.RegisteredAt),
	}, nil
}

// SendHeartbeat updates agent heartbeat
func (h *AgentsHandler) SendHeartbeat(ctx context.Context, req *lumov1.SendHeartbeatRequest) (*lumov1.SendHeartbeatResponse, error) {
	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent ID")
	}

	// Update heartbeat
	if err := h.agentRepo.UpdateHeartbeat(ctx, agentID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update heartbeat: %v", err)
	}

	return &lumov1.SendHeartbeatResponse{
		Acknowledged: true,
		ServerTime:   timestamppb.Now(),
	}, nil
}

// ListAgents lists all registered agents with optional filtering
func (h *AgentsHandler) ListAgents(ctx context.Context, req *lumov1.ListAgentsRequest) (*lumov1.ListAgentsResponse, error) {
	// Set defaults
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}
	offset := int(req.Offset)

	// Build filters
	filters := map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	}

	// Add status filter if provided
	if req.Status != "" {
		filters["status"] = models.AgentStatus(req.Status)
	}

	// List agents from database
	agents, err := h.agentRepo.List(ctx, filters)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list agents: %v", err)
	}

	// Convert to proto
	protoAgents := make([]*lumov1.Agent, len(agents))
	for i, agent := range agents {
		protoAgents[i] = toProtoAgent(agent)
	}

	return &lumov1.ListAgentsResponse{
		Agents:     protoAgents,
		TotalCount: int32(len(agents)),
	}, nil
}

// GetAgent retrieves a specific agent by ID
func (h *AgentsHandler) GetAgent(ctx context.Context, req *lumov1.GetAgentRequest) (*lumov1.GetAgentResponse, error) {
	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent ID")
	}

	agent, err := h.agentRepo.GetByID(ctx, agentID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "agent not found: %v", err)
	}

	return &lumov1.GetAgentResponse{
		Agent: toProtoAgent(agent),
	}, nil
}

// DeleteAgent removes an agent registration
func (h *AgentsHandler) DeleteAgent(ctx context.Context, req *lumov1.DeleteAgentRequest) (*lumov1.DeleteAgentResponse, error) {
	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent ID")
	}

	if err := h.agentRepo.Delete(ctx, agentID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete agent: %v", err)
	}

	return &lumov1.DeleteAgentResponse{
		Success: true,
	}, nil
}

// GetAgentStats returns aggregate agent statistics
func (h *AgentsHandler) GetAgentStats(ctx context.Context, req *lumov1.GetAgentStatsRequest) (*lumov1.GetAgentStatsResponse, error) {
	// Get counts by status
	statusCounts, err := h.agentRepo.CountByStatus(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get agent stats: %v", err)
	}

	// Calculate totals
	totalAgents := 0
	onlineAgents := statusCounts[models.AgentStatusOnline]
	offlineAgents := statusCounts[models.AgentStatusOffline]
	errorAgents := statusCounts[models.AgentStatusError]

	for _, count := range statusCounts {
		totalAgents += count
	}

	return &lumov1.GetAgentStatsResponse{
		TotalAgents:   int32(totalAgents),
		OnlineAgents:  int32(onlineAgents),
		OfflineAgents: int32(offlineAgents),
		ErrorAgents:   int32(errorAgents),
		ByPlatform:    make(map[string]int32), // TODO: Add platform stats
	}, nil
}

// Helper functions

func toProtoAgent(agent *models.Agent) *lumov1.Agent {
	ipAddress := ""
	if agent.IPAddress != nil {
		ipAddress = *agent.IPAddress
	}

	labels := make(map[string]string)
	if agent.Labels != nil {
		labelsMap, err := models.JSONBToMap(agent.Labels)
		if err == nil {
			labels = labelsMap
		}
	}

	return &lumov1.Agent{
		Id:               agent.ID.String(),
		Name:             agent.Name,
		Hostname:         agent.Hostname,
		IpAddress:        ipAddress,
		Platform:         string(agent.Platform),
		Architecture:     agent.Architecture,
		Version:          agent.Version,
		Status:           string(agent.Status),
		Capabilities:     agent.Capabilities,
		Labels:           labels,
		LastHeartbeatAt:  timestamppb.New(agent.LastHeartbeatAt),
		RegisteredAt:     timestamppb.New(agent.RegisteredAt),
	}
}
