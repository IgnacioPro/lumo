package models

import (
	"time"

	"github.com/google/uuid"
)

// TenantPlan represents the subscription plan
type TenantPlan string

const (
	TenantPlanTrial      TenantPlan = "trial"
	TenantPlanStarter    TenantPlan = "starter"
	TenantPlanPro        TenantPlan = "pro"
	TenantPlanEnterprise TenantPlan = "enterprise"
)

// TenantStatus represents the tenant's current status
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusCancelled TenantStatus = "cancelled"
	TenantStatusPending   TenantStatus = "pending"
)

// IsolationTier represents the infrastructure isolation level
type IsolationTier string

const (
	IsolationTierShared    IsolationTier = "shared"
	IsolationTierDedicated IsolationTier = "dedicated"
)

// TenantUserRole represents user roles within a tenant
type TenantUserRole string

const (
	TenantUserRoleOwner  TenantUserRole = "owner"
	TenantUserRoleAdmin  TenantUserRole = "admin"
	TenantUserRoleMember TenantUserRole = "member"
	TenantUserRoleViewer TenantUserRole = "viewer"
)

// DefaultTenantID is the UUID for the default tenant (backwards compatibility)
var DefaultTenantID = uuid.MustParse("00000000-0000-0000-0000-000000000000")

// Tenant represents a customer organization
type Tenant struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	DisplayName string    `json:"display_name"`

	// Plan & Subscription
	Plan                 TenantPlan `json:"plan"`
	PlanStartedAt        *time.Time `json:"plan_started_at,omitempty"`
	PlanExpiresAt        *time.Time `json:"plan_expires_at,omitempty"`
	StripeCustomerID     *string    `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID *string    `json:"stripe_subscription_id,omitempty"`

	// Limits
	MaxAgents         int  `json:"max_agents"`
	MaxEventsPerDay   int  `json:"max_events_per_day"`
	MaxUsers          int  `json:"max_users"`
	AIAnalysisEnabled bool `json:"ai_analysis_enabled"`
	RetentionDays     int  `json:"retention_days"`

	// Isolation
	IsolationTier      IsolationTier `json:"isolation_tier"`
	DedicatedNamespace *string       `json:"dedicated_namespace,omitempty"`
	DedicatedDBHost    *string       `json:"dedicated_db_host,omitempty"`

	// Status
	Status          TenantStatus `json:"status"`
	SuspendedReason *string      `json:"suspended_reason,omitempty"`

	// Metadata
	Settings  JSONB     `json:"settings,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsActive returns true if the tenant is active
func (t *Tenant) IsActive() bool {
	return t.Status == TenantStatusActive
}

// IsEnterprise returns true if the tenant is on the enterprise plan
func (t *Tenant) IsEnterprise() bool {
	return t.Plan == TenantPlanEnterprise
}

// IsDedicated returns true if the tenant has dedicated infrastructure
func (t *Tenant) IsDedicated() bool {
	return t.IsolationTier == IsolationTierDedicated
}

// IsDefault returns true if this is the default tenant
func (t *Tenant) IsDefault() bool {
	return t.ID == DefaultTenantID
}

// CanAddAgent returns true if the tenant can add more agents
func (t *Tenant) CanAddAgent(currentCount int) bool {
	return currentCount < t.MaxAgents
}

// TenantAPIKey represents an API key for agent provisioning
type TenantAPIKey struct {
	ID         uuid.UUID  `json:"id"`
	TenantID   uuid.UUID  `json:"tenant_id"`
	Name       string     `json:"name"`
	KeyHash    string     `json:"-"` // Never expose hash
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedBy  *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// IsRevoked returns true if the API key has been revoked
func (k *TenantAPIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}

// IsExpired returns true if the API key has expired
func (k *TenantAPIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsValid returns true if the API key is valid (not revoked and not expired)
func (k *TenantAPIKey) IsValid() bool {
	return !k.IsRevoked() && !k.IsExpired()
}

// HasScope returns true if the API key has the specified scope
func (k *TenantAPIKey) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// TenantUser represents a user within a tenant
type TenantUser struct {
	ID           uuid.UUID      `json:"id"`
	TenantID     uuid.UUID      `json:"tenant_id"`
	Email        string         `json:"email"`
	PasswordHash *string        `json:"-"` // Never expose
	Name         *string        `json:"name,omitempty"`
	Role         TenantUserRole `json:"role"`
	Status       string         `json:"status"`

	// SSO
	SSOProvider *string `json:"sso_provider,omitempty"`
	SSOID       *string `json:"sso_id,omitempty"`

	// MFA
	MFAEnabled bool    `json:"mfa_enabled"`
	MFASecret  *string `json:"-"` // Never expose

	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// IsOwner returns true if the user is the tenant owner
func (u *TenantUser) IsOwner() bool {
	return u.Role == TenantUserRoleOwner
}

// IsAdmin returns true if the user is an admin or owner
func (u *TenantUser) IsAdmin() bool {
	return u.Role == TenantUserRoleOwner || u.Role == TenantUserRoleAdmin
}

// CanManageUsers returns true if the user can manage other users
func (u *TenantUser) CanManageUsers() bool {
	return u.IsAdmin()
}

// TenantUsage represents daily usage statistics for a tenant
type TenantUsage struct {
	ID                uuid.UUID `json:"id"`
	TenantID          uuid.UUID `json:"tenant_id"`
	Date              time.Time `json:"date"`
	AgentCount        int       `json:"agent_count"`
	EventCount        int       `json:"event_count"`
	AIAnalysisCount   int       `json:"ai_analysis_count"`
	NotificationCount int       `json:"notification_count"`
	StorageUsedBytes  int64     `json:"storage_used_bytes"`
	PeakAgents        int       `json:"peak_agents"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// AuditLogEntry represents an audit log entry
type AuditLogEntry struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     *uuid.UUID `json:"tenant_id,omitempty"`
	UserID       *uuid.UUID `json:"user_id,omitempty"`
	AgentID      *uuid.UUID `json:"agent_id,omitempty"`
	Action       string     `json:"action"`
	ResourceType *string    `json:"resource_type,omitempty"`
	ResourceID   *string    `json:"resource_id,omitempty"`
	IPAddress    *string    `json:"ip_address,omitempty"`
	UserAgent    *string    `json:"user_agent,omitempty"`
	Details      JSONB      `json:"details,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}
