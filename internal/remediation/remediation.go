// Package remediation provides automated remediation actions for system issues.
package remediation

import (
	"context"
	"fmt"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/sirupsen/logrus"
)

// RiskLevel indicates the risk associated with a remediation action.
type RiskLevel string

const (
	// RiskSafe actions are low-risk and can be auto-approved.
	// Examples: cleaning temporary files, restarting non-critical services.
	RiskSafe RiskLevel = "safe"

	// RiskModerate actions require user approval but have low system impact.
	// Examples: restarting critical services, killing processes.
	RiskModerate RiskLevel = "moderate"

	// RiskCritical actions are high-risk and always require explicit approval.
	// Examples: modifying system configurations, force-killing processes.
	RiskCritical RiskLevel = "critical"
)

// String returns the string representation of a RiskLevel.
func (r RiskLevel) String() string {
	return string(r)
}

// ActionStatus represents the current state of a remediation action.
type ActionStatus string

const (
	// StatusPending indicates the action is queued but not started.
	StatusPending ActionStatus = "pending"

	// StatusApproved indicates the action was approved and is ready to execute.
	StatusApproved ActionStatus = "approved"

	// StatusRejected indicates the action was rejected by the user.
	StatusRejected ActionStatus = "rejected"

	// StatusExecuting indicates the action is currently running.
	StatusExecuting ActionStatus = "executing"

	// StatusSuccess indicates the action completed successfully.
	StatusSuccess ActionStatus = "success"

	// StatusFailed indicates the action failed during execution.
	StatusFailed ActionStatus = "failed"

	// StatusRolledBack indicates the action was rolled back.
	StatusRolledBack ActionStatus = "rolled_back"

	// StatusSkipped indicates the action was skipped (e.g., dry-run mode).
	StatusSkipped ActionStatus = "skipped"
)

// String returns the string representation of an ActionStatus.
func (s ActionStatus) String() string {
	return string(s)
}

// ActionCategory categorizes remediation actions by the system component they affect.
type ActionCategory string

const (
	// CategoryService for service management actions.
	CategoryService ActionCategory = "service"

	// CategoryDisk for disk cleanup and management actions.
	CategoryDisk ActionCategory = "disk"

	// CategoryProcess for process management actions.
	CategoryProcess ActionCategory = "process"

	// CategoryNetwork for network-related fixes.
	CategoryNetwork ActionCategory = "network"

	// CategorySystem for system-level configuration changes.
	CategorySystem ActionCategory = "system"

	// CategorySecurity for security-related fixes.
	CategorySecurity ActionCategory = "security"
)

// String returns the string representation of an ActionCategory.
func (c ActionCategory) String() string {
	return string(c)
}

// Action represents a remediation action that can be executed.
type Action interface {
	// ID returns a unique identifier for this action.
	ID() string

	// Name returns a human-readable name for this action.
	Name() string

	// Description returns a detailed description of what this action does.
	Description() string

	// Category returns the category of this action.
	Category() ActionCategory

	// Risk returns the risk level of this action.
	Risk() RiskLevel

	// IsReversible indicates whether this action can be rolled back.
	IsReversible() bool

	// Validate checks if prerequisites are met for executing this action.
	Validate(ctx context.Context, executor diagnostics.CommandExecutor) error

	// Execute performs the remediation action.
	Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error)

	// Rollback reverses the changes made by Execute (only if IsReversible returns true).
	Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error

	// EstimateImpact provides an estimate of the action's impact on the system.
	EstimateImpact() string
}

// ActionResult contains the result of executing a remediation action.
type ActionResult struct {
	// ActionID is the ID of the action that was executed.
	ActionID string `json:"action_id"`

	// Status is the final status of the action.
	Status ActionStatus `json:"status"`

	// Message provides details about the result.
	Message string `json:"message"`

	// StartTime is when the action started executing.
	StartTime time.Time `json:"start_time"`

	// EndTime is when the action finished executing.
	EndTime time.Time `json:"end_time"`

	// Duration is how long the action took to execute.
	Duration time.Duration `json:"duration"`

	// Output contains any stdout/stderr from the action.
	Output string `json:"output,omitempty"`

	// Error contains error details if the action failed.
	Error string `json:"error,omitempty"`

	// RollbackData stores state needed to rollback this action.
	RollbackData map[string]interface{} `json:"rollback_data,omitempty"`

	// ChangesApplied describes what was actually changed.
	ChangesApplied []string `json:"changes_applied,omitempty"`
}

// Succeeded returns true if the action completed successfully.
func (r *ActionResult) Succeeded() bool {
	return r.Status == StatusSuccess
}

// Failed returns true if the action failed.
func (r *ActionResult) Failed() bool {
	return r.Status == StatusFailed
}

// WasRolledBack returns true if the action was rolled back.
func (r *ActionResult) WasRolledBack() bool {
	return r.Status == StatusRolledBack
}

// RemediationPlan represents a plan of actions to remediate detected issues.
type RemediationPlan struct {
	// Actions is the list of actions to execute.
	Actions []Action

	// DryRun indicates whether to simulate actions without executing them.
	DryRun bool

	// AutoApprove enables automatic approval of safe actions.
	AutoApprove bool

	// SkipCategories lists action categories to skip.
	SkipCategories []ActionCategory

	// Logger for logging plan execution.
	Logger *logrus.Logger
}

// NewRemediationPlan creates a new remediation plan.
func NewRemediationPlan(logger *logrus.Logger) *RemediationPlan {
	return &RemediationPlan{
		Actions:        []Action{},
		DryRun:         false,
		AutoApprove:    false,
		SkipCategories: []ActionCategory{},
		Logger:         logger,
	}
}

// AddAction adds an action to the plan.
func (p *RemediationPlan) AddAction(action Action) {
	p.Actions = append(p.Actions, action)
}

// FilteredActions returns actions that should be executed based on skip settings.
func (p *RemediationPlan) FilteredActions() []Action {
	if len(p.SkipCategories) == 0 {
		return p.Actions
	}

	skipMap := make(map[ActionCategory]bool)
	for _, cat := range p.SkipCategories {
		skipMap[cat] = true
	}

	filtered := []Action{}
	for _, action := range p.Actions {
		if !skipMap[action.Category()] {
			filtered = append(filtered, action)
		}
	}
	return filtered
}

// SuggestFromDiagnostics generates a remediation plan from diagnostic results.
func SuggestFromDiagnostics(report *diagnostics.Report, logger *logrus.Logger) *RemediationPlan {
	plan := NewRemediationPlan(logger)

	// This will be implemented with specific suggestion logic
	// based on diagnostic findings
	logger.Debug("Generating remediation suggestions from diagnostic report")

	return plan
}

// BaseAction provides common functionality for all actions.
type BaseAction struct {
	id          string
	name        string
	description string
	category    ActionCategory
	risk        RiskLevel
	reversible  bool
	impact      string
	logger      *logrus.Logger
}

// NewBaseAction creates a new BaseAction with common fields.
func NewBaseAction(id, name, description string, category ActionCategory, risk RiskLevel, reversible bool, impact string, logger *logrus.Logger) *BaseAction {
	return &BaseAction{
		id:          id,
		name:        name,
		description: description,
		category:    category,
		risk:        risk,
		reversible:  reversible,
		impact:      impact,
		logger:      logger,
	}
}

// ID returns the action's unique identifier.
func (b *BaseAction) ID() string {
	return b.id
}

// Name returns the action's human-readable name.
func (b *BaseAction) Name() string {
	return b.name
}

// Description returns the action's detailed description.
func (b *BaseAction) Description() string {
	return b.description
}

// Category returns the action's category.
func (b *BaseAction) Category() ActionCategory {
	return b.category
}

// Risk returns the action's risk level.
func (b *BaseAction) Risk() RiskLevel {
	return b.risk
}

// IsReversible returns whether the action can be rolled back.
func (b *BaseAction) IsReversible() bool {
	return b.reversible
}

// EstimateImpact returns an estimate of the action's system impact.
func (b *BaseAction) EstimateImpact() string {
	return b.impact
}

// Validate performs basic validation. Subclasses should override with specific checks.
func (b *BaseAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if executor == nil {
		return fmt.Errorf("executor is nil")
	}
	return nil
}

// Execute must be implemented by concrete action types.
func (b *BaseAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	return nil, fmt.Errorf("Execute not implemented for %s", b.id)
}

// Rollback must be implemented by concrete action types if reversible.
func (b *BaseAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	if !b.reversible {
		return fmt.Errorf("action %s is not reversible", b.id)
	}
	return fmt.Errorf("Rollback not implemented for %s", b.id)
}
