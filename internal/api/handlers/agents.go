package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
	"github.com/sirupsen/logrus"
)

// AgentsHandler handles agent-related requests
type AgentsHandler struct {
	agentRepo *repository.AgentRepository
	logger    *logrus.Logger
}

// NewAgentsHandler creates a new agents handler
func NewAgentsHandler(agentRepo *repository.AgentRepository, logger *logrus.Logger) *AgentsHandler {
	return &AgentsHandler{
		agentRepo: agentRepo,
		logger:    logger,
	}
}

// RegisterRequest represents an agent registration request
type RegisterRequest struct {
	Name               string                     `json:"name"`
	Hostname           string                     `json:"hostname"`
	IPAddress          string                     `json:"ip_address,omitempty"`
	Platform           models.AgentPlatform       `json:"platform"`
	Architecture       string                     `json:"architecture"`
	Version            string                     `json:"version"`
	Capabilities       []string                   `json:"capabilities"`
	Labels             map[string]interface{}     `json:"labels,omitempty"`
	KubernetesMetadata *models.KubernetesMetadata `json:"kubernetes_metadata,omitempty"`
}

// RegisterResponse represents the response to an agent registration
type RegisterResponse struct {
	AgentID      string             `json:"agent_id"`
	Name         string             `json:"name"`
	Hostname     string             `json:"hostname"`
	Status       models.AgentStatus `json:"status"`
	RegisteredAt string             `json:"registered_at"`
}

// Register handles POST /api/v1/agents/register
func (h *AgentsHandler) Register(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	// Debug: Log the parsed request
	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	h.logger.WithField("request", string(reqJSON)).Debug("Received registration request")

	// Validate required fields
	if req.Name == "" {
		response.BadRequest(w, "Name is required")
		return
	}
	if req.Hostname == "" {
		response.BadRequest(w, "Hostname is required")
		return
	}
	if req.Platform == "" {
		response.BadRequest(w, "Platform is required")
		return
	}
	if req.Architecture == "" {
		response.BadRequest(w, "Architecture is required")
		return
	}
	if req.Version == "" {
		response.BadRequest(w, "Version is required")
		return
	}

	// Validate platform
	validPlatforms := map[models.AgentPlatform]bool{
		models.AgentPlatformLinux:      true,
		models.AgentPlatformDarwin:     true,
		models.AgentPlatformWindows:    true,
		models.AgentPlatformKubernetes: true,
	}
	if !validPlatforms[req.Platform] {
		response.BadRequest(w, "Invalid platform")
		return
	}

	// Check if agent with this hostname already exists
	existingAgent, err := h.agentRepo.GetByHostname(r.Context(), req.Hostname)
	if err == nil && existingAgent != nil {
		// Agent already registered - update it instead
		h.logger.WithField("hostname", req.Hostname).Info("Agent already registered, updating")

		existingAgent.Name = req.Name
		if req.IPAddress != "" {
			existingAgent.IPAddress = &req.IPAddress
		}
		existingAgent.Version = req.Version
		existingAgent.Status = models.AgentStatusOnline
		existingAgent.Capabilities = req.Capabilities

		// Ensure labels is not nil
		labels := req.Labels
		if labels == nil {
			labels = make(map[string]interface{})
		}
		existingAgent.Labels = models.JSONB(labels)

		// Sanitize Kubernetes metadata - set to nil if all fields are empty
		k8sMetadata := req.KubernetesMetadata
		if k8sMetadata != nil && k8sMetadata.Cluster == "" && k8sMetadata.Namespace == "" && k8sMetadata.NodeName == "" && k8sMetadata.PodName == "" {
			k8sMetadata = nil
		}
		existingAgent.KubernetesMetadata = k8sMetadata

		if err := h.agentRepo.Update(r.Context(), existingAgent); err != nil {
			h.logger.WithError(err).Error("Failed to update existing agent")
			response.InternalServerError(w, "Failed to update agent registration")
			return
		}

		// Also update heartbeat
		if err := h.agentRepo.UpdateHeartbeat(r.Context(), existingAgent.ID); err != nil {
			h.logger.WithError(err).Error("Failed to update heartbeat")
		}

		resp := RegisterResponse{
			AgentID:      existingAgent.ID.String(),
			Name:         existingAgent.Name,
			Hostname:     existingAgent.Hostname,
			Status:       existingAgent.Status,
			RegisteredAt: existingAgent.RegisteredAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		response.Success(w, resp)
		return
	}

	// Ensure labels is not nil
	labels := req.Labels
	if labels == nil {
		labels = make(map[string]interface{})
	}

	// Sanitize Kubernetes metadata - set to nil if all fields are empty
	k8sMetadata := req.KubernetesMetadata
	if k8sMetadata != nil && k8sMetadata.Cluster == "" && k8sMetadata.Namespace == "" && k8sMetadata.NodeName == "" && k8sMetadata.PodName == "" {
		k8sMetadata = nil
	}

	// Create new agent
	agent := &models.Agent{
		Name:               req.Name,
		Hostname:           req.Hostname,
		Platform:           req.Platform,
		Architecture:       req.Architecture,
		Version:            req.Version,
		Status:             models.AgentStatusOnline,
		Capabilities:       req.Capabilities,
		Labels:             models.JSONB(labels),
		KubernetesMetadata: k8sMetadata,
	}

	if req.IPAddress != "" {
		agent.IPAddress = &req.IPAddress
	}

	// Debug: Log agent before DB insert
	h.logger.WithFields(logrus.Fields{
		"name":         agent.Name,
		"hostname":     agent.Hostname,
		"labels":       agent.Labels,
		"k8s_metadata": agent.KubernetesMetadata,
	}).Debug("Attempting to insert agent into database")

	// Save to database
	if err := h.agentRepo.Create(r.Context(), agent); err != nil {
		h.logger.WithError(err).Error("Failed to register agent")
		response.InternalServerError(w, "Failed to register agent")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"agent_id": agent.ID,
		"hostname": agent.Hostname,
		"platform": agent.Platform,
	}).Info("Agent registered")

	resp := RegisterResponse{
		AgentID:      agent.ID.String(),
		Name:         agent.Name,
		Hostname:     agent.Hostname,
		Status:       agent.Status,
		RegisteredAt: agent.RegisteredAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	response.Created(w, resp)
}

// Heartbeat handles PUT /api/v1/agents/:id/heartbeat
func (h *AgentsHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	// Get agent ID from URL
	agentIDStr := chi.URLParam(r, "id")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		response.BadRequest(w, "Invalid agent ID")
		return
	}

	// Update heartbeat
	if err := h.agentRepo.UpdateHeartbeat(r.Context(), agentID); err != nil {
		h.logger.WithError(err).WithField("agent_id", agentID).Error("Failed to update heartbeat")
		response.NotFound(w, "Agent not found")
		return
	}

	h.logger.WithField("agent_id", agentID).Debug("Heartbeat received")

	response.Success(w, map[string]string{
		"message": "Heartbeat recorded",
		"status":  "online",
	})
}

// List handles GET /api/v1/agents
func (h *AgentsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	filters := make(map[string]interface{})

	// Status filter
	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = models.AgentStatus(status)
	}

	// Platform filter
	if platform := r.URL.Query().Get("platform"); platform != "" {
		filters["platform"] = models.AgentPlatform(platform)
	}

	// Sort parameter
	if sort := r.URL.Query().Get("sort"); sort != "" {
		filters["sort"] = sort
	}

	// Pagination
	limit, offset := parsePagination(r)
	if limit > 0 {
		filters["limit"] = limit
	}
	if offset > 0 {
		filters["offset"] = offset
	}

	// Get agents
	agents, err := h.agentRepo.List(r.Context(), filters)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list agents")
		response.InternalServerError(w, "Failed to list agents")
		return
	}

	response.Success(w, map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	})
}

// Get handles GET /api/v1/agents/:id
func (h *AgentsHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Get agent ID from URL
	agentIDStr := chi.URLParam(r, "id")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		response.BadRequest(w, "Invalid agent ID")
		return
	}

	// Get agent
	agent, err := h.agentRepo.GetByID(r.Context(), agentID)
	if err != nil {
		h.logger.WithError(err).WithField("agent_id", agentID).Error("Failed to get agent")
		response.NotFound(w, "Agent not found")
		return
	}

	response.Success(w, agent)
}

// UpdateRequest represents an agent update request
type UpdateRequest struct {
	Name               *string                    `json:"name,omitempty"`
	Status             *models.AgentStatus        `json:"status,omitempty"`
	IPAddress          *string                    `json:"ip_address,omitempty"`
	Version            *string                    `json:"version,omitempty"`
	Capabilities       []string                   `json:"capabilities,omitempty"`
	Labels             map[string]interface{}     `json:"labels,omitempty"`
	KubernetesMetadata *models.KubernetesMetadata `json:"kubernetes_metadata,omitempty"`
}

// Update handles PUT /api/v1/agents/:id
func (h *AgentsHandler) Update(w http.ResponseWriter, r *http.Request) {
	// Get agent ID from URL
	agentIDStr := chi.URLParam(r, "id")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		response.BadRequest(w, "Invalid agent ID")
		return
	}

	// Parse request
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	// Get existing agent
	agent, err := h.agentRepo.GetByID(r.Context(), agentID)
	if err != nil {
		h.logger.WithError(err).WithField("agent_id", agentID).Error("Failed to get agent")
		response.NotFound(w, "Agent not found")
		return
	}

	// Update fields if provided
	if req.Name != nil {
		agent.Name = *req.Name
	}
	if req.Status != nil {
		// Validate status
		validStatuses := map[models.AgentStatus]bool{
			models.AgentStatusOnline:  true,
			models.AgentStatusOffline: true,
			models.AgentStatusError:   true,
		}
		if !validStatuses[*req.Status] {
			response.BadRequest(w, "Invalid status")
			return
		}
		agent.Status = *req.Status
	}
	if req.IPAddress != nil {
		agent.IPAddress = req.IPAddress
	}
	if req.Version != nil {
		agent.Version = *req.Version
	}
	if req.Capabilities != nil {
		agent.Capabilities = req.Capabilities
	}
	if req.Labels != nil {
		agent.Labels = models.JSONB(req.Labels)
	}
	if req.KubernetesMetadata != nil {
		agent.KubernetesMetadata = req.KubernetesMetadata
	}

	// Update in database
	if err := h.agentRepo.Update(r.Context(), agent); err != nil {
		h.logger.WithError(err).WithField("agent_id", agentID).Error("Failed to update agent")
		response.InternalServerError(w, "Failed to update agent")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"agent_id": agentID,
		"name":     agent.Name,
		"status":   agent.Status,
	}).Info("Agent updated")

	response.Success(w, agent)
}

// Delete handles DELETE /api/v1/agents/:id
func (h *AgentsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Get agent ID from URL
	agentIDStr := chi.URLParam(r, "id")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		response.BadRequest(w, "Invalid agent ID")
		return
	}

	// Delete agent
	if err := h.agentRepo.Delete(r.Context(), agentID); err != nil {
		h.logger.WithError(err).WithField("agent_id", agentID).Error("Failed to delete agent")
		response.NotFound(w, "Agent not found")
		return
	}

	h.logger.WithField("agent_id", agentID).Info("Agent deleted")

	response.Success(w, map[string]string{
		"message": "Agent deleted successfully",
	})
}

// Stats handles GET /api/v1/agents/stats
func (h *AgentsHandler) Stats(w http.ResponseWriter, r *http.Request) {
	// Get count by status
	counts, err := h.agentRepo.CountByStatus(r.Context())
	if err != nil {
		h.logger.WithError(err).Error("Failed to get agent stats")
		response.InternalServerError(w, "Failed to get agent stats")
		return
	}

	// Calculate total
	total := 0
	for _, count := range counts {
		total += count
	}

	response.Success(w, map[string]interface{}{
		"total":     total,
		"by_status": counts,
		"online":    counts[models.AgentStatusOnline],
		"offline":   counts[models.AgentStatusOffline],
		"error":     counts[models.AgentStatusError],
	})
}
