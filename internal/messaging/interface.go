package messaging

import (
	"github.com/ignacio/lumo/internal/messaging/providers"
)

// Re-export types from providers package for convenience
type (
	Publisher      = providers.Publisher
	Subscriber     = providers.Subscriber
	MessageHandler = providers.MessageHandler
	Message        = providers.Message
	Config         = providers.Config
	Provider       = providers.Provider
)
