package main

import (
	"testing"

	"github.com/ignacio/lumo/internal/remediation"
)

func TestFormatRiskBadge(t *testing.T) {
	tests := []struct {
		name string
		risk remediation.RiskLevel
		want string
	}{
		{
			name: "safe risk",
			risk: remediation.RiskSafe,
			want: "✅ SAFE",
		},
		{
			name: "moderate risk",
			risk: remediation.RiskModerate,
			want: "⚠️  MODERATE",
		},
		{
			name: "critical risk",
			risk: remediation.RiskCritical,
			want: "⛔ CRITICAL",
		},
		{
			name: "unknown risk level",
			risk: remediation.RiskLevel("unknown"),
			want: "unknown",
		},
		{
			name: "empty risk level",
			risk: remediation.RiskLevel(""),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRiskBadge(tt.risk)
			if got != tt.want {
				t.Errorf("formatRiskBadge(%q) = %q, want %q", tt.risk, got, tt.want)
			}
		})
	}
}

func TestFormatStatusBadge(t *testing.T) {
	tests := []struct {
		name   string
		status remediation.ActionStatus
		want   string
	}{
		{
			name:   "success status",
			status: remediation.StatusSuccess,
			want:   "✅ SUCCESS",
		},
		{
			name:   "failed status",
			status: remediation.StatusFailed,
			want:   "❌ FAILED",
		},
		{
			name:   "skipped status",
			status: remediation.StatusSkipped,
			want:   "⏭️  SKIPPED",
		},
		{
			name:   "rejected status",
			status: remediation.StatusRejected,
			want:   "🚫 REJECTED",
		},
		{
			name:   "rolled back status",
			status: remediation.StatusRolledBack,
			want:   "↩️  ROLLED BACK",
		},
		{
			name:   "pending status (default case)",
			status: remediation.StatusPending,
			want:   "pending",
		},
		{
			name:   "approved status (default case)",
			status: remediation.StatusApproved,
			want:   "approved",
		},
		{
			name:   "unknown status",
			status: remediation.ActionStatus("unknown"),
			want:   "unknown",
		},
		{
			name:   "empty status",
			status: remediation.ActionStatus(""),
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatStatusBadge(tt.status)
			if got != tt.want {
				t.Errorf("formatStatusBadge(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
