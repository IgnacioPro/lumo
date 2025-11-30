package eventdriven

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/config"
)

var (
	eventsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_events_processed_total",
			Help: "Total number of Kubernetes events processed",
		},
		[]string{"event_type", "severity", "namespace"},
	)

	eventProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lumo_event_processing_duration_seconds",
			Help:    "Duration of event processing",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)
)

// APIEventProcessor submits events to the API server instead of processing them locally
type APIEventProcessor struct {
	ctx         context.Context
	logger      *logrus.Entry
	config      *config.Config
	agentID     uuid.UUID
	apiEndpoint string
	apiToken    string
	httpClient  *http.Client
	redisClient *redis.Client
	retryCache  *RetryCache
}

// APIProcessorConfig holds configuration for the API event processor
type APIProcessorConfig struct {
	Context     context.Context
	Config      *config.Config
	AgentID     uuid.UUID
	APIEndpoint string
	APIToken    string
	RedisClient *redis.Client
}

// NewAPIEventProcessor creates a new API event processor.
//
// Redis Client Lifecycle:
// The processor stores a reference to the Redis client provided in config but does NOT
// take ownership of it. The caller is responsible for closing the Redis client when the
// agent shuts down.
func NewAPIEventProcessor(cfg *APIProcessorConfig, logger *logrus.Logger) (*APIEventProcessor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("processor config cannot be nil")
	}
	if cfg.Context == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if cfg.APIEndpoint == "" {
		return nil, fmt.Errorf("API endpoint is required")
	}
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("API token is required")
	}

	// Create HTTP client with reasonable timeouts
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	// Create retry cache
	retryCache := NewRetryCache(cfg.RedisClient, logger)

	return &APIEventProcessor{
		ctx:         cfg.Context,
		logger:      logger.WithField("component", "api-event-processor"),
		config:      cfg.Config,
		agentID:     cfg.AgentID,
		apiEndpoint: cfg.APIEndpoint,
		apiToken:    cfg.APIToken,
		httpClient:  httpClient,
		redisClient: cfg.RedisClient,
		retryCache:  retryCache,
	}, nil
}

// Process implements the EventProcessor interface
func (p *APIEventProcessor) Process(event *KubernetesEvent) error {
	start := time.Now()
	p.logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"severity":   event.Severity,
		"resource":   fmt.Sprintf("%s/%s", event.ResourceKind, event.ResourceName),
		"namespace":  event.ResourceNamespace,
	}).Info("Processing event for API submission")

	// Convert to API submission format
	submission := p.convertToSubmission(event)

	// Attempt to submit to API
	if err := p.submitToAPI([]EventSubmission{submission}); err != nil {
		p.logger.WithError(err).Warn("Failed to submit event to API, caching for retry")

		// Cache for retry
		if cacheErr := p.retryCache.Add(submission); cacheErr != nil {
			p.logger.WithError(cacheErr).Error("Failed to cache event for retry")
		}

		// Record failure metric
		eventsProcessedTotal.WithLabelValues(string(event.Type), string(event.Severity), event.ResourceNamespace).Inc()
		eventProcessingDuration.WithLabelValues(string(event.Type)).Observe(time.Since(start).Seconds())

		return fmt.Errorf("failed to submit event: %w", err)
	}

	// Record success metric
	eventsProcessedTotal.WithLabelValues(string(event.Type), string(event.Severity), event.ResourceNamespace).Inc()
	eventProcessingDuration.WithLabelValues(string(event.Type)).Observe(time.Since(start).Seconds())

	p.logger.WithField("event_id", submission.ResourceUID).Info("Event submitted to API successfully")
	return nil
}

// ProcessBatch implements batch processing for efficiency
func (p *APIEventProcessor) ProcessBatch(events []*KubernetesEvent) error {
	if len(events) == 0 {
		return nil
	}

	p.logger.WithField("count", len(events)).Info("Processing event batch for API submission")

	// Convert all events to submissions
	submissions := make([]EventSubmission, 0, len(events))
	for _, event := range events {
		submissions = append(submissions, p.convertToSubmission(event))
	}

	// Attempt batch submission
	if err := p.submitToAPI(submissions); err != nil {
		p.logger.WithError(err).Warn("Failed to submit batch to API, caching for retry")

		// Cache all for retry
		for _, submission := range submissions {
			if cacheErr := p.retryCache.Add(submission); cacheErr != nil {
				p.logger.WithError(cacheErr).Error("Failed to cache event for retry")
			}
		}

		return fmt.Errorf("failed to submit batch: %w", err)
	}

	p.logger.WithField("count", len(submissions)).Info("Event batch submitted to API successfully")
	return nil
}

// StartRetryWorker starts a background worker that retries failed submissions
func (p *APIEventProcessor) StartRetryWorker() {
	p.logger.Info("Starting retry worker for failed event submissions")

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Info("Retry worker stopped")
			return
		case <-ticker.C:
			p.processRetries()
		}
	}
}

// processRetries attempts to resubmit cached events
func (p *APIEventProcessor) processRetries() {
	cachedEvents, err := p.retryCache.GetPending(100) // Process up to 100 at a time
	if err != nil {
		p.logger.WithError(err).Error("Failed to get pending retries")
		return
	}

	if len(cachedEvents) == 0 {
		return
	}

	p.logger.WithField("count", len(cachedEvents)).Info("Processing cached events for retry")

	// Attempt batch submission
	if err := p.submitToAPI(cachedEvents); err != nil {
		p.logger.WithError(err).Warn("Retry submission failed")
		return
	}

	// Remove from cache on success
	for _, event := range cachedEvents {
		if err := p.retryCache.Remove(event); err != nil {
			p.logger.WithError(err).Warn("Failed to remove event from cache")
		}
	}

	p.logger.WithField("count", len(cachedEvents)).Info("Successfully retried cached events")
}

// convertToSubmission converts a KubernetesEvent to API submission format
func (p *APIEventProcessor) convertToSubmission(event *KubernetesEvent) EventSubmission {
	// Build metadata from labels and annotations
	metadata := make(map[string]interface{})
	for k, v := range event.Labels {
		metadata["label_"+k] = v
	}
	for k, v := range event.Annotations {
		metadata["annotation_"+k] = v
	}
	metadata["reason"] = event.Reason
	if event.OwnerKind != "" {
		metadata["owner_kind"] = event.OwnerKind
		metadata["owner_name"] = event.OwnerName
		metadata["owner_uid"] = event.OwnerUID
	}

	return EventSubmission{
		EventType:      string(event.Type),
		Severity:       string(event.Severity),
		ResourceKind:   event.ResourceKind,
		ResourceName:   event.ResourceName,
		ResourceUID:    &event.ResourceUID,
		Namespace:      &event.ResourceNamespace,
		Message:        event.Message,
		Metadata:       metadata,
		EventTimestamp: event.Timestamp,
	}
}

// submitToAPI submits events to the API server
func (p *APIEventProcessor) submitToAPI(submissions []EventSubmission) error {
	// Build request
	requestBody := map[string]interface{}{
		"events": submissions,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/api/v1/events", p.apiEndpoint)
	req, err := http.NewRequestWithContext(p.ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiToken))

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // Safe after error check, ignore close error

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API returned error status: %d", resp.StatusCode)
	}

	p.logger.WithFields(logrus.Fields{
		"status_code": resp.StatusCode,
		"event_count": len(submissions),
	}).Debug("API submission successful")

	return nil
}

// EventSubmission represents a single event in API submission format
type EventSubmission struct {
	EventType      string                 `json:"event_type"`
	Severity       string                 `json:"severity"`
	ResourceKind   string                 `json:"resource_kind"`
	ResourceName   string                 `json:"resource_name"`
	ResourceUID    *string                `json:"resource_uid,omitempty"`
	Namespace      *string                `json:"namespace,omitempty"`
	Message        string                 `json:"message"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	EventTimestamp time.Time              `json:"event_timestamp"`
}

// RetryCache manages cached events for retry
type RetryCache struct {
	redisClient *redis.Client
	logger      *logrus.Entry
	keyPrefix   string
	maxRetries  int
	retryTTL    time.Duration
}

// NewRetryCache creates a new retry cache
func NewRetryCache(redisClient *redis.Client, logger *logrus.Logger) *RetryCache {
	return &RetryCache{
		redisClient: redisClient,
		logger:      logger.WithField("component", "retry-cache"),
		keyPrefix:   "lumo:event:retry:",
		maxRetries:  5,
		retryTTL:    24 * time.Hour,
	}
}

// Add adds an event to the retry cache
func (c *RetryCache) Add(event EventSubmission) error {
	if event.ResourceUID == nil {
		return fmt.Errorf("resource UID is required for retry cache")
	}
	key := fmt.Sprintf("%s%s:%d", c.keyPrefix, *event.ResourceUID, time.Now().UnixNano())

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	ctx := context.Background()
	if err := c.redisClient.Set(ctx, key, data, c.retryTTL).Err(); err != nil {
		return fmt.Errorf("failed to cache event: %w", err)
	}

	c.logger.WithField("key", key).Debug("Event added to retry cache")
	return nil
}

// GetPending retrieves pending events from cache
func (c *RetryCache) GetPending(limit int) ([]EventSubmission, error) {
	ctx := context.Background()

	// Scan for keys
	pattern := c.keyPrefix + "*"
	keys, err := c.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to scan keys: %w", err)
	}

	if len(keys) > limit {
		keys = keys[:limit]
	}

	events := make([]EventSubmission, 0, len(keys))
	for _, key := range keys {
		data, err := c.redisClient.Get(ctx, key).Result()
		if err != nil {
			c.logger.WithError(err).WithField("key", key).Warn("Failed to get cached event")
			continue
		}

		var event EventSubmission
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			c.logger.WithError(err).WithField("key", key).Warn("Failed to unmarshal cached event")
			continue
		}

		events = append(events, event)
	}

	return events, nil
}

// Remove removes an event from the retry cache
func (c *RetryCache) Remove(event EventSubmission) error {
	ctx := context.Background()

	// Find and delete the key
	pattern := fmt.Sprintf("%s%s:*", c.keyPrefix, *event.ResourceUID)
	keys, err := c.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to find keys: %w", err)
	}

	if len(keys) > 0 {
		if err := c.redisClient.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
		c.logger.WithField("count", len(keys)).Debug("Removed events from retry cache")
	}

	return nil
}
