package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/lib/pq"
)

// APIKeyRepository handles database operations for API keys
type APIKeyRepository struct {
	db *sql.DB
}

// NewAPIKeyRepository creates a new API key repository
func NewAPIKeyRepository(db *sql.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

// Create creates a new API key in the database
func (r *APIKeyRepository) Create(ctx context.Context, key *models.APIKey) error {
	query := `
		INSERT INTO api_keys (id, key_hash, name, scopes, created_at, expires_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	// Generate UUID if not provided
	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}

	// Set created_at if not provided
	if key.CreatedAt.IsZero() {
		key.CreatedAt = time.Now()
	}

	// Default scopes if empty
	if key.Scopes == nil {
		key.Scopes = []string{}
	}

	err := r.db.QueryRowContext(
		ctx,
		query,
		key.ID,
		key.KeyHash,
		key.Name,
		pq.Array(key.Scopes),
		key.CreatedAt,
		key.ExpiresAt,
		key.Metadata,
	).Scan(&key.ID, &key.CreatedAt)

	if err != nil {
		// Check for duplicate key_hash
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" { // unique_violation
				return fmt.Errorf("API key already exists")
			}
		}
		return fmt.Errorf("failed to create API key: %w", err)
	}

	return nil
}

// GetByID retrieves an API key by its ID
func (r *APIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	query := `
		SELECT id, key_hash, name, scopes, created_at, last_used_at,
		       expires_at, revoked, metadata
		FROM api_keys
		WHERE id = $1
	`

	key := &models.APIKey{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&key.ID,
		&key.KeyHash,
		&key.Name,
		pq.Array(&key.Scopes),
		&key.CreatedAt,
		&key.LastUsedAt,
		&key.ExpiresAt,
		&key.Revoked,
		&key.Metadata,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("API key not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return key, nil
}

// GetByHash retrieves an API key by its hash
func (r *APIKeyRepository) GetByHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	query := `
		SELECT id, key_hash, name, scopes, created_at, last_used_at,
		       expires_at, revoked, metadata
		FROM api_keys
		WHERE key_hash = $1 AND revoked = false
	`

	key := &models.APIKey{}
	err := r.db.QueryRowContext(ctx, query, keyHash).Scan(
		&key.ID,
		&key.KeyHash,
		&key.Name,
		pq.Array(&key.Scopes),
		&key.CreatedAt,
		&key.LastUsedAt,
		&key.ExpiresAt,
		&key.Revoked,
		&key.Metadata,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("API key not found or revoked")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return key, nil
}

// List retrieves all API keys (excluding revoked ones by default)
func (r *APIKeyRepository) List(ctx context.Context, includeRevoked bool) ([]*models.APIKey, error) {
	query := `
		SELECT id, key_hash, name, scopes, created_at, last_used_at,
		       expires_at, revoked, metadata
		FROM api_keys
	`

	if !includeRevoked {
		query += " WHERE revoked = false"
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	keys := make([]*models.APIKey, 0)
	for rows.Next() {
		key := &models.APIKey{}
		err := rows.Scan(
			&key.ID,
			&key.KeyHash,
			&key.Name,
			pq.Array(&key.Scopes),
			&key.CreatedAt,
			&key.LastUsedAt,
			&key.ExpiresAt,
			&key.Revoked,
			&key.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, key)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API keys: %w", err)
	}

	return keys, nil
}

// UpdateLastUsed updates the last_used_at timestamp
func (r *APIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE api_keys
		SET last_used_at = $1
		WHERE id = $2
	`

	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update last_used_at: %w", err)
	}

	return nil
}

// Revoke revokes an API key
func (r *APIKeyRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE api_keys
		SET revoked = true
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("API key not found: %s", id)
	}

	return nil
}

// Delete permanently deletes an API key
func (r *APIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM api_keys WHERE id = $1"

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("API key not found: %s", id)
	}

	return nil
}

// ValidateAndGet validates an API key and returns it if valid
// This also updates the last_used_at timestamp
func (r *APIKeyRepository) ValidateAndGet(ctx context.Context, plainKey string) (*models.APIKey, error) {
	// Hash the plain key
	keyHash := models.HashAPIKey(plainKey)

	// Get the key from database
	key, err := r.GetByHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}

	// Check if expired
	if key.IsExpired() {
		return nil, fmt.Errorf("API key has expired")
	}

	// Update last used timestamp (fire and forget)
	go r.UpdateLastUsed(context.Background(), key.ID)

	return key, nil
}
