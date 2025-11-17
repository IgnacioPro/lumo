package remediation

import (
	"context"
	"fmt"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// Planner orchestrates the remediation process
type Planner struct {
	mapper        *Mapper
	executor      Executor
	approver      Approver
	auditLog      *AuditLogger
	dryRun        bool
}

// NewPlanner creates a new remediation planner
func NewPlanner(executor Executor, approver Approver, auditLog *AuditLogger, dryRun bool) *Planner {
	return &Planner{
		mapper:   NewMapper(),
		executor: executor,
		approver: approver,
		auditLog: auditLog,
		dryRun:   dryRun,
	}
}

// PlanFromDiagnostics creates a remediation plan from diagnostic results
func (p *Planner) PlanFromDiagnostics(report *diagnostics.Report, hostname, username string) *Plan {
	return p.mapper.MapFromDiagnostics(report, hostname, username)
}

// ExecutePlan executes a remediation plan with approval
func (p *Planner) ExecutePlan(ctx context.Context, plan *Plan, autoApproveAll bool) ([]*Result, error) {
	results := []*Result{}

	for _, action := range plan.Actions {
		// Ask for approval if needed
		approved, err := p.approver.ApproveAction(action, autoApproveAll)
		if err != nil {
			p.auditLog.Log(&AuditLog{
				PlanID:    plan.ID,
				Timestamp: time.Now(),
				Action:    fmt.Sprintf("approval_failed_%s", action.Type),
				Details: map[string]interface{}{
					"action_id": action.ID,
					"error":     err.Error(),
				},
				Status: "failed",
			})
			continue
		}

		if !approved {
			result := &Result{
				ActionID:   action.ID,
				Status:     StatusRejected,
				ExecutedAt: time.Now(),
				Output:     "User rejected this action",
			}
			results = append(results, result)

			p.auditLog.Log(&AuditLog{
				PlanID:    plan.ID,
				Timestamp: time.Now(),
				Action:    fmt.Sprintf("rejected_%s", action.Type),
				Details: map[string]interface{}{
					"action_id": action.ID,
				},
				Status: "rejected",
			})
			continue
		}

		// Execute the action
		result, err := p.executor.Execute(ctx, action)
		if err != nil {
			p.auditLog.Log(&AuditLog{
				PlanID:    plan.ID,
				Timestamp: time.Now(),
				Action:    fmt.Sprintf("execution_failed_%s", action.Type),
				Details: map[string]interface{}{
					"action_id": action.ID,
					"error":     err.Error(),
				},
				Status: "failed",
				Output: result.Output,
			})
			results = append(results, result)
			continue
		}

		// Verify the action if verification commands are available
		if len(action.VerifyCommands) > 0 {
			result, err := p.executor.Verify(ctx, action, result)
			if err != nil {
				p.auditLog.Log(&AuditLog{
					PlanID:    plan.ID,
					Timestamp: time.Now(),
					Action:    fmt.Sprintf("verification_failed_%s", action.Type),
					Details: map[string]interface{}{
						"action_id": action.ID,
						"error":     err.Error(),
					},
					Status: "failed",
					Output: result.VerificationOutput,
				})

				// Attempt rollback on verification failure
				if err := p.executor.Rollback(ctx, action, result); err != nil {
					p.auditLog.Log(&AuditLog{
						PlanID:    plan.ID,
						Timestamp: time.Now(),
						Action:    fmt.Sprintf("rollback_failed_%s", action.Type),
						Details: map[string]interface{}{
							"action_id": action.ID,
							"error":     err.Error(),
						},
						Status: "failed",
					})
				}

				results = append(results, result)
				continue
			}
		}

		p.auditLog.Log(&AuditLog{
			PlanID:    plan.ID,
			Timestamp: time.Now(),
			Action:    fmt.Sprintf("executed_%s", action.Type),
			Details: map[string]interface{}{
				"action_id": action.ID,
			},
			Status: "success",
			Output: result.Output,
		})

		results = append(results, result)
	}

	return results, nil
}

// ExecuteAction executes a single action with approval
func (p *Planner) ExecuteAction(ctx context.Context, action *Action, autoApprove bool) (*Result, error) {
	// Ask for approval
	approved, err := p.approver.ApproveAction(action, autoApprove)
	if err != nil {
		return nil, fmt.Errorf("approval failed: %w", err)
	}

	if !approved {
		return &Result{
			ActionID:   action.ID,
			Status:     StatusRejected,
			ExecutedAt: time.Now(),
			Output:     "User rejected this action",
		}, nil
	}

	// Execute
	result, err := p.executor.Execute(ctx, action)
	if err != nil {
		return result, err
	}

	// Verify
	if len(action.VerifyCommands) > 0 {
		result, err = p.executor.Verify(ctx, action, result)
		if err != nil {
			// Rollback on failure
			_ = p.executor.Rollback(ctx, action, result)
			return result, err
		}
	}

	return result, nil
}

// Approver interface for getting user approval
type Approver interface {
	// ApproveAction asks user for approval to execute an action
	// Returns true if approved, false if rejected
	ApproveAction(action *Action, autoApproveIfSafe bool) (bool, error)

	// ApprovePlan asks user to review and approve an entire plan
	ApprovePlan(plan *Plan, autoApproveAllSafe bool) (bool, error)
}

// InteractiveApprover gets approval through interactive prompts
type InteractiveApprover struct {
	promptFunc func(string) (bool, error)
}

// NewInteractiveApprover creates an interactive approval system
func NewInteractiveApprover(promptFunc func(string) (bool, error)) *InteractiveApprover {
	return &InteractiveApprover{
		promptFunc: promptFunc,
	}
}

// ApproveAction prompts user for action approval
func (a *InteractiveApprover) ApproveAction(action *Action, autoApproveIfSafe bool) (bool, error) {
	// Auto-approve safe actions if requested
	if autoApproveIfSafe && action.Risk == RiskSafe {
		return true, nil
	}

	prompt := fmt.Sprintf(`
────────────────────────────────────────
Action: %s
Risk Level: %s
Source: %s
────────────────────────────────────────
%s

Commands to execute:
`, action.Title, action.Risk, action.SourceCheck, action.Description)

	for i, cmd := range action.Commands {
		prompt += fmt.Sprintf("  %d) %s\n", i+1, cmd)
	}

	if len(action.RollbackCommands) > 0 {
		prompt += "\nRollback available:\n"
		for i, cmd := range action.RollbackCommands {
			prompt += fmt.Sprintf("  %d) %s\n", i+1, cmd)
		}
	}

	prompt += "\nApprove this action? (yes/no): "

	return a.promptFunc(prompt)
}

// ApprovePlan prompts user for plan approval
func (a *InteractiveApprover) ApprovePlan(plan *Plan, autoApproveAllSafe bool) (bool, error) {
	if autoApproveAllSafe && plan.CriticalActions == 0 && plan.ModerateActions == 0 {
		return true, nil
	}

	prompt := fmt.Sprintf(`
════════════════════════════════════════
REMEDIATION PLAN SUMMARY
════════════════════════════════════════
Host: %s@%s
Total Actions: %d
  Safe: %d
  Moderate: %d
  Critical: %d
Estimated Duration: %v
════════════════════════════════════════

`, plan.Username, plan.Hostname, plan.TotalActions, plan.SafeActions, plan.ModerateActions, plan.CriticalActions, plan.EstimatedDuration)

	if plan.CriticalActions > 0 {
		prompt += fmt.Sprintf("⚠️  WARNING: %d critical actions require explicit approval!\n", plan.CriticalActions)
	}
	if plan.ModerateActions > 0 {
		prompt += fmt.Sprintf("⚠️  WARNING: %d moderate risk actions\n", plan.ModerateActions)
	}

	prompt += "\nReview the plan above and approve? (yes/no): "

	return a.promptFunc(prompt)
}

// AuditLogger logs all remediation activities
type AuditLogger struct {
	logFile string
	logs    []*AuditLog
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logFile string) *AuditLogger {
	return &AuditLogger{
		logFile: logFile,
		logs:    []*AuditLog{},
	}
}

// Log adds an entry to the audit log
func (al *AuditLogger) Log(entry *AuditLog) {
	al.logs = append(al.logs, entry)
	// TODO: Write to file
}

// GetLogs returns all logged entries
func (al *AuditLogger) GetLogs() []*AuditLog {
	return al.logs
}
