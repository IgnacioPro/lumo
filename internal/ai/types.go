// Package ai provides AI-powered analysis for diagnostic results.
// It supports multiple providers (Anthropic Claude, OpenAI GPT, local models)
// with a unified interface for analyzing system health and providing recommendations.
package ai

import (
	"context"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// Provider represents an AI provider that can analyze diagnostic results.
// Implementations must be safe for concurrent use.
type Provider interface {
	// Name returns the provider name (e.g., "anthropic", "openai", "ollama")
	Name() string

	// Analyze analyzes diagnostic results and returns insights and recommendations.
	// The context can be used for cancellation and timeout control.
	Analyze(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error)

	// AnalyzeStream analyzes diagnostic results with streaming response.
	// Returns a channel that receives analysis chunks as they arrive.
	// The channel is closed when analysis is complete or an error occurs.
	AnalyzeStream(ctx context.Context, req *AnalysisRequest) (<-chan StreamChunk, error)

	// Health checks if the provider is available and configured correctly.
	Health(ctx context.Context) error
}

// AnalysisRequest contains diagnostic results and context for AI analysis.
type AnalysisRequest struct {
	// Report contains the complete diagnostic report to analyze
	Report *diagnostics.Report

	// SystemInfo provides context about the system being analyzed
	SystemInfo SystemInfo

	// SelectedChecks indicates which checks were requested (empty = all checks)
	// Used to inform AI that missing data is expected, not a problem
	SelectedChecks []string

	// Focus specifies areas to focus analysis on (empty = analyze all)
	Focus []string

	// Options contains provider-specific options
	Options map[string]interface{}
}

// AnalysisResponse contains the AI provider's analysis and recommendations.
type AnalysisResponse struct {
	// Summary is a brief overview of findings
	Summary string

	// Findings contains detailed analysis of issues discovered
	Findings []Finding

	// Recommendations contains actionable steps to resolve issues
	Recommendations []Recommendation

	// OverallHealth is an assessment of system health (healthy, degraded, critical)
	OverallHealth HealthStatus

	// Confidence is the AI's confidence level in the analysis (0.0-1.0)
	Confidence float64

	// Provider is the name of the AI provider that generated this analysis
	Provider string

	// Model is the specific model used for analysis
	Model string

	// Timestamp is when the analysis was generated
	Timestamp time.Time

	// Duration is how long the analysis took
	Duration time.Duration

	// TokensUsed tracks API usage (if applicable)
	TokensUsed *TokenUsage
}

// Finding represents a specific issue or observation from analysis.
type Finding struct {
	// Category groups the finding (e.g., "CPU", "Memory", "Disk")
	Category string

	// Severity indicates how serious this finding is
	Severity diagnostics.Severity

	// Title is a brief description of the finding
	Title string

	// Description provides detailed explanation
	Description string

	// Evidence contains supporting data from diagnostic results
	Evidence map[string]interface{}

	// RelatedChecks lists which diagnostic checks revealed this finding
	RelatedChecks []string
}

// Recommendation represents an actionable step to improve system health.
type Recommendation struct {
	// Priority indicates urgency (critical, high, medium, low)
	Priority Priority

	// Title is a brief description of the recommendation
	Title string

	// Description explains what to do and why
	Description string

	// Commands contains specific commands to execute (if applicable)
	Commands []string

	// Risk assesses the risk of implementing this recommendation
	Risk RiskLevel

	// EstimatedImpact describes expected improvements
	EstimatedImpact string

	// RelatedFindings links to findings this addresses
	RelatedFindings []int
}

// SystemInfo provides context about the system being analyzed.
type SystemInfo struct {
	// Hostname of the system
	Hostname string

	// Platform (linux, darwin, windows, etc.)
	Platform string

	// Architecture (amd64, arm64, etc.)
	Architecture string

	// KernelVersion or OS version
	KernelVersion string

	// UptimeDays indicates how long the system has been running
	UptimeDays float64

	// Environment describes the system role (production, staging, development)
	Environment string

	// Tags for additional context
	Tags map[string]string
}

// HealthStatus represents overall system health assessment.
type HealthStatus string

const (
	HealthHealthy  HealthStatus = "healthy"  // No critical issues
	HealthDegraded HealthStatus = "degraded" // Some issues but functional
	HealthCritical HealthStatus = "critical" // Serious issues requiring immediate attention
	HealthUnknown  HealthStatus = "unknown"  // Unable to determine health
)

// Priority indicates recommendation urgency.
type Priority string

const (
	PriorityCritical Priority = "critical" // Immediate action required
	PriorityHigh     Priority = "high"     // Important, address soon
	PriorityMedium   Priority = "medium"   // Should address when possible
	PriorityLow      Priority = "low"      // Nice to have, non-urgent
)

// RiskLevel assesses the risk of implementing a recommendation.
type RiskLevel string

const (
	RiskSafe     RiskLevel = "safe"     // No risk, safe to implement
	RiskLow      RiskLevel = "low"      // Minimal risk
	RiskModerate RiskLevel = "moderate" // Some risk, test first
	RiskHigh     RiskLevel = "high"     // Significant risk, requires approval
	RiskCritical RiskLevel = "critical" // Dangerous, only for emergencies
)

// StreamChunk represents a piece of streaming analysis response.
type StreamChunk struct {
	// Type indicates what kind of chunk this is
	Type ChunkType

	// Content is the chunk data (format depends on Type)
	Content string

	// Metadata contains additional chunk information
	Metadata map[string]interface{}

	// Error if this chunk represents an error
	Error error

	// Done indicates this is the final chunk
	Done bool
}

// ChunkType categorizes streaming chunks.
type ChunkType string

const (
	ChunkSummary        ChunkType = "summary"        // Summary text chunk
	ChunkFinding        ChunkType = "finding"        // Finding chunk
	ChunkRecommendation ChunkType = "recommendation" // Recommendation chunk
	ChunkThinking       ChunkType = "thinking"       // Provider's reasoning
	ChunkError          ChunkType = "error"          // Error occurred
)

// TokenUsage tracks API token consumption.
type TokenUsage struct {
	// InputTokens consumed by the request
	InputTokens int

	// OutputTokens generated in the response
	OutputTokens int

	// TotalTokens is the sum of input and output
	TotalTokens int

	// EstimatedCost in USD (if available)
	EstimatedCost float64
}

// ProviderConfig contains configuration for an AI provider.
type ProviderConfig struct {
	// Name of the provider (anthropic, openai, ollama)
	Name string

	// APIKey for authentication (required for cloud providers)
	APIKey string

	// Model to use for analysis
	Model string

	// Endpoint URL (for custom/local deployments)
	Endpoint string

	// Timeout for API requests
	Timeout time.Duration

	// MaxRetries for transient failures
	MaxRetries int

	// Temperature controls randomness (0.0-1.0)
	Temperature float64

	// MaxTokens limits response length
	MaxTokens int

	// CustomHeaders for API requests
	CustomHeaders map[string]string
}

// Error types for AI operations.
type Error struct {
	// Op is the operation that failed
	Op string

	// Provider is the AI provider name
	Provider string

	// Err is the underlying error
	Err error

	// Retryable indicates if the operation can be retried
	Retryable bool
}

func (e *Error) Error() string {
	if e.Provider != "" {
		return e.Op + " [" + e.Provider + "]: " + e.Err.Error()
	}
	return e.Op + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}
