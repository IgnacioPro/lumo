package providers

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/sirupsen/logrus"
)

// KafkaPublisher implements Publisher for Kafka
type KafkaPublisher struct {
	writer *kafka.Writer
	logger *logrus.Logger
	cfg    interface{}
}

// KafkaSubscriber implements Subscriber for Kafka
type KafkaSubscriber struct {
	readers map[string]*kafka.Reader
	logger  *logrus.Logger
	cfg     interface{}
}

// NewKafkaPublisher creates a new Kafka publisher
func NewKafkaPublisher(cfg interface{}, logger *logrus.Logger) (*KafkaPublisher, error) {
	msgCfg, ok := cfg.(configGetter)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	// Create writer config
	writerConfig := kafka.WriterConfig{
		Brokers:  []string{msgCfg.GetURL()},
		Balancer: &kafka.LeastBytes{},
		Async:    false, // Synchronous writes for reliability
	}

	// Configure TLS if enabled
	if msgCfg.IsTLSEnabled() {
		dialer := &kafka.Dialer{
			TLS: &tls.Config{
				InsecureSkipVerify: msgCfg.IsTLSInsecure(),
			},
		}

		// Add SASL authentication if credentials provided
		if msgCfg.GetUsername() != "" && msgCfg.GetPassword() != "" {
			dialer.SASLMechanism = plain.Mechanism{
				Username: msgCfg.GetUsername(),
				Password: msgCfg.GetPassword(),
			}
		}

		// Note: kafka-go Writer doesn't have a direct Dialer field
		// We'll need to configure this differently in production
	}

	writer := kafka.NewWriter(writerConfig)

	logger.WithField("brokers", msgCfg.GetURL()).Info("Connected to Kafka")

	return &KafkaPublisher{
		writer: writer,
		logger: logger,
		cfg:    cfg,
	}, nil
}

// Publish sends a message to Kafka
func (p *KafkaPublisher) Publish(ctx context.Context, msg *Message) error {
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	kafkaMsg := kafka.Message{
		Topic: msg.Topic,
		Key:   []byte(msg.ID),
		Value: data,
		Headers: []kafka.Header{
			{Key: "message-id", Value: []byte(msg.ID)},
			{Key: "agent-id", Value: []byte(msg.AgentID)},
		},
	}

	if err := p.writer.WriteMessages(ctx, kafkaMsg); err != nil {
		return fmt.Errorf("failed to publish to Kafka: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"topic":      msg.Topic,
		"message_id": msg.ID,
	}).Debug("Published message to Kafka")

	return nil
}

// PublishBatch sends multiple messages in a batch
func (p *KafkaPublisher) PublishBatch(ctx context.Context, messages []*Message) error {
	kafkaMsgs := make([]kafka.Message, 0, len(messages))

	for _, msg := range messages {
		data, err := msg.Marshal()
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}

		kafkaMsgs = append(kafkaMsgs, kafka.Message{
			Topic: msg.Topic,
			Key:   []byte(msg.ID),
			Value: data,
		})
	}

	if err := p.writer.WriteMessages(ctx, kafkaMsgs...); err != nil {
		return fmt.Errorf("failed to publish batch to Kafka: %w", err)
	}

	p.logger.WithField("count", len(messages)).Debug("Published batch to Kafka")
	return nil
}

// Close closes the Kafka writer
func (p *KafkaPublisher) Close() error {
	if p.writer != nil {
		if err := p.writer.Close(); err != nil {
			return fmt.Errorf("failed to close Kafka writer: %w", err)
		}
		p.logger.Info("Kafka publisher closed")
	}
	return nil
}

// NewKafkaSubscriber creates a new Kafka subscriber
func NewKafkaSubscriber(cfg interface{}, logger *logrus.Logger) (*KafkaSubscriber, error) {
	msgCfg, ok := cfg.(configGetter)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	logger.WithField("brokers", msgCfg.GetURL()).Info("Kafka subscriber initialized")

	return &KafkaSubscriber{
		readers: make(map[string]*kafka.Reader),
		logger:  logger,
		cfg:     cfg,
	}, nil
}

// Subscribe starts receiving messages from Kafka topics
func (s *KafkaSubscriber) Subscribe(ctx context.Context, topics []string, handler func(context.Context, *Message) error) error {
	for _, topic := range topics {
		msgCfg := s.cfg.(configGetter)

		readerConfig := kafka.ReaderConfig{
			Brokers:  []string{msgCfg.GetURL()},
			Topic:    topic,
			GroupID:  "lumo-agent",
			MinBytes: 10e3, // 10KB
			MaxBytes: 10e6, // 10MB
		}

		reader := kafka.NewReader(readerConfig)
		s.readers[topic] = reader

		// Start reading in a goroutine
		go func(t string, r *kafka.Reader) {
			for {
				msg, err := r.ReadMessage(ctx)
				if err != nil {
					s.logger.WithError(err).Error("Failed to read Kafka message")
					continue
				}

				// Parse and handle message
				s.logger.WithField("topic", t).Debug("Received message from Kafka")
				// TODO: Parse msg.Value and call handler
			}
		}(topic, reader)

		s.logger.WithField("topic", topic).Info("Subscribed to Kafka topic")
	}

	return nil
}

// Unsubscribe stops receiving messages from specified topics
func (s *KafkaSubscriber) Unsubscribe(topics []string) error {
	for _, topic := range topics {
		if reader, ok := s.readers[topic]; ok {
			if err := reader.Close(); err != nil {
				return fmt.Errorf("failed to close reader for topic %s: %w", topic, err)
			}
			delete(s.readers, topic)
			s.logger.WithField("topic", topic).Info("Unsubscribed from Kafka topic")
		}
	}
	return nil
}

// Close closes all Kafka readers
func (s *KafkaSubscriber) Close() error {
	for topic, reader := range s.readers {
		if err := reader.Close(); err != nil {
			s.logger.WithError(err).WithField("topic", topic).Error("Failed to close Kafka reader")
		}
	}
	s.readers = make(map[string]*kafka.Reader)
	s.logger.Info("Kafka subscriber closed")
	return nil
}
