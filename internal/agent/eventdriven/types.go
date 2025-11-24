package eventdriven

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EventType represents the type of Kubernetes event
type EventType string

const (
	// Pod-related events
	EventTypePodFailure        EventType = "pod-failure"
	EventTypeImagePullBackOff  EventType = "image-pull-backoff"
	EventTypeCrashLoopBackOff  EventType = "crash-loop-backoff"
	EventTypeOOMKilled         EventType = "oom-killed"
	EventTypePodEvicted        EventType = "pod-evicted"
	EventTypePodPending        EventType = "pod-pending"
	EventTypeContainerCreating EventType = "container-creating"

	// Workload events
	EventTypeDeploymentFailed  EventType = "deployment-failed"
	EventTypeStatefulSetFailed EventType = "statefulset-failed"
	EventTypeDaemonSetFailed   EventType = "daemonset-failed"
	EventTypeJobFailed         EventType = "job-failed"
	EventTypeReplicaSetFailed  EventType = "replicaset-failed"

	// Volume events
	EventTypeVolumeFailedMount   EventType = "volume-failed-mount"
	EventTypeVolumeFailedBinding EventType = "volume-failed-binding"
	EventTypePVCProvisionFailed  EventType = "pvc-provision-failed"

	// Node events
	EventTypeNodeNotReady       EventType = "node-not-ready"
	EventTypeNodeMemoryPressure EventType = "node-memory-pressure"
	EventTypeNodeDiskPressure   EventType = "node-disk-pressure"
	EventTypeNodePIDPressure    EventType = "node-pid-pressure"
	EventTypeNodeNetworkUnavail EventType = "node-network-unavailable"

	// Scheduling events
	EventTypeSchedulingFailed   EventType = "scheduling-failed"
	EventTypeInsufficientMemory EventType = "insufficient-memory"
	EventTypeInsufficientCPU    EventType = "insufficient-cpu"

	// Generic events
	EventTypeWarning EventType = "warning"
	EventTypeError   EventType = "error"
)

// Severity represents the severity level of an event
type Severity string

const (
	SeverityCritical Severity = "critical" // Requires immediate attention
	SeverityHigh     Severity = "high"     // Should be addressed soon
	SeverityMedium   Severity = "medium"   // Normal priority
	SeverityLow      Severity = "low"      // Informational
)

// KubernetesEvent represents a processed Kubernetes event
type KubernetesEvent struct {
	// Unique identifier (UID + ResourceVersion)
	ID string

	// Event metadata
	Type      EventType
	Severity  Severity
	Timestamp time.Time

	// Resource information
	ResourceKind      string
	ResourceName      string
	ResourceNamespace string
	ResourceUID       string

	// Owner information (for grouping)
	OwnerKind string
	OwnerName string
	OwnerUID  string

	// Event details
	Reason  string
	Message string

	// Related events (for batching)
	RelatedEvents []*KubernetesEvent

	// Metadata
	Labels      map[string]string
	Annotations map[string]string

	// Tracking
	FirstSeen time.Time
	LastSeen  time.Time
	Count     int32
}

// EventKey creates a unique key for deduplication
func (e *KubernetesEvent) EventKey() string {
	return e.ResourceUID + "/" + e.Type.String()
}

// IsRelatedTo checks if this event is related to another event
func (e *KubernetesEvent) IsRelatedTo(other *KubernetesEvent) bool {
	// Same owner means related
	if e.OwnerUID != "" && e.OwnerUID == other.OwnerUID {
		return true
	}
	// Same resource means related
	if e.ResourceUID != "" && e.ResourceUID == other.ResourceUID {
		return true
	}
	return false
}

// String returns the event type as a string
func (t EventType) String() string {
	return string(t)
}

// String returns the severity as a string
func (s Severity) String() string {
	return string(s)
}

// ClassifyEventSeverity determines the severity of an event type
func ClassifyEventSeverity(eventType EventType) Severity {
	switch eventType {
	// Critical events - immediate action required
	case EventTypeOOMKilled,
		EventTypePodEvicted,
		EventTypeNodeNotReady,
		EventTypeJobFailed:
		return SeverityCritical

	// High severity - should be addressed soon
	case EventTypeImagePullBackOff,
		EventTypeCrashLoopBackOff,
		EventTypeDeploymentFailed,
		EventTypeStatefulSetFailed,
		EventTypeDaemonSetFailed,
		EventTypeVolumeFailedMount,
		EventTypeVolumeFailedBinding,
		EventTypeNodeMemoryPressure,
		EventTypeNodeDiskPressure:
		return SeverityHigh

	// Medium severity - normal priority
	case EventTypePodPending,
		EventTypeSchedulingFailed,
		EventTypeInsufficientMemory,
		EventTypeInsufficientCPU,
		EventTypePVCProvisionFailed:
		return SeverityMedium

	// Low severity - informational
	case EventTypeContainerCreating,
		EventTypeNodePIDPressure,
		EventTypeNodeNetworkUnavail:
		return SeverityLow

	// Default to medium
	default:
		return SeverityMedium
	}
}

// ResourceEventInfo extracts common event information from a Kubernetes object
type ResourceEventInfo struct {
	Kind        string
	Name        string
	Namespace   string
	UID         string
	Labels      map[string]string
	Annotations map[string]string
	OwnerRefs   []metav1.OwnerReference
	CreatedAt   time.Time
}

// GetOwnerInfo extracts owner information from OwnerReferences
func (r *ResourceEventInfo) GetOwnerInfo() (kind, name, uid string) {
	if len(r.OwnerRefs) == 0 {
		return "", "", ""
	}
	// Use the first controller owner, or first owner if no controller
	for _, ref := range r.OwnerRefs {
		if ref.Controller != nil && *ref.Controller {
			return ref.Kind, ref.Name, string(ref.UID)
		}
	}
	// Fallback to first owner
	ref := r.OwnerRefs[0]
	return ref.Kind, ref.Name, string(ref.UID)
}

// EventFilter defines criteria for filtering events
type EventFilter struct {
	// EventTypes to watch (empty = all)
	EventTypes []EventType
	// Namespaces to watch (empty = all)
	Namespaces []string
	// MinSeverity filters events below this severity
	MinSeverity Severity
	// ExcludeLabels filters out resources with these labels
	ExcludeLabels map[string]string
	// IncludeLabels only includes resources with these labels
	IncludeLabels map[string]string
}

// ShouldProcess checks if an event should be processed based on the filter
func (f *EventFilter) ShouldProcess(event *KubernetesEvent) bool {
	// Check namespace
	if len(f.Namespaces) > 0 {
		found := false
		for _, ns := range f.Namespaces {
			if event.ResourceNamespace == ns {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check event type
	if len(f.EventTypes) > 0 {
		found := false
		for _, et := range f.EventTypes {
			if event.Type == et {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check severity
	if !f.meetsMinSeverity(event.Severity) {
		return false
	}

	// Check include labels
	if len(f.IncludeLabels) > 0 {
		for key, value := range f.IncludeLabels {
			if event.Labels[key] != value {
				return false
			}
		}
	}

	// Check exclude labels
	if len(f.ExcludeLabels) > 0 {
		for key, value := range f.ExcludeLabels {
			if event.Labels[key] == value {
				return false
			}
		}
	}

	return true
}

// meetsMinSeverity checks if event severity meets minimum requirement
func (f *EventFilter) meetsMinSeverity(severity Severity) bool {
	severityOrder := map[Severity]int{
		SeverityLow:      1,
		SeverityMedium:   2,
		SeverityHigh:     3,
		SeverityCritical: 4,
	}

	minOrder := severityOrder[f.MinSeverity]
	eventOrder := severityOrder[severity]

	return eventOrder >= minOrder
}
