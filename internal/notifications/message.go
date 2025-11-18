package notifications

import (
	"fmt"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// Message represents a notification message to be sent
type Message struct {
	// Severity indicates the importance level of this notification
	Severity diagnostics.Severity `json:"severity"`

	// Title is the main heading/subject of the notification
	Title string `json:"title"`

	// Body is the main content/description
	Body string `json:"body"`

	// Fields are additional structured key-value pairs
	Fields []Field `json:"fields,omitempty"`

	// Timestamp when the notification was created
	Timestamp time.Time `json:"timestamp"`

	// Source indicates where the notification originated (e.g., "diagnostics", "remediation", "api")
	Source string `json:"source"`

	// HostInfo contains information about the affected host
	HostInfo *HostInfo `json:"host_info,omitempty"`

	// Attachments are optional file attachments
	Attachments []Attachment `json:"attachments,omitempty"`

	// Metadata contains additional provider-specific or custom data
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Tags for categorization and routing
	Tags []string `json:"tags,omitempty"`
}

// MessageBuilder provides a fluent API for building notification messages
type MessageBuilder struct {
	msg *Message
}

// NewMessageBuilder creates a new MessageBuilder with sensible defaults
func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{
		msg: &Message{
			Severity:  diagnostics.SeverityInfo,
			Timestamp: time.Now(),
			Fields:    []Field{},
			Tags:      []string{},
			Metadata:  make(map[string]interface{}),
		},
	}
}

// Severity sets the notification severity level
func (b *MessageBuilder) Severity(severity diagnostics.Severity) *MessageBuilder {
	b.msg.Severity = severity
	return b
}

// Title sets the notification title
func (b *MessageBuilder) Title(title string) *MessageBuilder {
	b.msg.Title = title
	return b
}

// Body sets the notification body text
func (b *MessageBuilder) Body(body string) *MessageBuilder {
	b.msg.Body = body
	return b
}

// Source sets the notification source
func (b *MessageBuilder) Source(source string) *MessageBuilder {
	b.msg.Source = source
	return b
}

// Host sets the host information
func (b *MessageBuilder) Host(info *HostInfo) *MessageBuilder {
	b.msg.HostInfo = info
	return b
}

// AddField adds a structured field to the notification
func (b *MessageBuilder) AddField(name, value string, short bool) *MessageBuilder {
	b.msg.Fields = append(b.msg.Fields, Field{
		Name:  name,
		Value: value,
		Short: short,
	})
	return b
}

// AddTag adds a tag for categorization
func (b *MessageBuilder) AddTag(tag string) *MessageBuilder {
	b.msg.Tags = append(b.msg.Tags, tag)
	return b
}

// AddMetadata adds custom metadata
func (b *MessageBuilder) AddMetadata(key string, value interface{}) *MessageBuilder {
	b.msg.Metadata[key] = value
	return b
}

// AddAttachment adds a file attachment
func (b *MessageBuilder) AddAttachment(attachment Attachment) *MessageBuilder {
	b.msg.Attachments = append(b.msg.Attachments, attachment)
	return b
}

// Timestamp sets a custom timestamp
func (b *MessageBuilder) Timestamp(t time.Time) *MessageBuilder {
	b.msg.Timestamp = t
	return b
}

// Build validates and returns the final Message
func (b *MessageBuilder) Build() (*Message, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}
	return b.msg, nil
}

// validate checks if the message has all required fields
func (b *MessageBuilder) validate() error {
	if b.msg.Title == "" {
		return fmt.Errorf("message title is required")
	}
	if b.msg.Body == "" {
		return fmt.Errorf("message body is required")
	}
	if b.msg.Source == "" {
		return fmt.Errorf("message source is required")
	}
	return nil
}

// SeverityEmoji returns an emoji representation of the severity
func (m *Message) SeverityEmoji() string {
	switch m.Severity {
	case diagnostics.SeverityCritical:
		return "🚨"
	case diagnostics.SeverityError:
		return "❌"
	case diagnostics.SeverityWarning:
		return "⚠️"
	case diagnostics.SeverityInfo:
		return "ℹ️"
	case diagnostics.SeverityOK:
		return "✅"
	default:
		return "📝"
	}
}

// SeverityColor returns a hex color code for the severity level
func (m *Message) SeverityColor() string {
	switch m.Severity {
	case diagnostics.SeverityCritical:
		return "#DC3545" // Red
	case diagnostics.SeverityError:
		return "#FD7E14" // Orange
	case diagnostics.SeverityWarning:
		return "#FFC107" // Yellow
	case diagnostics.SeverityInfo:
		return "#17A2B8" // Cyan
	case diagnostics.SeverityOK:
		return "#28A745" // Green
	default:
		return "#6C757D" // Gray
	}
}
