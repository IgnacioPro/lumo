package eventdriven

import (
	"fmt"
	"time"
)

// EventBuilder provides a fluent API for constructing KubernetesEvent objects.
// This reduces boilerplate and ensures consistent event creation across watchers.
type EventBuilder struct {
	event *KubernetesEvent
}

// NewEventBuilder creates a new EventBuilder with required fields.
func NewEventBuilder(eventType EventType, resourceKind, resourceName, resourceNamespace string) *EventBuilder {
	return &EventBuilder{
		event: &KubernetesEvent{
			Type:              eventType,
			Severity:          ClassifyEventSeverity(eventType),
			Timestamp:         time.Now(),
			ResourceKind:      resourceKind,
			ResourceName:      resourceName,
			ResourceNamespace: resourceNamespace,
			FirstSeen:         time.Now(),
			LastSeen:          time.Now(),
			Count:             1,
		},
	}
}

// WithID sets the event ID.
func (b *EventBuilder) WithID(id string) *EventBuilder {
	b.event.ID = id
	return b
}

// WithIDFromUID generates an ID from resource UID and event type suffix.
func (b *EventBuilder) WithIDFromUID(uid, suffix string) *EventBuilder {
	b.event.ID = fmt.Sprintf("%s-%s", uid, suffix)
	return b
}

// WithResourceUID sets the resource UID.
func (b *EventBuilder) WithResourceUID(uid string) *EventBuilder {
	b.event.ResourceUID = uid
	return b
}

// WithOwner sets the owner information.
func (b *EventBuilder) WithOwner(kind, name, uid string) *EventBuilder {
	b.event.OwnerKind = kind
	b.event.OwnerName = name
	b.event.OwnerUID = uid
	return b
}

// WithReason sets the event reason.
func (b *EventBuilder) WithReason(reason string) *EventBuilder {
	b.event.Reason = reason
	return b
}

// WithMessage sets the event message.
func (b *EventBuilder) WithMessage(message string) *EventBuilder {
	b.event.Message = message
	return b
}

// WithMessagef sets the event message with formatting.
func (b *EventBuilder) WithMessagef(format string, args ...interface{}) *EventBuilder {
	b.event.Message = fmt.Sprintf(format, args...)
	return b
}

// WithSeverity overrides the auto-classified severity.
func (b *EventBuilder) WithSeverity(severity Severity) *EventBuilder {
	b.event.Severity = severity
	return b
}

// WithLabels sets the labels map.
func (b *EventBuilder) WithLabels(labels map[string]string) *EventBuilder {
	b.event.Labels = labels
	return b
}

// WithAnnotations sets the annotations map.
func (b *EventBuilder) WithAnnotations(annotations map[string]string) *EventBuilder {
	b.event.Annotations = annotations
	return b
}

// WithTimestamp sets a custom timestamp.
func (b *EventBuilder) WithTimestamp(t time.Time) *EventBuilder {
	b.event.Timestamp = t
	return b
}

// WithFirstSeen sets when the event was first seen.
func (b *EventBuilder) WithFirstSeen(t time.Time) *EventBuilder {
	b.event.FirstSeen = t
	return b
}

// WithCount sets the event count.
func (b *EventBuilder) WithCount(count int32) *EventBuilder {
	b.event.Count = count
	return b
}

// Build returns the constructed KubernetesEvent.
func (b *EventBuilder) Build() *KubernetesEvent {
	return b.event
}

// ResourceEventInfoBuilder provides a fluent API for constructing ResourceEventInfo.
type ResourceEventInfoBuilder struct {
	info *ResourceEventInfo
}

// NewResourceInfoBuilder creates a new ResourceEventInfoBuilder.
func NewResourceInfoBuilder(kind, name, namespace, uid string) *ResourceEventInfoBuilder {
	return &ResourceEventInfoBuilder{
		info: &ResourceEventInfo{
			Kind:      kind,
			Name:      name,
			Namespace: namespace,
			UID:       uid,
			CreatedAt: time.Now(),
		},
	}
}

// WithLabels sets the labels.
func (b *ResourceEventInfoBuilder) WithLabels(labels map[string]string) *ResourceEventInfoBuilder {
	b.info.Labels = labels
	return b
}

// WithAnnotations sets the annotations.
func (b *ResourceEventInfoBuilder) WithAnnotations(annotations map[string]string) *ResourceEventInfoBuilder {
	b.info.Annotations = annotations
	return b
}

// WithCreatedAt sets the creation timestamp.
func (b *ResourceEventInfoBuilder) WithCreatedAt(t time.Time) *ResourceEventInfoBuilder {
	b.info.CreatedAt = t
	return b
}

// Build returns the constructed ResourceEventInfo.
func (b *ResourceEventInfoBuilder) Build() *ResourceEventInfo {
	return b.info
}
