# Example 5: Agent Deployment on Kubernetes

This example demonstrates deploying Lumo agents to Kubernetes clusters for **real-time event-driven monitoring**.

## What You'll Learn

- Deploy event-driven agents (recommended for Kubernetes)
- Configure real-time Kubernetes event monitoring
- Set up RBAC permissions
- Configure API server for agent reporting
- Monitor agent health and metrics
- Query events with `lumo events` command

## Prerequisites

- Kubernetes cluster (1.25+)
- kubectl configured
- Lumo API server running (or use `lumo serve`)
- Redis (required for event-driven mode)
- JWT token for agent authentication

## Architecture Overview

Lumo uses a **centralized intelligence architecture** for Kubernetes:

```
┌─────────────────────────────────────────────────────────────┐
│                   Kubernetes Cluster                         │
│                                                               │
│   ┌─────────────────────────────────────────────────────┐   │
│   │         Event-Driven Agent (Deployment)              │   │
│   │               2+ replicas for HA                     │   │
│   │                                                       │   │
│   │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │   │
│   │  │ Pod Watcher │  │  Workload   │  │   Volume    │  │   │
│   │  │             │  │   Watcher   │  │   Watcher   │  │   │
│   │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  │   │
│   │         │                 │                 │         │   │
│   │         └────────────────┬┴─────────────────┘         │   │
│   │                          │                             │   │
│   │                  ┌───────▼───────┐                    │   │
│   │                  │   Debouncer   │ ← 45s window       │   │
│   │                  │   (Redis)     │                    │   │
│   │                  └───────┬───────┘                    │   │
│   └──────────────────────────┼────────────────────────────┘   │
│                              │                                 │
└──────────────────────────────┼─────────────────────────────────┘
                               │ HTTP POST /api/v1/events
                      ┌────────▼─────────┐
                      │   Lumo API       │
                      │   Server         │
                      │                  │
                      │  • AI Analysis   │
                      │  • Notifications │
                      │  • Storage       │
                      └──────────────────┘
```

**Key Benefits:**
- **Real-time detection**: <60s latency (vs 5-minute polling)
- **90%+ API load reduction**: Watch streams vs periodic List() calls
- **Intelligent debouncing**: 45s window filters transient failures
- **Centralized AI**: All analysis done by API server (agents are lightweight)

## Quick Start

### 1. Deploy Full Stack with Kind (Recommended for Testing)

The easiest way to get started is with our automated deployment script:

```bash
cd deployments/kubernetes/kind
./deploy-lumo.sh
```

This deploys:
- PostgreSQL database
- Redis cache
- Lumo API server
- Event-driven agents (2 replicas)
- RBAC and secrets

### 2. Manual Deployment

#### Start API Server

```bash
# Local development
docker-compose up -d  # PostgreSQL + Redis
lumo serve --port 8080

# Or use existing API server
export LUMO_API_ENDPOINT=https://lumo-api.example.com
```

#### Deploy to Kubernetes

```bash
# Create namespace
kubectl create namespace lumo-system

# Apply RBAC
kubectl apply -f deployments/kubernetes/base/rbac.yaml

# Create secrets (update with your values)
kubectl create secret generic lumo-agent-secrets \
  --from-literal=jwt-token=$LUMO_AGENT_TOKEN \
  -n lumo-system

# Optional: AI provider for event analysis
kubectl create secret generic lumo-ai-secrets \
  --from-literal=anthropic-api-key=$ANTHROPIC_API_KEY \
  -n lumo-system

# Deploy Redis (required for event-driven mode)
kubectl apply -f deployments/kubernetes/redis/

# Deploy ConfigMap and Agent
kubectl apply -f deployments/kubernetes/base/configmap-agent.yaml
kubectl apply -f deployments/kubernetes/base/deployment-agent.yaml
```

### 3. Verify Deployment

```bash
# Check agent pods (should be 2 replicas)
kubectl get pods -n lumo-system -l app=lumo-agent

# Check logs for event processing
kubectl logs -n lumo-system -l app=lumo-agent -f

# Verify agent registration with API
curl http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer $LUMO_API_TOKEN"

# Query events
lumo events --limit 10
```

## Event-Driven Architecture

### How It Works

1. **Kubernetes Informers** watch for changes (no polling)
2. **Specialized Watchers** detect failures:
   - **PodWatcher**: ImagePullBackOff, CrashLoopBackOff, OOMKilled
   - **WorkloadWatcher**: Deployment failures, Job failures
   - **VolumeWatcher**: PVC provisioning failures, mount issues
   - **NodeWatcher**: Node NotReady, memory/disk pressure
3. **Debouncer** waits 45 seconds to filter transient issues
4. **API Processor** submits events to API server
5. **API Server** performs AI analysis and sends notifications

### Event Types & Severity

| Event Type | Trigger | Severity |
|------------|---------|----------|
| `oom-killed` | Container OOMKilled | Critical |
| `pod-evicted` | Pod evicted from node | Critical |
| `node-not-ready` | Node becomes NotReady | Critical |
| `job-failed` | BackoffLimitExceeded | Critical |
| `image-pull-backoff` | Image pull failures | High |
| `crash-loop-backoff` | Container crash loop | High |
| `deployment-failed` | ProgressDeadlineExceeded | High |
| `volume-failed-mount` | FailedMount event | High |
| `pvc-provision-failed` | Provisioning failed | Medium |
| `pod-pending` | Pending >5 minutes | Medium |

### Configuration

The ConfigMap controls event-driven behavior:

```yaml
# Key settings in configmap-agent.yaml
agent.mode: "event-driven"
agent.event-driven.enabled: "true"
agent.event-driven.debounce-window: "45s"      # Wait before processing
agent.event-driven.max-debounce-window: "3m"   # Prevents infinite debouncing
agent.event-driven.resync-period: "0s"         # Pure event-driven (no polling)
agent.event-driven.group-related-events: "true"
agent.event-driven.max-events-per-min: "100"   # Rate limiting
agent.event-driven.min-severity: "low"         # Process all severities

# Enable/disable specific watchers
agent.event-driven.watch-pod-events: "true"
agent.event-driven.watch-workloads: "true"
agent.event-driven.watch-volumes: "true"
agent.event-driven.watch-nodes: "true"
```

### Monitoring Event-Driven Agents

```bash
# Check agent status
kubectl get pods -n lumo-system -l mode=event-driven

# Follow event processing logs
kubectl logs -n lumo-system -l app=lumo-agent -f --tail=100

# Check Prometheus metrics
kubectl port-forward -n lumo-system svc/lumo-agent 9090:9090
curl http://localhost:9090/metrics | grep lumo_

# Key metrics:
# - lumo_events_processed_total
# - lumo_event_processing_duration_seconds
# - lumo_ai_analysis_total
# - lumo_notifications_sent_total
```

## Helm Deployment (Recommended for Production)

### Install Helm Chart

```bash
# Install with event-driven mode (default)
helm install lumo-agent deployments/kubernetes/helm/lumo-agent \
  --namespace lumo-system \
  --create-namespace \
  --set apiEndpoint=http://lumo-api:8080 \
  --set agent.token=$LUMO_AGENT_TOKEN \
  --set redis.enabled=true
```

### Custom Values

```yaml
# custom-values.yaml
apiEndpoint: https://lumo-api.example.com

agent:
  token: your-jwt-token
  mode: event-driven
  replicas: 2

redis:
  enabled: true
  url: redis://lumo-redis:6379/0

eventDriven:
  enabled: true
  debounceWindow: 45s
  maxDebounceWindow: 3m
  groupRelatedEvents: true
  maxEventsPerMin: 100
  minSeverity: low
  watchers:
    pods: true
    workloads: true
    volumes: true
    nodes: true
    events: true

resources:
  limits:
    memory: 256Mi
    cpu: 200m
  requests:
    memory: 128Mi
    cpu: 100m

prometheus:
  enabled: true
  serviceMonitor:
    enabled: true
```

Install with custom values:

```bash
helm install lumo-agent deployments/kubernetes/helm/lumo-agent \
  --namespace lumo-system \
  -f custom-values.yaml
```

### Upgrade Deployment

```bash
# Update values
helm upgrade lumo-agent deployments/kubernetes/helm/lumo-agent \
  --namespace lumo-system \
  -f custom-values.yaml

# Rollback if needed
helm rollback lumo-agent
```

## RBAC Configuration

### Minimal Permissions (Read-Only)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: lumo-agent-reader
rules:
- apiGroups: [""]
  resources: ["nodes", "pods", "services", "persistentvolumes", "persistentvolumeclaims"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["batch"]
  resources: ["jobs", "cronjobs"]
  verbs: ["get", "list", "watch"]
```

### Extended Permissions (With Remediation)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: lumo-agent-full
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch", "delete"]  # Delete for pod restart
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets"]
  verbs: ["get", "list", "watch", "patch"]  # Patch for scaling
- apiGroups: [""]
  resources: ["pods/log"]
  verbs: ["get"]  # Read logs
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["create"]  # Execute commands in pods
```

## Monitoring and Observability

### Prometheus Integration

```bash
# ServiceMonitor for Prometheus Operator
kubectl apply -f - <<EOF
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: lumo-agent
spec:
  selector:
    matchLabels:
      app: lumo-agent
  endpoints:
  - port: metrics
    interval: 30s
EOF
```

### Grafana Dashboard

Import dashboard from `../../deployments/kubernetes/grafana/lumo-dashboard.json` (if available)

**Key Metrics:**
- `lumo_agent_health` - Agent health status
- `lumo_diagnostics_total` - Total diagnostics run
- `lumo_diagnostics_duration_seconds` - Diagnostic execution time
- `lumo_api_requests_total` - API calls made
- `lumo_cache_hits_total` - Cache hit rate

### Logging

```bash
# All agents
kubectl logs -l app=lumo-agent --tail=100 -f

# Specific node
kubectl logs -l app=lumo-agent --node worker-01 -f

# Export to file
kubectl logs -l app=lumo-agent --since=1h > agent-logs.txt

# Integration with Loki/Elasticsearch
# (Configure via logging sidecar or daemonset)
```

## Advanced Configuration

### Multi-Cluster Setup

```bash
# Cluster 1
kubectl config use-context cluster-1
helm install lumo-agent-us-east deployments/kubernetes/helm/lumo-agent \
  --namespace lumo-system \
  --set cluster.name=us-east \
  --set apiEndpoint=https://lumo-api-central.example.com

# Cluster 2
kubectl config use-context cluster-2
helm install lumo-agent-us-west deployments/kubernetes/helm/lumo-agent \
  --namespace lumo-system \
  --set cluster.name=us-west \
  --set apiEndpoint=https://lumo-api-central.example.com
```

### Pod Affinity (HA Spread)

The deployment already includes anti-affinity to spread replicas across nodes:

```yaml
# Included in deployment-agent.yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app: lumo-agent
              mode: event-driven
          topologyKey: kubernetes.io/hostname
```

### Resource Limits

```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "256Mi"
    cpu: "200m"
```

## Troubleshooting

### Agents Not Registering

```bash
# Check agent logs
kubectl logs -n lumo-system -l app=lumo-agent

# Verify API endpoint reachability
kubectl run test -n lumo-system --rm -it --image=curlimages/curl -- \
  curl -v http://lumo-api:8080/api/v1/health

# Check secrets
kubectl get secret -n lumo-system lumo-agent-secrets -o yaml

# Verify RBAC
kubectl auth can-i get pods --as=system:serviceaccount:lumo-system:lumo-agent
kubectl auth can-i watch pods --as=system:serviceaccount:lumo-system:lumo-agent
```

### Events Not Being Processed

```bash
# Check Redis connection
kubectl logs -n lumo-system -l app=lumo-agent | grep -i redis

# Verify Redis is running
kubectl get pods -n lumo-system -l app=lumo-redis

# Check debounce window (events are held for 45s by default)
# Transient failures that self-heal won't be processed

# Check min severity setting
kubectl get configmap -n lumo-system lumo-agent-config -o yaml | grep min-severity
```

### High Resource Usage

```bash
# Check resource usage
kubectl top pods -n lumo-system -l app=lumo-agent

# Reduce event rate (adjust max-events-per-min)
kubectl edit configmap -n lumo-system lumo-agent-config

# Increase debounce window to reduce processing
# agent.event-driven.debounce-window: "60s"
```

### Networking Issues

```bash
# Test from agent pod
kubectl exec -n lumo-system -it deploy/lumo-agent -- sh
wget -O- http://lumo-api:8080/api/v1/health

# Check NetworkPolicy
kubectl get networkpolicy -n lumo-system
kubectl describe networkpolicy -n lumo-system lumo-agent

# Check DNS
kubectl exec -n lumo-system -it deploy/lumo-agent -- nslookup lumo-api
```

## Testing with Chaos Engineering

Trigger test failures to verify event detection:

```bash
# Run all failure scenarios
cd deployments/kubernetes/kind
./test-failure-scenarios.sh

# Run specific scenario
./test-failure-scenarios.sh --scenario oom-killed
./test-failure-scenarios.sh --scenario image-pull-backoff

# List available scenarios
./test-failure-scenarios.sh --list
```

Available scenarios:
- `image-pull-backoff` - Invalid image reference
- `crash-loop-backoff` - Container that immediately exits
- `oom-killed` - Container exceeds memory limit
- `deployment-failed` - Deployment with impossible resource requests
- `job-failed` - Job that exceeds backoff limit
- `pvc-provision-failed` - PVC with non-existent storage class

## Querying Events

Use the `lumo events` command to query stored events:

```bash
# Show recent events
lumo events

# Filter by severity
lumo events --severity critical
lumo events --severity high,critical

# Filter by type
lumo events --type oom-killed
lumo events --type crash-loop-backoff

# Filter by namespace
lumo events --namespace kube-system

# Show events from last 24 hours
lumo events --since 24h

# Output as JSON
lumo events --format json

# Hide AI analysis column
lumo events --no-analysis
```

## Production Best Practices

1. **Deploy 2+ replicas** for high availability
2. **Use Redis** for event deduplication (required)
3. **Enable Prometheus metrics** for observability
4. **Set appropriate debounce windows** (45s default is good for most cases)
5. **Use RBAC** with watch permissions on all monitored resources
6. **Enable TLS** for API communication in production
7. **Use Kubernetes secrets** for JWT tokens
8. **Monitor Redis** to ensure event state tracking works
9. **Set up alerts** for agent failures and high event rates
10. **Test with chaos engineering** before production deployment

## Next Steps

- **[Example 6: VM Agent Deployment](../06-agent-deployment-vms/)** - Deploy on bare metal/VMs
- **[Example 7: Querying Events](../07-events-query/)** - Advanced event querying
- **[K8s Deployment Guide](../../deployments/kubernetes/README.md)** - Complete documentation
- **[Event-Driven Implementation](../../EVENT_DRIVEN_IMPLEMENTATION.md)** - Technical details
- **[Kind Full Stack Deployment](../../deployments/kubernetes/kind/FULL_STACK_DEPLOYMENT.md)** - Local testing guide

## Additional Resources

- [Kubernetes Best Practices](../../docs/kubernetes-best-practices.md)
- [RBAC Configuration](../../docs/rbac.md)
- [Prometheus Metrics](../../docs/metrics.md)
