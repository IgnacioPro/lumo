package providers

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

// NATSPublisher implements Publisher for NATS
type NATSPublisher struct {
	conn   *nats.Conn
	logger *logrus.Logger
	cfg    *Config
}

// NATSSubscriber implements Subscriber for NATS
type NATSSubscriber struct {
	conn          *nats.Conn
	logger        *logrus.Logger
	cfg           *Config
	subscriptions map[string]*nats.Subscription
}

// Config is a placeholder - will use messaging.Config
type Config struct {
	Enabled  bool
	Provider string
	URL      string
	Username string
	Password string
	TLS      struct {
		Enabled            bool
		CertFile           string
		KeyFile            string
		CAFile             string
		InsecureSkipVerify bool
	}
	Retry struct {
		MaxAttempts int
		BaseDelay   time.Duration
		MaxDelay    time.Duration
	}
	DLQ struct {
		Enabled bool
		Topic   string
		MaxSize int
	}
	Timeout time.Duration
	Options map[string]interface{}
}

// Message represents a message to be published
type Message struct {
	ID        string                 `json:"id"`
	Topic     string                 `json:"topic"`
	Timestamp time.Time              `json:"timestamp"`
	AgentID   string                 `json:"agent_id,omitempty"`
	Hostname  string                 `json:"hostname,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
}

// Marshal serializes the message
func (m *Message) Marshal() ([]byte, error) {
	// This will be implemented in the parent package
	return nil, nil
}

// NewNATSPublisher creates a new NATS publisher
func NewNATSPublisher(cfg interface{}, logger *logrus.Logger) (*NATSPublisher, error) {
	// Type assertion to get the actual config
	// This is a simplified version - actual implementation will use messaging.Config
	opts := []nats.Option{
		nats.Name("lumo-agent-publisher"),
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.ReconnectWait(2 * time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				logger.WithError(err).Warn("NATS disconnected")
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected")
		}),
	}

	// Parse config from interface{}
	msgCfg, ok := cfg.(configGetter)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	// Add authentication if provided
	if msgCfg.GetUsername() != "" && msgCfg.GetPassword() != "" {
		opts = append(opts, nats.UserInfo(msgCfg.GetUsername(), msgCfg.GetPassword()))
	}

	// Add TLS if enabled
	if msgCfg.IsTLSEnabled() {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: msgCfg.IsTLSInsecure(),
		}
		opts = append(opts, nats.Secure(tlsConfig))
	}

	// Connect to NATS
	conn, err := nats.Connect(msgCfg.GetURL(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	logger.WithField("url", msgCfg.GetURL()).Info("Connected to NATS")

	return &NATSPublisher{
		conn:   conn,
		logger: logger,
	}, nil
}

// Publish sends a message to NATS
func (p *NATSPublisher) Publish(ctx context.Context, msg *Message) error {
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Convert topic to string
	topic := msg.Topic

	if err := p.conn.Publish(topic, data); err != nil {
		return fmt.Errorf("failed to publish to NATS: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"topic":      topic,
		"message_id": msg.ID,
	}).Debug("Published message to NATS")

	return nil
}

// PublishBatch sends multiple messages in a batch
func (p *NATSPublisher) PublishBatch(ctx context.Context, messages []*Message) error {
	for _, msg := range messages {
		if err := p.Publish(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the NATS connection
func (p *NATSPublisher) Close() error {
	if p.conn != nil {
		p.conn.Close()
		p.logger.Info("NATS publisher closed")
	}
	return nil
}

// NewNATSSubscriber creates a new NATS subscriber
func NewNATSSubscriber(cfg interface{}, logger *logrus.Logger) (*NATSSubscriber, error) {
	opts := []nats.Option{
		nats.Name("lumo-agent-subscriber"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
	}

	msgCfg, ok := cfg.(configGetter)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	if msgCfg.GetUsername() != "" && msgCfg.GetPassword() != "" {
		opts = append(opts, nats.UserInfo(msgCfg.GetUsername(), msgCfg.GetPassword()))
	}

	if msgCfg.IsTLSEnabled() {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: msgCfg.IsTLSInsecure(),
		}
		opts = append(opts, nats.Secure(tlsConfig))
	}

	conn, err := nats.Connect(msgCfg.GetURL(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &NATSSubscriber{
		conn:          conn,
		logger:        logger,
		subscriptions: make(map[string]*nats.Subscription),
	}, nil
}

// Subscribe starts receiving messages
func (s *NATSSubscriber) Subscribe(ctx context.Context, topics []string, handler func(context.Context, *Message) error) error {
	for _, topic := range topics {
		sub, err := s.conn.Subscribe(topic, func(msg *nats.Msg) {
			// Parse message
			// Call handler
			s.logger.WithField("topic", topic).Debug("Received message")
		})
		if err != nil {
			return fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
		}
		s.subscriptions[topic] = sub
	}
	return nil
}

// Unsubscribe stops receiving messages
func (s *NATSSubscriber) Unsubscribe(topics []string) error {
	for _, topic := range topics {
		if sub, ok := s.subscriptions[topic]; ok {
			if err := sub.Unsubscribe(); err != nil {
				return fmt.Errorf("failed to unsubscribe from topic %s: %w", topic, err)
			}
			delete(s.subscriptions, topic)
		}
	}
	return nil
}

// Close closes the NATS connection
func (s *NATSSubscriber) Close() error {
	for topic := range s.subscriptions {
		s.Unsubscribe([]string{topic})
	}
	if s.conn != nil {
		s.conn.Close()
	}
	return nil
}

// configGetter is an interface to extract config values
type configGetter interface {
	GetURL() string
	GetUsername() string
	GetPassword() string
	IsTLSEnabled() bool
	IsTLSInsecure() bool
}
