package remediation

import (
	"context"
	"fmt"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/sirupsen/logrus"
)

// Executor handles the execution of remediation actions with proper state management.
type Executor struct {
	executor    diagnostics.CommandExecutor
	auditor     *Auditor
	approver    *Approver
	logger      *logrus.Logger
	dryRun      bool
	autoApprove bool
}

// NewExecutor creates a new remediation executor.
func NewExecutor(executor diagnostics.CommandExecutor, auditor *Auditor, approver *Approver, logger *logrus.Logger, dryRun, autoApprove bool) *Executor {
	return &Executor{
		executor:    executor,
		auditor:     auditor,
		approver:    approver,
		logger:      logger,
		dryRun:      dryRun,
		autoApprove: autoApprove,
	}
}

// ExecutionReport contains the results of executing a remediation plan.
type ExecutionReport struct {
	StartTime    time.Time       `json:"start_time"`
	EndTime      time.Time       `json:"end_time"`
	Duration     time.Duration   `json:"duration"`
	TotalActions int             `json:"total_actions"`
	Executed     int             `json:"executed"`
	Succeeded    int             `json:"succeeded"`
	Failed       int             `json:"failed"`
	Skipped      int             `json:"skipped"`
	Rejected     int             `json:"rejected"`
	RolledBack   int             `json:"rolled_back"`
	Results      []*ActionResult `json:"results"`
	DryRun       bool            `json:"dry_run"`
}

// ExecutePlan executes a remediation plan and returns a report of the results.
func (e *Executor) ExecutePlan(ctx context.Context, plan *RemediationPlan) (*ExecutionReport, error) {
	startTime := time.Now()

	report := &ExecutionReport{
		StartTime:    startTime,
		TotalActions: len(plan.Actions),
		Results:      make([]*ActionResult, 0),
		DryRun:       e.dryRun || plan.DryRun,
	}

	e.logger.WithFields(logrus.Fields{
		"total_actions": report.TotalActions,
		"dry_run":       report.DryRun,
		"auto_approve":  e.autoApprove || plan.AutoApprove,
	}).Info("Starting remediation plan execution")

	// Get filtered actions based on skip categories
	actions := plan.FilteredActions()
	if len(actions) < len(plan.Actions) {
		e.logger.WithFields(logrus.Fields{
			"skipped_count": len(plan.Actions) - len(actions),
		}).Info("Some actions filtered due to skip categories")
	}

	// Execute each action
	for _, action := range actions {
		result := e.executeAction(ctx, action, e.autoApprove || plan.AutoApprove)
		report.Results = append(report.Results, result)

		// Update counters
		switch result.Status {
		case StatusSuccess:
			report.Executed++
			report.Succeeded++
		case StatusFailed:
			report.Executed++
			report.Failed++
		case StatusSkipped:
			report.Skipped++
		case StatusRejected:
			report.Rejected++
		case StatusRolledBack:
			report.RolledBack++
		}

		// Log the result
		if err := e.auditor.LogAction(action, result); err != nil {
			e.logger.WithError(err).Warn("Failed to log action to audit log")
		}
	}

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)

	e.logger.WithFields(logrus.Fields{
		"duration":    report.Duration,
		"executed":    report.Executed,
		"succeeded":   report.Succeeded,
		"failed":      report.Failed,
		"skipped":     report.Skipped,
		"rejected":    report.Rejected,
		"rolled_back": report.RolledBack,
	}).Info("Remediation plan execution completed")

	return report, nil
}

// executeAction executes a single remediation action with validation and approval.
func (e *Executor) executeAction(ctx context.Context, action Action, autoApprove bool) *ActionResult {
	result := &ActionResult{
		ActionID:  action.ID(),
		Status:    StatusPending,
		StartTime: time.Now(),
	}

	logger := e.logger.WithFields(logrus.Fields{
		"action_id":   action.ID(),
		"action_name": action.Name(),
		"category":    action.Category(),
		"risk":        action.Risk(),
	})

	logger.Info("Processing remediation action")

	// Step 1: Validate prerequisites
	if err := action.Validate(ctx, e.executor); err != nil {
		result.Status = StatusFailed
		result.Message = "Validation failed"
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		logger.WithError(err).Warn("Action validation failed")
		return result
	}

	// Step 2: Get user approval
	approved, err := e.approver.RequestApprovalWithContext(ctx, action, autoApprove)
	if err != nil {
		result.Status = StatusFailed
		result.Message = "Approval process failed"
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		logger.WithError(err).Error("Approval process failed")
		return result
	}

	if !approved {
		result.Status = StatusRejected
		result.Message = "Action rejected by user"
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		logger.Info("Action rejected by user")
		return result
	}

	result.Status = StatusApproved
	logger.Info("Action approved")

	// Step 3: Execute (or simulate if dry-run)
	if e.dryRun {
		result.Status = StatusSkipped
		result.Message = "Skipped (dry-run mode)"
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		logger.Info("Action skipped (dry-run mode)")
		return result
	}

	result.Status = StatusExecuting
	logger.Info("Executing action")

	// Execute the action
	execResult, err := action.Execute(ctx, e.executor)
	if err != nil {
		result.Status = StatusFailed
		result.Message = "Execution failed"
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		logger.WithError(err).Error("Action execution failed")

		// Attempt rollback if action is reversible
		if action.IsReversible() {
			logger.Info("Attempting to rollback failed action")
			if rollbackErr := action.Rollback(ctx, e.executor, result); rollbackErr != nil {
				logger.WithError(rollbackErr).Error("Rollback failed")
				result.Error = fmt.Sprintf("%s; rollback also failed: %v", err, rollbackErr)
			} else {
				result.Status = StatusRolledBack
				result.Message = "Execution failed, successfully rolled back"
				logger.Info("Action successfully rolled back")
			}
		}
		return result
	}

	// Merge execution result
	if execResult != nil {
		result.Status = execResult.Status
		result.Message = execResult.Message
		result.Output = execResult.Output
		result.Error = execResult.Error
		result.RollbackData = execResult.RollbackData
		result.ChangesApplied = execResult.ChangesApplied
	} else {
		result.Status = StatusSuccess
		result.Message = "Action completed successfully"
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	logger.WithFields(logrus.Fields{
		"duration": result.Duration,
		"status":   result.Status,
	}).Info("Action execution completed")

	return result
}

// RollbackAction attempts to rollback a previously executed action.
func (e *Executor) RollbackAction(ctx context.Context, action Action, result *ActionResult) error {
	if !action.IsReversible() {
		return fmt.Errorf("action %s is not reversible", action.ID())
	}

	logger := e.logger.WithFields(logrus.Fields{
		"action_id":   action.ID(),
		"action_name": action.Name(),
	})

	logger.Info("Attempting to rollback action")

	if e.dryRun {
		logger.Info("Rollback skipped (dry-run mode)")
		return nil
	}

	if err := action.Rollback(ctx, e.executor, result); err != nil {
		logger.WithError(err).Error("Rollback failed")
		return fmt.Errorf("rollback failed for action %s: %w", action.ID(), err)
	}

	logger.Info("Action rolled back successfully")

	// Update audit log
	rollbackResult := &ActionResult{
		ActionID:  action.ID(),
		Status:    StatusRolledBack,
		Message:   "Action manually rolled back",
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}
	if err := e.auditor.LogAction(action, rollbackResult); err != nil {
		e.logger.WithError(err).Warn("Failed to log rollback action to audit log")
	}

	return nil
}
