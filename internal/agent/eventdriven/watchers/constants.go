package watchers

import "time"

// Threshold constants for event detection
const (
	// HighRestartThreshold is the number of restarts that triggers a high-restart event
	HighRestartThreshold = 5

	// PendingTimeoutDuration is how long a pod can be pending before triggering an event
	PendingTimeoutDuration = 5 * time.Minute

	// ContainerCreatingTimeout is how long a container can be in ContainerCreating state
	ContainerCreatingTimeout = 2 * time.Minute

	// PVCPendingTimeout is how long a PVC can be pending before triggering an event
	PVCPendingTimeout = 2 * time.Minute

	// DeploymentProgressTimeout is how long to wait for deployment progress
	DeploymentProgressTimeout = 10 * time.Minute
)
