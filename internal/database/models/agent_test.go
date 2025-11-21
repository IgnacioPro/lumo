package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAgentStatus(t *testing.T) {
	assert.Equal(t, AgentStatus("online"), AgentStatusOnline)
	assert.Equal(t, AgentStatus("offline"), AgentStatusOffline)
	assert.Equal(t, AgentStatus("error"), AgentStatusError)
}

func TestAgent_Basic(t *testing.T) {
	ipAddr := "192.168.1.100"
	agent := &Agent{
		ID:           uuid.New(),
		Name:         "test-agent",
		Hostname:     "test-host",
		IPAddress:    &ipAddr,
		Platform:     AgentPlatformLinux,
		Architecture: "amd64",
		Version:      "1.0.0",
		Status:       AgentStatusOnline,
		Capabilities: []string{"diagnostic", "remediation"},
		RegisteredAt: time.Now(),
	}

	assert.NotEqual(t, uuid.Nil, agent.ID)
	assert.Equal(t, "test-agent", agent.Name)
	assert.Equal(t, "test-host", agent.Hostname)
	assert.Equal(t, AgentStatusOnline, agent.Status)
	assert.Len(t, agent.Capabilities, 2)
}

func TestAgent_WithLabels(t *testing.T) {
	labels := JSONB{
		"environment": "production",
		"datacenter":  "us-west-2",
		"tier":        "critical",
	}

	agent := &Agent{
		ID:     uuid.New(),
		Name:   "prod-agent",
		Labels: labels,
		Status: AgentStatusOnline,
	}

	assert.Equal(t, "production", agent.Labels["environment"])
	assert.Equal(t, "us-west-2", agent.Labels["datacenter"])
	assert.Equal(t, "critical", agent.Labels["tier"])
}

func TestAgent_Heartbeat(t *testing.T) {
	now := time.Now()
	agent := &Agent{
		ID:              uuid.New(),
		Name:            "test-agent",
		Status:          AgentStatusOnline,
		LastHeartbeatAt: now,
		RegisteredAt:    now.Add(-1 * time.Hour),
	}

	assert.False(t, agent.LastHeartbeatAt.IsZero())
	assert.True(t, agent.LastHeartbeatAt.After(agent.RegisteredAt))

	// Test TimeSinceHeartbeat
	timeSince := agent.TimeSinceHeartbeat()
	assert.GreaterOrEqual(t, timeSince, time.Duration(0))

	// Test IsStale
	assert.False(t, agent.IsStale(10*time.Minute))
	assert.True(t, agent.IsStale(1*time.Nanosecond))
}

func TestAgent_StatusTransitions(t *testing.T) {
	agent := &Agent{
		ID:     uuid.New(),
		Name:   "test-agent",
		Status: AgentStatusOffline,
	}

	// Offline -> Online
	agent.Status = AgentStatusOnline
	assert.Equal(t, AgentStatusOnline, agent.Status)
	assert.True(t, agent.IsOnline())
	assert.False(t, agent.IsOffline())

	// Online -> Error
	agent.Status = AgentStatusError
	assert.Equal(t, AgentStatusError, agent.Status)
	assert.True(t, agent.HasError())

	// Error -> Online (recovery)
	agent.Status = AgentStatusOnline
	assert.Equal(t, AgentStatusOnline, agent.Status)
	assert.True(t, agent.IsOnline())
}

func TestAgent_K8sMetadata(t *testing.T) {
	k8sMetadata := &KubernetesMetadata{
		Namespace: "production",
		PodName:   "lumo-agent-xyz",
		NodeName:  "worker-01",
		Cluster:   "prod-cluster",
	}

	agent := &Agent{
		ID:                 uuid.New(),
		Name:               "k8s-agent",
		Platform:           AgentPlatformKubernetes,
		KubernetesMetadata: k8sMetadata,
		Status:             AgentStatusOnline,
	}

	assert.NotNil(t, agent.KubernetesMetadata)
	assert.Equal(t, "production", agent.KubernetesMetadata.Namespace)
	assert.Equal(t, "lumo-agent-xyz", agent.KubernetesMetadata.PodName)
	assert.Equal(t, "worker-01", agent.KubernetesMetadata.NodeName)
	assert.True(t, agent.IsKubernetes())
}

func TestAgent_Capabilities(t *testing.T) {
	agent := &Agent{
		ID:           uuid.New(),
		Name:         "test-agent",
		Capabilities: []string{"cpu", "memory", "disk", "network"},
		Status:       AgentStatusOnline,
	}

	assert.True(t, agent.HasCapability("cpu"))
	assert.True(t, agent.HasCapability("memory"))
	assert.False(t, agent.HasCapability("nonexistent"))
}
