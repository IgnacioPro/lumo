// Package leaderelection provides leader election for Lumo agents using Kubernetes Lease objects.
// Only the leader agent actively watches and processes Kubernetes events;
// standby replicas wait to take over if the leader fails.
package leaderelection

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// Config holds leader election configuration
type Config struct {
	// LeaseName is the name of the Lease resource
	LeaseName string
	// Namespace is the namespace for the Lease
	Namespace string
	// LeaseDuration is how long a leader holds the lease
	LeaseDuration time.Duration
	// RenewDeadline is how long the leader has to renew before losing leadership
	RenewDeadline time.Duration
	// RetryPeriod is how often non-leaders retry to acquire the lease
	RetryPeriod time.Duration
}

// DefaultConfig returns sensible defaults for leader election
func DefaultConfig(namespace string) *Config {
	return &Config{
		LeaseName:     "lumo-agent-leader",
		Namespace:     namespace,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
	}
}

// Callbacks holds the callbacks for leader election events
type Callbacks struct {
	// OnStartedLeading is called when this instance becomes the leader
	OnStartedLeading func(ctx context.Context)
	// OnStoppedLeading is called when this instance loses leadership
	OnStoppedLeading func()
	// OnNewLeader is called when a new leader is elected (including self)
	OnNewLeader func(identity string)
}

// Elector manages leader election for an agent
type Elector struct {
	clientset *kubernetes.Clientset
	config    *Config
	identity  string
	logger    *logrus.Entry
	isLeader  bool
}

// NewElector creates a new leader elector
func NewElector(clientset *kubernetes.Clientset, config *Config, logger *logrus.Logger) (*Elector, error) {
	if clientset == nil {
		return nil, fmt.Errorf("clientset cannot be nil")
	}
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Build unique identity: podname or hostname + uuid suffix
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	identity := fmt.Sprintf("%s-%s", hostname, uuid.New().String()[:8])

	return &Elector{
		clientset: clientset,
		config:    config,
		identity:  identity,
		logger:    logger.WithField("component", "leader-election"),
		isLeader:  false,
	}, nil
}

// Run starts the leader election process. This blocks until ctx is cancelled.
func (e *Elector) Run(ctx context.Context, callbacks Callbacks) error {
	e.logger.WithFields(logrus.Fields{
		"identity":       e.identity,
		"lease_name":     e.config.LeaseName,
		"namespace":      e.config.Namespace,
		"lease_duration": e.config.LeaseDuration,
	}).Info("Starting leader election")

	// Create the Lease lock
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      e.config.LeaseName,
			Namespace: e.config.Namespace,
		},
		Client: e.clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: e.identity,
		},
	}

	// Create leader election config
	lec := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   e.config.LeaseDuration,
		RenewDeadline:   e.config.RenewDeadline,
		RetryPeriod:     e.config.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				e.isLeader = true
				e.logger.WithField("identity", e.identity).Info("Became leader - starting event processing")
				if callbacks.OnStartedLeading != nil {
					callbacks.OnStartedLeading(ctx)
				}
			},
			OnStoppedLeading: func() {
				e.isLeader = false
				e.logger.WithField("identity", e.identity).Warn("Lost leadership - stopping event processing")
				if callbacks.OnStoppedLeading != nil {
					callbacks.OnStoppedLeading()
				}
			},
			OnNewLeader: func(identity string) {
				if identity == e.identity {
					e.logger.Info("Still the leader")
				} else {
					e.logger.WithField("leader", identity).Info("New leader elected - standing by")
				}
				if callbacks.OnNewLeader != nil {
					callbacks.OnNewLeader(identity)
				}
			},
		},
	}

	// Run leader election (blocks until ctx cancelled)
	leaderelection.RunOrDie(ctx, lec)

	return nil
}

// IsLeader returns true if this instance is currently the leader
func (e *Elector) IsLeader() bool {
	return e.isLeader
}

// Identity returns this elector's unique identity
func (e *Elector) Identity() string {
	return e.identity
}
