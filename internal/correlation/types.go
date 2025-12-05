// Package correlation provides incident correlation and context gathering
// for Kubernetes events. Instead of treating each event in isolation, the
// correlation engine groups related events into incidents, fetches contextual
// data (logs, metrics, related events), and generates comprehensive AI analysis.
//
// The key insight: 50 individual alerts about pods crashing are noise.
// One incident report saying "Memory leak in service X caused cascading
// failures affecting 12 pods" is actionable intelligence.
package correlation

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ignacio/lumo/internal/database/models"
)

// IncidentState represents the lifecycle state of an incident
type IncidentState string

const (
	// IncidentStateOpen indicates an active incident collecting events
	IncidentStateOpen IncidentState = "open"
	// IncidentStateAnalyzing indicates the incident is being analyzed by AI
	IncidentStateAnalyzing IncidentState = "analyzing"
	// IncidentStateResolved indicates the incident has been processed and notified
	IncidentStateResolved IncidentState = "resolved"
	// IncidentStateSuppressed indicates the incident was suppressed (duplicate, low priority)
	IncidentStateSuppressed IncidentState = "suppressed"
)

// IncidentCategory represents the high-level category of an incident
type IncidentCategory string

const (
	CategoryMemory     IncidentCategory = "memory"     // OOMKilled, MemoryPressure
	CategoryCrash      IncidentCategory = "crash"      // CrashLoopBackOff, container failures
	CategoryImage      IncidentCategory = "image"      // ImagePullBackOff, registry issues
	CategoryStorage    IncidentCategory = "storage"    // PVC failures, mount issues
	CategoryNode       IncidentCategory = "node"       // Node conditions, NotReady
	CategoryScheduling IncidentCategory = "scheduling" // Pending pods, insufficient resources
	CategoryDeployment IncidentCategory = "deployment" // Rollout failures, replica issues
	CategoryNetwork    IncidentCategory = "network"    // Network unavailable, DNS issues
	CategoryUnknown    IncidentCategory = "unknown"    // Uncategorized events
)

// Incident represents a correlated group of related Kubernetes events
// that together form a single actionable incident
type Incident struct {
	// Unique identifier for this incident
	ID uuid.UUID `json:"id"`

	// Tenant context
	TenantID uuid.UUID `json:"tenant_id,omitempty"`

	// Incident classification
	Category IncidentCategory     `json:"category"`
	Severity models.EventSeverity `json:"severity"`
	State    IncidentState        `json:"state"`

	// IsCritical indicates if this incident affects critical services
	// Critical incidents bypass debouncing and get immediate notification
	IsCritical bool `json:"is_critical"`

	// Human-readable title (generated after correlation)
	Title string `json:"title"`

	// Root cause analysis (populated by AI)
	RootCause string `json:"root_cause,omitempty"`

	// Summary of what happened (populated by AI)
	Summary string `json:"summary,omitempty"`

	// Postmortem generated when incident closes
	Postmortem string `json:"postmortem,omitempty"`

	// Timeline of events
	Timeline []TimelineEntry `json:"timeline"`

	// All events that are part of this incident
	Events []*models.Event `json:"events"`

	// Affected resources
	AffectedResources []AffectedResource `json:"affected_resources"`

	// Primary affected resource (usually the root cause resource)
	PrimaryResource *AffectedResource `json:"primary_resource,omitempty"`

	// Cluster/Node context
	ClusterName string  `json:"cluster_name,omitempty"`
	NodeName    *string `json:"node_name,omitempty"` // If incident is node-scoped

	// Namespace (if all events are in same namespace)
	Namespace *string `json:"namespace,omitempty"`

	// Context gathered for AI analysis
	Context *IncidentContext `json:"context,omitempty"`

	// AI Analysis result
	AIAnalysis *AIAnalysisResult `json:"ai_analysis,omitempty"`

	// Analysis log for incremental updates
	AnalysisLog []AnalysisLogEntry `json:"analysis_log,omitempty"`

	// Timestamps
	FirstEventAt      time.Time  `json:"first_event_at"`
	LastEventAt       time.Time  `json:"last_event_at"`
	OpenedAt          time.Time  `json:"opened_at"`
	ClosedAt          *time.Time `json:"closed_at,omitempty"`
	AnalyzedAt        *time.Time `json:"analyzed_at,omitempty"`
	NotifiedAt        *time.Time `json:"notified_at,omitempty"`
	LastHealthCheckAt *time.Time `json:"last_health_check_at,omitempty"`

	// Notification tracking
	NotificationChannels []string   `json:"notification_channels,omitempty"`
	NotificationCount    int        `json:"notification_count"`
	LastNotificationAt   *time.Time `json:"last_notification_at,omitempty"`
	ThreadTS             string     `json:"thread_ts,omitempty"` // Slack thread timestamp

	// Correlation metadata
	CorrelationKey    string                 `json:"correlation_key"`
	CorrelationReason string                 `json:"correlation_reason"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`

	// Internal synchronization
	mu sync.RWMutex `json:"-"`
}

// TimelineEntry represents a single entry in the incident timeline
type TimelineEntry struct {
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	Resource  string    `json:"resource"` // kind/name
	Namespace string    `json:"namespace"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	EventID   uuid.UUID `json:"event_id"`
}

// AffectedResource represents a Kubernetes resource affected by the incident
type AffectedResource struct {
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	UID       string            `json:"uid,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`

	// Owner chain (e.g., Pod -> ReplicaSet -> Deployment)
	OwnerChain []ResourceRef `json:"owner_chain,omitempty"`

	// Resource-specific details
	Details map[string]interface{} `json:"details,omitempty"`
}

// ResourceRef is a reference to a Kubernetes resource
type ResourceRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	UID  string `json:"uid,omitempty"`
}

// IncidentContext holds contextual data gathered to enrich AI analysis
type IncidentContext struct {
	// Pod logs from affected pods (last N lines)
	PodLogs map[string][]LogEntry `json:"pod_logs,omitempty"`

	// Recent Kubernetes events for affected resources
	RelatedK8sEvents []K8sEventSummary `json:"related_k8s_events,omitempty"`

	// Node conditions (if node-related incident)
	NodeConditions map[string][]NodeCondition `json:"node_conditions,omitempty"`

	// Resource metrics (if available)
	Metrics *ResourceMetrics `json:"metrics,omitempty"`

	// Recent changes (deployments, config changes)
	RecentChanges []ChangeEvent `json:"recent_changes,omitempty"`

	// Error patterns extracted from logs
	ErrorPatterns []ErrorPattern `json:"error_patterns,omitempty"`

	// Timestamp of context gathering
	GatheredAt time.Time `json:"gathered_at"`
}

// LogEntry represents a single log line
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Container string    `json:"container"`
	Message   string    `json:"message"`
	Level     string    `json:"level,omitempty"` // info, warn, error (if detected)
}

// K8sEventSummary is a summary of a Kubernetes event
type K8sEventSummary struct {
	Type           string    `json:"type"` // Normal, Warning
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	Count          int32     `json:"count"`
	FirstTimestamp time.Time `json:"first_timestamp"`
	LastTimestamp  time.Time `json:"last_timestamp"`
	Resource       string    `json:"resource"` // kind/name
}

// NodeCondition represents a Kubernetes node condition
type NodeCondition struct {
	Type    string    `json:"type"`
	Status  string    `json:"status"`
	Reason  string    `json:"reason,omitempty"`
	Message string    `json:"message,omitempty"`
	Updated time.Time `json:"updated"`
}

// ResourceMetrics holds resource utilization metrics
type ResourceMetrics struct {
	// Node-level metrics
	NodeMetrics map[string]NodeMetric `json:"node_metrics,omitempty"`

	// Pod-level metrics
	PodMetrics map[string]PodMetric `json:"pod_metrics,omitempty"`
}

// NodeMetric represents metrics for a node
type NodeMetric struct {
	CPUUsagePercent    float64   `json:"cpu_usage_percent"`
	MemoryUsagePercent float64   `json:"memory_usage_percent"`
	MemoryUsageBytes   int64     `json:"memory_usage_bytes"`
	DiskUsagePercent   float64   `json:"disk_usage_percent,omitempty"`
	PodCount           int       `json:"pod_count"`
	Timestamp          time.Time `json:"timestamp"`
}

// PodMetric represents metrics for a pod
type PodMetric struct {
	CPUUsageCores      float64   `json:"cpu_usage_cores"`
	MemoryUsageBytes   int64     `json:"memory_usage_bytes"`
	MemoryLimitBytes   int64     `json:"memory_limit_bytes,omitempty"`
	MemoryUsagePercent float64   `json:"memory_usage_percent,omitempty"`
	RestartCount       int32     `json:"restart_count"`
	Timestamp          time.Time `json:"timestamp"`
}

// ChangeEvent represents a recent change in the cluster
type ChangeEvent struct {
	Type      string    `json:"type"`   // deployment, configmap, secret, etc.
	Action    string    `json:"action"` // created, updated, deleted
	Resource  string    `json:"resource"`
	Namespace string    `json:"namespace,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user,omitempty"` // Who made the change
}

// ErrorPattern represents a detected error pattern in logs
type ErrorPattern struct {
	Pattern     string   `json:"pattern"`
	Occurrences int      `json:"occurrences"`
	Samples     []string `json:"samples"`    // Example log lines
	Containers  []string `json:"containers"` // Which containers had this error
}

// AIAnalysisResult holds the structured AI analysis of an incident
type AIAnalysisResult struct {
	// Root cause identification
	RootCause RootCauseAnalysis `json:"root_cause"`

	// Impact assessment
	Impact ImpactAssessment `json:"impact"`

	// Recommended actions
	ImmediateActions []RecommendedAction `json:"immediate_actions"`
	LongTermActions  []RecommendedAction `json:"long_term_actions"`

	// Monitoring recommendations
	MonitoringRecommendations []string `json:"monitoring_recommendations"`

	// Confidence score (0-100)
	Confidence int `json:"confidence"`

	// Full analysis text (markdown)
	FullAnalysis string `json:"full_analysis"`

	// Tokens used for this analysis
	TokensUsed int `json:"tokens_used,omitempty"`

	// Time taken for analysis
	AnalysisDuration time.Duration `json:"analysis_duration"`
}

// RootCauseAnalysis describes the identified root cause
type RootCauseAnalysis struct {
	Summary     string   `json:"summary"`
	Explanation string   `json:"explanation"`
	Evidence    []string `json:"evidence"`   // Supporting evidence from events/logs
	Confidence  int      `json:"confidence"` // 0-100
}

// ImpactAssessment describes the impact of the incident
type ImpactAssessment struct {
	Severity         string   `json:"severity"`
	AffectedUsers    string   `json:"affected_users,omitempty"` // e.g., "All users of service X"
	AffectedServices []string `json:"affected_services"`
	DataLoss         bool     `json:"data_loss"`
	ServiceDegraded  bool     `json:"service_degraded"`
	ServiceDown      bool     `json:"service_down"`
}

// RecommendedAction represents a recommended remediation action
type RecommendedAction struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"` // kubectl command if applicable
	Priority    int    `json:"priority"`          // 1 = highest
	Automated   bool   `json:"automated"`         // Can be auto-remediated
}

// AnalysisLogEntry represents an incremental AI analysis update
type AnalysisLogEntry struct {
	ID           uuid.UUID `json:"id"`
	IncidentID   uuid.UUID `json:"incident_id"`
	AnalysisType string    `json:"analysis_type"` // initial, update, insight, root_cause, postmortem
	Content      string    `json:"content"`
	Notified     bool      `json:"notified"`
	CreatedAt    time.Time `json:"created_at"`
}

// AnalysisType constants for incremental analysis
const (
	AnalysisTypeInitial    = "initial"    // First "Lumo is on it" notification
	AnalysisTypeUpdate     = "update"     // Progress update (every 45s if no insights)
	AnalysisTypeInsight    = "insight"    // AI found something useful
	AnalysisTypeRootCause  = "root_cause" // Root cause identified
	AnalysisTypePostmortem = "postmortem" // Final postmortem
)

// CorrelationRule defines a rule for correlating events
type CorrelationRule struct {
	Name        string
	Description string
	Category    IncidentCategory
	// Match returns true if the event matches this rule
	Match func(event *models.Event) bool
	// CorrelationKey generates a key for grouping related events
	CorrelationKey func(event *models.Event) string
	// Priority determines which rule wins if multiple match (higher = higher priority)
	Priority int
}

// EngineConfig holds configuration for the correlation engine
type EngineConfig struct {
	// CorrelationWindow is how long to wait for related events (non-critical)
	// before closing an incident for analysis (default: 5 minutes)
	// NOTE: Critical incidents bypass this entirely
	CorrelationWindow time.Duration

	// MinEventsForIncident is the minimum events needed to form an incident
	// (default: 1 - single high-severity events can be incidents)
	MinEventsForIncident int

	// MaxEventsPerIncident limits incident size to prevent runaway correlation
	// (default: 100)
	MaxEventsPerIncident int

	// ContextGatherTimeout is how long to wait for context gathering
	// (default: 30 seconds)
	ContextGatherTimeout time.Duration

	// AIAnalysisTimeout is the timeout for AI analysis
	// (default: 60 seconds)
	AIAnalysisTimeout time.Duration

	// LogLinesPerPod is how many log lines to fetch per pod
	// (default: 100)
	LogLinesPerPod int

	// MetricsLookback is how far back to look for metrics
	// (default: 15 minutes)
	MetricsLookback time.Duration

	// EnableContextGathering enables fetching logs/metrics
	// (default: true)
	EnableContextGathering bool

	// EnableAIAnalysis enables AI-powered analysis
	// (default: true)
	EnableAIAnalysis bool

	// SuppressDuplicateWindow is how long to suppress similar incidents
	// (default: 1 hour)
	SuppressDuplicateWindow time.Duration

	// HealthCheckInterval is how often to check if incident resources are healthy
	// (default: 30 seconds)
	HealthCheckInterval time.Duration

	// ProgressNotificationInterval is how often to send "still working on it" updates
	// when no insights have been found (default: 45 seconds)
	ProgressNotificationInterval time.Duration

	// CriticalLabels are the labels that mark a resource as critical
	// (default: ["critical=true"])
	CriticalLabels map[string]string
}

// DefaultEngineConfig returns sensible defaults
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		CorrelationWindow:            5 * time.Minute,
		MinEventsForIncident:         1,
		MaxEventsPerIncident:         100,
		ContextGatherTimeout:         30 * time.Second,
		AIAnalysisTimeout:            60 * time.Second,
		LogLinesPerPod:               100,
		MetricsLookback:              15 * time.Minute,
		EnableContextGathering:       true,
		EnableAIAnalysis:             true,
		SuppressDuplicateWindow:      1 * time.Hour,
		HealthCheckInterval:          30 * time.Second,
		ProgressNotificationInterval: 45 * time.Second,
		CriticalLabels: map[string]string{
			"critical": "true",
		},
	}
}

// AddEvent adds an event to the incident
func (i *Incident) AddEvent(event *models.Event) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.Events = append(i.Events, event)

	// Update timestamps
	if event.EventTimestamp.Before(i.FirstEventAt) {
		i.FirstEventAt = event.EventTimestamp
	}
	if event.EventTimestamp.After(i.LastEventAt) {
		i.LastEventAt = event.EventTimestamp
	}

	// Update severity (upgrade to higher severity)
	if severityOrder(event.Severity) > severityOrder(i.Severity) {
		i.Severity = event.Severity
	}

	// Add to timeline
	ns := ""
	if event.Namespace != nil {
		ns = *event.Namespace
	}
	i.Timeline = append(i.Timeline, TimelineEntry{
		Timestamp: event.EventTimestamp,
		EventType: event.EventType,
		Resource:  event.ResourceKind + "/" + event.ResourceName,
		Namespace: ns,
		Message:   event.Message,
		Severity:  string(event.Severity),
		EventID:   event.ID,
	})

	// Track affected resources
	i.trackAffectedResource(event)
}

// trackAffectedResource adds a resource to the affected list if not already present
func (i *Incident) trackAffectedResource(event *models.Event) {
	ns := ""
	if event.Namespace != nil {
		ns = *event.Namespace
	}
	uid := ""
	if event.ResourceUID != nil {
		uid = *event.ResourceUID
	}

	// Check if already tracked
	for _, r := range i.AffectedResources {
		if r.Kind == event.ResourceKind && r.Name == event.ResourceName && r.Namespace == ns {
			return
		}
	}

	i.AffectedResources = append(i.AffectedResources, AffectedResource{
		Kind:      event.ResourceKind,
		Name:      event.ResourceName,
		Namespace: ns,
		UID:       uid,
	})
}

// EventCount returns the number of events in this incident
func (i *Incident) EventCount() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.Events)
}

// Duration returns how long the incident has been active
func (i *Incident) Duration() time.Duration {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.LastEventAt.Sub(i.FirstEventAt)
}

// IsOpen returns true if the incident is still collecting events
func (i *Incident) IsOpen() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.State == IncidentStateOpen
}

// severityOrder returns numeric order for severity comparison
func severityOrder(s models.EventSeverity) int {
	switch s {
	case models.EventSeverityLow:
		return 1
	case models.EventSeverityMedium:
		return 2
	case models.EventSeverityHigh:
		return 3
	case models.EventSeverityCritical:
		return 4
	default:
		return 0
	}
}
