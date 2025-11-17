package checkers

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestKubernetesChecker_Name(t *testing.T) {
	cfg := config.KubernetesConfig{Enabled: true}
	checker := NewKubernetesChecker(cfg, logrus.New())

	if checker.Name() != KubernetesCheckName {
		t.Errorf("expected name %s, got %s", KubernetesCheckName, checker.Name())
	}
}

func TestKubernetesChecker_Category(t *testing.T) {
	cfg := config.KubernetesConfig{Enabled: true}
	checker := NewKubernetesChecker(cfg, logrus.New())

	if checker.Category() != diagnostics.CategoryKubernetes {
		t.Errorf("expected category %s, got %s", diagnostics.CategoryKubernetes, checker.Category())
	}
}

func TestKubernetesChecker_Description(t *testing.T) {
	cfg := config.KubernetesConfig{Enabled: true}
	checker := NewKubernetesChecker(cfg, logrus.New())

	desc := checker.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestKubernetesChecker_RequiresRoot(t *testing.T) {
	cfg := config.KubernetesConfig{Enabled: true}
	checker := NewKubernetesChecker(cfg, logrus.New())

	if checker.RequiresRoot() {
		t.Error("Kubernetes checker should not require root")
	}
}

func TestMaxSeverity(t *testing.T) {
	tests := []struct {
		name     string
		a        diagnostics.Severity
		b        diagnostics.Severity
		expected diagnostics.Severity
	}{
		{
			name:     "critical vs high",
			a:        diagnostics.SeverityCritical,
			b:        diagnostics.SeverityCritical,
			expected: diagnostics.SeverityCritical,
		},
		{
			name:     "high vs critical",
			a:        diagnostics.SeverityWarning,
			b:        diagnostics.SeverityCritical,
			expected: diagnostics.SeverityCritical,
		},
		{
			name:     "medium vs low",
			a:        diagnostics.SeverityWarning,
			b:        diagnostics.SeverityInfo,
			expected: diagnostics.SeverityWarning,
		},
		{
			name:     "info vs info",
			a:        diagnostics.SeverityInfo,
			b:        diagnostics.SeverityInfo,
			expected: diagnostics.SeverityInfo,
		},
		{
			name:     "high vs medium",
			a:        diagnostics.SeverityCritical,
			b:        diagnostics.SeverityWarning,
			expected: diagnostics.SeverityCritical,
		},
		{
			name:     "error vs critical",
			a:        diagnostics.SeverityError,
			b:        diagnostics.SeverityCritical,
			expected: diagnostics.SeverityError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maxSeverity(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("maxSeverity(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestKubernetesChecker_CheckNodes_Healthy(t *testing.T) {
	// Create fake nodes
	nodes := []runtime.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-2"},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	cfg := config.KubernetesConfig{
		Enabled:    true,
		CheckNodes: true,
	}

	checker := NewKubernetesChecker(cfg, logrus.New())
	checker.clientset = fake.NewSimpleClientset(nodes...)

	result := &diagnostics.CheckResult{
		Metrics: []diagnostics.Metric{},
		Data:    make(map[string]interface{}),
	}

	err := checker.checkNodes(context.Background(), result)
	if err != nil {
		t.Fatalf("checkNodes failed: %v", err)
	}

	// Find metrics
	totalNodes := findMetricValue(result, "total_nodes")
	readyNodes := findMetricValue(result, "ready_nodes")
	notReadyNodes := findMetricValue(result, "not_ready_nodes")

	if totalNodes != 2 {
		t.Errorf("expected 2 total nodes, got %v", totalNodes)
	}

	if readyNodes != 2 {
		t.Errorf("expected 2 ready nodes, got %v", readyNodes)
	}

	if notReadyNodes != 0 {
		t.Errorf("expected 0 not ready nodes, got %v", notReadyNodes)
	}
}

func TestKubernetesChecker_CheckNodes_NotReady(t *testing.T) {
	// Create nodes with one not ready
	nodes := []runtime.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-2"},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionFalse,
					},
				},
			},
		},
	}

	cfg := config.KubernetesConfig{
		Enabled:    true,
		CheckNodes: true,
	}

	checker := NewKubernetesChecker(cfg, logrus.New())
	checker.clientset = fake.NewSimpleClientset(nodes...)

	result := &diagnostics.CheckResult{
		Metrics:  []diagnostics.Metric{},
		Data:     make(map[string]interface{}),
		Severity: diagnostics.SeverityInfo,
	}

	err := checker.checkNodes(context.Background(), result)
	if err != nil {
		t.Fatalf("checkNodes failed: %v", err)
	}

	notReadyNodes := findMetricValue(result, "not_ready_nodes")
	if notReadyNodes != 1 {
		t.Errorf("expected 1 not ready node, got %v", notReadyNodes)
	}

	if result.Severity != diagnostics.SeverityCritical {
		t.Errorf("expected critical severity for not ready nodes, got %v", result.Severity)
	}

	// Check that data was set
	if result.Data["node_suggestion"] == nil {
		t.Error("expected node_suggestion in data")
	}
}

func TestKubernetesChecker_CheckPods(t *testing.T) {
	// Create pods with various states
	pods := []runtime.Object{
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-running",
				Namespace: "default",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "container-1", RestartCount: 0},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-pending",
				Namespace: "default",
			},
			Status: corev1.PodStatus{
				Phase:   corev1.PodPending,
				Reason:  "ImagePullBackOff",
				Message: "Failed to pull image",
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-failed",
				Namespace: "default",
			},
			Status: corev1.PodStatus{
				Phase:   corev1.PodFailed,
				Reason:  "CrashLoopBackOff",
				Message: "Container crashed",
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-high-restarts",
				Namespace: "default",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "container-1", RestartCount: 10},
				},
			},
		},
	}

	cfg := config.KubernetesConfig{
		Enabled:   true,
		CheckPods: true,
	}

	checker := NewKubernetesChecker(cfg, logrus.New())
	checker.clientset = fake.NewSimpleClientset(pods...)

	result := &diagnostics.CheckResult{
		Metrics:  []diagnostics.Metric{},
		Data:     make(map[string]interface{}),
		Severity: diagnostics.SeverityInfo,
	}

	err := checker.checkPods(context.Background(), "default", result)
	if err != nil {
		t.Fatalf("checkPods failed: %v", err)
	}

	// Check metrics
	totalPods := findMetricValue(result, "pods_default_total")
	runningPods := findMetricValue(result, "pods_default_running")
	pendingPods := findMetricValue(result, "pods_default_pending")
	failedPods := findMetricValue(result, "pods_default_failed")

	if totalPods != 4 {
		t.Errorf("expected 4 total pods, got %v", totalPods)
	}

	if runningPods != 2 {
		t.Errorf("expected 2 running pods, got %v", runningPods)
	}

	if pendingPods != 1 {
		t.Errorf("expected 1 pending pod, got %v", pendingPods)
	}

	if failedPods != 1 {
		t.Errorf("expected 1 failed pod, got %v", failedPods)
	}

	// Check that problem pods were recorded
	if result.Data["problem_pods_default"] == nil {
		t.Error("expected problem_pods_default in data")
	}

	// Check that restart issues were recorded
	if result.Data["restart_issues_default"] == nil {
		t.Error("expected restart_issues_default in data")
	}

	if result.Severity != diagnostics.SeverityWarning {
		t.Errorf("expected warning severity for problem pods, got %v", result.Severity)
	}
}

func TestKubernetesChecker_CheckDeployments(t *testing.T) {
	// Create deployments
	deployments := []runtime.Object{
		&corev1.Service{}, // Need at least one object for fake client
	}

	cfg := config.KubernetesConfig{
		Enabled:          true,
		CheckDeployments: true,
	}

	checker := NewKubernetesChecker(cfg, logrus.New())
	checker.clientset = fake.NewSimpleClientset(deployments...)

	result := &diagnostics.CheckResult{
		Metrics:  []diagnostics.Metric{},
		Data:     make(map[string]interface{}),
		Severity: diagnostics.SeverityInfo,
	}

	// This should not error even with no deployments
	err := checker.checkDeployments(context.Background(), "default", result)
	if err != nil {
		t.Fatalf("checkDeployments failed: %v", err)
	}

	// Check that metrics were set (even if 0)
	totalDeployments := findMetricValue(result, "deployments_default_total")
	if totalDeployments != 0 {
		t.Errorf("expected 0 deployments, got %v", totalDeployments)
	}
}

func TestKubernetesChecker_CheckPVCs(t *testing.T) {
	// Create PVCs with various states
	pvcs := []runtime.Object{
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pvc-bound",
				Namespace: "default",
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Phase: corev1.ClaimBound,
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pvc-pending",
				Namespace: "default",
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Phase: corev1.ClaimPending,
			},
		},
	}

	cfg := config.KubernetesConfig{
		Enabled:   true,
		CheckPVCs: true,
	}

	checker := NewKubernetesChecker(cfg, logrus.New())
	checker.clientset = fake.NewSimpleClientset(pvcs...)

	result := &diagnostics.CheckResult{
		Metrics:  []diagnostics.Metric{},
		Data:     make(map[string]interface{}),
		Severity: diagnostics.SeverityInfo,
	}

	err := checker.checkPVCs(context.Background(), "default", result)
	if err != nil {
		t.Fatalf("checkPVCs failed: %v", err)
	}

	totalPVCs := findMetricValue(result, "pvcs_default_total")
	boundPVCs := findMetricValue(result, "pvcs_default_bound")
	pendingPVCs := findMetricValue(result, "pvcs_default_pending")

	if totalPVCs != 2 {
		t.Errorf("expected 2 total PVCs, got %v", totalPVCs)
	}

	if boundPVCs != 1 {
		t.Errorf("expected 1 bound PVC, got %v", boundPVCs)
	}

	if pendingPVCs != 1 {
		t.Errorf("expected 1 pending PVC, got %v", pendingPVCs)
	}

	if result.Severity != diagnostics.SeverityWarning {
		t.Errorf("expected medium severity for pending PVCs, got %v", result.Severity)
	}
}

func TestKubernetesChecker_CheckEvents(t *testing.T) {
	now := time.Now()
	recent := metav1.NewTime(now.Add(-5 * time.Minute))
	old := metav1.NewTime(now.Add(-60 * time.Minute))

	events := []runtime.Object{
		&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "event-warning-recent",
				Namespace: "default",
			},
			Type:          corev1.EventTypeWarning,
			Reason:        "FailedScheduling",
			Message:       "0/3 nodes are available",
			Count:         5,
			LastTimestamp: recent,
			InvolvedObject: corev1.ObjectReference{
				Kind: "Pod",
				Name: "test-pod",
			},
		},
		&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "event-error-recent",
				Namespace: "default",
			},
			Type:          corev1.EventTypeNormal,
			Reason:        "Failed",
			Message:       "Error: container failed",
			Count:         2,
			LastTimestamp: recent,
			InvolvedObject: corev1.ObjectReference{
				Kind: "Pod",
				Name: "test-pod-2",
			},
		},
		&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "event-warning-old",
				Namespace: "default",
			},
			Type:          corev1.EventTypeWarning,
			Reason:        "BackOff",
			Message:       "Back-off restarting failed container",
			Count:         1,
			LastTimestamp: old,
			InvolvedObject: corev1.ObjectReference{
				Kind: "Pod",
				Name: "test-pod-3",
			},
		},
	}

	cfg := config.KubernetesConfig{
		Enabled:           true,
		CheckEvents:       true,
		EventLookbackMins: 30,
	}

	checker := NewKubernetesChecker(cfg, logrus.New())
	checker.clientset = fake.NewSimpleClientset(events...)

	result := &diagnostics.CheckResult{
		Metrics:  []diagnostics.Metric{},
		Data:     make(map[string]interface{}),
		Severity: diagnostics.SeverityInfo,
	}

	err := checker.checkEvents(context.Background(), "default", result)
	if err != nil {
		t.Fatalf("checkEvents failed: %v", err)
	}

	// Should only count recent events (within 30 mins)
	warnings := findMetricValue(result, "events_default_warnings")
	errors := findMetricValue(result, "events_default_errors")

	if warnings != 1 {
		t.Errorf("expected 1 recent warning event, got %v", warnings)
	}

	if errors != 1 {
		t.Errorf("expected 1 recent error event, got %v", errors)
	}

	// Check that severity was elevated
	if result.Severity != diagnostics.SeverityCritical {
		t.Errorf("expected high severity for error events, got %v", result.Severity)
	}
}

func TestKubernetesChecker_UpdateOverallStatus(t *testing.T) {
	tests := []struct {
		name            string
		severity        diagnostics.Severity
		expectedMessage string
	}{
		{
			name:            "error severity",
			severity:        diagnostics.SeverityError,
			expectedMessage: "failed",
		},
		{
			name:            "critical severity",
			severity:        diagnostics.SeverityCritical,
			expectedMessage: "critical issues",
		},
		{
			name:            "warning severity",
			severity:        diagnostics.SeverityWarning,
			expectedMessage: "some issues",
		},
		{
			name:            "info severity",
			severity:        diagnostics.SeverityInfo,
			expectedMessage: "healthy",
		},
		{
			name:            "ok severity",
			severity:        diagnostics.SeverityOK,
			expectedMessage: "healthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.KubernetesConfig{Enabled: true}
			checker := NewKubernetesChecker(cfg, logrus.New())

			result := &diagnostics.CheckResult{
				Severity: tt.severity,
			}

			checker.updateOverallStatus(result)

			if !containsString(result.Message, tt.expectedMessage) {
				t.Errorf("expected message to contain %q, got %q", tt.expectedMessage, result.Message)
			}
		})
	}
}

func TestKubernetesChecker_GetNamespacesToCheck(t *testing.T) {
	tests := []struct {
		name          string
		configNS      []string
		clusterNS     []runtime.Object
		expectedCount int
		expectError   bool
	}{
		{
			name:          "configured namespaces",
			configNS:      []string{"default", "kube-system"},
			clusterNS:     nil,
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:     "all namespaces from cluster",
			configNS: []string{},
			clusterNS: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-public"}},
			},
			expectedCount: 3,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.KubernetesConfig{
				Enabled:    true,
				Namespaces: tt.configNS,
			}

			checker := NewKubernetesChecker(cfg, logrus.New())

			if tt.clusterNS != nil {
				checker.clientset = fake.NewSimpleClientset(tt.clusterNS...)
			}

			namespaces, err := checker.getNamespacesToCheck(context.Background())

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(namespaces) != tt.expectedCount {
				t.Errorf("expected %d namespaces, got %d", tt.expectedCount, len(namespaces))
			}
		})
	}
}

// Helper functions

func findMetricValue(result *diagnostics.CheckResult, name string) float64 {
	for _, metric := range result.Metrics {
		if metric.Name == name {
			return metric.Value
		}
	}
	return -1 // Metric not found
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
