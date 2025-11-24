package watchers

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/ignacio/lumo/internal/agent/eventdriven"
)

// DeploymentWatcher watches Deployment resources
type DeploymentWatcher struct {
	informer cache.SharedIndexInformer
	logger   *logrus.Entry
}

// NewDeploymentWatcher creates a new Deployment watcher
func NewDeploymentWatcher(logger *logrus.Logger) *DeploymentWatcher {
	return &DeploymentWatcher{
		logger: logger.WithField("watcher", "deployment"),
	}
}

func (w *DeploymentWatcher) Name() string {
	return "deployment"
}

func (w *DeploymentWatcher) Setup(factory informers.SharedInformerFactory, handler eventdriven.EventHandler) error {
	w.informer = factory.Apps().V1().Deployments().Informer()

	_, err := w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deployment := obj.(*appsv1.Deployment)
			w.checkDeployment(deployment, nil, handler)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeployment := oldObj.(*appsv1.Deployment)
			newDeployment := newObj.(*appsv1.Deployment)
			w.checkDeployment(newDeployment, oldDeployment, handler)
		},
		DeleteFunc: func(obj interface{}) {
			deployment := obj.(*appsv1.Deployment)
			w.logger.WithFields(logrus.Fields{
				"deployment": deployment.Name,
				"namespace":  deployment.Namespace,
			}).Debug("Deployment deleted")
		},
	})

	return err
}

func (w *DeploymentWatcher) GetInformer() cache.SharedIndexInformer {
	return w.informer
}

func (w *DeploymentWatcher) checkDeployment(deployment *appsv1.Deployment, oldDeployment *appsv1.Deployment, handler eventdriven.EventHandler) {
	events := make([]*eventdriven.KubernetesEvent, 0)

	// Check for ProgressDeadlineExceeded
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == "False" && condition.Reason == "ProgressDeadlineExceeded" {
			event := &eventdriven.KubernetesEvent{
				ID:                fmt.Sprintf("%s-progress-deadline", string(deployment.UID)),
				Type:              eventdriven.EventTypeDeploymentFailed,
				Severity:          eventdriven.SeverityHigh,
				Timestamp:         time.Now(),
				ResourceKind:      "Deployment",
				ResourceName:      deployment.Name,
				ResourceNamespace: deployment.Namespace,
				ResourceUID:       string(deployment.UID),
				OwnerKind:         "Deployment",
				OwnerName:         deployment.Name,
				OwnerUID:          string(deployment.UID),
				Reason:            "ProgressDeadlineExceeded",
				Message:           fmt.Sprintf("Deployment rollout has exceeded progress deadline: %s", condition.Message),
				Labels:            deployment.Labels,
				Annotations:       deployment.Annotations,
			}
			events = append(events, event)
		}
	}

	// Check for replica mismatch (unavailable replicas)
	if deployment.Spec.Replicas != nil && deployment.Status.UnavailableReplicas > 0 {
		// Only alert if this is new or has changed
		if oldDeployment == nil || oldDeployment.Status.UnavailableReplicas != deployment.Status.UnavailableReplicas {
			event := &eventdriven.KubernetesEvent{
				ID:                fmt.Sprintf("%s-unavailable-replicas", string(deployment.UID)),
				Type:              eventdriven.EventTypeDeploymentFailed,
				Severity:          eventdriven.SeverityMedium,
				Timestamp:         time.Now(),
				ResourceKind:      "Deployment",
				ResourceName:      deployment.Name,
				ResourceNamespace: deployment.Namespace,
				ResourceUID:       string(deployment.UID),
				OwnerKind:         "Deployment",
				OwnerName:         deployment.Name,
				OwnerUID:          string(deployment.UID),
				Reason:            "UnavailableReplicas",
				Message: fmt.Sprintf("Deployment has %d unavailable replicas (desired: %d, available: %d)",
					deployment.Status.UnavailableReplicas,
					*deployment.Spec.Replicas,
					deployment.Status.AvailableReplicas,
				),
				Labels:      deployment.Labels,
				Annotations: deployment.Annotations,
			}
			events = append(events, event)
		}
	}

	// Send events to handler
	for _, event := range events {
		handler.HandleEvent(event)
	}
}

// StatefulSetWatcher watches StatefulSet resources
type StatefulSetWatcher struct {
	informer cache.SharedIndexInformer
	logger   *logrus.Entry
}

// NewStatefulSetWatcher creates a new StatefulSet watcher
func NewStatefulSetWatcher(logger *logrus.Logger) *StatefulSetWatcher {
	return &StatefulSetWatcher{
		logger: logger.WithField("watcher", "statefulset"),
	}
}

func (w *StatefulSetWatcher) Name() string {
	return "statefulset"
}

func (w *StatefulSetWatcher) Setup(factory informers.SharedInformerFactory, handler eventdriven.EventHandler) error {
	w.informer = factory.Apps().V1().StatefulSets().Informer()

	_, err := w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			sts := obj.(*appsv1.StatefulSet)
			w.checkStatefulSet(sts, nil, handler)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSts := oldObj.(*appsv1.StatefulSet)
			newSts := newObj.(*appsv1.StatefulSet)
			w.checkStatefulSet(newSts, oldSts, handler)
		},
		DeleteFunc: func(obj interface{}) {
			sts := obj.(*appsv1.StatefulSet)
			w.logger.WithFields(logrus.Fields{
				"statefulset": sts.Name,
				"namespace":   sts.Namespace,
			}).Debug("StatefulSet deleted")
		},
	})

	return err
}

func (w *StatefulSetWatcher) GetInformer() cache.SharedIndexInformer {
	return w.informer
}

func (w *StatefulSetWatcher) checkStatefulSet(sts *appsv1.StatefulSet, oldSts *appsv1.StatefulSet, handler eventdriven.EventHandler) {
	events := make([]*eventdriven.KubernetesEvent, 0)

	// Check if not all replicas are ready
	if sts.Spec.Replicas != nil && sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		// Only alert if this is new or has gotten worse
		shouldAlert := oldSts == nil || oldSts.Status.ReadyReplicas != sts.Status.ReadyReplicas

		if shouldAlert {
			event := &eventdriven.KubernetesEvent{
				ID:                fmt.Sprintf("%s-not-ready", string(sts.UID)),
				Type:              eventdriven.EventTypeStatefulSetFailed,
				Severity:          eventdriven.SeverityMedium,
				Timestamp:         time.Now(),
				ResourceKind:      "StatefulSet",
				ResourceName:      sts.Name,
				ResourceNamespace: sts.Namespace,
				ResourceUID:       string(sts.UID),
				OwnerKind:         "StatefulSet",
				OwnerName:         sts.Name,
				OwnerUID:          string(sts.UID),
				Reason:            "NotAllReplicasReady",
				Message: fmt.Sprintf("StatefulSet has %d/%d ready replicas",
					sts.Status.ReadyReplicas,
					*sts.Spec.Replicas,
				),
				Labels:      sts.Labels,
				Annotations: sts.Annotations,
			}
			events = append(events, event)
		}
	}

	// Check for update failures
	if sts.Spec.Replicas != nil && sts.Status.CurrentRevision != sts.Status.UpdateRevision && sts.Status.UpdatedReplicas < *sts.Spec.Replicas {
		event := &eventdriven.KubernetesEvent{
			ID:                fmt.Sprintf("%s-update-stuck", string(sts.UID)),
			Type:              eventdriven.EventTypeStatefulSetFailed,
			Severity:          eventdriven.SeverityHigh,
			Timestamp:         time.Now(),
			ResourceKind:      "StatefulSet",
			ResourceName:      sts.Name,
			ResourceNamespace: sts.Namespace,
			ResourceUID:       string(sts.UID),
			OwnerKind:         "StatefulSet",
			OwnerName:         sts.Name,
			OwnerUID:          string(sts.UID),
			Reason:            "UpdateStuck",
			Message: fmt.Sprintf("StatefulSet update stuck: %d/%d replicas updated",
				sts.Status.UpdatedReplicas,
				*sts.Spec.Replicas,
			),
			Labels:      sts.Labels,
			Annotations: sts.Annotations,
		}
		events = append(events, event)
	}

	// Send events to handler
	for _, event := range events {
		handler.HandleEvent(event)
	}
}

// DaemonSetWatcher watches DaemonSet resources
type DaemonSetWatcher struct {
	informer cache.SharedIndexInformer
	logger   *logrus.Entry
}

// NewDaemonSetWatcher creates a new DaemonSet watcher
func NewDaemonSetWatcher(logger *logrus.Logger) *DaemonSetWatcher {
	return &DaemonSetWatcher{
		logger: logger.WithField("watcher", "daemonset"),
	}
}

func (w *DaemonSetWatcher) Name() string {
	return "daemonset"
}

func (w *DaemonSetWatcher) Setup(factory informers.SharedInformerFactory, handler eventdriven.EventHandler) error {
	w.informer = factory.Apps().V1().DaemonSets().Informer()

	_, err := w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ds := obj.(*appsv1.DaemonSet)
			w.checkDaemonSet(ds, nil, handler)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDs := oldObj.(*appsv1.DaemonSet)
			newDs := newObj.(*appsv1.DaemonSet)
			w.checkDaemonSet(newDs, oldDs, handler)
		},
		DeleteFunc: func(obj interface{}) {
			ds := obj.(*appsv1.DaemonSet)
			w.logger.WithFields(logrus.Fields{
				"daemonset": ds.Name,
				"namespace": ds.Namespace,
			}).Debug("DaemonSet deleted")
		},
	})

	return err
}

func (w *DaemonSetWatcher) GetInformer() cache.SharedIndexInformer {
	return w.informer
}

func (w *DaemonSetWatcher) checkDaemonSet(ds *appsv1.DaemonSet, oldDs *appsv1.DaemonSet, handler eventdriven.EventHandler) {
	events := make([]*eventdriven.KubernetesEvent, 0)

	// Check if not all desired pods are scheduled
	if ds.Status.DesiredNumberScheduled > ds.Status.CurrentNumberScheduled {
		shouldAlert := oldDs == nil || oldDs.Status.CurrentNumberScheduled != ds.Status.CurrentNumberScheduled

		if shouldAlert {
			event := &eventdriven.KubernetesEvent{
				ID:                fmt.Sprintf("%s-scheduling-failed", string(ds.UID)),
				Type:              eventdriven.EventTypeDaemonSetFailed,
				Severity:          eventdriven.SeverityHigh,
				Timestamp:         time.Now(),
				ResourceKind:      "DaemonSet",
				ResourceName:      ds.Name,
				ResourceNamespace: ds.Namespace,
				ResourceUID:       string(ds.UID),
				OwnerKind:         "DaemonSet",
				OwnerName:         ds.Name,
				OwnerUID:          string(ds.UID),
				Reason:            "SchedulingFailed",
				Message: fmt.Sprintf("DaemonSet has %d/%d pods scheduled",
					ds.Status.CurrentNumberScheduled,
					ds.Status.DesiredNumberScheduled,
				),
				Labels:      ds.Labels,
				Annotations: ds.Annotations,
			}
			events = append(events, event)
		}
	}

	// Check if not all scheduled pods are ready
	if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
		shouldAlert := oldDs == nil || oldDs.Status.NumberReady != ds.Status.NumberReady

		if shouldAlert {
			event := &eventdriven.KubernetesEvent{
				ID:                fmt.Sprintf("%s-not-ready", string(ds.UID)),
				Type:              eventdriven.EventTypeDaemonSetFailed,
				Severity:          eventdriven.SeverityMedium,
				Timestamp:         time.Now(),
				ResourceKind:      "DaemonSet",
				ResourceName:      ds.Name,
				ResourceNamespace: ds.Namespace,
				ResourceUID:       string(ds.UID),
				OwnerKind:         "DaemonSet",
				OwnerName:         ds.Name,
				OwnerUID:          string(ds.UID),
				Reason:            "NotAllPodsReady",
				Message: fmt.Sprintf("DaemonSet has %d/%d ready pods",
					ds.Status.NumberReady,
					ds.Status.DesiredNumberScheduled,
				),
				Labels:      ds.Labels,
				Annotations: ds.Annotations,
			}
			events = append(events, event)
		}
	}

	// Send events to handler
	for _, event := range events {
		handler.HandleEvent(event)
	}
}

// JobWatcher watches Job resources
type JobWatcher struct {
	informer cache.SharedIndexInformer
	logger   *logrus.Entry
}

// NewJobWatcher creates a new Job watcher
func NewJobWatcher(logger *logrus.Logger) *JobWatcher {
	return &JobWatcher{
		logger: logger.WithField("watcher", "job"),
	}
}

func (w *JobWatcher) Name() string {
	return "job"
}

func (w *JobWatcher) Setup(factory informers.SharedInformerFactory, handler eventdriven.EventHandler) error {
	w.informer = factory.Batch().V1().Jobs().Informer()

	_, err := w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			job := obj.(*batchv1.Job)
			w.checkJob(job, nil, handler)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldJob := oldObj.(*batchv1.Job)
			newJob := newObj.(*batchv1.Job)
			w.checkJob(newJob, oldJob, handler)
		},
		DeleteFunc: func(obj interface{}) {
			job := obj.(*batchv1.Job)
			w.logger.WithFields(logrus.Fields{
				"job":       job.Name,
				"namespace": job.Namespace,
			}).Debug("Job deleted")
		},
	})

	return err
}

func (w *JobWatcher) GetInformer() cache.SharedIndexInformer {
	return w.informer
}

func (w *JobWatcher) checkJob(job *batchv1.Job, oldJob *batchv1.Job, handler eventdriven.EventHandler) {
	events := make([]*eventdriven.KubernetesEvent, 0)

	// Check for job failure
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == "True" {
			event := &eventdriven.KubernetesEvent{
				ID:                fmt.Sprintf("%s-failed", string(job.UID)),
				Type:              eventdriven.EventTypeJobFailed,
				Severity:          eventdriven.SeverityCritical,
				Timestamp:         time.Now(),
				ResourceKind:      "Job",
				ResourceName:      job.Name,
				ResourceNamespace: job.Namespace,
				ResourceUID:       string(job.UID),
				OwnerKind:         "Job",
				OwnerName:         job.Name,
				OwnerUID:          string(job.UID),
				Reason:            condition.Reason,
				Message:           fmt.Sprintf("Job failed: %s", condition.Message),
				Labels:            job.Labels,
				Annotations:       job.Annotations,
			}
			events = append(events, event)
		}
	}

	// Check for BackoffLimitExceeded
	if job.Status.Failed > 0 && job.Spec.BackoffLimit != nil && job.Status.Failed >= *job.Spec.BackoffLimit {
		event := &eventdriven.KubernetesEvent{
			ID:                fmt.Sprintf("%s-backoff-limit", string(job.UID)),
			Type:              eventdriven.EventTypeJobFailed,
			Severity:          eventdriven.SeverityCritical,
			Timestamp:         time.Now(),
			ResourceKind:      "Job",
			ResourceName:      job.Name,
			ResourceNamespace: job.Namespace,
			ResourceUID:       string(job.UID),
			OwnerKind:         "Job",
			OwnerName:         job.Name,
			OwnerUID:          string(job.UID),
			Reason:            "BackoffLimitExceeded",
			Message: fmt.Sprintf("Job has failed %d times (backoff limit: %d)",
				job.Status.Failed,
				*job.Spec.BackoffLimit,
			),
			Labels:      job.Labels,
			Annotations: job.Annotations,
		}
		events = append(events, event)
	}

	// Send events to handler
	for _, event := range events {
		handler.HandleEvent(event)
	}
}
