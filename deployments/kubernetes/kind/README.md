# Testing Lumo Full Stack with kind (Kubernetes in Docker)

This directory contains everything you need to test the **complete Lumo stack** locally using [kind](https://kind.sigs.k8s.io/).

## Quick Start

### One-Command Full Stack Deployment

Run the complete test suite (creates cluster, builds images, deploys DB + API + Agents, runs integration tests):

```bash
./deploy-lumo.sh
```

**This deploys:**
- ✅ PostgreSQL (Database)
- ✅ Lumo API Server (REST API)
- ✅ Lumo Agents (DaemonSet + Deployment)
- ✅ Runs 10 automated tests

### Step-by-Step Setup

If you prefer to run each step manually:

```bash
# 1. Create kind cluster (3 nodes: 1 control-plane + 2 workers)
./setup-kind-cluster.sh

# 2. Build Docker images (API + Agent) and load into kind
./build-and-load.sh

# 3. Deploy PostgreSQL
kubectl apply -f manifests/postgres.yaml

# 4. Deploy API Server
kubectl apply -f manifests/api-server.yaml

# 5. Deploy agents to kind cluster
./deploy-to-kind.sh
```

## Prerequisites

The scripts will automatically install missing prerequisites, but you can install them manually:

- **Docker**: [Install Docker](https://docs.docker.com/get-docker/)
- **kind**: [Install kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- **kubectl**: [Install kubectl](https://kubernetes.io/docs/tasks/tools/)

## Files

| File | Purpose |
|------|---------|
| `deploy-lumo.sh` | **PRIMARY**: Complete full-stack deployment + tests |
| `test-failure-scenarios.sh` | Event-driven agent failure scenario tests |
| `setup-kind-cluster.sh` | Creates a 3-node kind cluster |
| `build-and-load.sh` | Builds Docker images (API + Agent) and loads into kind |
| `deploy-to-kind.sh` | Deploys agents to kind cluster |
| `test-workflow.sh` | Legacy full-stack deployment (use `deploy-lumo.sh` instead) |
| `manifests/postgres.yaml` | PostgreSQL deployment |
| `manifests/api-server.yaml` | Lumo API Server deployment |
| `FULL_STACK_DEPLOYMENT.md` | Detailed documentation |
| `README.md` | This file |

## Detailed Usage

### 1. setup-kind-cluster.sh

Creates a kind cluster with:
- 1 control-plane node
- 2 worker nodes
- Kubernetes v1.28.0 (configurable)
- Nodes labeled with `lumo.io/monitor=true`

**Options:**
```bash
# Use custom cluster name
export KIND_CLUSTER_NAME=my-test-cluster
./setup-kind-cluster.sh

# Use different Kubernetes version
export K8S_VERSION=v1.27.0
./setup-kind-cluster.sh
```

**What it does:**
1. Checks prerequisites (Docker, kind, kubectl)
2. Installs missing tools automatically
3. Creates kind cluster configuration
4. Creates cluster with 3 nodes
5. Waits for nodes to be ready
6. Displays cluster info

### 2. build-and-load.sh

Builds the lumo-agent Docker image and loads it into kind.

**Options:**
```bash
# Use custom image name/tag
export LUMO_IMAGE_NAME=my-lumo-agent
export LUMO_IMAGE_TAG=dev
./build-and-load.sh

# Use custom cluster name
export KIND_CLUSTER_NAME=my-test-cluster
./build-and-load.sh
```

**What it does:**
1. Builds Docker image using `Dockerfile`
2. Loads image into kind cluster nodes
3. Verifies image availability
4. Shows image details

**Image details:**
- Base image: `alpine:3.19`
- Binary: Built from source with optimizations
- Size: ~60-80 MB
- Ports: 8080 (health), 9090 (metrics)

### 3. deploy-to-kind.sh

Deploys lumo-agent to the kind cluster.

**Options:**
```bash
# Basic deployment
./deploy-to-kind.sh

# With API endpoint and token
export LUMO_API_ENDPOINT=http://my-api:8080
export LUMO_AGENT_TOKEN=my-token
./deploy-to-kind.sh

# With AI provider
export LUMO_AI_PROVIDER=anthropic
export LUMO_AI_API_KEY=sk-ant-...
./deploy-to-kind.sh

# Custom namespace
export LUMO_NAMESPACE=my-namespace
./deploy-to-kind.sh
```

**What it does:**
1. Creates namespace (`lumo-system` by default)
2. Creates secrets (agent token, AI API keys)
3. Updates manifests with local image reference
4. Deploys RBAC, ConfigMap, Services, DaemonSet, Deployment
5. Waits for pods to be ready
6. Shows pod status and recent events

**Deployed components:**
- **DaemonSet** (`lumo-agent-node`): Runs on every node
- **Deployment** (`lumo-agent-cluster`): 2 replicas for cluster monitoring
- **Services**: Health/metrics endpoints
- **RBAC**: ServiceAccount with read-only permissions

### 4. deploy-lumo.sh

Complete end-to-end test suite.

**Options:**
```bash
# Full test
./deploy-lumo.sh

# Skip cluster creation (use existing)
./deploy-lumo.sh --skip-cluster

# Skip image build (use existing)
./deploy-lumo.sh --skip-build

# Skip deployment (test existing)
./deploy-lumo.sh --skip-deploy

# Combine options
./deploy-lumo.sh --skip-cluster --skip-build

# Show help
./deploy-lumo.sh --help
```

**Test cases:**
1. ✓ Pod status check
2. ✓ Health endpoints (`/health`, `/ready`)
3. ✓ Metrics endpoint (`/metrics`)
4. ✓ Log analysis (checks for errors)
5. ✓ RBAC permissions verification
6. ✓ DaemonSet scheduling verification

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `KIND_CLUSTER_NAME` | `lumo-test` | Name of kind cluster |
| `K8S_VERSION` | `v1.28.0` | Kubernetes version |
| `LUMO_NAMESPACE` | `lumo-system` | Kubernetes namespace |
| `LUMO_IMAGE_NAME` | `lumo-agent` | Docker image name |
| `LUMO_IMAGE_TAG` | `local` | Docker image tag |
| `LUMO_API_ENDPOINT` | `http://lumo-api...` | Lumo API endpoint |
| `LUMO_AGENT_TOKEN` | `test-token-for-kind` | Agent authentication token |
| `LUMO_AI_PROVIDER` | _(empty)_ | AI provider (anthropic, openai, etc.) |
| `LUMO_AI_API_KEY` | _(empty)_ | AI API key |

### Agent Configuration

The agent is configured via ConfigMap (`lumo-agent-config`). Key settings:

```yaml
agent:
  mode: hybrid                  # scheduled|on-demand|continuous|hybrid
  schedule: "*/5 * * * *"       # Run every 5 minutes
  enabled_checks: "cpu,memory,disk,process,service,network,kubernetes"
  report_format: toon           # Use TOON format for efficiency
  offline_mode: true            # Continue if API unavailable
  health_check_port: 8080
  metrics_port: 9090
```

## Verifying the Deployment

### Check Pod Status

```bash
# All pods
kubectl get pods -n lumo-system

# DaemonSet pods (one per node)
kubectl get pods -n lumo-system -l app.kubernetes.io/component=node-monitor

# Deployment pods (cluster monitoring)
kubectl get pods -n lumo-system -l app.kubernetes.io/component=cluster-monitor
```

### View Logs

```bash
# All agent logs
kubectl logs -n lumo-system -l app.kubernetes.io/name=lumo-agent -f

# DaemonSet logs only
kubectl logs -n lumo-system -l app.kubernetes.io/component=node-monitor -f

# Deployment logs only
kubectl logs -n lumo-system -l app.kubernetes.io/component=cluster-monitor -f

# Specific pod
kubectl logs -n lumo-system <pod-name> -f
```

### Test Endpoints

```bash
# Health endpoint
kubectl port-forward -n lumo-system svc/lumo-agent-cluster 8080:8080
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/live

# Metrics endpoint
kubectl port-forward -n lumo-system svc/lumo-agent-cluster 9090:9090
curl http://localhost:9090/metrics | grep lumo_agent
```

### Exec into Pod

```bash
# Get a shell in a pod
kubectl exec -it -n lumo-system <pod-name> -- /bin/sh

# Run agent health command
kubectl exec -n lumo-system <pod-name> -- /usr/local/bin/lumo-agent health

# Check agent version
kubectl exec -n lumo-system <pod-name> -- /usr/local/bin/lumo-agent version
```

## Troubleshooting

### Pods in CrashLoopBackOff

```bash
# Check logs from previous crash
kubectl logs -n lumo-system <pod-name> --previous

# Describe pod for events
kubectl describe pod -n lumo-system <pod-name>

# Check events
kubectl get events -n lumo-system --sort-by='.lastTimestamp'
```

**Common causes:**
- Missing or invalid agent token
- Invalid API endpoint
- Resource limits too low
- RBAC permission issues

### Image Pull Errors

```bash
# Verify image is in kind
docker exec lumo-test-control-plane crictl images | grep lumo-agent

# Reload image if missing
./build-and-load.sh
```

### RBAC Permission Errors

```bash
# Check RBAC resources
kubectl get clusterrole lumo-agent-reader
kubectl get clusterrolebinding lumo-agent-reader
kubectl describe clusterrole lumo-agent-reader

# Test permissions
kubectl auth can-i list nodes --as=system:serviceaccount:lumo-system:lumo-agent
kubectl auth can-i list pods --as=system:serviceaccount:lumo-system:lumo-agent
```

### Network Issues

```bash
# Test DNS resolution
kubectl exec -n lumo-system <pod-name> -- nslookup kubernetes.default

# Test external connectivity
kubectl exec -n lumo-system <pod-name> -- wget -O- https://api.anthropic.com 2>&1 | head

# Check NetworkPolicy
kubectl get networkpolicy -n lumo-system
kubectl describe networkpolicy -n lumo-system lumo-agent-node
```

### Resource Issues

```bash
# Check resource usage
kubectl top nodes
kubectl top pods -n lumo-system

# Check resource limits
kubectl describe pod -n lumo-system <pod-name> | grep -A 5 "Limits:"
```

## Advanced Usage

### Multiple Clusters

Run multiple test clusters simultaneously:

```bash
# Cluster 1
KIND_CLUSTER_NAME=lumo-test-1 ./setup-kind-cluster.sh
KIND_CLUSTER_NAME=lumo-test-1 ./build-and-load.sh
KIND_CLUSTER_NAME=lumo-test-1 ./deploy-to-kind.sh

# Cluster 2
KIND_CLUSTER_NAME=lumo-test-2 ./setup-kind-cluster.sh
KIND_CLUSTER_NAME=lumo-test-2 ./build-and-load.sh
KIND_CLUSTER_NAME=lumo-test-2 ./deploy-to-kind.sh

# Switch between clusters
kubectl config use-context kind-lumo-test-1
kubectl config use-context kind-lumo-test-2
```

### Custom Node Count

Edit `setup-kind-cluster.sh` and add more worker nodes:

```yaml
nodes:
  - role: control-plane
  - role: worker
  - role: worker
  - role: worker  # Add more workers
  - role: worker
```

### With Actual API Server

Deploy the Lumo API server in the same cluster:

```bash
# 1. Start PostgreSQL and Redis in kind
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-config
data:
  POSTGRES_DB: lumo
  POSTGRES_USER: lumo
  POSTGRES_PASSWORD: lumo
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
spec:
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:16
        envFrom:
        - configMapRef:
            name: postgres-config
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
EOF

# 2. Build and deploy API server (similar process)

# 3. Deploy agent with API endpoint
export LUMO_API_ENDPOINT=http://lumo-api.lumo-system.svc.cluster.local:8080
./deploy-to-kind.sh
```

### Testing Different Configurations

Test different agent modes:

```bash
# Scheduled mode only
kubectl patch configmap -n lumo-system lumo-agent-config \
  --type merge \
  -p '{"data":{"agent.mode":"scheduled"}}'
kubectl rollout restart daemonset/lumo-agent-node -n lumo-system

# Continuous mode
kubectl patch configmap -n lumo-system lumo-agent-config \
  --type merge \
  -p '{"data":{"agent.mode":"continuous"}}'
kubectl rollout restart daemonset/lumo-agent-node -n lumo-system
```

## Cleanup

### Delete Agent Resources

```bash
# Delete namespace (removes all resources)
kubectl delete namespace lumo-system

# Or delete individual resources
kubectl delete daemonset/lumo-agent-node -n lumo-system
kubectl delete deployment/lumo-agent-cluster -n lumo-system
kubectl delete clusterrolebinding/lumo-agent-reader
kubectl delete clusterrole/lumo-agent-reader
```

### Delete kind Cluster

```bash
# Delete specific cluster
kind delete cluster --name lumo-test

# Delete all kind clusters
kind delete clusters --all
```

### Clean Docker Images

```bash
# Remove lumo-agent image
docker rmi lumo-agent:local

# Remove kind node images
docker rmi kindest/node:v1.28.0
```

## Performance Notes

### Build Times

- Initial build: ~2-5 minutes (depending on hardware)
- Incremental builds: ~30-60 seconds

### Cluster Creation

- Cluster creation: ~2-3 minutes
- Image loading: ~10-20 seconds
- Pod startup: ~30-60 seconds

### Resource Usage

**Per-node (DaemonSet):**
- CPU: 100m (request), 500m (limit)
- Memory: 128Mi (request), 256Mi (limit)

**Cluster monitoring (Deployment):**
- CPU: 50m (request), 200m (limit)
- Memory: 64Mi (request), 128Mi (limit)

**Total for 3-node cluster:**
- ~5 pods (3 DaemonSet + 2 Deployment)
- ~400m CPU, ~640Mi memory (requests)

## Tips and Best Practices

1. **Use `--skip-*` flags** to speed up iterations during development
2. **Check logs frequently** - they contain valuable debugging info
3. **Use port-forwarding** to test endpoints without exposing services
4. **Label nodes** to test DaemonSet node selection
5. **Monitor resource usage** with `kubectl top`
6. **Use debug mode** for verbose logging when troubleshooting
7. **Keep clusters small** - 3 nodes is enough for most testing

## Next Steps

After successful local testing:

1. **Test with actual API server** - Deploy full stack in kind
2. **Load testing** - Generate high diagnostic load
3. **Failure testing** - Kill pods, nodes, network
4. **Upgrade testing** - Test rolling updates
5. **Security testing** - Test RBAC, NetworkPolicy
6. **Deploy to real cluster** - Use production deployment guide

## Failure Scenario Testing

The `test-failure-scenarios.sh` script validates that the event-driven agent correctly detects and reports Kubernetes failures.

### Event Flow

```
K8s Failure → Informer → Watcher → Debouncer (45s) → API Processor → POST /api/v1/events → Database
```

1. **Kubernetes Failure** - A pod crashes, OOMs, fails to pull image, etc.
2. **Informer** - SharedInformerFactory receives the update from K8s API
3. **Watcher** - Specialized watcher (Pod, Workload, Volume, Node) detects the failure condition
4. **Debouncer** - Waits 45s to filter transient issues (configurable, max 3min for continuous events)
5. **API Processor** - Submits event to API server via HTTP POST
6. **Database** - Event stored for analysis and notifications

### Running Tests

```bash
# Run all failure scenarios (fast mode - polls for events)
./test-failure-scenarios.sh

# Run specific scenario
./test-failure-scenarios.sh --scenario oom-killed

# List available scenarios
./test-failure-scenarios.sh --list

# Disable fast mode (use fixed 180s waits)
./test-failure-scenarios.sh --slow
```

### Test Scenarios

| Scenario | Event Type | Trigger |
|----------|------------|---------|
| `image-pull-backoff` | `image-pull-backoff` | Invalid image name |
| `crash-loop-backoff` | `crash-loop-backoff` | Container exits immediately |
| `oom-killed` | `oom-killed` | Memory limit exceeded |
| `deployment-failed` | `deployment-failed` | Progress deadline exceeded |
| `job-failed` | `job-failed` | Backoff limit exceeded |
| `pvc-provision-failed` | `pvc-provision-failed` | Invalid storage class |
| `scheduling-failed` | `scheduling-failed` | Node selector mismatch |

### Fast Mode

By default, tests use **fast mode** which polls the database every 5 seconds after the debounce window instead of waiting a fixed 180 seconds. This reduces test time by ~60-70%.

```bash
# Environment variables
FAST_MODE=true          # Enable polling (default)
POLL_INTERVAL=5         # Seconds between polls
POLL_MAX_ATTEMPTS=60    # Max attempts (5min timeout)

# Disable fast mode
FAST_MODE=false ./test-failure-scenarios.sh
```

## Resources

- [kind Documentation](https://kind.sigs.k8s.io/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Lumo Documentation](../../README.md)
- [Deployment Guide](../README.md)

## Support

- GitHub Issues: https://github.com/ignacio/lumo/issues
- Discussions: https://github.com/ignacio/lumo/discussions
