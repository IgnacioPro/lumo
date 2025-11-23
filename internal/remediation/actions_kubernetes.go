package remediation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/sirupsen/logrus"
)

const (
	// Action types
	ActionK8sRolloutRestart = "kubernetes.rollout_restart"
	ActionK8sDeletePod      = "kubernetes.delete_pod"
)

// K8sRolloutRestartAction restarts a Kubernetes deployment or statefulset
type K8sRolloutRestartAction struct {
	*BaseAction
	namespace    string
	resourceType string
	resourceName string
}

// NewK8sRolloutRestartAction creates a new K8sRolloutRestartAction
func NewK8sRolloutRestartAction(namespace, resourceType, resourceName string, logger *logrus.Logger) *K8sRolloutRestartAction {
	if namespace == "" {
		namespace = "default"
	}
	
	// Validate resource type
	if resourceType != "deployment" && resourceType != "statefulset" && resourceType != "daemonset" {
		resourceType = "deployment" // Default
	}

	actionID := fmt.Sprintf("%s.%s.%s.%s", ActionK8sRolloutRestart, namespace, resourceType, resourceName)
	
	return &K8sRolloutRestartAction{
		BaseAction: NewBaseAction(
			actionID,
			fmt.Sprintf("Rollout restart %s %s/%s", resourceType, namespace, resourceName),
			fmt.Sprintf("Performs a rolling restart of the %s '%s' in namespace '%s'", resourceType, resourceName, namespace),
			CategorySystem, // Using System for now as Kubernetes fits best there or new K8s category
			RiskModerate,   // Rolling restart is generally safe but can cause temporary unavailability if not configured correctly
			true,           // Reversible via undo
			fmt.Sprintf("Will trigger a rolling restart of %s/%s. New pods will be created.", namespace, resourceName),
			logger,
		),
		namespace:    namespace,
		resourceType: resourceType,
		resourceName: resourceName,
	}
}

// NewK8sRolloutRestartActionFactory returns a factory for K8sRolloutRestartAction
func NewK8sRolloutRestartActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		namespace, _ := params["namespace"].(string)
		resourceType, _ := params["resource_type"].(string)
		resourceName, ok := params["resource_name"].(string)
		
		if !ok || resourceName == "" {
			return nil, fmt.Errorf("resource_name is required")
		}

		return NewK8sRolloutRestartAction(namespace, resourceType, resourceName, logger), nil
	}
}

// Validate checks if kubectl is available and the resource exists
func (a *K8sRolloutRestartAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	// Check kubectl availability
	if _, _, exitCode, _ := executor.ExecuteWithContext(ctx, "which kubectl"); exitCode != 0 {
		return fmt.Errorf("kubectl not found")
	}

	// Check if resource exists
	cmd := fmt.Sprintf("kubectl get %s %s -n %s", 
		shellQuote(a.resourceType), 
		shellQuote(a.resourceName), 
		shellQuote(a.namespace))
	
	if _, _, exitCode, _ := executor.ExecuteWithContext(ctx, cmd); exitCode != 0 {
		return fmt.Errorf("%s %s/%s not found", a.resourceType, a.namespace, a.resourceName)
	}

	return nil
}

// Execute performs the rollout restart
func (a *K8sRolloutRestartAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	// Perform rollout restart
	cmd := fmt.Sprintf("kubectl rollout restart %s %s -n %s",
		shellQuote(a.resourceType),
		shellQuote(a.resourceName),
		shellQuote(a.namespace))

	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	result.Output = fmt.Sprintf("stdout: %s\nstderr: %s", stdout, stderr)

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to restart %s %s/%s", a.resourceType, a.namespace, a.resourceName)
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("kubectl rollout restart failed: %w", err)
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully triggered restart for %s %s/%s", a.resourceType, a.namespace, a.resourceName)
	result.ChangesApplied = []string{fmt.Sprintf("Triggered rollout restart for %s/%s", a.namespace, a.resourceName)}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback undoes the rollout
func (a *K8sRolloutRestartAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	cmd := fmt.Sprintf("kubectl rollout undo %s %s -n %s",
		shellQuote(a.resourceType),
		shellQuote(a.resourceName),
		shellQuote(a.namespace))

	if _, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd); err != nil || exitCode != 0 {
		return fmt.Errorf("failed to undo rollout: %s", stderr)
	}

	return nil
}

// K8sDeletePodAction deletes a specific pod (forcing recreation if controlled by a controller)
type K8sDeletePodAction struct {
	*BaseAction
	namespace string
	podName   string
	force     bool
}

// NewK8sDeletePodAction creates a new K8sDeletePodAction
func NewK8sDeletePodAction(namespace, podName string, force bool, logger *logrus.Logger) *K8sDeletePodAction {
	if namespace == "" {
		namespace = "default"
	}

	actionID := fmt.Sprintf("%s.%s.%s", ActionK8sDeletePod, namespace, podName)
	
	return &K8sDeletePodAction{
		BaseAction: NewBaseAction(
			actionID,
			fmt.Sprintf("Delete pod %s/%s", namespace, podName),
			fmt.Sprintf("Deletes pod '%s' in namespace '%s'. If controlled by a deployment/statefulset, it will be recreated.", podName, namespace),
			CategorySystem, // Or CategoryKubernetes if added
			RiskModerate,   // Usually safe for controlled pods, but moderate risk to verify
			false,          // Cannot undelete a pod directly
			fmt.Sprintf("Pod %s/%s will be deleted.", namespace, podName),
			logger,
		),
		namespace: namespace,
		podName:   podName,
		force:     force,
	}
}

// NewK8sDeletePodActionFactory returns a factory for K8sDeletePodAction
func NewK8sDeletePodActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		namespace, _ := params["namespace"].(string)
		podName, ok := params["pod_name"].(string)
		force, _ := params["force"].(bool)

		if !ok || podName == "" {
			return nil, fmt.Errorf("pod_name is required")
		}

		return NewK8sDeletePodAction(namespace, podName, force, logger), nil
	}
}

// Validate checks if kubectl is available and the pod exists
func (a *K8sDeletePodAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	if _, _, exitCode, _ := executor.ExecuteWithContext(ctx, "which kubectl"); exitCode != 0 {
		return fmt.Errorf("kubectl not found")
	}

	cmd := fmt.Sprintf("kubectl get pod %s -n %s", shellQuote(a.podName), shellQuote(a.namespace))
	if _, _, exitCode, _ := executor.ExecuteWithContext(ctx, cmd); exitCode != 0 {
		return fmt.Errorf("pod %s/%s not found", a.namespace, a.podName)
	}

	return nil
}

// Execute deletes the pod
func (a *K8sDeletePodAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	args := []string{"delete", "pod", shellQuote(a.podName), "-n", shellQuote(a.namespace)}
	if a.force {
		args = append(args, "--force", "--grace-period=0")
	}

	cmd := fmt.Sprintf("kubectl %s", strings.Join(args, " "))
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	result.Output = fmt.Sprintf("stdout: %s\nstderr: %s", stdout, stderr)

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to delete pod %s/%s", a.namespace, a.podName)
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("kubectl delete pod failed: %w", err)
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully deleted pod %s/%s", a.namespace, a.podName)
	result.ChangesApplied = []string{fmt.Sprintf("Deleted pod %s/%s", a.namespace, a.podName)}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback is not supported for pod deletion
func (a *K8sDeletePodAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	return fmt.Errorf("pod deletion cannot be rolled back directly")
}

// init registers the actions
func init() {
	logger := logrus.StandardLogger()
	registry := GetDefaultRegistry(logger)
	_ = registry.Register(ActionK8sRolloutRestart, NewK8sRolloutRestartActionFactory())
	_ = registry.Register(ActionK8sDeletePod, NewK8sDeletePodActionFactory())
}
