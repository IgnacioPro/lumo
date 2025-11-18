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

	pa := &publisherAdapter{
		provider: cfg.Provider,
		logger:   logger,
	}
	var err error

	switch cfg.Provider {
	case "nats":
		pa.nats, err = providers.NewNATSPublisher(adapter, logger)
	case "kafka":
		pa.kafka, err = providers.NewKafkaPublisher(adapter, logger)
	case "rabbitmq":
		pa.rabbitmq, err = providers.NewRabbitMQPublisher(adapter, logger)
	case "redis":
		pa.redis, err = providers.NewRedisPublisher(adapter, logger)
	default:
		return nil, fmt.Errorf("unsupported messaging provider: %s", cfg.Provider)
	}

	// If provider not built, log warning and use NoOp
	if err != nil {
		logger.WithError(err).Warnf("Messaging provider %s not available, using NoOp", cfg.Provider)
		return &NoOpPublisher{}, nil
	}

	return pa, nil
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

	sa := &subscriberAdapter{
		provider: cfg.Provider,
		logger:   logger,
	}
	var err error

	switch cfg.Provider {
	case "nats":
		sa.nats, err = providers.NewNATSSubscriber(adapter, logger)
	case "kafka":
		sa.kafka, err = providers.NewKafkaSubscriber(adapter, logger)
	case "rabbitmq":
		sa.rabbitmq, err = providers.NewRabbitMQSubscriber(adapter, logger)
	case "redis":
		sa.redis, err = providers.NewRedisSubscriber(adapter, logger)
	default:
		return nil, fmt.Errorf("unsupported messaging provider: %s", cfg.Provider)
	}

	// If provider not built, log warning and use NoOp
	if err != nil {
		logger.WithError(err).Warnf("Messaging provider %s not available, using NoOp", cfg.Provider)
		return &NoOpSubscriber{}, nil
	}

	return sa, nil
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

// publisherAdapter adapts provider-specific publishers to the Publisher interface
type publisherAdapter struct {
	provider string
	nats     *providers.NATSPublisher
	kafka    *providers.KafkaPublisher
	rabbitmq *providers.RabbitMQPublisher
	redis    *providers.RedisPublisher
	logger   *logrus.Logger
}

func (pa *publisherAdapter) Publish(ctx context.Context, msg *Message) error {
	// Convert messaging.Message to provider message
	providerMsg := &providers.Message{
		ID:        msg.ID,
		Topic:     string(msg.Topic),
		Timestamp: msg.Timestamp,
		AgentID:   msg.AgentID,
		Hostname:  msg.Hostname,
		Data:      msg.Data,
		Metadata:  msg.Metadata,
	}

	switch pa.provider {
	case "nats":
		return pa.nats.Publish(ctx, providerMsg)
	case "kafka":
		return pa.kafka.Publish(ctx, providerMsg)
	case "rabbitmq":
		return pa.rabbitmq.Publish(ctx, providerMsg)
	case "redis":
		return pa.redis.Publish(ctx, providerMsg)
	default:
		return fmt.Errorf("unknown provider: %s", pa.provider)
	}
}

func (pa *publisherAdapter) PublishBatch(ctx context.Context, messages []*Message) error {
	// Convert messages
	providerMsgs := make([]*providers.Message, len(messages))
	for i, msg := range messages {
		providerMsgs[i] = &providers.Message{
			ID:        msg.ID,
			Topic:     string(msg.Topic),
			Timestamp: msg.Timestamp,
			AgentID:   msg.AgentID,
			Hostname:  msg.Hostname,
			Data:      msg.Data,
			Metadata:  msg.Metadata,
		}
	}

	switch pa.provider {
	case "nats":
		return pa.nats.PublishBatch(ctx, providerMsgs)
	case "kafka":
		return pa.kafka.PublishBatch(ctx, providerMsgs)
	case "rabbitmq":
		return pa.rabbitmq.PublishBatch(ctx, providerMsgs)
	case "redis":
		return pa.redis.PublishBatch(ctx, providerMsgs)
	default:
		return fmt.Errorf("unknown provider: %s", pa.provider)
	}
}

func (pa *publisherAdapter) Close() error {
	switch pa.provider {
	case "nats":
		return pa.nats.Close()
	case "kafka":
		return pa.kafka.Close()
	case "rabbitmq":
		return pa.rabbitmq.Close()
	case "redis":
		return pa.redis.Close()
	default:
		return fmt.Errorf("unknown provider: %s", pa.provider)
	}
}

// subscriberAdapter adapts provider-specific subscribers to the Subscriber interface
type subscriberAdapter struct {
	provider string
	nats     *providers.NATSSubscriber
	kafka    *providers.KafkaSubscriber
	rabbitmq *providers.RabbitMQSubscriber
	redis    *providers.RedisSubscriber
	logger   *logrus.Logger
}

func (sa *subscriberAdapter) Subscribe(ctx context.Context, topics []MessageTopic, handler MessageHandler) error {
	// Convert MessageTopic to string
	stringTopics := make([]string, len(topics))
	for i, topic := range topics {
		stringTopics[i] = string(topic)
	}

	// Convert MessageHandler to provider handler
	providerHandler := func(ctx context.Context, msg *providers.Message) error {
		// Convert provider message to messaging.Message
		message := &Message{
			ID:        msg.ID,
			Topic:     MessageTopic(msg.Topic),
			Timestamp: msg.Timestamp,
			AgentID:   msg.AgentID,
			Hostname:  msg.Hostname,
			Data:      msg.Data,
			Metadata:  msg.Metadata,
		}
		return handler(ctx, message)
	}

	switch sa.provider {
	case "nats":
		return sa.nats.Subscribe(ctx, stringTopics, providerHandler)
	case "kafka":
		return sa.kafka.Subscribe(ctx, stringTopics, providerHandler)
	case "rabbitmq":
		return sa.rabbitmq.Subscribe(ctx, stringTopics, providerHandler)
	case "redis":
		return sa.redis.Subscribe(ctx, stringTopics, providerHandler)
	default:
		return fmt.Errorf("unknown provider: %s", sa.provider)
	}
}

func (sa *subscriberAdapter) Unsubscribe(topics []MessageTopic) error {
	// Convert MessageTopic to string
	stringTopics := make([]string, len(topics))
	for i, topic := range topics {
		stringTopics[i] = string(topic)
	}

	switch sa.provider {
	case "nats":
		return sa.nats.Unsubscribe(stringTopics)
	case "kafka":
		return sa.kafka.Unsubscribe(stringTopics)
	case "rabbitmq":
		return sa.rabbitmq.Unsubscribe(stringTopics)
	case "redis":
		return sa.redis.Unsubscribe(stringTopics)
	default:
		return fmt.Errorf("unknown provider: %s", sa.provider)
	}
}

func (sa *subscriberAdapter) Close() error {
	switch sa.provider {
	case "nats":
		return sa.nats.Close()
	case "kafka":
		return sa.kafka.Close()
	case "rabbitmq":
		return sa.rabbitmq.Close()
	case "redis":
		return sa.redis.Close()
	default:
		return fmt.Errorf("unknown provider: %s", sa.provider)
	}
}
