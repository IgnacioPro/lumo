package providers

import (
	"context"
	"crypto/tls"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// RabbitMQPublisher implements Publisher for RabbitMQ
type RabbitMQPublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	logger  *logrus.Logger
	cfg     interface{}
}

// RabbitMQSubscriber implements Subscriber for RabbitMQ
type RabbitMQSubscriber struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	logger  *logrus.Logger
	cfg     interface{}
	queues  map[string]string // topic -> queue name
}

// NewRabbitMQPublisher creates a new RabbitMQ publisher
func NewRabbitMQPublisher(cfg interface{}, logger *logrus.Logger) (*RabbitMQPublisher, error) {
	msgCfg, ok := cfg.(configGetter)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	// Build connection URL
	connURL := msgCfg.GetURL()

	// Configure TLS if enabled
	var amqpConfig amqp.Config
	if msgCfg.IsTLSEnabled() {
		amqpConfig.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: msgCfg.IsTLSInsecure(),
		}
	}

	// Connect to RabbitMQ
	conn, err := amqp.DialConfig(connURL, amqpConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create channel
	channel, err := conn.Channel()
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.WithError(closeErr).Warn("Failed to close RabbitMQ connection after channel creation failure")
		}
		return nil, fmt.Errorf("failed to create RabbitMQ channel: %w", err)
	}

	logger.WithField("url", connURL).Info("Connected to RabbitMQ")

	return &RabbitMQPublisher{
		conn:    conn,
		channel: channel,
		logger:  logger,
		cfg:     cfg,
	}, nil
}

// Publish sends a message to RabbitMQ
func (p *RabbitMQPublisher) Publish(ctx context.Context, msg *Message) error {
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Declare exchange (topic-based routing)
	exchange := "lumo"
	if err := p.channel.ExchangeDeclare(
		exchange,
		"topic", // type
		true,    // durable
		false,   // auto-deleted
		false,   // internal
		false,   // no-wait
		nil,     // arguments
	); err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Publish message
	if err := p.channel.PublishWithContext(
		ctx,
		exchange,  // exchange
		msg.Topic, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    msg.ID,
			Timestamp:    msg.Timestamp,
			Body:         data,
			Headers: amqp.Table{
				"agent_id": msg.AgentID,
				"hostname": msg.Hostname,
			},
		},
	); err != nil {
		return fmt.Errorf("failed to publish to RabbitMQ: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"topic":      msg.Topic,
		"message_id": msg.ID,
	}).Debug("Published message to RabbitMQ")

	return nil
}

// PublishBatch sends multiple messages in a batch
func (p *RabbitMQPublisher) PublishBatch(ctx context.Context, messages []*Message) error {
	for _, msg := range messages {
		if err := p.Publish(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the RabbitMQ connection
func (p *RabbitMQPublisher) Close() error {
	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			p.logger.WithError(err).Warn("Failed to close RabbitMQ channel")
		}
	}
	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			return fmt.Errorf("failed to close RabbitMQ connection: %w", err)
		}
		p.logger.Info("RabbitMQ publisher closed")
	}
	return nil
}

// NewRabbitMQSubscriber creates a new RabbitMQ subscriber
func NewRabbitMQSubscriber(cfg interface{}, logger *logrus.Logger) (*RabbitMQSubscriber, error) {
	msgCfg, ok := cfg.(configGetter)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	connURL := msgCfg.GetURL()

	var amqpConfig amqp.Config
	if msgCfg.IsTLSEnabled() {
		amqpConfig.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: msgCfg.IsTLSInsecure(),
		}
	}

	conn, err := amqp.DialConfig(connURL, amqpConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.WithError(closeErr).Warn("Failed to close RabbitMQ connection after channel creation failure")
		}
		return nil, fmt.Errorf("failed to create RabbitMQ channel: %w", err)
	}

	logger.WithField("url", connURL).Info("RabbitMQ subscriber initialized")

	return &RabbitMQSubscriber{
		conn:    conn,
		channel: channel,
		logger:  logger,
		cfg:     cfg,
		queues:  make(map[string]string),
	}, nil
}

// Subscribe starts receiving messages from RabbitMQ topics
func (s *RabbitMQSubscriber) Subscribe(ctx context.Context, topics []string, handler func(context.Context, *Message) error) error {
	exchange := "lumo"

	// Declare exchange
	if err := s.channel.ExchangeDeclare(
		exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	for _, topic := range topics {
		// Declare queue
		queue, err := s.channel.QueueDeclare(
			"",    // name (auto-generated)
			false, // durable
			true,  // delete when unused
			true,  // exclusive
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to declare queue: %w", err)
		}

		// Bind queue to exchange with routing key (topic)
		if err := s.channel.QueueBind(
			queue.Name, // queue name
			topic,      // routing key
			exchange,   // exchange
			false,      // no-wait
			nil,        // arguments
		); err != nil {
			return fmt.Errorf("failed to bind queue: %w", err)
		}

		s.queues[topic] = queue.Name

		// Start consuming
		msgs, err := s.channel.Consume(
			queue.Name,
			"",    // consumer tag
			true,  // auto-ack
			false, // exclusive
			false, // no-local
			false, // no-wait
			nil,   // args
		)
		if err != nil {
			return fmt.Errorf("failed to start consuming: %w", err)
		}

		// Handle messages in goroutine
		go func(t string, deliveries <-chan amqp.Delivery) {
			for d := range deliveries {
				s.logger.WithFields(map[string]interface{}{
					"topic":         t,
					"delivery_tag":  d.DeliveryTag,
					"message_count": d.MessageCount,
				}).Debug("Received message from RabbitMQ")
				// TODO: Parse d.Body and call handler
				_ = d // Placeholder until handler is implemented
			}
		}(topic, msgs)

		s.logger.WithField("topic", topic).Info("Subscribed to RabbitMQ topic")
	}

	return nil
}

// Unsubscribe stops receiving messages from specified topics
func (s *RabbitMQSubscriber) Unsubscribe(topics []string) error {
	for _, topic := range topics {
		if queueName, ok := s.queues[topic]; ok {
			// Delete queue
			if _, err := s.channel.QueueDelete(queueName, false, false, false); err != nil {
				return fmt.Errorf("failed to delete queue for topic %s: %w", topic, err)
			}
			delete(s.queues, topic)
			s.logger.WithField("topic", topic).Info("Unsubscribed from RabbitMQ topic")
		}
	}
	return nil
}

// Close closes the RabbitMQ connection
func (s *RabbitMQSubscriber) Close() error {
	if s.channel != nil {
		if err := s.channel.Close(); err != nil {
			s.logger.WithError(err).Warn("Failed to close RabbitMQ channel")
		}
	}
	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			return fmt.Errorf("failed to close RabbitMQ connection: %w", err)
		}
		s.logger.Info("RabbitMQ subscriber closed")
	}
	return nil
}
