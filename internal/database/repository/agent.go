package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/ignacio/lumo/internal/database/models"
)

// AgentRepository handles database operations for agents
type AgentRepository struct {
	db *sql.DB
}

// NewAgentRepository creates a new agent repository
func NewAgentRepository(db *sql.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

// Create creates a new agent registration
func (r *AgentRepository) Create(ctx context.Context, agent *models.Agent) error {
	// Generate UUID if not provided
	if agent.ID == uuid.Nil {
		agent.ID = uuid.New()
	}

	// Set timestamps
	now := time.Now()
	agent.RegisteredAt = now
	agent.UpdatedAt = now
	agent.LastHeartbeatAt = now

	// Serialize labels
	var labelsJSON []byte
	var err error
	if agent.Labels != nil {
		val, err := agent.Labels.Value()
		if err != nil {
			return fmt.Errorf("failed to marshal labels: %w", err)
		}
		if val != nil {
			labelsJSON = val.([]byte)
		}
	}

	// Serialize Kubernetes metadata if present
	var k8sMetadataJSON interface{} = nil
	if agent.KubernetesMetadata != nil {
		k8sJSON, err := json.Marshal(agent.KubernetesMetadata)
		if err != nil {
			return fmt.Errorf("failed to marshal kubernetes metadata: %w", err)
		}
		k8sMetadataJSON = k8sJSON
	}

	query := `
		INSERT INTO agents (
			id, name, hostname, ip_address, platform, architecture, version,
			status, capabilities, labels, kubernetes_metadata,
			last_heartbeat_at, registered_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, registered_at, updated_at, last_heartbeat_at
	`

	// Pass labels as string for PostgreSQL JSONB compatibility
	var labelsStr interface{} = nil
	if labelsJSON != nil {
		labelsStr = string(labelsJSON)
	}

	err = r.db.QueryRowContext(
		ctx,
		query,
		agent.ID,
		agent.Name,
		agent.Hostname,
		agent.IPAddress,
		agent.Platform,
		agent.Architecture,
		agent.Version,
		agent.Status,
		pq.Array(agent.Capabilities),
		labelsStr,
		k8sMetadataJSON,
		agent.LastHeartbeatAt,
		agent.RegisteredAt,
		agent.UpdatedAt,
	).Scan(&agent.ID, &agent.RegisteredAt, &agent.UpdatedAt, &agent.LastHeartbeatAt)

	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	return nil
}

// agentColumns is the list of columns for agent queries
const agentColumns = `id, name, hostname, ip_address, platform, architecture, version,
		   status, capabilities, labels, kubernetes_metadata,
		   last_heartbeat_at, registered_at, updated_at`

// scanAgent scans a database row into an Agent model.
// This helper function reduces duplication across GetByID, GetByHostname, and List.
func (r *AgentRepository) scanAgent(scanner interface {
	Scan(dest ...interface{}) error
}) (*models.Agent, error) {
	agent := &models.Agent{}
	var capabilities pq.StringArray
	var k8sMetadataJSON []byte

	err := scanner.Scan(
		&agent.ID,
		&agent.Name,
		&agent.Hostname,
		&agent.IPAddress,
		&agent.Platform,
		&agent.Architecture,
		&agent.Version,
		&agent.Status,
		&capabilities,
		&agent.Labels,
		&k8sMetadataJSON,
		&agent.LastHeartbeatAt,
		&agent.RegisteredAt,
		&agent.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	agent.Capabilities = capabilities

	// Deserialize Kubernetes metadata if present
	if len(k8sMetadataJSON) > 0 {
		var k8sMeta models.KubernetesMetadata
		if err := json.Unmarshal(k8sMetadataJSON, &k8sMeta); err != nil {
			return nil, fmt.Errorf("failed to unmarshal kubernetes metadata: %w", err)
		}
		agent.KubernetesMetadata = &k8sMeta
	}

	return agent, nil
}

// GetByID retrieves an agent by ID
func (r *AgentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	query := `SELECT ` + agentColumns + ` FROM agents WHERE id = $1`

	agent, err := r.scanAgent(r.db.QueryRowContext(ctx, query, id))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent not found: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return agent, nil
}

// GetByHostname retrieves an agent by hostname
func (r *AgentRepository) GetByHostname(ctx context.Context, hostname string) (*models.Agent, error) {
	query := `SELECT ` + agentColumns + ` FROM agents WHERE hostname = $1`

	agent, err := r.scanAgent(r.db.QueryRowContext(ctx, query, hostname))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent not found: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return agent, nil
}

// List retrieves agents with optional filtering
func (r *AgentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*models.Agent, error) {
	query := `SELECT ` + agentColumns + ` FROM agents WHERE 1=1`

	args := []interface{}{}
	argCount := 1

	// Apply filters
	if status, ok := filters["status"].(models.AgentStatus); ok {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	if platform, ok := filters["platform"].(models.AgentPlatform); ok {
		query += fmt.Sprintf(" AND platform = $%d", argCount)
		args = append(args, platform)
		argCount++
	}

	// Sorting
	if sort, ok := filters["sort"].(string); ok && sort != "" {
		switch sort {
		case "registered_at", "last_heartbeat_at", "name", "hostname":
			query += " ORDER BY " + sort + " DESC"
		default:
			query += " ORDER BY registered_at DESC"
		}
	} else {
		query += " ORDER BY registered_at DESC"
	}

	// Pagination
	if limit, ok := filters["limit"].(int); ok && limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
		argCount++
	}

	if offset, ok := filters["offset"].(int); ok && offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	agents := []*models.Agent{}
	for rows.Next() {
		agent, err := r.scanAgent(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}
		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate agents: %w", err)
	}

	return agents, nil
}

// Update updates an agent's information
func (r *AgentRepository) Update(ctx context.Context, agent *models.Agent) error {
	// Update timestamp
	agent.UpdatedAt = time.Now()

	// Serialize labels
	var labelsJSON []byte
	var err error
	if agent.Labels != nil {
		val, err := agent.Labels.Value()
		if err != nil {
			return fmt.Errorf("failed to marshal labels: %w", err)
		}
		if val != nil {
			labelsJSON = val.([]byte)
		}
	}

	// Serialize Kubernetes metadata if present
	var k8sMetadataJSON interface{} = nil
	if agent.KubernetesMetadata != nil {
		k8sJSON, err := json.Marshal(agent.KubernetesMetadata)
		if err != nil {
			return fmt.Errorf("failed to marshal kubernetes metadata: %w", err)
		}
		k8sMetadataJSON = k8sJSON
	}

	// Use string for labels like in Create
	var labelsStr interface{} = nil
	if labelsJSON != nil {
		labelsStr = string(labelsJSON)
	}

	query := `
		UPDATE agents
		SET name = $1, ip_address = $2, version = $3, status = $4,
		    capabilities = $5, labels = $6, kubernetes_metadata = $7, updated_at = $8
		WHERE id = $9
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		agent.Name,
		agent.IPAddress,
		agent.Version,
		agent.Status,
		pq.Array(agent.Capabilities),
		labelsStr,
		k8sMetadataJSON,
		agent.UpdatedAt,
		agent.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("agent not found")
	}

	return nil
}

// UpdateHeartbeat updates the last heartbeat timestamp and status
func (r *AgentRepository) UpdateHeartbeat(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE agents
		SET last_heartbeat_at = $1, status = $2, updated_at = $3
		WHERE id = $4
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, now, models.AgentStatusOnline, now, id)
	if err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("agent not found")
	}

	return nil
}

// UpdateStatus updates an agent's status
func (r *AgentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.AgentStatus) error {
	query := `
		UPDATE agents
		SET status = $1, updated_at = $2
		WHERE id = $3
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, status, now, id)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("agent not found")
	}

	return nil
}

// Delete deletes an agent by ID
func (r *AgentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM agents WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("agent not found")
	}

	return nil
}

// CountByStatus returns the count of agents by status
func (r *AgentRepository) CountByStatus(ctx context.Context) (map[models.AgentStatus]int, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM agents
		GROUP BY status
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count agents by status: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	counts := make(map[models.AgentStatus]int)
	for rows.Next() {
		var status models.AgentStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan status count: %w", err)
		}
		counts[status] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate status counts: %w", err)
	}

	return counts, nil
}

// MarkStaleAgentsOffline marks agents as offline if they haven't sent a heartbeat recently
func (r *AgentRepository) MarkStaleAgentsOffline(ctx context.Context, threshold time.Duration) (int64, error) {
	query := `
		UPDATE agents
		SET status = $1, updated_at = $2
		WHERE status = $3 AND last_heartbeat_at < $4
	`

	now := time.Now()
	cutoff := now.Add(-threshold)

	result, err := r.db.ExecContext(ctx, query, models.AgentStatusOffline, now, models.AgentStatusOnline, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to mark stale agents offline: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}

// CountByPlatform returns the count of agents by platform
func (r *AgentRepository) CountByPlatform(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT platform, COUNT(*) as count
		FROM agents
		GROUP BY platform
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count agents by platform: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	counts := make(map[string]int)
	for rows.Next() {
		var platform string
		var count int
		if err := rows.Scan(&platform, &count); err != nil {
			return nil, fmt.Errorf("failed to scan platform count: %w", err)
		}
		counts[platform] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate platform counts: %w", err)
	}

	return counts, nil
}
