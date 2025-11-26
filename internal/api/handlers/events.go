package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/ai"
	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
	"github.com/ignacio/lumo/internal/notifications"
)

// Constants for event processing limits
const (
	// MaxEventsPerRequest is the maximum number of events that can be submitted in a single request
	MaxEventsPerRequest = 100
	// MaxListLimit is the maximum number of events that can be returned in a list query
	MaxListLimit = 1000
	// DefaultListLimit is the default number of events returned in a list query
	DefaultListLimit = 100
	// MaxConcurrentEventProcessors limits concurrent async event processing goroutines
	MaxConcurrentEventProcessors = 10
)

// EventsHandler handles Kubernetes event-related requests
type EventsHandler struct {
	eventRepo    *repository.EventRepository
	agentRepo    *repository.AgentRepository
	aiProvider   ai.Provider
	notifiers    []notifications.Notifier
	logger       *logrus.Logger
	aiEnabled    bool
	notifEnabled bool
}

// NewEventsHandler creates a new events handler
func NewEventsHandler(
	eventRepo *repository.EventRepository,
	agentRepo *repository.AgentRepository,
	aiProvider ai.Provider,
	notifiers []notifications.Notifier,
	logger *logrus.Logger,
	aiEnabled bool,
	notifEnabled bool,
) *EventsHandler {
	return &EventsHandler{
		eventRepo:    eventRepo,
		agentRepo:    agentRepo,
		aiProvider:   aiProvider,
		notifiers:    notifiers,
		logger:       logger,
		aiEnabled:    aiEnabled,
		notifEnabled: notifEnabled,
	}
}

// SubmitEvents handles POST /api/v1/events
// This is the main endpoint for agents to submit Kubernetes events
func (h *EventsHandler) SubmitEvents(w http.ResponseWriter, r *http.Request) {
	// Get agent ID from context (optional - may be Nil for API key auth)
	agentID, err := h.getAgentIDFromContext(r.Context())
	if err != nil {
		// Agent ID is optional - use Nil UUID for events submitted via API key
		h.logger.Debug("No agent ID in context, using API key authentication")
		agentID = uuid.Nil
	}

	// Parse request
	var req models.SubmitEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithError(err).Warn("Failed to decode event submission request")
		response.BadRequest(w, "Invalid request body")
		return
	}

	// Validate request
	if len(req.Events) == 0 {
		response.BadRequest(w, "At least one event is required")
		return
	}

	if len(req.Events) > MaxEventsPerRequest {
		response.BadRequest(w, fmt.Sprintf("Maximum %d events per request", MaxEventsPerRequest))
		return
	}

	logFields := logrus.Fields{
		"event_count": len(req.Events),
	}
	if agentID != uuid.Nil {
		logFields["agent_id"] = agentID
	}
	h.logger.WithFields(logFields).Info("Received event submission request")

	// Convert submissions to events
	events := make([]*models.Event, 0, len(req.Events))
	var validationErrors []string

	for i, submission := range req.Events {
		// Validate submission
		if err := h.validateEventSubmission(&submission); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("Event %d: %s", i, err.Error()))
			continue
		}

		event := &models.Event{
			AgentID:          agentID,
			EventType:        submission.EventType,
			Severity:         submission.Severity,
			ResourceKind:     submission.ResourceKind,
			ResourceName:     submission.ResourceName,
			ResourceUID:      submission.ResourceUID,
			Namespace:        submission.Namespace,
			Message:          submission.Message,
			Metadata:         models.JSONB(submission.Metadata),
			EventTimestamp:   submission.EventTimestamp,
			NotificationSent: false,
		}

		events = append(events, event)
	}

	// Store events in database (batch insert)
	if len(events) > 0 {
		if err := h.eventRepo.CreateBatch(r.Context(), events); err != nil {
			h.logger.WithError(err).Error("Failed to store events")
			response.InternalServerError(w, "Failed to store events")
			return
		}

		h.logger.WithField("stored_count", len(events)).Info("Events stored successfully")
		eventsProcessedTotal.WithLabelValues("accepted").Add(float64(len(events)))

		// Process events asynchronously (AI analysis + notifications)
		// Pass request context to preserve trace information
		go h.processEventsAsync(r.Context(), events)
	}

	// Record rejected events
	if len(validationErrors) > 0 {
		eventsProcessedTotal.WithLabelValues("rejected").Add(float64(len(validationErrors)))
	}

	// Build response
	eventIDs := make([]string, 0, len(events))
	for _, event := range events {
		eventIDs = append(eventIDs, event.ID.String())
	}

	resp := models.SubmitEventResponse{
		Accepted: len(events),
		Rejected: len(validationErrors),
		EventIDs: eventIDs,
		Errors:   validationErrors,
		Message:  fmt.Sprintf("Successfully submitted %d events", len(events)),
	}

	if len(validationErrors) > 0 {
		resp.Message += fmt.Sprintf(" (%d rejected due to validation errors)", len(validationErrors))
	}

	response.Created(w, resp)
}

// GetEvent handles GET /api/v1/events/:id
func (h *EventsHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	eventID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	event, err := h.eventRepo.GetByID(r.Context(), eventID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(w, "Event not found")
			return
		}
		h.logger.WithError(err).Error("Failed to get event")
		response.InternalServerError(w, "Failed to retrieve event")
		return
	}

	response.Success(w, event)
}

// ListEvents handles GET /api/v1/events
func (h *EventsHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	filters := make(map[string]interface{})

	if agentID := r.URL.Query().Get("agent_id"); agentID != "" {
		if id, err := uuid.Parse(agentID); err == nil {
			filters["agent_id"] = id
		}
	}

	if severity := r.URL.Query().Get("severity"); severity != "" {
		filters["severity"] = models.EventSeverity(severity)
	}

	if eventType := r.URL.Query().Get("event_type"); eventType != "" {
		filters["event_type"] = eventType
	}

	if namespace := r.URL.Query().Get("namespace"); namespace != "" {
		filters["namespace"] = namespace
	}

	if resourceKind := r.URL.Query().Get("resource_kind"); resourceKind != "" {
		filters["resource_kind"] = resourceKind
	}

	if resourceUID := r.URL.Query().Get("resource_uid"); resourceUID != "" {
		filters["resource_uid"] = resourceUID
	}

	// Time range filtering
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filters["since"] = t
		}
	}

	if until := r.URL.Query().Get("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			filters["until"] = t
		}
	}

	// Notification status
	if notificationSent := r.URL.Query().Get("notification_sent"); notificationSent != "" {
		if sent, err := strconv.ParseBool(notificationSent); err == nil {
			filters["notification_sent"] = sent
		}
	}

	// AI analysis status
	if hasAI := r.URL.Query().Get("has_ai_analysis"); hasAI != "" {
		if has, err := strconv.ParseBool(hasAI); err == nil {
			filters["has_ai_analysis"] = has
		}
	}

	// Pagination
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			if l > MaxListLimit {
				l = MaxListLimit
			}
			filters["limit"] = l
		}
	} else {
		filters["limit"] = DefaultListLimit
	}

	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filters["offset"] = o
		}
	}

	// Sorting
	if sort := r.URL.Query().Get("sort"); sort != "" {
		filters["sort"] = sort
	}

	events, err := h.eventRepo.List(r.Context(), filters)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list events")
		response.InternalServerError(w, "Failed to list events")
		return
	}

	response.Success(w, map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

// validateEventSubmission validates an event submission
func (h *EventsHandler) validateEventSubmission(submission *models.EventSubmission) error {
	if submission.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	if submission.Severity == "" {
		return fmt.Errorf("severity is required")
	}
	if submission.ResourceKind == "" {
		return fmt.Errorf("resource_kind is required")
	}
	if submission.ResourceName == "" {
		return fmt.Errorf("resource_name is required")
	}
	if submission.Message == "" {
		return fmt.Errorf("message is required")
	}
	if submission.EventTimestamp.IsZero() {
		return fmt.Errorf("event_timestamp is required")
	}

	// Validate severity
	validSeverities := map[models.EventSeverity]bool{
		models.EventSeverityLow:      true,
		models.EventSeverityMedium:   true,
		models.EventSeverityHigh:     true,
		models.EventSeverityCritical: true,
	}
	if !validSeverities[submission.Severity] {
		return fmt.Errorf("invalid severity: must be low, medium, high, or critical")
	}

	return nil
}

// getAgentIDFromContext extracts agent ID from request context
func (h *EventsHandler) getAgentIDFromContext(ctx context.Context) (uuid.UUID, error) {
	// This would typically be set by authentication middleware
	// For now, we'll look for it in the context value
	if agentID, ok := ctx.Value("agent_id").(uuid.UUID); ok {
		return agentID, nil
	}

	// Alternative: look for it as a string
	if agentIDStr, ok := ctx.Value("agent_id").(string); ok {
		return uuid.Parse(agentIDStr)
	}

	return uuid.Nil, fmt.Errorf("agent_id not found in context")
}

// processEventsAsync performs AI analysis and sends notifications asynchronously.
// Uses a 5-minute timeout for all async operations and limits concurrent goroutines
// via a semaphore to prevent resource exhaustion.
func (h *EventsHandler) processEventsAsync(parentCtx context.Context, events []*models.Event) {
	// Create detached context to allow async processing to complete independently of HTTP request
	// while preserving trace context. Use WithoutCancel to prevent parent cancellation from stopping
	// async processing, then add 5-minute timeout.
	detachedCtx := context.WithoutCancel(parentCtx)
	ctx, cancel := context.WithTimeout(detachedCtx, 5*time.Minute)
	defer cancel()

	// Use semaphore to limit concurrent goroutines
	semaphore := make(chan struct{}, MaxConcurrentEventProcessors)
	var wg sync.WaitGroup

	for _, event := range events {
		// Skip low priority events for async processing
		if !event.IsHighPriority() {
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(evt *models.Event) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			h.processSingleEvent(ctx, evt)
		}(event)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

// processSingleEvent handles AI analysis and notifications for a single event
func (h *EventsHandler) processSingleEvent(ctx context.Context, event *models.Event) {
	// AI Analysis
	if h.aiEnabled && h.aiProvider != nil {
		if err := h.analyzeEventWithAI(ctx, event); err != nil {
			h.logger.WithError(err).WithField("event_id", event.ID).Error("AI analysis failed")
			asyncProcessingErrors.WithLabelValues("ai_analysis").Inc()
		}
	}

	// Notifications
	if h.notifEnabled && len(h.notifiers) > 0 {
		if err := h.sendEventNotifications(ctx, event); err != nil {
			h.logger.WithError(err).WithField("event_id", event.ID).Error("Notification failed")
			asyncProcessingErrors.WithLabelValues("notification").Inc()
		}
	}
}

// analyzeEventWithAI performs AI analysis on an event
func (h *EventsHandler) analyzeEventWithAI(ctx context.Context, event *models.Event) error {
	h.logger.WithField("event_id", event.ID).Info("Starting AI analysis")
	startTime := time.Now()

	// Build prompt for AI analysis
	prompt := fmt.Sprintf(`A Kubernetes event has been detected:

Event Type: %s
Severity: %s
Resource: %s/%s
Namespace: %s
Message: %s

Please analyze this event and provide:
1. Root cause analysis
2. Potential impact assessment
3. Recommended remediation steps
4. Prevention strategies

Keep the response concise and actionable.`,
		event.EventType,
		event.Severity,
		event.ResourceKind,
		event.ResourceName,
		stringOrEmpty(event.Namespace),
		event.Message,
	)

	// Call AI provider
	analysisCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	response, _, err := h.aiProvider.Ask(analysisCtx, "You are a Kubernetes SRE expert. Analyze the following event and provide actionable insights.", prompt)
	if err != nil {
		aiAnalysisTotal.WithLabelValues("failure").Inc()
		return fmt.Errorf("AI provider analysis failed: %w", err)
	}

	// Record metrics
	aiAnalysisDuration.WithLabelValues(h.aiProvider.Name()).Observe(time.Since(startTime).Seconds())
	aiAnalysisTotal.WithLabelValues("success").Inc()

	// Store AI analysis
	if err := h.eventRepo.UpdateAIAnalysis(ctx, event.ID, response); err != nil {
		return fmt.Errorf("failed to store AI analysis: %w", err)
	}

	h.logger.WithField("event_id", event.ID).Info("AI analysis completed and stored")
	return nil
}

// sendEventNotifications sends notifications for an event
func (h *EventsHandler) sendEventNotifications(ctx context.Context, event *models.Event) error {
	h.logger.WithField("event_id", event.ID).Info("Sending event notifications")

	// Build notification message
	message := h.buildNotificationMessage(event)

	sentChannels := []string{}
	failedChannels := []string{}
	var lastErr error

	// Send to all configured notifiers
	for _, notifier := range h.notifiers {
		if err := notifier.Send(ctx, &message); err != nil {
			h.logger.WithError(err).WithField("provider", notifier.Name()).Error("Failed to send notification")
			notificationsSentTotal.WithLabelValues(notifier.Name(), "failure").Inc()
			failedChannels = append(failedChannels, notifier.Name())
			lastErr = err
		} else {
			sentChannels = append(sentChannels, notifier.Name())
			notificationsSentTotal.WithLabelValues(notifier.Name(), "success").Inc()
			h.logger.WithField("provider", notifier.Name()).Info("Notification sent successfully")
		}
	}

	// Log partial failures for visibility
	if len(failedChannels) > 0 && len(sentChannels) > 0 {
		h.logger.WithFields(logrus.Fields{
			"event_id":        event.ID,
			"sent_channels":   sentChannels,
			"failed_channels": failedChannels,
		}).Warn("Partial notification failure - some channels succeeded")
	}

	// Update notification status
	if len(sentChannels) > 0 {
		if err := h.eventRepo.UpdateNotificationStatus(ctx, event.ID, true, sentChannels); err != nil {
			h.logger.WithError(err).Error("Failed to update notification status")
		}
	}

	if lastErr != nil && len(sentChannels) == 0 {
		return fmt.Errorf("all notification attempts failed: %w", lastErr)
	}

	return nil
}

// buildNotificationMessage builds a notification message for an event
func (h *EventsHandler) buildNotificationMessage(event *models.Event) notifications.Notification {
	severityEmoji := map[models.EventSeverity]string{
		models.EventSeverityCritical: "🔴",
		models.EventSeverityHigh:     "🟠",
		models.EventSeverityMedium:   "🟡",
		models.EventSeverityLow:      "🔵",
	}

	emoji := severityEmoji[event.Severity]
	title := fmt.Sprintf("%s Kubernetes Event: %s", emoji, event.EventType)

	body := fmt.Sprintf(`*Severity:* %s
*Resource:* %s/%s
*Namespace:* %s
*Time:* %s

*Message:*
%s`,
		event.Severity,
		event.ResourceKind,
		event.ResourceName,
		stringOrEmpty(event.Namespace),
		event.EventTimestamp.Format(time.RFC3339),
		event.Message,
	)

	// Add AI analysis if available
	if event.HasAIAnalysis() {
		body += fmt.Sprintf("\n\n*AI Analysis:*\n%s", *event.AIAnalysis)
	}

	// Map event severity to notification level
	level := notifications.LevelInfo
	switch event.Severity {
	case models.EventSeverityCritical:
		level = notifications.LevelCritical
	case models.EventSeverityHigh:
		level = notifications.LevelError
	case models.EventSeverityMedium:
		level = notifications.LevelWarning
	case models.EventSeverityLow:
		level = notifications.LevelInfo
	}

	return notifications.Notification{
		Title:     title,
		Message:   body,
		Level:     level,
		Timestamp: event.EventTimestamp,
		Tags:      []string{"kubernetes", "event", string(event.Severity), event.EventType},
	}
}

// stringOrEmpty returns the dereferenced string or "N/A" if nil
func stringOrEmpty(s *string) string {
	if s == nil {
		return "N/A"
	}
	return *s
}
