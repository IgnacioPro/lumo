package eventdriven

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Manager handles the lifecycle of Kubernetes informers and event watchers
type Manager struct {
	clientset       *kubernetes.Clientset
	informerFactory informers.SharedInformerFactory
	watchers        []Watcher
	stopCh          chan struct{}
	wg              sync.WaitGroup
	logger          *logrus.Entry
	ctx             context.Context
	cancel          context.CancelFunc
	config          *Config
}

// Config holds configuration for the event-driven manager
type Config struct {
	// ResyncPeriod is how often to resync informer caches (0 = no resync)
	ResyncPeriod time.Duration
	// Namespaces to watch (empty = all namespaces)
	Namespaces []string
	// DebounceWindow is how long to wait before processing an event
	DebounceWindow time.Duration
	// MaxEventsPerMinute limits event processing rate
	MaxEventsPerMinute int
	// GroupRelatedEvents enables batching of related events
	GroupRelatedEvents bool
}

// Watcher represents a resource-specific event watcher
type Watcher interface {
	// Name returns the watcher name for logging
	Name() string
	// Setup configures the informer with event handlers
	Setup(factory informers.SharedInformerFactory, handler EventHandler) error
	// GetInformer returns the underlying informer
	GetInformer() cache.SharedIndexInformer
}

// EventHandler processes Kubernetes resource events
type EventHandler interface {
	// OnAdd is called when a resource is created
	OnAdd(obj interface{})
	// OnUpdate is called when a resource is updated
	OnUpdate(oldObj, newObj interface{})
	// OnDelete is called when a resource is deleted
	OnDelete(obj interface{})
	// HandleEvent processes a Kubernetes event
	HandleEvent(event *KubernetesEvent)
}

// NewManager creates a new event-driven manager
func NewManager(clientset *kubernetes.Clientset, config *Config, logger *logrus.Logger) (*Manager, error) {
	if clientset == nil {
		return nil, fmt.Errorf("clientset cannot be nil")
	}
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create shared informer factory
	// If ResyncPeriod is 0, informers will not resync (pure event-driven)
	var factory informers.SharedInformerFactory
	if len(config.Namespaces) == 0 {
		// Watch all namespaces
		factory = informers.NewSharedInformerFactory(clientset, config.ResyncPeriod)
	} else {
		// Watch specific namespaces (use first namespace for factory)
		// TODO: Support multiple namespace factories if needed
		factory = informers.NewSharedInformerFactoryWithOptions(
			clientset,
			config.ResyncPeriod,
			informers.WithNamespace(config.Namespaces[0]),
		)
	}

	return &Manager{
		clientset:       clientset,
		informerFactory: factory,
		watchers:        make([]Watcher, 0),
		stopCh:          make(chan struct{}),
		logger:          logger.WithField("component", "event-driven-manager"),
		ctx:             ctx,
		cancel:          cancel,
		config:          config,
	}, nil
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		ResyncPeriod:       0, // No resync - pure event-driven
		Namespaces:         []string{},
		DebounceWindow:     45 * time.Second,
		MaxEventsPerMinute: 100,
		GroupRelatedEvents: true,
	}
}

// AddWatcher registers a new resource watcher
func (m *Manager) AddWatcher(watcher Watcher, handler EventHandler) error {
	m.logger.WithField("watcher", watcher.Name()).Info("Registering watcher")

	if err := watcher.Setup(m.informerFactory, handler); err != nil {
		return fmt.Errorf("failed to setup watcher %s: %w", watcher.Name(), err)
	}

	m.watchers = append(m.watchers, watcher)
	return nil
}

// Start begins watching for Kubernetes events
func (m *Manager) Start() error {
	m.logger.Info("Starting event-driven manager")

	if len(m.watchers) == 0 {
		return fmt.Errorf("no watchers registered")
	}

	// Start informers
	m.logger.WithField("watchers", len(m.watchers)).Info("Starting informers")
	m.informerFactory.Start(m.stopCh)

	// Wait for caches to sync
	m.logger.Info("Waiting for informer caches to sync")
	synced := make(map[string]bool)
	for _, watcher := range m.watchers {
		informer := watcher.GetInformer()
		watcherName := watcher.Name()

		// Start a goroutine to wait for this informer to sync
		m.wg.Add(1)
		go func(name string, inf cache.SharedIndexInformer) {
			defer m.wg.Done()
			if !cache.WaitForCacheSync(m.stopCh, inf.HasSynced) {
				m.logger.WithField("watcher", name).Error("Failed to sync cache")
				synced[name] = false
				return
			}
			synced[name] = true
			m.logger.WithField("watcher", name).Info("Cache synced successfully")
		}(watcherName, informer)
	}

	// Wait for all sync operations to complete
	m.wg.Wait()

	// Check if all caches synced
	allSynced := true
	for name, success := range synced {
		if !success {
			m.logger.WithField("watcher", name).Error("Cache sync failed")
			allSynced = false
		}
	}

	if !allSynced {
		return fmt.Errorf("not all informer caches synced successfully")
	}

	m.logger.WithFields(logrus.Fields{
		"watchers":       len(m.watchers),
		"resync_period":  m.config.ResyncPeriod,
		"debounce":       m.config.DebounceWindow,
		"group_events":   m.config.GroupRelatedEvents,
	}).Info("Event-driven manager started successfully")

	return nil
}

// Stop gracefully shuts down the manager
func (m *Manager) Stop() error {
	m.logger.Info("Stopping event-driven manager")

	// Signal all informers to stop
	close(m.stopCh)

	// Cancel context
	m.cancel()

	// Wait for all goroutines to finish
	m.wg.Wait()

	m.logger.Info("Event-driven manager stopped")
	return nil
}

// IsRunning returns true if the manager is running
func (m *Manager) IsRunning() bool {
	select {
	case <-m.stopCh:
		return false
	default:
		return true
	}
}

// GetFactory returns the shared informer factory
func (m *Manager) GetFactory() informers.SharedInformerFactory {
	return m.informerFactory
}

// GetClientset returns the Kubernetes clientset
func (m *Manager) GetClientset() *kubernetes.Clientset {
	return m.clientset
}

// GetContext returns the manager's context
func (m *Manager) GetContext() context.Context {
	return m.ctx
}

// GetConfig returns the manager's configuration
func (m *Manager) GetConfig() *Config {
	return m.config
}

// WatcherCount returns the number of registered watchers
func (m *Manager) WatcherCount() int {
	return len(m.watchers)
}
