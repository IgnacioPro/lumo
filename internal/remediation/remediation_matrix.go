// Package remediation provides automated remediation actions for Kubernetes issues.
// The remediation matrix maps detected issues to appropriate fix strategies.
package remediation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/notifications"
)

// =============================================================================
// Issue Types - Common K8s problems that can be remediated
// =============================================================================

// IssueType identifies a specific type of Kubernetes issue
type IssueType string

const (
	// Image pull issues
	IssueImagePullTypo     IssueType = "image_pull_typo"
	IssueImagePullAuth     IssueType = "image_pull_auth"
	IssueImagePullNotFound IssueType = "image_pull_not_found"

	// Container crash issues
	IssueCrashLoopOOM     IssueType = "crash_loop_oom"
	IssueCrashLoopConfig  IssueType = "crash_loop_config"
	IssueCrashLoopStartup IssueType = "crash_loop_startup"

	// Resource issues
	IssueOOMKilled      IssueType = "oom_killed"
	IssueResourceQuota  IssueType = "resource_quota"
	IssueHPAMaxReplicas IssueType = "hpa_max_replicas"

	// Storage issues
	IssuePVCProvisionFailed IssueType = "pvc_provision_failed"

	// Node issues
	IssueNodeNotReady IssueType = "node_not_ready"

	// Deployment issues
	IssueDeploymentStuck IssueType = "deployment_stuck"
)

// =============================================================================
// Detection Rules - How to identify issues
// =============================================================================

// DetectionRule defines how to detect a specific issue from events and context
type DetectionRule struct {
	// EventMessagePatterns are regex patterns to match in event messages
	EventMessagePatterns []string
	// EventReason matches the Kubernetes event reason
	EventReason string
	// EventType matches the event type (Warning, Normal)
	EventType string
	// ContainerExitCode matches specific exit codes
	ContainerExitCode *int32
	// RequiredLabels are labels that must be present
	RequiredLabels map[string]string
	// ResourceKind filters by resource type
	ResourceKind string
	// LogPatterns are patterns to search for in container logs
	LogPatterns []string
}

// Matches checks if this rule matches the given event context
func (r *DetectionRule) Matches(eventType, reason, message string, exitCode *int32) bool {
	// Check event type if specified
	if r.EventType != "" && r.EventType != eventType {
		return false
	}

	// Check reason if specified
	if r.EventReason != "" && r.EventReason != reason {
		return false
	}

	// Check exit code if specified
	if r.ContainerExitCode != nil && exitCode != nil {
		if *r.ContainerExitCode != *exitCode {
			return false
		}
	}

	// Check message patterns
	if len(r.EventMessagePatterns) > 0 {
		matched := false
		for _, pattern := range r.EventMessagePatterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if re.MatchString(message) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// =============================================================================
// Remediation Strategy - How to fix issues
// =============================================================================

// RemediationStrategy defines how to remediate a specific issue type
type RemediationStrategy struct {
	// IssueType identifier
	IssueType IssueType
	// DisplayName is human-readable name
	DisplayName string
	// Description explains what this strategy does
	Description string
	// DetectionRules define how to identify this issue
	DetectionRules []DetectionRule
	// Risk level of the remediation
	Risk notifications.RiskLevel
	// AutoRemediate indicates if this can be fixed without human approval
	AutoRemediate bool
	// ActionTemplate describes the fix action
	ActionTemplate ActionTemplate
	// RollbackTemplate describes how to undo the fix (if reversible)
	RollbackTemplate *ActionTemplate
	// VerificationMethod describes how to verify the fix worked
	VerificationMethod string
	// EstimatedDuration is how long the fix typically takes
	EstimatedDuration time.Duration
}

// ActionTemplate describes a parameterized remediation action
type ActionTemplate struct {
	// ActionType identifies the action to create
	ActionType string
	// Command is a template string for the kubectl command
	Command string
	// Description is a human-readable description template
	Description string
}

// =============================================================================
// Proposed Fix - A concrete fix proposal
// =============================================================================

// RemediationProposal is a concrete fix for a specific incident
type RemediationProposal struct {
	// Strategy that generated this proposal
	Strategy *RemediationStrategy
	// IncidentID this proposal is for
	IncidentID string
	// Namespace of the affected resource
	Namespace string
	// ResourceName of the affected resource
	ResourceName string
	// ResourceKind of the affected resource
	ResourceKind string
	// Parameters for the action template
	Parameters map[string]string
	// Command is the rendered kubectl command
	Command string
	// Rollback is the rendered rollback command (if any)
	Rollback string
	// CreatedAt is when this proposal was created
	CreatedAt time.Time
	// Hash is a unique identifier for this proposal
	Hash string
}

// ToProposedFix converts to a notifications.ProposedFix for Slack display
func (p *RemediationProposal) ToProposedFix() *notifications.ProposedFix {
	return &notifications.ProposedFix{
		Title:             p.Strategy.DisplayName,
		Description:       p.Strategy.Description,
		Command:           p.Command,
		Risk:              p.Strategy.Risk,
		AutoApprove:       p.Strategy.AutoRemediate,
		Rollback:          p.Rollback,
		EstimatedDuration: p.Strategy.EstimatedDuration,
		Hash:              p.Hash,
	}
}

// GenerateHash creates a unique hash for this proposal
func (p *RemediationProposal) GenerateHash() string {
	data := fmt.Sprintf("%s:%s:%s:%s:%s:%d",
		p.IncidentID, p.Strategy.IssueType, p.Namespace, p.ResourceName, p.Command, p.CreatedAt.Unix())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // First 8 bytes = 16 hex chars
}

// =============================================================================
// Remediation Matrix - Registry of all strategies
// =============================================================================

// RemediationMatrix manages all remediation strategies
type RemediationMatrix struct {
	strategies map[IssueType]*RemediationStrategy
	logger     *logrus.Logger
}

// NewRemediationMatrix creates a new matrix with default strategies
func NewRemediationMatrix(logger *logrus.Logger) *RemediationMatrix {
	m := &RemediationMatrix{
		strategies: make(map[IssueType]*RemediationStrategy),
		logger:     logger,
	}
	m.registerDefaultStrategies()
	return m
}

// registerDefaultStrategies adds all built-in remediation strategies
func (m *RemediationMatrix) registerDefaultStrategies() {
	exitCode137 := int32(137)

	// ImagePullBackOff - Typo in image name
	m.Register(&RemediationStrategy{
		IssueType:   IssueImagePullTypo,
		DisplayName: "Fix Image Typo",
		Description: "Corrects a typo in the container image name",
		DetectionRules: []DetectionRule{{
			EventReason:          "Failed",
			EventMessagePatterns: []string{`ImagePullBackOff`, `ErrImagePull`},
		}},
		Risk:          notifications.RiskSafe,
		AutoRemediate: true,
		ActionTemplate: ActionTemplate{
			ActionType:  "patch_image",
			Command:     "kubectl set image deployment/{{.DeploymentName}} {{.ContainerName}}={{.CorrectImage}} -n {{.Namespace}}",
			Description: "Change image from {{.ErrorImage}} to {{.CorrectImage}}",
		},
		RollbackTemplate: &ActionTemplate{
			ActionType:  "patch_image",
			Command:     "kubectl set image deployment/{{.DeploymentName}} {{.ContainerName}}={{.ErrorImage}} -n {{.Namespace}}",
			Description: "Revert image to {{.ErrorImage}}",
		},
		VerificationMethod: "pod_running",
		EstimatedDuration:  30 * time.Second,
	})

	// ImagePullBackOff - Auth issue
	m.Register(&RemediationStrategy{
		IssueType:   IssueImagePullAuth,
		DisplayName: "Configure Image Pull Secret",
		Description: "Registry authentication failed - needs imagePullSecrets",
		DetectionRules: []DetectionRule{{
			EventReason:          "Failed",
			EventMessagePatterns: []string{`unauthorized`, `access denied`, `authentication required`},
		}},
		Risk:          notifications.RiskModerate,
		AutoRemediate: false,
		ActionTemplate: ActionTemplate{
			ActionType:  "suggest",
			Command:     "kubectl patch serviceaccount default -p '{\"imagePullSecrets\": [{\"name\": \"{{.SecretName}}\"}]}' -n {{.Namespace}}",
			Description: "Add imagePullSecrets to the service account or deployment",
		},
		VerificationMethod: "pod_running",
		EstimatedDuration:  2 * time.Minute,
	})

	// CrashLoopBackOff - OOM
	m.Register(&RemediationStrategy{
		IssueType:   IssueCrashLoopOOM,
		DisplayName: "Increase Memory Limit",
		Description: "Container was killed due to OOM - increasing memory limit by 50%",
		DetectionRules: []DetectionRule{{
			EventReason:       "OOMKilled",
			ContainerExitCode: &exitCode137,
		}},
		Risk:          notifications.RiskSafe,
		AutoRemediate: true,
		ActionTemplate: ActionTemplate{
			ActionType:  "adjust_resources",
			Command:     "kubectl set resources deployment/{{.DeploymentName}} --limits=memory={{.NewMemoryLimit}} -n {{.Namespace}}",
			Description: "Increase memory limit from {{.CurrentMemoryLimit}} to {{.NewMemoryLimit}}",
		},
		RollbackTemplate: &ActionTemplate{
			ActionType:  "adjust_resources",
			Command:     "kubectl set resources deployment/{{.DeploymentName}} --limits=memory={{.CurrentMemoryLimit}} -n {{.Namespace}}",
			Description: "Revert memory limit to {{.CurrentMemoryLimit}}",
		},
		VerificationMethod: "pod_running",
		EstimatedDuration:  45 * time.Second,
	})

	// OOMKilled (standalone event)
	m.Register(&RemediationStrategy{
		IssueType:   IssueOOMKilled,
		DisplayName: "Increase Memory Limit",
		Description: "Container was OOMKilled - increasing memory limit",
		DetectionRules: []DetectionRule{{
			EventReason:       "OOMKilled",
			ContainerExitCode: &exitCode137,
		}},
		Risk:          notifications.RiskSafe,
		AutoRemediate: true,
		ActionTemplate: ActionTemplate{
			ActionType:  "adjust_resources",
			Command:     "kubectl set resources deployment/{{.DeploymentName}} --limits=memory={{.NewMemoryLimit}} -n {{.Namespace}}",
			Description: "Increase memory limit to {{.NewMemoryLimit}}",
		},
		VerificationMethod: "pod_running",
		EstimatedDuration:  45 * time.Second,
	})

	// CrashLoopBackOff - Startup probe failure
	m.Register(&RemediationStrategy{
		IssueType:   IssueCrashLoopStartup,
		DisplayName: "Extend Startup Probe Timeout",
		Description: "Container failing startup probe - extending timeout",
		DetectionRules: []DetectionRule{{
			EventReason:          "Unhealthy",
			EventMessagePatterns: []string{`Startup probe failed`},
		}},
		Risk:          notifications.RiskSafe,
		AutoRemediate: true,
		ActionTemplate: ActionTemplate{
			ActionType:  "patch_probe",
			Command:     "kubectl patch deployment/{{.DeploymentName}} -p '{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"{{.ContainerName}}\",\"startupProbe\":{\"failureThreshold\":{{.NewThreshold}}}}]}}}}' -n {{.Namespace}}",
			Description: "Increase startup probe failure threshold from {{.CurrentThreshold}} to {{.NewThreshold}}",
		},
		VerificationMethod: "pod_running",
		EstimatedDuration:  60 * time.Second,
	})

	// Node NotReady
	m.Register(&RemediationStrategy{
		IssueType:   IssueNodeNotReady,
		DisplayName: "Cordon Unhealthy Node",
		Description: "Node is not ready - cordoning to prevent new pod scheduling",
		DetectionRules: []DetectionRule{{
			EventReason:          "NodeNotReady",
			EventMessagePatterns: []string{`node.*not ready`, `NodeNotReady`},
			ResourceKind:         "Node",
		}},
		Risk:          notifications.RiskCritical,
		AutoRemediate: false,
		ActionTemplate: ActionTemplate{
			ActionType:  "cordon_node",
			Command:     "kubectl cordon {{.NodeName}}",
			Description: "Mark node {{.NodeName}} as unschedulable",
		},
		RollbackTemplate: &ActionTemplate{
			ActionType:  "uncordon_node",
			Command:     "kubectl uncordon {{.NodeName}}",
			Description: "Allow scheduling on node {{.NodeName}} again",
		},
		VerificationMethod: "node_cordoned",
		EstimatedDuration:  10 * time.Second,
	})

	// Deployment stuck
	m.Register(&RemediationStrategy{
		IssueType:   IssueDeploymentStuck,
		DisplayName: "Rollback Deployment",
		Description: "Deployment is stuck - rolling back to previous version",
		DetectionRules: []DetectionRule{{
			EventReason:          "ProgressDeadlineExceeded",
			EventMessagePatterns: []string{`exceeded its progress deadline`},
			ResourceKind:         "Deployment",
		}},
		Risk:          notifications.RiskModerate,
		AutoRemediate: false,
		ActionTemplate: ActionTemplate{
			ActionType:  "rollback",
			Command:     "kubectl rollout undo deployment/{{.DeploymentName}} -n {{.Namespace}}",
			Description: "Rollback deployment to previous revision",
		},
		VerificationMethod: "deployment_available",
		EstimatedDuration:  2 * time.Minute,
	})

	// PVC Provisioning Failed
	m.Register(&RemediationStrategy{
		IssueType:   IssuePVCProvisionFailed,
		DisplayName: "Diagnose PVC Issue",
		Description: "PVC provisioning failed - requires manual investigation",
		DetectionRules: []DetectionRule{{
			EventReason:          "ProvisioningFailed",
			EventMessagePatterns: []string{`Failed to provision volume`, `no persistent volumes available`},
			ResourceKind:         "PersistentVolumeClaim",
		}},
		Risk:          notifications.RiskCritical,
		AutoRemediate: false,
		ActionTemplate: ActionTemplate{
			ActionType:  "diagnose",
			Command:     "kubectl describe pvc {{.PVCName}} -n {{.Namespace}} && kubectl get storageclass",
			Description: "Check PVC status and available storage classes",
		},
		VerificationMethod: "pvc_bound",
		EstimatedDuration:  5 * time.Minute,
	})
}

// Register adds a new remediation strategy
func (m *RemediationMatrix) Register(strategy *RemediationStrategy) {
	m.strategies[strategy.IssueType] = strategy
	m.logger.WithFields(logrus.Fields{
		"issue_type": strategy.IssueType,
		"risk":       strategy.Risk,
		"auto":       strategy.AutoRemediate,
	}).Debug("Registered remediation strategy")
}

// GetStrategy returns a strategy by issue type
func (m *RemediationMatrix) GetStrategy(issueType IssueType) (*RemediationStrategy, bool) {
	s, ok := m.strategies[issueType]
	return s, ok
}

// DetectIssue analyzes event context and returns matching strategies
func (m *RemediationMatrix) DetectIssue(eventType, reason, message string, exitCode *int32) []*RemediationStrategy {
	var matches []*RemediationStrategy

	for _, strategy := range m.strategies {
		for _, rule := range strategy.DetectionRules {
			if rule.Matches(eventType, reason, message, exitCode) {
				matches = append(matches, strategy)
				break // One match per strategy is enough
			}
		}
	}

	return matches
}

// CreateProposal generates a remediation proposal for an incident
func (m *RemediationMatrix) CreateProposal(
	strategy *RemediationStrategy,
	incidentID string,
	namespace string,
	resourceName string,
	resourceKind string,
	params map[string]string,
) *RemediationProposal {
	proposal := &RemediationProposal{
		Strategy:     strategy,
		IncidentID:   incidentID,
		Namespace:    namespace,
		ResourceName: resourceName,
		ResourceKind: resourceKind,
		Parameters:   params,
		CreatedAt:    time.Now(),
	}

	// Render command template
	proposal.Command = m.renderTemplate(strategy.ActionTemplate.Command, namespace, resourceName, params)

	// Render rollback if available
	if strategy.RollbackTemplate != nil {
		proposal.Rollback = m.renderTemplate(strategy.RollbackTemplate.Command, namespace, resourceName, params)
	}

	// Generate hash
	proposal.Hash = proposal.GenerateHash()

	return proposal
}

// renderTemplate replaces placeholders in command templates
func (m *RemediationMatrix) renderTemplate(template, namespace, resourceName string, params map[string]string) string {
	result := template
	result = strings.ReplaceAll(result, "{{.Namespace}}", namespace)
	result = strings.ReplaceAll(result, "{{.ResourceName}}", resourceName)

	for key, value := range params {
		result = strings.ReplaceAll(result, "{{."+key+"}}", value)
	}

	return result
}

// ListStrategies returns all registered strategies
func (m *RemediationMatrix) ListStrategies() []*RemediationStrategy {
	var list []*RemediationStrategy
	for _, s := range m.strategies {
		list = append(list, s)
	}
	return list
}

// ListAutoRemediable returns strategies that can be auto-remediated
func (m *RemediationMatrix) ListAutoRemediable() []*RemediationStrategy {
	var list []*RemediationStrategy
	for _, s := range m.strategies {
		if s.AutoRemediate {
			list = append(list, s)
		}
	}
	return list
}
