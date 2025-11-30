package middleware

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/api/auth"
	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
)

// Context keys for tenant information (using the existing contextKey type from auth.go)
const (
	TenantIDKey     contextKey = "tenant_id"
	TenantSlugKey   contextKey = "tenant_slug"
	TenantKey       contextKey = "tenant"
	ClaimsKey       contextKey = "claims"
	TenantSchemaKey contextKey = "tenant_schema"
)

// GetTenantID extracts the tenant ID from the context
func GetTenantID(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(TenantIDKey).(uuid.UUID); ok {
		return id
	}
	return models.DefaultTenantID
}

// GetTenantSlug extracts the tenant slug from the context
func GetTenantSlug(ctx context.Context) string {
	if slug, ok := ctx.Value(TenantSlugKey).(string); ok {
		return slug
	}
	return "default"
}

// GetTenantSchema returns the schema name for the current tenant
func GetTenantSchema(ctx context.Context) string {
	if schema, ok := ctx.Value(TenantSchemaKey).(string); ok {
		return schema
	}
	slug := GetTenantSlug(ctx)
	if slug == "default" || slug == "" {
		return "public"
	}
	return fmt.Sprintf("tenant_%s", slug)
}

// GetTenant extracts the full tenant from the context
func GetTenant(ctx context.Context) *models.Tenant {
	if tenant, ok := ctx.Value(TenantKey).(*models.Tenant); ok {
		return tenant
	}
	return nil
}

// GetClaims extracts the JWT claims from the context
func GetClaims(ctx context.Context) *auth.Claims {
	if claims, ok := ctx.Value(ClaimsKey).(*auth.Claims); ok {
		return claims
	}
	return nil
}

// TenantContext extracts tenant information from JWT and adds it to the context.
// It also validates that the tenant exists and is active.
func TenantContext(tenantRepo *repository.TenantRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get claims from context (set by JWT middleware)
			claims := GetClaims(r.Context())
			if claims == nil {
				// No claims means no auth required or auth failed
				// Use default tenant for backwards compatibility
				ctx := context.WithValue(r.Context(), TenantIDKey, models.DefaultTenantID)
				ctx = context.WithValue(ctx, TenantSlugKey, "default")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// If no tenant in claims, use default tenant
			if claims.TenantID == "" {
				ctx := context.WithValue(r.Context(), TenantIDKey, models.DefaultTenantID)
				ctx = context.WithValue(ctx, TenantSlugKey, "default")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Parse tenant ID
			tenantID, err := uuid.Parse(claims.TenantID)
			if err != nil {
				response.BadRequest(w, "Invalid tenant ID in token")
				return
			}

			// Fetch tenant from database
			tenant, err := tenantRepo.GetByID(r.Context(), tenantID)
			if err != nil {
				response.Unauthorized(w, "Tenant not found")
				return
			}

			// Check tenant status
			if !tenant.IsActive() {
				response.Error(w, http.StatusForbidden, "forbidden", "Tenant is suspended or cancelled")
				return
			}

			// Add tenant info to context
			ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
			ctx = context.WithValue(ctx, TenantSlugKey, tenant.Slug)
			ctx = context.WithValue(ctx, TenantKey, tenant)

			// Set schema name for tenant-scoped database queries
			schemaName := "public" // default uses public schema
			if tenant.Slug != "default" {
				schemaName = fmt.Sprintf("tenant_%s", tenant.Slug)
			}
			ctx = context.WithValue(ctx, TenantSchemaKey, schemaName)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireTenant ensures that a valid tenant is present in the context.
// Use this for endpoints that require tenant-scoped access.
func RequireTenant() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := GetTenantID(r.Context())
			if tenantID == uuid.Nil {
				response.Unauthorized(w, "Tenant context required")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireEnterpriseTenant ensures the tenant is on the enterprise plan.
// Use this for endpoints that are only available to enterprise customers.
func RequireEnterpriseTenant() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := GetTenant(r.Context())
			if tenant == nil {
				response.Unauthorized(w, "Tenant context required")
				return
			}

			if !tenant.IsEnterprise() {
				response.Error(w, http.StatusPaymentRequired, "payment_required", "Enterprise plan required")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TenantRateLimiter applies per-tenant rate limits based on the plan.
// This extends the base rate limiter with tenant-aware limits.
func TenantRateLimiter(baseLimiter func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := GetTenant(r.Context())
			if tenant == nil {
				// No tenant context, use base limiter
				baseLimiter(next).ServeHTTP(w, r)
				return
			}

			// Enterprise tenants get higher limits (handled by dedicated infrastructure)
			if tenant.IsEnterprise() && tenant.IsDedicated() {
				// Skip rate limiting for dedicated enterprise tenants
				next.ServeHTTP(w, r)
				return
			}

			// Apply base rate limiter for shared tenants
			baseLimiter(next).ServeHTTP(w, r)
		})
	}
}

// StoreClaimsInContext stores JWT claims in the request context.
// This is called by the JWT authentication middleware.
func StoreClaimsInContext(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, ClaimsKey, claims)
}

// SetTenantSchema sets the PostgreSQL search_path to the tenant's schema.
// This should be called at the beginning of database transactions for tenant-scoped queries.
// For the default tenant or when no tenant context exists, it uses the public schema.
func SetTenantSchema(ctx context.Context, db *sql.DB, logger *logrus.Logger) error {
	schema := GetTenantSchema(ctx)

	// Set the search_path for this connection
	// Using search_path ensures all unqualified table references use the tenant schema
	query := fmt.Sprintf("SET search_path TO %s, public", schema)

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		if logger != nil {
			logger.WithError(err).WithField("schema", schema).Error("Failed to set tenant schema")
		}
		return fmt.Errorf("failed to set tenant schema %s: %w", schema, err)
	}

	if logger != nil {
		logger.WithField("schema", schema).Debug("Set tenant schema search_path")
	}

	return nil
}

// WithTenantSchema wraps a database operation with tenant schema context.
// It sets the schema at the start and resets to public after completion.
// This is useful for single-query operations that need tenant isolation.
func WithTenantSchema(ctx context.Context, db *sql.DB, logger *logrus.Logger, fn func() error) error {
	// Set tenant schema
	if err := SetTenantSchema(ctx, db, logger); err != nil {
		return err
	}

	// Execute the function
	fnErr := fn()

	// Reset to public schema (best effort)
	_, _ = db.ExecContext(ctx, "SET search_path TO public")

	return fnErr
}
