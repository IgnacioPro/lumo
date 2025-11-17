package remediation

import "time"

// RiskLevel categorizes the risk of a remediation action
type RiskLevel string

const (
	// RiskSafe actions have minimal risk and can be auto-approved
	RiskSafe RiskLevel = "safe"
	// RiskModerate actions should be reviewed before execution
	RiskModerate RiskLevel = "moderate"
	// RiskCritical actions require explicit approval
	RiskCritical RiskLevel = "critical"
)

// ActionType categorizes the kind of remediation action
type ActionType string

const (
	// Service actions
	ActionRestartService ActionType = "restart_service"
	ActionStartService   ActionType = "start_service"
	ActionStopService    ActionType = "stop_service"

	// Process actions
	ActionKillProcess ActionType = "kill_process"
	ActionThrottleProcess ActionType = "throttle_process"

	// Disk actions
	ActionCleanCache  ActionType = "clean_cache"
	ActionCleanLogs   ActionType = "clean_logs"
	ActionCleanTemp   ActionType = "clean_temp"

	// Network actions
	ActionBlockIP ActionType = "block_ip"
	ActionClosePort ActionType = "close_port"

	// System actions
	ActionUpdatePackages ActionType = "update_packages"
	ActionApplySecurityPatch ActionType = "apply_security_patch"
	ActionRotateSSHKeys ActionType = "rotate_ssh_keys"
)

// Action represents a single remediation action
type Action struct {
	// Unique identifier for this action
	ID string `json:"id"`

	// Type of action to perform
	Type ActionType `json:"type"`

	// Risk level of this action
	Risk RiskLevel `json:"risk"`

	// Short human-readable title
	Title string `json:"title"`

	// Detailed description of the action and why it's needed
	Description string `json:"description"`

	// The diagnostic check that triggered this action
	SourceCheck string `json:"source_check"`

	// Metric value that triggered the action (e.g., "95% disk usage")
	Metric string `json:"metric"`

	// Command(s) to execute to perform the action
	Commands []string `json:"commands"`

	// Commands to verify the action was successful
	VerifyCommands []string `json:"verify_commands"`

	// Commands to rollback the action if needed
	RollbackCommands []string `json:"rollback_commands"`

	// Estimated time to complete
	EstimatedDuration time.Duration `json:"estimated_duration"`

	// Custom metadata
	Metadata map[string]string `json:"metadata"`
}

// Plan represents a set of remediation actions proposed for a system
type Plan struct {
	// Unique identifier for this plan
	ID string `json:"id"`

	// System being remediated
	Hostname string `json:"hostname"`
	Username string `json:"username"`

	// All proposed actions
	Actions []*Action `json:"actions"`

	// Summary counts
	TotalActions    int `json:"total_actions"`
	SafeActions     int `json:"safe_actions"`
	ModerateActions int `json:"moderate_actions"`
	CriticalActions int `json:"critical_actions"`

	// Estimated total time
	EstimatedDuration time.Duration `json:"estimated_duration"`

	// When the plan was created
	CreatedAt time.Time `json:"created_at"`
}

// Result represents the result of executing a remediation action
type Result struct {
	// ID of the action that was executed
	ActionID string `json:"action_id"`

	// Status of the execution
	Status ExecutionStatus `json:"status"`

	// Output from the command
	Output string `json:"output"`

	// Any errors that occurred
	Error string `json:"error,omitempty"`

	// Time it took to execute
	Duration time.Duration `json:"duration"`

	// Verification output (if verification was run)
	VerificationOutput string `json:"verification_output,omitempty"`

	// When the action was executed
	ExecutedAt time.Time `json:"executed_at"`

	// If needed, when rollback was performed
	RolledBackAt *time.Time `json:"rolled_back_at,omitempty"`
	RollbackError string `json:"rollback_error,omitempty"`
}

// ExecutionStatus represents the status of an action execution
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusApproved  ExecutionStatus = "approved"
	StatusRejected  ExecutionStatus = "rejected"
	StatusExecuting ExecutionStatus = "executing"
	StatusSuccess   ExecutionStatus = "success"
	StatusFailed    ExecutionStatus = "failed"
	StatusRolledBack ExecutionStatus = "rolled_back"
	StatusSkipped   ExecutionStatus = "skipped"
)

// AuditLog records all remediation activity for audit and rollback purposes
type AuditLog struct {
	// Plan ID for cross-referencing
	PlanID string `json:"plan_id"`

	// When this action happened
	Timestamp time.Time `json:"timestamp"`

	// What action was taken
	Action string `json:"action"`

	// Details of the action
	Details map[string]interface{} `json:"details"`

	// User who approved/executed the action
	User string `json:"user"`

	// Status of the action
	Status string `json:"status"`

	// Any output or error messages
	Output string `json:"output,omitempty"`
}
