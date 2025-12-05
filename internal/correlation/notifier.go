package correlation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/notifications"
)

// IncidentNotifierImpl implements IncidentNotifier and RealtimeIncidentNotifier
type IncidentNotifierImpl struct {
	notifiers  []notifications.Notifier
	logger     *logrus.Entry
	apiBaseURL string // Base URL for incident detail links
}

// NewIncidentNotifier creates a new incident notifier
func NewIncidentNotifier(
	notifiers []notifications.Notifier,
	apiBaseURL string,
	logger *logrus.Logger,
) *IncidentNotifierImpl {
	return &IncidentNotifierImpl{
		notifiers:  notifiers,
		logger:     logger.WithField("component", "incident-notifier"),
		apiBaseURL: apiBaseURL,
	}
}

// NotifyIncident sends notifications about an incident (legacy method)
func (n *IncidentNotifierImpl) NotifyIncident(ctx context.Context, incident *Incident) error {
	n.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"title":       incident.Title,
		"category":    incident.Category,
		"severity":    incident.Severity,
	}).Info("Sending incident notification")

	// Build the notification message
	notification := n.buildNotification(incident)

	var lastErr error
	sentChannels := make([]string, 0)

	// Send to all configured notifiers
	for _, notifier := range n.notifiers {
		if err := notifier.Send(ctx, &notification); err != nil {
			n.logger.WithError(err).WithField("provider", notifier.Name()).Error("Failed to send notification")
			lastErr = err
		} else {
			sentChannels = append(sentChannels, notifier.Name())
			n.logger.WithField("provider", notifier.Name()).Debug("Notification sent successfully")
		}
	}

	// Update incident with notification status
	incident.NotificationChannels = sentChannels
	now := time.Now()
	incident.NotifiedAt = &now

	if len(sentChannels) == 0 && lastErr != nil {
		return fmt.Errorf("all notification attempts failed: %w", lastErr)
	}

	n.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"channels":    sentChannels,
	}).Info("Incident notification sent")

	return nil
}

// NotifyIncidentCreated sends "Lumo is on it" notification for critical incidents
func (n *IncidentNotifierImpl) NotifyIncidentCreated(ctx context.Context, incident *Incident) error {
	n.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"title":       incident.Title,
		"is_critical": incident.IsCritical,
	}).Info("Sending incident created notification")

	notification := n.buildCreatedNotification(incident)
	return n.sendToAll(ctx, &notification)
}

// NotifyAnalysisUpdate sends incremental analysis update
func (n *IncidentNotifierImpl) NotifyAnalysisUpdate(ctx context.Context, incident *Incident, entry *AnalysisLogEntry) error {
	n.logger.WithFields(logrus.Fields{
		"incident_id":   incident.ID,
		"analysis_type": entry.AnalysisType,
	}).Info("Sending analysis update notification")

	notification := n.buildUpdateNotification(incident, entry)
	return n.sendToAll(ctx, &notification)
}

// NotifyIncidentResolved sends final notification with postmortem
func (n *IncidentNotifierImpl) NotifyIncidentResolved(ctx context.Context, incident *Incident) error {
	n.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"title":       incident.Title,
	}).Info("Sending incident resolved notification")

	notification := n.buildResolvedNotification(incident)
	return n.sendToAll(ctx, &notification)
}

// NotifyProgress sends "still working on it" notification
func (n *IncidentNotifierImpl) NotifyProgress(ctx context.Context, incident *Incident) error {
	n.logger.WithFields(logrus.Fields{
		"incident_id":        incident.ID,
		"notification_count": incident.NotificationCount,
	}).Debug("Sending progress notification")

	notification := n.buildProgressNotification(incident)
	return n.sendToAll(ctx, &notification)
}

// sendToAll sends a notification to all configured notifiers
func (n *IncidentNotifierImpl) sendToAll(ctx context.Context, notification *notifications.Notification) error {
	var lastErr error
	sentCount := 0

	for _, notifier := range n.notifiers {
		if err := notifier.Send(ctx, notification); err != nil {
			n.logger.WithError(err).WithField("provider", notifier.Name()).Error("Failed to send notification")
			lastErr = err
		} else {
			sentCount++
			n.logger.WithField("provider", notifier.Name()).Debug("Notification sent successfully")
		}
	}

	if sentCount == 0 && lastErr != nil {
		return fmt.Errorf("all notification attempts failed: %w", lastErr)
	}

	return nil
}

// buildCreatedNotification builds the "Lumo is on it" notification
func (n *IncidentNotifierImpl) buildCreatedNotification(incident *Incident) notifications.Notification {
	emoji := "🚨"
	if !incident.IsCritical {
		emoji = "⚠️"
	}

	title := fmt.Sprintf("%s ALERT: %s", emoji, incident.Title)

	var body strings.Builder
	body.WriteString("🤖 *Lumo has detected a critical incident and is actively analyzing it.*\n\n")

	body.WriteString(fmt.Sprintf("*Incident ID:* `%s`\n", incident.ID.String()[:8]))
	body.WriteString(fmt.Sprintf("*Category:* %s\n", formatCategory(incident.Category)))
	body.WriteString(fmt.Sprintf("*Severity:* %s\n", strings.ToUpper(string(incident.Severity))))

	if incident.Namespace != nil && *incident.Namespace != "" {
		body.WriteString(fmt.Sprintf("*Namespace:* `%s`\n", *incident.Namespace))
	}

	body.WriteString("\n")

	// Show first event
	if len(incident.Events) > 0 {
		event := incident.Events[0]
		body.WriteString("*Initial Event:*\n")
		body.WriteString(fmt.Sprintf("• Type: `%s`\n", event.EventType))
		body.WriteString(fmt.Sprintf("• Resource: `%s/%s`\n", event.ResourceKind, event.ResourceName))
		body.WriteString(fmt.Sprintf("• Message: %s\n", truncateStr(event.Message, 200)))
	}

	body.WriteString("\n_Lumo is gathering logs, metrics, and context. Updates will follow._")

	// Build fields
	fields := make(map[string]string)
	fields["Incident ID"] = incident.ID.String()
	fields["incident_id"] = incident.ID.String() // Key for Slack incident notification detection
	fields["event_id"] = incident.ID.String()    // Enable dynamic updates in Slack
	fields["Status"] = "🔄 Analyzing"
	if n.apiBaseURL != "" {
		fields["_incident_url"] = fmt.Sprintf("%s/api/v1/incidents/%s", n.apiBaseURL, incident.ID.String())
	}

	return notifications.Notification{
		Title:     title,
		Message:   body.String(),
		Level:     notifications.LevelCritical,
		Timestamp: incident.OpenedAt,
		Tags:      []string{"kubernetes", "incident", "critical", string(incident.Category)},
		Fields:    fields,
	}
}

// buildUpdateNotification builds an incremental analysis update notification
func (n *IncidentNotifierImpl) buildUpdateNotification(incident *Incident, entry *AnalysisLogEntry) notifications.Notification {
	var emoji, typeLabel string
	level := notifications.LevelInfo

	switch entry.AnalysisType {
	case AnalysisTypeInsight:
		emoji = "💡"
		typeLabel = "New Insight"
		level = notifications.LevelWarning
	case AnalysisTypeRootCause:
		emoji = "🎯"
		typeLabel = "Root Cause Identified"
		level = notifications.LevelError
	default:
		emoji = "📊"
		typeLabel = "Update"
	}

	title := fmt.Sprintf("%s Incident Update: %s", emoji, typeLabel)

	var body strings.Builder
	body.WriteString(fmt.Sprintf("*Incident:* %s\n", incident.Title))
	body.WriteString(fmt.Sprintf("*Incident ID:* `%s`\n\n", incident.ID.String()[:8]))

	body.WriteString(fmt.Sprintf("*%s:*\n", typeLabel))
	body.WriteString(entry.Content)
	body.WriteString("\n\n")

	body.WriteString(fmt.Sprintf("_Events collected: %d | Duration: %s_",
		len(incident.Events), formatDuration(incident.Duration())))

	fields := make(map[string]string)
	fields["Incident ID"] = incident.ID.String()
	fields["incident_id"] = incident.ID.String() // Key for Slack incident notification detection
	fields["event_id"] = incident.ID.String()
	fields["Update Type"] = typeLabel
	if n.apiBaseURL != "" {
		fields["_incident_url"] = fmt.Sprintf("%s/api/v1/incidents/%s", n.apiBaseURL, incident.ID.String())
	}

	return notifications.Notification{
		Title:     title,
		Message:   body.String(),
		Level:     level,
		Timestamp: entry.CreatedAt,
		Tags:      []string{"kubernetes", "incident", "update", entry.AnalysisType},
		Fields:    fields,
	}
}

// buildResolvedNotification builds the incident resolved notification with postmortem
func (n *IncidentNotifierImpl) buildResolvedNotification(incident *Incident) notifications.Notification {
	title := fmt.Sprintf("✅ Incident Resolved: %s", incident.Title)

	var body strings.Builder
	body.WriteString("*Incident has been resolved.*\n\n")

	body.WriteString(fmt.Sprintf("*Incident ID:* `%s`\n", incident.ID.String()[:8]))
	body.WriteString(fmt.Sprintf("*Category:* %s\n", formatCategory(incident.Category)))
	body.WriteString(fmt.Sprintf("*Duration:* %s\n", formatDuration(incident.Duration())))
	body.WriteString(fmt.Sprintf("*Total Events:* %d\n", len(incident.Events)))
	body.WriteString(fmt.Sprintf("*Affected Resources:* %d\n\n", len(incident.AffectedResources)))

	// Root cause summary
	if incident.RootCause != "" {
		body.WriteString("*🔍 Root Cause:*\n")
		body.WriteString(truncateStr(incident.RootCause, 500))
		body.WriteString("\n\n")
	}

	// Key actions from AI analysis
	if incident.AIAnalysis != nil && len(incident.AIAnalysis.ImmediateActions) > 0 {
		body.WriteString("*🛠 Key Actions Taken/Recommended:*\n")
		maxActions := 3
		if len(incident.AIAnalysis.ImmediateActions) < maxActions {
			maxActions = len(incident.AIAnalysis.ImmediateActions)
		}
		for i := 0; i < maxActions; i++ {
			action := incident.AIAnalysis.ImmediateActions[i]
			body.WriteString(fmt.Sprintf("• %s\n", action.Title))
		}
		body.WriteString("\n")
	}

	body.WriteString("_Full postmortem follows in thread._")

	fields := make(map[string]string)
	fields["Incident ID"] = incident.ID.String()
	fields["incident_id"] = incident.ID.String() // Key for Slack incident notification detection
	fields["event_id"] = incident.ID.String()
	fields["Status"] = "✅ Resolved"
	fields["Duration"] = formatDuration(incident.Duration())
	if n.apiBaseURL != "" {
		fields["_incident_url"] = fmt.Sprintf("%s/api/v1/incidents/%s/analysis", n.apiBaseURL, incident.ID.String())
	}

	return notifications.Notification{
		Title:      title,
		Message:    body.String(),
		Level:      notifications.LevelInfo,
		Timestamp:  time.Now(),
		Tags:       []string{"kubernetes", "incident", "resolved", string(incident.Category)},
		Fields:     fields,
		Postmortem: incident.Postmortem, // Include full postmortem for threaded delivery
	}
}

// buildProgressNotification builds a "still working on it" notification
func (n *IncidentNotifierImpl) buildProgressNotification(incident *Incident) notifications.Notification {
	title := fmt.Sprintf("⏳ Incident Update: %s", incident.Title)

	var body strings.Builder
	body.WriteString("🤖 *Lumo is still analyzing this incident.*\n\n")

	body.WriteString(fmt.Sprintf("*Incident ID:* `%s`\n", incident.ID.String()[:8]))
	body.WriteString(fmt.Sprintf("*Duration so far:* %s\n", formatDuration(time.Since(incident.OpenedAt))))
	body.WriteString(fmt.Sprintf("*Events collected:* %d\n", len(incident.Events)))
	body.WriteString(fmt.Sprintf("*Update #:* %d\n\n", incident.NotificationCount+1))

	// Show current status
	if incident.RootCause != "" {
		body.WriteString("*Current hypothesis:*\n")
		body.WriteString(truncateStr(incident.RootCause, 200))
		body.WriteString("\n\n")
	} else {
		body.WriteString("_Still gathering information to determine root cause..._\n\n")
	}

	// Show recent events
	if len(incident.Events) > 0 {
		body.WriteString("*Recent activity:*\n")
		recentCount := 3
		if len(incident.Events) < recentCount {
			recentCount = len(incident.Events)
		}
		for i := len(incident.Events) - recentCount; i < len(incident.Events); i++ {
			event := incident.Events[i]
			body.WriteString(fmt.Sprintf("• `%s` %s on %s/%s\n",
				event.EventTimestamp.Format("15:04:05"),
				event.EventType,
				event.ResourceKind,
				event.ResourceName))
		}
	}

	fields := make(map[string]string)
	fields["Incident ID"] = incident.ID.String()
	fields["incident_id"] = incident.ID.String() // Key for Slack incident notification detection
	fields["event_id"] = incident.ID.String()
	fields["Status"] = "🔄 Analyzing"
	if n.apiBaseURL != "" {
		fields["_incident_url"] = fmt.Sprintf("%s/api/v1/incidents/%s", n.apiBaseURL, incident.ID.String())
	}

	return notifications.Notification{
		Title:     title,
		Message:   body.String(),
		Level:     notifications.LevelWarning,
		Timestamp: time.Now(),
		Tags:      []string{"kubernetes", "incident", "progress"},
		Fields:    fields,
	}
}

// buildNotification creates a notification from an incident (legacy)
func (n *IncidentNotifierImpl) buildNotification(incident *Incident) notifications.Notification {
	// Severity emoji
	severityEmoji := map[string]string{
		"critical": "🔴",
		"high":     "🟠",
		"medium":   "🟡",
		"low":      "🔵",
	}
	emoji := severityEmoji[string(incident.Severity)]
	if emoji == "" {
		emoji = "⚪"
	}

	// Build title
	title := fmt.Sprintf("%s Incident: %s", emoji, incident.Title)

	// Build message body
	var body strings.Builder

	// Incident summary section
	body.WriteString(fmt.Sprintf("*Incident ID:* `%s`\n", incident.ID.String()[:8]))
	body.WriteString(fmt.Sprintf("*Category:* %s\n", formatCategory(incident.Category)))
	body.WriteString(fmt.Sprintf("*Severity:* %s\n", strings.ToUpper(string(incident.Severity))))
	body.WriteString(fmt.Sprintf("*Duration:* %s\n", formatDuration(incident.Duration())))
	body.WriteString(fmt.Sprintf("*Events:* %d correlated events\n", len(incident.Events)))
	body.WriteString(fmt.Sprintf("*Resources Affected:* %d\n", len(incident.AffectedResources)))

	if incident.Namespace != nil && *incident.Namespace != "" {
		body.WriteString(fmt.Sprintf("*Namespace:* `%s`\n", *incident.Namespace))
	}

	body.WriteString("\n")

	// Root cause section
	if incident.RootCause != "" {
		body.WriteString("*🔍 Root Cause:*\n")
		body.WriteString(incident.RootCause)
		body.WriteString("\n\n")
	}

	// Timeline highlights (top 5 events)
	body.WriteString("*📅 Timeline:*\n")
	maxEvents := 5
	if len(incident.Timeline) < maxEvents {
		maxEvents = len(incident.Timeline)
	}
	for i := 0; i < maxEvents; i++ {
		entry := incident.Timeline[i]
		body.WriteString(fmt.Sprintf("• `%s` %s on %s\n",
			entry.Timestamp.Format("15:04:05"),
			entry.EventType,
			entry.Resource))
	}
	if len(incident.Timeline) > 5 {
		body.WriteString(fmt.Sprintf("_... and %d more events_\n", len(incident.Timeline)-5))
	}
	body.WriteString("\n")

	// Immediate actions (if available)
	if incident.AIAnalysis != nil && len(incident.AIAnalysis.ImmediateActions) > 0 {
		body.WriteString("*🛠 Immediate Actions:*\n")
		maxActions := 3
		if len(incident.AIAnalysis.ImmediateActions) < maxActions {
			maxActions = len(incident.AIAnalysis.ImmediateActions)
		}
		for i := 0; i < maxActions; i++ {
			action := incident.AIAnalysis.ImmediateActions[i]
			body.WriteString(fmt.Sprintf("%d. %s\n", i+1, action.Title))
			if action.Command != "" {
				body.WriteString(fmt.Sprintf("   `%s`\n", action.Command))
			}
		}
		body.WriteString("\n")
	}

	// Determine notification level
	level := notifications.LevelInfo
	switch incident.Severity {
	case "critical":
		level = notifications.LevelCritical
	case "high":
		level = notifications.LevelError
	case "medium":
		level = notifications.LevelWarning
	case "low":
		level = notifications.LevelInfo
	}

	// Build tags
	tags := []string{
		"kubernetes",
		"incident",
		string(incident.Category),
		string(incident.Severity),
	}
	if incident.Namespace != nil && *incident.Namespace != "" {
		tags = append(tags, fmt.Sprintf("ns:%s", *incident.Namespace))
	}

	// Build fields for structured display
	fields := make(map[string]string)
	fields["Incident ID"] = incident.ID.String()
	fields["incident_id"] = incident.ID.String() // Key for Slack incident notification detection
	fields["event_id"] = incident.ID.String()
	fields["Category"] = string(incident.Category)
	fields["Events"] = fmt.Sprintf("%d", len(incident.Events))
	fields["Duration"] = formatDuration(incident.Duration())

	// Add link to full analysis
	if n.apiBaseURL != "" {
		fields["_incident_url"] = fmt.Sprintf("%s/api/v1/incidents/%s", n.apiBaseURL, incident.ID.String())
	}

	return notifications.Notification{
		Title:     title,
		Message:   body.String(),
		Level:     level,
		Timestamp: incident.FirstEventAt,
		Tags:      tags,
		Fields:    fields,
	}
}

// formatCategory formats the category for display
func formatCategory(category IncidentCategory) string {
	categoryNames := map[IncidentCategory]string{
		CategoryMemory:     "💾 Memory Issue",
		CategoryCrash:      "💥 Crash Loop",
		CategoryImage:      "🖼 Image Pull",
		CategoryStorage:    "💿 Storage",
		CategoryNode:       "🖥 Node Issue",
		CategoryScheduling: "📋 Scheduling",
		CategoryDeployment: "🚀 Deployment",
		CategoryNetwork:    "🌐 Network",
		CategoryUnknown:    "❓ Unknown",
	}

	if name, ok := categoryNames[category]; ok {
		return name
	}
	return string(category)
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// truncateStr truncates a string to max length with ellipsis
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
