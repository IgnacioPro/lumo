# Lumo Agent - Kubernetes Deployment

Deploy Lumo Agent to Kubernetes clusters for intelligent SRE/DevOps automation.

## Table of Contents

- [Overview](#overview)
- [Deployment Options](#deployment-options)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Deployment Methods](#deployment-methods)
  - [Installation Scripts](#installation-scripts)
  - [Helm Chart (Recommended)](#helm-chart-recommended)
  - [kubectl + Kustomize](#kubectl--kustomize)
  - [Plain kubectl](#plain-kubectl)
- [Configuration](#configuration)
- [Security](#security)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)
- [Files](#files)

## Overview

Lumo Agent runs in **event-driven mode** for Kubernetes monitoring. Agents are pure event reporters that submit events to the Lumo API server for centralized AI analysis and multi-channel notifications.

**Architecture (Nov 24, 2025):**
- **Agents:** Real-time Kubernetes monitoring with SharedInformerFactory (no AI, no notifications)
- **API Server:** Receives events, performs AI analysis, sends notifications
- **Benefits:** ~2,000 LOC reduction, easier scaling, single source of truth

## Deployment Options

| Component | Purpose | Resource Usage | Scope |
|-----------|---------|----------------|-------|
| **Event-Driven Deployment** | Real-time K8s monitoring | 50m CPU, 64Mi RAM | Pods, Deployments, StatefulSets, Jobs, Volumes, Nodes |

## Prerequisites

- Kubernetes 1.24+
- Helm 3.0+ (for Helm deployment)
- kubectl configured with cluster access
- Lumo API server deployed (for agent registration)
- API token for agent authentication

## Quick Start

### Option 1: Installation Script (Easiest) ⚡

The installation script handles everything automatically:

```bash
cd deployments/kubernetes

# Install event-driven agent
./install.sh \
  --api-endpoint "https://your-lumo-api.example.com" \
  --agent-token "your-jwt-token-here"

# Note: AI provider configuration is set on the API server, not the agent
# Agents are pure event reporters that submit to the API for analysis

# Dry run (preview changes)
./install.sh \
  --api-endpoint "https://your-lumo-api.example.com" \
  --agent-token "your-jwt-token-here" \
  --dry-run
```

**Uninstall:**

```bash
./uninstall.sh                    # Interactive
./uninstall.sh --force            # Skip confirmations
./uninstall.sh --delete-namespace # Delete namespace too
```

### Option 2: Manual Installation

#### 1. Create Namespace

```bash
kubectl create namespace lumo-system
```

#### 2. Create Secrets

```bash
# Create secret with API token
kubectl create secret generic lumo-agent-secret \
  --namespace=lumo-system \
  --from-literal=agent-token="your-jwt-token-here" \
  --dry-run=client -o yaml | kubectl apply -f -

# Note: AI API keys are configured on the API server, not the agent
```

#### 3. Deploy with Helm

```bash
cd deployments/kubernetes/helm

# Install with default values
helm install lumo-agent ./lumo-agent \
  --namespace lumo-system \
  --set agent.apiEndpoint="https://your-lumo-api.example.com"

# Or with custom values
helm install lumo-agent ./lumo-agent \
  --namespace lumo-system \
  --values custom-values.yaml
```

## Deployment Methods

### Installation Scripts

**`install.sh`** - Automated installation script

Features:
- ✅ Checks prerequisites (kubectl, cluster connectivity)
- ✅ Creates namespace automatically
- ✅ Generates secrets from command-line arguments
- ✅ Updates ConfigMap with API endpoint
- ✅ Applies all manifests in correct order
- ✅ Verifies deployment and shows status
- ✅ Supports dry-run mode
- ✅ Optional remediation permissions

**Usage:**

```bash
./install.sh --help  # Show all options

# Basic installation
./install.sh \
  --api-endpoint "https://lumo-api.example.com" \
  --agent-token "your-jwt-token"

# Full installation with AI
./install.sh \
  --namespace lumo-system \
  --api-endpoint "https://lumo-api.example.com" \
  --agent-token "your-jwt-token" \
  --ai-provider anthropic \
  --ai-api-key "sk-ant-..." \
  --enable-remediation  # CAUTION: Grants write permissions

# Custom deployment
./install.sh \
  --api-endpoint "https://lumo-api.example.com" \
  --agent-token "your-jwt-token" \
  --skip-secret         # Use existing secret
  --dry-run             # Preview changes
```

**`uninstall.sh`** - Automated uninstallation script

Features:
- ✅ Shows current state before uninstalling
- ✅ Confirmation prompts (can be skipped with `--force`)
- ✅ Graceful pod termination
- ✅ Optional namespace deletion
- ✅ Removes all RBAC resources

**Usage:**

```bash
./uninstall.sh --help  # Show all options

# Interactive uninstall
./uninstall.sh

# Force uninstall with namespace deletion
./uninstall.sh --force --delete-namespace

# Uninstall from custom namespace
./uninstall.sh --namespace my-namespace
```

### Helm Chart (Recommended)

Helm provides the easiest way to deploy and manage Lumo agents.

#### Install

```bash
# Add Lumo Helm repository (when published)
# helm repo add lumo https://ignacio.github.io/lumo-charts
# helm repo update

# Install from local chart
cd deployments/kubernetes/helm
helm install lumo-agent ./lumo-agent \
  --namespace lumo-system \
  --create-namespace \
  --set agent.apiEndpoint="https://lumo-api.example.com" \
  --set secrets.create=false
```

#### Upgrade

```bash
helm upgrade lumo-agent ./lumo-agent \
  --namespace lumo-system \
  --values custom-values.yaml
```

#### Uninstall

```bash
helm uninstall lumo-agent --namespace lumo-system
```

#### Customize Values

Create a `custom-values.yaml`:

```yaml
agent:
  mode: hybrid
  schedule: "*/10 * * * *"  # Every 10 minutes
  apiEndpoint: "https://lumo-api.example.com"
  enabledChecks: "cpu,memory,disk,kubernetes"

daemonset:
  enabled: true
  resources:
    requests:
      cpu: 200m
      memory: 256Mi

deployment:
  enabled: true
  replicas: 3

rbac:
  enableRemediation: false  # Keep disabled for security

prometheus:
  serviceMonitor:
    enabled: true
```

### kubectl + Kustomize

Kustomize allows declarative management with overlays for different environments.

```bash
# Deploy base configuration
kubectl apply -k deployments/kubernetes/base

# Deploy with environment-specific overlay
kubectl apply -k deployments/kubernetes/overlays/prod
```

#### Create Custom Overlay

Create `deployments/kubernetes/overlays/prod/kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: lumo-system

resources:
  - ../../base

patchesStrategicMerge:
  - daemonset-patch.yaml

configMapGenerator:
  - name: lumo-agent-config
    behavior: merge
    literals:
      - agent.schedule="*/10 * * * *"

images:
  - name: ghcr.io/ignacio/lumo-agent
    newTag: v1.0.0
```

### Plain kubectl

Deploy individual manifests:

```bash
cd deployments/kubernetes/base

# Create namespace
kubectl create namespace lumo-system

# Deploy in order
kubectl apply -f rbac.yaml
kubectl apply -f configmap.yaml
kubectl apply -f secret.yaml
kubectl apply -f daemonset.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f networkpolicy.yaml
```

## Configuration

### Agent Configuration

Key configuration options in ConfigMap:

```yaml
agent:
  mode: hybrid                  # scheduled, on-demand, continuous, hybrid
  schedule: "*/5 * * * *"       # Cron expression
  apiEndpoint: https://lumo-api.example.com
  enabledChecks: cpu,memory,disk,process,service,network,kubernetes
  reportFormat: toon            # 30-60% token reduction
  offlineMode: true             # Continue if API unavailable
```

### Environment Variables

Key environment variables (set via ConfigMap/Secret):

```bash
LUMO_AGENT_MODE=hybrid
LUMO_AGENT_SCHEDULE="*/5 * * * *"
LUMO_AGENT_API_ENDPOINT=https://lumo-api.example.com
LUMO_AGENT_TOKEN=<from-secret>
LUMO_ANTHROPIC_API_KEY=<from-secret>
```

## Security

### RBAC Permissions

By default, agents have **read-only** access:
- ✓ Read nodes, pods, services, deployments
- ✓ Read metrics (if metrics-server installed)
- ✗ Write access (disabled by default)

To enable remediation (use with caution):

```yaml
# values.yaml
rbac:
  enableRemediation: true  # Grants write permissions
```

### Network Policies

NetworkPolicies restrict traffic:
- **Ingress**: Only health/metrics endpoints from monitoring
- **Egress**: Only K8s API, Lumo API, DNS, HTTPS for AI providers

### Secrets Management

**Production**: Use external secret managers:

```yaml
# Example: External Secrets Operator
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: lumo-agent-secret
  namespace: lumo-system
spec:
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: lumo-agent-secret
  data:
    - secretKey: agent-token
      remoteRef:
        key: lumo/agent-token
```

## Monitoring

### Health Endpoints

```bash
# Check health
kubectl port-forward -n lumo-system service/lumo-agent-cluster 8080:8080
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/live
```

### Prometheus Metrics

```bash
# Access metrics
kubectl port-forward -n lumo-system service/lumo-agent-cluster 9090:9090
curl http://localhost:9090/metrics
```

### ServiceMonitor (Prometheus Operator)

Enable automatic Prometheus scraping:

```yaml
# values.yaml
prometheus:
  serviceMonitor:
    enabled: true
    interval: 30s
```

## Troubleshooting

### Check Pod Status

```bash
# List all Lumo pods
kubectl get pods -n lumo-system -l app.kubernetes.io/name=lumo-agent

# Check specific component
kubectl get pods -n lumo-system -l app.kubernetes.io/component=node-monitor
kubectl get pods -n lumo-system -l app.kubernetes.io/component=cluster-monitor
```

### View Logs

```bash
# Event-driven agent logs
kubectl logs -n lumo-system -l mode=event-driven --tail=100 -f

# Specific pod
kubectl logs -n lumo-system <pod-name> -f
```

### Check Events

```bash
kubectl get events -n lumo-system --sort-by='.lastTimestamp'
```

### Describe Resources

```bash
# Describe pod
kubectl describe pod -n lumo-system <pod-name>

# Describe deployment
kubectl describe deployment -n lumo-system lumo-agent-cluster
```

### Common Issues

#### Pods in CrashLoopBackOff

```bash
# Check logs
kubectl logs -n lumo-system <pod-name> --previous

# Common causes:
# 1. Missing secret (agent-token)
# 2. Invalid API endpoint
# 3. Resource limits too low
```

#### RBAC Permission Errors

```bash
# Check RBAC bindings
kubectl get clusterrolebinding | grep lumo
kubectl describe clusterrole lumo-agent-reader

# Verify service account
kubectl get serviceaccount -n lumo-system lumo-agent
```

#### Network Policy Blocking Traffic

```bash
# Temporarily disable NetworkPolicy
kubectl delete networkpolicy -n lumo-system lumo-agent-node

# Check connectivity
kubectl exec -n lumo-system <pod-name> -- curl -I https://api.anthropic.com
```

### Debug Mode

Enable verbose logging:

```bash
# Update ConfigMap
kubectl patch configmap -n lumo-system lumo-agent-config \
  --type merge \
  -p '{"data":{"logging.level":"debug"}}'

# Restart pods
kubectl rollout restart daemonset -n lumo-system lumo-agent-node
kubectl rollout restart deployment -n lumo-system lumo-agent-cluster
```

## Advanced Configuration

### Node Selection

Deploy agents only on specific nodes:

```yaml
# values.yaml
daemonset:
  nodeSelector:
    lumo.io/monitor: "true"
```

Label nodes:

```bash
kubectl label nodes <node-name> lumo.io/monitor=true
```

### Resource Limits

Adjust based on cluster size:

```yaml
# values.yaml
daemonset:
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 1000m
      memory: 512Mi
```

### High Availability

Increase replicas for cluster monitor:

```yaml
# values.yaml
deployment:
  replicas: 3
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        - labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/component
                operator: In
                values:
                  - cluster-monitor
          topologyKey: kubernetes.io/hostname
```

## Files

### Directory Structure

```
deployments/kubernetes/
├── install.sh              # Automated installation script
├── uninstall.sh            # Automated uninstallation script
├── README.md               # This file
├── base/                   # Base Kubernetes manifests
│   ├── rbac.yaml           # ServiceAccount, ClusterRole, ClusterRoleBinding
│   ├── configmap-agent.yaml # Agent configuration
│   ├── secret.yaml         # Secret template (with external secret examples)
│   ├── deployment-agent.yaml # Event-driven agent deployment
│   ├── service.yaml        # Services + ServiceMonitor
│   ├── networkpolicy.yaml  # NetworkPolicy for security
│   └── kustomization.yaml  # Kustomize configuration
├── helm/                   # Helm chart
│   └── lumo-agent/
│       ├── Chart.yaml      # Chart metadata
│       ├── values.yaml     # Default values (100+ options)
│       ├── .helmignore     # Helm ignore patterns
│       └── templates/      # Helm templates
│           ├── _helpers.tpl       # Template helpers
│           ├── NOTES.txt          # Post-install notes
│           ├── namespace.yaml     # Namespace
│           ├── serviceaccount.yaml # ServiceAccount
│           ├── rbac.yaml          # RBAC
│           └── configmap.yaml     # ConfigMap
└── overlays/               # Kustomize overlays (future)
    ├── dev/
    ├── staging/
    └── prod/
```

### Key Files

**Installation Scripts:**
- `install.sh` - One-command installation with validation and verification
- `uninstall.sh` - Clean removal with confirmation prompts

**Base Manifests:**
- `rbac.yaml` - Least-privilege RBAC (read-only by default)
- `deployment-agent.yaml` - Event-driven agent (50m CPU, 64Mi RAM)
- `configmap-agent.yaml` - Event-driven configuration
- `configmap.yaml` - Full agent configuration with config.yaml
- `secret.yaml` - Template + external secret manager examples
- `service.yaml` - Health/metrics endpoints + ServiceMonitor
- `networkpolicy.yaml` - Ingress/egress security controls

**Helm Chart:**
- `Chart.yaml` - Metadata, capabilities, version info
- `values.yaml` - 100+ customization options
- `templates/` - Templatized manifests with Helm functions

## Support

- Documentation: https://github.com/ignacio/lumo
- Issues: https://github.com/ignacio/lumo/issues
- Discussions: https://github.com/ignacio/lumo/discussions
