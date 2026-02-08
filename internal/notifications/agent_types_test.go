package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentPhase_Emoji(t *testing.T) {
	tests := []struct {
		phase    AgentPhase
		expected string
	}{
		{AgentPhaseDetected, "🔍"},
		{AgentPhaseAnalyzing, "🔄"},
		{AgentPhaseIdentified, "🎯"},
		{AgentPhaseRemediate, "🔧"},
		{AgentPhaseExecuting, "⏳"},
		{AgentPhaseVerifying, "✔️"},
		{AgentPhaseCompleted, "✅"},
		{AgentPhaseFailed, "❌"},
		{AgentPhaseEscalated, "🚨"},
		{AgentPhase("unknown"), "❔"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.phase.Emoji())
		})
	}
}

func TestAgentPhase_Message(t *testing.T) {
	tests := []struct {
		phase    AgentPhase
		expected string
	}{
		{AgentPhaseDetected, "Issue detected"},
		{AgentPhaseAnalyzing, "Analyzing..."},
		{AgentPhaseIdentified, "Root cause found"},
		{AgentPhaseCompleted, "Resolved"},
		{AgentPhaseFailed, "Fix failed"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.phase.Message())
		})
	}
}

func TestRiskLevel_Emoji(t *testing.T) {
	tests := []struct {
		risk     RiskLevel
		expected string
	}{
		{RiskSafe, "🟢"},
		{RiskModerate, "🟡"},
		{RiskCritical, "🔴"},
		{RiskLevel("unknown"), "⚪"},
	}

	for _, tt := range tests {
		t.Run(string(tt.risk), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.risk.Emoji())
		})
	}
}

func TestRenderProgressBar(t *testing.T) {
	tests := []struct {
		progress int
		expected string
	}{
		{0, "░░░░░░░░░░ 0%"},
		{50, "█████░░░░░ 50%"},
		{100, "██████████ 100%"},
		{-10, "░░░░░░░░░░ 0%"},
		{150, "██████████ 100%"},
		{30, "███░░░░░░░ 30%"},
		{75, "███████░░░ 75%"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := RenderProgressBar(tt.progress)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenderProgressBarEmoji(t *testing.T) {
	tests := []struct {
		progress int
		expected string
	}{
		{0, "⬜⬜⬜⬜⬜⬜ 0%"},
		{50, "🟩🟩🟩⬜⬜⬜ 50%"},
		{100, "🟩🟩🟩🟩🟩🟩 100%"},
		{-10, "⬜⬜⬜⬜⬜⬜ 0%"},
		{150, "🟩🟩🟩🟩🟩🟩 100%"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := RenderProgressBarEmoji(tt.progress)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateAgentProgress(t *testing.T) {
	tests := []struct {
		phase    AgentPhase
		expected int
	}{
		{AgentPhaseDetected, 10},
		{AgentPhaseAnalyzing, 30},
		{AgentPhaseIdentified, 50},
		{AgentPhaseRemediate, 60},
		{AgentPhaseExecuting, 75},
		{AgentPhaseVerifying, 90},
		{AgentPhaseCompleted, 100},
		{AgentPhaseFailed, 100},
		{AgentPhaseEscalated, 100},
		{AgentPhase("unknown"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			assert.Equal(t, tt.expected, CalculateAgentProgress(tt.phase))
		})
	}
}
