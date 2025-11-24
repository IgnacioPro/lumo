package eventdriven

import (
	"sync"

	"github.com/sirupsen/logrus"
)

// BaseHandler provides common functionality for event handlers
type BaseHandler struct {
	debouncer       *Debouncer
	filter          *EventFilter
	logger          *logrus.Entry
	processor       EventProcessor
	grouper         *EventGrouper
	groupingEnabled bool
}

// EventProcessor processes debounced events
type EventProcessor interface {
	// Process handles a debounced event (after wait window)
	Process(event *KubernetesEvent) error
}

// NewBaseHandler creates a new base event handler
func NewBaseHandler(
	debouncer *Debouncer,
	filter *EventFilter,
	processor EventProcessor,
	grouper *EventGrouper,
	groupingEnabled bool,
	logger *logrus.Logger,
) *BaseHandler {
	return &BaseHandler{
		debouncer:       debouncer,
		filter:          filter,
		logger:          logger.WithField("component", "event-handler"),
		processor:       processor,
		grouper:         grouper,
		groupingEnabled: groupingEnabled,
	}
}

// HandleEvent processes an event through the debouncing pipeline
func (h *BaseHandler) HandleEvent(event *KubernetesEvent) {
	// Check if event should be processed
	if !h.filter.ShouldProcess(event) {
		h.logger.WithFields(logrus.Fields{
			"event_type": event.Type,
			"resource":   event.ResourceKind + "/" + event.ResourceName,
			"namespace":  event.ResourceNamespace,
		}).Debug("Event filtered out")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"severity":   event.Severity,
		"resource":   event.ResourceKind + "/" + event.ResourceName,
		"namespace":  event.ResourceNamespace,
		"reason":     event.Reason,
	}).Info("Received Kubernetes event")

	// Add to grouper if enabled
	if h.groupingEnabled && h.grouper != nil {
		h.grouper.AddEvent(event)
	}

	// Debounce the event
	if err := h.debouncer.Debounce(event, h.onDebounced); err != nil {
		h.logger.WithError(err).Error("Failed to debounce event")
	}
}

// onDebounced is called when the debounce window expires
func (h *BaseHandler) onDebounced(event *KubernetesEvent) {
	// Check if we should group this event
	if h.groupingEnabled && h.grouper != nil {
		relatedEvents := h.grouper.GetRelatedEvents(event)
		if len(relatedEvents) > 0 {
			event.RelatedEvents = relatedEvents
			h.logger.WithFields(logrus.Fields{
				"event_type":    event.Type,
				"resource":      event.ResourceKind + "/" + event.ResourceName,
				"related_count": len(relatedEvents),
			}).Info("Grouped related events")
		}
	}

	// Process the event
	if err := h.processor.Process(event); err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"event_type": event.Type,
			"resource":   event.ResourceKind + "/" + event.ResourceName,
			"namespace":  event.ResourceNamespace,
		}).Error("Failed to process event")
	}
}

// OnAdd handles resource creation events
func (h *BaseHandler) OnAdd(obj interface{}) {
	// Implemented by specific watchers
	h.logger.Debug("OnAdd called (should be overridden by watcher)")
}

// OnUpdate handles resource update events
func (h *BaseHandler) OnUpdate(oldObj, newObj interface{}) {
	// Implemented by specific watchers
	h.logger.Debug("OnUpdate called (should be overridden by watcher)")
}

// OnDelete handles resource deletion events
func (h *BaseHandler) OnDelete(obj interface{}) {
	// Implemented by specific watchers
	h.logger.Debug("OnDelete called (should be overridden by watcher)")
}

// EventGrouper groups related events
type EventGrouper struct {
	mu     sync.RWMutex
	events map[string][]*KubernetesEvent
	logger *logrus.Entry
}

// NewEventGrouper creates a new event grouper
func NewEventGrouper(logger *logrus.Logger) *EventGrouper {
	return &EventGrouper{
		events: make(map[string][]*KubernetesEvent),
		logger: logger.WithField("component", "event-grouper"),
	}
}

// AddEvent adds an event to the grouper
func (g *EventGrouper) AddEvent(event *KubernetesEvent) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Group by owner UID
	key := event.OwnerUID
	if key == "" {
		key = event.ResourceUID
	}

	g.events[key] = append(g.events[key], event)

	g.logger.WithFields(logrus.Fields{
		"group_key":   key,
		"event_count": len(g.events[key]),
		"event_type":  event.Type,
	}).Debug("Event added to group")
}

// GetRelatedEvents retrieves events related to the given event
func (g *EventGrouper) GetRelatedEvents(event *KubernetesEvent) []*KubernetesEvent {
	g.mu.RLock()
	defer g.mu.RUnlock()

	key := event.OwnerUID
	if key == "" {
		key = event.ResourceUID
	}

	events := g.events[key]

	// Filter out the current event
	related := make([]*KubernetesEvent, 0)
	for _, e := range events {
		if e.EventKey() != event.EventKey() {
			related = append(related, e)
		}
	}

	return related
}

// Cleanup removes old events from the grouper
func (g *EventGrouper) Cleanup(olderThan int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Remove groups with no events
	for key, events := range g.events {
		if len(events) == 0 {
			delete(g.events, key)
		}
	}

	g.logger.WithField("remaining_groups", len(g.events)).Debug("Cleaned up event groups")
}

// GroupCount returns the number of event groups
func (g *EventGrouper) GroupCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.events)
}
