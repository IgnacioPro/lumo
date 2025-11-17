package main

import (
	"strings"
	"testing"

	"github.com/ignacio/lumo/internal/ai"
	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     bool
	}{
		{
			name:     "localhost",
			hostname: "localhost",
			want:     true,
		},
		{
			name:     "localhost uppercase",
			hostname: "LOCALHOST",
			want:     true,
		},
		{
			name:     "localhost with spaces",
			hostname: "  localhost  ",
			want:     true,
		},
		{
			name:     "localhost.localdomain",
			hostname: "localhost.localdomain",
			want:     true,
		},
		{
			name:     "127.0.0.1",
			hostname: "127.0.0.1",
			want:     true,
		},
		{
			name:     "IPv6 localhost",
			hostname: "::1",
			want:     true,
		},
		{
			name:     "0.0.0.0",
			hostname: "0.0.0.0",
			want:     true,
		},
		{
			name:     "empty hostname",
			hostname: "",
			want:     true,
		},
		{
			name:     "empty hostname with spaces",
			hostname: "   ",
			want:     true,
		},
		{
			name:     "remote host",
			hostname: "example.com",
			want:     false,
		},
		{
			name:     "remote IP",
			hostname: "192.168.1.10",
			want:     false,
		},
		{
			name:     "remote IPv6",
			hostname: "2001:db8::1",
			want:     false,
		},
		{
			name:     "hostname with localhost in it",
			hostname: "mylocalhost.com",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLocalhost(tt.hostname)
			if got != tt.want {
				t.Errorf("isLocalhost(%q) = %v, want %v", tt.hostname, got, tt.want)
			}
		})
	}
}

func TestFormatHealthStatus(t *testing.T) {
	tests := []struct {
		name   string
		status ai.HealthStatus
		color  bool
		want   string
	}{
		{
			name:   "healthy with color",
			status: ai.HealthHealthy,
			color:  true,
			want:   "\033[32m✓ HEALTHY\033[0m",
		},
		{
			name:   "healthy without color",
			status: ai.HealthHealthy,
			color:  false,
			want:   string(ai.HealthHealthy),
		},
		{
			name:   "degraded with color",
			status: ai.HealthDegraded,
			color:  true,
			want:   "\033[33m⚠ DEGRADED\033[0m",
		},
		{
			name:   "degraded without color",
			status: ai.HealthDegraded,
			color:  false,
			want:   string(ai.HealthDegraded),
		},
		{
			name:   "critical with color",
			status: ai.HealthCritical,
			color:  true,
			want:   "\033[31m✗ CRITICAL\033[0m",
		},
		{
			name:   "critical without color",
			status: ai.HealthCritical,
			color:  false,
			want:   string(ai.HealthCritical),
		},
		{
			name:   "unknown with color",
			status: ai.HealthStatus("unknown"),
			color:  true,
			want:   "\033[90m? UNKNOWN\033[0m",
		},
		{
			name:   "unknown without color",
			status: ai.HealthStatus("unknown"),
			color:  false,
			want:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHealthStatus(tt.status, tt.color)
			if got != tt.want {
				t.Errorf("formatHealthStatus(%v, %v) = %q, want %q", tt.status, tt.color, got, tt.want)
			}
		})
	}
}

func TestFormatSeverity(t *testing.T) {
	tests := []struct {
		name     string
		severity diagnostics.Severity
		color    bool
		want     string
	}{
		{
			name:     "info with color",
			severity: diagnostics.SeverityInfo,
			color:    true,
			want:     "\033[36mINFO\033[0m",
		},
		{
			name:     "info without color",
			severity: diagnostics.SeverityInfo,
			color:    false,
			want:     string(diagnostics.SeverityInfo),
		},
		{
			name:     "warning with color",
			severity: diagnostics.SeverityWarning,
			color:    true,
			want:     "\033[33mWARN\033[0m",
		},
		{
			name:     "warning without color",
			severity: diagnostics.SeverityWarning,
			color:    false,
			want:     string(diagnostics.SeverityWarning),
		},
		{
			name:     "error with color",
			severity: diagnostics.SeverityError,
			color:    true,
			want:     "\033[31mERROR\033[0m",
		},
		{
			name:     "error without color",
			severity: diagnostics.SeverityError,
			color:    false,
			want:     string(diagnostics.SeverityError),
		},
		{
			name:     "critical with color",
			severity: diagnostics.SeverityCritical,
			color:    true,
			want:     "\033[1;31mCRITICAL\033[0m",
		},
		{
			name:     "critical without color",
			severity: diagnostics.SeverityCritical,
			color:    false,
			want:     string(diagnostics.SeverityCritical),
		},
		{
			name:     "ok with color",
			severity: diagnostics.SeverityOK,
			color:    true,
			want:     string(diagnostics.SeverityOK),
		},
		{
			name:     "ok without color",
			severity: diagnostics.SeverityOK,
			color:    false,
			want:     string(diagnostics.SeverityOK),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSeverity(tt.severity, tt.color)
			if got != tt.want {
				t.Errorf("formatSeverity(%v, %v) = %q, want %q", tt.severity, tt.color, got, tt.want)
			}
		})
	}
}

func TestFormatPriority(t *testing.T) {
	tests := []struct {
		name     string
		priority ai.Priority
		color    bool
		want     string
	}{
		{
			name:     "critical with color",
			priority: ai.PriorityCritical,
			color:    true,
			want:     "\033[1;31mCRITICAL\033[0m",
		},
		{
			name:     "critical without color",
			priority: ai.PriorityCritical,
			color:    false,
			want:     string(ai.PriorityCritical),
		},
		{
			name:     "high with color",
			priority: ai.PriorityHigh,
			color:    true,
			want:     "\033[31mHIGH\033[0m",
		},
		{
			name:     "high without color",
			priority: ai.PriorityHigh,
			color:    false,
			want:     string(ai.PriorityHigh),
		},
		{
			name:     "medium with color",
			priority: ai.PriorityMedium,
			color:    true,
			want:     "\033[33mMEDIUM\033[0m",
		},
		{
			name:     "medium without color",
			priority: ai.PriorityMedium,
			color:    false,
			want:     string(ai.PriorityMedium),
		},
		{
			name:     "low with color",
			priority: ai.PriorityLow,
			color:    true,
			want:     "\033[32mLOW\033[0m",
		},
		{
			name:     "low without color",
			priority: ai.PriorityLow,
			color:    false,
			want:     string(ai.PriorityLow),
		},
		{
			name:     "unknown with color",
			priority: ai.Priority("unknown"),
			color:    true,
			want:     "unknown",
		},
		{
			name:     "unknown without color",
			priority: ai.Priority("unknown"),
			color:    false,
			want:     "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPriority(tt.priority, tt.color)
			if got != tt.want {
				t.Errorf("formatPriority(%v, %v) = %q, want %q", tt.priority, tt.color, got, tt.want)
			}
		})
	}
}

func TestFormatRisk(t *testing.T) {
	tests := []struct {
		name  string
		risk  ai.RiskLevel
		color bool
		want  string
	}{
		{
			name:  "critical with color",
			risk:  ai.RiskCritical,
			color: true,
			want:  "\033[1;31mCRITICAL\033[0m",
		},
		{
			name:  "critical without color",
			risk:  ai.RiskCritical,
			color: false,
			want:  string(ai.RiskCritical),
		},
		{
			name:  "high with color",
			risk:  ai.RiskHigh,
			color: true,
			want:  "\033[31mHIGH\033[0m",
		},
		{
			name:  "high without color",
			risk:  ai.RiskHigh,
			color: false,
			want:  string(ai.RiskHigh),
		},
		{
			name:  "moderate with color",
			risk:  ai.RiskModerate,
			color: true,
			want:  "\033[33mMODERATE\033[0m",
		},
		{
			name:  "moderate without color",
			risk:  ai.RiskModerate,
			color: false,
			want:  string(ai.RiskModerate),
		},
		{
			name:  "low with color",
			risk:  ai.RiskLow,
			color: true,
			want:  "\033[36mLOW\033[0m",
		},
		{
			name:  "low without color",
			risk:  ai.RiskLow,
			color: false,
			want:  string(ai.RiskLow),
		},
		{
			name:  "safe with color",
			risk:  ai.RiskSafe,
			color: true,
			want:  "\033[32mSAFE\033[0m",
		},
		{
			name:  "safe without color",
			risk:  ai.RiskSafe,
			color: false,
			want:  string(ai.RiskSafe),
		},
		{
			name:  "unknown with color",
			risk:  ai.RiskLevel("unknown"),
			color: true,
			want:  "unknown",
		},
		{
			name:  "unknown without color",
			risk:  ai.RiskLevel("unknown"),
			color: false,
			want:  "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRisk(tt.risk, tt.color)
			if got != tt.want {
				t.Errorf("formatRisk(%v, %v) = %q, want %q", tt.risk, tt.color, got, tt.want)
			}
		})
	}
}

func TestFormatAIAnalysisJSON(t *testing.T) {
	tests := []struct {
		name     string
		analysis *ai.AnalysisResponse
		wantErr  bool
		checkFn  func(string) bool
	}{
		{
			name: "valid analysis",
			analysis: &ai.AnalysisResponse{
				OverallHealth:   ai.HealthHealthy,
				Summary:         "System is healthy",
				Findings:        []ai.Finding{},
				Recommendations: []ai.Recommendation{},
			},
			wantErr: false,
			checkFn: func(s string) bool {
				return strings.Contains(s, "OverallHealth") &&
					strings.Contains(s, "Summary") &&
					strings.Contains(s, "System is healthy")
			},
		},
		{
			name: "analysis with findings",
			analysis: &ai.AnalysisResponse{
				OverallHealth: ai.HealthDegraded,
				Summary:       "System has issues",
				Findings: []ai.Finding{
					{
						Category:    "cpu",
						Severity:    diagnostics.SeverityWarning,
						Description: "High CPU usage",
					},
				},
				Recommendations: []ai.Recommendation{},
			},
			wantErr: false,
			checkFn: func(s string) bool {
				return strings.Contains(s, "Findings") &&
					strings.Contains(s, "High CPU usage")
			},
		},
		{
			name: "analysis with recommendations",
			analysis: &ai.AnalysisResponse{
				OverallHealth: ai.HealthCritical,
				Summary:       "Critical issues",
				Findings:      []ai.Finding{},
				Recommendations: []ai.Recommendation{
					{
						Title:       "Restart Service",
						Priority:    ai.PriorityHigh,
						Description: "Restart the service",
						Risk:        ai.RiskLow,
					},
				},
			},
			wantErr: false,
			checkFn: func(s string) bool {
				return strings.Contains(s, "Recommendations") &&
					strings.Contains(s, "Restart")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatAIAnalysisJSON(tt.analysis)

			if (err != nil) != tt.wantErr {
				t.Errorf("formatAIAnalysisJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if got == "" {
				t.Error("formatAIAnalysisJSON() returned empty string")
			}

			if tt.checkFn != nil && !tt.checkFn(got) {
				t.Errorf("formatAIAnalysisJSON() output doesn't match expected pattern:\n%s", got)
			}
		})
	}
}

func TestFormatAIAnalysisText(t *testing.T) {
	tests := []struct {
		name     string
		analysis *ai.AnalysisResponse
		color    bool
		checkFn  func(string) bool
	}{
		{
			name: "basic analysis without color",
			analysis: &ai.AnalysisResponse{
				OverallHealth:   ai.HealthHealthy,
				Summary:         "System is healthy",
				Findings:        []ai.Finding{},
				Recommendations: []ai.Recommendation{},
			},
			color: false,
			checkFn: func(s string) bool {
				return strings.Contains(s, "System is healthy") &&
					strings.Contains(s, "Overall Health")
			},
		},
		{
			name: "analysis with findings",
			analysis: &ai.AnalysisResponse{
				OverallHealth: ai.HealthDegraded,
				Summary:       "System has issues",
				Findings: []ai.Finding{
					{
						Category:    "cpu",
						Severity:    diagnostics.SeverityWarning,
						Description: "High CPU usage detected",
					},
				},
				Recommendations: []ai.Recommendation{},
			},
			color: false,
			checkFn: func(s string) bool {
				return strings.Contains(s, "High CPU usage detected") &&
					strings.Contains(s, "FINDINGS")
			},
		},
		{
			name: "analysis with recommendations",
			analysis: &ai.AnalysisResponse{
				OverallHealth: ai.HealthCritical,
				Summary:       "Critical issues found",
				Findings:      []ai.Finding{},
				Recommendations: []ai.Recommendation{
					{
						Title:       "Restart Nginx",
						Priority:    ai.PriorityHigh,
						Description: "Restart nginx service",
						Risk:        ai.RiskLow,
					},
				},
			},
			color: false,
			checkFn: func(s string) bool {
				return strings.Contains(s, "Restart nginx service") &&
					strings.Contains(s, "RECOMMENDATIONS")
			},
		},
		{
			name: "analysis with color enabled",
			analysis: &ai.AnalysisResponse{
				OverallHealth:   ai.HealthHealthy,
				Summary:         "All good",
				Findings:        []ai.Finding{},
				Recommendations: []ai.Recommendation{},
			},
			color: true,
			checkFn: func(s string) bool {
				// With color, should contain ANSI escape codes
				return strings.Contains(s, "\033[") &&
					strings.Contains(s, "All good")
			},
		},
		{
			name: "analysis with tokens used",
			analysis: &ai.AnalysisResponse{
				OverallHealth:   ai.HealthHealthy,
				Summary:         "System OK",
				Findings:        []ai.Finding{},
				Recommendations: []ai.Recommendation{},
				TokensUsed: &ai.TokenUsage{
					TotalTokens: 1500,
				},
			},
			color: false,
			checkFn: func(s string) bool {
				return strings.Contains(s, "Tokens: 1500")
			},
		},
		{
			name: "finding with title",
			analysis: &ai.AnalysisResponse{
				OverallHealth: ai.HealthDegraded,
				Summary:       "Issues detected",
				Findings: []ai.Finding{
					{
						Category:    "memory",
						Severity:    diagnostics.SeverityWarning,
						Title:       "Memory Pressure",
						Description: "Memory usage is high",
					},
				},
				Recommendations: []ai.Recommendation{},
			},
			color: false,
			checkFn: func(s string) bool {
				return strings.Contains(s, "Memory Pressure") &&
					strings.Contains(s, "Memory usage is high")
			},
		},
		{
			name: "recommendation with commands",
			analysis: &ai.AnalysisResponse{
				OverallHealth: ai.HealthCritical,
				Summary:       "Action required",
				Findings:      []ai.Finding{},
				Recommendations: []ai.Recommendation{
					{
						Title:       "Restart Service",
						Priority:    ai.PriorityHigh,
						Description: "Service needs restart",
						Risk:        ai.RiskLow,
						Commands:    []string{"systemctl restart nginx", "systemctl status nginx"},
					},
				},
			},
			color: false,
			checkFn: func(s string) bool {
				return strings.Contains(s, "systemctl restart nginx") &&
					strings.Contains(s, "Commands:")
			},
		},
		{
			name: "recommendation with estimated impact",
			analysis: &ai.AnalysisResponse{
				OverallHealth: ai.HealthCritical,
				Summary:       "Optimization needed",
				Findings:      []ai.Finding{},
				Recommendations: []ai.Recommendation{
					{
						Title:           "Upgrade Memory",
						Priority:        ai.PriorityMedium,
						Description:     "Add more RAM",
						Risk:            ai.RiskModerate,
						EstimatedImpact: "50% performance improvement",
					},
				},
			},
			color: false,
			checkFn: func(s string) bool {
				return strings.Contains(s, "Impact: 50% performance improvement")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAIAnalysisText(tt.analysis, tt.color)

			if got == "" {
				t.Error("formatAIAnalysisText() returned empty string")
			}

			if tt.checkFn != nil && !tt.checkFn(got) {
				t.Errorf("formatAIAnalysisText() output doesn't match expected pattern:\n%s", got)
			}
		})
	}
}
