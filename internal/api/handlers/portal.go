package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/api/middleware"
	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
)

// PortalHandler handles customer portal requests
type PortalHandler struct {
	tenantRepo *repository.TenantRepository
	agentRepo  *repository.AgentRepository
	eventRepo  *repository.EventRepository
	logger     *logrus.Logger
}

// NewPortalHandler creates a new portal handler
func NewPortalHandler(
	tenantRepo *repository.TenantRepository,
	agentRepo *repository.AgentRepository,
	eventRepo *repository.EventRepository,
	logger *logrus.Logger,
) *PortalHandler {
	return &PortalHandler{
		tenantRepo: tenantRepo,
		agentRepo:  agentRepo,
		eventRepo:  eventRepo,
		logger:     logger,
	}
}

// DashboardResponse represents the portal dashboard data
type DashboardResponse struct {
	Tenant       *TenantSummary  `json:"tenant"`
	Usage        *UsageSummary   `json:"usage"`
	Agents       *AgentsSummary  `json:"agents"`
	RecentEvents []*EventSummary `json:"recent_events"`
	Alerts       []string        `json:"alerts,omitempty"`
}

// TenantSummary is a summary of tenant information
type TenantSummary struct {
	ID            uuid.UUID            `json:"id"`
	Name          string               `json:"name"`
	Slug          string               `json:"slug"`
	Plan          models.TenantPlan    `json:"plan"`
	Status        models.TenantStatus  `json:"status"`
	IsolationTier models.IsolationTier `json:"isolation_tier"`
	MaxAgents     int                  `json:"max_agents"`
	MaxEventsDay  int                  `json:"max_events_per_day"`
	CreatedAt     time.Time            `json:"created_at"`
}

// UsageSummary is a summary of usage statistics
type UsageSummary struct {
	Today      *DailyUsage  `json:"today"`
	Last7Days  *PeriodUsage `json:"last_7_days"`
	Last30Days *PeriodUsage `json:"last_30_days"`
}

// DailyUsage represents usage for a single day
type DailyUsage struct {
	Date            time.Time `json:"date"`
	EventCount      int       `json:"event_count"`
	AIAnalysisCount int       `json:"ai_analysis_count"`
	AgentCount      int       `json:"agent_count"`
	Limit           int       `json:"limit"`
	UsagePercent    float64   `json:"usage_percent"`
}

// PeriodUsage represents usage over a period
type PeriodUsage struct {
	TotalEvents        int     `json:"total_events"`
	TotalAIAnalyses    int     `json:"total_ai_analyses"`
	TotalNotifications int     `json:"total_notifications"`
	AvgEventsPerDay    float64 `json:"avg_events_per_day"`
	PeakAgents         int     `json:"peak_agents"`
}

// AgentsSummary is a summary of agents
type AgentsSummary struct {
	Total   int  `json:"total"`
	Online  int  `json:"online"`
	Offline int  `json:"offline"`
	Limit   int  `json:"limit"`
	CanAdd  bool `json:"can_add"`
}

// EventSummary is a summary of an event
type EventSummary struct {
	ID           uuid.UUID            `json:"id"`
	EventType    string               `json:"event_type"`
	Severity     models.EventSeverity `json:"severity"`
	ResourceKind string               `json:"resource_kind"`
	ResourceName string               `json:"resource_name"`
	Namespace    *string              `json:"namespace,omitempty"`
	Message      string               `json:"message"`
	Timestamp    time.Time            `json:"timestamp"`
	HasAnalysis  bool                 `json:"has_analysis"`
}

// Dashboard handles GET /api/v1/portal/dashboard
func (h *PortalHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.GetTenant(r.Context())
	if tenant == nil {
		response.Unauthorized(w, "Tenant context required")
		return
	}

	// Build tenant summary
	tenantSummary := &TenantSummary{
		ID:            tenant.ID,
		Name:          tenant.Name,
		Slug:          tenant.Slug,
		Plan:          tenant.Plan,
		Status:        tenant.Status,
		IsolationTier: tenant.IsolationTier,
		MaxAgents:     tenant.MaxAgents,
		MaxEventsDay:  tenant.MaxEventsPerDay,
		CreatedAt:     tenant.CreatedAt,
	}

	// Get usage summary
	usageSummary, err := h.getUsageSummary(r.Context(), tenant)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get usage summary")
		usageSummary = &UsageSummary{}
	}

	// Get agents summary
	agentsSummary, err := h.getAgentsSummary(r.Context(), tenant)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get agents summary")
		agentsSummary = &AgentsSummary{}
	}

	// Get recent events
	recentEvents, err := h.getRecentEvents(r.Context(), tenant.ID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get recent events")
		recentEvents = []*EventSummary{}
	}

	// Build alerts
	alerts := h.buildAlerts(tenant, usageSummary, agentsSummary)

	response.Success(w, DashboardResponse{
		Tenant:       tenantSummary,
		Usage:        usageSummary,
		Agents:       agentsSummary,
		RecentEvents: recentEvents,
		Alerts:       alerts,
	})
}

func (h *PortalHandler) getUsageSummary(ctx context.Context, tenant *models.Tenant) (*UsageSummary, error) {
	// Get today's usage
	todayUsage, err := h.tenantRepo.GetTodayUsage(ctx, tenant.ID)
	if err != nil {
		return nil, err
	}

	limit := tenant.MaxEventsPerDay
	usagePercent := 0.0
	if limit > 0 {
		usagePercent = float64(todayUsage.EventCount) / float64(limit) * 100
	}

	today := &DailyUsage{
		Date:            todayUsage.Date,
		EventCount:      todayUsage.EventCount,
		AIAnalysisCount: todayUsage.AIAnalysisCount,
		AgentCount:      todayUsage.AgentCount,
		Limit:           limit,
		UsagePercent:    usagePercent,
	}

	// Get last 7 days
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	weekUsage, err := h.tenantRepo.GetUsage(ctx, tenant.ID, weekAgo, now)
	if err != nil {
		return nil, err
	}

	last7Days := h.aggregateUsage(weekUsage, 7)

	// Get last 30 days
	monthAgo := now.AddDate(0, 0, -30)
	monthUsage, err := h.tenantRepo.GetUsage(ctx, tenant.ID, monthAgo, now)
	if err != nil {
		return nil, err
	}

	last30Days := h.aggregateUsage(monthUsage, 30)

	return &UsageSummary{
		Today:      today,
		Last7Days:  last7Days,
		Last30Days: last30Days,
	}, nil
}

func (h *PortalHandler) aggregateUsage(usages []*models.TenantUsage, days int) *PeriodUsage {
	result := &PeriodUsage{}

	for _, u := range usages {
		result.TotalEvents += u.EventCount
		result.TotalAIAnalyses += u.AIAnalysisCount
		result.TotalNotifications += u.NotificationCount
		if u.PeakAgents > result.PeakAgents {
			result.PeakAgents = u.PeakAgents
		}
	}

	if days > 0 {
		result.AvgEventsPerDay = float64(result.TotalEvents) / float64(days)
	}

	return result
}

func (h *PortalHandler) getAgentsSummary(ctx context.Context, tenant *models.Tenant) (*AgentsSummary, error) {
	agents, err := h.agentRepo.ListByTenant(ctx, tenant.ID)
	if err != nil {
		return nil, err
	}

	summary := &AgentsSummary{
		Total:  len(agents),
		Limit:  tenant.MaxAgents,
		CanAdd: tenant.CanAddAgent(len(agents)),
	}

	for _, agent := range agents {
		if agent.IsOnline() {
			summary.Online++
		} else {
			summary.Offline++
		}
	}

	return summary, nil
}

func (h *PortalHandler) getRecentEvents(ctx context.Context, tenantID uuid.UUID) ([]*EventSummary, error) {
	filters := map[string]interface{}{
		"tenant_id": tenantID,
		"limit":     10,
		"sort":      "event_timestamp",
	}

	events, err := h.eventRepo.List(ctx, filters)
	if err != nil {
		return nil, err
	}

	summaries := make([]*EventSummary, len(events))
	for i, e := range events {
		summaries[i] = &EventSummary{
			ID:           e.ID,
			EventType:    e.EventType,
			Severity:     e.Severity,
			ResourceKind: e.ResourceKind,
			ResourceName: e.ResourceName,
			Namespace:    e.Namespace,
			Message:      e.Message,
			Timestamp:    e.EventTimestamp,
			HasAnalysis:  e.HasAIAnalysis(),
		}
	}

	return summaries, nil
}

func (h *PortalHandler) buildAlerts(tenant *models.Tenant, usage *UsageSummary, agents *AgentsSummary) []string {
	var alerts []string

	// Check event usage
	if usage != nil && usage.Today != nil && usage.Today.UsagePercent >= 90 {
		alerts = append(alerts, "Event limit almost reached (90%+). Consider upgrading your plan.")
	} else if usage != nil && usage.Today != nil && usage.Today.UsagePercent >= 75 {
		alerts = append(alerts, "Event usage at 75%+ of daily limit.")
	}

	// Check agent limit
	if agents != nil && !agents.CanAdd {
		alerts = append(alerts, "Agent limit reached. Upgrade to add more agents.")
	} else if agents != nil && agents.Total >= agents.Limit-2 {
		alerts = append(alerts, "Approaching agent limit. Consider upgrading your plan.")
	}

	// Check offline agents
	if agents != nil && agents.Offline > 0 {
		alerts = append(alerts, "Some agents are offline. Check agent health.")
	}

	// Check tenant status
	if tenant.Status == models.TenantStatusSuspended {
		alerts = append(alerts, "Account is suspended. Please contact support.")
	}

	return alerts
}

// GetProfile handles GET /api/v1/portal/profile
func (h *PortalHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.GetTenant(r.Context())
	if tenant == nil {
		response.Unauthorized(w, "Tenant context required")
		return
	}

	response.Success(w, tenant)
}

// UpdateProfileRequest represents a request to update profile
type UpdateProfileRequest struct {
	Name        *string `json:"name,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
}

// UpdateProfile handles PUT /api/v1/portal/profile
func (h *PortalHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.GetTenant(r.Context())
	if tenant == nil {
		response.Unauthorized(w, "Tenant context required")
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	if req.Name != nil {
		tenant.Name = *req.Name
	}
	if req.DisplayName != nil {
		tenant.DisplayName = *req.DisplayName
	}

	if err := h.tenantRepo.Update(r.Context(), tenant); err != nil {
		h.logger.WithError(err).Error("Failed to update profile")
		response.InternalServerError(w, "Failed to update profile")
		return
	}

	response.Success(w, tenant)
}

// GetUsageHistory handles GET /api/v1/portal/usage
func (h *PortalHandler) GetUsageHistory(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.GetTenant(r.Context())
	if tenant == nil {
		response.Unauthorized(w, "Tenant context required")
		return
	}

	// Parse date range
	days := 30 // default
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := json.Number(d).Int64(); err == nil && parsed > 0 && parsed <= 90 {
			days = int(parsed)
		}
	}

	now := time.Now()
	from := now.AddDate(0, 0, -days)

	usage, err := h.tenantRepo.GetUsage(r.Context(), tenant.ID, from, now)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get usage history")
		response.InternalServerError(w, "Failed to get usage history")
		return
	}

	response.Success(w, map[string]interface{}{
		"usage": usage,
		"days":  days,
		"from":  from,
		"to":    now,
	})
}

// ListAgents handles GET /api/v1/portal/agents
func (h *PortalHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.GetTenant(r.Context())
	if tenant == nil {
		response.Unauthorized(w, "Tenant context required")
		return
	}

	agents, err := h.agentRepo.ListByTenant(r.Context(), tenant.ID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list agents")
		response.InternalServerError(w, "Failed to list agents")
		return
	}

	response.Success(w, map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
		"limit":  tenant.MaxAgents,
	})
}

// GetAgent handles GET /api/v1/portal/agents/:id
func (h *PortalHandler) GetAgent(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.GetTenant(r.Context())
	if tenant == nil {
		response.Unauthorized(w, "Tenant context required")
		return
	}

	agentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid agent ID")
		return
	}

	agent, err := h.agentRepo.GetByID(r.Context(), agentID)
	if err != nil {
		response.NotFound(w, "Agent not found")
		return
	}

	// Verify agent belongs to tenant
	if agent.TenantID != tenant.ID {
		response.NotFound(w, "Agent not found")
		return
	}

	response.Success(w, agent)
}

// DeleteAgent handles DELETE /api/v1/portal/agents/:id
func (h *PortalHandler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.GetTenant(r.Context())
	if tenant == nil {
		response.Unauthorized(w, "Tenant context required")
		return
	}

	agentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid agent ID")
		return
	}

	// Verify agent belongs to tenant
	agent, err := h.agentRepo.GetByID(r.Context(), agentID)
	if err != nil {
		response.NotFound(w, "Agent not found")
		return
	}

	if agent.TenantID != tenant.ID {
		response.NotFound(w, "Agent not found")
		return
	}

	if err := h.agentRepo.Delete(r.Context(), agentID); err != nil {
		h.logger.WithError(err).Error("Failed to delete agent")
		response.InternalServerError(w, "Failed to delete agent")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"tenant_id": tenant.ID,
		"agent_id":  agentID,
	}).Info("Agent deleted via portal")

	response.NoContent(w)
}

// ListAPIKeys handles GET /api/v1/portal/api-keys
func (h *PortalHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.GetTenant(r.Context())
	if tenant == nil {
		response.Unauthorized(w, "Tenant context required")
		return
	}

	keys, err := h.tenantRepo.ListAPIKeys(r.Context(), tenant.ID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list API keys")
		response.InternalServerError(w, "Failed to list API keys")
		return
	}

	// Return without actual key values
	keyInfos := make([]map[string]interface{}, len(keys))
	for i, key := range keys {
		keyInfos[i] = map[string]interface{}{
			"id":           key.ID,
			"name":         key.Name,
			"key_prefix":   key.KeyPrefix,
			"scopes":       key.Scopes,
			"expires_at":   key.ExpiresAt,
			"last_used_at": key.LastUsedAt,
			"created_at":   key.CreatedAt,
			"is_revoked":   key.IsRevoked(),
			"is_expired":   key.IsExpired(),
		}
	}

	response.Success(w, map[string]interface{}{
		"api_keys": keyInfos,
		"count":    len(keyInfos),
	})
}

// BillingInfo represents billing information
type BillingInfo struct {
	Plan            models.TenantPlan `json:"plan"`
	PlanDisplayName string            `json:"plan_display_name"`
	PlanStartedAt   *time.Time        `json:"plan_started_at,omitempty"`
	PlanExpiresAt   *time.Time        `json:"plan_expires_at,omitempty"`
	NextBillingDate *time.Time        `json:"next_billing_date,omitempty"`
	MonthlyPrice    float64           `json:"monthly_price"`
	AnnualPrice     float64           `json:"annual_price"`
	Features        []string          `json:"features"`
	UpgradeOptions  []PlanOption      `json:"upgrade_options,omitempty"`
}

// PlanOption represents an upgrade option
type PlanOption struct {
	Plan            models.TenantPlan `json:"plan"`
	DisplayName     string            `json:"display_name"`
	MonthlyPrice    float64           `json:"monthly_price"`
	AnnualPrice     float64           `json:"annual_price"`
	MaxAgents       int               `json:"max_agents"`
	MaxEventsPerDay int               `json:"max_events_per_day"`
	Features        []string          `json:"features"`
}

// GetBilling handles GET /api/v1/portal/billing
func (h *PortalHandler) GetBilling(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.GetTenant(r.Context())
	if tenant == nil {
		response.Unauthorized(w, "Tenant context required")
		return
	}

	pricing := getPricing()
	currentPlan := pricing[tenant.Plan]

	billing := BillingInfo{
		Plan:            tenant.Plan,
		PlanDisplayName: currentPlan.DisplayName,
		PlanStartedAt:   tenant.PlanStartedAt,
		PlanExpiresAt:   tenant.PlanExpiresAt,
		MonthlyPrice:    currentPlan.MonthlyPrice,
		AnnualPrice:     currentPlan.AnnualPrice,
		Features:        currentPlan.Features,
	}

	// Add upgrade options
	if tenant.Plan != models.TenantPlanEnterprise {
		var upgradeOptions []PlanOption
		planOrder := []models.TenantPlan{
			models.TenantPlanStarter,
			models.TenantPlanPro,
			models.TenantPlanEnterprise,
		}

		foundCurrent := false
		for _, plan := range planOrder {
			if plan == tenant.Plan {
				foundCurrent = true
				continue
			}
			if foundCurrent {
				upgradeOptions = append(upgradeOptions, pricing[plan])
			}
		}
		billing.UpgradeOptions = upgradeOptions
	}

	response.Success(w, billing)
}

func getPricing() map[models.TenantPlan]PlanOption {
	return map[models.TenantPlan]PlanOption{
		models.TenantPlanTrial: {
			Plan:            models.TenantPlanTrial,
			DisplayName:     "Trial",
			MonthlyPrice:    0,
			AnnualPrice:     0,
			MaxAgents:       5,
			MaxEventsPerDay: 1000,
			Features: []string{
				"Up to 5 agents",
				"1,000 events/day",
				"Basic AI analysis",
				"Email notifications",
				"7-day retention",
			},
		},
		models.TenantPlanStarter: {
			Plan:            models.TenantPlanStarter,
			DisplayName:     "Starter",
			MonthlyPrice:    49,
			AnnualPrice:     490,
			MaxAgents:       25,
			MaxEventsPerDay: 10000,
			Features: []string{
				"Up to 25 agents",
				"10,000 events/day",
				"Advanced AI analysis",
				"Slack & Email notifications",
				"30-day retention",
				"API access",
			},
		},
		models.TenantPlanPro: {
			Plan:            models.TenantPlanPro,
			DisplayName:     "Pro",
			MonthlyPrice:    199,
			AnnualPrice:     1990,
			MaxAgents:       100,
			MaxEventsPerDay: 100000,
			Features: []string{
				"Up to 100 agents",
				"100,000 events/day",
				"Premium AI analysis",
				"All notification channels",
				"90-day retention",
				"Priority support",
				"Custom integrations",
			},
		},
		models.TenantPlanEnterprise: {
			Plan:            models.TenantPlanEnterprise,
			DisplayName:     "Enterprise",
			MonthlyPrice:    999,
			AnnualPrice:     9990,
			MaxAgents:       1000,
			MaxEventsPerDay: 1000000,
			Features: []string{
				"Unlimited agents",
				"1,000,000 events/day",
				"Dedicated AI models",
				"All notification channels",
				"1-year retention",
				"24/7 dedicated support",
				"Dedicated infrastructure",
				"Custom SLAs",
				"On-premise option",
			},
		},
	}
}
