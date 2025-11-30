package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTenant_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status TenantStatus
		want   bool
	}{
		{"active tenant", TenantStatusActive, true},
		{"suspended tenant", TenantStatusSuspended, false},
		{"cancelled tenant", TenantStatusCancelled, false},
		{"pending tenant", TenantStatusPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant := &Tenant{Status: tt.status}
			if got := tenant.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenant_IsEnterprise(t *testing.T) {
	tests := []struct {
		name string
		plan TenantPlan
		want bool
	}{
		{"trial plan", TenantPlanTrial, false},
		{"starter plan", TenantPlanStarter, false},
		{"pro plan", TenantPlanPro, false},
		{"enterprise plan", TenantPlanEnterprise, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant := &Tenant{Plan: tt.plan}
			if got := tenant.IsEnterprise(); got != tt.want {
				t.Errorf("IsEnterprise() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenant_IsDedicated(t *testing.T) {
	tests := []struct {
		name          string
		isolationTier IsolationTier
		want          bool
	}{
		{"shared tier", IsolationTierShared, false},
		{"dedicated tier", IsolationTierDedicated, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant := &Tenant{IsolationTier: tt.isolationTier}
			if got := tenant.IsDedicated(); got != tt.want {
				t.Errorf("IsDedicated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenant_IsDefault(t *testing.T) {
	tests := []struct {
		name string
		id   uuid.UUID
		want bool
	}{
		{"default tenant", DefaultTenantID, true},
		{"non-default tenant", uuid.New(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant := &Tenant{ID: tt.id}
			if got := tenant.IsDefault(); got != tt.want {
				t.Errorf("IsDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenant_CanAddAgent(t *testing.T) {
	tenant := &Tenant{MaxAgents: 10}

	tests := []struct {
		name         string
		currentCount int
		want         bool
	}{
		{"under limit", 5, true},
		{"at limit minus one", 9, true},
		{"at limit", 10, false},
		{"over limit", 15, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tenant.CanAddAgent(tt.currentCount); got != tt.want {
				t.Errorf("CanAddAgent(%d) = %v, want %v", tt.currentCount, got, tt.want)
			}
		})
	}
}

func TestTenantAPIKey_IsRevoked(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		revokedAt *time.Time
		want      bool
	}{
		{"not revoked", nil, false},
		{"revoked", &now, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &TenantAPIKey{RevokedAt: tt.revokedAt}
			if got := key.IsRevoked(); got != tt.want {
				t.Errorf("IsRevoked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenantAPIKey_IsExpired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{"no expiration", nil, false},
		{"expired", &past, true},
		{"not expired", &future, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &TenantAPIKey{ExpiresAt: tt.expiresAt}
			if got := key.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenantAPIKey_IsValid(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)
	now := time.Now()

	tests := []struct {
		name      string
		revokedAt *time.Time
		expiresAt *time.Time
		want      bool
	}{
		{"valid - no expiration, not revoked", nil, nil, true},
		{"valid - future expiration", nil, &future, true},
		{"invalid - revoked", &now, nil, false},
		{"invalid - expired", nil, &past, false},
		{"invalid - revoked and expired", &now, &past, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &TenantAPIKey{
				RevokedAt: tt.revokedAt,
				ExpiresAt: tt.expiresAt,
			}
			if got := key.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenantAPIKey_HasScope(t *testing.T) {
	key := &TenantAPIKey{
		Scopes: []string{"agent:register", "agent:read", "events:write"},
	}

	tests := []struct {
		name  string
		scope string
		want  bool
	}{
		{"has scope", "agent:register", true},
		{"has another scope", "events:write", true},
		{"missing scope", "admin:all", false},
		{"empty scope", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := key.HasScope(tt.scope); got != tt.want {
				t.Errorf("HasScope(%q) = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestTenantUser_IsOwner(t *testing.T) {
	tests := []struct {
		name string
		role TenantUserRole
		want bool
	}{
		{"owner", TenantUserRoleOwner, true},
		{"admin", TenantUserRoleAdmin, false},
		{"member", TenantUserRoleMember, false},
		{"viewer", TenantUserRoleViewer, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &TenantUser{Role: tt.role}
			if got := user.IsOwner(); got != tt.want {
				t.Errorf("IsOwner() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenantUser_IsAdmin(t *testing.T) {
	tests := []struct {
		name string
		role TenantUserRole
		want bool
	}{
		{"owner is admin", TenantUserRoleOwner, true},
		{"admin is admin", TenantUserRoleAdmin, true},
		{"member is not admin", TenantUserRoleMember, false},
		{"viewer is not admin", TenantUserRoleViewer, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &TenantUser{Role: tt.role}
			if got := user.IsAdmin(); got != tt.want {
				t.Errorf("IsAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenantUser_CanManageUsers(t *testing.T) {
	tests := []struct {
		name string
		role TenantUserRole
		want bool
	}{
		{"owner can manage", TenantUserRoleOwner, true},
		{"admin can manage", TenantUserRoleAdmin, true},
		{"member cannot manage", TenantUserRoleMember, false},
		{"viewer cannot manage", TenantUserRoleViewer, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &TenantUser{Role: tt.role}
			if got := user.CanManageUsers(); got != tt.want {
				t.Errorf("CanManageUsers() = %v, want %v", got, tt.want)
			}
		})
	}
}
