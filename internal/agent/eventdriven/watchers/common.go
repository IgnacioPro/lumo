package watchers

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ignacio/lumo/internal/agent/eventdriven"
)

// ExtractPodResourceInfo extracts ResourceEventInfo from a Pod.
func ExtractPodResourceInfo(pod *corev1.Pod) *eventdriven.ResourceEventInfo {
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

// ExtractNodeResourceInfo extracts ResourceEventInfo from a Node.
func ExtractNodeResourceInfo(node *corev1.Node) *eventdriven.ResourceEventInfo {
	return &eventdriven.ResourceEventInfo{
		Kind:        "Node",
		Name:        node.Name,
		Namespace:   "", // Nodes are cluster-scoped
		UID:         string(node.UID),
		Labels:      node.Labels,
		Annotations: node.Annotations,
		OwnerRefs:   node.OwnerReferences,
		CreatedAt:   node.CreationTimestamp.Time,
	}
}

// ExtractPVCResourceInfo extracts ResourceEventInfo from a PersistentVolumeClaim.
func ExtractPVCResourceInfo(pvc *corev1.PersistentVolumeClaim) *eventdriven.ResourceEventInfo {
	return &eventdriven.ResourceEventInfo{
		Kind:        "PersistentVolumeClaim",
		Name:        pvc.Name,
		Namespace:   pvc.Namespace,
		UID:         string(pvc.UID),
		Labels:      pvc.Labels,
		Annotations: pvc.Annotations,
		OwnerRefs:   pvc.OwnerReferences,
		CreatedAt:   pvc.CreationTimestamp.Time,
	}
}

// ExtractEventResourceInfo extracts ResourceEventInfo from a Kubernetes Event.
func ExtractEventResourceInfo(event *corev1.Event) *eventdriven.ResourceEventInfo {
	return &eventdriven.ResourceEventInfo{
		Kind:      "Event",
		Name:      event.Name,
		Namespace: event.Namespace,
		UID:       string(event.UID),
		CreatedAt: event.CreationTimestamp.Time,
	}
}

// ExtractGenericResourceInfo extracts ResourceEventInfo from ObjectMeta.
// Use this for resources that don't have a specific extractor.
func ExtractGenericResourceInfo(kind string, meta metav1.ObjectMeta) *eventdriven.ResourceEventInfo {
	return &eventdriven.ResourceEventInfo{
		Kind:        kind,
		Name:        meta.Name,
		Namespace:   meta.Namespace,
		UID:         string(meta.UID),
		Labels:      meta.Labels,
		Annotations: meta.Annotations,
		OwnerRefs:   meta.OwnerReferences,
		CreatedAt:   meta.CreationTimestamp.Time,
	}
}

// GetOwnerFromRefs extracts the first owner reference (typically the controller).
func GetOwnerFromRefs(refs []metav1.OwnerReference) (kind, name, uid string) {
	if len(refs) == 0 {
		return "", "", ""
	}
	// Prefer the controller owner
	for _, ref := range refs {
		if ref.Controller != nil && *ref.Controller {
			return ref.Kind, ref.Name, string(ref.UID)
		}
	}
	// Fall back to first owner
	return refs[0].Kind, refs[0].Name, string(refs[0].UID)
}

// TimeSinceCreation returns the duration since the resource was created.
func TimeSinceCreation(meta metav1.ObjectMeta) time.Duration {
	return time.Since(meta.CreationTimestamp.Time)
}
