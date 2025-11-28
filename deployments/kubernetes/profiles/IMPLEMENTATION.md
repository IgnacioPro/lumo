# Deployment Profiles - Implementation Guide

## How It Works in Practice

### 1. CLI Flag (Recommended)

```bash
# Deploy with small-business profile (default)
./deployments/kubernetes/kind/deploy-lumo.sh --profile small-business

# Deploy with startup profile
./deployments/kubernetes/kind/deploy-lumo.sh --profile startup

# Deploy with enterprise profile
./deployments/kubernetes/kind/deploy-lumo.sh --profile enterprise
```

### 2. Environment Variable

```bash
export MESSAGING_PROFILE=small-business
./deployments/kubernetes/kind/deploy-lumo.sh
```

### 3. In Code (for programmatic deployment)

```go
import "github.com/ignacio/lumo/internal/messaging"

// Get recommended profile based on agent count
profile := messaging.ProfileRecommendation(agentCount)

// Get configuration for that profile
config, _ := messaging.ProfileConfig(profile)

// Use the config
provider, _ := messaging.NewProvider(config)
publisher, _ := provider.NewPublisher(config)
```

## What the Script Does

When you run `./deploy-lumo.sh --profile small-business`, the script:

1. **Validates the profile** (startup|small-business|enterprise|hyperscale)
2. **Deploys the right infrastructure:**
   - `startup` → No additional infrastructure (uses existing Redis)
   - `small-business` → Deploys NATS single server
   - `enterprise` → Deploys NATS 3-node StatefulSet
   - `hyperscale` → Deploys Kafka cluster + ZooKeeper

3. **Configures the agent ConfigMap:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lumo-agent-config
data:
  messaging.enabled: "true"
  messaging.provider: "nats"  # Changes based on profile
  messaging.brokers: "nats.lumo-system:4222"  # Changes based on profile
  messaging.event_topic: "lumo.events"
  messaging.consumer_group: "lumo-agent"
```

4. **Agents read the ConfigMap** and automatically use the right provider

## Migration Between Profiles

### Startup → Small Business (Add NATS)

```bash
# Currently running startup profile
# Want to upgrade to NATS

./deploy-lumo.sh --profile small-business --skip-cluster --skip-build
```

**What happens:**
1. NATS server deployed (new)
2. Agent ConfigMap updated (messaging.provider: redis → nats)
3. Agents rolling restart
4. Old Redis still works during transition (zero downtime)

### Small Business → Enterprise (Add HA)

```bash
# Currently running single NATS
# Want 3-node cluster for HA

./deploy-lumo.sh --profile enterprise --skip-cluster --skip-build
```

**What happens:**
1. NATS scaled from 1 → 3 replicas
2. ConfigMap updated with 3 broker addresses
3. Agents reconnect to cluster automatically

### Enterprise → Hyperscale (Switch to Kafka)

```bash
# Currently running NATS cluster
# Need extreme throughput

./deploy-lumo.sh --profile hyperscale --skip-cluster --skip-build
```

**What happens:**
1. Kafka cluster deployed (parallel to NATS)
2. ConfigMap updated (provider: nats → kafka)
3. Agents rolling restart to use Kafka
4. NATS can be deleted manually after verification

## Configuration Files

### Startup Profile
**File:** `deployments/kubernetes/profiles/startup/configmap.yaml`
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lumo-agent-config
data:
  messaging.enabled: "true"
  messaging.provider: "redis"
  messaging.brokers: "lumo-redis.lumo-system:6379"
```

### Small Business Profile
**File:** `deployments/kubernetes/profiles/small-business/nats.yaml`
```yaml
apiVersion: v1
kind: Service
metadata:
  name: nats
spec:
  ports:
  - port: 4222
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nats
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: nats
        image: nats:2.10-alpine
        args: ["-js"]  # JetStream enabled
        resources:
          requests:
            memory: "64Mi"
            cpu: "250m"
```

### Enterprise Profile
**File:** `deployments/kubernetes/profiles/enterprise/nats-cluster.yaml`
```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: nats
spec:
  serviceName: nats
  replicas: 3  # HA cluster
  template:
    spec:
      containers:
      - name: nats
        image: nats:2.10-alpine
        args:
          - "-js"
          - "-cluster"
          - "nats://0.0.0.0:6222"
```

## Current Implementation Status

**✅ Implemented:**
- Profile selection logic (`internal/messaging/profiles.go`)
- Profile recommendation based on agent count
- Profile configuration generation
- Profile tests (21 test cases)

**🔄 To Be Implemented (Week 3):**
- Actual manifest files per profile (YAML files)
- Script logic to deploy based on `$MESSAGING_PROFILE`
- Integration into `deploy-lumo.sh`
- Upgrade/migration scripts

## Next Steps (Integration Work)

### 1. Create Profile Manifests

```bash
# Create directory structure
deployments/kubernetes/profiles/
├── startup/
│   └── configmap.yaml         # Redis config only
├── small-business/
│   ├── nats.yaml               # Single NATS server
│   └── configmap.yaml
├── enterprise/
│   ├── nats-cluster.yaml       # 3-node StatefulSet
│   └── configmap.yaml
└── hyperscale/
    ├── kafka-cluster.yaml      # Kafka + ZooKeeper
    └── configmap.yaml
```

### 2. Update `deploy-lumo.sh`

Add section after infrastructure deployment:

```bash
deploy_messaging() {
    log_info "Step 3.5/7: Deploying messaging infrastructure for profile: $MESSAGING_PROFILE"
    
    case "$MESSAGING_PROFILE" in
        startup)
            log_info "Using existing Redis, no additional deployment needed"
            ;;
        small-business)
            kubectl apply -f ../profiles/small-business/nats.yaml
            ;;
        enterprise)
            kubectl apply -f ../profiles/enterprise/nats-cluster.yaml
            ;;
        hyperscale)
            kubectl apply -f ../profiles/hyperscale/kafka-cluster.yaml
            ;;
    esac
}
```

### 3. Update Agent ConfigMap Generation

Current agent ConfigMap would be updated to include:

```yaml
messaging.enabled: "{{ .Values.messaging.enabled }}"
messaging.provider: "{{ .Values.messaging.provider }}"
messaging.brokers: "{{ .Values.messaging.brokers }}"
```

## Summary

**Right now:** Profiles work in Go code, flag is added to script, documentation written

**Next:** Create actual YAML manifests and wire into deploy script

**Benefit:** You can say `./deploy-lumo.sh --profile startup` and get zero-config messaging, or `--profile enterprise` and get full HA cluster, all automatic!
