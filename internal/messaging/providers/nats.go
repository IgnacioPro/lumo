package providers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ignacio/lumo/internal/reliability"
)

var (
	natsPublishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lumo_nats_publish_total",
		Help: "Total number of messages published to NATS",
	}, []string{"topic", "status"})

	natsPublishDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lumo_nats_publish_duration_seconds",
		Help:    "Duration of NATS publish operations",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
	}, []string{"topic"})

	natsSubscribeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lumo_nats_subscribe_total",
		Help: "Total number of messages received from NATS",
	}, []string{"topic", "status"})
)

// NATSProvider implements the Provider interface for NATS
type NATSProvider struct{}

// NewNATSProvider creates a new NATS provider
func NewNATSProvider() *NATSProvider {
	return &NATSProvider{}
}

// NewPublisher creates a new NATS publisher
func (p *NATSProvider) NewPublisher(config *Config) (Publisher, error) {
	conn, js, err := p.connect(config)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	// Create stream if it doesn't exist
	streamName := "LUMO_EVENTS"
	_, err = js.StreamInfo(streamName)
	if err != nil {
		// Stream doesn't exist, create it
		_, err = js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{config.EventTopic, config.DeadLetterTopic},
			Storage:  nats.FileStorage,
			MaxAge:   24 * time.Hour,
		})
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("create stream: %w", err)
		}
	}

	cb := reliability.NewCircuitBreaker("nats-publisher")

	return &NATSPublisher{
		conn:           conn,
		js:             js,
		config:         config,
		circuitBreaker: cb,
		tracer:         otel.Tracer("messaging/nats"),
	}, nil
}

// NewSubscriber creates a new NATS subscriber
func (p *NATSProvider) NewSubscriber(config *Config) (Subscriber, error) {
	conn, js, err := p.connect(config)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	return &NATSSubscriber{
		conn:   conn,
		js:     js,
		config: config,
		subs:   make(map[string]*nats.Subscription),
		tracer: otel.Tracer("messaging/nats"),
	}, nil
}

// Health checks the NATS connection health
func (p *NATSProvider) Health() error {
	// This is a factory method, actual health checks are done on the connections
	return nil
}

func (p *NATSProvider) connect(config *Config) (*nats.Conn, nats.JetStreamContext, error) {
	if len(config.Brokers) == 0 {
		return nil, nil, fmt.Errorf("no NATS brokers configured")
	}

	opts := []nats.Option{
		nats.Name("lumo-agent"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.WithError(err).Warn("NATS disconnected")
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info("NATS reconnected")
		}),
	}

	if config.Username != "" && config.Password != "" {
		opts = append(opts, nats.UserInfo(config.Username, config.Password))
	}

	if config.TLS {
		opts = append(opts, nats.Secure())
	}

	conn, err := nats.Connect(config.Brokers[0], opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to NATS server: %w", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("create JetStream context: %w", err)
	}

	return conn, js, nil
}

// NATSPublisher implements the Publisher interface for NATS
type NATSPublisher struct {
	conn           *nats.Conn
	js             nats.JetStreamContext
	config         *Config
	circuitBreaker *reliability.CircuitBreaker
	tracer         trace.Tracer
}

// Publish sends a single message to NATS
func (p *NATSPublisher) Publish(ctx context.Context, topic string, message []byte) error {
	ctx, span := p.tracer.Start(ctx, "nats.publish",
		trace.WithAttributes(
			attribute.String("topic", topic),
			attribute.Int("message_size", len(message)),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		natsPublishDuration.WithLabelValues(topic).Observe(duration)
	}()

	_, err := p.circuitBreaker.Execute(func() (interface{}, error) {
		return nil, p.publishWithRetry(ctx, topic, message)
	})

	if err != nil {
		natsPublishTotal.WithLabelValues(topic, "error").Inc()
		span.RecordError(err)
		return fmt.Errorf("publish to NATS: %w", err)
	}

	natsPublishTotal.WithLabelValues(topic, "success").Inc()
	return nil
}

func (p *NATSPublisher) publishWithRetry(ctx context.Context, topic string, message []byte) error {
	var lastErr error
	backoff := p.config.RetryBackoff
	if backoff == 0 {
		backoff = 5 * time.Second
	}

	maxRetries := p.config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff * time.Duration(attempt)):
			}
		}

		_, err := p.js.Publish(topic, message, nats.Context(ctx))
		if err == nil {
			return nil
		}

		lastErr = err
		log.WithFields(log.Fields{
			"topic":   topic,
			"attempt": attempt + 1,
			"error":   err,
		}).Warn("NATS publish attempt failed")
	}

	return fmt.Errorf("publish failed after %d attempts: %w", maxRetries+1, lastErr)
}

// PublishBatch sends multiple messages to NATS in a batch
func (p *NATSPublisher) PublishBatch(ctx context.Context, topic string, messages [][]byte) error {
	ctx, span := p.tracer.Start(ctx, "nats.publish_batch",
		trace.WithAttributes(
			attribute.String("topic", topic),
			attribute.Int("batch_size", len(messages)),
		))
	defer span.End()

	if len(messages) == 0 {
		return nil
	}

	// NATS doesn't have native batch support, publish sequentially
	// but we can optimize with goroutines for better throughput
	errCh := make(chan error, len(messages))
	for _, msg := range messages {
		go func(m []byte) {
			errCh <- p.Publish(ctx, topic, m)
		}(msg)
	}

	var errs []error
	for i := 0; i < len(messages); i++ {
		if err := <-errCh; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("batch publish failed: %d errors, first: %w", len(errs), errs[0])
	}

	return nil
}

// Close closes the NATS connection
func (p *NATSPublisher) Close() error {
	if p.conn != nil {
		p.conn.Close()
	}
	return nil
}

// NATSSubscriber implements the Subscriber interface for NATS
type NATSSubscriber struct {
	conn   *nats.Conn
	js     nats.JetStreamContext
	config *Config
	subs   map[string]*nats.Subscription
	mu     sync.Mutex
	tracer trace.Tracer
}

// Subscribe subscribes to a topic
func (s *NATSSubscriber) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.subs[topic]; exists {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}

	consumerName := s.config.ConsumerGroup
	if consumerName == "" {
		consumerName = "lumo-api-consumer"
	}

	// Create or get consumer
	_, err := s.js.AddConsumer("LUMO_EVENTS", &nats.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     nats.AckExplicitPolicy,
		MaxAckPending: s.config.MaxInFlight,
		FilterSubject: topic,
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	sub, err := s.js.Subscribe(topic, func(msg *nats.Msg) {
		s.handleMessage(ctx, msg, handler)
	}, nats.Durable(consumerName), nats.ManualAck())
	if err != nil {
		return fmt.Errorf("subscribe to topic: %w", err)
	}

	s.subs[topic] = sub
	log.WithField("topic", topic).Info("Subscribed to NATS topic")
	return nil
}

func (s *NATSSubscriber) handleMessage(ctx context.Context, natsMsg *nats.Msg, handler MessageHandler) {
	ctx, span := s.tracer.Start(ctx, "nats.handle_message",
		trace.WithAttributes(
			attribute.String("topic", natsMsg.Subject),
		))
	defer span.End()

	msg := &Message{
		ID:        natsMsg.Header.Get("Nats-Msg-Id"),
		Topic:     natsMsg.Subject,
		Data:      natsMsg.Data,
		Headers:   make(map[string]string),
		Timestamp: time.Now(),
	}

	// Copy headers
	for key := range natsMsg.Header {
		msg.Headers[key] = natsMsg.Header.Get(key)
	}

	err := handler(ctx, msg)
	if err != nil {
		natsSubscribeTotal.WithLabelValues(msg.Topic, "error").Inc()
		log.WithFields(log.Fields{
			"topic": msg.Topic,
			"error": err,
		}).Error("Message handler failed")

		// Send to dead-letter queue if configured
		if s.config.DeadLetterTopic != "" {
			_, dlqErr := s.js.Publish(s.config.DeadLetterTopic, natsMsg.Data)
			if dlqErr != nil {
				log.WithError(dlqErr).Error("Failed to send message to dead-letter queue")
			}
		}

		// Negative ack to retry
		if nakErr := natsMsg.Nak(); nakErr != nil {
			log.WithError(nakErr).Error("Failed to NAK message")
		}
		return
	}

	natsSubscribeTotal.WithLabelValues(msg.Topic, "success").Inc()
	if ackErr := natsMsg.Ack(); ackErr != nil {
		log.WithError(ackErr).Error("Failed to ACK message")
	}
}

// SubscribeMulti subscribes to multiple topics
func (s *NATSSubscriber) SubscribeMulti(ctx context.Context, topics []string, handler MessageHandler) error {
	for _, topic := range topics {
		if err := s.Subscribe(ctx, topic, handler); err != nil {
			return fmt.Errorf("subscribe to %s: %w", topic, err)
		}
	}
	return nil
}

// Unsubscribe unsubscribes from a topic
func (s *NATSSubscriber) Unsubscribe(topic string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, exists := s.subs[topic]
	if !exists {
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}

	if err := sub.Unsubscribe(); err != nil {
		return fmt.Errorf("unsubscribe: %w", err)
	}

	delete(s.subs, topic)
	return nil
}

// Close closes all subscriptions and the connection
func (s *NATSSubscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for topic, sub := range s.subs {
		if err := sub.Unsubscribe(); err != nil {
			log.WithError(err).WithField("topic", topic).Error("Failed to unsubscribe")
		}
	}

	if s.conn != nil {
		s.conn.Close()
	}

	return nil
}
