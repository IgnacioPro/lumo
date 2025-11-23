package eventdriven

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/ai"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/notifications"
)

// DefaultEventProcessor processes debounced events with AI analysis and notifications
type DefaultEventProcessor struct {
	logger       *logrus.Entry
	aiProvider   ai.Provider
	notifiers    []notifications.Notifier
	config       *config.Config
	aiEnabled    bool
	notifEnabled bool
}

// ProcessorConfig holds configuration for the event processor
type ProcessorConfig struct {
	Config      *config.Config
	AIProvider  ai.Provider
	Notifiers   []notifications.Notifier
	EnableAI    bool
	EnableNotif bool
}

// NewDefaultEventProcessor creates a new event processor
func NewDefaultEventProcessor(cfg *ProcessorConfig, logger *logrus.Logger) (*DefaultEventProcessor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("processor config cannot be nil")
	}

	return &DefaultEventProcessor{
		logger:       logger.WithField("component", "event-processor"),
		aiProvider:   cfg.AIProvider,
		notifiers:    cfg.Notifiers,
		config:       cfg.Config,
		aiEnabled:    cfg.EnableAI && cfg.AIProvider != nil,
		notifEnabled: cfg.EnableNotif && len(cfg.Notifiers) > 0,
	}, nil
}

// Process handles a debounced Kubernetes event
func (p *DefaultEventProcessor) Process(event *KubernetesEvent) error {
	p.logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"severity":   event.Severity,
		"resource":   event.ResourceKind + "/" + event.ResourceName,
		"namespace":  event.ResourceNamespace,
		"count":      event.Count,
		"first_seen": event.FirstSeen,
		"last_seen":  event.LastSeen,
	}).Info("Processing Kubernetes event")

	// Build event summary
	summary := p.buildEventSummary(event)

	// Perform AI analysis if enabled
	var analysis string
	if p.aiEnabled {
		p.logger.Debug("Performing AI analysis of event")
		aiAnalysis, err := p.analyzeWithAI(event, summary)
		if err != nil {
			p.logger.WithError(err).Error("AI analysis failed")
			analysis = fmt.Sprintf("AI analysis failed: %v", err)
		} else {
			analysis = aiAnalysis
		}
	} else {
		analysis = "AI analysis disabled"
	}

	// Send notifications if enabled
	if p.notifEnabled {
		p.logger.Debug("Sending event notifications")
		if err := p.sendNotifications(event, summary, analysis); err != nil {
			p.logger.WithError(err).Error("Failed to send notifications")
			return fmt.Errorf("notification failed: %w", err)
		}
	} else {
		p.logger.Debug("Notifications disabled")
	}

	p.logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"resource":   event.ResourceKind + "/" + event.ResourceName,
	}).Info("Event processing completed")

	return nil
}

// buildEventSummary creates a human-readable summary of the event
func (p *DefaultEventProcessor) buildEventSummary(event *KubernetesEvent) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("🚨 Kubernetes Event Alert: %s\n\n", event.Type))

	// Severity
	sb.WriteString(fmt.Sprintf("**Severity:** %s\n", event.Severity))

	// Resource
	sb.WriteString(fmt.Sprintf("**Resource:** %s/%s", event.ResourceKind, event.ResourceName))
	if event.ResourceNamespace != "" {
		sb.WriteString(fmt.Sprintf(" (namespace: %s)", event.ResourceNamespace))
	}
	sb.WriteString("\n\n")

	// Owner (if different from resource)
	if event.OwnerKind != "" && event.OwnerKind != event.ResourceKind {
		sb.WriteString(fmt.Sprintf("**Owner:** %s/%s\n", event.OwnerKind, event.OwnerName))
	}

	// Event details
	sb.WriteString(fmt.Sprintf("**Reason:** %s\n", event.Reason))
	sb.WriteString(fmt.Sprintf("**Message:** %s\n\n", event.Message))

	// Timing
	if event.Count > 1 {
		sb.WriteString(fmt.Sprintf("**Occurrences:** %d times\n", event.Count))
		sb.WriteString(fmt.Sprintf("**First Seen:** %s\n", event.FirstSeen.Format("2006-01-02 15:04:05 MST")))
		sb.WriteString(fmt.Sprintf("**Last Seen:** %s\n", event.LastSeen.Format("2006-01-02 15:04:05 MST")))
	} else {
		sb.WriteString(fmt.Sprintf("**Detected At:** %s\n", event.Timestamp.Format("2006-01-02 15:04:05 MST")))
	}

	// Related events
	if len(event.RelatedEvents) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Related Events:** %d\n", len(event.RelatedEvents)))
		for i, related := range event.RelatedEvents {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("... and %d more\n", len(event.RelatedEvents)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s: %s/%s - %s\n",
				related.Type,
				related.ResourceKind,
				related.ResourceName,
				related.Reason,
			))
		}
	}

	// Labels
	if len(event.Labels) > 0 {
		sb.WriteString("\n**Labels:**\n")
		for key, value := range event.Labels {
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", key, value))
		}
	}

	return sb.String()
}

// analyzeWithAI performs AI analysis of the event
func (p *DefaultEventProcessor) analyzeWithAI(event *KubernetesEvent, summary string) (string, error) {
	// Build prompt for AI
	prompt := p.buildAIPrompt(event, summary)

	// Call AI provider using Ask method (simpler than full Analyze for event analysis)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	response, _, err := p.aiProvider.Ask(ctx, "You are a Kubernetes SRE expert analyzing cluster events.", prompt)
	if err != nil {
		return "", fmt.Errorf("AI provider analysis failed: %w", err)
	}

	return response, nil
}

// buildAIPrompt creates an AI prompt for event analysis
func (p *DefaultEventProcessor) buildAIPrompt(event *KubernetesEvent, summary string) string {
	var sb strings.Builder

	sb.WriteString("You are a Kubernetes SRE expert. Analyze this Kubernetes event and provide:\n")
	sb.WriteString("1. Root cause analysis\n")
	sb.WriteString("2. Impact assessment\n")
	sb.WriteString("3. Recommended remediation steps\n")
	sb.WriteString("4. Prevention measures\n\n")

	sb.WriteString("Event Details:\n")
	sb.WriteString(summary)
	sb.WriteString("\n")

	// Add related events context
	if len(event.RelatedEvents) > 0 {
		sb.WriteString(fmt.Sprintf("\nRelated Events (%d):\n", len(event.RelatedEvents)))
		for _, related := range event.RelatedEvents {
			sb.WriteString(fmt.Sprintf("- %s: %s/%s - %s\n",
				related.Type,
				related.ResourceKind,
				related.ResourceName,
				related.Reason,
			))
		}
	}

	sb.WriteString("\nProvide a concise, actionable analysis.")

	return sb.String()
}

// sendNotifications sends event notifications through configured channels
func (p *DefaultEventProcessor) sendNotifications(event *KubernetesEvent, summary, analysis string) error {
	// Build notification message
	message := p.buildNotificationMessage(event, summary, analysis)

	// Determine notification level based on severity
	var level notifications.NotificationLevel
	switch event.Severity {
	case SeverityCritical:
		level = notifications.LevelCritical
	case SeverityHigh:
		level = notifications.LevelError
	case SeverityMedium:
		level = notifications.LevelWarning
	case SeverityLow:
		level = notifications.LevelInfo
	default:
		level = notifications.LevelInfo
	}

	// Create notification
	notif := &notifications.Notification{
		Title:     fmt.Sprintf("Kubernetes Event: %s", event.Type),
		Message:   message,
		Level:     level,
		Timestamp: time.Now(),
		Fields: map[string]string{
			"event_type": string(event.Type),
			"severity":   string(event.Severity),
			"resource":   event.ResourceKind + "/" + event.ResourceName,
			"namespace":  event.ResourceNamespace,
			"source":     "lumo-event-driven-agent",
		},
		Tags: []string{
			string(event.Type),
			string(event.Severity),
			event.ResourceKind,
			event.ResourceNamespace,
		},
	}

	// Send to all notifiers
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lastErr error
	successCount := 0
	for _, notifier := range p.notifiers {
		if err := notifier.Send(ctx, notif); err != nil {
			p.logger.WithError(err).WithField("notifier", notifier.Name()).Error("Failed to send notification")
			lastErr = err
		} else {
			successCount++
		}
	}

	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("all notifiers failed, last error: %w", lastErr)
	}

	p.logger.WithFields(logrus.Fields{
		"event_type":     event.Type,
		"severity":       event.Severity,
		"notifiers_sent": successCount,
		"notifiers_total": len(p.notifiers),
	}).Info("Notifications sent")

	return nil
}

// buildNotificationMessage creates the notification message
func (p *DefaultEventProcessor) buildNotificationMessage(event *KubernetesEvent, summary, analysis string) string {
	var sb strings.Builder

	// Summary
	sb.WriteString(summary)
	sb.WriteString("\n---\n\n")

	// AI Analysis
	if analysis != "" && analysis != "AI analysis disabled" {
		sb.WriteString("**AI Analysis:**\n")
		sb.WriteString(analysis)
		sb.WriteString("\n")
	}

	return sb.String()
}
