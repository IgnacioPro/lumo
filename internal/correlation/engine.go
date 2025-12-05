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

// Prometheus metrics for correlation engine
var (
	incidentsCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_incidents_created_total",
			Help: "Total number of incidents created",
		},
		[]string{"category", "severity"},
	)

	incidentsResolvedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_incidents_resolved_total",
			Help: "Total number of incidents resolved",
		},
		[]string{"category", "severity"},
	)

	eventsCorrelatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "lumo_events_correlated_total",
			Help: "Total number of events correlated into incidents",
		},
	)

	incidentDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lumo_incident_duration_seconds",
			Help:    "Duration of incidents from first to last event",
			Buckets: []float64{30, 60, 120, 300, 600, 1800, 3600},
		},
		[]string{"category"},
	)

	openIncidentsGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lumo_open_incidents",
			Help: "Current number of open incidents",
		},
	)

	correlationWindowDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "lumo_correlation_window_duration_seconds",
			Help:    "Time spent in correlation window before closing incident",
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600},
		},
	)
)

// Engine is the main incident correlation engine
type Engine struct {
	config *EngineConfig
	logger *logrus.Entry

	// Active incidents by correlation key
	incidents   map[string]*Incident
	incidentsMu sync.RWMutex

	// Timers for closing incidents after correlation window
	timers   map[string]*time.Timer
	timersMu sync.Mutex

	// Context gatherer for enriching incidents
	contextGatherer ContextGatherer

	// AI analyzer for generating analysis
	aiAnalyzer AIAnalyzer

	// Notification sender
	notifier IncidentNotifier

	// Incident repository for persistence
	repo IncidentRepository

	// Correlation rules
	rules []CorrelationRule

	// Suppression cache (correlation key -> last incident time)
	suppressionCache   map[string]time.Time
	suppressionCacheMu sync.RWMutex

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ContextGatherer gathers contextual information for incidents
type ContextGatherer interface {
	// GatherContext collects logs, metrics, and related events
	GatherContext(ctx context.Context, incident *Incident) (*IncidentContext, error)
}

// AIAnalyzer performs AI analysis on incidents
type AIAnalyzer interface {
	// Analyze generates AI-powered analysis for an incident
	Analyze(ctx context.Context, incident *Incident) (*AIAnalysisResult, error)
}

// IncidentNotifier sends notifications for incidents
type IncidentNotifier interface {
	// NotifyIncident sends notifications about an incident
	NotifyIncident(ctx context.Context, incident *Incident) error
	// NotifyIncidentCreated sends an immediate notification when a critical incident is created
	NotifyIncidentCreated(ctx context.Context, incident *Incident) (string, error)
	// NotifyAnalysisUpdate sends incremental analysis update
	NotifyAnalysisUpdate(ctx context.Context, incident *Incident, entry *AnalysisLogEntry) error
}

// IncidentRepository persists incidents
type IncidentRepository interface {
	// Create stores a new incident
	Create(ctx context.Context, incident *Incident) error
	// Update updates an existing incident
	Update(ctx context.Context, incident *Incident) error
	// GetByCorrelationKey retrieves an incident by correlation key
	GetByCorrelationKey(ctx context.Context, key string) (*Incident, error)
	// ListOpen retrieves all open incidents
	ListOpen(ctx context.Context) ([]*Incident, error)
	// AddEvent links an event to an incident (optional - not all implementations need this)
	AddEvent(ctx context.Context, incidentID, eventID uuid.UUID) error
	// AddAnalysisLog adds an incremental analysis entry
	AddAnalysisLog(ctx context.Context, entry *AnalysisLogEntry) error
	// MarkAnalysisLogNotified marks an analysis log entry as notified
	MarkAnalysisLogNotified(ctx context.Context, entryID uuid.UUID) error
	// UpdateLastHealthCheck updates the last health check timestamp
	UpdateLastHealthCheck(ctx context.Context, id uuid.UUID) error
	// IncrementNotificationCount increments the notification count
	IncrementNotificationCount(ctx context.Context, id uuid.UUID) error
}

// NewEngine creates a new correlation engine
func NewEngine(
	config *EngineConfig,
	contextGatherer ContextGatherer,
	aiAnalyzer AIAnalyzer,
	notifier IncidentNotifier,
	repo IncidentRepository,
	logger *logrus.Logger,
) *Engine {
	if config == nil {
		config = DefaultEngineConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		config:           config,
		logger:           logger.WithField("component", "correlation-engine"),
		incidents:        make(map[string]*Incident),
		timers:           make(map[string]*time.Timer),
		contextGatherer:  contextGatherer,
		aiAnalyzer:       aiAnalyzer,
		notifier:         notifier,
		repo:             repo,
		rules:            defaultCorrelationRules(),
		suppressionCache: make(map[string]time.Time),
		ctx:              ctx,
		cancel:           cancel,
	}

	return e
}

// Start begins the correlation engine background processes
func (e *Engine) Start() error {
	e.logger.Info("Starting correlation engine")

	// Start suppression cache cleanup
	e.wg.Add(1)
	go e.cleanupSuppressionCache()

	return nil
}

// Stop gracefully shuts down the correlation engine
func (e *Engine) Stop() {
	e.logger.Info("Stopping correlation engine")
	e.cancel()

	// Cancel all pending timers
	e.timersMu.Lock()
	for key, timer := range e.timers {
		timer.Stop()
		delete(e.timers, key)
	}
	e.timersMu.Unlock()

	e.wg.Wait()
	e.logger.Info("Correlation engine stopped")
}

// ProcessEvent processes an incoming event and correlates it with existing incidents
func (e *Engine) ProcessEvent(ctx context.Context, event *models.Event) error {
	e.logger.WithFields(logrus.Fields{
		"event_id":   event.ID,
		"event_type": event.EventType,
		"resource":   event.ResourceKind + "/" + event.ResourceName,
		"severity":   event.Severity,
	}).Debug("Processing event for correlation")

	// Find matching correlation rule
	rule := e.findMatchingRule(event)
	if rule == nil {
		e.logger.WithField("event_id", event.ID).Debug("No correlation rule matched, creating standalone incident")
		rule = &defaultRule
	}

	// Generate correlation key
	correlationKey := rule.CorrelationKey(event)

	// Check suppression
	if e.isSuppressed(correlationKey) {
		e.logger.WithField("correlation_key", correlationKey).Debug("Incident suppressed (duplicate)")
		return nil
	}

	// Find or create incident
	incident, isNew := e.getOrCreateIncident(correlationKey, rule, event)

	// Add event to incident
	incident.AddEvent(event)
	eventsCorrelatedTotal.Inc()

	if isNew {
		incidentsCreatedTotal.WithLabelValues(string(incident.Category), string(incident.Severity)).Inc()
		openIncidentsGauge.Inc()

		// Persist new incident
		if e.repo != nil {
			if err := e.repo.Create(ctx, incident); err != nil {
				e.logger.WithError(err).Error("Failed to persist incident")
			}
		}

		e.logger.WithFields(logrus.Fields{
			"incident_id":     incident.ID,
			"correlation_key": correlationKey,
			"category":        incident.Category,
		}).Info("New incident created")

		// Immediate notification for critical incidents
		if incident.IsCritical && e.notifier != nil {
			go func(inc *Incident) {
				// Send "I'm on it" notification immediately
				ts, err := e.notifier.NotifyIncidentCreated(ctx, inc)
				if err != nil {
					e.logger.WithError(err).Error("Failed to send immediate notification")
				} else {
					inc.mu.Lock()
					inc.ThreadTS = ts
					inc.mu.Unlock()
				}

				// Trigger early investigation
				e.triggerEarlyInvestigation(inc)
			}(incident)
		}
	} else {
		// Update existing incident
		if e.repo != nil {
			if err := e.repo.Update(ctx, incident); err != nil {
				e.logger.WithError(err).Error("Failed to update incident")
			}
		}

		e.logger.WithFields(logrus.Fields{
			"incident_id": incident.ID,
			"event_count": incident.EventCount(),
		}).Debug("Event added to existing incident")
	}

	// Reset or start the correlation window timer
	e.resetCorrelationTimer(correlationKey, incident)

	return nil
}

// getOrCreateIncident finds an existing incident or creates a new one
func (e *Engine) getOrCreateIncident(correlationKey string, rule *CorrelationRule, event *models.Event) (*Incident, bool) {
	e.incidentsMu.Lock()
	defer e.incidentsMu.Unlock()

	if incident, exists := e.incidents[correlationKey]; exists {
		// Check if incident is still open and not at max capacity
		if incident.IsOpen() && incident.EventCount() < e.config.MaxEventsPerIncident {
			return incident, false
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
		IsCritical:        event.Severity == "critical",
		State:             IncidentStateOpen,
		Timeline:          make([]TimelineEntry, 0),
		Events:            make([]*models.Event, 0),
		AffectedResources: make([]AffectedResource, 0),
		Namespace:         nsPtr,
		FirstEventAt:      event.EventTimestamp,
		LastEventAt:       event.EventTimestamp,
		OpenedAt:          time.Now(),
		CorrelationKey:    correlationKey,
		CorrelationReason: rule.Description,
		Metadata:          make(map[string]interface{}),
	}

	e.incidents[correlationKey] = incident
	return incident, true
}

// resetCorrelationTimer resets the timer for closing an incident's correlation window
func (e *Engine) resetCorrelationTimer(correlationKey string, incident *Incident) {
	e.timersMu.Lock()
	defer e.timersMu.Unlock()

	// Cancel existing timer
	if timer, exists := e.timers[correlationKey]; exists {
		timer.Stop()
	}

	// Create new timer
	e.timers[correlationKey] = time.AfterFunc(e.config.CorrelationWindow, func() {
		e.closeIncident(correlationKey, incident)
	})
}

// closeIncident closes an incident after the correlation window expires
func (e *Engine) closeIncident(correlationKey string, incident *Incident) {
	e.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"event_count": incident.EventCount(),
		"duration":    incident.Duration(),
	}).Info("Closing incident correlation window")

	// Update state
	incident.mu.Lock()
	incident.State = IncidentStateAnalyzing
	incident.mu.Unlock()

	// Record metrics
	correlationWindowDuration.Observe(time.Since(incident.OpenedAt).Seconds())
	incidentDuration.WithLabelValues(string(incident.Category)).Observe(incident.Duration().Seconds())
	openIncidentsGauge.Dec()

	// Clean up
	e.timersMu.Lock()
	delete(e.timers, correlationKey)
	e.timersMu.Unlock()

	e.incidentsMu.Lock()
	delete(e.incidents, correlationKey)
	e.incidentsMu.Unlock()

	// Process incident asynchronously
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.processIncident(incident)
	}()
}

// processIncident handles the full incident processing pipeline
func (e *Engine) processIncident(incident *Incident) {
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Minute)
	defer cancel()

	e.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"category":    incident.Category,
		"event_count": incident.EventCount(),
	}).Info("Processing incident")

	// Generate incident title
	incident.Title = e.generateTitle(incident)

	// Step 1: Gather context (logs, metrics, related events)
	if e.config.EnableContextGathering && e.contextGatherer != nil {
		ctxTimeout, ctxCancel := context.WithTimeout(ctx, e.config.ContextGatherTimeout)
		incidentCtx, err := e.contextGatherer.GatherContext(ctxTimeout, incident)
		ctxCancel()

		if err != nil {
			e.logger.WithError(err).Warn("Failed to gather incident context")
		} else {
			incident.Context = incidentCtx
		}
	}

	// Step 2: AI Analysis
	if e.config.EnableAIAnalysis && e.aiAnalyzer != nil {
		aiTimeout, aiCancel := context.WithTimeout(ctx, e.config.AIAnalysisTimeout)
		analysis, err := e.aiAnalyzer.Analyze(aiTimeout, incident)
		aiCancel()

		if err != nil {
			e.logger.WithError(err).Warn("Failed to perform AI analysis")
		} else {
			incident.AIAnalysis = analysis
			incident.RootCause = analysis.RootCause.Summary
			incident.Summary = analysis.FullAnalysis
			now := time.Now()
			incident.AnalyzedAt = &now
		}
	}

	// Step 3: Update state and persist
	incident.mu.Lock()
	incident.State = IncidentStateResolved
	now := time.Now()
	incident.ClosedAt = &now
	incident.mu.Unlock()

	if e.repo != nil {
		if err := e.repo.Update(ctx, incident); err != nil {
			e.logger.WithError(err).Error("Failed to update incident after analysis")
		}
	}

	// Step 4: Send notification
	if e.notifier != nil {
		if err := e.notifier.NotifyIncident(ctx, incident); err != nil {
			e.logger.WithError(err).Error("Failed to send incident notification")
		} else {
			notifyTime := time.Now()
			incident.NotifiedAt = &notifyTime
		}
	}

	// Step 5: Update suppression cache
	e.addToSuppressionCache(incident.CorrelationKey)

	incidentsResolvedTotal.WithLabelValues(string(incident.Category), string(incident.Severity)).Inc()

	e.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"title":       incident.Title,
		"root_cause":  incident.RootCause,
	}).Info("Incident processed and notified")
}

// triggerEarlyInvestigation performs an early analysis for critical incidents
func (e *Engine) triggerEarlyInvestigation(incident *Incident) {
	// Wait a short buffer to allow immediate related events to arrive (e.g. logs following an error)
	time.Sleep(5 * time.Second)

	e.logger.WithField("incident_id", incident.ID).Info("Starting early investigation")

	ctx, cancel := context.WithTimeout(e.ctx, 2*time.Minute)
	defer cancel()

	// Step 1: Gather context
	if e.config.EnableContextGathering && e.contextGatherer != nil {
		incidentCtx, err := e.contextGatherer.GatherContext(ctx, incident)
		if err != nil {
			e.logger.WithError(err).Warn("Failed to gather early context")
		} else {
			incident.Context = incidentCtx
		}
	}

	// Step 2: AI Analysis (Initial)
	if e.config.EnableAIAnalysis && e.aiAnalyzer != nil {
		analysis, err := e.aiAnalyzer.Analyze(ctx, incident)
		if err != nil {
			e.logger.WithError(err).Warn("Failed to perform early AI analysis")
		} else {
			incident.AIAnalysis = analysis
			incident.RootCause = analysis.RootCause.Summary

			// Notify with analysis results
			if e.notifier != nil {
				// Create an analysis entry for the update
				entry := &AnalysisLogEntry{
					ID:           uuid.New(),
					IncidentID:   incident.ID,
					AnalysisType: AnalysisTypeInsight,
					Content:      analysis.FullAnalysis,
					CreatedAt:    time.Now(),
				}
				if err := e.notifier.NotifyAnalysisUpdate(ctx, incident, entry); err != nil {
					e.logger.WithError(err).Error("Failed to send early analysis update")
				}
			}
		}
	}
}

// generateTitle creates a human-readable title for the incident
func (e *Engine) generateTitle(incident *Incident) string {
	eventCount := incident.EventCount()
	resourceCount := len(incident.AffectedResources)

	// Get namespace info
	ns := "cluster-wide"
	if incident.Namespace != nil && *incident.Namespace != "" {
		ns = *incident.Namespace
	}

	// Generate category-specific title
	switch incident.Category {
	case CategoryMemory:
		if resourceCount == 1 {
			return fmt.Sprintf("Memory Issue: %s/%s in %s",
				incident.AffectedResources[0].Kind,
				incident.AffectedResources[0].Name,
				ns)
		}
		return fmt.Sprintf("Memory Issues: %d resources affected in %s", resourceCount, ns)

	case CategoryCrash:
		if resourceCount == 1 {
			return fmt.Sprintf("Crash Loop: %s/%s in %s",
				incident.AffectedResources[0].Kind,
				incident.AffectedResources[0].Name,
				ns)
		}
		return fmt.Sprintf("Crash Loops: %d pods affected in %s", resourceCount, ns)

	case CategoryImage:
		return fmt.Sprintf("Image Pull Failures: %d resources in %s", resourceCount, ns)

	case CategoryStorage:
		return fmt.Sprintf("Storage Issues: %d volumes affected in %s", resourceCount, ns)

	case CategoryNode:
		if incident.NodeName != nil {
			return fmt.Sprintf("Node Issue: %s", *incident.NodeName)
		}
		return fmt.Sprintf("Node Issues: %d events", eventCount)

	case CategoryScheduling:
		return fmt.Sprintf("Scheduling Problems: %d pods pending in %s", resourceCount, ns)

	case CategoryDeployment:
		if resourceCount == 1 {
			return fmt.Sprintf("Deployment Failed: %s in %s",
				incident.AffectedResources[0].Name,
				ns)
		}
		return fmt.Sprintf("Deployment Failures: %d workloads in %s", resourceCount, ns)

	default:
		return fmt.Sprintf("Kubernetes Incident: %d events, %d resources in %s",
			eventCount, resourceCount, ns)
	}
}

// findMatchingRule finds the best matching correlation rule for an event
func (e *Engine) findMatchingRule(event *models.Event) *CorrelationRule {
	var bestMatch *CorrelationRule
	bestPriority := -1

	for i := range e.rules {
		rule := &e.rules[i]
		if rule.Match(event) && rule.Priority > bestPriority {
			bestMatch = rule
			bestPriority = rule.Priority
		}
	}

	return bestMatch
}

// isSuppressed checks if an incident with this correlation key should be suppressed
func (e *Engine) isSuppressed(correlationKey string) bool {
	e.suppressionCacheMu.RLock()
	defer e.suppressionCacheMu.RUnlock()

	if lastTime, exists := e.suppressionCache[correlationKey]; exists {
		return time.Since(lastTime) < e.config.SuppressDuplicateWindow
	}
	return false
}

// addToSuppressionCache adds a correlation key to the suppression cache
func (e *Engine) addToSuppressionCache(correlationKey string) {
	e.suppressionCacheMu.Lock()
	defer e.suppressionCacheMu.Unlock()
	e.suppressionCache[correlationKey] = time.Now()
}

// cleanupSuppressionCache periodically removes expired entries from suppression cache
func (e *Engine) cleanupSuppressionCache() {
	defer e.wg.Done()

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.suppressionCacheMu.Lock()
			for key, lastTime := range e.suppressionCache {
				if time.Since(lastTime) > e.config.SuppressDuplicateWindow {
					delete(e.suppressionCache, key)
				}
			}
			e.suppressionCacheMu.Unlock()
		}
	}
}

// GetOpenIncidents returns all currently open incidents
func (e *Engine) GetOpenIncidents() []*Incident {
	e.incidentsMu.RLock()
	defer e.incidentsMu.RUnlock()

	incidents := make([]*Incident, 0, len(e.incidents))
	for _, incident := range e.incidents {
		incidents = append(incidents, incident)
	}

	// Sort by first event time
	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].FirstEventAt.Before(incidents[j].FirstEventAt)
	})

	return incidents
}

// GetOpenIncidentCount returns the number of open incidents
func (e *Engine) GetOpenIncidentCount() int {
	e.incidentsMu.RLock()
	defer e.incidentsMu.RUnlock()
	return len(e.incidents)
}

// defaultCorrelationRules returns the default set of correlation rules
func defaultCorrelationRules() []CorrelationRule {
	return []CorrelationRule{
		// Memory-related events (highest priority - often root cause)
		{
			Name:        "memory-pressure",
			Description: "Memory pressure and OOMKilled events on the same node",
			Category:    CategoryMemory,
			Priority:    100,
			Match: func(event *models.Event) bool {
				return event.EventType == "oom-killed" ||
					event.EventType == "node-memory-pressure" ||
					event.EventType == "pod-evicted"
			},
			CorrelationKey: func(event *models.Event) string {
				// Correlate by node if available, otherwise by namespace
				if meta, ok := event.Metadata["node_name"].(string); ok && meta != "" {
					return fmt.Sprintf("memory:node:%s", meta)
				}
				ns := "default"
				if event.Namespace != nil {
					ns = *event.Namespace
				}
				return fmt.Sprintf("memory:ns:%s", ns)
			},
		},

		// Crash loop events
		{
			Name:        "crash-loop",
			Description: "Container crash loops grouped by owner workload",
			Category:    CategoryCrash,
			Priority:    90,
			Match: func(event *models.Event) bool {
				return event.EventType == "crash-loop-backoff"
			},
			CorrelationKey: func(event *models.Event) string {
				// Correlate by owner (Deployment/StatefulSet/DaemonSet)
				if owner, ok := event.Metadata["owner_uid"].(string); ok && owner != "" {
					return fmt.Sprintf("crash:owner:%s", owner)
				}
				// Fallback to resource UID
				if event.ResourceUID != nil {
					return fmt.Sprintf("crash:pod:%s", *event.ResourceUID)
				}
				return fmt.Sprintf("crash:pod:%s/%s", safeNamespace(event), event.ResourceName)
			},
		},

		// Image pull issues
		{
			Name:        "image-pull",
			Description: "Image pull failures grouped by image name",
			Category:    CategoryImage,
			Priority:    80,
			Match: func(event *models.Event) bool {
				return event.EventType == "image-pull-backoff"
			},
			CorrelationKey: func(event *models.Event) string {
				// Correlate by image if available
				if image, ok := event.Metadata["image"].(string); ok && image != "" {
					// Normalize image name (remove tag for grouping)
					parts := strings.Split(image, ":")
					return fmt.Sprintf("image:%s", parts[0])
				}
				return fmt.Sprintf("image:ns:%s", safeNamespace(event))
			},
		},

		// Node issues
		{
			Name:        "node-condition",
			Description: "Node condition changes and related events",
			Category:    CategoryNode,
			Priority:    95,
			Match: func(event *models.Event) bool {
				return strings.HasPrefix(event.EventType, "node-")
			},
			CorrelationKey: func(event *models.Event) string {
				// Always correlate by node name for node events
				if event.ResourceKind == "Node" {
					return fmt.Sprintf("node:%s", event.ResourceName)
				}
				if nodeName, ok := event.Metadata["node_name"].(string); ok {
					return fmt.Sprintf("node:%s", nodeName)
				}
				return fmt.Sprintf("node:unknown:%d", time.Now().Unix()/300) // 5-min bucket
			},
		},

		// Storage issues
		{
			Name:        "storage",
			Description: "Volume and PVC failures",
			Category:    CategoryStorage,
			Priority:    70,
			Match: func(event *models.Event) bool {
				return event.EventType == "volume-failed-mount" ||
					event.EventType == "volume-failed-binding" ||
					event.EventType == "pvc-provision-failed"
			},
			CorrelationKey: func(event *models.Event) string {
				// Correlate by storage class if available
				if sc, ok := event.Metadata["storage_class"].(string); ok && sc != "" {
					return fmt.Sprintf("storage:class:%s", sc)
				}
				return fmt.Sprintf("storage:ns:%s", safeNamespace(event))
			},
		},

		// Deployment failures
		{
			Name:        "deployment-failure",
			Description: "Deployment and workload failures",
			Category:    CategoryDeployment,
			Priority:    85,
			Match: func(event *models.Event) bool {
				return event.EventType == "deployment-failed" ||
					event.EventType == "statefulset-failed" ||
					event.EventType == "daemonset-failed" ||
					event.EventType == "replicaset-failed" ||
					event.EventType == "job-failed"
			},
			CorrelationKey: func(event *models.Event) string {
				// Correlate by resource UID
				if event.ResourceUID != nil {
					return fmt.Sprintf("workload:%s", *event.ResourceUID)
				}
				return fmt.Sprintf("workload:%s/%s/%s",
					safeNamespace(event), event.ResourceKind, event.ResourceName)
			},
		},

		// Scheduling issues
		{
			Name:        "scheduling",
			Description: "Pod scheduling failures",
			Category:    CategoryScheduling,
			Priority:    60,
			Match: func(event *models.Event) bool {
				return event.EventType == "pod-pending" ||
					event.EventType == "scheduling-failed" ||
					event.EventType == "insufficient-memory" ||
					event.EventType == "insufficient-cpu"
			},
			CorrelationKey: func(event *models.Event) string {
				// Correlate by namespace for scheduling issues
				return fmt.Sprintf("scheduling:ns:%s", safeNamespace(event))
			},
		},
	}
}

// defaultRule is used when no other rule matches
var defaultRule = CorrelationRule{
	Name:        "default",
	Description: "Uncategorized events grouped by namespace",
	Category:    CategoryUnknown,
	Priority:    0,
	Match: func(event *models.Event) bool {
		return true // Matches everything
	},
	CorrelationKey: func(event *models.Event) string {
		return fmt.Sprintf("unknown:ns:%s:type:%s", safeNamespace(event), event.EventType)
	},
}

// safeNamespace returns the namespace or "cluster-wide"
func safeNamespace(event *models.Event) string {
	if event.Namespace != nil && *event.Namespace != "" {
		return *event.Namespace
	}
	return "cluster-wide"
}
