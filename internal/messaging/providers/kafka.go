package providers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ignacio/lumo/internal/reliability"
)

var (
	kafkaPublishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lumo_kafka_publish_total",
		Help: "Total number of messages published to Kafka",
	}, []string{"topic", "status"})

	kafkaPublishDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lumo_kafka_publish_duration_seconds",
		Help:    "Duration of Kafka publish operations",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
	}, []string{"topic"})

	kafkaSubscribeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lumo_kafka_subscribe_total",
		Help: "Total number of messages received from Kafka",
	}, []string{"topic", "status"})
)

// KafkaProvider implements the Provider interface for Kafka
type KafkaProvider struct{}

// NewKafkaProvider creates a new Kafka provider
func NewKafkaProvider() *KafkaProvider {
	return &KafkaProvider{}
}

// NewPublisher creates a new Kafka publisher
func (p *KafkaProvider) NewPublisher(config *Config) (Publisher, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("no Kafka brokers configured")
	}

	transport := &kafka.Transport{
		DialTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	// TLS configuration if needed in future
	// if config.TLS {
	// 	transport.TLS = &tls.Config{}
	// }

	writer := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Topic:        config.EventTopic,
		Balancer:     &kafka.LeastBytes{},
		Compression:  kafka.Snappy,
		MaxAttempts:  config.MaxRetries,
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		Transport:    transport,
	}

	cb := reliability.NewCircuitBreaker("kafka-publisher")

	return &KafkaPublisher{
		writer:         writer,
		config:         config,
		circuitBreaker: cb,
		tracer:         otel.Tracer("messaging/kafka"),
	}, nil
}

// NewSubscriber creates a new Kafka subscriber
func (p *KafkaProvider) NewSubscriber(config *Config) (Subscriber, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("no Kafka brokers configured")
	}

	return &KafkaSubscriber{
		config:  config,
		readers: make(map[string]*kafka.Reader),
		tracer:  otel.Tracer("messaging/kafka"),
	}, nil
}

// Health checks the Kafka connection health
func (p *KafkaProvider) Health() error {
	return nil
}

// KafkaPublisher implements the Publisher interface for Kafka
type KafkaPublisher struct {
	writer         *kafka.Writer
	config         *Config
	circuitBreaker *reliability.CircuitBreaker
	tracer         trace.Tracer
}

// Publish sends a single message to Kafka
func (p *KafkaPublisher) Publish(ctx context.Context, topic string, message []byte) error {
	ctx, span := p.tracer.Start(ctx, "kafka.publish",
		trace.WithAttributes(
			attribute.String("topic", topic),
			attribute.Int("message_size", len(message)),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		kafkaPublishDuration.WithLabelValues(topic).Observe(duration)
	}()

	_, err := p.circuitBreaker.Execute(func() (interface{}, error) {
		kafkaMsg := kafka.Message{
			Topic: topic,
			Value: message,
			Time:  time.Now(),
		}
		return nil, p.writer.WriteMessages(ctx, kafkaMsg)
	})

	if err != nil {
		kafkaPublishTotal.WithLabelValues(topic, "error").Inc()
		span.RecordError(err)
		return fmt.Errorf("publish to Kafka: %w", err)
	}

	kafkaPublishTotal.WithLabelValues(topic, "success").Inc()
	return nil
}

// PublishBatch sends multiple messages to Kafka in a batch
func (p *KafkaPublisher) PublishBatch(ctx context.Context, topic string, messages [][]byte) error {
	ctx, span := p.tracer.Start(ctx, "kafka.publish_batch",
		trace.WithAttributes(
			attribute.String("topic", topic),
			attribute.Int("batch_size", len(messages)),
		))
	defer span.End()

	if len(messages) == 0 {
		return nil
	}

	kafkaMessages := make([]kafka.Message, len(messages))
	for i, msg := range messages {
		kafkaMessages[i] = kafka.Message{
			Topic: topic,
			Value: msg,
			Time:  time.Now(),
		}
	}

	_, err := p.circuitBreaker.Execute(func() (interface{}, error) {
		return nil, p.writer.WriteMessages(ctx, kafkaMessages...)
	})

	if err != nil {
		return fmt.Errorf("batch publish to Kafka: %w", err)
	}

	return nil
}

// Close closes the Kafka writer
func (p *KafkaPublisher) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}

// KafkaSubscriber implements the Subscriber interface for Kafka
type KafkaSubscriber struct {
	config  *Config
	readers map[string]*kafka.Reader
	mu      sync.Mutex
	tracer  trace.Tracer
}

// Subscribe subscribes to a topic
func (s *KafkaSubscriber) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.readers[topic]; exists {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}

	groupID := s.config.ConsumerGroup
	if groupID == "" {
		groupID = "lumo-api-consumer"
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        s.config.Brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
		MaxWait:        500 * time.Millisecond,
		ReadBackoffMin: 100 * time.Millisecond,
		ReadBackoffMax: 1 * time.Second,
	})

	s.readers[topic] = reader

	// Start consuming in a goroutine
	go s.consume(ctx, reader, handler)

	log.WithField("topic", topic).Info("Subscribed to Kafka topic")
	return nil
}

func (s *KafkaSubscriber) consume(ctx context.Context, reader *kafka.Reader, handler MessageHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			kafkaMsg, err := reader.FetchMessage(ctx)
			if err != nil {
				if err == context.Canceled {
					return
				}
				log.WithError(err).Error("Failed to fetch Kafka message")
				continue
			}

			s.handleMessage(ctx, kafkaMsg, reader, handler)
		}
	}
}

func (s *KafkaSubscriber) handleMessage(ctx context.Context, kafkaMsg kafka.Message, reader *kafka.Reader, handler MessageHandler) {
	ctx, span := s.tracer.Start(ctx, "kafka.handle_message",
		trace.WithAttributes(
			attribute.String("topic", kafkaMsg.Topic),
			attribute.Int("partition", kafkaMsg.Partition),
		))
	defer span.End()

	msg := &Message{
		ID:        fmt.Sprintf("%s-%d-%d", kafkaMsg.Topic, kafkaMsg.Partition, kafkaMsg.Offset),
		Topic:     kafkaMsg.Topic,
		Data:      kafkaMsg.Value,
		Headers:   make(map[string]string),
		Timestamp: kafkaMsg.Time,
	}

	// Copy headers
	for _, header := range kafkaMsg.Headers {
		msg.Headers[header.Key] = string(header.Value)
	}

	err := handler(ctx, msg)
	if err != nil {
		kafkaSubscribeTotal.WithLabelValues(msg.Topic, "error").Inc()
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

	kafkaSubscribeTotal.WithLabelValues(msg.Topic, "success").Inc()
	if err := reader.CommitMessages(ctx, kafkaMsg); err != nil {
		log.WithError(err).Error("Failed to commit Kafka message")
	}
}

func (s *KafkaSubscriber) sendToDLQ(ctx context.Context, msg *Message) {
	// Create a temporary writer for DLQ
	writer := &kafka.Writer{
		Addr:     kafka.TCP(s.config.Brokers...),
		Topic:    s.config.DeadLetterTopic,
		Balancer: &kafka.LeastBytes{},
	}
	defer func() { _ = writer.Close() }()

	dlqMsg := kafka.Message{
		Value: msg.Data,
		Time:  time.Now(),
	}

	if err := writer.WriteMessages(ctx, dlqMsg); err != nil {
		log.WithError(err).Error("Failed to send message to dead-letter queue")
	}
}

// SubscribeMulti subscribes to multiple topics
func (s *KafkaSubscriber) SubscribeMulti(ctx context.Context, topics []string, handler MessageHandler) error {
	for _, topic := range topics {
		if err := s.Subscribe(ctx, topic, handler); err != nil {
			return fmt.Errorf("subscribe to %s: %w", topic, err)
		}
	}
	return nil
}

// Unsubscribe unsubscribes from a topic
func (s *KafkaSubscriber) Unsubscribe(topic string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	reader, exists := s.readers[topic]
	if !exists {
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}

	if err := reader.Close(); err != nil {
		return fmt.Errorf("close reader: %w", err)
	}

	delete(s.readers, topic)
	return nil
}

// Close closes all readers
func (s *KafkaSubscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for topic, reader := range s.readers {
		if err := reader.Close(); err != nil {
			log.WithError(err).WithField("topic", topic).Error("Failed to close reader")
		}
	}

	return nil
}
