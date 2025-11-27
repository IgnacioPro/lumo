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

// EventRepository handles database operations for Kubernetes events
type EventRepository struct {
	db *sql.DB
}

// NewEventRepository creates a new event repository
func NewEventRepository(db *sql.DB) *EventRepository {
	return &EventRepository{db: db}
}

// Create creates a single event
func (r *EventRepository) Create(ctx context.Context, event *models.Event) error {
	// Generate UUID if not provided
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}

	// Set timestamps
	now := time.Now()
	event.CreatedAt = now
	event.UpdatedAt = now

	// Serialize metadata
	var metadataJSON interface{} = nil
	if event.Metadata != nil {
		metaJSON, err := json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = string(metaJSON)
	}

	query := `
		INSERT INTO events (
			id, agent_id, event_type, severity, resource_kind, resource_name,
			resource_uid, namespace, message, metadata, event_timestamp,
			notification_sent, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		event.ID,
		event.AgentID,
		event.EventType,
		event.Severity,
		event.ResourceKind,
		event.ResourceName,
		event.ResourceUID,
		event.Namespace,
		event.Message,
		metadataJSON,
		event.EventTimestamp,
		event.NotificationSent,
		event.CreatedAt,
		event.UpdatedAt,
	).Scan(&event.ID, &event.CreatedAt, &event.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	return nil
}

// CreateBatch creates multiple events in a single transaction
func (r *EventRepository) CreateBatch(ctx context.Context, events []*models.Event) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO events (
			id, agent_id, event_type, severity, resource_kind, resource_name,
			resource_uid, namespace, message, metadata, event_timestamp,
			notification_sent, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		_ = stmt.Close()
	}()

	now := time.Now()
	for _, event := range events {
		// Generate UUID if not provided
		if event.ID == uuid.Nil {
			event.ID = uuid.New()
		}

		// Set timestamps
		event.CreatedAt = now
		event.UpdatedAt = now

		// Serialize metadata
		var metadataJSON interface{} = nil
		if event.Metadata != nil {
			metaJSON, err := json.Marshal(event.Metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}
			metadataJSON = string(metaJSON)
		}

		_, err := stmt.ExecContext(
			ctx,
			event.ID,
			event.AgentID,
			event.EventType,
			event.Severity,
			event.ResourceKind,
			event.ResourceName,
			event.ResourceUID,
			event.Namespace,
			event.Message,
			metadataJSON,
			event.EventTimestamp,
			event.NotificationSent,
			event.CreatedAt,
			event.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetByID retrieves an event by ID
func (r *EventRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Event, error) {
	query := `
		SELECT id, agent_id, event_type, severity, resource_kind, resource_name,
		       resource_uid, namespace, message, metadata, event_timestamp,
		       ai_analysis, ai_analyzed_at, notification_sent, notification_sent_at,
		       notification_channels, created_at, updated_at
		FROM events
		WHERE id = $1
	`

	event := &models.Event{}
	var metadataJSON []byte
	var notificationChannels pq.StringArray

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.AgentID,
		&event.EventType,
		&event.Severity,
		&event.ResourceKind,
		&event.ResourceName,
		&event.ResourceUID,
		&event.Namespace,
		&event.Message,
		&metadataJSON,
		&event.EventTimestamp,
		&event.AIAnalysis,
		&event.AIAnalyzedAt,
		&event.NotificationSent,
		&event.NotificationSentAt,
		&notificationChannels,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("event not found: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	event.NotificationChannels = notificationChannels

	// Deserialize metadata if present
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return event, nil
}

// List retrieves events with optional filtering
func (r *EventRepository) List(ctx context.Context, filters map[string]interface{}) ([]*models.Event, error) {
	query := `
		SELECT id, agent_id, event_type, severity, resource_kind, resource_name,
		       resource_uid, namespace, message, metadata, event_timestamp,
		       ai_analysis, ai_analyzed_at, notification_sent, notification_sent_at,
		       notification_channels, created_at, updated_at
		FROM events
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 1

	// Apply filters
	if agentID, ok := filters["agent_id"].(uuid.UUID); ok {
		query += fmt.Sprintf(" AND agent_id = $%d", argCount)
		args = append(args, agentID)
		argCount++
	}

	if severity, ok := filters["severity"].(models.EventSeverity); ok {
		query += fmt.Sprintf(" AND severity = $%d", argCount)
		args = append(args, severity)
		argCount++
	} else if severities, ok := filters["severities"].([]models.EventSeverity); ok && len(severities) > 0 {
		// Support multiple severities with IN clause
		query += fmt.Sprintf(" AND severity = ANY($%d)", argCount)
		// Convert to string slice for pq.Array
		strSeverities := make([]string, len(severities))
		for i, s := range severities {
			strSeverities[i] = string(s)
		}
		args = append(args, pq.Array(strSeverities))
		argCount++
	}

	if eventType, ok := filters["event_type"].(string); ok {
		query += fmt.Sprintf(" AND event_type = $%d", argCount)
		args = append(args, eventType)
		argCount++
	}

	if namespace, ok := filters["namespace"].(string); ok {
		query += fmt.Sprintf(" AND namespace = $%d", argCount)
		args = append(args, namespace)
		argCount++
	}

	if resourceKind, ok := filters["resource_kind"].(string); ok {
		query += fmt.Sprintf(" AND resource_kind = $%d", argCount)
		args = append(args, resourceKind)
		argCount++
	}

	if resourceUID, ok := filters["resource_uid"].(string); ok {
		query += fmt.Sprintf(" AND resource_uid = $%d", argCount)
		args = append(args, resourceUID)
		argCount++
	}

	// Time range filtering
	if since, ok := filters["since"].(time.Time); ok {
		query += fmt.Sprintf(" AND event_timestamp >= $%d", argCount)
		args = append(args, since)
		argCount++
	}

	if until, ok := filters["until"].(time.Time); ok {
		query += fmt.Sprintf(" AND event_timestamp <= $%d", argCount)
		args = append(args, until)
		argCount++
	}

	// Notification status
	if notificationSent, ok := filters["notification_sent"].(bool); ok {
		query += fmt.Sprintf(" AND notification_sent = $%d", argCount)
		args = append(args, notificationSent)
		argCount++
	}

	// AI analysis status
	if hasAIAnalysis, ok := filters["has_ai_analysis"].(bool); ok {
		if hasAIAnalysis {
			query += " AND ai_analysis IS NOT NULL AND ai_analysis != ''"
		} else {
			query += " AND (ai_analysis IS NULL OR ai_analysis = '')"
		}
	}

	// Sorting - use safe column mapping to prevent SQL injection
	sortColumnMap := map[string]string{
		"event_timestamp": "event_timestamp",
		"created_at":      "created_at",
		"severity":        "severity",
		"event_type":      "event_type",
	}
	sortColumn := "event_timestamp" // default
	if sort, ok := filters["sort"].(string); ok && sort != "" {
		if col, valid := sortColumnMap[sort]; valid {
			sortColumn = col
		}
	}
	query += " ORDER BY " + sortColumn + " DESC"

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
		return nil, fmt.Errorf("failed to list events: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	events := []*models.Event{}
	for rows.Next() {
		event := &models.Event{}
		var metadataJSON []byte
		var notificationChannels pq.StringArray

		err := rows.Scan(
			&event.ID,
			&event.AgentID,
			&event.EventType,
			&event.Severity,
			&event.ResourceKind,
			&event.ResourceName,
			&event.ResourceUID,
			&event.Namespace,
			&event.Message,
			&metadataJSON,
			&event.EventTimestamp,
			&event.AIAnalysis,
			&event.AIAnalyzedAt,
			&event.NotificationSent,
			&event.NotificationSentAt,
			&notificationChannels,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		event.NotificationChannels = notificationChannels

		// Deserialize metadata if present
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate events: %w", err)
	}

	return events, nil
}

// UpdateAIAnalysis updates the AI analysis for an event
func (r *EventRepository) UpdateAIAnalysis(ctx context.Context, id uuid.UUID, analysis string) error {
	query := `
		UPDATE events
		SET ai_analysis = $1, ai_analyzed_at = $2, updated_at = $3
		WHERE id = $4
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, analysis, now, now, id)
	if err != nil {
		return fmt.Errorf("failed to update AI analysis: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("event not found")
	}

	return nil
}

// UpdateNotificationStatus updates the notification status for an event
func (r *EventRepository) UpdateNotificationStatus(ctx context.Context, id uuid.UUID, sent bool, channels []string) error {
	query := `
		UPDATE events
		SET notification_sent = $1, notification_sent_at = $2, notification_channels = $3, updated_at = $4
		WHERE id = $5
	`

	now := time.Now()
	var sentAt *time.Time
	if sent {
		sentAt = &now
	}

	result, err := r.db.ExecContext(ctx, query, sent, sentAt, pq.Array(channels), now, id)
	if err != nil {
		return fmt.Errorf("failed to update notification status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("event not found")
	}

	return nil
}

// GetUnnotifiedHighPriority retrieves unnotified high/critical severity events
func (r *EventRepository) GetUnnotifiedHighPriority(ctx context.Context, limit int) ([]*models.Event, error) {
	query := `
		SELECT id, agent_id, event_type, severity, resource_kind, resource_name,
		       resource_uid, namespace, message, metadata, event_timestamp,
		       ai_analysis, ai_analyzed_at, notification_sent, notification_sent_at,
		       notification_channels, created_at, updated_at
		FROM events
		WHERE notification_sent = FALSE
		  AND severity IN ('high', 'critical')
		ORDER BY event_timestamp DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get unnotified high priority events: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	events := []*models.Event{}
	for rows.Next() {
		event := &models.Event{}
		var metadataJSON []byte
		var notificationChannels pq.StringArray

		err := rows.Scan(
			&event.ID,
			&event.AgentID,
			&event.EventType,
			&event.Severity,
			&event.ResourceKind,
			&event.ResourceName,
			&event.ResourceUID,
			&event.Namespace,
			&event.Message,
			&metadataJSON,
			&event.EventTimestamp,
			&event.AIAnalysis,
			&event.AIAnalyzedAt,
			&event.NotificationSent,
			&event.NotificationSentAt,
			&notificationChannels,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		event.NotificationChannels = notificationChannels

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate events: %w", err)
	}

	return events, nil
}

// CountBySeverity returns the count of events by severity
func (r *EventRepository) CountBySeverity(ctx context.Context) (map[models.EventSeverity]int, error) {
	query := `
		SELECT severity, COUNT(*) as count
		FROM events
		GROUP BY severity
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count events by severity: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	counts := make(map[models.EventSeverity]int)
	for rows.Next() {
		var severity models.EventSeverity
		var count int
		if err := rows.Scan(&severity, &count); err != nil {
			return nil, fmt.Errorf("failed to scan severity count: %w", err)
		}
		counts[severity] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate severity counts: %w", err)
	}

	return counts, nil
}

// Delete deletes an event by ID
func (r *EventRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM events WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("event not found")
	}

	return nil
}

// DeleteOlderThan deletes events older than the specified duration
func (r *EventRepository) DeleteOlderThan(ctx context.Context, retention time.Duration) (int64, error) {
	query := `DELETE FROM events WHERE event_timestamp < $1`

	cutoff := time.Now().Add(-retention)
	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old events: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}
