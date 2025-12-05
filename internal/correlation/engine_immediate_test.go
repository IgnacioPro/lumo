package correlation

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/database/models"
)

// MockIncidentNotifier for testing
type MockIncidentNotifier struct {
	mu                  sync.Mutex
	createdCalls        []*Incident
	analysisUpdateCalls []*Incident
	incidentCalls       []*Incident
	createdReturnTS     string
	createdReturnErr    error
}

func (m *MockIncidentNotifier) NotifyIncident(ctx context.Context, incident *Incident) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incidentCalls = append(m.incidentCalls, incident)
	return nil
}

func (m *MockIncidentNotifier) NotifyIncidentCreated(ctx context.Context, incident *Incident) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createdCalls = append(m.createdCalls, incident)
	return m.createdReturnTS, m.createdReturnErr
}

func (m *MockIncidentNotifier) NotifyAnalysisUpdate(ctx context.Context, incident *Incident, entry *AnalysisLogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.analysisUpdateCalls = append(m.analysisUpdateCalls, incident)
	return nil
}

// MockContextGatherer for testing
type MockContextGatherer struct{}

func (m *MockContextGatherer) GatherContext(ctx context.Context, incident *Incident) (*IncidentContext, error) {
	return &IncidentContext{}, nil
}

// MockAIAnalyzer for testing
type MockAIAnalyzer struct{}

func (m *MockAIAnalyzer) Analyze(ctx context.Context, incident *Incident) (*AIAnalysisResult, error) {
	return &AIAnalysisResult{
		FullAnalysis: "Test Analysis",
		RootCause: RootCauseAnalysis{
			Summary: "Test Root Cause",
		},
	}, nil
}

func TestImmediateNotification(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Setup mocks
	notifier := &MockIncidentNotifier{
		createdReturnTS: "12345.67890",
	}
	gatherer := &MockContextGatherer{}
	analyzer := &MockAIAnalyzer{}
	repo := NewInMemoryIncidentRepository(logger)

	config := &EngineConfig{
		CorrelationWindow:       1 * time.Minute, // Long enough to ensure we don't close naturally
		MinEventsForIncident:    1,
		MaxEventsPerIncident:    100,
		SuppressDuplicateWindow: 1 * time.Hour,
		EnableContextGathering:  true,
		EnableAIAnalysis:        true,
	}

	engine := NewEngine(config, gatherer, analyzer, notifier, repo, logger)
	defer engine.Stop()

	ctx := context.Background()
	ns := "default"

	// 1. Send Critical Event
	event := &models.Event{
		ID:             uuid.New(),
		EventType:      "crash-loop-backoff",
		Severity:       models.EventSeverityCritical, // Critical!
		ResourceKind:   "Pod",
		ResourceName:   "critical-pod",
		Namespace:      &ns,
		Message:        "Critical failure",
		EventTimestamp: time.Now(),
	}

	err := engine.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to process event: %v", err)
	}

	// 2. Verify NotifyIncidentCreated called immediately
	// We might need a small sleep as it's in a goroutine, but usually it's fast.
	// Use Eventually-like loop
	success := false
	for i := 0; i < 10; i++ {
		notifier.mu.Lock()
		count := len(notifier.createdCalls)
		notifier.mu.Unlock()
		if count > 0 {
			success = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !success {
		t.Fatal("NotifyIncidentCreated was not called immediately")
	}

	notifier.mu.Lock()
	incident := notifier.createdCalls[0]
	incidentID := incident.ID // Save the ID while under mutex to avoid race
	notifier.mu.Unlock()

	// 3. Verify Early Investigation (NotifyAnalysisUpdate)
	// triggerEarlyInvestigation sleeps for 5 seconds. We don't want to wait 5s in test.
	// But we can't easily mock time.Sleep in the engine without dependency injection of a clock/timer.
	// For this test, we might just have to wait, or we can assume if the goroutine started, it will run.
	// To make test faster, we could make the sleep duration configurable in EngineConfig.

	// Let's check if we can update EngineConfig to have a shorter delay.
	// engine.go uses hardcoded `time.Sleep(5 * time.Second)`.
	// I should probably make that configurable or just wait 5s. 5s is acceptable for a test.

	t.Log("Waiting for early investigation (approx 5s)...")
	success = false
	for i := 0; i < 60; i++ { // Wait up to 6s
		notifier.mu.Lock()
		count := len(notifier.analysisUpdateCalls)
		notifier.mu.Unlock()
		if count > 0 {
			success = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !success {
		t.Fatal("NotifyAnalysisUpdate was not called (early investigation failed)")
	}

	// Finally, verify that the ThreadTS was updated in the stored incident
	// Give a bit more time for the goroutine to update ThreadTS before checking
	time.Sleep(500 * time.Millisecond)
	stored, err := repo.GetByID(ctx, incidentID)
	if err != nil {
		t.Fatalf("Failed to get incident from repo: %v", err)
	}
	if stored.ThreadTS != "12345.67890" {
		t.Errorf("Expected ThreadTS to be updated to 12345.67890, got %s", stored.ThreadTS)
	}
}
