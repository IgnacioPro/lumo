package correlation

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/database/models"
)

// InMemoryIncidentRepository implements IncidentRepository with in-memory storage
// This is suitable for single-instance deployments or as a cache layer
// For production multi-instance deployments, use PostgreSQL-backed repository
type InMemoryIncidentRepository struct {
	incidents   map[uuid.UUID]*Incident
	byKey       map[string]uuid.UUID // correlation key -> incident ID
	analysisLog map[uuid.UUID][]AnalysisLogEntry
	logger      *logrus.Entry
}

// NewInMemoryIncidentRepository creates a new in-memory incident repository
func NewInMemoryIncidentRepository(logger *logrus.Logger) *InMemoryIncidentRepository {
	return &InMemoryIncidentRepository{
		incidents:   make(map[uuid.UUID]*Incident),
		byKey:       make(map[string]uuid.UUID),
		analysisLog: make(map[uuid.UUID][]AnalysisLogEntry),
		logger:      logger.WithField("component", "incident-repo-inmem"),
	}
}

// Create stores a new incident
func (r *InMemoryIncidentRepository) Create(ctx context.Context, incident *Incident) error {
	r.incidents[incident.ID] = incident
	r.byKey[incident.CorrelationKey] = incident.ID
	r.logger.WithField("incident_id", incident.ID).Debug("Incident created")
	return nil
}

// Update updates an existing incident
func (r *InMemoryIncidentRepository) Update(ctx context.Context, incident *Incident) error {
	if _, exists := r.incidents[incident.ID]; !exists {
		return fmt.Errorf("incident not found: %s", incident.ID)
	}
	r.incidents[incident.ID] = incident
	r.logger.WithField("incident_id", incident.ID).Debug("Incident updated")
	return nil
}

// GetByID retrieves an incident by ID
func (r *InMemoryIncidentRepository) GetByID(ctx context.Context, id uuid.UUID) (*Incident, error) {
	if incident, exists := r.incidents[id]; exists {
		return incident, nil
	}
	return nil, fmt.Errorf("incident not found: %s", id)
}

// GetByCorrelationKey retrieves an incident by correlation key
func (r *InMemoryIncidentRepository) GetByCorrelationKey(ctx context.Context, key string) (*Incident, error) {
	if id, exists := r.byKey[key]; exists {
		return r.GetByID(ctx, id)
	}
	return nil, fmt.Errorf("incident not found for key: %s", key)
}

// ListOpen retrieves all open incidents
func (r *InMemoryIncidentRepository) ListOpen(ctx context.Context) ([]*Incident, error) {
	open := make([]*Incident, 0)
	for _, incident := range r.incidents {
		if incident.State == IncidentStateOpen {
			open = append(open, incident)
		}
	}
	return open, nil
}

// List retrieves incidents with filters
func (r *InMemoryIncidentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Incident, error) {
	results := make([]*Incident, 0)
	for _, incident := range r.incidents {
		// Apply filters
		if state, ok := filters["state"].(IncidentState); ok {
			if incident.State != state {
				continue
			}
		}
		if category, ok := filters["category"].(IncidentCategory); ok {
			if incident.Category != category {
				continue
			}
		}
		if severity, ok := filters["severity"].(models.EventSeverity); ok {
			if incident.Severity != severity {
				continue
			}
		}
		results = append(results, incident)
	}
	return results, nil
}

// AddEvent links an event to an incident (no-op for in-memory, events are stored in incident)
func (r *InMemoryIncidentRepository) AddEvent(ctx context.Context, incidentID, eventID uuid.UUID) error {
	// Events are already added directly to the incident in memory
	return nil
}

// AddAnalysisLog adds an incremental analysis entry
func (r *InMemoryIncidentRepository) AddAnalysisLog(ctx context.Context, entry *AnalysisLogEntry) error {
	r.analysisLog[entry.IncidentID] = append(r.analysisLog[entry.IncidentID], *entry)
	return nil
}

// MarkAnalysisLogNotified marks an analysis log entry as notified
func (r *InMemoryIncidentRepository) MarkAnalysisLogNotified(ctx context.Context, entryID uuid.UUID) error {
	for incidentID, entries := range r.analysisLog {
		for i, entry := range entries {
			if entry.ID == entryID {
				r.analysisLog[incidentID][i].Notified = true
				return nil
			}
		}
	}
	return fmt.Errorf("analysis log entry not found: %s", entryID)
}

// UpdateLastHealthCheck updates the last health check timestamp (no-op for in-memory)
func (r *InMemoryIncidentRepository) UpdateLastHealthCheck(ctx context.Context, id uuid.UUID) error {
	// Health check timestamp is updated directly on the incident object
	return nil
}

// IncrementNotificationCount increments the notification count (no-op for in-memory)
func (r *InMemoryIncidentRepository) IncrementNotificationCount(ctx context.Context, id uuid.UUID) error {
	// Notification count is updated directly on the incident object
	return nil
}

// Delete removes an incident
func (r *InMemoryIncidentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if incident, exists := r.incidents[id]; exists {
		delete(r.byKey, incident.CorrelationKey)
		delete(r.incidents, id)
		delete(r.analysisLog, id)
		return nil
	}
	return fmt.Errorf("incident not found: %s", id)
}

// Count returns the total number of incidents
func (r *InMemoryIncidentRepository) Count(ctx context.Context) int {
	return len(r.incidents)
}

// CountByState returns the number of incidents in a given state
func (r *InMemoryIncidentRepository) CountByState(ctx context.Context, state IncidentState) int {
	count := 0
	for _, incident := range r.incidents {
		if incident.State == state {
			count++
		}
	}
	return count
}
