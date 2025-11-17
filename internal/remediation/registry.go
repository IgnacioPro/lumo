package remediation

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// Registry maintains a catalog of available remediation actions.
type Registry struct {
	actions map[string]ActionFactory
	mu      sync.RWMutex
	logger  *logrus.Logger
}

// ActionFactory is a function that creates a new Action instance.
type ActionFactory func(params map[string]interface{}, logger *logrus.Logger) (Action, error)

// NewRegistry creates a new action registry.
func NewRegistry(logger *logrus.Logger) *Registry {
	registry := &Registry{
		actions: make(map[string]ActionFactory),
		logger:  logger,
	}

	// Register built-in actions
	registry.registerBuiltInActions()

	return registry
}

// Register registers an action factory with the registry.
func (r *Registry) Register(actionID string, factory ActionFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.actions[actionID]; exists {
		return fmt.Errorf("action %s is already registered", actionID)
	}

	r.actions[actionID] = factory
	r.logger.WithField("action_id", actionID).Debug("Action registered")
	return nil
}

// Unregister removes an action from the registry.
func (r *Registry) Unregister(actionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.actions, actionID)
	r.logger.WithField("action_id", actionID).Debug("Action unregistered")
}

// Create creates a new action instance by ID with the given parameters.
func (r *Registry) Create(actionID string, params map[string]interface{}) (Action, error) {
	r.mu.RLock()
	factory, exists := r.actions[actionID]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("action %s is not registered", actionID)
	}

	action, err := factory(params, r.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create action %s: %w", actionID, err)
	}

	return action, nil
}

// List returns a list of all registered action IDs.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.actions))
	for id := range r.actions {
		ids = append(ids, id)
	}
	return ids
}

// ListByCategory returns action IDs filtered by category.
func (r *Registry) ListByCategory(category ActionCategory) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0)

	// We need to create temporary instances to check categories
	// This is inefficient but necessary without metadata storage
	for id, factory := range r.actions {
		action, err := factory(nil, r.logger)
		if err != nil {
			continue // Skip actions that can't be created without params
		}
		if action.Category() == category {
			ids = append(ids, id)
		}
	}

	return ids, nil
}

// GetActionInfo returns metadata about a registered action without creating it.
type ActionInfo struct {
	ID          string
	Name        string
	Description string
	Category    ActionCategory
	Risk        RiskLevel
	Reversible  bool
}

// GetInfo returns information about a registered action.
func (r *Registry) GetInfo(actionID string) (*ActionInfo, error) {
	r.mu.RLock()
	factory, exists := r.actions[actionID]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("action %s is not registered", actionID)
	}

	// Create a temporary instance to get metadata
	action, err := factory(nil, r.logger)
	if err != nil {
		// Try with empty params map
		action, err = factory(make(map[string]interface{}), r.logger)
		if err != nil {
			return nil, fmt.Errorf("failed to get info for action %s: %w", actionID, err)
		}
	}

	return &ActionInfo{
		ID:          action.ID(),
		Name:        action.Name(),
		Description: action.Description(),
		Category:    action.Category(),
		Risk:        action.Risk(),
		Reversible:  action.IsReversible(),
	}, nil
}

// registerBuiltInActions registers all built-in remediation actions.
func (r *Registry) registerBuiltInActions() {
	// Service actions
	r.Register("service.restart", NewRestartServiceActionFactory())
	r.Register("service.start", NewStartServiceActionFactory())
	r.Register("service.stop", NewStopServiceActionFactory())

	// Disk actions
	r.Register("disk.clean_logs", NewCleanLogsActionFactory())
	r.Register("disk.clean_temp", NewCleanTempActionFactory())
	r.Register("disk.clean_cache", NewCleanCacheActionFactory())
	r.Register("disk.clean_apt_cache", NewCleanAptCacheActionFactory())

	// Process actions
	r.Register("process.kill", NewKillProcessActionFactory())
	r.Register("process.kill_graceful", NewKillProcessGracefulActionFactory())

	// Network actions (placeholders for now)
	// r.Register("network.restart_interface", NewRestartNetworkInterfaceActionFactory())

	// System actions (placeholders for now)
	// r.Register("system.update_sysctl", NewUpdateSysctlActionFactory())
}

// DefaultRegistry is the global action registry instance.
var (
	defaultRegistry     *Registry
	defaultRegistryOnce sync.Once
)

// GetDefaultRegistry returns the global action registry.
func GetDefaultRegistry(logger *logrus.Logger) *Registry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewRegistry(logger)
	})
	return defaultRegistry
}
