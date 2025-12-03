package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/correlation"
	"github.com/ignacio/lumo/internal/database/models"
)

// IncidentRepository handles incident persistence in PostgreSQL
type IncidentRepository struct {
	db     *sql.DB
	logger *logrus.Entry
}

// NewIncidentRepository creates a new incident repository
func NewIncidentRepository(db *sql.DB, logger *logrus.Logger) *IncidentRepository {
	return &IncidentRepository{
		db:     db,
		logger: logger.WithField("component", "incident-repo"),
	}
}

// Create stores a new incident
func (r *IncidentRepository) Create(ctx context.Context, incident *correlation.Incident) error {
	// Serialize complex fields
	aiAnalysisJSON, err := json.Marshal(incident.AIAnalysis)
	if err != nil {
		return fmt.Errorf("failed to marshal ai_analysis: %w", err)
	}

	affectedResourcesJSON, err := json.Marshal(incident.AffectedResources)
	if err != nil {
		return fmt.Errorf("failed to marshal affected_resources: %w", err)
	}

	primaryResourceJSON, err := json.Marshal(incident.PrimaryResource)
	if err != nil {
		return fmt.Errorf("failed to marshal primary_resource: %w", err)
	}

	contextJSON, err := json.Marshal(incident.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	timelineJSON, err := json.Marshal(incident.Timeline)
	if err != nil {
		return fmt.Errorf("failed to marshal timeline: %w", err)
	}

	metadataJSON, err := json.Marshal(incident.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO incidents (
			id, tenant_id, category, severity, status, is_critical, title,
			correlation_key, correlation_reason, root_cause, summary, postmortem,
			ai_analysis, affected_resources, primary_resource, cluster_name,
			node_name, namespace, context, timeline, metadata, first_event_at,
			last_event_at, opened_at, closed_at, analyzed_at, notified_at,
			last_health_check_at, notification_channels, notification_count,
			last_notification_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31
		)`

	_, err = r.db.ExecContext(ctx, query,
		incident.ID,
		incident.TenantID,
		incident.Category,
		incident.Severity,
		incident.State,
		incident.IsCritical,
		incident.Title,
		incident.CorrelationKey,
		incident.CorrelationReason,
		incident.RootCause,
		incident.Summary,
		incident.Postmortem,
		aiAnalysisJSON,
		affectedResourcesJSON,
		primaryResourceJSON,
		incident.ClusterName,
		incident.NodeName,
		incident.Namespace,
		contextJSON,
		timelineJSON,
		metadataJSON,
		incident.FirstEventAt,
		incident.LastEventAt,
		incident.OpenedAt,
		incident.ClosedAt,
		incident.AnalyzedAt,
		incident.NotifiedAt,
		incident.LastHealthCheckAt,
		pq.Array(incident.NotificationChannels),
		incident.NotificationCount,
		incident.LastNotificationAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create incident: %w", err)
	}

	r.logger.WithField("incident_id", incident.ID).Debug("Incident created")
	return nil
}

// Update updates an existing incident
func (r *IncidentRepository) Update(ctx context.Context, incident *correlation.Incident) error {
	// Serialize complex fields
	aiAnalysisJSON, err := json.Marshal(incident.AIAnalysis)
	if err != nil {
		return fmt.Errorf("failed to marshal ai_analysis: %w", err)
	}

	affectedResourcesJSON, err := json.Marshal(incident.AffectedResources)
	if err != nil {
		return fmt.Errorf("failed to marshal affected_resources: %w", err)
	}

	primaryResourceJSON, err := json.Marshal(incident.PrimaryResource)
	if err != nil {
		return fmt.Errorf("failed to marshal primary_resource: %w", err)
	}

	contextJSON, err := json.Marshal(incident.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	timelineJSON, err := json.Marshal(incident.Timeline)
	if err != nil {
		return fmt.Errorf("failed to marshal timeline: %w", err)
	}

	metadataJSON, err := json.Marshal(incident.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE incidents SET
			category = $2, severity = $3, status = $4, is_critical = $5, title = $6,
			correlation_key = $7, correlation_reason = $8, root_cause = $9,
			summary = $10, postmortem = $11, ai_analysis = $12, affected_resources = $13,
			primary_resource = $14, cluster_name = $15, node_name = $16, namespace = $17,
			context = $18, timeline = $19, metadata = $20, first_event_at = $21,
			last_event_at = $22, closed_at = $23, analyzed_at = $24, notified_at = $25,
			last_health_check_at = $26, notification_channels = $27, notification_count = $28,
			last_notification_at = $29, updated_at = NOW()
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		incident.ID,
		incident.Category,
		incident.Severity,
		incident.State,
		incident.IsCritical,
		incident.Title,
		incident.CorrelationKey,
		incident.CorrelationReason,
		incident.RootCause,
		incident.Summary,
		incident.Postmortem,
		aiAnalysisJSON,
		affectedResourcesJSON,
		primaryResourceJSON,
		incident.ClusterName,
		incident.NodeName,
		incident.Namespace,
		contextJSON,
		timelineJSON,
		metadataJSON,
		incident.FirstEventAt,
		incident.LastEventAt,
		incident.ClosedAt,
		incident.AnalyzedAt,
		incident.NotifiedAt,
		incident.LastHealthCheckAt,
		pq.Array(incident.NotificationChannels),
		incident.NotificationCount,
		incident.LastNotificationAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update incident: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("incident not found: %s", incident.ID)
	}

	r.logger.WithField("incident_id", incident.ID).Debug("Incident updated")
	return nil
}

// GetByID retrieves an incident by ID
func (r *IncidentRepository) GetByID(ctx context.Context, id uuid.UUID) (*correlation.Incident, error) {
	query := `
		SELECT id, tenant_id, category, severity, status, is_critical, title,
			correlation_key, correlation_reason, root_cause, summary, postmortem,
			ai_analysis, affected_resources, primary_resource, cluster_name,
			node_name, namespace, context, timeline, metadata, first_event_at,
			last_event_at, opened_at, closed_at, analyzed_at, notified_at,
			last_health_check_at, notification_channels, notification_count,
			last_notification_at
		FROM incidents WHERE id = $1`

	return r.scanIncident(r.db.QueryRowContext(ctx, query, id))
}

// GetByCorrelationKey retrieves an incident by correlation key
func (r *IncidentRepository) GetByCorrelationKey(ctx context.Context, key string) (*correlation.Incident, error) {
	query := `
		SELECT id, tenant_id, category, severity, status, is_critical, title,
			correlation_key, correlation_reason, root_cause, summary, postmortem,
			ai_analysis, affected_resources, primary_resource, cluster_name,
			node_name, namespace, context, timeline, metadata, first_event_at,
			last_event_at, opened_at, closed_at, analyzed_at, notified_at,
			last_health_check_at, notification_channels, notification_count,
			last_notification_at
		FROM incidents WHERE correlation_key = $1 AND status = 'open'
		ORDER BY opened_at DESC LIMIT 1`

	return r.scanIncident(r.db.QueryRowContext(ctx, query, key))
}

// ListOpen retrieves all open incidents
func (r *IncidentRepository) ListOpen(ctx context.Context) ([]*correlation.Incident, error) {
	query := `
		SELECT id, tenant_id, category, severity, status, is_critical, title,
			correlation_key, correlation_reason, root_cause, summary, postmortem,
			ai_analysis, affected_resources, primary_resource, cluster_name,
			node_name, namespace, context, timeline, metadata, first_event_at,
			last_event_at, opened_at, closed_at, analyzed_at, notified_at,
			last_health_check_at, notification_channels, notification_count,
			last_notification_at
		FROM incidents WHERE status = 'open'
		ORDER BY opened_at DESC`

	return r.scanIncidents(ctx, query)
}

// ListCriticalOpen retrieves all open critical incidents
func (r *IncidentRepository) ListCriticalOpen(ctx context.Context) ([]*correlation.Incident, error) {
	query := `
		SELECT id, tenant_id, category, severity, status, is_critical, title,
			correlation_key, correlation_reason, root_cause, summary, postmortem,
			ai_analysis, affected_resources, primary_resource, cluster_name,
			node_name, namespace, context, timeline, metadata, first_event_at,
			last_event_at, opened_at, closed_at, analyzed_at, notified_at,
			last_health_check_at, notification_channels, notification_count,
			last_notification_at
		FROM incidents WHERE status = 'open' AND is_critical = TRUE
		ORDER BY opened_at DESC`

	return r.scanIncidents(ctx, query)
}

// List retrieves incidents with filters
func (r *IncidentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*correlation.Incident, error) {
	query := `
		SELECT id, tenant_id, category, severity, status, is_critical, title,
			correlation_key, correlation_reason, root_cause, summary, postmortem,
			ai_analysis, affected_resources, primary_resource, cluster_name,
			node_name, namespace, context, timeline, metadata, first_event_at,
			last_event_at, opened_at, closed_at, analyzed_at, notified_at,
			last_health_check_at, notification_channels, notification_count,
			last_notification_at
		FROM incidents WHERE 1=1`

	args := []interface{}{}
	argNum := 1

	if tenantID, ok := filters["tenant_id"].(uuid.UUID); ok {
		query += fmt.Sprintf(" AND tenant_id = $%d", argNum)
		args = append(args, tenantID)
		argNum++
	}

	if state, ok := filters["status"].(correlation.IncidentState); ok {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, state)
		argNum++
	}

	if category, ok := filters["category"].(correlation.IncidentCategory); ok {
		query += fmt.Sprintf(" AND category = $%d", argNum)
		args = append(args, category)
		argNum++
	}

	if severity, ok := filters["severity"].(models.EventSeverity); ok {
		query += fmt.Sprintf(" AND severity = $%d", argNum)
		args = append(args, severity)
		argNum++
	}

	if isCritical, ok := filters["is_critical"].(bool); ok {
		query += fmt.Sprintf(" AND is_critical = $%d", argNum)
		args = append(args, isCritical)
		argNum++
	}

	if namespace, ok := filters["namespace"].(string); ok {
		query += fmt.Sprintf(" AND namespace = $%d", argNum)
		args = append(args, namespace)
		_ = argNum // Last use of argNum, increment removed to avoid ineffassign
	}

	// Default ordering
	query += " ORDER BY opened_at DESC"

	// Limit
	if limit, ok := filters["limit"].(int); ok {
		query += fmt.Sprintf(" LIMIT %d", limit)
	} else {
		query += " LIMIT 100"
	}

	// Offset
	if offset, ok := filters["offset"].(int); ok {
		query += fmt.Sprintf(" OFFSET %d", offset)
	}

	return r.scanIncidentsWithArgs(ctx, query, args...)
}

// AddEvent links an event to an incident
func (r *IncidentRepository) AddEvent(ctx context.Context, incidentID, eventID uuid.UUID) error {
	// Insert into junction table
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO incident_events (incident_id, event_id)
		VALUES ($1, $2)
		ON CONFLICT (incident_id, event_id) DO NOTHING`,
		incidentID, eventID)
	if err != nil {
		return fmt.Errorf("failed to link event to incident: %w", err)
	}

	// Update event with incident_id
	_, err = r.db.ExecContext(ctx, `
		UPDATE events SET incident_id = $1, updated_at = NOW()
		WHERE id = $2`,
		incidentID, eventID)
	if err != nil {
		return fmt.Errorf("failed to update event incident_id: %w", err)
	}

	return nil
}

// GetIncidentEvents retrieves all events for an incident
func (r *IncidentRepository) GetIncidentEvents(ctx context.Context, incidentID uuid.UUID) ([]*models.Event, error) {
	query := `
		SELECT e.id, e.tenant_id, e.agent_id, e.event_type, e.severity, e.resource_kind,
			e.resource_name, e.resource_uid, e.namespace, e.message, e.metadata,
			e.event_timestamp, e.ai_analysis, e.ai_analyzed_at, e.notification_sent,
			e.notification_sent_at, e.notification_channels, e.created_at, e.updated_at
		FROM events e
		JOIN incident_events ie ON e.id = ie.event_id
		WHERE ie.incident_id = $1
		ORDER BY e.event_timestamp ASC`

	rows, err := r.db.QueryContext(ctx, query, incidentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query incident events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []*models.Event
	for rows.Next() {
		event := &models.Event{}
		var metadataJSON []byte
		var channels pq.StringArray

		err := rows.Scan(
			&event.ID, &event.TenantID, &event.AgentID, &event.EventType, &event.Severity,
			&event.ResourceKind, &event.ResourceName, &event.ResourceUID, &event.Namespace,
			&event.Message, &metadataJSON, &event.EventTimestamp, &event.AIAnalysis,
			&event.AIAnalyzedAt, &event.NotificationSent, &event.NotificationSentAt,
			&channels, &event.CreatedAt, &event.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if metadataJSON != nil {
			if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
				// Log but don't fail - metadata is optional
				_ = err
			}
		}
		event.NotificationChannels = channels

		events = append(events, event)
	}

	return events, rows.Err()
}

// AddAnalysisLog adds an incremental analysis entry
func (r *IncidentRepository) AddAnalysisLog(ctx context.Context, entry *correlation.AnalysisLogEntry) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO incident_analysis_log (id, incident_id, analysis_type, content, notified)
		VALUES ($1, $2, $3, $4, $5)`,
		entry.ID, entry.IncidentID, entry.AnalysisType, entry.Content, entry.Notified)
	if err != nil {
		return fmt.Errorf("failed to add analysis log: %w", err)
	}
	return nil
}

// GetAnalysisLog retrieves analysis log entries for an incident
func (r *IncidentRepository) GetAnalysisLog(ctx context.Context, incidentID uuid.UUID) ([]correlation.AnalysisLogEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, incident_id, analysis_type, content, notified, created_at
		FROM incident_analysis_log
		WHERE incident_id = $1
		ORDER BY created_at ASC`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query analysis log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []correlation.AnalysisLogEntry
	for rows.Next() {
		var entry correlation.AnalysisLogEntry
		if err := rows.Scan(&entry.ID, &entry.IncidentID, &entry.AnalysisType,
			&entry.Content, &entry.Notified, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan analysis log entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// MarkAnalysisLogNotified marks an analysis log entry as notified
func (r *IncidentRepository) MarkAnalysisLogNotified(ctx context.Context, entryID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE incident_analysis_log SET notified = TRUE
		WHERE id = $1`, entryID)
	return err
}

// UpdateStatus updates the incident status
func (r *IncidentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status correlation.IncidentState) error {
	query := `UPDATE incidents SET status = $2, updated_at = NOW() WHERE id = $1`
	if status == correlation.IncidentStateResolved {
		query = `UPDATE incidents SET status = $2, closed_at = NOW(), updated_at = NOW() WHERE id = $1`
	}

	result, err := r.db.ExecContext(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update incident status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("incident not found: %s", id)
	}

	return nil
}

// UpdatePostmortem updates the incident postmortem
func (r *IncidentRepository) UpdatePostmortem(ctx context.Context, id uuid.UUID, postmortem string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE incidents SET postmortem = $2, updated_at = NOW()
		WHERE id = $1`, id, postmortem)
	return err
}

// UpdateLastHealthCheck updates the last health check timestamp
func (r *IncidentRepository) UpdateLastHealthCheck(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE incidents SET last_health_check_at = NOW(), updated_at = NOW()
		WHERE id = $1`, id)
	return err
}

// IncrementNotificationCount increments the notification count
func (r *IncidentRepository) IncrementNotificationCount(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE incidents 
		SET notification_count = notification_count + 1,
			last_notification_at = NOW(),
			updated_at = NOW()
		WHERE id = $1`, id)
	return err
}

// Delete removes an incident
func (r *IncidentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM incidents WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete incident: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("incident not found: %s", id)
	}

	return nil
}

// scanIncident scans a single incident from a row
func (r *IncidentRepository) scanIncident(row *sql.Row) (*correlation.Incident, error) {
	incident := &correlation.Incident{}

	var (
		aiAnalysisJSON, affectedResourcesJSON, primaryResourceJSON []byte
		contextJSON, timelineJSON, metadataJSON                    []byte
		channels                                                   pq.StringArray
	)

	err := row.Scan(
		&incident.ID, &incident.TenantID, &incident.Category, &incident.Severity,
		&incident.State, &incident.IsCritical, &incident.Title,
		&incident.CorrelationKey, &incident.CorrelationReason,
		&incident.RootCause, &incident.Summary, &incident.Postmortem,
		&aiAnalysisJSON, &affectedResourcesJSON, &primaryResourceJSON,
		&incident.ClusterName, &incident.NodeName, &incident.Namespace,
		&contextJSON, &timelineJSON, &metadataJSON,
		&incident.FirstEventAt, &incident.LastEventAt, &incident.OpenedAt,
		&incident.ClosedAt, &incident.AnalyzedAt, &incident.NotifiedAt,
		&incident.LastHealthCheckAt, &channels, &incident.NotificationCount,
		&incident.LastNotificationAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("incident not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan incident: %w", err)
	}

	// Unmarshal JSON fields (errors are logged but not returned - these are optional fields)
	if aiAnalysisJSON != nil {
		_ = json.Unmarshal(aiAnalysisJSON, &incident.AIAnalysis)
	}
	if affectedResourcesJSON != nil {
		_ = json.Unmarshal(affectedResourcesJSON, &incident.AffectedResources)
	}
	if primaryResourceJSON != nil {
		_ = json.Unmarshal(primaryResourceJSON, &incident.PrimaryResource)
	}
	if contextJSON != nil {
		_ = json.Unmarshal(contextJSON, &incident.Context)
	}
	if timelineJSON != nil {
		_ = json.Unmarshal(timelineJSON, &incident.Timeline)
	}
	if metadataJSON != nil {
		_ = json.Unmarshal(metadataJSON, &incident.Metadata)
	}

	incident.NotificationChannels = channels

	return incident, nil
}

// scanIncidents scans multiple incidents from a query
func (r *IncidentRepository) scanIncidents(ctx context.Context, query string) ([]*correlation.Incident, error) {
	return r.scanIncidentsWithArgs(ctx, query)
}

// scanIncidentsWithArgs scans multiple incidents with arguments
func (r *IncidentRepository) scanIncidentsWithArgs(ctx context.Context, query string, args ...interface{}) ([]*correlation.Incident, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query incidents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var incidents []*correlation.Incident
	for rows.Next() {
		incident := &correlation.Incident{}
		var (
			aiAnalysisJSON, affectedResourcesJSON, primaryResourceJSON []byte
			contextJSON, timelineJSON, metadataJSON                    []byte
			channels                                                   pq.StringArray
		)

		err := rows.Scan(
			&incident.ID, &incident.TenantID, &incident.Category, &incident.Severity,
			&incident.State, &incident.IsCritical, &incident.Title,
			&incident.CorrelationKey, &incident.CorrelationReason,
			&incident.RootCause, &incident.Summary, &incident.Postmortem,
			&aiAnalysisJSON, &affectedResourcesJSON, &primaryResourceJSON,
			&incident.ClusterName, &incident.NodeName, &incident.Namespace,
			&contextJSON, &timelineJSON, &metadataJSON,
			&incident.FirstEventAt, &incident.LastEventAt, &incident.OpenedAt,
			&incident.ClosedAt, &incident.AnalyzedAt, &incident.NotifiedAt,
			&incident.LastHealthCheckAt, &channels, &incident.NotificationCount,
			&incident.LastNotificationAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan incident: %w", err)
		}

		// Unmarshal JSON fields (errors are logged but not returned - these are optional fields)
		if aiAnalysisJSON != nil {
			_ = json.Unmarshal(aiAnalysisJSON, &incident.AIAnalysis)
		}
		if affectedResourcesJSON != nil {
			_ = json.Unmarshal(affectedResourcesJSON, &incident.AffectedResources)
		}
		if primaryResourceJSON != nil {
			_ = json.Unmarshal(primaryResourceJSON, &incident.PrimaryResource)
		}
		if contextJSON != nil {
			_ = json.Unmarshal(contextJSON, &incident.Context)
		}
		if timelineJSON != nil {
			_ = json.Unmarshal(timelineJSON, &incident.Timeline)
		}
		if metadataJSON != nil {
			_ = json.Unmarshal(metadataJSON, &incident.Metadata)
		}

		incident.NotificationChannels = channels
		incidents = append(incidents, incident)
	}

	return incidents, rows.Err()
}

// Count returns the total number of incidents
func (r *IncidentRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM incidents`).Scan(&count)
	return count, err
}

// CountByStatus returns the number of incidents in a given status
func (r *IncidentRepository) CountByStatus(ctx context.Context, status correlation.IncidentState) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM incidents WHERE status = $1`, status).Scan(&count)
	return count, err
}

// GetIncidentsNeedingHealthCheck retrieves open incidents that need health checks
func (r *IncidentRepository) GetIncidentsNeedingHealthCheck(ctx context.Context, olderThan time.Duration) ([]*correlation.Incident, error) {
	threshold := time.Now().Add(-olderThan)
	query := `
		SELECT id, tenant_id, category, severity, status, is_critical, title,
			correlation_key, correlation_reason, root_cause, summary, postmortem,
			ai_analysis, affected_resources, primary_resource, cluster_name,
			node_name, namespace, context, timeline, metadata, first_event_at,
			last_event_at, opened_at, closed_at, analyzed_at, notified_at,
			last_health_check_at, notification_channels, notification_count,
			last_notification_at
		FROM incidents 
		WHERE status = 'open' 
			AND (last_health_check_at IS NULL OR last_health_check_at < $1)
		ORDER BY is_critical DESC, opened_at ASC`

	return r.scanIncidentsWithArgs(ctx, query, threshold)
}

// GetIncidentsNeedingProgressNotification retrieves critical open incidents that need progress updates
func (r *IncidentRepository) GetIncidentsNeedingProgressNotification(ctx context.Context, interval time.Duration) ([]*correlation.Incident, error) {
	threshold := time.Now().Add(-interval)
	query := `
		SELECT id, tenant_id, category, severity, status, is_critical, title,
			correlation_key, correlation_reason, root_cause, summary, postmortem,
			ai_analysis, affected_resources, primary_resource, cluster_name,
			node_name, namespace, context, timeline, metadata, first_event_at,
			last_event_at, opened_at, closed_at, analyzed_at, notified_at,
			last_health_check_at, notification_channels, notification_count,
			last_notification_at
		FROM incidents 
		WHERE status = 'open' 
			AND is_critical = TRUE
			AND (last_notification_at IS NULL OR last_notification_at < $1)
		ORDER BY opened_at ASC`

	return r.scanIncidentsWithArgs(ctx, query, threshold)
}
