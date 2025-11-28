package providers

import (
	"context"
	"time"
)

// Publisher sends messages to topics
type Publisher interface {
	Publish(ctx context.Context, topic string, message []byte) error
	PublishBatch(ctx context.Context, topic string, messages [][]byte) error
	Close() error
}

// Subscriber receives messages from topics
type Subscriber interface {
	Subscribe(ctx context.Context, topic string, handler MessageHandler) error
	SubscribeMulti(ctx context.Context, topics []string, handler MessageHandler) error
	Unsubscribe(topic string) error
	Close() error
}

// MessageHandler processes received messages
type MessageHandler func(ctx context.Context, msg *Message) error

// Message represents a message from the queue
type Message struct {
	ID        string
	Topic     string
	Data      []byte
	Headers   map[string]string
	Timestamp time.Time
	Attempt   int // For retry tracking
}

// Config holds messaging configuration
type Config struct {
	Provider        string
	Brokers         []string
	Username        string
	Password        string
	TLS             bool
	EventTopic      string
	DeadLetterTopic string
	MaxRetries      int
	RetryBackoff    time.Duration
	AckWaitTimeout  time.Duration
	ConsumerGroup   string
	MaxInFlight     int
}

// Provider creates Publisher and Subscriber instances
type Provider interface {
	NewPublisher(config *Config) (Publisher, error)
	NewSubscriber(config *Config) (Subscriber, error)
	Health() error
}
