package notifications

import (
	"fmt"
	"strings"
	"time"
)

// =============================================================================
// Agent Phase Constants and Types
// These define the dynamic AI Agent experience in notifications
// =============================================================================

// AgentPhase represents the current phase of the AI agent's work on an incident
type AgentPhase string

const (
	// AgentPhaseDetected - Initial incident detection
	AgentPhaseDetected AgentPhase = "detected"
	// AgentPhaseAnalyzing - AI is gathering context and analyzing
	AgentPhaseAnalyzing AgentPhase = "analyzing"
	// AgentPhaseIdentified - Root cause has been identified
	AgentPhaseIdentified AgentPhase = "identified"
	// AgentPhaseRemediate - Ready to apply a fix
	AgentPhaseRemediate AgentPhase = "remediate"
	// AgentPhaseExecuting - Currently applying the fix
	AgentPhaseExecuting AgentPhase = "executing"
	// AgentPhaseVerifying - Verifying the fix worked
	AgentPhaseVerifying AgentPhase = "verifying"
	// AgentPhaseCompleted - Issue successfully resolved
	AgentPhaseCompleted AgentPhase = "completed"
	// AgentPhaseFailed - Remediation failed
	AgentPhaseFailed AgentPhase = "failed"
	// AgentPhaseEscalated - Requires human intervention
	AgentPhaseEscalated AgentPhase = "escalated"
)

// Emoji returns the emoji representation for this phase
func (p AgentPhase) Emoji() string {
	switch p {
	case AgentPhaseDetected:
		return "🔍"
	case AgentPhaseAnalyzing:
		return "🔄"
	case AgentPhaseIdentified:
		return "🎯"
	case AgentPhaseRemediate:
		return "🔧"
	case AgentPhaseExecuting:
		return "⏳"
	case AgentPhaseVerifying:
		return "✔️"
	case AgentPhaseCompleted:
		return "✅"
	case AgentPhaseFailed:
		return "❌"
	case AgentPhaseEscalated:
		return "🚨"
	default:
		return "❔"
	}
}

// Message returns a short human-readable message for this phase
func (p AgentPhase) Message() string {
	switch p {
	case AgentPhaseDetected:
		return "Issue detected"
	case AgentPhaseAnalyzing:
		return "Analyzing..."
	case AgentPhaseIdentified:
		return "Root cause found"
	case AgentPhaseRemediate:
		return "Ready to fix"
	case AgentPhaseExecuting:
		return "Applying fix..."
	case AgentPhaseVerifying:
		return "Verifying..."
	case AgentPhaseCompleted:
		return "Resolved"
	case AgentPhaseFailed:
		return "Fix failed"
	case AgentPhaseEscalated:
		return "Needs attention"
	default:
		return "Unknown"
	}
}

// RiskLevel indicates the risk associated with a proposed fix
type RiskLevel string

const (
	// RiskSafe - Auto-remediation allowed without human approval
	RiskSafe RiskLevel = "safe"
	// RiskModerate - Requires human approval
	RiskModerate RiskLevel = "moderate"
	// RiskCritical - High-risk, always requires explicit approval
	RiskCritical RiskLevel = "critical"
)

// Emoji returns the emoji representation for this risk level
func (r RiskLevel) Emoji() string {
	switch r {
	case RiskSafe:
		return "🟢"
	case RiskModerate:
		return "🟡"
	case RiskCritical:
		return "🔴"
	default:
		return "⚪"
	}
}

// ProposedFix represents a suggested remediation action
type ProposedFix struct {
	// Title is a short description of the fix
	Title string `json:"title"`
	// Description provides more detail about what the fix does
	Description string `json:"description,omitempty"`
	// Command is the kubectl or shell command to execute
	Command string `json:"command,omitempty"`
	// Risk indicates the risk level of this fix
	Risk RiskLevel `json:"risk"`
	// AutoApprove indicates if this can be executed without human approval
	AutoApprove bool `json:"auto_approve"`
	// Rollback is the command to undo this fix
	Rollback string `json:"rollback,omitempty"`
	// EstimatedDuration is how long the fix is expected to take
	EstimatedDuration time.Duration `json:"estimated_duration,omitempty"`
	// Hash is a unique identifier for this fix proposal
	Hash string `json:"hash"`
}

// AgentUpdate represents a dynamic update to an incident notification
type AgentUpdate struct {
	// IncidentID is the unique identifier for the incident
	IncidentID string `json:"incident_id"`
	// Phase is the current agent phase
	Phase AgentPhase `json:"phase"`
	// Message is a brief status message (< 100 chars recommended)
	Message string `json:"message"`
	// Progress is a percentage (0-100) indicating completion
	Progress int `json:"progress"`
	// ProposedFix is the suggested remediation (if any)
	ProposedFix *ProposedFix `json:"proposed_fix,omitempty"`
	// Insights are key findings from the analysis
	Insights []string `json:"insights,omitempty"`
	// UpdatedAt is when this update was created
	UpdatedAt time.Time `json:"updated_at"`
}

// =============================================================================
// Progress Bar Rendering
// =============================================================================

const (
	progressBarLength = 10 // Number of characters in progress bar
	progressFilled    = "█"
	progressEmpty     = "░"
)

// RenderProgressBar creates a text-based progress bar for Slack
// Example: ████████░░ 80%
func RenderProgressBar(progress int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	filled := (progress * progressBarLength) / 100
	empty := progressBarLength - filled

	return fmt.Sprintf("%s%s %d%%",
		strings.Repeat(progressFilled, filled),
		strings.Repeat(progressEmpty, empty),
		progress)
}

// RenderProgressBarEmoji creates an emoji-based progress bar for Slack
// Useful for mobile rendering
// Example: 🟩🟩🟩🟩⬜⬜ 66%
func RenderProgressBarEmoji(progress int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	filled := (progress * 6) / 100 // Using 6 squares
	empty := 6 - filled

	return fmt.Sprintf("%s%s %d%%",
		strings.Repeat("🟩", filled),
		strings.Repeat("⬜", empty),
		progress)
}

// CalculateAgentProgress returns approximate progress percentage for a phase
func CalculateAgentProgress(phase AgentPhase) int {
	switch phase {
	case AgentPhaseDetected:
		return 10
	case AgentPhaseAnalyzing:
		return 30
	case AgentPhaseIdentified:
		return 50
	case AgentPhaseRemediate:
		return 60
	case AgentPhaseExecuting:
		return 75
	case AgentPhaseVerifying:
		return 90
	case AgentPhaseCompleted:
		return 100
	case AgentPhaseFailed:
		return 100
	case AgentPhaseEscalated:
		return 100
	default:
		return 0
	}
}
