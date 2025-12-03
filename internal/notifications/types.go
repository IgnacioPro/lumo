package notifications

import "time"

// Notification represents a message to be sent via a notifier.
type Notification struct {
	// Title is the notification title/subject
	Title string

	// Message is the main notification body
	Message string

	// Level indicates the notification severity
	Level NotificationLevel

	// Timestamp is when the notification was created
	Timestamp time.Time

	// Fields contains additional structured data
	Fields map[string]string

	// Tags can be used for filtering/routing
	Tags []string

	// Postmortem contains a detailed postmortem report (optional)
	// When set, it will be sent as follow-up messages (e.g., threaded replies in Slack)
	Postmortem string
}

// NotificationLevel represents the severity of a notification.
type NotificationLevel string

const (
	// LevelInfo represents informational notifications
	LevelInfo NotificationLevel = "info"

	// LevelWarning represents warning notifications
	LevelWarning NotificationLevel = "warning"

	// LevelError represents error notifications
	LevelError NotificationLevel = "error"

	// LevelCritical represents critical notifications
	LevelCritical NotificationLevel = "critical"

	// LevelSuccess represents success notifications
	LevelSuccess NotificationLevel = "success"
)

// String returns the string representation of the notification level.
func (l NotificationLevel) String() string {
	return string(l)
}

// Icon returns an emoji icon for the notification level.
func (l NotificationLevel) Icon() string {
	switch l {
	case LevelInfo:
		return "ℹ️"
	case LevelWarning:
		return "⚠️"
	case LevelError:
		return "❌"
	case LevelCritical:
		return "🚨"
	case LevelSuccess:
		return "✅"
	default:
		return "📝"
	}
}

// Color returns a color code for the notification level (used by Slack, Discord, etc.).
func (l NotificationLevel) Color() string {
	switch l {
	case LevelInfo:
		return "#0066CC" // Blue
	case LevelWarning:
		return "#FFA500" // Orange
	case LevelError:
		return "#DC143C" // Red
	case LevelCritical:
		return "#8B0000" // Dark Red
	case LevelSuccess:
		return "#32CD32" // Green
	default:
		return "#808080" // Gray
	}
}

// NewNotification creates a new notification with the given parameters.
func NewNotification(title, message string, level NotificationLevel) *Notification {
	return &Notification{
		Title:     title,
		Message:   message,
		Level:     level,
		Timestamp: time.Now(),
		Fields:    make(map[string]string),
		Tags:      []string{},
	}
}

// WithField adds a field to the notification.
func (n *Notification) WithField(key, value string) *Notification {
	if n.Fields == nil {
		n.Fields = make(map[string]string)
	}
	n.Fields[key] = value
	return n
}

// WithTag adds a tag to the notification.
func (n *Notification) WithTag(tag string) *Notification {
	n.Tags = append(n.Tags, tag)
	return n
}
