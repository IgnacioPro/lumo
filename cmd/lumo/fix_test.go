package main

import (
	"testing"

	"github.com/ignacio/lumo/internal/remediation"
)

func TestFormatRiskBadge(t *testing.T) {
	tests := []struct {
		name     string
		risk     remediation.RiskLevel
		expected string
	}{
		{
			name:     "safe risk",
			risk:     remediation.RiskSafe,
			expected: "✅ SAFE",
		},
		{
			name:     "moderate risk",
			risk:     remediation.RiskModerate,
			expected: "⚠️  MODERATE",
		},
		{
			name:     "critical risk",
			risk:     remediation.RiskCritical,
			expected: "⛔ CRITICAL",
		},
		{
			name:     "unknown risk",
			risk:     remediation.RiskLevel("unknown"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRiskBadge(tt.risk)
			if result != tt.expected {
				t.Errorf("formatRiskBadge(%q) = %q, want %q", tt.risk, result, tt.expected)
			}
		})
	}
}

func TestFormatStatusBadge(t *testing.T) {
	tests := []struct {
		name     string
		status   remediation.ActionStatus
		expected string
	}{
		{
			name:     "success status",
			status:   remediation.StatusSuccess,
			expected: "✅ SUCCESS",
		},
		{
			name:     "failed status",
			status:   remediation.StatusFailed,
			expected: "❌ FAILED",
		},
		{
			name:     "skipped status",
			status:   remediation.StatusSkipped,
			expected: "⏭️  SKIPPED",
		},
		{
			name:     "rejected status",
			status:   remediation.StatusRejected,
			expected: "🚫 REJECTED",
		},
		{
			name:     "rolled back status",
			status:   remediation.StatusRolledBack,
			expected: "↩️  ROLLED BACK",
		},
		{
			name:     "unknown status",
			status:   remediation.ActionStatus("unknown"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatStatusBadge(tt.status)
			if result != tt.expected {
				t.Errorf("formatStatusBadge(%q) = %q, want %q", tt.status, result, tt.expected)
			}
		})
	}
}
