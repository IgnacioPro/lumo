package correlation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/notifications"
)

// NotifyAgentStarted sends an initial "agent is analyzing" update.
func (n *IncidentNotifierImpl) NotifyAgentStarted(ctx context.Context, incident *Incident) error {
	return n.notifyAgentPhase(ctx, incident, notifications.AgentPhaseAnalyzing, "Lumo is analyzing this incident now.", nil, nil)
}

// NotifyAgentIdentified sends an update when the root cause is identified.
func (n *IncidentNotifierImpl) NotifyAgentIdentified(
	ctx context.Context,
	incident *Incident,
	message string,
	insights []string,
) error {
	if strings.TrimSpace(message) == "" {
		message = "Root cause identified."
	}
	return n.notifyAgentPhase(ctx, incident, notifications.AgentPhaseIdentified, message, nil, insights)
}

// NotifyAgentProposal sends an update with a proposed remediation.
func (n *IncidentNotifierImpl) NotifyAgentProposal(
	ctx context.Context,
	incident *Incident,
	fix *notifications.ProposedFix,
	insights []string,
) error {
	msg := "A remediation proposal is ready for review."
	if fix != nil && fix.Title != "" {
		msg = fmt.Sprintf("Proposed fix: %s", fix.Title)
	}
	return n.notifyAgentPhase(ctx, incident, notifications.AgentPhaseRemediate, msg, fix, insights)
}

// NotifyAgentCompleted sends a success update when remediation completes.
func (n *IncidentNotifierImpl) NotifyAgentCompleted(ctx context.Context, incident *Incident, message string) error {
	if strings.TrimSpace(message) == "" {
		message = "Remediation completed successfully."
	}
	return n.notifyAgentPhase(ctx, incident, notifications.AgentPhaseCompleted, message, nil, nil)
}

// NotifyAgentFailed sends an update when remediation fails.
func (n *IncidentNotifierImpl) NotifyAgentFailed(ctx context.Context, incident *Incident, message string) error {
	if strings.TrimSpace(message) == "" {
		message = "Remediation failed and requires attention."
	}
	return n.notifyAgentPhase(ctx, incident, notifications.AgentPhaseFailed, message, nil, nil)
}

func (n *IncidentNotifierImpl) notifyAgentPhase(
	ctx context.Context,
	incident *Incident,
	phase notifications.AgentPhase,
	message string,
	fix *notifications.ProposedFix,
	insights []string,
) error {
	if incident == nil {
		return fmt.Errorf("incident is nil")
	}

	progress := notifications.CalculateAgentProgress(phase)
	title := fmt.Sprintf("%s Agent Update: %s", phase.Emoji(), incident.Title)
	phaseLine := fmt.Sprintf("*%s* %s", phase.Emoji(), phase.Message())
	progressLine := fmt.Sprintf("Progress: `%s`", notifications.RenderProgressBar(progress))
	body := fmt.Sprintf("%s\n%s\n\n%s", phaseLine, progressLine, message)

	level := notifications.LevelInfo
	switch phase {
	case notifications.AgentPhaseIdentified, notifications.AgentPhaseRemediate, notifications.AgentPhaseExecuting, notifications.AgentPhaseVerifying:
		level = notifications.LevelWarning
	case notifications.AgentPhaseCompleted:
		level = notifications.LevelSuccess
	case notifications.AgentPhaseFailed, notifications.AgentPhaseEscalated:
		level = notifications.LevelError
	}

	notification := notifications.NewNotification(title, body, level).
		WithField("incident_id", incident.ID.String()).
		WithField("event_id", incident.ID.String()).
		WithField("Phase", string(phase)).
		WithField("Progress", fmt.Sprintf("%d%%", progress))

	if len(insights) > 0 {
		notification.WithField("hypothesis", truncateStr(insights[0], 220))
	}
	if len(insights) > 1 {
		var lines []string
		for i, insight := range insights[1:] {
			if i >= 4 {
				break
			}
			lines = append(lines, "• "+truncateStr(insight, 180))
		}
		if len(lines) > 0 {
			notification.WithField("signals", strings.Join(lines, "\n"))
		}
	}

	if fix != nil {
		if fix.Hash != "" {
			notification.WithField("fix_hash", fix.Hash)
		}
		if fix.Command != "" {
			notification.WithField("proposed_fix", fix.Command)
		} else if fix.Title != "" {
			notification.WithField("proposed_fix", fix.Title)
		}
		if fix.Rollback != "" {
			notification.WithField("rollback", fix.Rollback)
		}
		if fix.Risk != "" {
			notification.WithField("risk", string(fix.Risk))
		}
		if fix.EstimatedDuration > 0 {
			notification.WithField("estimated_duration", fix.EstimatedDuration.Round(time.Second).String())
		}
	}

	if n.apiBaseURL != "" {
		notification.WithField("audit_url", fmt.Sprintf("%s/api/v1/incidents/%s/analysis", n.apiBaseURL, incident.ID.String()))
	}

	return n.sendToAll(ctx, notification)
}
