# Example 5: Agent Deployment on Kubernetes

This example demonstrates deploying Lumo agents to Kubernetes clusters for continuous monitoring.

## What You'll Learn

- Deploy DaemonSet (per-node monitoring)
- Deploy Deployment (cluster-wide monitoring)
- Configure RBAC permissions
- Set up API server for agent reporting
- Monitor agent health and metrics
- Scale and manage agents

## Prerequisites

- Kubernetes cluster (1.25+)
- kubectl configured
- Lumo API server running (or use `lumo serve`)
- API key for agent authentication

## Architecture Overview

```
┌─────────────────────────────────────────┐
│         Kubernetes Cluster              │
│                                         │
│  ┌──────────────┐  ┌──────────────┐   │
│  │ Node 1       │  │ Node 2       │   │
│  │ ┌──────────┐ │  │ ┌──────────┐ │   │
│  │ │  Agent   │ │  │ │  Agent   │ │   │
│  │ │ DaemonSet│ │  │ │ DaemonSet│ │   │
│  │ └────┬─────┘ │  │ └────┬─────┘ │   │
│  └──────┼───────┘  └──────┼───────┘   │
│         │                  │           │
│         └──────────┬───────┘           │
│                    │                   │
│         ┌──────────▼────────┐          │
│         │   Cluster Agent   │          │
│         │   (Deployment)    │          │
│         └──────────┬────────┘          │
└────────────────────┼───────────────────┘
                     │
            ┌────────▼─────────┐
            │   Lumo API       │
            │   Server         │
            └──────────────────┘
```

## Quick Start

### 1. Start API Server

```bash
# Local development
docker-compose up -d  # PostgreSQL + Redis
lumo serve --port 8080

# Or use existing API server
export LUMO_API_ENDPOINT=https://lumo-api.example.com
```

### 2. Create API Key

```bash
# Access PostgreSQL
docker exec -it lumo-postgres psql -U lumo

# Create API key
INSERT INTO api_keys (id, name, key_hash, scopes, created_at, expires_at)
VALUES (
  gen_random_uuid(),
  'k8s-agents',
  crypt('your-secret-key', gen_salt('bf')),
  ARRAY['agent:register', 'agent:heartbeat', 'diagnostics:create'],
  NOW(),
  NOW() + INTERVAL '1 year'
);

# Save the key
export LUMO_API_KEY=your-secret-key
```

### 3. Deploy DaemonSet (Per-Node Monitoring)

```bash
# Apply RBAC
kubectl apply -f ../../deployments/kubernetes/base/rbac.yaml

# Create secret with API key
kubectl create secret generic lumo-agent-secret \
  --from-literal=api-key=$LUMO_API_KEY \
  --from-literal=api-endpoint=http://lumo-api:8080

# Deploy DaemonSet
kubectl apply -f ../../deployments/kubernetes/base/daemonset.yaml
```

### 4. Verify Deployment

```bash
# Check agent pods
kubectl get pods -l app=lumo-agent -o wide

# Check logs
kubectl logs -l app=lumo-agent -f

# Check agent registration
curl http://localhost:8080/api/v1/agents \
  -H "X-API-Key: $LUMO_API_KEY"
```

## DaemonSet Deployment (Node-Level Monitoring)

### What It Does

- Runs one agent per Kubernetes node
- Monitors node-level resources (CPU, memory, disk)
- Uses `hostNetwork`, `hostPID` for full system access
- Reports to API server every 5 minutes

### Configuration

```yaml
# daemonset-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lumo-agent-config
data:
  config.yaml: |
    agent:
      mode: hybrid
      schedule: "*/5 * * * *"
      api_endpoint: http://lumo-api:8080
      enabled_checks:
        - cpu
        - memory
        - disk
        - process
        - service
        - network
      report_format: toon
      offline_mode: true
      health_check_port: 8080
      metrics_port: 9090
```

Apply configuration:

```bash
kubectl apply -f daemonset-config.yaml
```

### Monitoring DaemonSet Agents

```bash
# Pod status
kubectl get daemonset lumo-agent

# Logs from specific node
kubectl logs -l app=lumo-agent --node worker-01

# Health check
kubectl get pods -l app=lumo-agent -o wide
kubectl port-forward lumo-agent-xxxxx 8080:8080
curl http://localhost:8080/health

# Metrics
kubectl port-forward lumo-agent-xxxxx 9090:9090
curl http://localhost:9090/metrics
```

## Deployment for Cluster Monitoring

### What It Does

- 2+ replicas for high availability
- Monitors cluster-wide resources via K8s API
- Checks pods, deployments, services, statefulsets
- No hostNetwork required

### Deploy

```bash
kubectl apply -f ../../deployments/kubernetes/base/deployment.yaml
```

### Monitoring Cluster Agents

```bash
# Deployment status
kubectl get deployment lumo-agent-cluster

# Scale replicas
kubectl scale deployment lumo-agent-cluster --replicas=3

# Logs
kubectl logs -l app=lumo-agent-cluster -f
```

## Helm Deployment (Recommended)

### Install Helm Chart

```bash
# Add local chart
helm install lumo-agent ../../deployments/kubernetes/helm/lumo-agent \
  --set apiEndpoint=http://lumo-api:8080 \
  --set apiKey=$LUMO_API_KEY \
  --set daemonset.enabled=true \
  --set deployment.enabled=true
```

### Custom Values

```yaml
# custom-values.yaml
apiEndpoint: https://lumo-api.example.com
apiKey: your-api-key

daemonset:
  enabled: true
  resources:
    limits:
      memory: 256Mi
      cpu: 200m
    requests:
      memory: 128Mi
      cpu: 100m

deployment:
  enabled: true
  replicas: 2
  resources:
    limits:
      memory: 512Mi
      cpu: 500m

config:
  mode: hybrid
  schedule: "*/5 * * * *"
  enabledChecks:
    - cpu
    - memory
    - disk
    - kubernetes
  logLevel: info
  reportFormat: toon

prometheus:
  enabled: true
  serviceMonitor:
    enabled: true
```

Install with custom values:

```bash
helm install lumo-agent ../../deployments/kubernetes/helm/lumo-agent \
  -f custom-values.yaml
```

### Upgrade Deployment

```bash
# Update values
helm upgrade lumo-agent ../../deployments/kubernetes/helm/lumo-agent \
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
helm install lumo-agent-us-east ./helm/lumo-agent \
  --set cluster.name=us-east \
  --set apiEndpoint=https://lumo-api-central.example.com

# Cluster 2
kubectl config use-context cluster-2
helm install lumo-agent-us-west ./helm/lumo-agent \
  --set cluster.name=us-west \
  --set apiEndpoint=https://lumo-api-central.example.com
```

### Node Affinity / Taints

```yaml
# Only run on specific nodes
daemonset:
  nodeSelector:
    monitoring: enabled

  tolerations:
  - key: monitoring
    operator: Equal
    value: "true"
    effect: NoSchedule

  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node-role.kubernetes.io/master
            operator: DoesNotExist
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
kubectl logs -l app=lumo-agent

# Verify API endpoint reachability
kubectl run test --rm -it --image=curlimages/curl -- \
  curl -v http://lumo-api:8080/api/v1/health

# Check secret
kubectl get secret lumo-agent-secret -o yaml

# Verify RBAC
kubectl auth can-i get pods --as=system:serviceaccount:default:lumo-agent
```

### High Resource Usage

```bash
# Check resource usage
kubectl top pods -l app=lumo-agent

# Reduce check frequency
# Edit ConfigMap, change schedule to "*/15 * * * *"

# Limit checks
# Remove expensive checks (process, kubernetes)
```

### Networking Issues

```bash
# Test from agent pod
kubectl exec -it lumo-agent-xxxxx -- sh
wget -O- http://lumo-api:8080/api/v1/health

# Check NetworkPolicy
kubectl get networkpolicy
kubectl describe networkpolicy lumo-agent

# Check DNS
kubectl exec -it lumo-agent-xxxxx -- nslookup lumo-api
```

## Production Best Practices

1. **Use Helm** for easier management
2. **Enable Prometheus metrics** for observability
3. **Set resource limits** to prevent resource exhaustion
4. **Use RBAC** with minimal permissions
5. **Enable TLS** for API communication
6. **Use secrets** for API keys (not ConfigMaps)
7. **Test in staging** before production
8. **Monitor agent health** via Prometheus/Grafana
9. **Set up alerts** for agent failures
10. **Keep agents updated** with latest version

## Next Steps

- **[Example 6: VM Agent Deployment](../06-agent-deployment-vms/)** - Deploy on bare metal/VMs
- **[K8s Deployment Guide](../../deployments/kubernetes/README.md)** - Complete documentation
- **[API Reference](../../api/README.md)** - Agent API specification
- **[Helm Chart Values](../../deployments/kubernetes/helm/lumo-agent/values.yaml)** - All options

## Additional Resources

- [Kubernetes Best Practices](../../docs/kubernetes-best-practices.md)
- [RBAC Configuration](../../docs/rbac.md)
- [Prometheus Metrics](../../docs/metrics.md)
