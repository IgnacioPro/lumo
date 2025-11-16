package checkers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/diagnostics"
)

const (
	// Check name
	KubernetesCheckName = "kubernetes"

	// Resource limits
	maxEventsPerNamespace = 100
	maxPodsPerNamespace   = 500
)

// KubernetesChecker implements Kubernetes cluster diagnostics using native client
type KubernetesChecker struct {
	config    config.KubernetesConfig
	clientset kubernetes.Interface
	logger    *logrus.Logger
}

// NewKubernetesChecker creates a new Kubernetes diagnostics checker
func NewKubernetesChecker(cfg config.KubernetesConfig, logger *logrus.Logger) *KubernetesChecker {
	return &KubernetesChecker{
		config: cfg,
		logger: logger,
	}
}

// Name returns the checker name
func (k *KubernetesChecker) Name() string {
	return KubernetesCheckName
}

// Category returns the checker category
func (k *KubernetesChecker) Category() diagnostics.CheckCategory {
	return diagnostics.CategoryKubernetes
}

// Description returns the checker description
func (k *KubernetesChecker) Description() string {
	return "Kubernetes cluster health diagnostics"
}

// RequiresRoot returns false as Kubernetes checks use kubeconfig for auth
func (k *KubernetesChecker) RequiresRoot() bool {
	return false
}

// Run executes the Kubernetes diagnostics check
// Note: executor parameter is ignored as we use native Kubernetes client
func (k *KubernetesChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
	startTime := time.Now()

	result := &diagnostics.CheckResult{
		Name:      k.Name(),
		Category:  k.Category(),
		Status:    diagnostics.StatusCompleted,
		Severity:  diagnostics.SeverityInfo,
		Message:   "Kubernetes cluster is healthy",
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
		Metrics:   []diagnostics.Metric{},
	}

	// Initialize Kubernetes client
	if err := k.initClient(); err != nil {
		result.Status = diagnostics.StatusFailed
		result.Severity = diagnostics.SeverityError
		result.Message = fmt.Sprintf("Failed to initialize Kubernetes client: %v", err)
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Get cluster version
	if err := k.checkClusterVersion(ctx, result); err != nil {
		k.logger.Warnf("Failed to get cluster version: %v", err)
	}

	// Check nodes if enabled
	if k.config.CheckNodes {
		if err := k.checkNodes(ctx, result); err != nil {
			k.logger.Warnf("Failed to check nodes: %v", err)
			result.SetData("node_check_error", err.Error())
		}
	}

	// Determine which namespaces to check
	namespaces, err := k.getNamespacesToCheck(ctx)
	if err != nil {
		result.Status = diagnostics.StatusFailed
		result.Severity = diagnostics.SeverityError
		result.Message = fmt.Sprintf("Failed to get namespaces: %v", err)
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Store namespace count
	result.AddMetric(diagnostics.Metric{
		Name:  "namespace_count",
		Value: float64(len(namespaces)),
		Unit:  "count",
	})
	result.SetData("namespaces", namespaces)

	// Check resources in each namespace
	for _, ns := range namespaces {
		k.logger.Debugf("Checking namespace: %s", ns)

		// Check pods
		if k.config.CheckPods {
			if err := k.checkPods(ctx, ns, result); err != nil {
				k.logger.Warnf("Failed to check pods in namespace %s: %v", ns, err)
			}
		}

		// Check deployments
		if k.config.CheckDeployments {
			if err := k.checkDeployments(ctx, ns, result); err != nil {
				k.logger.Warnf("Failed to check deployments in namespace %s: %v", ns, err)
			}
		}

		// Check StatefulSets
		if k.config.CheckStatefulSets {
			if err := k.checkStatefulSets(ctx, ns, result); err != nil {
				k.logger.Warnf("Failed to check StatefulSets in namespace %s: %v", ns, err)
			}
		}

		// Check DaemonSets
		if k.config.CheckDaemonSets {
			if err := k.checkDaemonSets(ctx, ns, result); err != nil {
				k.logger.Warnf("Failed to check DaemonSets in namespace %s: %v", ns, err)
			}
		}

		// Check services
		if k.config.CheckServices {
			if err := k.checkServices(ctx, ns, result); err != nil {
				k.logger.Warnf("Failed to check services in namespace %s: %v", ns, err)
			}
		}

		// Check PVCs
		if k.config.CheckPVCs {
			if err := k.checkPVCs(ctx, ns, result); err != nil {
				k.logger.Warnf("Failed to check PVCs in namespace %s: %v", ns, err)
			}
		}

		// Check events
		if k.config.CheckEvents {
			if err := k.checkEvents(ctx, ns, result); err != nil {
				k.logger.Warnf("Failed to check events in namespace %s: %v", ns, err)
			}
		}
	}

	// Determine overall severity based on findings
	k.updateOverallStatus(result)

	result.Duration = time.Since(startTime)
	return result, nil
}

// initClient initializes the Kubernetes client using kubeconfig
func (k *KubernetesChecker) initClient() error {
	var err error
	var restConfig *rest.Config

	// Determine kubeconfig path
	kubeconfigPath := k.config.KubeconfigPath
	if kubeconfigPath == "" {
		// Use default path
		if home := homedir.HomeDir(); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		} else {
			return fmt.Errorf("cannot determine kubeconfig path: home directory not found")
		}
	}

	// Build config from kubeconfig
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	configOverrides := &clientcmd.ConfigOverrides{}

	// Use specific context if provided
	if k.config.Context != "" {
		configOverrides.CurrentContext = k.config.Context
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	restConfig, err = kubeConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Create clientset
	k.clientset, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return nil
}

// checkClusterVersion gets the Kubernetes cluster version
func (k *KubernetesChecker) checkClusterVersion(ctx context.Context, result *diagnostics.CheckResult) error {
	versionInfo, err := k.clientset.Discovery().ServerVersion()
	if err != nil {
		return err
	}

	result.SetData("cluster_version", versionInfo.GitVersion)
	result.SetData("cluster_platform", versionInfo.Platform)
	k.logger.Debugf("Cluster version: %s", versionInfo.GitVersion)

	return nil
}

// getNamespacesToCheck returns the list of namespaces to check
func (k *KubernetesChecker) getNamespacesToCheck(ctx context.Context) ([]string, error) {
	// If specific namespaces are configured, use those
	if len(k.config.Namespaces) > 0 {
		return k.config.Namespaces, nil
	}

	// Otherwise, get all namespaces
	nsList, err := k.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	namespaces := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		namespaces = append(namespaces, ns.Name)
	}

	return namespaces, nil
}

// checkNodes checks the health of cluster nodes
func (k *KubernetesChecker) checkNodes(ctx context.Context, result *diagnostics.CheckResult) error {
	nodeList, err := k.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	totalNodes := len(nodeList.Items)
	readyNodes := 0
	notReadyNodes := make([]string, 0)
	nodeIssues := make([]map[string]interface{}, 0)

	for _, node := range nodeList.Items {
		// Check node conditions
		nodeReady := false
		nodeConditions := make(map[string]string)

		for _, condition := range node.Status.Conditions {
			nodeConditions[string(condition.Type)] = string(condition.Status)
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				nodeReady = true
			}
		}

		if nodeReady {
			readyNodes++
		} else {
			notReadyNodes = append(notReadyNodes, node.Name)
			nodeIssues = append(nodeIssues, map[string]interface{}{
				"node":       node.Name,
				"conditions": nodeConditions,
			})
		}

		// Check resource capacity and allocatable
		k.logger.Debugf("Node %s - CPU: %s/%s, Memory: %s/%s",
			node.Name,
			node.Status.Allocatable.Cpu().String(),
			node.Status.Capacity.Cpu().String(),
			node.Status.Allocatable.Memory().String(),
			node.Status.Capacity.Memory().String(),
		)
	}

	// Store node metrics
	result.AddMetric(diagnostics.Metric{
		Name:  "total_nodes",
		Value: float64(totalNodes),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  "ready_nodes",
		Value: float64(readyNodes),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  "not_ready_nodes",
		Value: float64(len(notReadyNodes)),
		Unit:  "count",
	})

	if len(notReadyNodes) > 0 {
		result.SetData("not_ready_node_names", notReadyNodes)
		result.SetData("node_issues", nodeIssues)
		result.Severity = diagnostics.SeverityCritical
		result.Message = fmt.Sprintf("%d of %d nodes are not ready", len(notReadyNodes), totalNodes)
		result.SetData("node_suggestion", fmt.Sprintf("Investigate not-ready nodes: %s", strings.Join(notReadyNodes, ", ")))
	}

	return nil
}

// checkPods checks pod health in a namespace
func (k *KubernetesChecker) checkPods(ctx context.Context, namespace string, result *diagnostics.CheckResult) error {
	podList, err := k.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{Limit: maxPodsPerNamespace})
	if err != nil {
		return err
	}

	var (
		totalPods     = len(podList.Items)
		runningPods   = 0
		pendingPods   = 0
		failedPods    = 0
		unknownPods   = 0
		problemPods   = make([]map[string]interface{}, 0)
		restartIssues = make([]map[string]interface{}, 0)
	)

	for _, pod := range podList.Items {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			runningPods++
		case corev1.PodPending:
			pendingPods++
			problemPods = append(problemPods, map[string]interface{}{
				"namespace": namespace,
				"name":      pod.Name,
				"phase":     string(pod.Status.Phase),
				"reason":    pod.Status.Reason,
				"message":   pod.Status.Message,
			})
		case corev1.PodFailed:
			failedPods++
			problemPods = append(problemPods, map[string]interface{}{
				"namespace": namespace,
				"name":      pod.Name,
				"phase":     string(pod.Status.Phase),
				"reason":    pod.Status.Reason,
				"message":   pod.Status.Message,
			})
		case corev1.PodUnknown:
			unknownPods++
			problemPods = append(problemPods, map[string]interface{}{
				"namespace": namespace,
				"name":      pod.Name,
				"phase":     string(pod.Status.Phase),
			})
		}

		// Check for high restart counts
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.RestartCount > 5 {
				restartIssues = append(restartIssues, map[string]interface{}{
					"namespace":     namespace,
					"pod":           pod.Name,
					"container":     containerStatus.Name,
					"restart_count": containerStatus.RestartCount,
				})
			}
		}
	}

	// Store pod metrics
	nsPrefix := fmt.Sprintf("pods_%s", namespace)
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_total", nsPrefix),
		Value: float64(totalPods),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_running", nsPrefix),
		Value: float64(runningPods),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_pending", nsPrefix),
		Value: float64(pendingPods),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_failed", nsPrefix),
		Value: float64(failedPods),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_unknown", nsPrefix),
		Value: float64(unknownPods),
		Unit:  "count",
	})

	// Report issues
	if len(problemPods) > 0 {
		key := fmt.Sprintf("problem_pods_%s", namespace)
		result.SetData(key, problemPods)
		result.Severity = maxSeverity(result.Severity, diagnostics.SeverityWarning)
		result.SetData(fmt.Sprintf("%s_suggestion", key),
			fmt.Sprintf("Namespace %s: Investigate %d problematic pods", namespace, len(problemPods)))
	}

	if len(restartIssues) > 0 {
		key := fmt.Sprintf("restart_issues_%s", namespace)
		result.SetData(key, restartIssues)
		result.Severity = maxSeverity(result.Severity, diagnostics.SeverityWarning)
		result.SetData(fmt.Sprintf("%s_suggestion", key),
			fmt.Sprintf("Namespace %s: %d containers have high restart counts", namespace, len(restartIssues)))
	}

	return nil
}

// checkDeployments checks deployment health
func (k *KubernetesChecker) checkDeployments(ctx context.Context, namespace string, result *diagnostics.CheckResult) error {
	deployList, err := k.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	var (
		totalDeployments   = len(deployList.Items)
		healthyDeployments = 0
		unhealthyDeploys   = make([]map[string]interface{}, 0)
	)

	for _, deploy := range deployList.Items {
		desired := deploy.Status.Replicas
		ready := deploy.Status.ReadyReplicas
		available := deploy.Status.AvailableReplicas

		if ready == desired && available == desired && desired > 0 {
			healthyDeployments++
		} else {
			unhealthyDeploys = append(unhealthyDeploys, map[string]interface{}{
				"namespace": namespace,
				"name":      deploy.Name,
				"desired":   desired,
				"ready":     ready,
				"available": available,
			})
		}
	}

	// Store deployment metrics
	nsPrefix := fmt.Sprintf("deployments_%s", namespace)
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_total", nsPrefix),
		Value: float64(totalDeployments),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_healthy", nsPrefix),
		Value: float64(healthyDeployments),
		Unit:  "count",
	})

	if len(unhealthyDeploys) > 0 {
		key := fmt.Sprintf("unhealthy_deployments_%s", namespace)
		result.SetData(key, unhealthyDeploys)
		result.Severity = maxSeverity(result.Severity, diagnostics.SeverityCritical)
		result.SetData(fmt.Sprintf("%s_suggestion", key),
			fmt.Sprintf("Namespace %s: %d deployments are not fully available", namespace, len(unhealthyDeploys)))
	}

	return nil
}

// checkStatefulSets checks StatefulSet health
func (k *KubernetesChecker) checkStatefulSets(ctx context.Context, namespace string, result *diagnostics.CheckResult) error {
	stsList, err := k.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	var (
		totalSts     = len(stsList.Items)
		healthySts   = 0
		unhealthySts = make([]map[string]interface{}, 0)
	)

	for _, sts := range stsList.Items {
		desired := sts.Status.Replicas
		ready := sts.Status.ReadyReplicas

		if ready == desired && desired > 0 {
			healthySts++
		} else {
			unhealthySts = append(unhealthySts, map[string]interface{}{
				"namespace": namespace,
				"name":      sts.Name,
				"desired":   desired,
				"ready":     ready,
			})
		}
	}

	// Store StatefulSet metrics
	nsPrefix := fmt.Sprintf("statefulsets_%s", namespace)
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_total", nsPrefix),
		Value: float64(totalSts),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_healthy", nsPrefix),
		Value: float64(healthySts),
		Unit:  "count",
	})

	if len(unhealthySts) > 0 {
		key := fmt.Sprintf("unhealthy_statefulsets_%s", namespace)
		result.SetData(key, unhealthySts)
		result.Severity = maxSeverity(result.Severity, diagnostics.SeverityCritical)
		result.SetData(fmt.Sprintf("%s_suggestion", key),
			fmt.Sprintf("Namespace %s: %d StatefulSets are not fully ready", namespace, len(unhealthySts)))
	}

	return nil
}

// checkDaemonSets checks DaemonSet health
func (k *KubernetesChecker) checkDaemonSets(ctx context.Context, namespace string, result *diagnostics.CheckResult) error {
	dsList, err := k.clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	var (
		totalDs     = len(dsList.Items)
		healthyDs   = 0
		unhealthyDs = make([]map[string]interface{}, 0)
	)

	for _, ds := range dsList.Items {
		desired := ds.Status.DesiredNumberScheduled
		ready := ds.Status.NumberReady

		if ready == desired && desired > 0 {
			healthyDs++
		} else {
			unhealthyDs = append(unhealthyDs, map[string]interface{}{
				"namespace": namespace,
				"name":      ds.Name,
				"desired":   desired,
				"ready":     ready,
			})
		}
	}

	// Store DaemonSet metrics
	nsPrefix := fmt.Sprintf("daemonsets_%s", namespace)
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_total", nsPrefix),
		Value: float64(totalDs),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_healthy", nsPrefix),
		Value: float64(healthyDs),
		Unit:  "count",
	})

	if len(unhealthyDs) > 0 {
		key := fmt.Sprintf("unhealthy_daemonsets_%s", namespace)
		result.SetData(key, unhealthyDs)
		result.Severity = maxSeverity(result.Severity, diagnostics.SeverityCritical)
		result.SetData(fmt.Sprintf("%s_suggestion", key),
			fmt.Sprintf("Namespace %s: %d DaemonSets are not fully ready", namespace, len(unhealthyDs)))
	}

	return nil
}

// checkServices checks service endpoint health
func (k *KubernetesChecker) checkServices(ctx context.Context, namespace string, result *diagnostics.CheckResult) error {
	svcList, err := k.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	var (
		totalServices      = len(svcList.Items)
		servicesWithoutEPs = make([]string, 0)
	)

	for _, svc := range svcList.Items {
		// Skip headless services and ExternalName services
		if svc.Spec.ClusterIP == "None" || svc.Spec.Type == corev1.ServiceTypeExternalName {
			continue
		}

		// Check if service has endpoints
		endpoints, err := k.clientset.CoreV1().Endpoints(namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			k.logger.Debugf("Could not get endpoints for service %s/%s: %v", namespace, svc.Name, err)
			continue
		}

		// Check if endpoints exist
		hasEndpoints := false
		for _, subset := range endpoints.Subsets {
			if len(subset.Addresses) > 0 {
				hasEndpoints = true
				break
			}
		}

		if !hasEndpoints {
			servicesWithoutEPs = append(servicesWithoutEPs, svc.Name)
		}
	}

	// Store service metrics
	nsPrefix := fmt.Sprintf("services_%s", namespace)
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_total", nsPrefix),
		Value: float64(totalServices),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_without_endpoints", nsPrefix),
		Value: float64(len(servicesWithoutEPs)),
		Unit:  "count",
	})

	if len(servicesWithoutEPs) > 0 {
		key := fmt.Sprintf("services_without_endpoints_%s", namespace)
		result.SetData(key, servicesWithoutEPs)
		result.Severity = maxSeverity(result.Severity, diagnostics.SeverityWarning)
		result.SetData(fmt.Sprintf("%s_suggestion", key),
			fmt.Sprintf("Namespace %s: %d services have no endpoints", namespace, len(servicesWithoutEPs)))
	}

	return nil
}

// checkPVCs checks PersistentVolumeClaim status
func (k *KubernetesChecker) checkPVCs(ctx context.Context, namespace string, result *diagnostics.CheckResult) error {
	pvcList, err := k.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	var (
		totalPVCs   = len(pvcList.Items)
		boundPVCs   = 0
		pendingPVCs = make([]map[string]interface{}, 0)
		lostPVCs    = make([]string, 0)
	)

	for _, pvc := range pvcList.Items {
		switch pvc.Status.Phase {
		case corev1.ClaimBound:
			boundPVCs++
		case corev1.ClaimPending:
			storageClass := ""
			if pvc.Spec.StorageClassName != nil {
				storageClass = *pvc.Spec.StorageClassName
			}
			pendingPVCs = append(pendingPVCs, map[string]interface{}{
				"namespace":     namespace,
				"name":          pvc.Name,
				"storage_class": storageClass,
				"requested":     pvc.Spec.Resources.Requests.Storage().String(),
			})
		case corev1.ClaimLost:
			lostPVCs = append(lostPVCs, pvc.Name)
		}
	}

	// Store PVC metrics
	nsPrefix := fmt.Sprintf("pvcs_%s", namespace)
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_total", nsPrefix),
		Value: float64(totalPVCs),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_bound", nsPrefix),
		Value: float64(boundPVCs),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_pending", nsPrefix),
		Value: float64(len(pendingPVCs)),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_lost", nsPrefix),
		Value: float64(len(lostPVCs)),
		Unit:  "count",
	})

	if len(pendingPVCs) > 0 {
		key := fmt.Sprintf("pending_pvcs_%s", namespace)
		result.SetData(key, pendingPVCs)
		result.Severity = maxSeverity(result.Severity, diagnostics.SeverityWarning)
		result.SetData(fmt.Sprintf("%s_suggestion", key),
			fmt.Sprintf("Namespace %s: %d PVCs are pending", namespace, len(pendingPVCs)))
	}

	if len(lostPVCs) > 0 {
		key := fmt.Sprintf("lost_pvcs_%s", namespace)
		result.SetData(key, lostPVCs)
		result.Severity = maxSeverity(result.Severity, diagnostics.SeverityCritical)
		result.SetData(fmt.Sprintf("%s_suggestion", key),
			fmt.Sprintf("Namespace %s: %d PVCs are in lost state", namespace, len(lostPVCs)))
	}

	return nil
}

// checkEvents checks recent cluster events
func (k *KubernetesChecker) checkEvents(ctx context.Context, namespace string, result *diagnostics.CheckResult) error {
	// Calculate time threshold
	lookbackDuration := time.Duration(k.config.EventLookbackMins) * time.Minute
	threshold := time.Now().Add(-lookbackDuration)

	eventList, err := k.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		Limit: maxEventsPerNamespace,
	})
	if err != nil {
		return err
	}

	var (
		recentWarnings = make([]map[string]interface{}, 0)
		recentErrors   = make([]map[string]interface{}, 0)
	)

	for _, event := range eventList.Items {
		// Only consider recent events
		if event.LastTimestamp.Time.Before(threshold) {
			continue
		}

		eventInfo := map[string]interface{}{
			"namespace": namespace,
			"type":      event.Type,
			"reason":    event.Reason,
			"message":   event.Message,
			"object":    fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name),
			"count":     event.Count,
			"timestamp": event.LastTimestamp.Time.Format(time.RFC3339),
		}

		if event.Type == corev1.EventTypeWarning {
			recentWarnings = append(recentWarnings, eventInfo)
		} else if strings.Contains(strings.ToLower(event.Reason), "error") ||
			strings.Contains(strings.ToLower(event.Reason), "failed") {
			recentErrors = append(recentErrors, eventInfo)
		}
	}

	// Store event metrics
	nsPrefix := fmt.Sprintf("events_%s", namespace)
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_warnings", nsPrefix),
		Value: float64(len(recentWarnings)),
		Unit:  "count",
	})
	result.AddMetric(diagnostics.Metric{
		Name:  fmt.Sprintf("%s_errors", nsPrefix),
		Value: float64(len(recentErrors)),
		Unit:  "count",
	})

	if len(recentWarnings) > 0 {
		key := fmt.Sprintf("recent_warning_events_%s", namespace)
		result.SetData(key, recentWarnings)
		result.Severity = maxSeverity(result.Severity, diagnostics.SeverityWarning)
	}

	if len(recentErrors) > 0 {
		key := fmt.Sprintf("recent_error_events_%s", namespace)
		result.SetData(key, recentErrors)
		result.Severity = maxSeverity(result.Severity, diagnostics.SeverityCritical)
		result.SetData(fmt.Sprintf("%s_suggestion", key),
			fmt.Sprintf("Namespace %s: %d error events in the last %d minutes", namespace, len(recentErrors), k.config.EventLookbackMins))
	}

	return nil
}

// updateOverallStatus updates the overall status and message based on findings
func (k *KubernetesChecker) updateOverallStatus(result *diagnostics.CheckResult) {
	switch result.Severity {
	case diagnostics.SeverityError:
		result.Message = "Kubernetes cluster health check failed"
	case diagnostics.SeverityCritical:
		result.Message = "Kubernetes cluster has critical issues"
	case diagnostics.SeverityWarning:
		result.Message = "Kubernetes cluster has some issues"
	case diagnostics.SeverityInfo:
		result.Message = "Kubernetes cluster is mostly healthy"
	default:
		result.Message = "Kubernetes cluster is healthy"
	}
}

// maxSeverity returns the higher of two severities
func maxSeverity(a, b diagnostics.Severity) diagnostics.Severity {
	severityOrder := map[diagnostics.Severity]int{
		diagnostics.SeverityOK:       0,
		diagnostics.SeverityInfo:     1,
		diagnostics.SeverityWarning:  2,
		diagnostics.SeverityCritical: 3,
		diagnostics.SeverityError:    4,
	}

	if severityOrder[a] > severityOrder[b] {
		return a
	}
	return b
}
