package models

import (
	"time"

	"github.com/google/uuid"
)

// AgentPlatform represents the agent's operating system/platform
type AgentPlatform string

const (
	AgentPlatformLinux      AgentPlatform = "linux"
	AgentPlatformDarwin     AgentPlatform = "darwin"
	AgentPlatformWindows    AgentPlatform = "windows"
	AgentPlatformKubernetes AgentPlatform = "kubernetes"
)

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	AgentStatusOnline  AgentStatus = "online"
	AgentStatusOffline AgentStatus = "offline"
	AgentStatusError   AgentStatus = "error"
)

// KubernetesMetadata contains Kubernetes-specific agent information
type KubernetesMetadata struct {
	Cluster   string `json:"cluster,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	NodeName  string `json:"node_name,omitempty"`
	PodName   string `json:"pod_name,omitempty"`
}

// Agent represents a registered agent
type Agent struct {
	ID                  uuid.UUID           `json:"id"`
	Name                string              `json:"name"`
	Hostname            string              `json:"hostname"`
	IPAddress           *string             `json:"ip_address,omitempty"`
	Platform            AgentPlatform       `json:"platform"`
	Architecture        string              `json:"architecture"`
	Version             string              `json:"version"`
	Status              AgentStatus         `json:"status"`
	Capabilities        []string            `json:"capabilities"`
	Labels              JSONB               `json:"labels,omitempty"`
	KubernetesMetadata  *KubernetesMetadata `json:"kubernetes_metadata,omitempty"`
	LastHeartbeatAt     time.Time           `json:"last_heartbeat_at"`
	RegisteredAt        time.Time           `json:"registered_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
}

// IsOnline returns true if the agent is currently online
func (a *Agent) IsOnline() bool {
	return a.Status == AgentStatusOnline
}

// IsOffline returns true if the agent is offline
func (a *Agent) IsOffline() bool {
	return a.Status == AgentStatusOffline
}

// HasError returns true if the agent is in an error state
func (a *Agent) HasError() bool {
	return a.Status == AgentStatusError
}

// IsKubernetes returns true if the agent is running on Kubernetes
func (a *Agent) IsKubernetes() bool {
	return a.Platform == AgentPlatformKubernetes
}

// HasCapability returns true if the agent has the specified capability/checker
func (a *Agent) HasCapability(capability string) bool {
	for _, c := range a.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// TimeSinceHeartbeat returns the duration since the last heartbeat
func (a *Agent) TimeSinceHeartbeat() time.Duration {
	return time.Since(a.LastHeartbeatAt)
}

// IsStale returns true if the agent hasn't sent a heartbeat recently
// An agent is considered stale if it hasn't sent a heartbeat in the given duration
func (a *Agent) IsStale(threshold time.Duration) bool {
	return a.TimeSinceHeartbeat() > threshold
}
