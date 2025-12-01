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
	"github.com/ignacio/lumo/internal/api/middleware"
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
	// Parse request first to get agent_id if provided
	var req models.SubmitEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithError(err).Warn("Failed to decode event submission request")
		response.BadRequest(w, "Invalid request body")
		return
	}

	// Get agent ID - priority: request body > context (API key metadata)
	var agentID uuid.UUID
	var err error
	if req.AgentID != "" {
		agentID, err = uuid.Parse(req.AgentID)
		if err != nil {
			h.logger.WithError(err).Warn("Invalid agent_id in request")
			response.BadRequest(w, "Invalid agent_id format")
			return
		}
	} else {
		// Try to get from context (API key metadata)
		agentID, err = h.getAgentIDFromContext(r.Context())
		if err != nil {
			h.logger.Debug("No agent ID in request or context")
			agentID = uuid.Nil
		}
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

	// Get tenant ID from context
	tenantID := middleware.GetTenantID(r.Context())

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
			TenantID:         tenantID,
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

	// Tenant filter from context
	tenantID := middleware.GetTenantID(r.Context())
	filters["tenant_id"] = tenantID

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

	// Try to extract from API key metadata (set during agent API key creation)
	if apiKey, ok := middleware.GetAPIKeyFromContext(ctx); ok && apiKey != nil {
		if apiKey.Metadata != nil {
			if agentIDStr, ok := apiKey.Metadata["agent_id"].(string); ok && agentIDStr != "" {
				return uuid.Parse(agentIDStr)
			}
		}
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

	// Extract metadata for context (JSONB is already map[string]interface{})
	metadata := make(map[string]interface{})
	if event.Metadata != nil {
		metadata = event.Metadata
	}

	// Build enhanced prompt for AI analysis with structured output
	prompt := h.buildAIAnalysisPrompt(event, metadata)

	// Call AI provider
	analysisCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	systemPrompt := `You are a Kubernetes SRE expert with deep knowledge of container orchestration, cloud-native architecture, and incident response.

Your analysis should be:
- Clear and concise (3-5 sentences per section)
- Actionable with specific kubectl commands or configuration changes
- Prioritized by impact and urgency
- Formatted in Markdown with proper sections

Always include specific commands, file paths, or configuration snippets when suggesting remediation.`

	response, _, err := h.aiProvider.Ask(analysisCtx, systemPrompt, prompt)
	if err != nil {
		aiAnalysisTotal.WithLabelValues("failure").Inc()
		return fmt.Errorf("AI provider analysis failed: %w", err)
	}

	// Record metrics
	aiAnalysisDuration.WithLabelValues(h.aiProvider.Name()).Observe(time.Since(startTime).Seconds())
	aiAnalysisTotal.WithLabelValues("success").Inc()

	// Store AI analysis in database
	if err := h.eventRepo.UpdateAIAnalysis(ctx, event.ID, response); err != nil {
		return fmt.Errorf("failed to store AI analysis: %w", err)
	}

	// Update the in-memory event object so notification includes AI analysis
	event.AIAnalysis = &response

	h.logger.WithField("event_id", event.ID).Info("AI analysis completed and stored")
	return nil
}

// buildAIAnalysisPrompt creates a comprehensive, structured prompt for AI analysis
func (h *EventsHandler) buildAIAnalysisPrompt(event *models.Event, metadata map[string]interface{}) string {
	var prompt strings.Builder

	prompt.WriteString("# Kubernetes Event Analysis Request\n\n")

	// Event Overview
	prompt.WriteString("## Event Details\n")
	prompt.WriteString(fmt.Sprintf("- **Event Type**: `%s`\n", event.EventType))
	prompt.WriteString(fmt.Sprintf("- **Severity**: `%s`\n", event.Severity))
	prompt.WriteString(fmt.Sprintf("- **Resource**: `%s/%s`\n", event.ResourceKind, event.ResourceName))
	prompt.WriteString(fmt.Sprintf("- **Namespace**: `%s`\n", stringOrEmpty(event.Namespace)))
	prompt.WriteString(fmt.Sprintf("- **Timestamp**: `%s`\n", event.EventTimestamp.Format(time.RFC3339)))
	prompt.WriteString(fmt.Sprintf("- **Message**: %s\n\n", event.Message))

	// Additional metadata if available
	if len(metadata) > 0 {
		prompt.WriteString("## Additional Context\n")
		if labels, ok := metadata["labels"].(map[string]interface{}); ok && len(labels) > 0 {
			prompt.WriteString("**Labels**:\n")
			for k, v := range labels {
				prompt.WriteString(fmt.Sprintf("- `%s`: `%v`\n", k, v))
			}
		}

		// Add specific context based on event type
		h.addEventSpecificContext(&prompt, event, metadata)
		prompt.WriteString("\n")
	}

	// Analysis request
	prompt.WriteString("## Required Analysis\n\n")
	prompt.WriteString("Provide a structured analysis with the following sections:\n\n")

	prompt.WriteString("### 1. Root Cause\n")
	prompt.WriteString("Identify the most likely root cause(s) of this event. ")
	prompt.WriteString("Be specific about what failed and why.\n\n")

	prompt.WriteString("### 2. Impact Assessment\n")
	prompt.WriteString("Evaluate the impact on:\n")
	prompt.WriteString("- User-facing services\n")
	prompt.WriteString("- System resources\n")
	prompt.WriteString("- Related workloads\n\n")

	prompt.WriteString("### 3. Immediate Actions\n")
	prompt.WriteString("Provide 3-5 immediate remediation steps with specific commands. Example:\n")
	prompt.WriteString("```bash\n")
	prompt.WriteString("kubectl describe pod <pod-name> -n <namespace>\n")
	prompt.WriteString("```\n\n")

	prompt.WriteString("### 4. Long-term Prevention\n")
	prompt.WriteString("Suggest configuration changes, resource adjustments, or architectural improvements ")
	prompt.WriteString("to prevent recurrence. Include YAML snippets if relevant.\n\n")

	prompt.WriteString("### 5. Monitoring Recommendations\n")
	prompt.WriteString("Suggest specific metrics, alerts, or logs to monitor for early detection.\n\n")

	return prompt.String()
}

// addEventSpecificContext adds context specific to the event type
func (h *EventsHandler) addEventSpecificContext(prompt *strings.Builder, event *models.Event, metadata map[string]interface{}) {
	switch event.EventType {
	case "oom-killed":
		if limits, ok := metadata["container_limits"].(map[string]interface{}); ok {
			prompt.WriteString("\n**Container Resource Limits**:\n")
			if memory, ok := limits["memory"].(string); ok {
				fmt.Fprintf(prompt, "- Memory Limit: `%s`\n", memory)
			}
		}
		if usage, ok := metadata["last_memory_usage"].(string); ok {
			fmt.Fprintf(prompt, "- Last Memory Usage: `%s`\n", usage)
		}

	case "image-pull-backoff":
		if image, ok := metadata["image"].(string); ok {
			fmt.Fprintf(prompt, "\n**Image**: `%s`\n", image)
		}
		if reason, ok := metadata["reason"].(string); ok {
			fmt.Fprintf(prompt, "**Pull Error**: %s\n", reason)
		}

	case "crash-loop-backoff":
		if restarts, ok := metadata["restart_count"].(float64); ok {
			fmt.Fprintf(prompt, "\n**Restart Count**: `%.0f`\n", restarts)
		}
		if exitCode, ok := metadata["exit_code"].(float64); ok {
			fmt.Fprintf(prompt, "**Last Exit Code**: `%.0f`\n", exitCode)
		}

	case "pvc-provision-failed":
		if storageClass, ok := metadata["storage_class"].(string); ok {
			fmt.Fprintf(prompt, "\n**Storage Class**: `%s`\n", storageClass)
		}
		if requestedSize, ok := metadata["requested_size"].(string); ok {
			fmt.Fprintf(prompt, "**Requested Size**: `%s`\n", requestedSize)
		}
	}
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
	title := fmt.Sprintf("%s K8s Alert: %s", emoji, formatEventTypeName(event.EventType))

	// Build structured message body
	var body strings.Builder

	// Resource information section
	body.WriteString(fmt.Sprintf("*Resource:* `%s/%s`\n", event.ResourceKind, event.ResourceName))
	if event.Namespace != nil && *event.Namespace != "" {
		body.WriteString(fmt.Sprintf("*Namespace:* `%s`\n", *event.Namespace))
	}

	// Event message
	body.WriteString(fmt.Sprintf("\n*Event Details:*\n%s\n", event.Message))

	// Add metadata context if available (JSONB is already map[string]interface{})
	if event.Metadata != nil {
		contextInfo := h.extractContextInfo(event.Metadata)
		if contextInfo != "" {
			body.WriteString(fmt.Sprintf("\n*Context:*\n%s\n", contextInfo))
		}
	}

	// Add AI analysis if available
	if event.HasAIAnalysis() {
		body.WriteString(fmt.Sprintf("\n*AI Analysis:*\n%s", *event.AIAnalysis))
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

	// Build tags
	tags := []string{"kubernetes", string(event.Severity), event.EventType}
	if event.Namespace != nil {
		tags = append(tags, fmt.Sprintf("ns:%s", *event.Namespace))
	}

	// Add fields for structured display (used by Slack Block Kit)
	fields := make(map[string]string)
	if event.ResourceUID != nil {
		fields["Resource UID"] = *event.ResourceUID
	}
	fields["Event ID"] = event.ID.String()
	// Add analysis URL for "View Full Analysis" links
	fields["_event_url"] = fmt.Sprintf("/api/v1/events/%s/analysis", event.ID.String())

	return notifications.Notification{
		Title:     title,
		Message:   body.String(),
		Level:     level,
		Timestamp: event.EventTimestamp,
		Tags:      tags,
		Fields:    fields,
	}
}

// formatEventTypeName converts event type to human-readable format
func formatEventTypeName(eventType string) string {
	// Convert kebab-case to Title Case
	words := strings.Split(eventType, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// extractContextInfo extracts relevant context information from metadata
func (h *EventsHandler) extractContextInfo(metadata map[string]interface{}) string {
	var context strings.Builder

	// Extract restart count for pod events
	if restarts, ok := metadata["restart_count"].(float64); ok && restarts > 0 {
		context.WriteString(fmt.Sprintf("• Restart count: `%.0f`\n", restarts))
	}

	// Extract exit code for crash events
	if exitCode, ok := metadata["exit_code"].(float64); ok {
		context.WriteString(fmt.Sprintf("• Exit code: `%.0f`\n", exitCode))
	}

	// Extract image information
	if image, ok := metadata["image"].(string); ok {
		context.WriteString(fmt.Sprintf("• Image: `%s`\n", image))
	}

	// Extract container limits for resource events
	if limits, ok := metadata["container_limits"].(map[string]interface{}); ok {
		if memory, ok := limits["memory"].(string); ok {
			context.WriteString(fmt.Sprintf("• Memory limit: `%s`\n", memory))
		}
		if cpu, ok := limits["cpu"].(string); ok {
			context.WriteString(fmt.Sprintf("• CPU limit: `%s`\n", cpu))
		}
	}

	// Extract storage information for PVC events
	if storageClass, ok := metadata["storage_class"].(string); ok {
		context.WriteString(fmt.Sprintf("• Storage class: `%s`\n", storageClass))
	}
	if requestedSize, ok := metadata["requested_size"].(string); ok {
		context.WriteString(fmt.Sprintf("• Requested size: `%s`\n", requestedSize))
	}

	// Extract node information
	if nodeName, ok := metadata["node_name"].(string); ok {
		context.WriteString(fmt.Sprintf("• Node: `%s`\n", nodeName))
	}

	return context.String()
}

// stringOrEmpty returns the dereferenced string or "N/A" if nil
func stringOrEmpty(s *string) string {
	if s == nil {
		return "N/A"
	}
	return *s
}
