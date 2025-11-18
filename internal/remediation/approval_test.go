package remediation

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestApprover_FormatRiskLevel(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&testLogWriter{t: t})

	approver := &Approver{
		logger: logger,
	}

	tests := []struct {
		name string
		risk RiskLevel
		want string
	}{
		{
			name: "safe risk level",
			risk: RiskSafe,
			want: "✅ SAFE",
		},
		{
			name: "moderate risk level",
			risk: RiskModerate,
			want: "⚠️  MODERATE",
		},
		{
			name: "critical risk level",
			risk: RiskCritical,
			want: "⛔ CRITICAL",
		},
		{
			name: "unknown risk level",
			risk: RiskLevel("unknown"),
			want: "unknown",
		},
		{
			name: "empty risk level",
			risk: RiskLevel(""),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := approver.formatRiskLevel(tt.risk)
			if got != tt.want {
				t.Errorf("formatRiskLevel(%q) = %q, want %q", tt.risk, got, tt.want)
			}
		})
	}
}
