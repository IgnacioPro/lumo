#!/usr/bin/env bash
#
# Example: How to use deployment profiles in practice
#

set -euo pipefail

# Example 1: Startup profile (minimal, uses existing Redis)
./deploy-lumo.sh --profile startup

# This will:
# - Skip deploying NATS/Kafka/RabbitMQ
# - Configure agents to use Redis Streams
# - Set messaging.provider=redis in ConfigMap
# - Set messaging.brokers=lumo-redis.lumo-system:6379

# Example 2: Small Business profile (default, recommended)
./deploy-lumo.sh --profile small-business

# This will:
# - Deploy single NATS server (64MB RAM)
# - Configure agents to use NATS
# - Set messaging.provider=nats
# - Set messaging.brokers=nats.lumo-system:4222

# Example 3: Enterprise profile (HA cluster)
./deploy-lumo.sh --profile enterprise

# This will:
# - Deploy 3-node NATS cluster (StatefulSet)
# - Configure agents to use NATS cluster
# - Set messaging.provider=nats
# - Set messaging.brokers=nats-0:4222,nats-1:4222,nats-2:4222

# Example 4: Hyperscale profile (Kafka cluster)
./deploy-lumo.sh --profile hyperscale

# This will:
# - Deploy 3-broker Kafka cluster + ZooKeeper
# - Configure agents to use Kafka
# - Set messaging.provider=kafka
# - Set messaging.brokers=kafka-0:9092,kafka-1:9092,kafka-2:9092

# Example 5: Environment variable (alternative to flag)
export MESSAGING_PROFILE=small-business
./deploy-lumo.sh

# Example 6: Upgrade path (startup → small-business)
# Currently running startup profile, want to upgrade:
./deploy-lumo.sh --profile small-business --skip-cluster --skip-build

# This will:
# - Keep existing cluster
# - Deploy NATS server
# - Update agent ConfigMap to use NATS
# - Rolling restart agents (zero downtime with Redis still there)

echo "✓ All examples shown"
echo ""
echo "See deployments/kubernetes/profiles/README.md for full documentation"
