package correlation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/database/models"
)

func TestDefaultEngineConfig(t *testing.T) {
	config := DefaultEngineConfig()

	if config.CorrelationWindow != 5*time.Minute {
		t.Errorf("Expected CorrelationWindow 5m, got %v", config.CorrelationWindow)
	}
	if config.MinEventsForIncident != 1 {
		t.Errorf("Expected MinEventsForIncident 1, got %d", config.MinEventsForIncident)
	}
	if config.MaxEventsPerIncident != 100 {
		t.Errorf("Expected MaxEventsPerIncident 100, got %d", config.MaxEventsPerIncident)
	}
	if !config.EnableContextGathering {
		t.Error("Expected EnableContextGathering true")
	}
	if !config.EnableAIAnalysis {
		t.Error("Expected EnableAIAnalysis true")
	}
}

func TestIncidentAddEvent(t *testing.T) {
	incident := &Incident{
		ID:                uuid.New(),
		State:             IncidentStateOpen,
		Severity:          models.EventSeverityMedium,
		Timeline:          make([]TimelineEntry, 0),
		Events:            make([]*models.Event, 0),
		AffectedResources: make([]AffectedResource, 0),
		FirstEventAt:      time.Now(),
		LastEventAt:       time.Now(),
	}

	ns := "default"
	event := &models.Event{
		ID:             uuid.New(),
		EventType:      "crash-loop-backoff",
		Severity:       models.EventSeverityHigh,
		ResourceKind:   "Pod",
		ResourceName:   "test-pod",
		Namespace:      &ns,
		Message:        "Container crashed",
		EventTimestamp: time.Now(),
	}

	incident.AddEvent(event)

	if incident.EventCount() != 1 {
		t.Errorf("Expected 1 event, got %d", incident.EventCount())
	}

	if incident.Severity != models.EventSeverityHigh {
		t.Errorf("Expected severity to be upgraded to high, got %s", incident.Severity)
	}

	if len(incident.Timeline) != 1 {
		t.Errorf("Expected 1 timeline entry, got %d", len(incident.Timeline))
	}

	if len(incident.AffectedResources) != 1 {
		t.Errorf("Expected 1 affected resource, got %d", len(incident.AffectedResources))
	}
}

func TestIncidentSeverityUpgrade(t *testing.T) {
	incident := &Incident{
		ID:                uuid.New(),
		State:             IncidentStateOpen,
		Severity:          models.EventSeverityLow,
		Timeline:          make([]TimelineEntry, 0),
		Events:            make([]*models.Event, 0),
		AffectedResources: make([]AffectedResource, 0),
		FirstEventAt:      time.Now(),
		LastEventAt:       time.Now(),
	}

	ns := "default"

	// Add medium severity event
	incident.AddEvent(&models.Event{
		ID:             uuid.New(),
		EventType:      "warning",
		Severity:       models.EventSeverityMedium,
		ResourceKind:   "Pod",
		ResourceName:   "test-pod-1",
		Namespace:      &ns,
		EventTimestamp: time.Now(),
	})

	if incident.Severity != models.EventSeverityMedium {
		t.Errorf("Expected severity medium, got %s", incident.Severity)
	}

	// Add critical severity event
	incident.AddEvent(&models.Event{
		ID:             uuid.New(),
		EventType:      "oom-killed",
		Severity:       models.EventSeverityCritical,
		ResourceKind:   "Pod",
		ResourceName:   "test-pod-2",
		Namespace:      &ns,
		EventTimestamp: time.Now(),
	})

	if incident.Severity != models.EventSeverityCritical {
		t.Errorf("Expected severity critical, got %s", incident.Severity)
	}

	// Adding lower severity shouldn't downgrade
	incident.AddEvent(&models.Event{
		ID:             uuid.New(),
		EventType:      "warning",
		Severity:       models.EventSeverityLow,
		ResourceKind:   "Pod",
		ResourceName:   "test-pod-3",
		Namespace:      &ns,
		EventTimestamp: time.Now(),
	})

	if incident.Severity != models.EventSeverityCritical {
		t.Errorf("Expected severity to remain critical, got %s", incident.Severity)
	}
}

func TestCorrelationRulesMatch(t *testing.T) {
	rules := defaultCorrelationRules()

	tests := []struct {
		name             string
		eventType        string
		expectedCategory IncidentCategory
		shouldMatch      bool
	}{
		{"OOMKilled matches memory", "oom-killed", CategoryMemory, true},
		{"CrashLoop matches crash", "crash-loop-backoff", CategoryCrash, true},
		{"ImagePullBackOff matches image", "image-pull-backoff", CategoryImage, true},
		{"NodeNotReady matches node", "node-not-ready", CategoryNode, true},
		{"DeploymentFailed matches deployment", "deployment-failed", CategoryDeployment, true},
		{"VolumeFailedMount matches storage", "volume-failed-mount", CategoryStorage, true},
		{"PodPending matches scheduling", "pod-pending", CategoryScheduling, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &models.Event{
				EventType: tt.eventType,
			}

			var matchedRule *CorrelationRule
			for i := range rules {
				if rules[i].Match(event) {
					if matchedRule == nil || rules[i].Priority > matchedRule.Priority {
						matchedRule = &rules[i]
					}
				}
			}

			if tt.shouldMatch {
				if matchedRule == nil {
					t.Errorf("Expected event %s to match a rule", tt.eventType)
					return
				}
				if matchedRule.Category != tt.expectedCategory {
					t.Errorf("Expected category %s, got %s", tt.expectedCategory, matchedRule.Category)
				}
			}
		})
	}
}

func TestCorrelationKeyGeneration(t *testing.T) {
	rules := defaultCorrelationRules()

	// Find memory rule
	var memoryRule *CorrelationRule
	for i := range rules {
		if rules[i].Category == CategoryMemory {
			memoryRule = &rules[i]
			break
		}
	}

	if memoryRule == nil {
		t.Fatal("Memory rule not found")
	}

	ns := "production"
	nodeName := "node-1"

	// Event with node name in metadata
	event1 := &models.Event{
		EventType: "oom-killed",
		Namespace: &ns,
		Metadata:  models.JSONB{"node_name": nodeName},
	}

	key1 := memoryRule.CorrelationKey(event1)
	expectedKey := "memory:node:node-1"
	if key1 != expectedKey {
		t.Errorf("Expected key %s, got %s", expectedKey, key1)
	}

	// Event without node name should fall back to namespace
	event2 := &models.Event{
		EventType: "oom-killed",
		Namespace: &ns,
		Metadata:  models.JSONB{},
	}

	key2 := memoryRule.CorrelationKey(event2)
	expectedKey2 := "memory:ns:production"
	if key2 != expectedKey2 {
		t.Errorf("Expected key %s, got %s", expectedKey2, key2)
	}
}

func TestEngineProcessEvent(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	// Create engine with short correlation window for testing
	config := &EngineConfig{
		CorrelationWindow:       100 * time.Millisecond,
		MinEventsForIncident:    1,
		MaxEventsPerIncident:    10,
		SuppressDuplicateWindow: 1 * time.Hour,
		EnableContextGathering:  false,
		EnableAIAnalysis:        false,
	}

	repo := NewInMemoryIncidentRepository(logger)
	engine := NewEngine(config, nil, nil, nil, repo, logger)
	defer engine.Stop()

	ctx := context.Background()
	ns := "default"

	// Process first event
	event1 := &models.Event{
		ID:             uuid.New(),
		EventType:      "crash-loop-backoff",
		Severity:       models.EventSeverityHigh,
		ResourceKind:   "Pod",
		ResourceName:   "test-pod",
		Namespace:      &ns,
		Message:        "Container crashed",
		EventTimestamp: time.Now(),
		Metadata:       models.JSONB{"owner_uid": "owner-123"},
	}

	err := engine.ProcessEvent(ctx, event1)
	if err != nil {
		t.Fatalf("Failed to process event: %v", err)
	}

	// Should have 1 open incident
	if engine.GetOpenIncidentCount() != 1 {
		t.Errorf("Expected 1 open incident, got %d", engine.GetOpenIncidentCount())
	}

	// Process related event (same owner)
	event2 := &models.Event{
		ID:             uuid.New(),
		EventType:      "crash-loop-backoff",
		Severity:       models.EventSeverityCritical,
		ResourceKind:   "Pod",
		ResourceName:   "test-pod-2",
		Namespace:      &ns,
		Message:        "Another container crashed",
		EventTimestamp: time.Now(),
		Metadata:       models.JSONB{"owner_uid": "owner-123"},
	}

	err = engine.ProcessEvent(ctx, event2)
	if err != nil {
		t.Fatalf("Failed to process event: %v", err)
	}

	// Should still have 1 open incident (events correlated)
	if engine.GetOpenIncidentCount() != 1 {
		t.Errorf("Expected 1 open incident after correlation, got %d", engine.GetOpenIncidentCount())
	}

	// Verify incident has 2 events
	incidents := engine.GetOpenIncidents()
	if len(incidents) != 1 {
		t.Fatalf("Expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].EventCount() != 2 {
		t.Errorf("Expected 2 events in incident, got %d", incidents[0].EventCount())
	}

	// Severity should be upgraded to critical
	if incidents[0].Severity != models.EventSeverityCritical {
		t.Errorf("Expected critical severity, got %s", incidents[0].Severity)
	}
}

func TestInMemoryRepository(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	repo := NewInMemoryIncidentRepository(logger)
	ctx := context.Background()

	// Create incident
	incident := &Incident{
		ID:             uuid.New(),
		Category:       CategoryCrash,
		Severity:       models.EventSeverityHigh,
		State:          IncidentStateOpen,
		CorrelationKey: "test-key",
	}

	err := repo.Create(ctx, incident)
	if err != nil {
		t.Fatalf("Failed to create incident: %v", err)
	}

	// Get by ID
	retrieved, err := repo.GetByID(ctx, incident.ID)
	if err != nil {
		t.Fatalf("Failed to get incident: %v", err)
	}
	if retrieved.ID != incident.ID {
		t.Errorf("Retrieved incident ID mismatch")
	}

	// Get by correlation key
	retrieved, err = repo.GetByCorrelationKey(ctx, "test-key")
	if err != nil {
		t.Fatalf("Failed to get by correlation key: %v", err)
	}
	if retrieved.ID != incident.ID {
		t.Errorf("Retrieved incident ID mismatch")
	}

	// List open
	open, err := repo.ListOpen(ctx)
	if err != nil {
		t.Fatalf("Failed to list open: %v", err)
	}
	if len(open) != 1 {
		t.Errorf("Expected 1 open incident, got %d", len(open))
	}

	// Update
	incident.State = IncidentStateResolved
	err = repo.Update(ctx, incident)
	if err != nil {
		t.Fatalf("Failed to update: %v", err)
	}

	// List open should be empty now
	open, err = repo.ListOpen(ctx)
	if err != nil {
		t.Fatalf("Failed to list open: %v", err)
	}
	if len(open) != 0 {
		t.Errorf("Expected 0 open incidents after resolve, got %d", len(open))
	}

	// Delete
	err = repo.Delete(ctx, incident.ID)
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	// Should not exist
	_, err = repo.GetByID(ctx, incident.ID)
	if err == nil {
		t.Error("Expected error getting deleted incident")
	}
}

func TestSeverityOrder(t *testing.T) {
	tests := []struct {
		severity models.EventSeverity
		expected int
	}{
		{models.EventSeverityLow, 1},
		{models.EventSeverityMedium, 2},
		{models.EventSeverityHigh, 3},
		{models.EventSeverityCritical, 4},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			if severityOrder(tt.severity) != tt.expected {
				t.Errorf("Expected order %d for %s, got %d",
					tt.expected, tt.severity, severityOrder(tt.severity))
			}
		})
	}
}

func TestGenerateTitle(t *testing.T) {
	logger := logrus.New()
	engine := NewEngine(nil, nil, nil, nil, nil, logger)

	ns := "production"

	tests := []struct {
		name     string
		incident *Incident
		contains string
	}{
		{
			name: "Memory incident",
			incident: &Incident{
				Category:  CategoryMemory,
				Namespace: &ns,
				AffectedResources: []AffectedResource{
					{Kind: "Pod", Name: "test-pod", Namespace: ns},
				},
			},
			contains: "Memory",
		},
		{
			name: "Crash incident with multiple pods",
			incident: &Incident{
				Category:  CategoryCrash,
				Namespace: &ns,
				AffectedResources: []AffectedResource{
					{Kind: "Pod", Name: "pod-1", Namespace: ns},
					{Kind: "Pod", Name: "pod-2", Namespace: ns},
				},
			},
			contains: "2 pods",
		},
		{
			name: "Node incident",
			incident: &Incident{
				Category: CategoryNode,
				NodeName: stringPtr("node-1"),
			},
			contains: "node-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title := engine.generateTitle(tt.incident)
			if !containsIgnoreCase(title, tt.contains) {
				t.Errorf("Expected title to contain '%s', got '%s'", tt.contains, title)
			}
		})
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > 0 && len(substr) > 0 &&
				(s[0] == substr[0] || s[0]+32 == substr[0] || s[0]-32 == substr[0]) &&
				containsIgnoreCase(s[1:], substr[1:]) ||
			len(s) > 0 && containsIgnoreCase(s[1:], substr))
}

func TestInferRootCauseFromEvents(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	manager := &RealtimeIncidentManager{
		logger: logger.WithField("test", true),
	}

	tests := []struct {
		name            string
		incident        *Incident
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "scheduling failed with taint",
			incident: &Incident{
				Category: CategoryScheduling,
				Events: []*models.Event{
					{
						EventType:    "scheduling-failed",
						Message:      "0/1 nodes are available: 1 node(s) had untolerated taint {node-role.kubernetes.io/control-plane: }",
						ResourceName: "test-pod",
					},
				},
			},
			wantContains: []string{"taint", "toleration"},
		},
		{
			name: "image pull with typo",
			incident: &Incident{
				Category: CategoryImage,
				Events: []*models.Event{
					{
						EventType: "image-pull-backoff",
						Message:   "repository does not exist",
						Metadata:  models.JSONB{"image": "nginxx:latest"},
					},
				},
			},
			wantContains: []string{"nginxx:latest", "not found"},
		},
		{
			name: "OOM killed with limits",
			incident: &Incident{
				Category: CategoryMemory,
				Events: []*models.Event{
					{
						EventType: "oom-killed",
						Message:   "Container was OOMKilled",
						Metadata: models.JSONB{
							"container_limits": map[string]interface{}{
								"memory": "128Mi",
							},
						},
					},
				},
			},
			wantContains: []string{"128Mi", "memory limit"},
		},
		{
			name: "crash loop with restarts",
			incident: &Incident{
				Category: CategoryCrash,
				Events: []*models.Event{
					{
						EventType:    "crash-loop-backoff",
						Message:      "Container crashed",
						ResourceName: "my-pod",
						Namespace:    stringPtr("default"),
						Metadata:     models.JSONB{"restart_count": float64(5)},
					},
				},
			},
			wantContains: []string{"5 times", "kubectl logs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.inferRootCauseFromEvents(tt.incident)

			for _, want := range tt.wantContains {
				if !strings.Contains(strings.ToLower(result), strings.ToLower(want)) {
					t.Errorf("inferRootCauseFromEvents() result = %q, want to contain %q", result, want)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(strings.ToLower(result), strings.ToLower(notWant)) {
					t.Errorf("inferRootCauseFromEvents() result = %q, should not contain %q", result, notWant)
				}
			}
		})
	}
}
