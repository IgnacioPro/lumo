# Lumo Full Stack - Quick Reference

## One-Command Deployment

```bash
cd /Users/ignacio/Code/lumo/deployments/kubernetes/kind
./test-agent.sh
```

**Deploys:** PostgreSQL → API Server → Agents  
**Tests:** 10 automated tests  
**Time:** ~5-7 minutes

---

## What Gets Deployed

| Component | Type | Purpose | Replicas |
|-----------|------|---------|----------|
| PostgreSQL | Deployment | Database | 1 |
| Lumo API | Deployment | REST API | 1 |
| Agent DaemonSet | DaemonSet | Per-node monitoring | N (# nodes) |
| Agent Cluster | Deployment | Cluster monitoring | 2 |

**Total:** 7 pods in a 3-node cluster

---

## Common Commands

### Full Deployment
```bash
./test-agent.sh
```

### Use Existing Cluster
```bash
./test-agent.sh --skip-cluster
```

### Rebuild & Redeploy
```bash
./test-agent.sh --skip-cluster --skip-infrastructure --skip-api
```

### Test Only
```bash
./test-agent.sh --skip-cluster --skip-build --skip-infrastructure --skip-api --skip-deploy
```

### Clean Slate
```bash
kind delete cluster --name lumo-test
./test-agent.sh
```

---

## Skip Flags

| Flag | Skips | Use When |
|------|-------|----------|
| `--skip-cluster` | Cluster creation | Cluster already exists |
| `--skip-build` | Image builds | Images already built |
| `--skip-infrastructure` | PostgreSQL | DB already deployed |
| `--skip-api` | API Server | API already deployed |
| `--skip-deploy` | Agents | Agents already deployed |

---

## Access Services

### PostgreSQL
```bash
kubectl port-forward -n lumo-system svc/postgres 5432:5432 &
PGPASSWORD=lumo psql -h localhost -U lumo -d lumo
```

### API Server
```bash
kubectl port-forward -n lumo-system svc/lumo-api 8080:8080 &
curl http://localhost:8080/api/v1/health
```

### Agent Metrics
```bash
kubectl port-forward -n lumo-system svc/lumo-agent-cluster 9090:9090 &
curl http://localhost:9090/metrics | grep lumo_agent
```

---

## View Logs

```bash
# PostgreSQL
kubectl logs -n lumo-system -l app=postgres -f

# API Server
kubectl logs -n lumo-system -l app=lumo-api -f

# All Agents
kubectl logs -n lumo-system -l app.kubernetes.io/name=lumo-agent -f

# DaemonSet only
kubectl logs -n lumo-system -l app.kubernetes.io/component=node-monitor -f

# Deployment only
kubectl logs -n lumo-system -l app.kubernetes.io/component=cluster-monitor -f
```

---

## Check Status

```bash
# All pods
kubectl get pods -n lumo-system -o wide

# Expected:
# postgres-xxx              1/1   Running
# lumo-api-xxx              1/1   Running
# lumo-agent-node-xxx (×3)  1/1   Running
# lumo-agent-cluster-xxx    1/1   Running
# lumo-agent-cluster-yyy    1/1   Running

# Services
kubectl get svc -n lumo-system

# Events
kubectl get events -n lumo-system --sort-by='.lastTimestamp'
```

---

## Test Output

### Success Indicators
```
✓ All pods are running (7/7)
✓ PostgreSQL is accessible and functional
✓ API server health endpoint responding
✓ Agent health endpoint responding
✓ Agent ready endpoint responding
✓ Metrics endpoint responding
✓ ServiceAccount has required permissions
✓ DaemonSet scheduled on all nodes (3/3)
✓ Agent registration activity found in API logs
✓ Agent is attempting API communication
✓ No fatal errors found in any component
```

### Total: 10 Tests Passed ✅

---

## Troubleshooting

### Pods Not Starting
```bash
kubectl describe pod -n lumo-system <pod-name>
kubectl logs -n lumo-system <pod-name> --previous
```

### API Connection Issues
```bash
# Test from agent pod
kubectl exec -n lumo-system <agent-pod> -- \
  wget -O- http://lumo-api.lumo-system.svc.cluster.local:8080/api/v1/health
```

### Database Issues
```bash
# Test PostgreSQL
kubectl exec -n lumo-system <postgres-pod> -- \
  psql -U lumo -d lumo -c "SELECT 1"
```

### Re-run Tests
```bash
SKIP_CLUSTER_SETUP=true \
SKIP_BUILD=true \
SKIP_INFRASTRUCTURE=true \
SKIP_API=true \
SKIP_DEPLOY=true \
./test-agent.sh
```

---

## Architecture

```
┌─────────────────────────────────────┐
│  kind Cluster: lumo-test            │
│  Namespace: lumo-system             │
│                                      │
│  PostgreSQL:5432                    │
│       ↑                              │
│  Lumo API:8080                      │
│       ↑                              │
│  ┌────┴─────┐                       │
│  │          │                        │
│  Agent    Agent (×5)                │
│  DaemonSet  Deployment              │
└─────────────────────────────────────┘
```

---

## Development Workflow

```bash
# 1. Make code changes
vim internal/diagnostics/cpu.go

# 2. Rebuild & redeploy (keeps cluster, DB, API)
./test-agent.sh --skip-cluster --skip-infrastructure --skip-api

# 3. View logs
kubectl logs -n lumo-system -l app.kubernetes.io/name=lumo-agent -f

# 4. Iterate
```

---

## Environment Variables

```bash
export SKIP_CLUSTER_SETUP=true
export SKIP_BUILD=true
export SKIP_INFRASTRUCTURE=true
export SKIP_API=true
export SKIP_DEPLOY=true
export KIND_CLUSTER_NAME=lumo-test
export LUMO_NAMESPACE=lumo-system
```

---

## Files

- **test-agent.sh** - Main deployment script (use this!)
- **FULL_STACK_DEPLOYMENT.md** - Complete documentation
- **CHANGES_SUMMARY.md** - What changed
- **README.md** - General kind testing guide
- **manifests/postgres.yaml** - PostgreSQL deployment
- **manifests/api-server.yaml** - API server deployment

---

## Help

```bash
./test-agent.sh --help
```

---

## Cleanup

```bash
# Delete cluster (removes everything)
kind delete cluster --name lumo-test

# Delete namespace only
kubectl delete namespace lumo-system
```

---

## Next Steps

1. **Read**: [FULL_STACK_DEPLOYMENT.md](FULL_STACK_DEPLOYMENT.md)
2. **Deploy**: `./test-agent.sh`
3. **Explore**: Access services and view logs
4. **Develop**: Make changes and iterate

---

**Questions?** Check [README.md](README.md) or [FULL_STACK_DEPLOYMENT.md](FULL_STACK_DEPLOYMENT.md)
