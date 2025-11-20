package handlers

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
)

// AgentsHandler implements the AgentsService gRPC service
type AgentsHandler struct {
	lumov1.UnimplementedAgentsServiceServer

	agentRepo repository.AgentRepository
}

// NewAgentsHandler creates a new agents service handler
func NewAgentsHandler(agentRepo repository.AgentRepository) *AgentsHandler {
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
	agent := &models.Agent{
		ID:           uuid.New(),
		Name:         req.Name,
		Hostname:     req.Hostname,
		IPAddress:    req.IpAddress,
		Platform:     req.Platform,
		Architecture: req.Architecture,
		Version:      req.Version,
		Status:       "online",
		Capabilities: req.Capabilities,
		Labels:       req.Labels,
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

	// List agents from database
	agents, err := h.agentRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list agents: %v", err)
	}

	// Convert to proto
	protoAgents := make([]*lumov1.Agent, len(agents))
	for i, agent := range agents {
		protoAgents[i] = toProtoAgent(agent)
	}

	// Get total count
	totalCount, err := h.agentRepo.Count(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to count agents: %v", err)
	}

	return &lumov1.ListAgentsResponse{
		Agents:     protoAgents,
		TotalCount: int32(totalCount),
	}, nil
}

// GetAgent retrieves a specific agent by ID
func (h *AgentsHandler) GetAgent(ctx context.Context, req *lumov1.GetAgentRequest) (*lumov1.GetAgentResponse, error) {
	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid agent ID")
	}

	agent, err := h.agentRepo.Get(ctx, agentID)
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
	stats, err := h.agentRepo.GetStats(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get agent stats: %v", err)
	}

	return &lumov1.GetAgentStatsResponse{
		TotalAgents:   int32(stats.TotalAgents),
		OnlineAgents:  int32(stats.OnlineAgents),
		OfflineAgents: int32(stats.OfflineAgents),
		ErrorAgents:   int32(stats.ErrorAgents),
		ByPlatform:    stats.ByPlatform,
	}, nil
}

// Helper functions

func toProtoAgent(agent *models.Agent) *lumov1.Agent {
	return &lumov1.Agent{
		Id:               agent.ID.String(),
		Name:             agent.Name,
		Hostname:         agent.Hostname,
		IpAddress:        agent.IPAddress,
		Platform:         agent.Platform,
		Architecture:     agent.Architecture,
		Version:          agent.Version,
		Status:           agent.Status,
		Capabilities:     agent.Capabilities,
		Labels:           agent.Labels,
		LastHeartbeatAt:  timestamppb.New(agent.LastHeartbeatAt),
		RegisteredAt:     timestamppb.New(agent.RegisteredAt),
	}
}
