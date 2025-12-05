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

// NodeWatcher watches Node resources for health and pressure issues
type NodeWatcher struct {
	informer cache.SharedIndexInformer
	logger   *logrus.Entry
}

// NewNodeWatcher creates a new Node watcher
func NewNodeWatcher(logger *logrus.Logger) *NodeWatcher {
	return &NodeWatcher{
		logger: logger.WithField("watcher", "node"),
	}
}

func (w *NodeWatcher) Name() string {
	return "node"
}

func (w *NodeWatcher) Setup(factory informers.SharedInformerFactory, handler eventdriven.EventHandler) error {
	w.informer = factory.Core().V1().Nodes().Informer()

	_, err := w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node, ok := obj.(*corev1.Node)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					w.logger.WithField("object", fmt.Sprintf("%T", obj)).Error("Unexpected object type in AddFunc")
					return
				}
				node, ok = tombstone.Obj.(*corev1.Node)
				if !ok {
					w.logger.WithField("object", fmt.Sprintf("%T", tombstone.Obj)).Error("Unexpected object type in tombstone")
					return
				}
			}
			w.checkNode(node, nil, handler)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNode, ok := oldObj.(*corev1.Node)
			if !ok {
				w.logger.WithField("object", fmt.Sprintf("%T", oldObj)).Error("Unexpected old object type in UpdateFunc")
				return
			}
			newNode, ok := newObj.(*corev1.Node)
			if !ok {
				w.logger.WithField("object", fmt.Sprintf("%T", newObj)).Error("Unexpected new object type in UpdateFunc")
				return
			}
			w.checkNode(newNode, oldNode, handler)
		},
		DeleteFunc: func(obj interface{}) {
			node, ok := obj.(*corev1.Node)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					w.logger.WithField("object", fmt.Sprintf("%T", obj)).Error("Unexpected object type in DeleteFunc")
					return
				}
				node, ok = tombstone.Obj.(*corev1.Node)
				if !ok {
					w.logger.WithField("object", fmt.Sprintf("%T", tombstone.Obj)).Error("Unexpected object type in tombstone")
					return
				}
			}
			w.logger.WithField("node", node.Name).Debug("Node deleted")
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add node event handler: %w", err)
	}

	return nil
}

func (w *NodeWatcher) GetInformer() cache.SharedIndexInformer {
	return w.informer
}

func (w *NodeWatcher) checkNode(node *corev1.Node, oldNode *corev1.Node, handler eventdriven.EventHandler) {
	events := make([]*eventdriven.KubernetesEvent, 0)

	// Check node conditions
	for _, condition := range node.Status.Conditions {
		var event *eventdriven.KubernetesEvent

		switch condition.Type {
		case corev1.NodeReady:
			// Node is NotReady
			if condition.Status != corev1.ConditionTrue {
				// Only alert if this is a new NotReady state
				if oldNode != nil {
					oldReady := getNodeCondition(oldNode, corev1.NodeReady)
					if oldReady != nil && oldReady.Status != corev1.ConditionTrue {
						// Already was NotReady, skip
						continue
					}
				}

				event = &eventdriven.KubernetesEvent{
					ID:           fmt.Sprintf("%s-not-ready", string(node.UID)),
					Type:         eventdriven.EventTypeNodeNotReady,
					Severity:     eventdriven.SeverityCritical,
					Timestamp:    time.Now(),
					ResourceKind: "Node",
					ResourceName: node.Name,
					ResourceUID:  string(node.UID),
					OwnerKind:    "Node",
					OwnerName:    node.Name,
					OwnerUID:     string(node.UID),
					Reason:       condition.Reason,
					Message:      fmt.Sprintf("Node is NotReady: %s", condition.Message),
					Labels:       node.Labels,
					Annotations:  node.Annotations,
				}
			}

		case corev1.NodeMemoryPressure:
			// Memory pressure detected
			if condition.Status == corev1.ConditionTrue {
				// Only alert if this is a new pressure state
				if oldNode != nil {
					oldCondition := getNodeCondition(oldNode, corev1.NodeMemoryPressure)
					if oldCondition != nil && oldCondition.Status == corev1.ConditionTrue {
						continue
					}
				}

				event = &eventdriven.KubernetesEvent{
					ID:           fmt.Sprintf("%s-memory-pressure", string(node.UID)),
					Type:         eventdriven.EventTypeNodeMemoryPressure,
					Severity:     eventdriven.SeverityHigh,
					Timestamp:    time.Now(),
					ResourceKind: "Node",
					ResourceName: node.Name,
					ResourceUID:  string(node.UID),
					OwnerKind:    "Node",
					OwnerName:    node.Name,
					OwnerUID:     string(node.UID),
					Reason:       condition.Reason,
					Message:      fmt.Sprintf("Node under memory pressure: %s", condition.Message),
					Labels:       node.Labels,
					Annotations:  node.Annotations,
				}
			}

		case corev1.NodeDiskPressure:
			// Disk pressure detected
			if condition.Status == corev1.ConditionTrue {
				if oldNode != nil {
					oldCondition := getNodeCondition(oldNode, corev1.NodeDiskPressure)
					if oldCondition != nil && oldCondition.Status == corev1.ConditionTrue {
						continue
					}
				}

				event = &eventdriven.KubernetesEvent{
					ID:           fmt.Sprintf("%s-disk-pressure", string(node.UID)),
					Type:         eventdriven.EventTypeNodeDiskPressure,
					Severity:     eventdriven.SeverityHigh,
					Timestamp:    time.Now(),
					ResourceKind: "Node",
					ResourceName: node.Name,
					ResourceUID:  string(node.UID),
					OwnerKind:    "Node",
					OwnerName:    node.Name,
					OwnerUID:     string(node.UID),
					Reason:       condition.Reason,
					Message:      fmt.Sprintf("Node under disk pressure: %s", condition.Message),
					Labels:       node.Labels,
					Annotations:  node.Annotations,
				}
			}

		case corev1.NodePIDPressure:
			// PID pressure detected
			if condition.Status == corev1.ConditionTrue {
				if oldNode != nil {
					oldCondition := getNodeCondition(oldNode, corev1.NodePIDPressure)
					if oldCondition != nil && oldCondition.Status == corev1.ConditionTrue {
						continue
					}
				}

				event = &eventdriven.KubernetesEvent{
					ID:           fmt.Sprintf("%s-pid-pressure", string(node.UID)),
					Type:         eventdriven.EventTypeNodePIDPressure,
					Severity:     eventdriven.SeverityMedium,
					Timestamp:    time.Now(),
					ResourceKind: "Node",
					ResourceName: node.Name,
					ResourceUID:  string(node.UID),
					OwnerKind:    "Node",
					OwnerName:    node.Name,
					OwnerUID:     string(node.UID),
					Reason:       condition.Reason,
					Message:      fmt.Sprintf("Node under PID pressure: %s", condition.Message),
					Labels:       node.Labels,
					Annotations:  node.Annotations,
				}
			}

		case corev1.NodeNetworkUnavailable:
			// Network unavailable
			if condition.Status == corev1.ConditionTrue {
				if oldNode != nil {
					oldCondition := getNodeCondition(oldNode, corev1.NodeNetworkUnavailable)
					if oldCondition != nil && oldCondition.Status == corev1.ConditionTrue {
						continue
					}
				}

				event = &eventdriven.KubernetesEvent{
					ID:           fmt.Sprintf("%s-network-unavailable", string(node.UID)),
					Type:         eventdriven.EventTypeNodeNetworkUnavail,
					Severity:     eventdriven.SeverityHigh,
					Timestamp:    time.Now(),
					ResourceKind: "Node",
					ResourceName: node.Name,
					ResourceUID:  string(node.UID),
					OwnerKind:    "Node",
					OwnerName:    node.Name,
					OwnerUID:     string(node.UID),
					Reason:       condition.Reason,
					Message:      fmt.Sprintf("Node network unavailable: %s", condition.Message),
					Labels:       node.Labels,
					Annotations:  node.Annotations,
				}
			}
		}

		if event != nil {
			events = append(events, event)
		}
	}

	// Check if node is unschedulable
	if node.Spec.Unschedulable {
		// Only alert if this is a new unschedulable state
		if oldNode == nil || !oldNode.Spec.Unschedulable {
			event := &eventdriven.KubernetesEvent{
				ID:           fmt.Sprintf("%s-unschedulable", string(node.UID)),
				Type:         eventdriven.EventTypeNodeNotReady,
				Severity:     eventdriven.SeverityMedium,
				Timestamp:    time.Now(),
				ResourceKind: "Node",
				ResourceName: node.Name,
				ResourceUID:  string(node.UID),
				OwnerKind:    "Node",
				OwnerName:    node.Name,
				OwnerUID:     string(node.UID),
				Reason:       "Unschedulable",
				Message:      "Node has been marked as unschedulable",
				Labels:       node.Labels,
				Annotations:  node.Annotations,
			}
			events = append(events, event)
		}
	}

	// Send events to handler
	for _, event := range events {
		handler.HandleEvent(event)
	}
}

// getNodeCondition retrieves a specific node condition
func getNodeCondition(node *corev1.Node, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
	for _, condition := range node.Status.Conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}
