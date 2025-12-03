package correlation

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/database/models"
)

// Prometheus metrics for real-time incident manager
var (
	criticalIncidentsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "lumo_critical_incidents_total",
			Help: "Total number of critical incidents created (immediate notification)",
		},
	)

	incidentAnalysisUpdates = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_incident_analysis_updates_total",
			Help: "Total number of incremental analysis updates",
		},
		[]string{"type"}, // initial, update, insight, root_cause, postmortem
	)

	healthCheckDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "lumo_incident_health_check_duration_seconds",
			Help:    "Duration of incident health checks",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
		},
	)
)

// RealtimeIncidentManager handles the new real-time incident workflow:
// - Critical incidents: Immediate notification, incremental analysis, auto-close on health
// - Non-critical incidents: Debounced, postmortem on close
type RealtimeIncidentManager struct {
	config *EngineConfig
	logger *logrus.Entry

	// Repository for persistence
	repo IncidentRepository

	// Context gatherer for enriching incidents
	contextGatherer ContextGatherer

	// AI analyzer for generating analysis
	aiAnalyzer AIAnalyzer

	// Notification sender
	notifier RealtimeIncidentNotifier

	// Health checker for auto-close
	healthChecker HealthChecker

	// Correlation rules
	rules []CorrelationRule

	// In-memory cache for open incidents (keyed by correlation key)
	openIncidents   map[string]*Incident
	openIncidentsMu sync.RWMutex

	// Timers for non-critical incidents (debounce)
	debounceTimers   map[string]*time.Timer
	debounceTimersMu sync.Mutex

	// Suppression cache
	suppressionCache   map[string]time.Time
	suppressionCacheMu sync.RWMutex

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// RealtimeIncidentNotifier extends IncidentNotifier with progress notifications
type RealtimeIncidentNotifier interface {
	IncidentNotifier
	// NotifyIncidentCreated sends "Lumo is on it" notification for critical incidents
	NotifyIncidentCreated(ctx context.Context, incident *Incident) error
	// NotifyAnalysisUpdate sends incremental analysis update
	NotifyAnalysisUpdate(ctx context.Context, incident *Incident, entry *AnalysisLogEntry) error
	// NotifyIncidentResolved sends final notification with postmortem
	NotifyIncidentResolved(ctx context.Context, incident *Incident) error
	// NotifyProgress sends "still working on it" notification
	NotifyProgress(ctx context.Context, incident *Incident) error
}

// HealthChecker checks if incident resources are healthy
type HealthChecker interface {
	// CheckResourcesHealthy returns true if all affected resources are healthy
	CheckResourcesHealthy(ctx context.Context, incident *Incident) (bool, error)
}

// NewRealtimeIncidentManager creates a new real-time incident manager
func NewRealtimeIncidentManager(
	config *EngineConfig,
	repo IncidentRepository,
	contextGatherer ContextGatherer,
	aiAnalyzer AIAnalyzer,
	notifier RealtimeIncidentNotifier,
	healthChecker HealthChecker,
	logger *logrus.Logger,
) *RealtimeIncidentManager {
	if config == nil {
		config = DefaultEngineConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &RealtimeIncidentManager{
		config:           config,
		logger:           logger.WithField("component", "realtime-incident-manager"),
		repo:             repo,
		contextGatherer:  contextGatherer,
		aiAnalyzer:       aiAnalyzer,
		notifier:         notifier,
		healthChecker:    healthChecker,
		rules:            defaultCorrelationRules(),
		openIncidents:    make(map[string]*Incident),
		debounceTimers:   make(map[string]*time.Timer),
		suppressionCache: make(map[string]time.Time),
		ctx:              ctx,
		cancel:           cancel,
	}

	return m
}

// Start begins background processes
func (m *RealtimeIncidentManager) Start() error {
	m.logger.Info("Starting real-time incident manager")

	// Start health check loop
	m.wg.Add(1)
	go m.healthCheckLoop()

	// Start progress notification loop
	m.wg.Add(1)
	go m.progressNotificationLoop()

	// Start suppression cache cleanup
	m.wg.Add(1)
	go m.cleanupSuppressionCache()

	// Load open incidents from database
	if err := m.loadOpenIncidents(); err != nil {
		m.logger.WithError(err).Warn("Failed to load open incidents from database")
	}

	return nil
}

// Stop gracefully shuts down
func (m *RealtimeIncidentManager) Stop() {
	m.logger.Info("Stopping real-time incident manager")
	m.cancel()

	// Cancel all debounce timers
	m.debounceTimersMu.Lock()
	for key, timer := range m.debounceTimers {
		timer.Stop()
		delete(m.debounceTimers, key)
	}
	m.debounceTimersMu.Unlock()

	m.wg.Wait()
	m.logger.Info("Real-time incident manager stopped")
}

// ProcessEvent processes an incoming event
func (m *RealtimeIncidentManager) ProcessEvent(ctx context.Context, event *models.Event) error {
	m.logger.WithFields(logrus.Fields{
		"event_id":   event.ID,
		"event_type": event.EventType,
		"resource":   event.ResourceKind + "/" + event.ResourceName,
		"severity":   event.Severity,
	}).Debug("Processing event")

	// Find matching correlation rule
	rule := m.findMatchingRule(event)
	if rule == nil {
		rule = &defaultRule
	}

	correlationKey := rule.CorrelationKey(event)

	// Check suppression
	if m.isSuppressed(correlationKey) {
		m.logger.WithField("correlation_key", correlationKey).Debug("Incident suppressed (duplicate)")
		return nil
	}

	// Check if this is a critical event
	isCritical := m.isEventCritical(event)

	// Find or create incident
	incident, isNew, err := m.getOrCreateIncident(ctx, correlationKey, rule, event, isCritical)
	if err != nil {
		return fmt.Errorf("failed to get or create incident: %w", err)
	}

	// Add event to incident
	incident.AddEvent(event)
	eventsCorrelatedTotal.Inc()

	// Link event to incident in database
	if m.repo != nil {
		if err := m.linkEventToIncident(ctx, incident.ID, event.ID); err != nil {
			m.logger.WithError(err).Warn("Failed to link event to incident")
		}

		// Update incident in database
		if err := m.repo.Update(ctx, incident); err != nil {
			m.logger.WithError(err).Error("Failed to update incident")
		}
	}

	if isNew {
		incidentsCreatedTotal.WithLabelValues(string(incident.Category), string(incident.Severity)).Inc()
		openIncidentsGauge.Inc()

		if isCritical {
			criticalIncidentsTotal.Inc()
			// Critical path: Immediate notification and start analysis
			go m.handleCriticalIncident(incident)
		} else {
			// Non-critical path: Start debounce timer
			m.startDebounceTimer(correlationKey, incident)
		}
	} else if !isCritical {
		// Reset debounce timer for non-critical incidents
		m.resetDebounceTimer(correlationKey, incident)
	}

	// For critical incidents, analyze each new event incrementally
	if incident.IsCritical && !isNew {
		go m.analyzeIncrementalEvent(ctx, incident, event)
	}

	return nil
}

// isEventCritical checks if an event affects critical resources
func (m *RealtimeIncidentManager) isEventCritical(event *models.Event) bool {
	// Check metadata labels (stored as label_<key> by the agent)
	if event.Metadata != nil {
		for labelKey, labelValue := range m.config.CriticalLabels {
			metadataKey := "label_" + labelKey
			if val, exists := event.Metadata[metadataKey]; exists {
				if strVal, ok := val.(string); ok && strVal == labelValue {
					m.logger.WithFields(logrus.Fields{
						"label_key":   labelKey,
						"label_value": labelValue,
						"event_id":    event.ID,
					}).Info("Event marked as critical based on label")
					return true
				}
			}
		}

		// Also check nested labels format (for backwards compatibility)
		if labels, ok := event.Metadata["labels"].(map[string]interface{}); ok {
			for labelKey, labelValue := range m.config.CriticalLabels {
				if val, exists := labels[labelKey]; exists {
					if strVal, ok := val.(string); ok && strVal == labelValue {
						return true
					}
				}
			}
		}

		// Check owner labels (for pods owned by critical deployments)
		if ownerLabels, ok := event.Metadata["owner_labels"].(map[string]interface{}); ok {
			for labelKey, labelValue := range m.config.CriticalLabels {
				if val, exists := ownerLabels[labelKey]; exists {
					if strVal, ok := val.(string); ok && strVal == labelValue {
						return true
					}
				}
			}
		}
	}

	return false
}

// getOrCreateIncident finds an existing incident or creates a new one
func (m *RealtimeIncidentManager) getOrCreateIncident(
	ctx context.Context,
	correlationKey string,
	rule *CorrelationRule,
	event *models.Event,
	isCritical bool,
) (*Incident, bool, error) {
	m.openIncidentsMu.Lock()
	defer m.openIncidentsMu.Unlock()

	// Check in-memory cache first
	if incident, exists := m.openIncidents[correlationKey]; exists {
		if incident.IsOpen() && incident.EventCount() < m.config.MaxEventsPerIncident {
			return incident, false, nil
		}
	}

	// Check database
	if m.repo != nil {
		existing, err := m.repo.GetByCorrelationKey(ctx, correlationKey)
		if err == nil && existing != nil && existing.IsOpen() {
			m.openIncidents[correlationKey] = existing
			return existing, false, nil
		}
	}

	// Create new incident
	ns := ""
	if event.Namespace != nil {
		ns = *event.Namespace
	}
	nsPtr := &ns

	incident := &Incident{
		ID:                uuid.New(),
		TenantID:          event.TenantID,
		Category:          rule.Category,
		Severity:          event.Severity,
		State:             IncidentStateOpen,
		IsCritical:        isCritical,
		Timeline:          make([]TimelineEntry, 0),
		Events:            make([]*models.Event, 0),
		AffectedResources: make([]AffectedResource, 0),
		AnalysisLog:       make([]AnalysisLogEntry, 0),
		Namespace:         nsPtr,
		FirstEventAt:      event.EventTimestamp,
		LastEventAt:       event.EventTimestamp,
		OpenedAt:          time.Now(),
		CorrelationKey:    correlationKey,
		CorrelationReason: rule.Description,
		Metadata:          make(map[string]interface{}),
	}

	// Generate title
	incident.Title = m.generateTitle(incident, event)

	// Store in database
	if m.repo != nil {
		if err := m.repo.Create(ctx, incident); err != nil {
			return nil, false, fmt.Errorf("failed to create incident: %w", err)
		}
	}

	// Cache in memory
	m.openIncidents[correlationKey] = incident

	m.logger.WithFields(logrus.Fields{
		"incident_id":     incident.ID,
		"correlation_key": correlationKey,
		"category":        incident.Category,
		"is_critical":     isCritical,
	}).Info("New incident created")

	return incident, true, nil
}

// handleCriticalIncident handles the critical incident workflow
func (m *RealtimeIncidentManager) handleCriticalIncident(incident *Incident) {
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	m.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"title":       incident.Title,
	}).Info("Handling critical incident - immediate notification")

	// Send immediate "Lumo is on it" notification
	if m.notifier != nil {
		if err := m.notifier.NotifyIncidentCreated(ctx, incident); err != nil {
			m.logger.WithError(err).Error("Failed to send incident created notification")
		} else {
			// Log the initial notification
			entry := &AnalysisLogEntry{
				ID:           uuid.New(),
				IncidentID:   incident.ID,
				AnalysisType: AnalysisTypeInitial,
				Content:      "Lumo detected a critical incident and is actively analyzing it.",
				Notified:     true,
				CreatedAt:    time.Now(),
			}
			m.addAnalysisLog(ctx, incident, entry)
			incidentAnalysisUpdates.WithLabelValues(AnalysisTypeInitial).Inc()
		}
	}

	// Start initial analysis in background
	go m.performInitialAnalysis(incident)
}

// performInitialAnalysis performs the first AI analysis for a critical incident
func (m *RealtimeIncidentManager) performInitialAnalysis(incident *Incident) {
	ctx, cancel := context.WithTimeout(m.ctx, m.config.AIAnalysisTimeout)
	defer cancel()

	if m.aiAnalyzer == nil || !m.config.EnableAIAnalysis {
		return
	}

	m.logger.WithField("incident_id", incident.ID).Info("Performing initial AI analysis")

	// Gather context if enabled
	if m.config.EnableContextGathering && m.contextGatherer != nil {
		ctxTimeout, ctxCancel := context.WithTimeout(ctx, m.config.ContextGatherTimeout)
		incidentCtx, err := m.contextGatherer.GatherContext(ctxTimeout, incident)
		ctxCancel()

		if err != nil {
			m.logger.WithError(err).Warn("Failed to gather incident context")
		} else {
			incident.Context = incidentCtx
		}
	}

	// Perform AI analysis
	analysis, err := m.aiAnalyzer.Analyze(ctx, incident)
	if err != nil {
		m.logger.WithError(err).Warn("Initial AI analysis failed")
		return
	}

	incident.mu.Lock()
	incident.AIAnalysis = analysis
	incident.RootCause = analysis.RootCause.Summary
	now := time.Now()
	incident.AnalyzedAt = &now
	incident.mu.Unlock()

	// Update database
	if m.repo != nil {
		if err := m.repo.Update(ctx, incident); err != nil {
			m.logger.WithError(err).Warn("Failed to update incident after initial analysis")
		}
	}

	// If we found a root cause, notify
	if analysis.RootCause.Summary != "" {
		entry := &AnalysisLogEntry{
			ID:           uuid.New(),
			IncidentID:   incident.ID,
			AnalysisType: AnalysisTypeRootCause,
			Content:      fmt.Sprintf("Root cause identified: %s", analysis.RootCause.Summary),
			Notified:     false,
			CreatedAt:    time.Now(),
		}
		m.addAnalysisLog(ctx, incident, entry)
		m.notifyAnalysisUpdate(ctx, incident, entry)
		incidentAnalysisUpdates.WithLabelValues(AnalysisTypeRootCause).Inc()
	}
}

// analyzeIncrementalEvent analyzes a new event in context of the incident
func (m *RealtimeIncidentManager) analyzeIncrementalEvent(ctx context.Context, incident *Incident, event *models.Event) {
	if m.aiAnalyzer == nil || !m.config.EnableAIAnalysis {
		return
	}

	m.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"event_id":    event.ID,
	}).Debug("Analyzing incremental event")

	// Build incremental prompt
	prompt := m.buildIncrementalPrompt(incident, event)

	analysisCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Ask AI for incremental insight
	systemPrompt := `You are a Kubernetes SRE analyzing a new event in an ongoing incident.
Be brief (2-3 sentences). Only respond if this event provides NEW insight not already known.
If this is just more of the same, respond with "NO_NEW_INSIGHT".
Focus on: Does this change our understanding? Is there a new affected resource? Does this help identify root cause?`

	response, _, err := m.aiAnalyzer.(*AIIncidentAnalyzer).provider.Ask(analysisCtx, systemPrompt, prompt)
	if err != nil {
		m.logger.WithError(err).Debug("Incremental analysis failed")
		return
	}

	// Check if we have a meaningful insight
	if strings.Contains(strings.ToUpper(response), "NO_NEW_INSIGHT") {
		return
	}

	// We have an insight - log and notify
	entry := &AnalysisLogEntry{
		ID:           uuid.New(),
		IncidentID:   incident.ID,
		AnalysisType: AnalysisTypeInsight,
		Content:      response,
		Notified:     false,
		CreatedAt:    time.Now(),
	}
	m.addAnalysisLog(ctx, incident, entry)
	m.notifyAnalysisUpdate(ctx, incident, entry)
	incidentAnalysisUpdates.WithLabelValues(AnalysisTypeInsight).Inc()
}

// buildIncrementalPrompt builds a prompt for incremental event analysis
func (m *RealtimeIncidentManager) buildIncrementalPrompt(incident *Incident, event *models.Event) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("# Incident Update: %s\n\n", incident.Title))
	prompt.WriteString(fmt.Sprintf("Current root cause hypothesis: %s\n\n", incident.RootCause))
	prompt.WriteString(fmt.Sprintf("Events so far: %d\n", len(incident.Events)))
	prompt.WriteString(fmt.Sprintf("Affected resources: %d\n\n", len(incident.AffectedResources)))

	prompt.WriteString("## New Event\n")
	prompt.WriteString(fmt.Sprintf("- Type: %s\n", event.EventType))
	prompt.WriteString(fmt.Sprintf("- Severity: %s\n", event.Severity))
	prompt.WriteString(fmt.Sprintf("- Resource: %s/%s\n", event.ResourceKind, event.ResourceName))
	prompt.WriteString(fmt.Sprintf("- Message: %s\n", event.Message))

	prompt.WriteString("\nDoes this event provide new insight into the incident?")

	return prompt.String()
}

// startDebounceTimer starts a debounce timer for non-critical incidents
func (m *RealtimeIncidentManager) startDebounceTimer(correlationKey string, incident *Incident) {
	m.debounceTimersMu.Lock()
	defer m.debounceTimersMu.Unlock()

	m.debounceTimers[correlationKey] = time.AfterFunc(m.config.CorrelationWindow, func() {
		m.closeNonCriticalIncident(correlationKey, incident)
	})
}

// resetDebounceTimer resets the debounce timer
func (m *RealtimeIncidentManager) resetDebounceTimer(correlationKey string, incident *Incident) {
	m.debounceTimersMu.Lock()
	defer m.debounceTimersMu.Unlock()

	if timer, exists := m.debounceTimers[correlationKey]; exists {
		timer.Stop()
	}

	m.debounceTimers[correlationKey] = time.AfterFunc(m.config.CorrelationWindow, func() {
		m.closeNonCriticalIncident(correlationKey, incident)
	})
}

// closeNonCriticalIncident closes a non-critical incident after debounce window
func (m *RealtimeIncidentManager) closeNonCriticalIncident(correlationKey string, incident *Incident) {
	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Minute)
	defer cancel()

	m.logger.WithField("incident_id", incident.ID).Info("Closing non-critical incident after debounce")

	// Clean up
	m.debounceTimersMu.Lock()
	delete(m.debounceTimers, correlationKey)
	m.debounceTimersMu.Unlock()

	m.openIncidentsMu.Lock()
	delete(m.openIncidents, correlationKey)
	m.openIncidentsMu.Unlock()

	// Perform full analysis and generate postmortem
	m.closeIncident(ctx, incident)
}

// closeIncident performs final analysis and closes an incident
func (m *RealtimeIncidentManager) closeIncident(ctx context.Context, incident *Incident) {
	m.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"event_count": incident.EventCount(),
		"is_critical": incident.IsCritical,
	}).Info("Closing incident")

	// Update state
	incident.mu.Lock()
	incident.State = IncidentStateAnalyzing
	incident.mu.Unlock()

	// Gather context if not already done
	if incident.Context == nil && m.config.EnableContextGathering && m.contextGatherer != nil {
		ctxTimeout, ctxCancel := context.WithTimeout(ctx, m.config.ContextGatherTimeout)
		incidentCtx, err := m.contextGatherer.GatherContext(ctxTimeout, incident)
		ctxCancel()

		if err != nil {
			m.logger.WithError(err).Warn("Failed to gather incident context")
		} else {
			incident.Context = incidentCtx
		}
	}

	// Perform final AI analysis if not done
	if incident.AIAnalysis == nil && m.config.EnableAIAnalysis && m.aiAnalyzer != nil {
		aiTimeout, aiCancel := context.WithTimeout(ctx, m.config.AIAnalysisTimeout)
		analysis, err := m.aiAnalyzer.Analyze(aiTimeout, incident)
		aiCancel()

		if err != nil {
			m.logger.WithError(err).Warn("Failed to perform AI analysis")
		} else {
			incident.AIAnalysis = analysis
			incident.RootCause = analysis.RootCause.Summary
			incident.Summary = analysis.FullAnalysis
		}
	}

	// Generate postmortem
	postmortem := m.generatePostmortem(incident)
	incident.Postmortem = postmortem

	// Update state to resolved
	incident.mu.Lock()
	incident.State = IncidentStateResolved
	now := time.Now()
	incident.ClosedAt = &now
	incident.mu.Unlock()

	// Persist
	if m.repo != nil {
		if err := m.repo.Update(ctx, incident); err != nil {
			m.logger.WithError(err).Warn("Failed to update incident after closing")
		}
	}

	// Add postmortem to analysis log
	entry := &AnalysisLogEntry{
		ID:           uuid.New(),
		IncidentID:   incident.ID,
		AnalysisType: AnalysisTypePostmortem,
		Content:      postmortem,
		Notified:     false,
		CreatedAt:    time.Now(),
	}
	m.addAnalysisLog(ctx, incident, entry)
	incidentAnalysisUpdates.WithLabelValues(AnalysisTypePostmortem).Inc()

	// Send final notification
	if m.notifier != nil {
		if err := m.notifier.NotifyIncidentResolved(ctx, incident); err != nil {
			m.logger.WithError(err).Error("Failed to send incident resolved notification")
		}
	}

	// Update suppression cache
	m.addToSuppressionCache(incident.CorrelationKey)

	// Update metrics
	incidentsResolvedTotal.WithLabelValues(string(incident.Category), string(incident.Severity)).Inc()
	openIncidentsGauge.Dec()

	m.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"title":       incident.Title,
		"root_cause":  incident.RootCause,
	}).Info("Incident closed with postmortem")
}

// generatePostmortem creates a comprehensive postmortem for the incident
func (m *RealtimeIncidentManager) generatePostmortem(incident *Incident) string {
	var pm strings.Builder

	// Title and Summary
	pm.WriteString(fmt.Sprintf("# Postmortem: %s\n\n", incident.Title))
	pm.WriteString(fmt.Sprintf("**Incident ID:** %s\n", incident.ID.String()))
	pm.WriteString(fmt.Sprintf("**Category:** %s\n", incident.Category))
	pm.WriteString(fmt.Sprintf("**Severity:** %s\n", incident.Severity))
	if incident.IsCritical {
		pm.WriteString("**Priority:** 🚨 Critical\n")
	}
	pm.WriteString("\n")

	// Timeline Overview
	pm.WriteString("## Timeline\n\n")
	pm.WriteString(fmt.Sprintf("- **First Event:** %s\n", incident.FirstEventAt.Format(time.RFC3339)))
	pm.WriteString(fmt.Sprintf("- **Last Event:** %s\n", incident.LastEventAt.Format(time.RFC3339)))
	pm.WriteString(fmt.Sprintf("- **Duration:** %s\n", formatDurationVerbose(incident.Duration())))
	if incident.ClosedAt != nil {
		pm.WriteString(fmt.Sprintf("- **Resolved At:** %s\n", incident.ClosedAt.Format(time.RFC3339)))
	}
	pm.WriteString("\n")

	// Root Cause
	pm.WriteString("## Root Cause Analysis\n\n")
	if incident.RootCause != "" {
		pm.WriteString(incident.RootCause)
	} else {
		pm.WriteString("_Root cause could not be automatically determined._")
	}
	pm.WriteString("\n\n")

	// Impact Assessment
	pm.WriteString("## Impact\n\n")
	pm.WriteString(fmt.Sprintf("- **Total Events:** %d\n", len(incident.Events)))
	pm.WriteString(fmt.Sprintf("- **Affected Resources:** %d\n", len(incident.AffectedResources)))
	if incident.Namespace != nil && *incident.Namespace != "" {
		pm.WriteString(fmt.Sprintf("- **Namespace:** %s\n", *incident.Namespace))
	}

	if len(incident.AffectedResources) > 0 {
		pm.WriteString("\n### Affected Resources\n\n")
		for _, resource := range incident.AffectedResources {
			pm.WriteString(fmt.Sprintf("- %s/%s", resource.Kind, resource.Name))
			if resource.Namespace != "" {
				pm.WriteString(fmt.Sprintf(" (ns: %s)", resource.Namespace))
			}
			pm.WriteString("\n")
		}
	}
	pm.WriteString("\n")

	// Event Summary
	pm.WriteString("## Event Summary\n\n")
	eventTypes := make(map[string]int)
	for _, event := range incident.Events {
		eventTypes[event.EventType]++
	}
	for eventType, count := range eventTypes {
		pm.WriteString(fmt.Sprintf("- **%s:** %d occurrences\n", eventType, count))
	}
	pm.WriteString("\n")

	// Key Events (First 10)
	if len(incident.Timeline) > 0 {
		pm.WriteString("### Key Events Timeline\n\n")
		pm.WriteString("| Time | Type | Resource | Message |\n")
		pm.WriteString("|------|------|----------|--------|\n")
		limit := 10
		if len(incident.Timeline) < limit {
			limit = len(incident.Timeline)
		}
		for i := 0; i < limit; i++ {
			entry := incident.Timeline[i]
			msg := entry.Message
			if len(msg) > 60 {
				msg = msg[:57] + "..."
			}
			pm.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				entry.Timestamp.Format("15:04:05"),
				entry.EventType,
				entry.Resource,
				msg))
		}
		if len(incident.Timeline) > 10 {
			pm.WriteString(fmt.Sprintf("\n_... and %d more events_\n", len(incident.Timeline)-10))
		}
		pm.WriteString("\n")
	}

	// AI Analysis Summary
	if incident.AIAnalysis != nil {
		pm.WriteString("## AI Analysis Summary\n\n")

		// Immediate Actions
		if len(incident.AIAnalysis.ImmediateActions) > 0 {
			pm.WriteString("### Recommended Immediate Actions\n\n")
			for i, action := range incident.AIAnalysis.ImmediateActions {
				pm.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, action.Title))
				if action.Command != "" {
					pm.WriteString(fmt.Sprintf("   ```\n   %s\n   ```\n", action.Command))
				}
			}
			pm.WriteString("\n")
		}

		// Long-term Actions
		if len(incident.AIAnalysis.LongTermActions) > 0 {
			pm.WriteString("### Prevention Recommendations\n\n")
			for i, action := range incident.AIAnalysis.LongTermActions {
				pm.WriteString(fmt.Sprintf("%d. %s\n", i+1, action.Title))
			}
			pm.WriteString("\n")
		}

		// Monitoring Recommendations
		if len(incident.AIAnalysis.MonitoringRecommendations) > 0 {
			pm.WriteString("### Monitoring Recommendations\n\n")
			for _, rec := range incident.AIAnalysis.MonitoringRecommendations {
				pm.WriteString(fmt.Sprintf("- %s\n", rec))
			}
			pm.WriteString("\n")
		}
	}

	// Notification History
	if incident.NotificationCount > 0 {
		pm.WriteString("## Notification History\n\n")
		pm.WriteString(fmt.Sprintf("- **Total Notifications Sent:** %d\n", incident.NotificationCount))
		if len(incident.NotificationChannels) > 0 {
			pm.WriteString(fmt.Sprintf("- **Channels:** %s\n", strings.Join(incident.NotificationChannels, ", ")))
		}
		pm.WriteString("\n")
	}

	// Footer
	pm.WriteString("---\n")
	pm.WriteString(fmt.Sprintf("_Generated by Lumo at %s_\n", time.Now().Format(time.RFC3339)))

	return pm.String()
}

// formatDurationVerbose returns a human-readable duration string
func formatDurationVerbose(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs > 0 {
			return fmt.Sprintf("%d minutes %d seconds", mins, secs)
		}
		return fmt.Sprintf("%d minutes", mins)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins > 0 {
		return fmt.Sprintf("%d hours %d minutes", hours, mins)
	}
	return fmt.Sprintf("%d hours", hours)
}

// healthCheckLoop periodically checks if incident resources are healthy
func (m *RealtimeIncidentManager) healthCheckLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkOpenIncidentsHealth()
		}
	}
}

// checkOpenIncidentsHealth checks health of all open incidents
func (m *RealtimeIncidentManager) checkOpenIncidentsHealth() {
	if m.healthChecker == nil {
		return
	}

	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	m.openIncidentsMu.RLock()
	incidents := make([]*Incident, 0, len(m.openIncidents))
	for _, incident := range m.openIncidents {
		incidents = append(incidents, incident)
	}
	m.openIncidentsMu.RUnlock()

	for _, incident := range incidents {
		start := time.Now()
		healthy, err := m.healthChecker.CheckResourcesHealthy(ctx, incident)
		healthCheckDuration.Observe(time.Since(start).Seconds())

		if err != nil {
			m.logger.WithError(err).WithField("incident_id", incident.ID).Debug("Health check failed")
			continue
		}

		// Update last health check time
		now := time.Now()
		incident.LastHealthCheckAt = &now
		if m.repo != nil {
			if err := m.repo.UpdateLastHealthCheck(ctx, incident.ID); err != nil {
				m.logger.WithError(err).Debug("Failed to update last health check")
			}
		}

		if healthy {
			m.logger.WithField("incident_id", incident.ID).Info("Incident resources healthy - closing")

			// Remove from in-memory cache
			m.openIncidentsMu.Lock()
			delete(m.openIncidents, incident.CorrelationKey)
			m.openIncidentsMu.Unlock()

			// Cancel debounce timer if exists
			m.debounceTimersMu.Lock()
			if timer, exists := m.debounceTimers[incident.CorrelationKey]; exists {
				timer.Stop()
				delete(m.debounceTimers, incident.CorrelationKey)
			}
			m.debounceTimersMu.Unlock()

			// Close the incident
			go m.closeIncident(ctx, incident)
		}
	}
}

// progressNotificationLoop sends progress updates for critical incidents
func (m *RealtimeIncidentManager) progressNotificationLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.ProgressNotificationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.sendProgressNotifications()
		}
	}
}

// sendProgressNotifications sends "still working on it" for critical incidents
func (m *RealtimeIncidentManager) sendProgressNotifications() {
	if m.notifier == nil {
		return
	}

	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	m.openIncidentsMu.RLock()
	var criticalIncidents []*Incident
	for _, incident := range m.openIncidents {
		if incident.IsCritical {
			// Check if we need to send a progress notification
			if incident.LastNotificationAt == nil ||
				time.Since(*incident.LastNotificationAt) >= m.config.ProgressNotificationInterval {
				criticalIncidents = append(criticalIncidents, incident)
			}
		}
	}
	m.openIncidentsMu.RUnlock()

	for _, incident := range criticalIncidents {
		if err := m.notifier.NotifyProgress(ctx, incident); err != nil {
			m.logger.WithError(err).WithField("incident_id", incident.ID).Warn("Failed to send progress notification")
		} else {
			now := time.Now()
			incident.LastNotificationAt = &now
			incident.NotificationCount++

			// Log the progress update
			entry := &AnalysisLogEntry{
				ID:           uuid.New(),
				IncidentID:   incident.ID,
				AnalysisType: AnalysisTypeUpdate,
				Content:      fmt.Sprintf("Progress update #%d - Lumo is still analyzing the incident", incident.NotificationCount),
				Notified:     true,
				CreatedAt:    time.Now(),
			}
			m.addAnalysisLog(ctx, incident, entry)
			incidentAnalysisUpdates.WithLabelValues(AnalysisTypeUpdate).Inc()

			if m.repo != nil {
				if err := m.repo.IncrementNotificationCount(ctx, incident.ID); err != nil {
					m.logger.WithError(err).Debug("Failed to increment notification count")
				}
			}
		}
	}
}

// Helper functions

func (m *RealtimeIncidentManager) addAnalysisLog(ctx context.Context, incident *Incident, entry *AnalysisLogEntry) {
	incident.mu.Lock()
	incident.AnalysisLog = append(incident.AnalysisLog, *entry)
	incident.mu.Unlock()

	if m.repo != nil {
		if err := m.repo.AddAnalysisLog(ctx, entry); err != nil {
			m.logger.WithError(err).Debug("Failed to add analysis log")
		}
	}
}

func (m *RealtimeIncidentManager) notifyAnalysisUpdate(ctx context.Context, incident *Incident, entry *AnalysisLogEntry) {
	if m.notifier == nil {
		return
	}

	if err := m.notifier.NotifyAnalysisUpdate(ctx, incident, entry); err != nil {
		m.logger.WithError(err).Warn("Failed to send analysis update notification")
	} else {
		entry.Notified = true
		now := time.Now()
		incident.LastNotificationAt = &now
		incident.NotificationCount++

		if m.repo != nil {
			if err := m.repo.MarkAnalysisLogNotified(ctx, entry.ID); err != nil {
				m.logger.WithError(err).Debug("Failed to mark analysis log notified")
			}
			if err := m.repo.IncrementNotificationCount(ctx, incident.ID); err != nil {
				m.logger.WithError(err).Debug("Failed to increment notification count")
			}
		}
	}
}

func (m *RealtimeIncidentManager) linkEventToIncident(ctx context.Context, incidentID, eventID uuid.UUID) error {
	// This is handled by the repository interface if it supports it
	if repo, ok := m.repo.(interface {
		AddEvent(ctx context.Context, incidentID, eventID uuid.UUID) error
	}); ok {
		return repo.AddEvent(ctx, incidentID, eventID)
	}
	return nil
}

func (m *RealtimeIncidentManager) loadOpenIncidents() error {
	if m.repo == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	incidents, err := m.repo.ListOpen(ctx)
	if err != nil {
		return err
	}

	m.openIncidentsMu.Lock()
	defer m.openIncidentsMu.Unlock()

	for _, incident := range incidents {
		m.openIncidents[incident.CorrelationKey] = incident
	}

	m.logger.WithField("count", len(incidents)).Info("Loaded open incidents from database")
	return nil
}

func (m *RealtimeIncidentManager) findMatchingRule(event *models.Event) *CorrelationRule {
	var bestMatch *CorrelationRule
	bestPriority := -1

	for i := range m.rules {
		rule := &m.rules[i]
		if rule.Match(event) && rule.Priority > bestPriority {
			bestMatch = rule
			bestPriority = rule.Priority
		}
	}

	return bestMatch
}

func (m *RealtimeIncidentManager) isSuppressed(correlationKey string) bool {
	m.suppressionCacheMu.RLock()
	defer m.suppressionCacheMu.RUnlock()

	if lastTime, exists := m.suppressionCache[correlationKey]; exists {
		return time.Since(lastTime) < m.config.SuppressDuplicateWindow
	}
	return false
}

func (m *RealtimeIncidentManager) addToSuppressionCache(correlationKey string) {
	m.suppressionCacheMu.Lock()
	defer m.suppressionCacheMu.Unlock()
	m.suppressionCache[correlationKey] = time.Now()
}

func (m *RealtimeIncidentManager) cleanupSuppressionCache() {
	defer m.wg.Done()

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.suppressionCacheMu.Lock()
			for key, lastTime := range m.suppressionCache {
				if time.Since(lastTime) > m.config.SuppressDuplicateWindow {
					delete(m.suppressionCache, key)
				}
			}
			m.suppressionCacheMu.Unlock()
		}
	}
}

func (m *RealtimeIncidentManager) generateTitle(incident *Incident, event *models.Event) string {
	// Use the original title generation logic
	ns := "cluster-wide"
	if incident.Namespace != nil && *incident.Namespace != "" {
		ns = *incident.Namespace
	}

	prefix := ""
	if incident.IsCritical {
		prefix = "🚨 CRITICAL: "
	}

	switch incident.Category {
	case CategoryMemory:
		return fmt.Sprintf("%sMemory Issue in %s", prefix, ns)
	case CategoryCrash:
		return fmt.Sprintf("%sCrash Loop: %s/%s in %s", prefix, event.ResourceKind, event.ResourceName, ns)
	case CategoryImage:
		return fmt.Sprintf("%sImage Pull Failure in %s", prefix, ns)
	case CategoryStorage:
		return fmt.Sprintf("%sStorage Issue in %s", prefix, ns)
	case CategoryNode:
		return fmt.Sprintf("%sNode Issue", prefix)
	case CategoryScheduling:
		return fmt.Sprintf("%sScheduling Problem in %s", prefix, ns)
	case CategoryDeployment:
		return fmt.Sprintf("%sDeployment Failed in %s", prefix, ns)
	default:
		return fmt.Sprintf("%sKubernetes Incident in %s", prefix, ns)
	}
}

// GetOpenIncidents returns all currently open incidents
func (m *RealtimeIncidentManager) GetOpenIncidents() []*Incident {
	m.openIncidentsMu.RLock()
	defer m.openIncidentsMu.RUnlock()

	incidents := make([]*Incident, 0, len(m.openIncidents))
	for _, incident := range m.openIncidents {
		incidents = append(incidents, incident)
	}

	sort.Slice(incidents, func(i, j int) bool {
		// Critical incidents first, then by time
		if incidents[i].IsCritical != incidents[j].IsCritical {
			return incidents[i].IsCritical
		}
		return incidents[i].FirstEventAt.Before(incidents[j].FirstEventAt)
	})

	return incidents
}

// GetOpenIncidentCount returns the number of open incidents
func (m *RealtimeIncidentManager) GetOpenIncidentCount() int {
	m.openIncidentsMu.RLock()
	defer m.openIncidentsMu.RUnlock()
	return len(m.openIncidents)
}
