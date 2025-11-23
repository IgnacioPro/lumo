# Lumo Full Stack Deployment in kind

## Overview

The `test-agent.sh` script has been enhanced to deploy the **complete Lumo stack** in a local kind cluster:

1. **PostgreSQL** - Database for storing agent registrations, jobs, and reports
2. **Lumo API Server** - Central control plane (REST API)
3. **Lumo Agents** - DaemonSet (per-node) + Deployment (cluster-wide)
4. **Integration Tests** - Verify end-to-end functionality

## What Changed

### Before
- Only deployed agents (DaemonSet + Deployment)
- Agents couldn't actually communicate with API (didn't exist)
- No database

### After (Full Stack)
- ✅ PostgreSQL deployment + health verification
- ✅ API Server deployment + health verification
- ✅ Agents connecting to real API server
- ✅ Component tests (each service individually)
- ✅ Integration tests (services talking to each other)

## Architecture

```
┌────────────────────────────────────────────────────┐
│  kind Cluster (lumo-test)                          │
│  Namespace: lumo-system                            │
│                                                     │
│  Step 3: PostgreSQL                                │
│  ├─ Deployment: postgres                           │
│  └─ Service: postgres:5432                         │
│       │                                             │
│  Step 4: API Server                                │
│  ├─ Deployment: lumo-api                           │
│  │  ├─ Image: lumo:local                           │
│  │  ├─ Command: /app/lumo serve                    │
│  │  └─ Connects to PostgreSQL                      │
│  └─ Service: lumo-api:8080                         │
│       │                                             │
│  Step 5: Agents                                    │
│  ├─ DaemonSet: lumo-agent-node (per node)         │
│  │  ├─ Image: lumo-agent:local                     │
│  │  └─ Monitors: CPU, Memory, Disk, Processes      │
│  └─ Deployment: lumo-agent-cluster (2 replicas)   │
│     ├─ Image: lumo-agent:local                     │
│     └─ Monitors: Pods, Deployments, Services       │
│                                                     │
│  All agents connect to: lumo-api:8080              │
└────────────────────────────────────────────────────┘
```

## Usage

### Quick Start (Full Stack)

```bash
cd /Users/ignacio/Code/lumo/deployments/kubernetes/kind

# Deploy everything with one command
./test-agent.sh
```

**What this does:**
1. Creates 3-node kind cluster
2. Builds both images (CLI for API, Agent for agents)
3. Deploys PostgreSQL
4. Deploys API Server
5. Deploys Agents (DaemonSet + Deployment)
6. Runs component tests (7 tests)
7. Runs integration tests (3 tests)
8. Shows summary with helpful commands

### Step-by-Step Deployment

```bash
# 1. Create cluster
./test-agent.sh --skip-build --skip-infrastructure --skip-api --skip-deploy

# 2. Build images
./test-agent.sh --skip-cluster --skip-infrastructure --skip-api --skip-deploy

# 3. Deploy PostgreSQL only
./test-agent.sh --skip-cluster --skip-build --skip-api --skip-deploy

# 4. Deploy API Server
./test-agent.sh --skip-cluster --skip-build --skip-infrastructure --skip-deploy

# 5. Run full test suite on existing deployment
SKIP_CLUSTER_SETUP=true SKIP_BUILD=true SKIP_INFRASTRUCTURE=true SKIP_API=true SKIP_DEPLOY=true ./test-agent.sh
```

### Selective Deployment Options

```bash
# Use existing cluster, rebuild everything, redeploy all
./test-agent.sh --skip-cluster

# Use existing infrastructure, redeploy only agents
./test-agent.sh --skip-cluster --skip-build --skip-infrastructure --skip-api

# Just run tests on existing deployment
./test-agent.sh --skip-cluster --skip-build --skip-infrastructure --skip-api --skip-deploy
```

## New Script Stages

### Stage 1: Cluster Setup (Step 1/7)
- Creates 3-node kind cluster (1 control-plane + 2 workers)
- Configures kubectl context

### Stage 2: Image Build (Step 2/7)
- Builds `lumo:local` (API Server image from `Dockerfile`)
- Builds `lumo-agent:local` (Agent image from `Dockerfile.agent`)
- Loads both images into kind cluster

### Stage 3: Infrastructure Deployment (Step 3/7) - NEW
- Creates `lumo-system` namespace
- Deploys PostgreSQL from `manifests/postgres.yaml`
- Waits for PostgreSQL to be ready (120s timeout)
- Verifies PostgreSQL connection with test query

### Stage 4: API Server Deployment (Step 4/7) - NEW
- Deploys Lumo API Server from `manifests/api-server.yaml`
- Runs `lumo serve` command
- Connects to PostgreSQL via service DNS
- Waits for API Server to be ready (120s timeout)
- Verifies `/api/v1/health` endpoint

### Stage 5: Agent Deployment (Step 5/7)
- Sets `LUMO_API_ENDPOINT` to in-cluster API service
- Deploys agents via `deploy-to-kind.sh`
- Agents configured to connect to API server
- DaemonSet (per-node) + Deployment (cluster-wide)

### Stage 6: Component Tests (Step 6/7) - ENHANCED
- Test 1: All pods running check
- Test 2: PostgreSQL connection test
- Test 3: API server health endpoint
- Test 4: Agent health endpoints
- Test 5: Agent metrics endpoint
- Test 6: RBAC permissions
- Test 7: DaemonSet scheduling

### Stage 7: Integration Tests (Step 7/7) - NEW
- Test 1: Agent registration with API (checks logs)
- Test 2: Agent → API communication attempts
- Test 3: No fatal errors across all components

## Test Output Example

```
==================================================
  Lumo Full Stack - kind Testing Suite
  DB → API Server → Agents → Integration Tests
==================================================

[INFO] Step 1/7: Setting up kind cluster...
✓ Cluster created successfully

[INFO] Step 2/7: Building and loading images (API + Agent)...
✓ Images built successfully

[INFO] Step 3/7: Deploying infrastructure (PostgreSQL)...
✓ PostgreSQL is ready and accepting connections

[INFO] Step 4/7: Deploying Lumo API Server...
✓ API server is healthy and responding

[INFO] Step 5/7: Deploying agents (DaemonSet + Deployment)...
✓ Agents deployed successfully

[INFO] Step 6/7: Running component tests...
✓ All pods are running (7/7)
✓ PostgreSQL is accessible and functional
✓ API server health endpoint responding
✓ Agent health endpoint responding
✓ Agent ready endpoint responding
✓ Metrics endpoint responding
✓ ServiceAccount has required permissions
✓ DaemonSet scheduled on all nodes (3/3)

[INFO] Step 7/7: Running integration tests...
✓ Agent registration activity found in API logs
✓ Agent is attempting API communication
✓ No fatal errors found in any component

========================================================
Lumo Full Stack Testing Complete!
========================================================

Deployed Components:
  ✓ PostgreSQL (Database)
  ✓ Lumo API Server
  ✓ Lumo Agents (DaemonSet + Deployment)
```

## Verification Commands

After deployment, verify the stack:

```bash
# Check all pods
kubectl get pods -n lumo-system -o wide

# Expected output:
# NAME                          READY   STATUS    RESTARTS   AGE
# postgres-xxx                  1/1     Running   0          2m
# lumo-api-xxx                  1/1     Running   0          1m
# lumo-agent-node-aaa           1/1     Running   0          30s
# lumo-agent-node-bbb           1/1     Running   0          30s
# lumo-agent-node-ccc           1/1     Running   0          30s
# lumo-agent-cluster-xxx        1/1     Running   0          30s
# lumo-agent-cluster-yyy        1/1     Running   0          30s

# Test PostgreSQL
kubectl port-forward -n lumo-system svc/postgres 5432:5432 &
PGPASSWORD=lumo psql -h localhost -U lumo -d lumo -c "\dt"

# Test API Server
kubectl port-forward -n lumo-system svc/lumo-api 8080:8080 &
curl http://localhost:8080/api/v1/health

# Check API logs for agent registrations
kubectl logs -n lumo-system -l app=lumo-api | grep -i register

# Check agent logs
kubectl logs -n lumo-system -l app.kubernetes.io/name=lumo-agent -f
```

## Environment Variables

All skip flags can be set via environment variables:

```bash
export SKIP_CLUSTER_SETUP=true      # Skip cluster creation
export SKIP_BUILD=true               # Skip image builds
export SKIP_INFRASTRUCTURE=true      # Skip PostgreSQL
export SKIP_API=true                 # Skip API server
export SKIP_DEPLOY=true              # Skip agents
export KIND_CLUSTER_NAME=my-cluster  # Custom cluster name
export LUMO_NAMESPACE=my-namespace   # Custom namespace

./test-agent.sh
```

## Cleanup

```bash
# Delete the entire cluster (removes everything)
kind delete cluster --name lumo-test

# Or delete just the namespace
kubectl delete namespace lumo-system
```

## Images Built

### 1. `lumo:local` (API Server)
- **Dockerfile**: `/Dockerfile`
- **Source**: `cmd/lumo/`
- **Binary**: `/app/lumo`
- **Command**: `./lumo serve`
- **Purpose**: REST API server

### 2. `lumo-agent:local` (Agents)
- **Dockerfile**: `/Dockerfile.agent`
- **Source**: `cmd/lumo-agent/`
- **Binary**: `/usr/local/bin/lumo-agent`
- **Command**: Default agent daemon
- **Purpose**: DaemonSet and Deployment agents

## Next Steps

1. **Develop features**: Make changes to code
2. **Rebuild**: `./test-agent.sh --skip-cluster`
3. **Test**: Verify changes work end-to-end
4. **Iterate**: Repeat cycle

## Troubleshooting

### PostgreSQL won't start
```bash
kubectl describe pod -n lumo-system -l app=postgres
kubectl logs -n lumo-system -l app=postgres
```

### API Server won't start
```bash
kubectl describe pod -n lumo-system -l app=lumo-api
kubectl logs -n lumo-system -l app=lumo-api
```

### Agents can't connect to API
```bash
# Check API service DNS
kubectl get svc -n lumo-system lumo-api

# Check agent logs
kubectl logs -n lumo-system -l app.kubernetes.io/name=lumo-agent

# Verify agents can reach API
kubectl exec -n lumo-system <agent-pod> -- wget -O- http://lumo-api.lumo-system.svc.cluster.local:8080/api/v1/health
```

### Tests failing
```bash
# Re-run just the tests
SKIP_CLUSTER_SETUP=true SKIP_BUILD=true SKIP_INFRASTRUCTURE=true SKIP_API=true SKIP_DEPLOY=true ./test-agent.sh

# Check all pods
kubectl get pods -n lumo-system

# Check events
kubectl get events -n lumo-system --sort-by='.lastTimestamp'
```

## Comparison: test-agent.sh vs test-workflow.sh

| Feature | test-agent.sh (NEW) | test-workflow.sh (OLD) |
|---------|---------------------|------------------------|
| PostgreSQL | ✅ Yes | ✅ Yes |
| API Server | ✅ Yes | ✅ Yes |
| Agents | ✅ Yes | ✅ Yes |
| Component Tests | ✅ 7 tests | ❌ No |
| Integration Tests | ✅ 3 tests | ✅ Basic |
| Skip Options | ✅ 5 flags | ❌ Limited |
| Help System | ✅ Full usage | ❌ None |
| Status | **Primary** | Deprecated |

**Recommendation**: Use `test-agent.sh` for all testing going forward.

## Summary

The enhanced `test-agent.sh` now provides:
- ✅ Complete full-stack deployment
- ✅ Step-by-step deployment control
- ✅ Comprehensive testing (10 tests total)
- ✅ Better observability and debugging
- ✅ Production-like local environment
- ✅ Ready for development workflows
