package correlation

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// K8sHealthChecker checks Kubernetes resource health
type K8sHealthChecker struct {
	client kubernetes.Interface
	logger *logrus.Entry
}

// NewK8sHealthChecker creates a new Kubernetes health checker
func NewK8sHealthChecker(client kubernetes.Interface, logger *logrus.Logger) *K8sHealthChecker {
	return &K8sHealthChecker{
		client: client,
		logger: logger.WithField("component", "k8s-health-checker"),
	}
}

// CheckResourcesHealthy checks if all affected resources in an incident are healthy
func (h *K8sHealthChecker) CheckResourcesHealthy(ctx context.Context, incident *Incident) (bool, error) {
	if h.client == nil {
		return false, fmt.Errorf("kubernetes client not configured")
	}

	if len(incident.AffectedResources) == 0 {
		// No resources to check, consider healthy
		return true, nil
	}

	h.logger.WithFields(logrus.Fields{
		"incident_id":    incident.ID,
		"resource_count": len(incident.AffectedResources),
	}).Debug("Checking resource health")

	allHealthy := true

	for _, resource := range incident.AffectedResources {
		healthy, err := h.checkResourceHealth(ctx, resource)
		if err != nil {
			h.logger.WithError(err).WithFields(logrus.Fields{
				"kind":      resource.Kind,
				"name":      resource.Name,
				"namespace": resource.Namespace,
			}).Debug("Failed to check resource health")
			// Continue checking other resources
			allHealthy = false
			continue
		}

		if !healthy {
			h.logger.WithFields(logrus.Fields{
				"kind":      resource.Kind,
				"name":      resource.Name,
				"namespace": resource.Namespace,
			}).Debug("Resource not healthy")
			allHealthy = false
		}
	}

	return allHealthy, nil
}

// checkResourceHealth checks health of a single resource
func (h *K8sHealthChecker) checkResourceHealth(ctx context.Context, resource AffectedResource) (bool, error) {
	switch resource.Kind {
	case "Pod":
		return h.checkPodHealth(ctx, resource.Namespace, resource.Name)
	case "Deployment":
		return h.checkDeploymentHealth(ctx, resource.Namespace, resource.Name)
	case "StatefulSet":
		return h.checkStatefulSetHealth(ctx, resource.Namespace, resource.Name)
	case "DaemonSet":
		return h.checkDaemonSetHealth(ctx, resource.Namespace, resource.Name)
	case "ReplicaSet":
		return h.checkReplicaSetHealth(ctx, resource.Namespace, resource.Name)
	case "Node":
		return h.checkNodeHealth(ctx, resource.Name)
	case "PersistentVolumeClaim":
		return h.checkPVCHealth(ctx, resource.Namespace, resource.Name)
	case "Job":
		return h.checkJobHealth(ctx, resource.Namespace, resource.Name)
	default:
		// Unknown resource type, assume healthy to avoid blocking closure
		h.logger.WithField("kind", resource.Kind).Debug("Unknown resource kind, assuming healthy")
		return true, nil
	}
}

// checkPodHealth checks if a pod is running and ready
func (h *K8sHealthChecker) checkPodHealth(ctx context.Context, namespace, name string) (bool, error) {
	pod, err := h.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// Pod not found might mean it was deleted/recreated, check deployment instead
		return false, fmt.Errorf("failed to get pod: %w", err)
	}

	// Check pod phase
	if pod.Status.Phase != corev1.PodRunning {
		return false, nil
	}

	// Check all containers are ready
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
			return false, nil
		}
	}

	// Check no containers are in CrashLoopBackOff or waiting
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			if cs.State.Waiting.Reason == "CrashLoopBackOff" ||
				cs.State.Waiting.Reason == "ImagePullBackOff" ||
				cs.State.Waiting.Reason == "ErrImagePull" {
				return false, nil
			}
		}
		if !cs.Ready {
			return false, nil
		}
	}

	return true, nil
}

// checkDeploymentHealth checks if a deployment is available
func (h *K8sHealthChecker) checkDeploymentHealth(ctx context.Context, namespace, name string) (bool, error) {
	deployment, err := h.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get deployment: %w", err)
	}

	// Check if desired replicas match available replicas
	if deployment.Status.AvailableReplicas < *deployment.Spec.Replicas {
		return false, nil
	}

	// Check conditions
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == "Available" && condition.Status != corev1.ConditionTrue {
			return false, nil
		}
		if condition.Type == "Progressing" && condition.Status != corev1.ConditionTrue {
			return false, nil
		}
	}

	return true, nil
}

// checkStatefulSetHealth checks if a statefulset is ready
func (h *K8sHealthChecker) checkStatefulSetHealth(ctx context.Context, namespace, name string) (bool, error) {
	sts, err := h.client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get statefulset: %w", err)
	}

	// Check if all replicas are ready
	if sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		return false, nil
	}

	return true, nil
}

// checkDaemonSetHealth checks if a daemonset is ready
func (h *K8sHealthChecker) checkDaemonSetHealth(ctx context.Context, namespace, name string) (bool, error) {
	ds, err := h.client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get daemonset: %w", err)
	}

	// Check if desired equals ready
	if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
		return false, nil
	}

	// Check if any are unavailable
	if ds.Status.NumberUnavailable > 0 {
		return false, nil
	}

	return true, nil
}

// checkReplicaSetHealth checks if a replicaset is ready
func (h *K8sHealthChecker) checkReplicaSetHealth(ctx context.Context, namespace, name string) (bool, error) {
	rs, err := h.client.AppsV1().ReplicaSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get replicaset: %w", err)
	}

	// Check if all replicas are ready
	if rs.Status.ReadyReplicas < *rs.Spec.Replicas {
		return false, nil
	}

	return true, nil
}

// checkNodeHealth checks if a node is ready
func (h *K8sHealthChecker) checkNodeHealth(ctx context.Context, name string) (bool, error) {
	node, err := h.client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get node: %w", err)
	}

	// Check node conditions
	for _, condition := range node.Status.Conditions {
		switch condition.Type {
		case corev1.NodeReady:
			if condition.Status != corev1.ConditionTrue {
				return false, nil
			}
		case corev1.NodeMemoryPressure, corev1.NodeDiskPressure, corev1.NodePIDPressure:
			if condition.Status == corev1.ConditionTrue {
				return false, nil
			}
		}
	}

	return true, nil
}

// checkPVCHealth checks if a PVC is bound
func (h *K8sHealthChecker) checkPVCHealth(ctx context.Context, namespace, name string) (bool, error) {
	pvc, err := h.client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get pvc: %w", err)
	}

	// Check if PVC is bound
	if pvc.Status.Phase != corev1.ClaimBound {
		return false, nil
	}

	return true, nil
}

// checkJobHealth checks if a job completed successfully
func (h *K8sHealthChecker) checkJobHealth(ctx context.Context, namespace, name string) (bool, error) {
	job, err := h.client.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get job: %w", err)
	}

	// Check if job succeeded or is active
	if job.Status.Succeeded > 0 {
		return true, nil
	}

	// If job is still active and not failing, consider it healthy
	if job.Status.Active > 0 && job.Status.Failed == 0 {
		return true, nil
	}

	// Job failed
	if job.Status.Failed > 0 {
		return false, nil
	}

	return true, nil
}

// NoOpHealthChecker is a health checker that always returns true (for testing or when K8s is unavailable)
type NoOpHealthChecker struct{}

// CheckResourcesHealthy always returns false (incident never auto-closes)
func (h *NoOpHealthChecker) CheckResourcesHealthy(ctx context.Context, incident *Incident) (bool, error) {
	// Don't auto-close without K8s client - require manual closure or debounce timeout
	return false, nil
}
