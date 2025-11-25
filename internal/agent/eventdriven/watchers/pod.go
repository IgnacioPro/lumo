package watchers

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/ignacio/lumo/internal/agent/eventdriven"
)

// PodWatcher watches Pod resources for failures
type PodWatcher struct {
	informer cache.SharedIndexInformer
	logger   *logrus.Entry
}

// NewPodWatcher creates a new Pod watcher
func NewPodWatcher(logger *logrus.Logger) *PodWatcher {
	return &PodWatcher{
		logger: logger.WithField("watcher", "pod"),
	}
}

// Name returns the watcher name
func (w *PodWatcher) Name() string {
	return "pod"
}

// Setup configures the informer with event handlers
func (w *PodWatcher) Setup(factory informers.SharedInformerFactory, handler eventdriven.EventHandler) error {
	w.informer = factory.Core().V1().Pods().Informer()

	_, err := w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			w.handlePodEvent(pod, nil, handler)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)
			w.handlePodEvent(newPod, oldPod, handler)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			w.logger.WithFields(logrus.Fields{
				"pod":       pod.Name,
				"namespace": pod.Namespace,
			}).Debug("Pod deleted")
		},
	})

	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	return nil
}

// GetInformer returns the underlying informer
func (w *PodWatcher) GetInformer() cache.SharedIndexInformer {
	return w.informer
}

// handlePodEvent processes pod events and extracts failures
func (w *PodWatcher) handlePodEvent(pod *corev1.Pod, oldPod *corev1.Pod, handler eventdriven.EventHandler) {
	// Extract base resource info
	resourceInfo := extractResourceInfo(pod)

	// Check for various pod failure conditions
	events := w.detectPodIssues(pod, oldPod, resourceInfo)

	// Send events to handler
	for _, event := range events {
		handler.HandleEvent(event)
	}
}

// detectPodIssues checks for various pod failure conditions
func (w *PodWatcher) detectPodIssues(pod *corev1.Pod, oldPod *corev1.Pod, info *eventdriven.ResourceEventInfo) []*eventdriven.KubernetesEvent {
	events := make([]*eventdriven.KubernetesEvent, 0)

	// Check container statuses
	for _, containerStatus := range pod.Status.ContainerStatuses {
		// Check for ImagePullBackOff / ErrImagePull
		if containerStatus.State.Waiting != nil {
			waiting := containerStatus.State.Waiting
			event := w.checkWaitingState(pod, containerStatus.Name, waiting, info)
			if event != nil {
				events = append(events, event)
			}
		}

		// Check for CrashLoopBackOff
		if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
			event := w.createCrashLoopEvent(pod, containerStatus.Name, info)
			if event != nil {
				events = append(events, event)
			}
		}

		// Check for OOMKilled in current state (restartPolicy: Never)
		if containerStatus.State.Terminated != nil {
			terminated := containerStatus.State.Terminated
			if terminated.Reason == "OOMKilled" {
				event := w.createOOMKilledEvent(pod, containerStatus.Name, terminated, info)
				if event != nil {
					events = append(events, event)
				}
			}
		}

		// Check for OOMKilled in last termination state (after restart)
		if containerStatus.LastTerminationState.Terminated != nil {
			terminated := containerStatus.LastTerminationState.Terminated
			if terminated.Reason == "OOMKilled" {
				// Only trigger if this is a new OOMKill (restart count changed)
				if oldPod != nil {
					oldStatus := findContainerStatus(oldPod, containerStatus.Name)
					if oldStatus == nil || oldStatus.RestartCount != containerStatus.RestartCount {
						event := w.createOOMKilledEvent(pod, containerStatus.Name, terminated, info)
						if event != nil {
							events = append(events, event)
						}
					}
				} else {
					// New pod - report OOMKilled
					event := w.createOOMKilledEvent(pod, containerStatus.Name, terminated, info)
					if event != nil {
						events = append(events, event)
					}
				}
			}
		}

		// Check for high restart count
		if containerStatus.RestartCount > 5 {
			// Only trigger if restart count changed
			if oldPod != nil {
				oldStatus := findContainerStatus(oldPod, containerStatus.Name)
				if oldStatus == nil || oldStatus.RestartCount != containerStatus.RestartCount {
					event := w.createHighRestartEvent(pod, containerStatus.Name, containerStatus.RestartCount, info)
					if event != nil {
						events = append(events, event)
					}
				}
			}
		}
	}

	// Check init container statuses
	for _, initStatus := range pod.Status.InitContainerStatuses {
		if initStatus.State.Waiting != nil {
			waiting := initStatus.State.Waiting
			event := w.checkWaitingState(pod, initStatus.Name, waiting, info)
			if event != nil {
				events = append(events, event)
			}
		}
	}

	// Check pod phase
	if pod.Status.Phase == corev1.PodFailed {
		event := w.createPodFailedEvent(pod, info)
		if event != nil {
			events = append(events, event)
		}
	}

	// Check for eviction
	if pod.Status.Reason == "Evicted" {
		event := w.createEvictedEvent(pod, info)
		if event != nil {
			events = append(events, event)
		}
	}

	// Check for pending timeout (pending > 5 minutes)
	if pod.Status.Phase == corev1.PodPending {
		if time.Since(pod.CreationTimestamp.Time) > 5*time.Minute {
			// Only trigger if it's a new pending timeout
			if oldPod == nil || oldPod.Status.Phase != corev1.PodPending {
				event := w.createPendingTimeoutEvent(pod, info)
				if event != nil {
					events = append(events, event)
				}
			}
		}
	}

	return events
}

// checkWaitingState checks container waiting states for issues
func (w *PodWatcher) checkWaitingState(pod *corev1.Pod, containerName string, waiting *corev1.ContainerStateWaiting, info *eventdriven.ResourceEventInfo) *eventdriven.KubernetesEvent {
	reason := waiting.Reason
	message := waiting.Message

	var eventType eventdriven.EventType

	switch {
	case reason == "ImagePullBackOff" || reason == "ErrImagePull":
		eventType = eventdriven.EventTypeImagePullBackOff
	case reason == "InvalidImageName":
		eventType = eventdriven.EventTypeImagePullBackOff
	case reason == "CrashLoopBackOff":
		eventType = eventdriven.EventTypeCrashLoopBackOff
	case reason == "CreateContainerError":
		eventType = eventdriven.EventTypePodFailure
	case strings.Contains(reason, "ContainerCreating"):
		// Only report if stuck for > 2 minutes
		if time.Since(pod.CreationTimestamp.Time) > 2*time.Minute {
			eventType = eventdriven.EventTypeContainerCreating
		} else {
			return nil
		}
	default:
		return nil
	}

	ownerKind, ownerName, ownerUID := info.GetOwnerInfo()

	return &eventdriven.KubernetesEvent{
		ID:                fmt.Sprintf("%s-%s-%s", string(pod.UID), containerName, reason),
		Type:              eventType,
		Severity:          eventdriven.ClassifyEventSeverity(eventType),
		Timestamp:         time.Now(),
		ResourceKind:      "Pod",
		ResourceName:      pod.Name,
		ResourceNamespace: pod.Namespace,
		ResourceUID:       string(pod.UID),
		OwnerKind:         ownerKind,
		OwnerName:         ownerName,
		OwnerUID:          ownerUID,
		Reason:            reason,
		Message:           fmt.Sprintf("Container %s: %s - %s", containerName, reason, message),
		Labels:            pod.Labels,
		Annotations:       pod.Annotations,
	}
}

// createCrashLoopEvent creates an event for CrashLoopBackOff
func (w *PodWatcher) createCrashLoopEvent(pod *corev1.Pod, containerName string, info *eventdriven.ResourceEventInfo) *eventdriven.KubernetesEvent {
	ownerKind, ownerName, ownerUID := info.GetOwnerInfo()

	return &eventdriven.KubernetesEvent{
		ID:                fmt.Sprintf("%s-%s-crashloop", string(pod.UID), containerName),
		Type:              eventdriven.EventTypeCrashLoopBackOff,
		Severity:          eventdriven.SeverityHigh,
		Timestamp:         time.Now(),
		ResourceKind:      "Pod",
		ResourceName:      pod.Name,
		ResourceNamespace: pod.Namespace,
		ResourceUID:       string(pod.UID),
		OwnerKind:         ownerKind,
		OwnerName:         ownerName,
		OwnerUID:          ownerUID,
		Reason:            "CrashLoopBackOff",
		Message:           fmt.Sprintf("Container %s is in CrashLoopBackOff", containerName),
		Labels:            pod.Labels,
		Annotations:       pod.Annotations,
	}
}

// createOOMKilledEvent creates an event for OOMKilled containers
func (w *PodWatcher) createOOMKilledEvent(pod *corev1.Pod, containerName string, terminated *corev1.ContainerStateTerminated, info *eventdriven.ResourceEventInfo) *eventdriven.KubernetesEvent {
	ownerKind, ownerName, ownerUID := info.GetOwnerInfo()

	return &eventdriven.KubernetesEvent{
		ID:                fmt.Sprintf("%s-%s-oom", string(pod.UID), containerName),
		Type:              eventdriven.EventTypeOOMKilled,
		Severity:          eventdriven.SeverityCritical,
		Timestamp:         time.Now(),
		ResourceKind:      "Pod",
		ResourceName:      pod.Name,
		ResourceNamespace: pod.Namespace,
		ResourceUID:       string(pod.UID),
		OwnerKind:         ownerKind,
		OwnerName:         ownerName,
		OwnerUID:          ownerUID,
		Reason:            "OOMKilled",
		Message:           fmt.Sprintf("Container %s was OOMKilled (exit code: %d)", containerName, terminated.ExitCode),
		Labels:            pod.Labels,
		Annotations:       pod.Annotations,
	}
}

// createHighRestartEvent creates an event for high restart counts
func (w *PodWatcher) createHighRestartEvent(pod *corev1.Pod, containerName string, restartCount int32, info *eventdriven.ResourceEventInfo) *eventdriven.KubernetesEvent {
	ownerKind, ownerName, ownerUID := info.GetOwnerInfo()

	return &eventdriven.KubernetesEvent{
		ID:                fmt.Sprintf("%s-%s-restarts", string(pod.UID), containerName),
		Type:              eventdriven.EventTypePodFailure,
		Severity:          eventdriven.SeverityHigh,
		Timestamp:         time.Now(),
		ResourceKind:      "Pod",
		ResourceName:      pod.Name,
		ResourceNamespace: pod.Namespace,
		ResourceUID:       string(pod.UID),
		OwnerKind:         ownerKind,
		OwnerName:         ownerName,
		OwnerUID:          ownerUID,
		Reason:            "HighRestartCount",
		Message:           fmt.Sprintf("Container %s has restarted %d times", containerName, restartCount),
		Labels:            pod.Labels,
		Annotations:       pod.Annotations,
	}
}

// createPodFailedEvent creates an event for failed pods
func (w *PodWatcher) createPodFailedEvent(pod *corev1.Pod, info *eventdriven.ResourceEventInfo) *eventdriven.KubernetesEvent {
	ownerKind, ownerName, ownerUID := info.GetOwnerInfo()

	return &eventdriven.KubernetesEvent{
		ID:                fmt.Sprintf("%s-failed", string(pod.UID)),
		Type:              eventdriven.EventTypePodFailure,
		Severity:          eventdriven.SeverityHigh,
		Timestamp:         time.Now(),
		ResourceKind:      "Pod",
		ResourceName:      pod.Name,
		ResourceNamespace: pod.Namespace,
		ResourceUID:       string(pod.UID),
		OwnerKind:         ownerKind,
		OwnerName:         ownerName,
		OwnerUID:          ownerUID,
		Reason:            "PodFailed",
		Message:           fmt.Sprintf("Pod failed: %s", pod.Status.Message),
		Labels:            pod.Labels,
		Annotations:       pod.Annotations,
	}
}

// createEvictedEvent creates an event for evicted pods
func (w *PodWatcher) createEvictedEvent(pod *corev1.Pod, info *eventdriven.ResourceEventInfo) *eventdriven.KubernetesEvent {
	ownerKind, ownerName, ownerUID := info.GetOwnerInfo()

	return &eventdriven.KubernetesEvent{
		ID:                fmt.Sprintf("%s-evicted", string(pod.UID)),
		Type:              eventdriven.EventTypePodEvicted,
		Severity:          eventdriven.SeverityCritical,
		Timestamp:         time.Now(),
		ResourceKind:      "Pod",
		ResourceName:      pod.Name,
		ResourceNamespace: pod.Namespace,
		ResourceUID:       string(pod.UID),
		OwnerKind:         ownerKind,
		OwnerName:         ownerName,
		OwnerUID:          ownerUID,
		Reason:            "Evicted",
		Message:           fmt.Sprintf("Pod evicted: %s", pod.Status.Message),
		Labels:            pod.Labels,
		Annotations:       pod.Annotations,
	}
}

// createPendingTimeoutEvent creates an event for pods stuck in pending
func (w *PodWatcher) createPendingTimeoutEvent(pod *corev1.Pod, info *eventdriven.ResourceEventInfo) *eventdriven.KubernetesEvent {
	ownerKind, ownerName, ownerUID := info.GetOwnerInfo()

	return &eventdriven.KubernetesEvent{
		ID:                fmt.Sprintf("%s-pending-timeout", string(pod.UID)),
		Type:              eventdriven.EventTypePodPending,
		Severity:          eventdriven.SeverityMedium,
		Timestamp:         time.Now(),
		ResourceKind:      "Pod",
		ResourceName:      pod.Name,
		ResourceNamespace: pod.Namespace,
		ResourceUID:       string(pod.UID),
		OwnerKind:         ownerKind,
		OwnerName:         ownerName,
		OwnerUID:          ownerUID,
		Reason:            "PendingTimeout",
		Message:           fmt.Sprintf("Pod has been pending for %v", time.Since(pod.CreationTimestamp.Time)),
		Labels:            pod.Labels,
		Annotations:       pod.Annotations,
	}
}

// Helper functions

func extractResourceInfo(pod *corev1.Pod) *eventdriven.ResourceEventInfo {
	return &eventdriven.ResourceEventInfo{
		Kind:        "Pod",
		Name:        pod.Name,
		Namespace:   pod.Namespace,
		UID:         string(pod.UID),
		Labels:      pod.Labels,
		Annotations: pod.Annotations,
		OwnerRefs:   pod.OwnerReferences,
		CreatedAt:   pod.CreationTimestamp.Time,
	}
}

func findContainerStatus(pod *corev1.Pod, name string) *corev1.ContainerStatus {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == name {
			return &status
		}
	}
	return nil
}
