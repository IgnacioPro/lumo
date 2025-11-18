package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MessageTopic represents the different types of messages that can be published
type MessageTopic string

const (
	// TopicDiagnostics is for diagnostic results
	TopicDiagnostics MessageTopic = "lumo.diagnostics"
	// TopicRemediation is for remediation actions and results
	TopicRemediation MessageTopic = "lumo.remediation"
	// TopicAlerts is for system alerts and warnings
	TopicAlerts MessageTopic = "lumo.alerts"
	// TopicLifecycle is for agent lifecycle events (startup, shutdown, heartbeat)
	TopicLifecycle MessageTopic = "lumo.lifecycle"
	// TopicMetrics is for agent metrics and performance data
	TopicMetrics MessageTopic = "lumo.metrics"
)

// Message represents a message to be published or received
type Message struct {
	ID        string                 `json:"id"`
	Topic     MessageTopic           `json:"topic"`
	Timestamp time.Time              `json:"timestamp"`
	AgentID   string                 `json:"agent_id,omitempty"`
	Hostname  string                 `json:"hostname,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
}

// NewMessage creates a new message with a unique ID and current timestamp
func NewMessage(topic MessageTopic, data map[string]interface{}) *Message {
	return &Message{
		ID:        uuid.New().String(),
		Topic:     topic,
		Timestamp: time.Now().UTC(),
		Data:      data,
		Metadata:  make(map[string]string),
	}
}

// Marshal serializes the message to JSON
func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// Unmarshal deserializes a message from JSON
func Unmarshal(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	return &msg, nil
}

// Publisher defines the interface for publishing messages
type Publisher interface {
	// Publish sends a message to the specified topic
	Publish(ctx context.Context, msg *Message) error

	// PublishBatch sends multiple messages in a batch (more efficient)
	PublishBatch(ctx context.Context, messages []*Message) error

	// Close closes the publisher and releases resources
	Close() error
}

// Subscriber defines the interface for receiving messages
type Subscriber interface {
	// Subscribe starts receiving messages from specified topics
	Subscribe(ctx context.Context, topics []MessageTopic, handler MessageHandler) error

	// Unsubscribe stops receiving messages from specified topics
	Unsubscribe(topics []MessageTopic) error

	// Close closes the subscriber and releases resources
	Close() error
}

// MessageHandler is a function that handles received messages
type MessageHandler func(ctx context.Context, msg *Message) error

// Config contains messaging configuration
type Config struct {
	Enabled  bool          `mapstructure:"enabled"`
	Provider string        `mapstructure:"provider"` // nats|kafka|rabbitmq|redis
	URL      string        `mapstructure:"url"`
	Username string        `mapstructure:"username"`
	Password string        `mapstructure:"password"`
	TLS      TLSConfig     `mapstructure:"tls"`
	Retry    RetryConfig   `mapstructure:"retry"`
	DLQ      DLQConfig     `mapstructure:"dlq"` // Dead Letter Queue
	Timeout  time.Duration `mapstructure:"timeout"`

	// Provider-specific options
	Options map[string]interface{} `mapstructure:"options"`
}

// TLSConfig contains TLS settings for messaging
type TLSConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
	CAFile             string `mapstructure:"ca_file"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
}

// RetryConfig contains retry settings
type RetryConfig struct {
	MaxAttempts int           `mapstructure:"max_attempts"`
	BaseDelay   time.Duration `mapstructure:"base_delay"`
	MaxDelay    time.Duration `mapstructure:"max_delay"`
}

// DLQConfig contains dead letter queue settings
type DLQConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Topic   string `mapstructure:"topic"` // Topic/queue for failed messages
	MaxSize int    `mapstructure:"max_size"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Enabled:  false,
		Provider: "nats",
		URL:      "nats://localhost:4222",
		Timeout:  30 * time.Second,
		TLS: TLSConfig{
			Enabled:            false,
			InsecureSkipVerify: false,
		},
		Retry: RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   1 * time.Second,
			MaxDelay:    30 * time.Second,
		},
		DLQ: DLQConfig{
			Enabled: true,
			Topic:   "lumo.dlq",
			MaxSize: 10000,
		},
		Options: make(map[string]interface{}),
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	validProviders := map[string]bool{
		"nats":     true,
		"kafka":    true,
		"rabbitmq": true,
		"redis":    true,
	}
	if !validProviders[c.Provider] {
		return fmt.Errorf("unsupported messaging provider: %s (supported: nats, kafka, rabbitmq, redis)", c.Provider)
	}

	if c.URL == "" {
		return fmt.Errorf("messaging URL cannot be empty")
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("messaging timeout must be positive")
	}

	if c.Retry.MaxAttempts < 1 {
		return fmt.Errorf("retry max_attempts must be at least 1")
	}

	if c.Retry.BaseDelay <= 0 {
		return fmt.Errorf("retry base_delay must be positive")
	}

	if c.TLS.Enabled {
		if c.TLS.CertFile == "" || c.TLS.KeyFile == "" {
			return fmt.Errorf("TLS enabled but cert_file or key_file not specified")
		}
	}

	return nil
}
