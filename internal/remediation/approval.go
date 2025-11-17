package remediation

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// Approver handles user approval for remediation actions.
type Approver struct {
	logger *logrus.Logger
	reader *bufio.Reader
}

// NewApprover creates a new approval handler.
func NewApprover(logger *logrus.Logger) *Approver {
	return &Approver{
		logger: logger,
		reader: bufio.NewReader(os.Stdin),
	}
}

// RequestApproval requests user approval for an action.
// Returns true if approved, false if rejected.
func (a *Approver) RequestApproval(action Action, autoApprove bool) (bool, error) {
	// Auto-approve safe actions if enabled
	if autoApprove && action.Risk() == RiskSafe {
		a.logger.WithFields(logrus.Fields{
			"action_id": action.ID(),
			"risk":      action.Risk(),
		}).Debug("Auto-approving safe action")
		return true, nil
	}

	// Display action details
	a.displayActionDetails(action)

	// Prompt for approval
	fmt.Print("\n")
	switch action.Risk() {
	case RiskSafe:
		fmt.Print("Approve this safe action? [Y/n]: ")
	case RiskModerate:
		fmt.Print("⚠️  Approve this moderate-risk action? [y/N]: ")
	case RiskCritical:
		fmt.Print("⛔ Approve this CRITICAL-risk action? [y/N]: ")
	default:
		fmt.Print("Approve this action? [y/N]: ")
	}

	// Read user input
	input, err := a.reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read user input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))

	// Determine approval based on risk level
	switch action.Risk() {
	case RiskSafe:
		// Default to yes for safe actions
		approved := input == "" || input == "y" || input == "yes"
		return approved, nil
	case RiskModerate, RiskCritical:
		// Require explicit yes for moderate/critical actions
		approved := input == "y" || input == "yes"
		return approved, nil
	default:
		// Default to no for unknown risk levels
		approved := input == "y" || input == "yes"
		return approved, nil
	}
}

// displayActionDetails prints detailed information about an action to the console.
func (a *Approver) displayActionDetails(action Action) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("Action: %s\n", action.Name())
	fmt.Printf("ID: %s\n", action.ID())
	fmt.Printf("Category: %s\n", action.Category())
	fmt.Printf("Risk Level: %s\n", a.formatRiskLevel(action.Risk()))
	fmt.Printf("Reversible: %v\n", action.IsReversible())
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("Description:\n  %s\n", action.Description())
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("Estimated Impact:\n  %s\n", action.EstimateImpact())
	fmt.Println(strings.Repeat("=", 70))
}

// formatRiskLevel returns a formatted string for the risk level with visual indicators.
func (a *Approver) formatRiskLevel(risk RiskLevel) string {
	switch risk {
	case RiskSafe:
		return "✅ SAFE"
	case RiskModerate:
		return "⚠️  MODERATE"
	case RiskCritical:
		return "⛔ CRITICAL"
	default:
		return string(risk)
	}
}

// BatchApprovalRequest represents a batch approval request for multiple actions.
type BatchApprovalRequest struct {
	Actions      []Action
	AutoApprove  bool
	ApprovalMode string // "all", "none", "selective"
}

// RequestBatchApproval requests approval for a batch of actions.
// Returns a map of action IDs to approval decisions.
func (a *Approver) RequestBatchApproval(request *BatchApprovalRequest) (map[string]bool, error) {
	approvals := make(map[string]bool)

	if len(request.Actions) == 0 {
		return approvals, nil
	}

	// Display summary
	a.displayBatchSummary(request.Actions)

	// Determine approval mode
	mode := request.ApprovalMode
	if mode == "" {
		mode = a.promptApprovalMode()
	}

	switch mode {
	case "all":
		// Approve all actions
		for _, action := range request.Actions {
			approvals[action.ID()] = true
		}
		fmt.Println("\n✅ All actions approved")

	case "none":
		// Reject all actions
		for _, action := range request.Actions {
			approvals[action.ID()] = false
		}
		fmt.Println("\n❌ All actions rejected")

	case "selective":
		// Individual approval for each action
		fmt.Println("\n--- Individual Action Approval ---")
		for _, action := range request.Actions {
			approved, err := a.RequestApproval(action, request.AutoApprove)
			if err != nil {
				return approvals, err
			}
			approvals[action.ID()] = approved
		}

	default:
		return approvals, fmt.Errorf("unknown approval mode: %s", mode)
	}

	return approvals, nil
}

// displayBatchSummary displays a summary of actions in a batch approval request.
func (a *Approver) displayBatchSummary(actions []Action) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("Remediation Plan Summary (%d actions)\n", len(actions))
	fmt.Println(strings.Repeat("=", 70))

	// Count actions by risk level
	riskCounts := make(map[RiskLevel]int)
	categoryCounts := make(map[ActionCategory]int)

	for _, action := range actions {
		riskCounts[action.Risk()]++
		categoryCounts[action.Category()]++
	}

	fmt.Println("\nActions by Risk Level:")
	for _, risk := range []RiskLevel{RiskSafe, RiskModerate, RiskCritical} {
		if count := riskCounts[risk]; count > 0 {
			fmt.Printf("  %s: %d\n", a.formatRiskLevel(risk), count)
		}
	}

	fmt.Println("\nActions by Category:")
	for category, count := range categoryCounts {
		fmt.Printf("  %s: %d\n", category, count)
	}

	fmt.Println("\nAction Details:")
	for i, action := range actions {
		fmt.Printf("  %d. [%s] %s - %s\n",
			i+1,
			a.formatRiskLevel(action.Risk()),
			action.Name(),
			action.Category())
	}
	fmt.Println(strings.Repeat("=", 70))
}

// promptApprovalMode prompts the user to select an approval mode.
func (a *Approver) promptApprovalMode() string {
	fmt.Println("\nApproval Options:")
	fmt.Println("  1. all - Approve all actions")
	fmt.Println("  2. none - Reject all actions")
	fmt.Println("  3. selective - Review each action individually")
	fmt.Print("\nSelect approval mode [1/2/3] (default: 3): ")

	input, err := a.reader.ReadString('\n')
	if err != nil {
		a.logger.WithError(err).Warn("Failed to read approval mode, defaulting to selective")
		return "selective"
	}

	input = strings.TrimSpace(input)

	switch input {
	case "1", "all":
		return "all"
	case "2", "none":
		return "none"
	case "3", "selective", "":
		return "selective"
	default:
		a.logger.WithField("input", input).Warn("Invalid approval mode, defaulting to selective")
		return "selective"
	}
}
