package notifications

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/sirupsen/logrus"
)

// Notifier orchestrates sending notifications to multiple providers
type Notifier struct {
	providers map[string]Provider
	routing   map[diagnostics.Severity][]string
	logger    *logrus.Logger
	mu        sync.RWMutex
}

// NotifierConfig contains configuration for the Notifier
type NotifierConfig struct {
	// Routing maps severity levels to provider names
	Routing map[diagnostics.Severity][]string

	// RetryEnabled enables retry logic for failed notifications
	RetryEnabled bool

	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// RetryDelay is the initial delay between retries (exponential backoff)
	RetryDelay time.Duration
}

// NewNotifier creates a new Notifier instance
func NewNotifier(logger *logrus.Logger) *Notifier {
	if logger == nil {
		logger = logrus.New()
	}

	return &Notifier{
		providers: make(map[string]Provider),
		routing:   make(map[diagnostics.Severity][]string),
		logger:    logger,
	}
}

// RegisterProvider adds a new notification provider
func (n *Notifier) RegisterProvider(provider Provider) error {
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	name := provider.Name()
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if _, exists := n.providers[name]; exists {
		return fmt.Errorf("provider %s is already registered", name)
	}

	n.providers[name] = provider
	n.logger.Debugf("Registered notification provider: %s", name)

	return nil
}

// UnregisterProvider removes a notification provider
func (n *Notifier) UnregisterProvider(name string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, exists := n.providers[name]; !exists {
		return fmt.Errorf("provider %s is not registered", name)
	}

	delete(n.providers, name)
	n.logger.Debugf("Unregistered notification provider: %s", name)

	return nil
}

// SetRouting configures which providers receive notifications for each severity level
func (n *Notifier) SetRouting(routing map[diagnostics.Severity][]string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.routing = routing
}

// Notify sends a notification to all providers configured for the message's severity level
func (n *Notifier) Notify(ctx context.Context, msg *Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	// Get providers for this severity level
	providers := n.getProvidersForSeverity(msg.Severity)
	if len(providers) == 0 {
		n.logger.Debugf("No providers configured for severity %s, skipping notification", msg.Severity)
		return nil
	}

	// Send to all providers concurrently
	return n.sendToProviders(ctx, msg, providers)
}

// NotifyProvider sends a notification to a specific provider, bypassing routing rules
func (n *Notifier) NotifyProvider(ctx context.Context, providerName string, msg *Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	n.mu.RLock()
	provider, exists := n.providers[providerName]
	n.mu.RUnlock()

	if !exists {
		return fmt.Errorf("provider %s is not registered", providerName)
	}

	return n.sendWithRetry(ctx, provider, msg)
}

// NotifyAll sends a notification to all registered providers, bypassing routing rules
func (n *Notifier) NotifyAll(ctx context.Context, msg *Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	n.mu.RLock()
	providers := make([]Provider, 0, len(n.providers))
	for _, provider := range n.providers {
		providers = append(providers, provider)
	}
	n.mu.RUnlock()

	if len(providers) == 0 {
		return fmt.Errorf("no providers registered")
	}

	return n.sendToProviders(ctx, msg, providers)
}

// ListProviders returns a list of all registered provider names
func (n *Notifier) ListProviders() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	names := make([]string, 0, len(n.providers))
	for name := range n.providers {
		names = append(names, name)
	}

	return names
}

// HealthCheck checks the health of all registered providers
func (n *Notifier) HealthCheck(ctx context.Context) map[string]error {
	n.mu.RLock()
	providers := make(map[string]Provider, len(n.providers))
	for name, provider := range n.providers {
		providers[name] = provider
	}
	n.mu.RUnlock()

	results := make(map[string]error, len(providers))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, provider := range providers {
		wg.Add(1)
		go func(name string, provider Provider) {
			defer wg.Done()

			err := provider.Health(ctx)

			mu.Lock()
			results[name] = err
			mu.Unlock()

			if err != nil {
				n.logger.Warnf("Health check failed for provider %s: %v", name, err)
			} else {
				n.logger.Debugf("Health check passed for provider %s", name)
			}
		}(name, provider)
	}

	wg.Wait()
	return results
}

// getProvidersForSeverity returns the providers that should receive notifications for a given severity
func (n *Notifier) getProvidersForSeverity(severity diagnostics.Severity) []Provider {
	n.mu.RLock()
	defer n.mu.RUnlock()

	providerNames, exists := n.routing[severity]
	if !exists || len(providerNames) == 0 {
		return nil
	}

	providers := make([]Provider, 0, len(providerNames))
	for _, name := range providerNames {
		if provider, ok := n.providers[name]; ok {
			providers = append(providers, provider)
		} else {
			n.logger.Warnf("Provider %s configured for severity %s but not registered", name, severity)
		}
	}

	return providers
}

// sendToProviders sends a message to multiple providers concurrently
func (n *Notifier) sendToProviders(ctx context.Context, msg *Message, providers []Provider) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(providers))

	for _, provider := range providers {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()

			if err := n.sendWithRetry(ctx, p, msg); err != nil {
				errChan <- fmt.Errorf("provider %s: %w", p.Name(), err)
			}
		}(provider)
	}

	wg.Wait()
	close(errChan)

	// Collect all errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("notification failed for %d/%d providers: %v", len(errors), len(providers), errors)
	}

	return nil
}

// sendWithRetry sends a notification with exponential backoff retry logic
func (n *Notifier) sendWithRetry(ctx context.Context, provider Provider, msg *Message) error {
	operation := func() error {
		start := time.Now()
		err := provider.Send(ctx, msg)
		duration := time.Since(start)

		if err != nil {
			n.logger.Warnf("Failed to send notification via %s (took %v): %v", provider.Name(), duration, err)
			return err
		}

		n.logger.Infof("Successfully sent notification via %s (took %v)", provider.Name(), duration)
		return nil
	}

	// Configure exponential backoff
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 2 * time.Second
	bo.MaxInterval = 30 * time.Second
	bo.MaxElapsedTime = 2 * time.Minute

	// Wrap with context
	boWithContext := backoff.WithContext(bo, ctx)

	// Retry with backoff
	return backoff.Retry(operation, backoff.WithMaxRetries(boWithContext, 3))
}
