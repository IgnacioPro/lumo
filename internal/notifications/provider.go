package notifications

import (
	"context"
)

// Provider defines the interface for all notification providers
type Provider interface {
	// Name returns the unique name of this provider (e.g., "slack", "teams")
	Name() string

	// Send sends a notification message via this provider
	Send(ctx context.Context, msg *Message) error

	// Health checks if the provider is properly configured and reachable
	Health(ctx context.Context) error
}

// ProviderConfig represents common configuration for all providers
type ProviderConfig struct {
	Enabled bool
	Name    string
}

// HostInfo contains information about the host that generated the notification
type HostInfo struct {
	Hostname     string `json:"hostname"`
	IP           string `json:"ip,omitempty"`
	OS           string `json:"os,omitempty"`
	Platform     string `json:"platform,omitempty"`
	Architecture string `json:"architecture,omitempty"`
}

// Attachment represents a file or content attachment to a notification
type Attachment struct {
	Title    string `json:"title,omitempty"`
	Content  string `json:"content"`
	Filename string `json:"filename,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// Field represents a key-value field in a notification
type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Short bool   `json:"short,omitempty"` // Display as short (inline) field
}
