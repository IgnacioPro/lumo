package watchers

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/ignacio/lumo/internal/agent/eventdriven"
)

// PVCWatcher watches PersistentVolumeClaim resources
type PVCWatcher struct {
	informer cache.SharedIndexInformer
	logger   *logrus.Entry
}

// NewPVCWatcher creates a new PVC watcher
func NewPVCWatcher(logger *logrus.Logger) *PVCWatcher {
	return &PVCWatcher{
		logger: logger.WithField("watcher", "pvc"),
	}
}

func (w *PVCWatcher) Name() string {
	return "pvc"
}

func (w *PVCWatcher) Setup(factory informers.SharedInformerFactory, handler eventdriven.EventHandler) error {
	w.informer = factory.Core().V1().PersistentVolumeClaims().Informer()

	_, err := w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pvc := obj.(*corev1.PersistentVolumeClaim)
			w.checkPVC(pvc, nil, handler)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPVC := oldObj.(*corev1.PersistentVolumeClaim)
			newPVC := newObj.(*corev1.PersistentVolumeClaim)
			w.checkPVC(newPVC, oldPVC, handler)
		},
		DeleteFunc: func(obj interface{}) {
			pvc := obj.(*corev1.PersistentVolumeClaim)
			w.logger.WithFields(logrus.Fields{
				"pvc":       pvc.Name,
				"namespace": pvc.Namespace,
			}).Debug("PVC deleted")
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add PVC event handler: %w", err)
	}

	return nil
}

func (w *PVCWatcher) GetInformer() cache.SharedIndexInformer {
	return w.informer
}

func (w *PVCWatcher) checkPVC(pvc *corev1.PersistentVolumeClaim, oldPVC *corev1.PersistentVolumeClaim, handler eventdriven.EventHandler) {
	events := make([]*eventdriven.KubernetesEvent, 0)

	// Check PVC phase
	switch pvc.Status.Phase {
	case corev1.ClaimPending:
		// Only alert if pending for more than PVCPendingTimeout
		pendingDuration := time.Since(pvc.CreationTimestamp.Time)
		if pendingDuration > PVCPendingTimeout {
			// Check if we should trigger an event:
			// 1. New PVC (oldPVC == nil) that's been pending too long
			// 2. Status changed to Pending (oldPVC.Status.Phase != corev1.ClaimPending)
			// 3. PVC is still pending and we haven't recently alerted (check if oldPVC was also pending for >2 min)
			shouldTrigger := false

			if oldPVC == nil {
				// New PVC first seen - trigger if already pending too long
				shouldTrigger = true
			} else if oldPVC.Status.Phase != corev1.ClaimPending {
				// Status just changed to Pending - trigger
				shouldTrigger = true
			} else {
				// PVC was already pending - check if old one was under threshold
				oldPendingDuration := time.Since(oldPVC.CreationTimestamp.Time)
				if oldPendingDuration <= PVCPendingTimeout {
					// Just crossed the threshold - trigger
					shouldTrigger = true
				}
			}

			if shouldTrigger {
				event := &eventdriven.KubernetesEvent{
					ID:                fmt.Sprintf("%s-pending", string(pvc.UID)),
					Type:              eventdriven.EventTypePVCProvisionFailed,
					Severity:          eventdriven.SeverityMedium,
					Timestamp:         time.Now(),
					ResourceKind:      "PersistentVolumeClaim",
					ResourceName:      pvc.Name,
					ResourceNamespace: pvc.Namespace,
					ResourceUID:       string(pvc.UID),
					OwnerKind:         "PersistentVolumeClaim",
					OwnerName:         pvc.Name,
					OwnerUID:          string(pvc.UID),
					Reason:            "PendingTooLong",
					Message: fmt.Sprintf("PVC has been pending for %v",
						pendingDuration),
					Labels:      pvc.Labels,
					Annotations: pvc.Annotations,
				}
				events = append(events, event)
			}
		}

	case corev1.ClaimLost:
		event := &eventdriven.KubernetesEvent{
			ID:                fmt.Sprintf("%s-lost", string(pvc.UID)),
			Type:              eventdriven.EventTypeVolumeFailedBinding,
			Severity:          eventdriven.SeverityCritical,
			Timestamp:         time.Now(),
			ResourceKind:      "PersistentVolumeClaim",
			ResourceName:      pvc.Name,
			ResourceNamespace: pvc.Namespace,
			ResourceUID:       string(pvc.UID),
			OwnerKind:         "PersistentVolumeClaim",
			OwnerName:         pvc.Name,
			OwnerUID:          string(pvc.UID),
			Reason:            "PVCLost",
			Message:           "PVC is in Lost state - underlying volume no longer exists",
			Labels:            pvc.Labels,
			Annotations:       pvc.Annotations,
		}
		events = append(events, event)
	}

	// Check for conditions indicating issues
	for _, condition := range pvc.Status.Conditions {
		if condition.Status == corev1.ConditionFalse {
			var eventType eventdriven.EventType
			var severity eventdriven.Severity

			switch condition.Type {
			case corev1.PersistentVolumeClaimResizing:
				eventType = eventdriven.EventTypePVCProvisionFailed
				severity = eventdriven.SeverityHigh
			default:
				eventType = eventdriven.EventTypePVCProvisionFailed
				severity = eventdriven.SeverityMedium
			}

			event := &eventdriven.KubernetesEvent{
				ID:                fmt.Sprintf("%s-condition-%s", string(pvc.UID), string(condition.Type)),
				Type:              eventType,
				Severity:          severity,
				Timestamp:         time.Now(),
				ResourceKind:      "PersistentVolumeClaim",
				ResourceName:      pvc.Name,
				ResourceNamespace: pvc.Namespace,
				ResourceUID:       string(pvc.UID),
				OwnerKind:         "PersistentVolumeClaim",
				OwnerName:         pvc.Name,
				OwnerUID:          string(pvc.UID),
				Reason:            condition.Reason,
				Message:           fmt.Sprintf("PVC condition %s is false: %s", condition.Type, condition.Message),
				Labels:            pvc.Labels,
				Annotations:       pvc.Annotations,
			}
			events = append(events, event)
		}
	}

	// Send events to handler
	for _, event := range events {
		handler.HandleEvent(event)
	}
}

// EventWatcher watches Kubernetes Event resources to detect volume mount failures
type EventWatcher struct {
	informer cache.SharedIndexInformer
	logger   *logrus.Entry
}

// NewEventWatcher creates a new Event watcher for volume-related events
func NewEventWatcher(logger *logrus.Logger) *EventWatcher {
	return &EventWatcher{
		logger: logger.WithField("watcher", "event"),
	}
}

func (w *EventWatcher) Name() string {
	return "event"
}

func (w *EventWatcher) Setup(factory informers.SharedInformerFactory, handler eventdriven.EventHandler) error {
	w.informer = factory.Core().V1().Events().Informer()

	_, err := w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			event := obj.(*corev1.Event)
			w.processEvent(event, handler)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			event := newObj.(*corev1.Event)
			w.processEvent(event, handler)
		},
		DeleteFunc: func(obj interface{}) {
			// Don't care about event deletions
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	return nil
}

func (w *EventWatcher) GetInformer() cache.SharedIndexInformer {
	return w.informer
}

func (w *EventWatcher) processEvent(k8sEvent *corev1.Event, handler eventdriven.EventHandler) {
	// Only process Warning and Error events
	if k8sEvent.Type != corev1.EventTypeWarning && k8sEvent.Type != "Error" {
		return
	}

	// Filter for specific reasons we care about
	var eventType eventdriven.EventType
	var severity eventdriven.Severity

	switch k8sEvent.Reason {
	// Volume mount failures
	case "FailedMount":
		eventType = eventdriven.EventTypeVolumeFailedMount
		severity = eventdriven.SeverityHigh
	case "FailedAttachVolume":
		eventType = eventdriven.EventTypeVolumeFailedMount
		severity = eventdriven.SeverityHigh
	case "VolumeResizeFailed":
		eventType = eventdriven.EventTypePVCProvisionFailed
		severity = eventdriven.SeverityMedium

	// PVC binding failures
	case "FailedBinding":
		eventType = eventdriven.EventTypeVolumeFailedBinding
		severity = eventdriven.SeverityHigh
	case "ProvisioningFailed":
		eventType = eventdriven.EventTypePVCProvisionFailed
		severity = eventdriven.SeverityHigh

	// Scheduling issues
	case "FailedScheduling":
		eventType = eventdriven.EventTypeSchedulingFailed
		severity = eventdriven.SeverityMedium
	case "InsufficientMemory", "InsufficientCPU", "InsufficientStorage":
		eventType = eventdriven.EventTypeSchedulingFailed
		severity = eventdriven.SeverityHigh

	// Image pull issues (backup detection)
	case "Failed", "BackOff":
		if k8sEvent.InvolvedObject.Kind == "Pod" {
			eventType = eventdriven.EventTypeImagePullBackOff
			severity = eventdriven.SeverityHigh
		} else {
			return
		}

	default:
		// Not an event type we're interested in
		return
	}

	// Create our internal event
	lumoEvent := &eventdriven.KubernetesEvent{
		ID:                fmt.Sprintf("%s-%s-%d", string(k8sEvent.InvolvedObject.UID), k8sEvent.Reason, k8sEvent.Count),
		Type:              eventType,
		Severity:          severity,
		Timestamp:         time.Now(),
		ResourceKind:      k8sEvent.InvolvedObject.Kind,
		ResourceName:      k8sEvent.InvolvedObject.Name,
		ResourceNamespace: k8sEvent.InvolvedObject.Namespace,
		ResourceUID:       string(k8sEvent.InvolvedObject.UID),
		OwnerKind:         k8sEvent.InvolvedObject.Kind,
		OwnerName:         k8sEvent.InvolvedObject.Name,
		OwnerUID:          string(k8sEvent.InvolvedObject.UID),
		Reason:            k8sEvent.Reason,
		Message:           k8sEvent.Message,
		Count:             k8sEvent.Count,
	}

	// Process through handler
	handler.HandleEvent(lumoEvent)
}
