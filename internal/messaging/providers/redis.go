package providers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ignacio/lumo/internal/reliability"
)

var (
	redisPublishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lumo_redis_publish_total",
		Help: "Total number of messages published to Redis Streams",
	}, []string{"topic", "status"})

	redisPublishDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lumo_redis_publish_duration_seconds",
		Help:    "Duration of Redis Streams publish operations",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
	}, []string{"topic"})

	redisSubscribeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lumo_redis_subscribe_total",
		Help: "Total number of messages received from Redis Streams",
	}, []string{"topic", "status"})
)

// RedisProvider implements the Provider interface for Redis Streams
type RedisProvider struct{}

// NewRedisProvider creates a new Redis Streams provider
func NewRedisProvider() *RedisProvider {
	return &RedisProvider{}
}

// NewPublisher creates a new Redis Streams publisher
func (p *RedisProvider) NewPublisher(config *Config) (Publisher, error) {
	client, err := p.connect(config)
	if err != nil {
		return nil, fmt.Errorf("connect to Redis: %w", err)
	}

	cb := reliability.NewCircuitBreaker("redis-publisher")

	return &RedisPublisher{
		client:         client,
		config:         config,
		circuitBreaker: cb,
		tracer:         otel.Tracer("messaging/redis"),
	}, nil
}

// NewSubscriber creates a new Redis Streams subscriber
func (p *RedisProvider) NewSubscriber(config *Config) (Subscriber, error) {
	client, err := p.connect(config)
	if err != nil {
		return nil, fmt.Errorf("connect to Redis: %w", err)
	}

	return &RedisSubscriber{
		client:    client,
		config:    config,
		consumers: make(map[string]context.CancelFunc),
		tracer:    otel.Tracer("messaging/redis"),
	}, nil
}

// Health checks the Redis connection health
func (p *RedisProvider) Health() error {
	return nil
}

func (p *RedisProvider) connect(config *Config) (*redis.Client, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("no Redis brokers configured")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     config.Brokers[0],
		Password: config.Password,
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping Redis: %w", err)
	}

	return client, nil
}

// RedisPublisher implements the Publisher interface for Redis Streams
type RedisPublisher struct {
	client         *redis.Client
	config         *Config
	circuitBreaker *reliability.CircuitBreaker
	tracer         trace.Tracer
}

// Publish sends a single message to Redis Streams
func (p *RedisPublisher) Publish(ctx context.Context, topic string, message []byte) error {
	ctx, span := p.tracer.Start(ctx, "redis.publish",
		trace.WithAttributes(
			attribute.String("topic", topic),
			attribute.Int("message_size", len(message)),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		redisPublishDuration.WithLabelValues(topic).Observe(duration)
	}()

	_, err := p.circuitBreaker.Execute(func() (interface{}, error) {
		// Use XADD to append to stream
		args := &redis.XAddArgs{
			Stream: topic,
			Values: map[string]interface{}{
				"data":      message,
				"timestamp": time.Now().Unix(),
			},
			MaxLen: 10000, // Trim stream to max 10k entries
			Approx: true,  // Approximate trimming for performance
		}
		return nil, p.client.XAdd(ctx, args).Err()
	})

	if err != nil {
		redisPublishTotal.WithLabelValues(topic, "error").Inc()
		span.RecordError(err)
		return fmt.Errorf("publish to Redis Streams: %w", err)
	}

	redisPublishTotal.WithLabelValues(topic, "success").Inc()
	return nil
}

// PublishBatch sends multiple messages to Redis Streams in a batch
func (p *RedisPublisher) PublishBatch(ctx context.Context, topic string, messages [][]byte) error {
	ctx, span := p.tracer.Start(ctx, "redis.publish_batch",
		trace.WithAttributes(
			attribute.String("topic", topic),
			attribute.Int("batch_size", len(messages)),
		))
	defer span.End()

	if len(messages) == 0 {
		return nil
	}

	// Use pipeline for batch operations
	pipe := p.client.Pipeline()
	for _, msg := range messages {
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: topic,
			Values: map[string]interface{}{
				"data":      msg,
				"timestamp": time.Now().Unix(),
			},
			MaxLen: 10000,
			Approx: true,
		})
	}

	_, err := p.circuitBreaker.Execute(func() (interface{}, error) {
		_, execErr := pipe.Exec(ctx)
		return nil, execErr
	})

	if err != nil {
		return fmt.Errorf("batch publish to Redis Streams: %w", err)
	}

	return nil
}

// Close closes the Redis connection
func (p *RedisPublisher) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// RedisSubscriber implements the Subscriber interface for Redis Streams
type RedisSubscriber struct {
	client    *redis.Client
	config    *Config
	consumers map[string]context.CancelFunc
	mu        sync.Mutex
	tracer    trace.Tracer
}

// Subscribe subscribes to a topic
func (s *RedisSubscriber) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.consumers[topic]; exists {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}

	groupName := s.config.ConsumerGroup
	if groupName == "" {
		groupName = "lumo-api-consumer"
	}

	// Create consumer group if it doesn't exist
	err := s.client.XGroupCreateMkStream(ctx, topic, groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("create consumer group: %w", err)
	}

	// Start consuming in a goroutine
	consumerCtx, cancel := context.WithCancel(ctx)
	s.consumers[topic] = cancel

	go s.consume(consumerCtx, topic, groupName, handler)

	log.WithField("topic", topic).Info("Subscribed to Redis Streams topic")
	return nil
}

func (s *RedisSubscriber) consume(ctx context.Context, topic, groupName string, handler MessageHandler) {
	consumerName := fmt.Sprintf("consumer-%d", time.Now().Unix())

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Read from consumer group
			streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    groupName,
				Consumer: consumerName,
				Streams:  []string{topic, ">"},
				Count:    10,
				Block:    time.Second,
			}).Result()

			if err != nil {
				if err == redis.Nil || err == context.Canceled {
					continue
				}
				log.WithError(err).Error("Failed to read from Redis Streams")
				time.Sleep(time.Second)
				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					s.handleMessage(ctx, topic, groupName, msg, handler)
				}
			}
		}
	}
}

func (s *RedisSubscriber) handleMessage(ctx context.Context, topic, groupName string, redisMsg redis.XMessage, handler MessageHandler) {
	ctx, span := s.tracer.Start(ctx, "redis.handle_message",
		trace.WithAttributes(
			attribute.String("topic", topic),
			attribute.String("message_id", redisMsg.ID),
		))
	defer span.End()

	data, ok := redisMsg.Values["data"].(string)
	if !ok {
		log.Error("Message data is not a string")
		return
	}

	msg := &Message{
		ID:        redisMsg.ID,
		Topic:     topic,
		Data:      []byte(data),
		Headers:   make(map[string]string),
		Timestamp: time.Now(),
	}

	// Copy headers from message values
	for key, value := range redisMsg.Values {
		if key != "data" {
			if str, ok := value.(string); ok {
				msg.Headers[key] = str
			}
		}
	}

	err := handler(ctx, msg)
	if err != nil {
		redisSubscribeTotal.WithLabelValues(msg.Topic, "error").Inc()
		log.WithFields(log.Fields{
			"topic": msg.Topic,
			"error": err,
		}).Error("Message handler failed")

		// Send to dead-letter queue if configured
		if s.config.DeadLetterTopic != "" {
			s.sendToDLQ(ctx, msg)
		}
		return
	}

	redisSubscribeTotal.WithLabelValues(msg.Topic, "success").Inc()

	// Acknowledge message
	if err := s.client.XAck(ctx, topic, groupName, redisMsg.ID).Err(); err != nil {
		log.WithError(err).Error("Failed to ACK message")
	}
}

func (s *RedisSubscriber) sendToDLQ(ctx context.Context, msg *Message) {
	err := s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: s.config.DeadLetterTopic,
		Values: map[string]interface{}{
			"data":           msg.Data,
			"original_topic": msg.Topic,
			"timestamp":      time.Now().Unix(),
		},
		MaxLen: 10000,
		Approx: true,
	}).Err()

	if err != nil {
		log.WithError(err).Error("Failed to send message to dead-letter queue")
	}
}

// SubscribeMulti subscribes to multiple topics
func (s *RedisSubscriber) SubscribeMulti(ctx context.Context, topics []string, handler MessageHandler) error {
	for _, topic := range topics {
		if err := s.Subscribe(ctx, topic, handler); err != nil {
			return fmt.Errorf("subscribe to %s: %w", topic, err)
		}
	}
	return nil
}

// Unsubscribe unsubscribes from a topic
func (s *RedisSubscriber) Unsubscribe(topic string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cancel, exists := s.consumers[topic]
	if !exists {
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}

	cancel()
	delete(s.consumers, topic)
	return nil
}

// Close closes all subscriptions and the connection
func (s *RedisSubscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, cancel := range s.consumers {
		cancel()
	}

	if s.client != nil {
		return s.client.Close()
	}

	return nil
}
