# Redis Monitoring Quick Start

## TL;DR

Redis monitoring is **already configured** and working! Just update your Grafana dashboard.

## 3-Step Setup

### 1. Deploy Redis with Exporter (if not already done)

```bash
kubectl apply -f deployments/kubernetes/kind/manifests/redis.yaml
```

This deploys Redis with a sidecar container that exports metrics on port 9121.

### 2. Update Grafana Dashboard

```bash
kubectl apply -f deployments/kubernetes/monitoring/grafana/dashboard-configmap.yaml
kubectl rollout restart deployment/grafana -n monitoring
```

### 3. View Metrics

Wait 1-2 minutes, then access Grafana:

```bash
kubectl port-forward -n monitoring svc/grafana 3000:80
```

Open http://localhost:3000 → Dashboards → Lumo → **Lumo Overview** → Scroll to **"Redis Metrics"** section

## What You'll See

**10 Redis Panels:**
- Connected Clients (stat)
- Memory Usage % (gauge)
- Redis Status (up/down)
- Memory Usage in Bytes
- Total Keys
- Commands/sec by Type (timeseries)
- Cache Hit Rate % (timeseries)
- Network I/O (timeseries)
- Keys by Database (stacked bars)
- Evicted/Expired Keys (timeseries)

## How It Works

1. **Redis Exporter** runs as a sidecar in the Redis pod
2. **Prometheus** automatically discovers pods with `prometheus.io/scrape: "true"` annotation
3. **Grafana** queries Prometheus and visualizes the metrics

No ServiceMonitor needed! (unless you're using Prometheus Operator)

## Verification

```bash
# 1. Check Redis exporter is running
kubectl get pods -n lumo-system -l app=lumo-redis
# Should show: 2/2 Running

# 2. Test metrics endpoint
kubectl exec -n lumo-system $(kubectl get pod -n lumo-system -l app=lumo-redis -o name) -c redis-exporter -- wget -qO- localhost:9121/metrics | grep redis_up
# Should show: redis_up 1

# 3. Wait 1-2 minutes, then check Grafana
# Redis Metrics section should populate with data
```

## Troubleshooting

### "ServiceMonitor not found" error

**This is expected!** You're using standard Prometheus (not Prometheus Operator).

ServiceMonitor is **optional** and not needed for your setup. Prometheus automatically discovers pods via annotations.

### No data in Grafana panels

**Wait 1-2 minutes** - Prometheus scrapes pods every 30 seconds.

If still no data after 2 minutes:
```bash
# Check exporter logs
kubectl logs -n lumo-system -l app=lumo-redis -c redis-exporter

# Check pod has scrape annotations
kubectl get pod -n lumo-system -l app=lumo-redis -o jsonpath='{.items[0].metadata.annotations}' | grep prometheus
```

## Key Metrics

| Metric | Good | Warning | Critical |
|--------|------|---------|----------|
| Memory Usage | <70% | 70-90% | >90% |
| Cache Hit Rate | >80% | 50-80% | <50% |
| Evicted Keys/sec | 0-10 | 10-100 | >100 |
| Connected Clients | 5-50 | 50-100 | >100 |

## Full Documentation

See [REDIS_MONITORING.md](./REDIS_MONITORING.md) for:
- Complete architecture details
- All available metrics
- Performance baselines by deployment profile
- Alert rules
- Troubleshooting guide

## Need Help?

1. Check the full documentation: `REDIS_MONITORING.md`
2. Verify Prometheus is scraping: http://localhost:9090/targets (via port-forward)
3. Check Redis exporter logs: `kubectl logs -n lumo-system -l app=lumo-redis -c redis-exporter`
4. Open a GitHub issue: https://github.com/ignacio/lumo/issues (label: `component:redis` or `component:monitoring`)
