package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/api/auth"
	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
)

// generateAPIKey generates a random API key
func generateAPIKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to uuid if crypto/rand fails
		return "lk_" + uuid.New().String()
	}
	return "lk_" + hex.EncodeToString(bytes)
}

// hashAPIKey creates a SHA-256 hash of an API key
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// TenantHandler handles tenant management requests
type TenantHandler struct {
	tenantRepo *repository.TenantRepository
	agentRepo  *repository.AgentRepository
	jwtManager *auth.JWTManager
	logger     *logrus.Logger
}

// NewTenantHandler creates a new tenant handler
func NewTenantHandler(
	tenantRepo *repository.TenantRepository,
	agentRepo *repository.AgentRepository,
	jwtManager *auth.JWTManager,
	logger *logrus.Logger,
) *TenantHandler {
	return &TenantHandler{
		tenantRepo: tenantRepo,
		agentRepo:  agentRepo,
		jwtManager: jwtManager,
		logger:     logger,
	}
}

// CreateTenantRequest represents a request to create a new tenant
type CreateTenantRequest struct {
	Name          string               `json:"name" validate:"required"`
	Slug          string               `json:"slug" validate:"required"`
	DisplayName   string               `json:"display_name"`
	Plan          models.TenantPlan    `json:"plan" validate:"required"`
	IsolationTier models.IsolationTier `json:"isolation_tier"`
	MaxAgents     int                  `json:"max_agents"`
	MaxEventsDay  int                  `json:"max_events_per_day"`
}

// CreateTenantResponse represents the response after creating a tenant
type CreateTenantResponse struct {
	Tenant *models.Tenant `json:"tenant"`
	APIKey *APIKeyInfo    `json:"api_key"`
}

// APIKeyInfo contains non-sensitive API key information plus the key itself (only on creation)
type APIKeyInfo struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	KeyPrefix string     `json:"key_prefix"`
	Key       string     `json:"key,omitempty"` // Only returned on creation
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Create handles POST /api/v1/admin/tenants
func (h *TenantHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		response.BadRequest(w, "Name is required")
		return
	}
	if req.Slug == "" {
		response.BadRequest(w, "Slug is required")
		return
	}

	// Validate plan
	validPlans := map[models.TenantPlan]bool{
		models.TenantPlanTrial:      true,
		models.TenantPlanStarter:    true,
		models.TenantPlanPro:        true,
		models.TenantPlanEnterprise: true,
	}
	if !validPlans[req.Plan] {
		response.BadRequest(w, "Invalid plan")
		return
	}

	// Set defaults based on plan
	maxAgents := req.MaxAgents
	maxEventsDay := req.MaxEventsDay
	isolationTier := req.IsolationTier

	if maxAgents == 0 {
		switch req.Plan {
		case models.TenantPlanTrial:
			maxAgents = 5
		case models.TenantPlanStarter:
			maxAgents = 25
		case models.TenantPlanPro:
			maxAgents = 100
		case models.TenantPlanEnterprise:
			maxAgents = 1000
		}
	}

	if maxEventsDay == 0 {
		switch req.Plan {
		case models.TenantPlanTrial:
			maxEventsDay = 1000
		case models.TenantPlanStarter:
			maxEventsDay = 10000
		case models.TenantPlanPro:
			maxEventsDay = 100000
		case models.TenantPlanEnterprise:
			maxEventsDay = 1000000
		}
	}

	if isolationTier == "" {
		if req.Plan == models.TenantPlanEnterprise {
			isolationTier = models.IsolationTierDedicated
		} else {
			isolationTier = models.IsolationTierShared
		}
	}

	// Check if slug is already taken
	existing, _ := h.tenantRepo.GetBySlug(r.Context(), req.Slug)
	if existing != nil {
		response.Error(w, http.StatusConflict, "conflict", "Tenant slug already exists")
		return
	}

	// Create tenant
	tenant := &models.Tenant{
		Name:            req.Name,
		Slug:            req.Slug,
		DisplayName:     req.DisplayName,
		Plan:            req.Plan,
		Status:          models.TenantStatusActive,
		IsolationTier:   isolationTier,
		MaxAgents:       maxAgents,
		MaxEventsPerDay: maxEventsDay,
	}

	if err := h.tenantRepo.Create(r.Context(), tenant); err != nil {
		h.logger.WithError(err).Error("Failed to create tenant")
		response.InternalServerError(w, "Failed to create tenant")
		return
	}

	// Create initial API key for the tenant
	apiKeyID := uuid.New()
	rawKey := generateAPIKey()
	keyHash := hashAPIKey(rawKey)
	keyPrefix := rawKey[:8]

	apiKey := &models.TenantAPIKey{
		ID:        apiKeyID,
		TenantID:  tenant.ID,
		Name:      "default",
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Scopes: []string{
			"agent:register",
			"agent:read",
			"events:submit",
			"events:read",
			"jobs:read",
		},
	}

	if err := h.tenantRepo.CreateAPIKey(r.Context(), apiKey, rawKey); err != nil {
		h.logger.WithError(err).Error("Failed to create API key")
		// Tenant was created, but API key failed - still return success with warning
		response.Created(w, CreateTenantResponse{
			Tenant: tenant,
			APIKey: nil,
		})
		return
	}

	h.logger.WithFields(logrus.Fields{
		"tenant_id":   tenant.ID,
		"tenant_slug": tenant.Slug,
		"plan":        tenant.Plan,
	}).Info("Tenant created")

	response.Created(w, CreateTenantResponse{
		Tenant: tenant,
		APIKey: &APIKeyInfo{
			ID:        apiKey.ID,
			Name:      apiKey.Name,
			KeyPrefix: apiKey.KeyPrefix,
			Key:       rawKey, // Only returned on creation
			Scopes:    apiKey.Scopes,
			ExpiresAt: apiKey.ExpiresAt,
			CreatedAt: apiKey.CreatedAt,
		},
	})
}

// Get handles GET /api/v1/admin/tenants/:id
func (h *TenantHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid tenant ID")
		return
	}

	tenant, err := h.tenantRepo.GetByID(r.Context(), tenantID)
	if err != nil {
		response.NotFound(w, "Tenant not found")
		return
	}

	response.Success(w, tenant)
}

// List handles GET /api/v1/admin/tenants
func (h *TenantHandler) List(w http.ResponseWriter, r *http.Request) {
	filters := make(map[string]interface{})

	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = models.TenantStatus(status)
	}

	if plan := r.URL.Query().Get("plan"); plan != "" {
		filters["plan"] = models.TenantPlan(plan)
	}

	limit, offset := parsePagination(r)
	filters["limit"] = limit
	filters["offset"] = offset

	tenants, err := h.tenantRepo.List(r.Context(), filters)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list tenants")
		response.InternalServerError(w, "Failed to list tenants")
		return
	}

	response.Success(w, map[string]interface{}{
		"tenants": tenants,
		"count":   len(tenants),
	})
}

// UpdateTenantRequest represents a request to update a tenant
type UpdateTenantRequest struct {
	Name         *string              `json:"name,omitempty"`
	DisplayName  *string              `json:"display_name,omitempty"`
	Plan         *models.TenantPlan   `json:"plan,omitempty"`
	Status       *models.TenantStatus `json:"status,omitempty"`
	MaxAgents    *int                 `json:"max_agents,omitempty"`
	MaxEventsDay *int                 `json:"max_events_per_day,omitempty"`
}

// Update handles PUT /api/v1/admin/tenants/:id
func (h *TenantHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid tenant ID")
		return
	}

	var req UpdateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	// Get existing tenant
	tenant, err := h.tenantRepo.GetByID(r.Context(), tenantID)
	if err != nil {
		response.NotFound(w, "Tenant not found")
		return
	}

	// Apply updates
	if req.Name != nil {
		tenant.Name = *req.Name
	}
	if req.DisplayName != nil {
		tenant.DisplayName = *req.DisplayName
	}
	if req.Plan != nil {
		tenant.Plan = *req.Plan
	}
	if req.Status != nil {
		tenant.Status = *req.Status
	}
	if req.MaxAgents != nil {
		tenant.MaxAgents = *req.MaxAgents
	}
	if req.MaxEventsDay != nil {
		tenant.MaxEventsPerDay = *req.MaxEventsDay
	}

	if err := h.tenantRepo.Update(r.Context(), tenant); err != nil {
		h.logger.WithError(err).Error("Failed to update tenant")
		response.InternalServerError(w, "Failed to update tenant")
		return
	}

	h.logger.WithField("tenant_id", tenant.ID).Info("Tenant updated")
	response.Success(w, tenant)
}

// Delete handles DELETE /api/v1/admin/tenants/:id
func (h *TenantHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid tenant ID")
		return
	}

	// Don't allow deleting the default tenant
	if tenantID == models.DefaultTenantID {
		response.Error(w, http.StatusForbidden, "forbidden", "Cannot delete default tenant")
		return
	}

	if err := h.tenantRepo.Delete(r.Context(), tenantID); err != nil {
		h.logger.WithError(err).Error("Failed to delete tenant")
		response.InternalServerError(w, "Failed to delete tenant")
		return
	}

	h.logger.WithField("tenant_id", tenantID).Info("Tenant deleted")
	response.NoContent(w)
}

// GetStats handles GET /api/v1/admin/tenants/:id/stats
func (h *TenantHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid tenant ID")
		return
	}

	// Verify tenant exists
	tenant, err := h.tenantRepo.GetByID(r.Context(), tenantID)
	if err != nil {
		response.NotFound(w, "Tenant not found")
		return
	}

	// Get agent count
	agentCount, err := h.agentRepo.CountByTenant(r.Context(), tenantID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to count agents")
		agentCount = 0
	}

	response.Success(w, map[string]interface{}{
		"tenant_id":    tenant.ID,
		"tenant_slug":  tenant.Slug,
		"agent_count":  agentCount,
		"agent_limit":  tenant.MaxAgents,
		"plan":         tenant.Plan,
		"status":       tenant.Status,
		"can_add_more": tenant.CanAddAgent(agentCount),
	})
}

// ProvisionAgentRequest represents a request to provision a new agent
type ProvisionAgentRequest struct {
	Name        string            `json:"name" validate:"required"`
	Platform    string            `json:"platform" validate:"required"`
	Labels      map[string]string `json:"labels,omitempty"`
	Cluster     string            `json:"cluster,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	TokenExpiry string            `json:"token_expiry,omitempty"` // e.g., "24h", "7d", "30d"
}

// ProvisionAgentResponse represents the response after provisioning an agent
type ProvisionAgentResponse struct {
	AgentID       string            `json:"agent_id"`
	Name          string            `json:"name"`
	Token         string            `json:"token"`
	TokenExpiry   time.Time         `json:"token_expiry"`
	APIEndpoint   string            `json:"api_endpoint"`
	TenantID      string            `json:"tenant_id"`
	TenantSlug    string            `json:"tenant_slug"`
	KubeManifest  string            `json:"kube_manifest,omitempty"`
	Configuration map[string]string `json:"configuration"`
}

// ProvisionAgent handles POST /api/v1/admin/tenants/:id/agents/provision
func (h *TenantHandler) ProvisionAgent(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid tenant ID")
		return
	}

	var req ProvisionAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		response.BadRequest(w, "Name is required")
		return
	}
	if req.Platform == "" {
		response.BadRequest(w, "Platform is required")
		return
	}

	// Get tenant
	tenant, err := h.tenantRepo.GetByID(r.Context(), tenantID)
	if err != nil {
		response.NotFound(w, "Tenant not found")
		return
	}

	// Check if tenant is active
	if !tenant.IsActive() {
		response.Error(w, http.StatusForbidden, "forbidden", "Tenant is not active")
		return
	}

	// Check agent limit
	agentCount, err := h.agentRepo.CountByTenant(r.Context(), tenantID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to count agents")
		response.InternalServerError(w, "Failed to check agent limit")
		return
	}

	if !tenant.CanAddAgent(agentCount) {
		response.Error(w, http.StatusPaymentRequired, "limit_exceeded",
			"Agent limit reached. Please upgrade your plan.")
		return
	}

	// Generate agent ID
	agentID := uuid.New()

	// Parse token expiry
	tokenExpiry := 24 * time.Hour * 30 // Default: 30 days
	if req.TokenExpiry != "" {
		if duration, err := time.ParseDuration(req.TokenExpiry); err == nil {
			tokenExpiry = duration
		}
	}

	// Generate agent token
	token, err := h.jwtManager.GenerateAgentToken(
		tenant.ID.String(),
		tenant.Slug,
		agentID.String(),
		[]string{"agent:heartbeat", "events:submit"},
	)
	if err != nil {
		h.logger.WithError(err).Error("Failed to generate agent token")
		response.InternalServerError(w, "Failed to generate agent token")
		return
	}

	// Build configuration
	config := map[string]string{
		"LUMO_AGENT_ID":           agentID.String(),
		"LUMO_AGENT_NAME":         req.Name,
		"LUMO_TENANT_ID":          tenant.ID.String(),
		"LUMO_TENANT_SLUG":        tenant.Slug,
		"LUMO_AGENT_TOKEN":        token,
		"LUMO_AGENT_MODE":         "event-driven",
		"LUMO_AGENT_API_ENDPOINT": "https://api.lumo.io", // TODO: Make configurable
	}

	if req.Cluster != "" {
		config["LUMO_AGENT_CLUSTER"] = req.Cluster
	}
	if req.Namespace != "" {
		config["LUMO_AGENT_NAMESPACE"] = req.Namespace
	}

	// Generate Kubernetes manifest if platform is kubernetes
	var kubeManifest string
	if req.Platform == "kubernetes" {
		kubeManifest = h.generateKubeManifest(agentID.String(), req.Name, tenant.Slug, config, req.Labels)
	}

	h.logger.WithFields(logrus.Fields{
		"tenant_id": tenant.ID,
		"agent_id":  agentID,
		"name":      req.Name,
		"platform":  req.Platform,
	}).Info("Agent provisioned")

	response.Created(w, ProvisionAgentResponse{
		AgentID:       agentID.String(),
		Name:          req.Name,
		Token:         token,
		TokenExpiry:   time.Now().Add(tokenExpiry),
		APIEndpoint:   "https://api.lumo.io",
		TenantID:      tenant.ID.String(),
		TenantSlug:    tenant.Slug,
		KubeManifest:  kubeManifest,
		Configuration: config,
	})
}

// generateKubeManifest generates a Kubernetes deployment manifest for the agent
func (h *TenantHandler) generateKubeManifest(agentID, name, tenantSlug string, config map[string]string, labels map[string]string) string {
	// Build labels string
	labelsYAML := ""
	for k, v := range labels {
		labelsYAML += "      " + k + ": \"" + v + "\"\n"
	}

	manifest := `---
apiVersion: v1
kind: Namespace
metadata:
  name: lumo-agent
  labels:
    app.kubernetes.io/name: lumo-agent
    tenant: ` + tenantSlug + `
---
apiVersion: v1
kind: Secret
metadata:
  name: lumo-agent-token
  namespace: lumo-agent
type: Opaque
stringData:
  token: "` + config["LUMO_AGENT_TOKEN"] + `"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: lumo-agent-config
  namespace: lumo-agent
data:
  LUMO_AGENT_ID: "` + agentID + `"
  LUMO_AGENT_NAME: "` + name + `"
  LUMO_TENANT_ID: "` + config["LUMO_TENANT_ID"] + `"
  LUMO_TENANT_SLUG: "` + tenantSlug + `"
  LUMO_AGENT_MODE: "event-driven"
  LUMO_AGENT_API_ENDPOINT: "` + config["LUMO_AGENT_API_ENDPOINT"] + `"
  LUMO_AGENT_EVENT_DRIVEN_ENABLED: "true"
  LUMO_AGENT_EVENT_DRIVEN_DEBOUNCE_WINDOW: "45s"
  LUMO_AGENT_ENABLED_CHECKS: "kubernetes"
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: lumo-agent
  namespace: lumo-agent
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: lumo-agent
rules:
  - apiGroups: [""]
    resources: ["pods", "nodes", "events", "namespaces", "persistentvolumeclaims"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: lumo-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: lumo-agent
subjects:
  - kind: ServiceAccount
    name: lumo-agent
    namespace: lumo-agent
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lumo-agent
  namespace: lumo-agent
  labels:
    app.kubernetes.io/name: lumo-agent
    tenant: ` + tenantSlug + `
` + labelsYAML + `
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: lumo-agent
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lumo-agent
        tenant: ` + tenantSlug + `
    spec:
      serviceAccountName: lumo-agent
      containers:
        - name: lumo-agent
          image: ghcr.io/ignacio/lumo-agent:latest
          envFrom:
            - configMapRef:
                name: lumo-agent-config
          env:
            - name: LUMO_AGENT_TOKEN
              valueFrom:
                secretKeyRef:
                  name: lumo-agent-token
                  key: token
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
          livenessProbe:
            httpGet:
              path: /health
              port: 8081
            initialDelaySeconds: 10
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /health
              port: 8081
            initialDelaySeconds: 5
            periodSeconds: 10
`
	return manifest
}

// ListAPIKeys handles GET /api/v1/admin/tenants/:id/api-keys
func (h *TenantHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid tenant ID")
		return
	}

	keys, err := h.tenantRepo.ListAPIKeys(r.Context(), tenantID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list API keys")
		response.InternalServerError(w, "Failed to list API keys")
		return
	}

	// Convert to APIKeyInfo (without the actual key)
	keyInfos := make([]APIKeyInfo, len(keys))
	for i, key := range keys {
		keyInfos[i] = APIKeyInfo{
			ID:        key.ID,
			Name:      key.Name,
			KeyPrefix: key.KeyPrefix,
			Scopes:    key.Scopes,
			ExpiresAt: key.ExpiresAt,
			CreatedAt: key.CreatedAt,
		}
	}

	response.Success(w, map[string]interface{}{
		"api_keys": keyInfos,
		"count":    len(keyInfos),
	})
}

// CreateAPIKeyRequest represents a request to create a new API key
type CreateAPIKeyRequest struct {
	Name      string   `json:"name" validate:"required"`
	Scopes    []string `json:"scopes" validate:"required"`
	ExpiresIn string   `json:"expires_in,omitempty"` // e.g., "30d", "90d", "1y"
}

// CreateAPIKey handles POST /api/v1/admin/tenants/:id/api-keys
func (h *TenantHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid tenant ID")
		return
	}

	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	if req.Name == "" {
		response.BadRequest(w, "Name is required")
		return
	}
	if len(req.Scopes) == 0 {
		response.BadRequest(w, "At least one scope is required")
		return
	}

	// Parse expiry
	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		duration, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			// Try parsing as days (e.g., "30d")
			if len(req.ExpiresIn) > 1 && req.ExpiresIn[len(req.ExpiresIn)-1] == 'd' {
				days := req.ExpiresIn[:len(req.ExpiresIn)-1]
				if d, err := time.ParseDuration(days + "h"); err == nil {
					duration = d * 24
				}
			}
		}
		if duration > 0 {
			t := time.Now().Add(duration)
			expiresAt = &t
		}
	}

	// Generate API key
	rawKey := generateAPIKey()
	keyHash := hashAPIKey(rawKey)
	keyPrefix := rawKey[:8]

	apiKey := &models.TenantAPIKey{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Name:      req.Name,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Scopes:    req.Scopes,
		ExpiresAt: expiresAt,
	}

	if err := h.tenantRepo.CreateAPIKey(r.Context(), apiKey, rawKey); err != nil {
		h.logger.WithError(err).Error("Failed to create API key")
		response.InternalServerError(w, "Failed to create API key")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"tenant_id":  tenantID,
		"key_id":     apiKey.ID,
		"key_prefix": apiKey.KeyPrefix,
	}).Info("API key created")

	response.Created(w, APIKeyInfo{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		KeyPrefix: apiKey.KeyPrefix,
		Key:       rawKey, // Only returned on creation
		Scopes:    apiKey.Scopes,
		ExpiresAt: apiKey.ExpiresAt,
		CreatedAt: apiKey.CreatedAt,
	})
}

// RevokeAPIKey handles DELETE /api/v1/admin/tenants/:id/api-keys/:keyId
func (h *TenantHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid tenant ID")
		return
	}

	keyID, err := uuid.Parse(chi.URLParam(r, "keyId"))
	if err != nil {
		response.BadRequest(w, "Invalid API key ID")
		return
	}

	// TODO: Could add tenant verification here to ensure the key belongs to the tenant
	_ = tenantID // tenantID available for verification if needed

	if err := h.tenantRepo.RevokeAPIKey(r.Context(), keyID); err != nil {
		h.logger.WithError(err).Error("Failed to revoke API key")
		response.InternalServerError(w, "Failed to revoke API key")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"key_id":    keyID,
	}).Info("API key revoked")

	response.NoContent(w)
}
