package eventdriven

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// Debouncer handles event debouncing with state tracking
type Debouncer struct {
	cache             *redis.Client
	logger            *logrus.Entry
	debounceWindow    time.Duration
	maxDebounceWindow time.Duration
	mu                sync.RWMutex
	timers            map[string]*time.Timer
	callbacks         map[string]func(*KubernetesEvent)
	ctx               context.Context
}

// DebouncerConfig holds configuration for the debouncer
type DebouncerConfig struct {
	DebounceWindow    time.Duration
	MaxDebounceWindow time.Duration // Maximum time to wait before processing (even if events keep coming)
	RedisClient       *redis.Client
}

const (
	// Redis key prefixes
	redisEventPrefix     = "lumo:event-driven:event:"
	redisSeenPrefix      = "lumo:event-driven:seen:"
	redisTimestampPrefix = "lumo:event-driven:timestamp:"
	redisCountPrefix     = "lumo:event-driven:count:"

	// TTL for Redis keys
	eventStateTTL = 24 * time.Hour
)

// NewDebouncer creates a new event debouncer.
//
// Redis Client Lifecycle:
// The debouncer stores a reference to the Redis client provided in config but does NOT
// take ownership of it. The caller is responsible for closing the Redis client when the
// agent shuts down. The debouncer's Shutdown() method only clears pending timers and
// does not close the Redis connection.
func NewDebouncer(ctx context.Context, config *DebouncerConfig, logger *logrus.Logger) (*Debouncer, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if config == nil {
		return nil, fmt.Errorf("debouncer config cannot be nil")
	}
	if config.RedisClient == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}
	if config.DebounceWindow == 0 {
		config.DebounceWindow = 45 * time.Second // Default
	}
	if config.MaxDebounceWindow == 0 {
		config.MaxDebounceWindow = 3 * time.Minute // Default: max 3 minutes even if events keep coming
	}

	return &Debouncer{
		cache:             config.RedisClient,
		logger:            logger.WithField("component", "debouncer"),
		debounceWindow:    config.DebounceWindow,
		maxDebounceWindow: config.MaxDebounceWindow,
		timers:            make(map[string]*time.Timer),
		callbacks:         make(map[string]func(*KubernetesEvent)),
		ctx:               ctx,
	}, nil
}

// Debounce processes an event with debouncing logic
func (d *Debouncer) Debounce(event *KubernetesEvent, callback func(*KubernetesEvent)) error {
	eventKey := event.EventKey()

	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if we've seen this event before
	seen, err := d.hasSeenEvent(eventKey)
	if err != nil {
		d.logger.WithError(err).WithField("event_key", eventKey).Error("Failed to check if event was seen")
		return err
	}

	// Get event count
	count, err := d.incrementEventCount(eventKey)
	if err != nil {
		d.logger.WithError(err).WithField("event_key", eventKey).Error("Failed to increment event count")
		return err
	}
	event.Count = count

	// Update first/last seen timestamps
	if !seen {
		event.FirstSeen = event.Timestamp
		if err := d.setFirstSeen(eventKey, event.Timestamp); err != nil {
			d.logger.WithError(err).Error("Failed to set first seen timestamp")
		}
	} else {
		firstSeen, err := d.getFirstSeen(eventKey)
		if err == nil {
			event.FirstSeen = firstSeen
		}
	}
	event.LastSeen = event.Timestamp

	// Check if we've exceeded the maximum debounce window
	// This prevents infinite debouncing for events that keep repeating
	timeSinceFirstSeen := event.Timestamp.Sub(event.FirstSeen)
	if seen && timeSinceFirstSeen >= d.maxDebounceWindow {
		d.logger.WithFields(logrus.Fields{
			"event_key":           eventKey,
			"event_type":          event.Type,
			"resource":            event.ResourceKind + "/" + event.ResourceName,
			"namespace":           event.ResourceNamespace,
			"time_since_first":    timeSinceFirstSeen,
			"max_debounce_window": d.maxDebounceWindow,
			"count":               count,
		}).Info("Maximum debounce window exceeded - processing event immediately")

		// Store event in Redis first
		if err := d.storeEvent(eventKey, event); err != nil {
			d.logger.WithError(err).Error("Failed to store event in Redis")
			return err
		}

		// Cancel existing timer if present
		if timer, exists := d.timers[eventKey]; exists {
			timer.Stop()
			delete(d.timers, eventKey)
			delete(d.callbacks, eventKey)
		}

		// Process immediately
		go func() {
			d.processDebounced(eventKey, callback)
		}()

		return nil
	}

	// Cancel existing timer if present
	if timer, exists := d.timers[eventKey]; exists {
		timer.Stop()
		d.logger.WithFields(logrus.Fields{
			"event_key":  eventKey,
			"event_type": event.Type,
			"count":      count,
		}).Debug("Resetting debounce timer (event seen again)")
	}

	// Store event in Redis
	if err := d.storeEvent(eventKey, event); err != nil {
		d.logger.WithError(err).Error("Failed to store event in Redis")
		return err
	}

	// Create new timer with context awareness
	timer := time.AfterFunc(d.debounceWindow, func() {
		// Check if context is cancelled before processing
		select {
		case <-d.ctx.Done():
			d.logger.Debug("Skipping debounced processing - context cancelled")
			return
		default:
			d.processDebounced(eventKey, callback)
		}
	})

	d.timers[eventKey] = timer
	d.callbacks[eventKey] = callback

	d.logger.WithFields(logrus.Fields{
		"event_key":       eventKey,
		"event_type":      event.Type,
		"resource":        event.ResourceKind + "/" + event.ResourceName,
		"namespace":       event.ResourceNamespace,
		"debounce_window": d.debounceWindow,
		"seen_before":     seen,
		"count":           count,
	}).Debug("Event debounce timer started")

	return nil
}

// processDebounced is called after the debounce window expires
func (d *Debouncer) processDebounced(eventKey string, callback func(*KubernetesEvent)) {
	d.mu.Lock()
	// Clean up timer and callback
	delete(d.timers, eventKey)
	delete(d.callbacks, eventKey)
	d.mu.Unlock()

	// Retrieve event from Redis
	event, err := d.getEvent(eventKey)
	if err != nil {
		d.logger.WithError(err).WithField("event_key", eventKey).Error("Failed to retrieve event from Redis")
		return
	}

	if event == nil {
		d.logger.WithField("event_key", eventKey).Warn("Event not found in Redis after debounce")
		return
	}

	d.logger.WithFields(logrus.Fields{
		"event_key":  eventKey,
		"event_type": event.Type,
		"severity":   event.Severity,
		"resource":   event.ResourceKind + "/" + event.ResourceName,
		"namespace":  event.ResourceNamespace,
		"count":      event.Count,
		"first_seen": event.FirstSeen,
		"last_seen":  event.LastSeen,
	}).Info("Debounce window expired, processing event")

	// Mark as seen to prevent duplicate processing
	if err := d.markEventSeen(eventKey); err != nil {
		d.logger.WithError(err).Error("Failed to mark event as seen")
	}

	// Call the callback
	callback(event)
}

// hasSeenEvent checks if an event has been seen before
func (d *Debouncer) hasSeenEvent(eventKey string) (bool, error) {
	key := redisSeenPrefix + eventKey
	result, err := d.cache.Exists(d.ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// markEventSeen marks an event as processed
func (d *Debouncer) markEventSeen(eventKey string) error {
	key := redisSeenPrefix + eventKey
	return d.cache.Set(d.ctx, key, time.Now().Unix(), eventStateTTL).Err()
}

// incrementEventCount increments the event count
func (d *Debouncer) incrementEventCount(eventKey string) (int32, error) {
	key := redisCountPrefix + eventKey
	count, err := d.cache.Incr(d.ctx, key).Result()
	if err != nil {
		return 0, err
	}

	// Set TTL on first increment
	if count == 1 {
		d.cache.Expire(d.ctx, key, eventStateTTL)
	}

	return int32(count), nil
}

// setFirstSeen stores the first seen timestamp
func (d *Debouncer) setFirstSeen(eventKey string, timestamp time.Time) error {
	key := redisTimestampPrefix + eventKey
	return d.cache.Set(d.ctx, key, timestamp.Unix(), eventStateTTL).Err()
}

// getFirstSeen retrieves the first seen timestamp
func (d *Debouncer) getFirstSeen(eventKey string) (time.Time, error) {
	key := redisTimestampPrefix + eventKey
	timestamp, err := d.cache.Get(d.ctx, key).Int64()
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(timestamp, 0), nil
}

// storeEvent stores event data in Redis
func (d *Debouncer) storeEvent(eventKey string, event *KubernetesEvent) error {
	key := redisEventPrefix + eventKey
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	return d.cache.Set(d.ctx, key, data, eventStateTTL).Err()
}

// getEvent retrieves event data from Redis
func (d *Debouncer) getEvent(eventKey string) (*KubernetesEvent, error) {
	key := redisEventPrefix + eventKey
	data, err := d.cache.Get(d.ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var event KubernetesEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}

	return &event, nil
}

// CancelPending cancels a pending debounce timer
func (d *Debouncer) CancelPending(eventKey string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if timer, exists := d.timers[eventKey]; exists {
		timer.Stop()
		delete(d.timers, eventKey)
		delete(d.callbacks, eventKey)
		d.logger.WithField("event_key", eventKey).Debug("Cancelled pending debounce timer")
	}
}

// PendingCount returns the number of events waiting in debounce window
func (d *Debouncer) PendingCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.timers)
}

// Clear cancels all pending timers and clears state
func (d *Debouncer) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel all timers
	for _, timer := range d.timers {
		timer.Stop()
	}

	// Clear maps
	d.timers = make(map[string]*time.Timer)
	d.callbacks = make(map[string]func(*KubernetesEvent))

	d.logger.Info("Cleared all pending debounce timers")
}

// Shutdown gracefully shuts down the debouncer.
// Note: This method only clears pending timers. The Redis client lifecycle
// is managed by the caller and must be closed separately.
func (d *Debouncer) Shutdown() {
	d.logger.Info("Shutting down debouncer")
	d.Clear()
}
