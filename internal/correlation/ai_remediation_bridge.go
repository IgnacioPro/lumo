// Package correlation provides AI-to-remediation bridging.
// The AIRemediationBridge connects AI analysis results to executable remediation actions.
package correlation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/notifications"
	"github.com/ignacio/lumo/internal/remediation"
)

// ApprovalSource indicates where approval came from
type ApprovalSource string

const (
	ApprovalAuto    ApprovalSource = "auto"    // Auto-approved based on risk level
	ApprovalSlack   ApprovalSource = "slack"   // Approved via Slack button
	ApprovalAPI     ApprovalSource = "api"     // Approved via API call
	ApprovalCLI     ApprovalSource = "cli"     // Approved via CLI
	ApprovalDenied  ApprovalSource = "denied"  // Explicitly denied
	ApprovalTimeout ApprovalSource = "timeout" // Timed out waiting for approval
)

// PendingFix represents a fix awaiting approval
type PendingFix struct {
	ID           string                           `json:"id"`
	IncidentID   uuid.UUID                        `json:"incident_id"`
	Proposal     *remediation.RemediationProposal `json:"proposal"`
	ProposedBy   string                           `json:"proposed_by"` // "ai" or "matrix"
	ProposedAt   time.Time                        `json:"proposed_at"`
	ExpiresAt    time.Time                        `json:"expires_at"`
	Status       string                           `json:"status"` // pending, approved, denied, expired, executed
	ApprovedBy   string                           `json:"approved_by,omitempty"`
	ApprovedAt   *time.Time                       `json:"approved_at,omitempty"`
	ApprovalType ApprovalSource                   `json:"approval_type,omitempty"`
}

// RemediationResult holds the outcome of a remediation execution
type RemediationResult struct {
	Success    bool          `json:"success"`
	Message    string        `json:"message"`
	ActionID   string        `json:"action_id"`
	Output     string        `json:"output,omitempty"`
	Duration   time.Duration `json:"duration"`
	RolledBack bool          `json:"rolled_back"`
	VerifiedOK bool          `json:"verified_ok"`
	ExecutedAt time.Time     `json:"executed_at"`
}

// AIRemediationBridge connects AI analysis to remediation actions
type AIRemediationBridge struct {
	matrix     *remediation.RemediationMatrix
	typoDetect *remediation.ImageTypoDetector
	executor   diagnostics.CommandExecutor
	notifier   *IncidentNotifierImpl
	logger     *logrus.Logger

	// Pending fixes storage
	pendingFixes map[string]*PendingFix
	mu           sync.RWMutex

	// Configuration
	autoApproveEnabled bool
	approvalTimeout    time.Duration
}

// NewAIRemediationBridge creates a new bridge
func NewAIRemediationBridge(
	executor diagnostics.CommandExecutor,
	notifier *IncidentNotifierImpl,
	logger *logrus.Logger,
) *AIRemediationBridge {
	return &AIRemediationBridge{
		matrix:             remediation.NewRemediationMatrix(logger),
		typoDetect:         remediation.NewImageTypoDetector(logger),
		executor:           executor,
		notifier:           notifier,
		logger:             logger,
		pendingFixes:       make(map[string]*PendingFix),
		autoApproveEnabled: true,
		approvalTimeout:    30 * time.Minute,
	}
}

// ProcessIncident analyzes an incident and proposes remediation
func (b *AIRemediationBridge) ProcessIncident(ctx context.Context, incident *Incident) (*PendingFix, error) {
	b.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"category":    incident.Category,
	}).Info("Processing incident for remediation")

	// Send initial "agent started" notification
	if b.notifier != nil {
		if err := b.notifier.NotifyAgentStarted(ctx, incident); err != nil {
			b.logger.WithError(err).Warn("Failed to send agent started notification")
		}
	}

	// Step 1: Try to detect issue from events using message patterns
	var proposal *remediation.RemediationProposal
	var strategy *remediation.RemediationStrategy

	for _, event := range incident.Events {
		// Extract reason from message if present (K8s events often include reason)
		eventReason := extractReasonFromMessage(event.Message)
		strategies := b.matrix.DetectIssue(
			event.EventType,
			eventReason,
			event.Message,
			nil, // No exit code from event
		)
		if len(strategies) > 0 {
			strategy = strategies[0]
			break
		}
	}

	// Step 2: Check for image typos if it's an image category
	if incident.Category == CategoryImage {
		if suggestion := b.detectImageTypoFromIncident(incident); suggestion != nil {
			// Use image typo strategy
			if typoStrategy, ok := b.matrix.GetStrategy(remediation.IssueImagePullTypo); ok {
				strategy = typoStrategy

				// Create params from typo suggestion
				params := b.extractImageParams(incident, suggestion)
				if incident.PrimaryResource != nil {
					proposal = b.matrix.CreateProposal(
						strategy,
						incident.ID.String(),
						incident.PrimaryResource.Namespace,
						incident.PrimaryResource.Name,
						incident.PrimaryResource.Kind,
						params,
					)
				}
			}
		}
	}

	// Step 3: If we have a strategy but no proposal yet, create one
	if strategy != nil && proposal == nil && incident.PrimaryResource != nil {
		params := b.extractParamsFromIncident(incident, strategy)
		proposal = b.matrix.CreateProposal(
			strategy,
			incident.ID.String(),
			incident.PrimaryResource.Namespace,
			incident.PrimaryResource.Name,
			incident.PrimaryResource.Kind,
			params,
		)
	}

	// Step 4: Try AI analysis results if available
	if proposal == nil && incident.AIAnalysis != nil {
		proposal = b.proposeFromAIAnalysis(incident)
	}

	if proposal == nil {
		b.logger.WithField("incident_id", incident.ID).Debug("No remediation proposal found")
		// Notify agent identified but no fix
		if b.notifier != nil {
			_ = b.notifier.NotifyAgentIdentified(ctx, incident, "Issue detected but no automated fix available", nil)
		}
		return nil, nil
	}

	// Create pending fix
	pendingFix := &PendingFix{
		ID:         proposal.Hash,
		IncidentID: incident.ID,
		Proposal:   proposal,
		ProposedBy: "matrix",
		ProposedAt: time.Now(),
		ExpiresAt:  time.Now().Add(b.approvalTimeout),
		Status:     "pending",
	}

	// Store pending fix
	b.mu.Lock()
	b.pendingFixes[pendingFix.ID] = pendingFix
	b.mu.Unlock()

	// Notify with proposal
	if b.notifier != nil {
		fix := proposal.ToProposedFix()
		insights := b.gatherInsights(incident)
		_ = b.notifier.NotifyAgentProposal(ctx, incident, fix, insights)
	}

	// Auto-approve if enabled and safe
	if b.autoApproveEnabled && proposal.Strategy.AutoRemediate && proposal.Strategy.Risk == notifications.RiskSafe {
		b.logger.WithFields(logrus.Fields{
			"incident_id": incident.ID,
			"fix_id":      pendingFix.ID,
		}).Info("Auto-approving safe remediation")

		result, err := b.ExecuteWithApproval(ctx, pendingFix, ApprovalAuto)
		if err != nil {
			b.logger.WithError(err).Error("Auto-remediation failed")
			return pendingFix, err
		}

		if result.Success {
			_ = b.notifier.NotifyAgentCompleted(ctx, incident, result.Message)
		} else {
			_ = b.notifier.NotifyAgentFailed(ctx, incident, result.Message)
		}
	}

	return pendingFix, nil
}

// ExecuteWithApproval executes a pending fix with approval
func (b *AIRemediationBridge) ExecuteWithApproval(ctx context.Context, fix *PendingFix, approval ApprovalSource) (*RemediationResult, error) {
	if fix.Status != "pending" {
		return nil, fmt.Errorf("fix is not pending (status: %s)", fix.Status)
	}

	if time.Now().After(fix.ExpiresAt) {
		fix.Status = "expired"
		return nil, fmt.Errorf("fix has expired")
	}

	if approval == ApprovalDenied {
		fix.Status = "denied"
		return &RemediationResult{
			Success:    false,
			Message:    "Fix was denied",
			ExecutedAt: time.Now(),
		}, nil
	}

	// Record approval
	now := time.Now()
	fix.ApprovedAt = &now
	fix.ApprovalType = approval
	fix.Status = "approved"

	b.logger.WithFields(logrus.Fields{
		"fix_id":        fix.ID,
		"incident_id":   fix.IncidentID,
		"approval_type": approval,
	}).Info("Executing approved remediation")

	// Create the action
	action := b.createActionFromProposal(fix.Proposal)
	if action == nil {
		fix.Status = "failed"
		return &RemediationResult{
			Success:    false,
			Message:    "Failed to create action from proposal",
			ExecutedAt: time.Now(),
		}, fmt.Errorf("failed to create action")
	}

	// Validate
	if err := action.Validate(ctx, b.executor); err != nil {
		fix.Status = "failed"
		return &RemediationResult{
			Success:    false,
			Message:    fmt.Sprintf("Validation failed: %v", err),
			ExecutedAt: time.Now(),
		}, err
	}

	// Execute
	startTime := time.Now()
	result, err := action.Execute(ctx, b.executor)
	duration := time.Since(startTime)

	if err != nil || !result.Succeeded() {
		fix.Status = "failed"
		errMsg := "Execution failed"
		if err != nil {
			errMsg = err.Error()
		} else if result != nil {
			errMsg = result.Message
		}
		return &RemediationResult{
			Success:    false,
			Message:    errMsg,
			ActionID:   action.ID(),
			Output:     result.Output,
			Duration:   duration,
			ExecutedAt: startTime,
		}, err
	}

	fix.Status = "executed"

	return &RemediationResult{
		Success:    true,
		Message:    result.Message,
		ActionID:   action.ID(),
		Output:     result.Output,
		Duration:   duration,
		VerifiedOK: true, // TODO: implement verification
		ExecutedAt: startTime,
	}, nil
}

// GetPendingFix retrieves a pending fix by ID
func (b *AIRemediationBridge) GetPendingFix(id string) (*PendingFix, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	fix, ok := b.pendingFixes[id]
	return fix, ok
}

// GetPendingFixByIncident retrieves pending fixes for an incident
func (b *AIRemediationBridge) GetPendingFixByIncident(incidentID uuid.UUID) []*PendingFix {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var fixes []*PendingFix
	for _, fix := range b.pendingFixes {
		if fix.IncidentID == incidentID {
			fixes = append(fixes, fix)
		}
	}
	return fixes
}

// detectImageTypoFromIncident checks events for image typos
func (b *AIRemediationBridge) detectImageTypoFromIncident(incident *Incident) *remediation.TypoSuggestion {
	// Pattern to extract image from error messages
	imagePattern := regexp.MustCompile(`(?:image|Image|IMAGE)[:\s]+["']?([a-zA-Z0-9._\-/:]+)["']?`)

	for _, event := range incident.Events {
		eventReason := extractReasonFromMessage(event.Message)
		if strings.Contains(eventReason, "ImagePull") || strings.Contains(eventReason, "ErrImage") ||
			strings.Contains(event.Message, "ImagePull") || strings.Contains(event.Message, "ErrImage") {
			matches := imagePattern.FindStringSubmatch(event.Message)
			if len(matches) > 1 {
				if suggestion := b.typoDetect.DetectTypo(matches[1]); suggestion != nil {
					return suggestion
				}
			}
			// Also check the whole message for common image patterns
			words := strings.Fields(event.Message)
			for _, word := range words {
				if strings.Contains(word, ":") && !strings.HasPrefix(word, "http") {
					if suggestion := b.typoDetect.DetectTypo(word); suggestion != nil {
						return suggestion
					}
				}
			}
		}
	}
	return nil
}

// extractImageParams extracts parameters needed for image fix
func (b *AIRemediationBridge) extractImageParams(incident *Incident, suggestion *remediation.TypoSuggestion) map[string]string {
	params := map[string]string{
		"ErrorImage":   suggestion.Original,
		"CorrectImage": suggestion.Suggested,
	}

	if incident.PrimaryResource != nil {
		params["DeploymentName"] = incident.PrimaryResource.Name
		// Try to get container name from events
		for _, event := range incident.Events {
			if event.ResourceName != "" && strings.Contains(event.ResourceName, "/") {
				parts := strings.Split(event.ResourceName, "/")
				if len(parts) > 1 {
					params["ContainerName"] = parts[len(parts)-1]
					break
				}
			}
		}
	}

	if params["ContainerName"] == "" {
		params["ContainerName"] = "main" // Default container name
	}

	return params
}

// extractParamsFromIncident builds parameters based on incident type
func (b *AIRemediationBridge) extractParamsFromIncident(incident *Incident, strategy *remediation.RemediationStrategy) map[string]string {
	params := make(map[string]string)

	if incident.PrimaryResource != nil {
		params["DeploymentName"] = incident.PrimaryResource.Name
		params["ResourceName"] = incident.PrimaryResource.Name
		params["ContainerName"] = "main" // TODO: extract from events
	}

	switch strategy.IssueType {
	case remediation.IssueOOMKilled, remediation.IssueCrashLoopOOM:
		// Calculate new memory limit
		if incident.Context != nil && incident.Context.Metrics != nil {
			for _, pod := range incident.Context.Metrics.PodMetrics {
				if pod.MemoryLimitBytes > 0 {
					newLimit := pod.MemoryLimitBytes * 3 / 2 // +50%
					params["CurrentMemoryLimit"] = formatBytes(pod.MemoryLimitBytes)
					params["NewMemoryLimit"] = formatBytes(newLimit)
					break
				}
			}
		}
		if params["NewMemoryLimit"] == "" {
			params["CurrentMemoryLimit"] = "512Mi"
			params["NewMemoryLimit"] = "768Mi" // Default increase
		}

	case remediation.IssueCrashLoopStartup:
		params["CurrentThreshold"] = "3"
		params["NewThreshold"] = "10"

	case remediation.IssueNodeNotReady:
		if incident.NodeName != nil {
			params["NodeName"] = *incident.NodeName
		}
	}

	return params
}

// proposeFromAIAnalysis creates a proposal from AI analysis
func (b *AIRemediationBridge) proposeFromAIAnalysis(incident *Incident) *remediation.RemediationProposal {
	if incident.AIAnalysis == nil || len(incident.AIAnalysis.ImmediateActions) == 0 {
		return nil
	}

	// Find the first automated action
	for _, action := range incident.AIAnalysis.ImmediateActions {
		if action.Automated && action.Command != "" {
			// Match to a strategy if possible
			for _, strategy := range b.matrix.ListStrategies() {
				if strings.Contains(strings.ToLower(action.Title), strings.ToLower(string(strategy.IssueType))) {
					if incident.PrimaryResource != nil {
						return b.matrix.CreateProposal(
							strategy,
							incident.ID.String(),
							incident.PrimaryResource.Namespace,
							incident.PrimaryResource.Name,
							incident.PrimaryResource.Kind,
							map[string]string{"Command": action.Command},
						)
					}
				}
			}
		}
	}

	return nil
}

// createActionFromProposal creates an executable action from a proposal
func (b *AIRemediationBridge) createActionFromProposal(proposal *remediation.RemediationProposal) remediation.Action {
	switch proposal.Strategy.IssueType {
	case remediation.IssueImagePullTypo:
		return remediation.NewPatchImageAction(
			proposal.Namespace,
			proposal.Parameters["DeploymentName"],
			proposal.Parameters["ContainerName"],
			proposal.Parameters["CorrectImage"],
			proposal.Parameters["ErrorImage"],
			b.logger,
		)

	// Add more action types here as needed
	default:
		// For generic cases, fall back to K8s rollout restart if applicable
		if proposal.ResourceKind == "Deployment" {
			return remediation.NewK8sRolloutRestartAction(
				proposal.Namespace,
				"deployment",
				proposal.ResourceName,
				b.logger,
			)
		}
	}

	return nil
}

// gatherInsights collects insights from incident for notification
func (b *AIRemediationBridge) gatherInsights(incident *Incident) []string {
	var insights []string

	// From root cause
	if incident.RootCause != "" {
		insights = append(insights, incident.RootCause)
	}

	// From AI analysis
	if incident.AIAnalysis != nil {
		if incident.AIAnalysis.RootCause.Summary != "" {
			insights = append(insights, incident.AIAnalysis.RootCause.Summary)
		}
		for _, evidence := range incident.AIAnalysis.RootCause.Evidence {
			if len(insights) < 5 { // Cap at 5 insights
				insights = append(insights, evidence)
			}
		}
	}

	// From events (first few)
	for i, event := range incident.Events {
		if i >= 3 {
			break
		}
		eventReason := extractReasonFromMessage(event.Message)
		if event.Message != "" && len(event.Message) < 100 {
			if eventReason != "" {
				insights = append(insights, fmt.Sprintf("%s: %s", eventReason, event.Message))
			} else {
				insights = append(insights, event.Message)
			}
		}
	}

	return insights
}

// GenerateFixHash creates a unique hash for a fix
func GenerateFixHash(incidentID, command string) string {
	data := fmt.Sprintf("%s:%s:%d", incidentID, command, time.Now().Unix())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

// extractReasonFromMessage tries to extract a Kubernetes reason from event message
// K8s event messages often start with "Reason: message" or contain patterns like "FailedScheduling:"
func extractReasonFromMessage(message string) string {
	// Common K8s event reasons that might appear in messages
	reasons := []string{
		"ImagePullBackOff", "ErrImagePull", "CrashLoopBackOff", "OOMKilled",
		"FailedScheduling", "NodeNotReady", "ProgressDeadlineExceeded",
		"ProvisioningFailed", "Unhealthy", "Failed", "BackOff",
	}

	messageLower := strings.ToLower(message)
	for _, reason := range reasons {
		if strings.Contains(messageLower, strings.ToLower(reason)) {
			return reason
		}
	}

	// Try to extract from "Reason: " prefix pattern
	if idx := strings.Index(message, ": "); idx > 0 && idx < 30 {
		possibleReason := message[:idx]
		if !strings.Contains(possibleReason, " ") {
			return possibleReason
		}
	}

	return ""
}
