package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/ignacio/lumo/internal/database/models"
)

// TenantRepository handles database operations for tenants
type TenantRepository struct {
	db *sql.DB
}

// NewTenantRepository creates a new tenant repository
func NewTenantRepository(db *sql.DB) *TenantRepository {
	return &TenantRepository{db: db}
}

// Create creates a new tenant
func (r *TenantRepository) Create(ctx context.Context, tenant *models.Tenant) error {
	if tenant.ID == uuid.Nil {
		tenant.ID = uuid.New()
	}

	now := time.Now()
	tenant.CreatedAt = now
	tenant.UpdatedAt = now

	query := `
		INSERT INTO tenants (
			id, name, slug, display_name, plan, plan_started_at, plan_expires_at,
			stripe_customer_id, stripe_subscription_id, max_agents, max_events_per_day,
			max_users, ai_analysis_enabled, retention_days, isolation_tier,
			dedicated_namespace, dedicated_db_host, status, suspended_reason,
			settings, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
		RETURNING id, created_at, updated_at
	`

	var settingsJSON interface{} = nil
	if tenant.Settings != nil {
		val, err := tenant.Settings.Value()
		if err != nil {
			return fmt.Errorf("failed to marshal settings: %w", err)
		}
		if val != nil {
			settingsJSON = string(val.([]byte))
		}
	}

	err := r.db.QueryRowContext(
		ctx,
		query,
		tenant.ID,
		tenant.Name,
		tenant.Slug,
		tenant.DisplayName,
		tenant.Plan,
		tenant.PlanStartedAt,
		tenant.PlanExpiresAt,
		tenant.StripeCustomerID,
		tenant.StripeSubscriptionID,
		tenant.MaxAgents,
		tenant.MaxEventsPerDay,
		tenant.MaxUsers,
		tenant.AIAnalysisEnabled,
		tenant.RetentionDays,
		tenant.IsolationTier,
		tenant.DedicatedNamespace,
		tenant.DedicatedDBHost,
		tenant.Status,
		tenant.SuspendedReason,
		settingsJSON,
		tenant.CreatedAt,
		tenant.UpdatedAt,
	).Scan(&tenant.ID, &tenant.CreatedAt, &tenant.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	return nil
}

// GetByID retrieves a tenant by ID
func (r *TenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error) {
	query := `
		SELECT id, name, slug, display_name, plan, plan_started_at, plan_expires_at,
			stripe_customer_id, stripe_subscription_id, max_agents, max_events_per_day,
			max_users, ai_analysis_enabled, retention_days, isolation_tier,
			dedicated_namespace, dedicated_db_host, status, suspended_reason,
			settings, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`

	return r.scanTenant(r.db.QueryRowContext(ctx, query, id))
}

// GetBySlug retrieves a tenant by slug
func (r *TenantRepository) GetBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	query := `
		SELECT id, name, slug, display_name, plan, plan_started_at, plan_expires_at,
			stripe_customer_id, stripe_subscription_id, max_agents, max_events_per_day,
			max_users, ai_analysis_enabled, retention_days, isolation_tier,
			dedicated_namespace, dedicated_db_host, status, suspended_reason,
			settings, created_at, updated_at
		FROM tenants
		WHERE slug = $1
	`

	return r.scanTenant(r.db.QueryRowContext(ctx, query, slug))
}

// GetDefault retrieves the default tenant
func (r *TenantRepository) GetDefault(ctx context.Context) (*models.Tenant, error) {
	return r.GetByID(ctx, models.DefaultTenantID)
}

// List retrieves tenants with optional filters
func (r *TenantRepository) List(ctx context.Context, filters map[string]interface{}) ([]*models.Tenant, error) {
	query := `
		SELECT id, name, slug, display_name, plan, plan_started_at, plan_expires_at,
			stripe_customer_id, stripe_subscription_id, max_agents, max_events_per_day,
			max_users, ai_analysis_enabled, retention_days, isolation_tier,
			dedicated_namespace, dedicated_db_host, status, suspended_reason,
			settings, created_at, updated_at
		FROM tenants
		WHERE 1=1
	`

	args := []interface{}{}
	argIndex := 1

	if status, ok := filters["status"].(models.TenantStatus); ok {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, status)
		argIndex++
	}

	if plan, ok := filters["plan"].(models.TenantPlan); ok {
		query += fmt.Sprintf(" AND plan = $%d", argIndex)
		args = append(args, plan)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	if limit, ok := filters["limit"].(int); ok {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, limit)
		argIndex++
	}

	if offset, ok := filters["offset"].(int); ok {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tenants []*models.Tenant
	for rows.Next() {
		tenant, err := r.scanTenantRow(rows)
		if err != nil {
			return nil, err
		}
		tenants = append(tenants, tenant)
	}

	return tenants, rows.Err()
}

// Update updates a tenant
func (r *TenantRepository) Update(ctx context.Context, tenant *models.Tenant) error {
	tenant.UpdatedAt = time.Now()

	var settingsJSON interface{} = nil
	if tenant.Settings != nil {
		val, err := tenant.Settings.Value()
		if err != nil {
			return fmt.Errorf("failed to marshal settings: %w", err)
		}
		if val != nil {
			settingsJSON = string(val.([]byte))
		}
	}

	query := `
		UPDATE tenants SET
			name = $2, display_name = $3, plan = $4, plan_started_at = $5,
			plan_expires_at = $6, stripe_customer_id = $7, stripe_subscription_id = $8,
			max_agents = $9, max_events_per_day = $10, max_users = $11,
			ai_analysis_enabled = $12, retention_days = $13, isolation_tier = $14,
			dedicated_namespace = $15, dedicated_db_host = $16, status = $17,
			suspended_reason = $18, settings = $19, updated_at = $20
		WHERE id = $1
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		tenant.ID,
		tenant.Name,
		tenant.DisplayName,
		tenant.Plan,
		tenant.PlanStartedAt,
		tenant.PlanExpiresAt,
		tenant.StripeCustomerID,
		tenant.StripeSubscriptionID,
		tenant.MaxAgents,
		tenant.MaxEventsPerDay,
		tenant.MaxUsers,
		tenant.AIAnalysisEnabled,
		tenant.RetentionDays,
		tenant.IsolationTier,
		tenant.DedicatedNamespace,
		tenant.DedicatedDBHost,
		tenant.Status,
		tenant.SuspendedReason,
		settingsJSON,
		tenant.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update tenant: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// UpdateStatus updates the tenant status
func (r *TenantRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.TenantStatus, reason *string) error {
	query := `
		UPDATE tenants SET status = $2, suspended_reason = $3, updated_at = $4
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, status, reason, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update tenant status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// Delete soft-deletes a tenant by setting status to cancelled
func (r *TenantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.UpdateStatus(ctx, id, models.TenantStatusCancelled, nil)
}

// CountByStatus returns tenant counts grouped by status
func (r *TenantRepository) CountByStatus(ctx context.Context) (map[models.TenantStatus]int, error) {
	query := `
		SELECT status, COUNT(*) FROM tenants GROUP BY status
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count tenants by status: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[models.TenantStatus]int)
	for rows.Next() {
		var status models.TenantStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan count: %w", err)
		}
		counts[status] = count
	}

	return counts, rows.Err()
}

// scanTenant scans a single tenant row
func (r *TenantRepository) scanTenant(row *sql.Row) (*models.Tenant, error) {
	var tenant models.Tenant
	var settings sql.NullString

	err := row.Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.Slug,
		&tenant.DisplayName,
		&tenant.Plan,
		&tenant.PlanStartedAt,
		&tenant.PlanExpiresAt,
		&tenant.StripeCustomerID,
		&tenant.StripeSubscriptionID,
		&tenant.MaxAgents,
		&tenant.MaxEventsPerDay,
		&tenant.MaxUsers,
		&tenant.AIAnalysisEnabled,
		&tenant.RetentionDays,
		&tenant.IsolationTier,
		&tenant.DedicatedNamespace,
		&tenant.DedicatedDBHost,
		&tenant.Status,
		&tenant.SuspendedReason,
		&settings,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("failed to scan tenant: %w", err)
	}

	if settings.Valid {
		tenant.Settings = models.JSONB{}
		if err := tenant.Settings.Scan([]byte(settings.String)); err != nil {
			return nil, fmt.Errorf("failed to parse settings: %w", err)
		}
	}

	return &tenant, nil
}

// scanTenantRow scans a tenant from rows
func (r *TenantRepository) scanTenantRow(rows *sql.Rows) (*models.Tenant, error) {
	var tenant models.Tenant
	var settings sql.NullString

	err := rows.Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.Slug,
		&tenant.DisplayName,
		&tenant.Plan,
		&tenant.PlanStartedAt,
		&tenant.PlanExpiresAt,
		&tenant.StripeCustomerID,
		&tenant.StripeSubscriptionID,
		&tenant.MaxAgents,
		&tenant.MaxEventsPerDay,
		&tenant.MaxUsers,
		&tenant.AIAnalysisEnabled,
		&tenant.RetentionDays,
		&tenant.IsolationTier,
		&tenant.DedicatedNamespace,
		&tenant.DedicatedDBHost,
		&tenant.Status,
		&tenant.SuspendedReason,
		&settings,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan tenant row: %w", err)
	}

	if settings.Valid {
		tenant.Settings = models.JSONB{}
		if err := tenant.Settings.Scan([]byte(settings.String)); err != nil {
			return nil, fmt.Errorf("failed to parse settings: %w", err)
		}
	}

	return &tenant, nil
}

// ============================================
// TENANT API KEY OPERATIONS
// ============================================

// CreateAPIKey creates a new API key for a tenant
// Returns the full key (only available once) and the stored key model
func (r *TenantRepository) CreateAPIKey(ctx context.Context, key *models.TenantAPIKey, rawKey string) error {
	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}

	key.CreatedAt = time.Now()

	// Hash the key
	hash := sha256.Sum256([]byte(rawKey))
	key.KeyHash = hex.EncodeToString(hash[:])

	// Extract prefix (first 12 chars)
	if len(rawKey) >= 12 {
		key.KeyPrefix = rawKey[:12]
	}

	query := `
		INSERT INTO tenant_api_keys (
			id, tenant_id, name, key_hash, key_prefix, scopes,
			expires_at, created_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		key.ID,
		key.TenantID,
		key.Name,
		key.KeyHash,
		key.KeyPrefix,
		pq.Array(key.Scopes),
		key.ExpiresAt,
		key.CreatedBy,
		key.CreatedAt,
	).Scan(&key.ID, &key.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	return nil
}

// GetAPIKeyByPrefix retrieves an API key by its prefix
func (r *TenantRepository) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*models.TenantAPIKey, error) {
	query := `
		SELECT id, tenant_id, name, key_hash, key_prefix, scopes,
			last_used_at, expires_at, created_by, created_at, revoked_at
		FROM tenant_api_keys
		WHERE key_prefix = $1
	`

	return r.scanAPIKey(r.db.QueryRowContext(ctx, query, prefix))
}

// ValidateAPIKey validates an API key and returns the associated key model
func (r *TenantRepository) ValidateAPIKey(ctx context.Context, rawKey string) (*models.TenantAPIKey, error) {
	if len(rawKey) < 12 {
		return nil, fmt.Errorf("invalid API key format")
	}

	prefix := rawKey[:12]
	key, err := r.GetAPIKeyByPrefix(ctx, prefix)
	if err != nil {
		return nil, err
	}

	// Verify hash
	hash := sha256.Sum256([]byte(rawKey))
	expectedHash := hex.EncodeToString(hash[:])

	if key.KeyHash != expectedHash {
		return nil, fmt.Errorf("invalid API key")
	}

	if !key.IsValid() {
		return nil, fmt.Errorf("API key is expired or revoked")
	}

	// Update last used timestamp
	_, _ = r.db.ExecContext(ctx,
		"UPDATE tenant_api_keys SET last_used_at = $1 WHERE id = $2",
		time.Now(), key.ID)

	return key, nil
}

// ListAPIKeys lists all API keys for a tenant
func (r *TenantRepository) ListAPIKeys(ctx context.Context, tenantID uuid.UUID) ([]*models.TenantAPIKey, error) {
	query := `
		SELECT id, tenant_id, name, key_hash, key_prefix, scopes,
			last_used_at, expires_at, created_by, created_at, revoked_at
		FROM tenant_api_keys
		WHERE tenant_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var keys []*models.TenantAPIKey
	for rows.Next() {
		key, err := r.scanAPIKeyRow(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// RevokeAPIKey revokes an API key
func (r *TenantRepository) RevokeAPIKey(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tenant_api_keys SET revoked_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

// scanAPIKey scans a single API key row
func (r *TenantRepository) scanAPIKey(row *sql.Row) (*models.TenantAPIKey, error) {
	var key models.TenantAPIKey

	err := row.Scan(
		&key.ID,
		&key.TenantID,
		&key.Name,
		&key.KeyHash,
		&key.KeyPrefix,
		pq.Array(&key.Scopes),
		&key.LastUsedAt,
		&key.ExpiresAt,
		&key.CreatedBy,
		&key.CreatedAt,
		&key.RevokedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("failed to scan API key: %w", err)
	}

	return &key, nil
}

// scanAPIKeyRow scans an API key from rows
func (r *TenantRepository) scanAPIKeyRow(rows *sql.Rows) (*models.TenantAPIKey, error) {
	var key models.TenantAPIKey

	err := rows.Scan(
		&key.ID,
		&key.TenantID,
		&key.Name,
		&key.KeyHash,
		&key.KeyPrefix,
		pq.Array(&key.Scopes),
		&key.LastUsedAt,
		&key.ExpiresAt,
		&key.CreatedBy,
		&key.CreatedAt,
		&key.RevokedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan API key row: %w", err)
	}

	return &key, nil
}
