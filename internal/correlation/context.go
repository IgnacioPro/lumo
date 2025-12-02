package correlation

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned"
)

// K8sContextGatherer implements ContextGatherer using Kubernetes API
type K8sContextGatherer struct {
	k8sClient     kubernetes.Interface
	metricsClient metricsv1beta1.Interface // Can be nil if metrics-server not available
	logger        *logrus.Entry
	config        *ContextGathererConfig
}

// ContextGathererConfig holds configuration for the context gatherer
type ContextGathererConfig struct {
	// LogLinesPerContainer is how many log lines to fetch per container
	LogLinesPerContainer int64
	// LogSinceSeconds limits how far back to look for logs
	LogSinceSeconds int64
	// MaxPodsToFetch limits how many pods to gather logs from
	MaxPodsToFetch int
	// RelatedEventsLimit limits how many K8s events to fetch
	RelatedEventsLimit int
	// MetricsEnabled controls whether to attempt metrics gathering
	MetricsEnabled bool
}

// DefaultContextGathererConfig returns sensible defaults
func DefaultContextGathererConfig() *ContextGathererConfig {
	return &ContextGathererConfig{
		LogLinesPerContainer: 100,
		LogSinceSeconds:      600, // 10 minutes
		MaxPodsToFetch:       10,
		RelatedEventsLimit:   50,
		MetricsEnabled:       true,
	}
}

// NewK8sContextGatherer creates a new Kubernetes-based context gatherer
func NewK8sContextGatherer(
	k8sClient kubernetes.Interface,
	metricsClient metricsv1beta1.Interface,
	logger *logrus.Logger,
	config *ContextGathererConfig,
) *K8sContextGatherer {
	if config == nil {
		config = DefaultContextGathererConfig()
	}

	return &K8sContextGatherer{
		k8sClient:     k8sClient,
		metricsClient: metricsClient,
		logger:        logger.WithField("component", "context-gatherer"),
		config:        config,
	}
}

// GatherContext collects contextual information for an incident
func (g *K8sContextGatherer) GatherContext(ctx context.Context, incident *Incident) (*IncidentContext, error) {
	g.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"category":    incident.Category,
		"resources":   len(incident.AffectedResources),
	}).Info("Gathering incident context")

	incidentCtx := &IncidentContext{
		PodLogs:          make(map[string][]LogEntry),
		RelatedK8sEvents: make([]K8sEventSummary, 0),
		NodeConditions:   make(map[string][]NodeCondition),
		GatheredAt:       time.Now(),
	}

	// Gather pod logs for affected pods
	if err := g.gatherPodLogs(ctx, incident, incidentCtx); err != nil {
		g.logger.WithError(err).Warn("Failed to gather pod logs")
	}

	// Gather related Kubernetes events
	if err := g.gatherRelatedEvents(ctx, incident, incidentCtx); err != nil {
		g.logger.WithError(err).Warn("Failed to gather related events")
	}

	// Gather node conditions for node-related incidents
	if incident.Category == CategoryNode || incident.Category == CategoryMemory {
		if err := g.gatherNodeConditions(ctx, incident, incidentCtx); err != nil {
			g.logger.WithError(err).Warn("Failed to gather node conditions")
		}
	}

	// Gather metrics if available
	if g.config.MetricsEnabled && g.metricsClient != nil {
		if err := g.gatherMetrics(ctx, incident, incidentCtx); err != nil {
			g.logger.WithError(err).Warn("Failed to gather metrics")
		}
	}

	// Extract error patterns from logs
	g.extractErrorPatterns(incidentCtx)

	g.logger.WithFields(logrus.Fields{
		"incident_id":    incident.ID,
		"pods_with_logs": len(incidentCtx.PodLogs),
		"k8s_events":     len(incidentCtx.RelatedK8sEvents),
		"error_patterns": len(incidentCtx.ErrorPatterns),
	}).Info("Context gathering complete")

	return incidentCtx, nil
}

// gatherPodLogs fetches logs from affected pods
func (g *K8sContextGatherer) gatherPodLogs(ctx context.Context, incident *Incident, incidentCtx *IncidentContext) error {
	// Find affected pods
	pods := g.getAffectedPods(incident)
	if len(pods) > g.config.MaxPodsToFetch {
		pods = pods[:g.config.MaxPodsToFetch]
	}

	for _, podRef := range pods {
		podKey := fmt.Sprintf("%s/%s", podRef.Namespace, podRef.Name)

		// Get pod to find containers
		pod, err := g.k8sClient.CoreV1().Pods(podRef.Namespace).Get(ctx, podRef.Name, metav1.GetOptions{})
		if err != nil {
			g.logger.WithError(err).WithField("pod", podKey).Debug("Failed to get pod")
			continue
		}

		// Get logs from each container
		for _, container := range pod.Spec.Containers {
			logs, err := g.fetchContainerLogs(ctx, podRef.Namespace, podRef.Name, container.Name)
			if err != nil {
				g.logger.WithError(err).WithFields(logrus.Fields{
					"pod":       podKey,
					"container": container.Name,
				}).Debug("Failed to fetch container logs")
				continue
			}

			if len(logs) > 0 {
				incidentCtx.PodLogs[podKey] = append(incidentCtx.PodLogs[podKey], logs...)
			}
		}
	}

	return nil
}

// fetchContainerLogs fetches logs from a specific container
func (g *K8sContextGatherer) fetchContainerLogs(ctx context.Context, namespace, podName, containerName string) ([]LogEntry, error) {
	sinceSeconds := g.config.LogSinceSeconds
	tailLines := g.config.LogLinesPerContainer

	opts := &corev1.PodLogOptions{
		Container:    containerName,
		TailLines:    &tailLines,
		SinceSeconds: &sinceSeconds,
		Timestamps:   true,
	}

	req := g.k8sClient.CoreV1().Pods(namespace).GetLogs(podName, opts)
	logBytes, err := req.DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch logs: %w", err)
	}

	return parseLogLines(string(logBytes), containerName), nil
}

// parseLogLines parses raw log output into LogEntry structs
func parseLogLines(rawLogs string, containerName string) []LogEntry {
	lines := strings.Split(rawLogs, "\n")
	entries := make([]LogEntry, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		entry := LogEntry{
			Container: containerName,
			Message:   line,
		}

		// Try to parse timestamp (format: 2006-01-02T15:04:05.000000000Z)
		if len(line) > 30 && line[10] == 'T' {
			if ts, err := time.Parse(time.RFC3339Nano, line[:30]); err == nil {
				entry.Timestamp = ts
				entry.Message = strings.TrimSpace(line[31:])
			}
		}

		// Detect log level
		entry.Level = detectLogLevel(entry.Message)

		entries = append(entries, entry)
	}

	return entries
}

// detectLogLevel tries to detect the log level from a message
func detectLogLevel(message string) string {
	lower := strings.ToLower(message)

	// Check for common log level patterns
	if strings.Contains(lower, "error") || strings.Contains(lower, "err") ||
		strings.Contains(lower, "fatal") || strings.Contains(lower, "panic") {
		return "error"
	}
	if strings.Contains(lower, "warn") {
		return "warn"
	}
	if strings.Contains(lower, "debug") {
		return "debug"
	}
	return "info"
}

// gatherRelatedEvents fetches Kubernetes events for affected resources
func (g *K8sContextGatherer) gatherRelatedEvents(ctx context.Context, incident *Incident, incidentCtx *IncidentContext) error {
	// Collect unique namespaces
	namespaces := make(map[string]bool)
	for _, resource := range incident.AffectedResources {
		if resource.Namespace != "" {
			namespaces[resource.Namespace] = true
		}
	}

	// Get events from each namespace
	eventMap := make(map[string]K8sEventSummary) // Deduplicate by key

	for ns := range namespaces {
		events, err := g.k8sClient.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
			Limit: int64(g.config.RelatedEventsLimit),
		})
		if err != nil {
			g.logger.WithError(err).WithField("namespace", ns).Debug("Failed to list events")
			continue
		}

		// Filter events related to our affected resources
		for _, event := range events.Items {
			// Only include Warning events or recent Normal events
			if event.Type != "Warning" {
				continue
			}

			// Check if event is related to our resources
			isRelated := false
			for _, resource := range incident.AffectedResources {
				if event.InvolvedObject.Name == resource.Name &&
					event.InvolvedObject.Kind == resource.Kind {
					isRelated = true
					break
				}
			}

			// Also include events from the incident time window
			if !isRelated {
				eventTime := event.LastTimestamp.Time
				if eventTime.IsZero() {
					eventTime = event.EventTime.Time
				}
				if eventTime.After(incident.FirstEventAt.Add(-5*time.Minute)) &&
					eventTime.Before(incident.LastEventAt.Add(5*time.Minute)) {
					isRelated = true
				}
			}

			if !isRelated {
				continue
			}

			// Create summary
			key := fmt.Sprintf("%s/%s/%s/%s",
				event.InvolvedObject.Kind,
				event.InvolvedObject.Name,
				event.Reason,
				event.Message)

			lastTime := event.LastTimestamp.Time
			if lastTime.IsZero() {
				lastTime = event.EventTime.Time
			}
			firstTime := event.FirstTimestamp.Time
			if firstTime.IsZero() {
				firstTime = event.EventTime.Time
			}

			if existing, exists := eventMap[key]; exists {
				// Update count and times
				existing.Count += event.Count
				if firstTime.Before(existing.FirstTimestamp) {
					existing.FirstTimestamp = firstTime
				}
				if lastTime.After(existing.LastTimestamp) {
					existing.LastTimestamp = lastTime
				}
				eventMap[key] = existing
			} else {
				eventMap[key] = K8sEventSummary{
					Type:           event.Type,
					Reason:         event.Reason,
					Message:        event.Message,
					Count:          event.Count,
					FirstTimestamp: firstTime,
					LastTimestamp:  lastTime,
					Resource:       fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name),
				}
			}
		}
	}

	// Convert map to slice and sort by last timestamp
	for _, summary := range eventMap {
		incidentCtx.RelatedK8sEvents = append(incidentCtx.RelatedK8sEvents, summary)
	}
	sort.Slice(incidentCtx.RelatedK8sEvents, func(i, j int) bool {
		return incidentCtx.RelatedK8sEvents[i].LastTimestamp.After(incidentCtx.RelatedK8sEvents[j].LastTimestamp)
	})

	// Limit results
	if len(incidentCtx.RelatedK8sEvents) > g.config.RelatedEventsLimit {
		incidentCtx.RelatedK8sEvents = incidentCtx.RelatedK8sEvents[:g.config.RelatedEventsLimit]
	}

	return nil
}

// gatherNodeConditions fetches node conditions for node-related incidents
func (g *K8sContextGatherer) gatherNodeConditions(ctx context.Context, incident *Incident, incidentCtx *IncidentContext) error {
	// Find affected nodes
	nodeNames := make(map[string]bool)

	// From affected resources
	for _, resource := range incident.AffectedResources {
		if resource.Kind == "Node" {
			nodeNames[resource.Name] = true
		}
	}

	// From event metadata
	for _, event := range incident.Events {
		if nodeName, ok := event.Metadata["node_name"].(string); ok && nodeName != "" {
			nodeNames[nodeName] = true
		}
	}

	// Fetch conditions for each node
	for nodeName := range nodeNames {
		node, err := g.k8sClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			g.logger.WithError(err).WithField("node", nodeName).Debug("Failed to get node")
			continue
		}

		conditions := make([]NodeCondition, 0, len(node.Status.Conditions))
		for _, cond := range node.Status.Conditions {
			conditions = append(conditions, NodeCondition{
				Type:    string(cond.Type),
				Status:  string(cond.Status),
				Reason:  cond.Reason,
				Message: cond.Message,
				Updated: cond.LastTransitionTime.Time,
			})
		}

		incidentCtx.NodeConditions[nodeName] = conditions
	}

	return nil
}

// gatherMetrics fetches resource metrics for affected pods/nodes
func (g *K8sContextGatherer) gatherMetrics(ctx context.Context, incident *Incident, incidentCtx *IncidentContext) error {
	if g.metricsClient == nil {
		return nil
	}

	metrics := &ResourceMetrics{
		NodeMetrics: make(map[string]NodeMetric),
		PodMetrics:  make(map[string]PodMetric),
	}

	// Get node metrics
	nodeNames := make(map[string]bool)
	for _, event := range incident.Events {
		if nodeName, ok := event.Metadata["node_name"].(string); ok && nodeName != "" {
			nodeNames[nodeName] = true
		}
	}

	for nodeName := range nodeNames {
		nodeMetrics, err := g.metricsClient.MetricsV1beta1().NodeMetricses().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		cpuQuantity := nodeMetrics.Usage.Cpu()
		memQuantity := nodeMetrics.Usage.Memory()

		metrics.NodeMetrics[nodeName] = NodeMetric{
			CPUUsagePercent:    float64(cpuQuantity.MilliValue()) / 10, // Rough estimate
			MemoryUsageBytes:   memQuantity.Value(),
			MemoryUsagePercent: 0, // Would need node capacity to calculate
			Timestamp:          nodeMetrics.Timestamp.Time,
		}
	}

	// Get pod metrics for affected pods
	for _, resource := range incident.AffectedResources {
		if resource.Kind != "Pod" {
			continue
		}

		podMetrics, err := g.metricsClient.MetricsV1beta1().PodMetricses(resource.Namespace).Get(ctx, resource.Name, metav1.GetOptions{})
		if err != nil {
			continue
		}

		var totalCPU, totalMem int64
		for _, container := range podMetrics.Containers {
			totalCPU += container.Usage.Cpu().MilliValue()
			totalMem += container.Usage.Memory().Value()
		}

		podKey := fmt.Sprintf("%s/%s", resource.Namespace, resource.Name)
		metrics.PodMetrics[podKey] = PodMetric{
			CPUUsageCores:    float64(totalCPU) / 1000,
			MemoryUsageBytes: totalMem,
			Timestamp:        podMetrics.Timestamp.Time,
		}
	}

	if len(metrics.NodeMetrics) > 0 || len(metrics.PodMetrics) > 0 {
		incidentCtx.Metrics = metrics
	}

	return nil
}

// extractErrorPatterns analyzes logs to find common error patterns
func (g *K8sContextGatherer) extractErrorPatterns(incidentCtx *IncidentContext) {
	patternCounts := make(map[string]*ErrorPattern)

	// Common error patterns to look for
	errorPatterns := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"OutOfMemory", regexp.MustCompile(`(?i)(out of memory|oom|memory limit|cannot allocate)`)},
		{"ConnectionRefused", regexp.MustCompile(`(?i)(connection refused|ECONNREFUSED|dial tcp.*refused)`)},
		{"Timeout", regexp.MustCompile(`(?i)(timeout|timed out|deadline exceeded)`)},
		{"PermissionDenied", regexp.MustCompile(`(?i)(permission denied|access denied|forbidden|unauthorized)`)},
		{"FileNotFound", regexp.MustCompile(`(?i)(no such file|file not found|ENOENT)`)},
		{"DependencyError", regexp.MustCompile(`(?i)(failed to connect|could not resolve|service unavailable)`)},
		{"CrashRestart", regexp.MustCompile(`(?i)(fatal|panic|segfault|crash|exited with code [1-9])`)},
		{"ResourceExhausted", regexp.MustCompile(`(?i)(resource exhausted|too many|limit exceeded|quota)`)},
	}

	// Scan all logs
	for podKey, logs := range incidentCtx.PodLogs {
		for _, log := range logs {
			if log.Level != "error" && log.Level != "warn" {
				continue
			}

			for _, ep := range errorPatterns {
				if ep.pattern.MatchString(log.Message) {
					if existing, exists := patternCounts[ep.name]; exists {
						existing.Occurrences++
						if len(existing.Samples) < 3 {
							existing.Samples = append(existing.Samples, log.Message)
						}
						// Track which containers have this error
						found := false
						for _, c := range existing.Containers {
							if c == podKey {
								found = true
								break
							}
						}
						if !found {
							existing.Containers = append(existing.Containers, podKey)
						}
					} else {
						patternCounts[ep.name] = &ErrorPattern{
							Pattern:     ep.name,
							Occurrences: 1,
							Samples:     []string{log.Message},
							Containers:  []string{podKey},
						}
					}
					break // Only match first pattern per log line
				}
			}
		}
	}

	// Convert to slice and sort by occurrences
	for _, pattern := range patternCounts {
		incidentCtx.ErrorPatterns = append(incidentCtx.ErrorPatterns, *pattern)
	}
	sort.Slice(incidentCtx.ErrorPatterns, func(i, j int) bool {
		return incidentCtx.ErrorPatterns[i].Occurrences > incidentCtx.ErrorPatterns[j].Occurrences
	})
}

// getAffectedPods returns pods from affected resources
func (g *K8sContextGatherer) getAffectedPods(incident *Incident) []AffectedResource {
	pods := make([]AffectedResource, 0)
	for _, resource := range incident.AffectedResources {
		if resource.Kind == "Pod" {
			pods = append(pods, resource)
		}
	}
	return pods
}
