// Package correlation provides smart alert deduplication for incidents.
package correlation

import (
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/database/models"
)

// Prometheus metrics for deduplication
var (
	incidentsSuppressedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_incidents_suppressed_total",
			Help: "Total number of incidents suppressed by deduplication",
		},
		[]string{"reason"},
	)

	incidentsEscalatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "lumo_incidents_escalated_total",
			Help: "Total number of incidents that bypassed suppression due to severity escalation",
		},
	)

	suppressedIncidentsGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lumo_suppressed_incidents",
			Help: "Current number of incidents being suppressed (in dedup window)",
		},
	)

	suppressedOccurrencesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "lumo_suppressed_occurrences_total",
			Help: "Total number of suppressed incident occurrences (aggregated)",
		},
	)
)

// SuppressionReason indicates why an incident was suppressed
type SuppressionReason string

const (
	SuppressionNotSuppressed    SuppressionReason = ""
	SuppressionDuplicate        SuppressionReason = "duplicate"
	SuppressionTooFrequent      SuppressionReason = "too_frequent"
	SuppressionAggregated       SuppressionReason = "aggregated"
	SuppressionSeverityEscalate SuppressionReason = "severity_escalation" // NOT suppressed - bypassed
)

// IncidentRecord tracks a deduplicated incident
type IncidentRecord struct {
	IncidentID       uuid.UUID              `json:"incident_id"`
	CorrelationKey   string                 `json:"correlation_key"`
	Category         IncidentCategory       `json:"category"`
	HighestSeverity  models.EventSeverity   `json:"highest_severity"`
	FirstSeen        time.Time              `json:"first_seen"`
	LastSeen         time.Time              `json:"last_seen"`
	LastNotified     time.Time              `json:"last_notified"`
	OccurrenceCount  int                    `json:"occurrence_count"`
	SuppressedCount  int                    `json:"suppressed_count"`
	Suppressed       bool                   `json:"suppressed"`
	SuppressedEvents []*SuppressedEventInfo `json:"suppressed_events,omitempty"` // Recent suppressed events (capped)
}

// SuppressedEventInfo stores brief info about a suppressed event
type SuppressedEventInfo struct {
	EventID   uuid.UUID `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// DeduplicationCache provides smart alert deduplication
type DeduplicationCache struct {
	records map[string]*IncidentRecord // key: correlation_key
	mu      sync.RWMutex
	logger  *logrus.Entry

	// Configuration
	windowDuration      time.Duration
	maxSuppressedEvents int           // Max suppressed events to track per record
	severityEscalation  bool          // Allow severity escalation to bypass suppression
	minNotifyInterval   time.Duration // Minimum time between notifications for same key
}

// DeduplicationConfig configures the deduplication behavior
type DeduplicationConfig struct {
	// WindowDuration is how long to suppress duplicates after first notification
	WindowDuration time.Duration
	// MaxSuppressedEvents is how many suppressed events to track per correlation key
	MaxSuppressedEvents int
	// SeverityEscalation allows higher severity to bypass suppression
	SeverityEscalation bool
	// MinNotifyInterval is minimum time between notifications (prevents spam)
	MinNotifyInterval time.Duration
}

// DefaultDeduplicationConfig returns sensible defaults
func DefaultDeduplicationConfig() *DeduplicationConfig {
	return &DeduplicationConfig{
		WindowDuration:      1 * time.Hour,
		MaxSuppressedEvents: 10,
		SeverityEscalation:  true,
		MinNotifyInterval:   5 * time.Minute,
	}
}

// NewDeduplicationCache creates a new deduplication cache
func NewDeduplicationCache(config *DeduplicationConfig, logger *logrus.Logger) *DeduplicationCache {
	if config == nil {
		config = DefaultDeduplicationConfig()
	}

	return &DeduplicationCache{
		records:             make(map[string]*IncidentRecord),
		logger:              logger.WithField("component", "deduplication"),
		windowDuration:      config.WindowDuration,
		maxSuppressedEvents: config.MaxSuppressedEvents,
		severityEscalation:  config.SeverityEscalation,
		minNotifyInterval:   config.MinNotifyInterval,
	}
}

// ShouldSuppress determines if an incident should be deduplicated
// Returns: (suppress bool, reason SuppressionReason, existingRecord *IncidentRecord)
func (c *DeduplicationCache) ShouldSuppress(
	correlationKey string,
	severity models.EventSeverity,
	incidentID uuid.UUID,
) (bool, SuppressionReason, *IncidentRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, exists := c.records[correlationKey]
	now := time.Now()

	// No existing record - don't suppress, create new record
	if !exists {
		c.records[correlationKey] = &IncidentRecord{
			IncidentID:       incidentID,
			CorrelationKey:   correlationKey,
			HighestSeverity:  severity,
			FirstSeen:        now,
			LastSeen:         now,
			LastNotified:     now,
			OccurrenceCount:  1,
			SuppressedCount:  0,
			Suppressed:       false,
			SuppressedEvents: make([]*SuppressedEventInfo, 0),
		}
		suppressedIncidentsGauge.Inc()
		return false, SuppressionNotSuppressed, nil
	}

	// Update record
	record.LastSeen = now
	record.OccurrenceCount++

	// Check if suppression window has expired
	if now.Sub(record.LastNotified) > c.windowDuration {
		// Window expired - allow notification and reset
		c.logger.WithFields(logrus.Fields{
			"correlation_key":   correlationKey,
			"occurrences":       record.OccurrenceCount,
			"suppressed_count":  record.SuppressedCount,
			"window_expired_at": record.LastNotified.Add(c.windowDuration),
		}).Debug("Deduplication window expired, allowing notification")

		record.LastNotified = now
		record.SuppressedCount = 0
		record.SuppressedEvents = make([]*SuppressedEventInfo, 0)
		record.Suppressed = false
		return false, SuppressionNotSuppressed, record
	}

	// Check for severity escalation
	if c.severityEscalation && isSeverityHigher(severity, record.HighestSeverity) {
		c.logger.WithFields(logrus.Fields{
			"correlation_key": correlationKey,
			"old_severity":    record.HighestSeverity,
			"new_severity":    severity,
		}).Info("Severity escalation - bypassing suppression")

		record.HighestSeverity = severity
		record.LastNotified = now
		incidentsEscalatedTotal.Inc()
		return false, SuppressionSeverityEscalate, record
	}

	// Check minimum notify interval
	if now.Sub(record.LastNotified) < c.minNotifyInterval {
		record.SuppressedCount++
		record.Suppressed = true
		incidentsSuppressedTotal.WithLabelValues("too_frequent").Inc()
		suppressedOccurrencesTotal.Inc()
		return true, SuppressionTooFrequent, record
	}

	// Within window - suppress as duplicate
	record.SuppressedCount++
	record.Suppressed = true
	incidentsSuppressedTotal.WithLabelValues("duplicate").Inc()
	suppressedOccurrencesTotal.Inc()

	c.logger.WithFields(logrus.Fields{
		"correlation_key":  correlationKey,
		"suppressed_count": record.SuppressedCount,
		"occurrences":      record.OccurrenceCount,
	}).Debug("Incident suppressed as duplicate")

	return true, SuppressionDuplicate, record
}

// AddSuppressedEvent records a suppressed event for later aggregation
func (c *DeduplicationCache) AddSuppressedEvent(correlationKey string, eventID uuid.UUID, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, exists := c.records[correlationKey]
	if !exists {
		return
	}

	info := &SuppressedEventInfo{
		EventID:   eventID,
		Timestamp: time.Now(),
		Message:   truncateMessage(message, 100),
	}

	// Cap the stored events
	if len(record.SuppressedEvents) >= c.maxSuppressedEvents {
		// Remove oldest
		record.SuppressedEvents = record.SuppressedEvents[1:]
	}
	record.SuppressedEvents = append(record.SuppressedEvents, info)
}

// GetRecord retrieves an incident record by correlation key
func (c *DeduplicationCache) GetRecord(correlationKey string) (*IncidentRecord, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	record, exists := c.records[correlationKey]
	return record, exists
}

// GetSuppressedRecords returns all currently suppressed records
func (c *DeduplicationCache) GetSuppressedRecords() []*IncidentRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	records := make([]*IncidentRecord, 0)
	for _, record := range c.records {
		if record.Suppressed && record.SuppressedCount > 0 {
			records = append(records, record)
		}
	}
	return records
}

// GetAggregationSummary returns a summary of suppressed incidents for notification
func (c *DeduplicationCache) GetAggregationSummary(correlationKey string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	record, exists := c.records[correlationKey]
	if !exists || record.SuppressedCount == 0 {
		return ""
	}

	return formatAggregationSummary(record)
}

// MarkNotified updates the last notification time for a correlation key
func (c *DeduplicationCache) MarkNotified(correlationKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if record, exists := c.records[correlationKey]; exists {
		record.LastNotified = time.Now()
		record.Suppressed = false
	}
}

// IsSuppressed checks if a correlation key is currently being suppressed
// This is a simple check for use in the realtime manager
func (c *DeduplicationCache) IsSuppressed(correlationKey string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if record, exists := c.records[correlationKey]; exists {
		// Suppressed if within window and hasn't escalated
		return time.Since(record.LastNotified) < c.windowDuration
	}
	return false
}

// Cleanup removes expired entries from the cache
func (c *DeduplicationCache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, record := range c.records {
		// Remove if window expired and no recent activity
		if now.Sub(record.LastSeen) > c.windowDuration*2 {
			delete(c.records, key)
			suppressedIncidentsGauge.Dec()
			removed++
		}
	}

	if removed > 0 {
		c.logger.WithField("removed", removed).Debug("Cleaned up deduplication cache")
	}

	return removed
}

// Stats returns current deduplication statistics
func (c *DeduplicationCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalSuppressed := 0
	totalOccurrences := 0
	for _, record := range c.records {
		totalSuppressed += record.SuppressedCount
		totalOccurrences += record.OccurrenceCount
	}

	return map[string]interface{}{
		"tracked_keys":        len(c.records),
		"total_occurrences":   totalOccurrences,
		"total_suppressed":    totalSuppressed,
		"window_duration":     c.windowDuration.String(),
		"severity_escalation": c.severityEscalation,
	}
}

// isSeverityHigher returns true if newSeverity is higher than oldSeverity
func isSeverityHigher(newSeverity, oldSeverity models.EventSeverity) bool {
	severityOrder := map[models.EventSeverity]int{
		models.EventSeverityLow:      1,
		models.EventSeverityMedium:   2,
		models.EventSeverityHigh:     3,
		models.EventSeverityCritical: 4,
	}

	newOrder, okNew := severityOrder[newSeverity]
	oldOrder, okOld := severityOrder[oldSeverity]

	if !okNew || !okOld {
		return false
	}

	return newOrder > oldOrder
}

// truncateMessage truncates a message to maxLen characters
func truncateMessage(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen-3] + "..."
}

// formatAggregationSummary creates a human-readable summary of suppressed events
func formatAggregationSummary(record *IncidentRecord) string {
	if record.SuppressedCount == 0 {
		return ""
	}

	duration := time.Since(record.FirstSeen).Round(time.Minute)
	return "📊 *Aggregated:* " +
		"+" + formatCount(record.SuppressedCount) + " similar events in " + duration.String() +
		" (total: " + formatCount(record.OccurrenceCount) + " occurrences)"
}

// formatCount formats a count for display
func formatCount(n int) string {
	return strconv.Itoa(n)
}
