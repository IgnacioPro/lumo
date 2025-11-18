package messaging

import (
	"context"
	"fmt"

	"github.com/ignacio/lumo/internal/messaging/providers"
	"github.com/sirupsen/logrus"
)

// NewPublisher creates a new publisher based on the configuration
func NewPublisher(cfg *Config, logger *logrus.Logger) (Publisher, error) {
	if !cfg.Enabled {
		return &NoOpPublisher{}, nil
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid messaging config: %w", err)
	}

	// Create config adapter for providers
	adapter := NewConfigAdapter(cfg)

	switch cfg.Provider {
	case "nats":
		return providers.NewNATSPublisher(adapter, logger)
	case "kafka":
		return providers.NewKafkaPublisher(adapter, logger)
	case "rabbitmq":
		return providers.NewRabbitMQPublisher(adapter, logger)
	case "redis":
		return providers.NewRedisPublisher(adapter, logger)
	default:
		return nil, fmt.Errorf("unsupported messaging provider: %s", cfg.Provider)
	}
}

// NewSubscriber creates a new subscriber based on the configuration
func NewSubscriber(cfg *Config, logger *logrus.Logger) (Subscriber, error) {
	if !cfg.Enabled {
		return &NoOpSubscriber{}, nil
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid messaging config: %w", err)
	}

	// Create config adapter for providers
	adapter := NewConfigAdapter(cfg)

	switch cfg.Provider {
	case "nats":
		return providers.NewNATSSubscriber(adapter, logger)
	case "kafka":
		return providers.NewKafkaSubscriber(adapter, logger)
	case "rabbitmq":
		return providers.NewRabbitMQSubscriber(adapter, logger)
	case "redis":
		return providers.NewRedisSubscriber(adapter, logger)
	default:
		return nil, fmt.Errorf("unsupported messaging provider: %s", cfg.Provider)
	}
}

// NoOpPublisher is a no-op publisher that does nothing (when messaging is disabled)
type NoOpPublisher struct{}

func (n *NoOpPublisher) Publish(ctx context.Context, msg *Message) error {
	return nil
}

func (n *NoOpPublisher) PublishBatch(ctx context.Context, messages []*Message) error {
	return nil
}

func (n *NoOpPublisher) Close() error {
	return nil
}

// NoOpSubscriber is a no-op subscriber that does nothing (when messaging is disabled)
type NoOpSubscriber struct{}

func (n *NoOpSubscriber) Subscribe(ctx context.Context, topics []MessageTopic, handler MessageHandler) error {
	return nil
}

func (n *NoOpSubscriber) Unsubscribe(topics []MessageTopic) error {
	return nil
}

func (n *NoOpSubscriber) Close() error {
	return nil
}
