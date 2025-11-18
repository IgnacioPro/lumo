package providers

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// RedisPublisher implements Publisher for Redis Pub/Sub
type RedisPublisher struct {
	client *redis.Client
	logger *logrus.Logger
	cfg    interface{}
}

// RedisSubscriber implements Subscriber for Redis Pub/Sub
type RedisSubscriber struct {
	client *redis.Client
	pubsub *redis.PubSub
	logger *logrus.Logger
	cfg    interface{}
}

// NewRedisPublisher creates a new Redis publisher
func NewRedisPublisher(cfg interface{}, logger *logrus.Logger) (*RedisPublisher, error) {
	msgCfg, ok := cfg.(configGetter)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	// Parse Redis options
	opts := &redis.Options{
		Addr:     msgCfg.GetURL(),
		Password: msgCfg.GetPassword(),
		DB:       0, // Default DB
	}

	// Configure TLS if enabled
	if msgCfg.IsTLSEnabled() {
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: msgCfg.IsTLSInsecure(),
		}
	}

	// Create client
	client := redis.NewClient(opts)

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.WithField("addr", opts.Addr).Info("Connected to Redis")

	return &RedisPublisher{
		client: client,
		logger: logger,
		cfg:    cfg,
	}, nil
}

// Publish sends a message to Redis pub/sub
func (p *RedisPublisher) Publish(ctx context.Context, msg *Message) error {
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish to Redis channel (topic)
	if err := p.client.Publish(ctx, msg.Topic, data).Err(); err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"topic":      msg.Topic,
		"message_id": msg.ID,
	}).Debug("Published message to Redis")

	return nil
}

// PublishBatch sends multiple messages in a batch using pipeline
func (p *RedisPublisher) PublishBatch(ctx context.Context, messages []*Message) error {
	pipe := p.client.Pipeline()

	for _, msg := range messages {
		data, err := msg.Marshal()
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}

		pipe.Publish(ctx, msg.Topic, data)
	}

	// Execute pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to publish batch to Redis: %w", err)
	}

	p.logger.WithField("count", len(messages)).Debug("Published batch to Redis")
	return nil
}

// Close closes the Redis connection
func (p *RedisPublisher) Close() error {
	if p.client != nil {
		if err := p.client.Close(); err != nil {
			return fmt.Errorf("failed to close Redis client: %w", err)
		}
		p.logger.Info("Redis publisher closed")
	}
	return nil
}

// NewRedisSubscriber creates a new Redis subscriber
func NewRedisSubscriber(cfg interface{}, logger *logrus.Logger) (*RedisSubscriber, error) {
	msgCfg, ok := cfg.(configGetter)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	opts := &redis.Options{
		Addr:     msgCfg.GetURL(),
		Password: msgCfg.GetPassword(),
		DB:       0,
	}

	if msgCfg.IsTLSEnabled() {
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: msgCfg.IsTLSInsecure(),
		}
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.WithField("addr", opts.Addr).Info("Redis subscriber initialized")

	return &RedisSubscriber{
		client: client,
		logger: logger,
		cfg:    cfg,
	}, nil
}

// Subscribe starts receiving messages from Redis pub/sub channels
func (s *RedisSubscriber) Subscribe(ctx context.Context, topics []string, handler func(context.Context, *Message) error) error {
	// Subscribe to channels
	s.pubsub = s.client.Subscribe(ctx, topics...)

	// Wait for subscription confirmation
	if _, err := s.pubsub.Receive(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to Redis channels: %w", err)
	}

	// Start message handler in goroutine
	go func() {
		ch := s.pubsub.Channel()
		for msg := range ch {
			s.logger.WithField("channel", msg.Channel).Debug("Received message from Redis")
			// TODO: Parse msg.Payload and call handler
		}
	}()

	s.logger.WithField("topics", topics).Info("Subscribed to Redis channels")
	return nil
}

// Unsubscribe stops receiving messages from specified topics
func (s *RedisSubscriber) Unsubscribe(topics []string) error {
	if s.pubsub != nil {
		if err := s.pubsub.Unsubscribe(context.Background(), topics...); err != nil {
			return fmt.Errorf("failed to unsubscribe from Redis channels: %w", err)
		}
		s.logger.WithField("topics", topics).Info("Unsubscribed from Redis channels")
	}
	return nil
}

// Close closes the Redis connection
func (s *RedisSubscriber) Close() error {
	if s.pubsub != nil {
		if err := s.pubsub.Close(); err != nil {
			s.logger.WithError(err).Error("Failed to close Redis pubsub")
		}
	}
	if s.client != nil {
		if err := s.client.Close(); err != nil {
			return fmt.Errorf("failed to close Redis client: %w", err)
		}
		s.logger.Info("Redis subscriber closed")
	}
	return nil
}
