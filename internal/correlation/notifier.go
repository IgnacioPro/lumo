package correlation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/notifications"
)

// IncidentNotifierImpl implements IncidentNotifier using the notifications package
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

// NotifyIncident sends notifications about an incident
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

// buildNotification creates a notification from an incident
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
