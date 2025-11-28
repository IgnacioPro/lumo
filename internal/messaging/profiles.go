package messaging

import (
	"fmt"

	"github.com/ignacio/lumo/internal/messaging/providers"
)

// DeploymentProfile represents a pre-configured messaging setup
type DeploymentProfile string

const (
	// ProfileXS - Extra Small (0-10 agents)
	// Uses: Redis Streams (if Redis already exists)
	// Throughput: ~100 events/sec
	// Infrastructure: Zero additional components
	ProfileXS DeploymentProfile = "xs"

	// ProfileS - Small (10-50 agents)
	// Uses: NATS (single instance)
	// Throughput: 1,000+ events/sec
	// Infrastructure: 1 NATS server (64MB RAM)
	ProfileS DeploymentProfile = "s"

	// ProfileM - Medium (50-500 agents)
	// Uses: NATS cluster (3 nodes)
	// Throughput: 10,000+ events/sec
	// Infrastructure: NATS cluster (HA)
	ProfileM DeploymentProfile = "m"

	// ProfileXL - Extra Large (500+ agents, multi-cluster)
	// Uses: Kafka cluster with partitioning
	// Throughput: 100,000+ events/sec
	// Infrastructure: Kafka cluster (3-5 brokers + ZooKeeper/KRaft)
	ProfileXL DeploymentProfile = "xl"
)

// ProfileConfig returns recommended configuration for a deployment profile
func ProfileConfig(profile DeploymentProfile) (*providers.Config, error) {
	switch profile {
	case ProfileXS:
		return &providers.Config{
			Provider:        "redis",
			Brokers:         []string{"localhost:6379"},
			EventTopic:      "lumo.events",
			DeadLetterTopic: "lumo.events.dlq",
			MaxRetries:      3,
			ConsumerGroup:   "lumo-api-consumer",
			MaxInFlight:     5,
		}, nil

	case ProfileS:
		return &providers.Config{
			Provider:        "nats",
			Brokers:         []string{"nats://localhost:4222"},
			EventTopic:      "lumo.events",
			DeadLetterTopic: "lumo.events.dlq",
			MaxRetries:      3,
			ConsumerGroup:   "lumo-api-consumer",
			MaxInFlight:     10,
		}, nil

	case ProfileM:
		return &providers.Config{
			Provider: "nats",
			Brokers: []string{
				"nats://nats-0.nats:4222",
				"nats://nats-1.nats:4222",
				"nats://nats-2.nats:4222",
			},
			EventTopic:      "lumo.events",
			DeadLetterTopic: "lumo.events.dlq",
			MaxRetries:      3,
			ConsumerGroup:   "lumo-api-consumer",
			MaxInFlight:     50,
		}, nil

	case ProfileXL:
		return &providers.Config{
			Provider: "kafka",
			Brokers: []string{
				"kafka-0.kafka:9092",
				"kafka-1.kafka:9092",
				"kafka-2.kafka:9092",
			},
			EventTopic:      "lumo.events",
			DeadLetterTopic: "lumo.events.dlq",
			MaxRetries:      3,
			ConsumerGroup:   "lumo-api-consumer",
			MaxInFlight:     100,
		}, nil

	default:
		return nil, fmt.Errorf("unknown deployment profile: %s", profile)
	}
}

// ProfileRecommendation suggests a profile based on agent count
func ProfileRecommendation(agentCount int) DeploymentProfile {
	switch {
	case agentCount <= 10:
		return ProfileXS
	case agentCount <= 50:
		return ProfileS
	case agentCount <= 500:
		return ProfileM
	default:
		return ProfileXL
	}
}

// ProfileDescription returns human-readable description of a profile
func ProfileDescription(profile DeploymentProfile) string {
	switch profile {
	case ProfileXS:
		return "XS (0-10 agents): Redis Streams, ~100 events/sec, zero infra"
	case ProfileS:
		return "S (10-50 agents): NATS single node, 1,000+ events/sec, 64MB RAM"
	case ProfileM:
		return "M (50-500 agents): NATS cluster, 10,000+ events/sec, HA"
	case ProfileXL:
		return "XL (500+ agents): Kafka cluster, 100,000+ events/sec, multi-cluster"
	default:
		return "Unknown profile"
	}
}

// ProfileResourceRequirements returns estimated resource needs
type ResourceRequirements struct {
	MinMemoryMB int
	MinCPUCores float64
	MinNodes    int
	Storage     string
}

func ProfileResources(profile DeploymentProfile) ResourceRequirements {
	switch profile {
	case ProfileXS:
		return ResourceRequirements{
			MinMemoryMB: 0,   // Uses existing Redis
			MinCPUCores: 0,   // No additional CPU
			MinNodes:    0,   // No additional nodes
			Storage:     "0", // In-memory only
		}
	case ProfileS:
		return ResourceRequirements{
			MinMemoryMB: 64,
			MinCPUCores: 0.25,
			MinNodes:    1,
			Storage:     "1GB",
		}
	case ProfileM:
		return ResourceRequirements{
			MinMemoryMB: 512,
			MinCPUCores: 1.0,
			MinNodes:    3,
			Storage:     "10GB per node",
		}
	case ProfileXL:
		return ResourceRequirements{
			MinMemoryMB: 2048,
			MinCPUCores: 2.0,
			MinNodes:    5,
			Storage:     "100GB per broker",
		}
	default:
		return ResourceRequirements{}
	}
}
