package ai

import (
	"context"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// MockProvider is a test double for the Provider interface
type MockProvider struct {
	ProviderName  string
	ProviderModel string
	HealthError   error
	AnalyzeError  error
	AnalyzeResult *AnalysisResponse
	AnalyzeFunc   func(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error)
	HealthFunc    func(ctx context.Context) error
	StreamFunc    func(ctx context.Context, req *AnalysisRequest, callback func(string) error) (*AnalysisResponse, error)
	CallCount     int
	LastRequest   *AnalysisRequest
}

// Name returns the provider name
func (m *MockProvider) Name() string {
	if m.ProviderName != "" {
		return m.ProviderName
	}
	return "mock"
}

// Model returns the model name
func (m *MockProvider) Model() string {
	if m.ProviderModel != "" {
		return m.ProviderModel
	}
	return "mock-model-v1"
}

// Health checks provider health
func (m *MockProvider) Health(ctx context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}
	return m.HealthError
}

// Analyze performs analysis
func (m *MockProvider) Analyze(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error) {
	m.CallCount++
	m.LastRequest = req

	if m.AnalyzeFunc != nil {
		return m.AnalyzeFunc(ctx, req)
	}

	if m.AnalyzeError != nil {
		return nil, m.AnalyzeError
	}

	if m.AnalyzeResult != nil {
		return m.AnalyzeResult, nil
	}

	// Default successful response
	return &AnalysisResponse{
		Summary:         "System is healthy",
		OverallHealth:   HealthHealthy,
		Confidence:      0.95,
		Provider:        m.Name(),
		Model:           m.Model(),
		Timestamp:       time.Now(),
		Duration:        100 * time.Millisecond,
		Findings:        []Finding{},
		Recommendations: []Recommendation{},
	}, nil
}

// AnalyzeStream performs streaming analysis
func (m *MockProvider) AnalyzeStream(ctx context.Context, req *AnalysisRequest, callback func(string) error) (*AnalysisResponse, error) {
	m.CallCount++
	m.LastRequest = req

	if m.StreamFunc != nil {
		return m.StreamFunc(ctx, req, callback)
	}

	// Call callback with mock chunks
	if callback != nil {
		callback("Mock")
		callback(" streaming")
		callback(" response")
	}

	// Return same as Analyze
	return m.Analyze(ctx, req)
}

// NewMockAnalysisResponse creates a mock analysis response for testing
func NewMockAnalysisResponse(health HealthStatus, findings int, recommendations int) *AnalysisResponse {
	resp := &AnalysisResponse{
		Summary:         "Mock analysis complete",
		OverallHealth:   health,
		Confidence:      0.90,
		Provider:        "mock",
		Model:           "mock-v1",
		Timestamp:       time.Now(),
		Duration:        50 * time.Millisecond,
		Findings:        []Finding{},
		Recommendations: []Recommendation{},
		TokensUsed: &TokenUsage{
			InputTokens:  100,
			OutputTokens: 200,
			TotalTokens:  300,
		},
	}

	// Add mock findings
	for i := 0; i < findings; i++ {
		resp.Findings = append(resp.Findings, Finding{
			Category:    "test",
			Severity:    diagnostics.SeverityWarning,
			Title:       "Mock Finding",
			Description: "This is a mock finding for testing",
		})
	}

	// Add mock recommendations
	for i := 0; i < recommendations; i++ {
		resp.Recommendations = append(resp.Recommendations, Recommendation{
			Priority:        PriorityMedium,
			Title:           "Mock Recommendation",
			Description:     "This is a mock recommendation for testing",
			Risk:            RiskLow,
			Commands:        []string{"echo test"},
			EstimatedImpact: "Improves test coverage",
		})
	}

	return resp
}
