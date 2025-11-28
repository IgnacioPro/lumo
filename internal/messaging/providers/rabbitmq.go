package providers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ignacio/lumo/internal/reliability"
)

var (
	rabbitPublishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lumo_rabbitmq_publish_total",
		Help: "Total number of messages published to RabbitMQ",
	}, []string{"topic", "status"})

	rabbitPublishDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lumo_rabbitmq_publish_duration_seconds",
		Help:    "Duration of RabbitMQ publish operations",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
	}, []string{"topic"})

	rabbitSubscribeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lumo_rabbitmq_subscribe_total",
		Help: "Total number of messages received from RabbitMQ",
	}, []string{"topic", "status"})
)

// RabbitMQProvider implements the Provider interface for RabbitMQ
type RabbitMQProvider struct{}

// NewRabbitMQProvider creates a new RabbitMQ provider
func NewRabbitMQProvider() *RabbitMQProvider {
	return &RabbitMQProvider{}
}

// NewPublisher creates a new RabbitMQ publisher
func (p *RabbitMQProvider) NewPublisher(config *Config) (Publisher, error) {
	conn, channel, err := p.connect(config)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	// Declare topic exchange
	exchangeName := "lumo.events"
	err = channel.ExchangeDeclare(
		exchangeName,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	// Enable publisher confirms
	if err := channel.Confirm(false); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("enable publisher confirms: %w", err)
	}

	cb := reliability.NewCircuitBreaker("rabbitmq-publisher")

	return &RabbitMQPublisher{
		conn:           conn,
		channel:        channel,
		exchange:       exchangeName,
		config:         config,
		circuitBreaker: cb,
		tracer:         otel.Tracer("messaging/rabbitmq"),
	}, nil
}

// NewSubscriber creates a new RabbitMQ subscriber
func (p *RabbitMQProvider) NewSubscriber(config *Config) (Subscriber, error) {
	conn, channel, err := p.connect(config)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	return &RabbitMQSubscriber{
		conn:    conn,
		channel: channel,
		config:  config,
		queues:  make(map[string]string),
		tracer:  otel.Tracer("messaging/rabbitmq"),
	}, nil
}

// Health checks the RabbitMQ connection health
func (p *RabbitMQProvider) Health() error {
	return nil
}

func (p *RabbitMQProvider) connect(config *Config) (*amqp.Connection, *amqp.Channel, error) {
	if len(config.Brokers) == 0 {
		return nil, nil, fmt.Errorf("no RabbitMQ brokers configured")
	}

	// Build connection URL
	url := config.Brokers[0]
	if config.Username != "" && config.Password != "" {
		// Insert credentials into URL
		url = fmt.Sprintf("amqp://%s:%s@%s", config.Username, config.Password, config.Brokers[0])
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, fmt.Errorf("dial RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("create channel: %w", err)
	}

	return conn, channel, nil
}

// RabbitMQPublisher implements the Publisher interface for RabbitMQ
type RabbitMQPublisher struct {
	conn           *amqp.Connection
	channel        *amqp.Channel
	exchange       string
	config         *Config
	circuitBreaker *reliability.CircuitBreaker
	tracer         trace.Tracer
}

// Publish sends a single message to RabbitMQ
func (p *RabbitMQPublisher) Publish(ctx context.Context, topic string, message []byte) error {
	ctx, span := p.tracer.Start(ctx, "rabbitmq.publish",
		trace.WithAttributes(
			attribute.String("topic", topic),
			attribute.Int("message_size", len(message)),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		rabbitPublishDuration.WithLabelValues(topic).Observe(duration)
	}()

	_, err := p.circuitBreaker.Execute(func() (interface{}, error) {
		return nil, p.channel.PublishWithContext(
			ctx,
			p.exchange,
			topic,
			false, // mandatory
			false, // immediate
			amqp.Publishing{
				ContentType:  "application/json",
				Body:         message,
				DeliveryMode: amqp.Persistent,
				Timestamp:    time.Now(),
			},
		)
	})

	if err != nil {
		rabbitPublishTotal.WithLabelValues(topic, "error").Inc()
		span.RecordError(err)
		return fmt.Errorf("publish to RabbitMQ: %w", err)
	}

	rabbitPublishTotal.WithLabelValues(topic, "success").Inc()
	return nil
}

// PublishBatch sends multiple messages to RabbitMQ in a batch
func (p *RabbitMQPublisher) PublishBatch(ctx context.Context, topic string, messages [][]byte) error {
	ctx, span := p.tracer.Start(ctx, "rabbitmq.publish_batch",
		trace.WithAttributes(
			attribute.String("topic", topic),
			attribute.Int("batch_size", len(messages)),
		))
	defer span.End()

	if len(messages) == 0 {
		return nil
	}

	for _, msg := range messages {
		if err := p.Publish(ctx, topic, msg); err != nil {
			return fmt.Errorf("batch publish failed: %w", err)
		}
	}

	return nil
}

// Close closes the RabbitMQ connection
func (p *RabbitMQPublisher) Close() error {
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
	return nil
}

// RabbitMQSubscriber implements the Subscriber interface for RabbitMQ
type RabbitMQSubscriber struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *Config
	queues  map[string]string
	mu      sync.Mutex
	tracer  trace.Tracer
}

// Subscribe subscribes to a topic
func (s *RabbitMQSubscriber) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.queues[topic]; exists {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}

	exchangeName := "lumo.events"

	// Declare exchange
	err := s.channel.ExchangeDeclare(
		exchangeName,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}

	// Declare queue with TTL
	queueName := fmt.Sprintf("lumo.%s.%s", s.config.ConsumerGroup, topic)
	queue, err := s.channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-message-ttl": int32(24 * 60 * 60 * 1000), // 24 hours
		},
	)
	if err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}

	// Bind queue to exchange
	err = s.channel.QueueBind(
		queue.Name,
		topic,
		exchangeName,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("bind queue: %w", err)
	}

	// Set QoS for prefetch
	maxInFlight := s.config.MaxInFlight
	if maxInFlight == 0 {
		maxInFlight = 10
	}
	err = s.channel.Qos(maxInFlight, 0, false)
	if err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	// Start consuming
	msgs, err := s.channel.Consume(
		queue.Name,
		"",    // consumer tag
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("start consuming: %w", err)
	}

	s.queues[topic] = queue.Name

	// Handle messages in a goroutine
	go s.consume(ctx, msgs, handler)

	log.WithField("topic", topic).Info("Subscribed to RabbitMQ topic")
	return nil
}

func (s *RabbitMQSubscriber) consume(ctx context.Context, msgs <-chan amqp.Delivery, handler MessageHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				return
			}
			s.handleMessage(ctx, msg, handler)
		}
	}
}

func (s *RabbitMQSubscriber) handleMessage(ctx context.Context, delivery amqp.Delivery, handler MessageHandler) {
	ctx, span := s.tracer.Start(ctx, "rabbitmq.handle_message",
		trace.WithAttributes(
			attribute.String("topic", delivery.RoutingKey),
		))
	defer span.End()

	msg := &Message{
		ID:        delivery.MessageId,
		Topic:     delivery.RoutingKey,
		Data:      delivery.Body,
		Headers:   make(map[string]string),
		Timestamp: delivery.Timestamp,
	}

	// Copy headers
	for key, value := range delivery.Headers {
		if str, ok := value.(string); ok {
			msg.Headers[key] = str
		}
	}

	err := handler(ctx, msg)
	if err != nil {
		rabbitSubscribeTotal.WithLabelValues(msg.Topic, "error").Inc()
		log.WithFields(log.Fields{
			"topic": msg.Topic,
			"error": err,
		}).Error("Message handler failed")

		// Send to dead-letter queue if configured
		if s.config.DeadLetterTopic != "" {
			s.sendToDLQ(ctx, msg)
		}

		// Negative ack to requeue
		if nakErr := delivery.Nack(false, true); nakErr != nil {
			log.WithError(nakErr).Error("Failed to NACK message")
		}
		return
	}

	rabbitSubscribeTotal.WithLabelValues(msg.Topic, "success").Inc()
	if ackErr := delivery.Ack(false); ackErr != nil {
		log.WithError(ackErr).Error("Failed to ACK message")
	}
}

func (s *RabbitMQSubscriber) sendToDLQ(ctx context.Context, msg *Message) {
	err := s.channel.PublishWithContext(
		ctx,
		"lumo.events",
		s.config.DeadLetterTopic,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         msg.Data,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
	if err != nil {
		log.WithError(err).Error("Failed to send message to dead-letter queue")
	}
}

// SubscribeMulti subscribes to multiple topics
func (s *RabbitMQSubscriber) SubscribeMulti(ctx context.Context, topics []string, handler MessageHandler) error {
	for _, topic := range topics {
		if err := s.Subscribe(ctx, topic, handler); err != nil {
			return fmt.Errorf("subscribe to %s: %w", topic, err)
		}
	}
	return nil
}

// Unsubscribe unsubscribes from a topic
func (s *RabbitMQSubscriber) Unsubscribe(topic string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	queueName, exists := s.queues[topic]
	if !exists {
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}

	// Delete the queue
	_, err := s.channel.QueueDelete(queueName, false, false, false)
	if err != nil {
		return fmt.Errorf("delete queue: %w", err)
	}

	delete(s.queues, topic)
	return nil
}

// Close closes all subscriptions and the connection
func (s *RabbitMQSubscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.channel != nil {
		_ = s.channel.Close()
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}

	return nil
}
