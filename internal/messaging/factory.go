package messaging

import (
	"fmt"

	"github.com/ignacio/lumo/internal/messaging/providers"
)

// NewProvider creates a messaging provider based on configuration
func NewProvider(config *Config) (Provider, error) {
	if config == nil {
		return nil, fmt.Errorf("messaging config is nil")
	}

	switch config.Provider {
	case "nats":
		return providers.NewNATSProvider(), nil
	case "kafka":
		return providers.NewKafkaProvider(), nil
	case "rabbitmq":
		return providers.NewRabbitMQProvider(), nil
	case "redis":
		return providers.NewRedisProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported messaging provider: %s", config.Provider)
	}
}
