# Redis Monitoring for Lumo

This document describes the comprehensive Redis monitoring setup for Lumo's caching layer.

## Overview

Lumo uses Redis for:
- Event debouncing state (event-driven K8s monitoring)
- Agent cache hit/miss tracking
- API response caching
- Distributed locks

The monitoring stack provides real-time visibility into Redis performance, memory usage, and cache efficiency.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Redis Pod                                │
│  ┌──────────────────┐      ┌──────────────────────────┐    │
│  │                  │      │                          │    │
│  │  Redis Server    │──────│  Redis Exporter          │    │
│  │  (port 6379)     │      │  (port 9121)             │    │
│  │                  │      │  oliver006/redis_exporter│    │
│  └──────────────────┘      └──────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                                      │
                                      │ /metrics
                                      ▼
                            ┌──────────────────┐
                            │   Prometheus     │
                            │   (scrapes every │
                            │    30 seconds)   │
                            └──────────────────┘
                                      │
                                      ▼
                            ┌──────────────────┐
                            │    Grafana       │
                            │  (visualizes)    │
                            └──────────────────┘
```

## Components

### 1. Redis Exporter Sidecar

**Image:** `oliver006/redis_exporter:v1.55.0-alpine`
**Port:** 9121
**Location:** `deployments/kubernetes/kind/manifests/redis.yaml`

The Redis Exporter runs as a sidecar container in the Redis pod, exposing Prometheus-compatible metrics.

**Key Metrics Exposed:**
- `redis_up` - Redis availability (1 = up, 0 = down)
- `redis_connected_clients` - Number of connected clients
- `redis_memory_used_bytes` - Current memory usage
- `redis_memory_max_bytes` - Maximum memory limit
- `redis_commands_total` - Total commands processed by type
- `redis_keyspace_hits_total` - Cache hits
- `redis_keyspace_misses_total` - Cache misses
- `redis_db_keys` - Number of keys per database
- `redis_evicted_keys_total` - Keys evicted due to memory pressure
- `redis_expired_keys_total` - Keys expired (TTL)
- `redis_net_input_bytes_total` - Network input bytes
- `redis_net_output_bytes_total` - Network output bytes

### 2. Service Configuration

The Redis service exposes two ports:
- **6379**: Redis server (internal)
- **9121**: Metrics endpoint (for Prometheus)

```yaml
spec:
  ports:
    - port: 6379
      targetPort: 6379
      name: redis
    - port: 9121
      targetPort: 9121
      name: metrics
```

### 3. Prometheus Discovery

**Method:** Annotation-based discovery (standard Prometheus)

The Redis pod includes these annotations:
```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "9121"
  prometheus.io/path: "/metrics"
```

Prometheus automatically discovers and scrapes pods with these annotations.

**Alternative: ServiceMonitor** (requires Prometheus Operator)

**Location:** `deployments/kubernetes/kind/manifests/redis-servicemonitor.yaml`

If you're using Prometheus Operator, you can optionally use ServiceMonitor instead of annotations.

**Note:** Most Lumo deployments use the standard Prometheus Helm chart, which uses annotation-based discovery. ServiceMonitor is only needed if you have Prometheus Operator installed.

### 4. Grafana Dashboard

**Location:** `deployments/kubernetes/monitoring/grafana/dashboards/lumo-overview.json`

The Lumo Overview dashboard includes a dedicated "Redis Metrics" section with 10 panels:

#### Summary Panels (Stats)
1. **Connected Clients** - Current number of Redis clients
   - Thresholds: Green (<50), Yellow (50-100), Red (>100)

2. **Memory Usage %** - Gauge showing memory utilization
   - Thresholds: Green (<70%), Yellow (70-90%), Red (>90%)

3. **Redis Status** - Up/Down indicator
   - Background color: Green (up) / Red (down)

4. **Memory Usage (Bytes)** - Absolute memory usage vs max

5. **Total Keys** - Sum of keys across all databases

#### Time Series Panels
6. **Commands/sec by Type** - Command throughput broken down by command type (GET, SET, DEL, etc.)
   - Helps identify command patterns and potential bottlenecks

7. **Cache Hit Rate %** - Percentage of cache hits vs total requests
   - Formula: `100 * (hits / (hits + misses))`
   - Thresholds: Red (<50%), Yellow (50-80%), Green (>80%)
   - Low hit rates may indicate:
     - Insufficient cache size
     - Poor TTL configuration
     - Cache warming needed

8. **Network I/O** - Bytes in/out per second
   - Monitors network throughput
   - Useful for capacity planning

9. **Keys by Database** - Stacked bar chart of keys per Redis database (db0, db1, etc.)
   - Helps understand data distribution

10. **Evicted/Expired Keys** - Rate of key evictions and expirations
    - Evictions (red): Memory pressure - consider increasing memory limit
    - Expirations (yellow): Normal TTL-based cleanup

## Metrics Reference

### Critical Metrics to Monitor

| Metric | Significance | Alert Threshold |
|--------|--------------|----------------|
| `redis_up` | Redis availability | < 1 (critical) |
| Memory Usage % | Memory pressure | > 90% (warning), > 95% (critical) |
| Cache Hit Rate | Cache efficiency | < 50% (investigate), < 20% (critical) |
| Connected Clients | Connection pool health | > 100 (warning) |
| Evicted Keys/sec | Memory exhaustion | > 100 (warning), > 1000 (critical) |
| Commands/sec | Load profile | Baseline + 200% (capacity planning) |

### Performance Baselines (Expected Values)

**Lumo Startup Profile (1-10 agents):**
- Connected Clients: 5-15
- Memory Usage: 10-50 MB
- Commands/sec: 10-50
- Cache Hit Rate: 70-90%

**Small Business Profile (10-100 agents):**
- Connected Clients: 15-50
- Memory Usage: 50-200 MB
- Commands/sec: 100-500
- Cache Hit Rate: 80-95%

**Enterprise Profile (100-1000 agents):**
- Connected Clients: 50-200
- Memory Usage: 200-512 MB
- Commands/sec: 500-5000
- Cache Hit Rate: 85-95%

## Deployment

### Apply Redis with Monitoring

```bash
# Deploy Redis with exporter sidecar
kubectl apply -f deployments/kubernetes/kind/manifests/redis.yaml

# Verify Redis Exporter is running
kubectl get pods -n lumo-system -l app=lumo-redis
# Should show: 2/2 Running (redis + redis-exporter)

kubectl logs -n lumo-system -l app=lumo-redis -c redis-exporter
# Should show: "Redis Exporter v1.55.0 starting..."

# Test metrics endpoint directly
kubectl exec -n lumo-system -it $(kubectl get pod -n lumo-system -l app=lumo-redis -o name) -c redis-exporter -- wget -O- http://localhost:9121/metrics | grep redis_up
# Should show: redis_up 1

# Wait 30-60 seconds for Prometheus to discover the pod
# Prometheus scrapes pods with prometheus.io/scrape annotation every 30s
```

### Optional: Deploy ServiceMonitor (Prometheus Operator only)

```bash
# Only if you have Prometheus Operator installed:
kubectl apply -f deployments/kubernetes/kind/manifests/redis-servicemonitor.yaml

# Check if ServiceMonitor CRD exists first:
kubectl get crd servicemonitors.monitoring.coreos.com
```

### Deploy Grafana Dashboard

The dashboard is automatically loaded via ConfigMap:

```bash
# Apply dashboard ConfigMap
kubectl apply -f deployments/kubernetes/monitoring/grafana/dashboard-configmap.yaml

# Restart Grafana to reload (if using static provisioning)
kubectl rollout restart deployment/grafana -n monitoring

# Access Grafana
kubectl port-forward -n monitoring svc/grafana 3000:80
# Open http://localhost:3000
# Navigate to: Dashboards → Lumo → Lumo Overview
# Scroll to: "Redis Metrics" section
```

## Troubleshooting

### ServiceMonitor CRD Not Found Error

**Error:** `no matches for kind "ServiceMonitor" in version "monitoring.coreos.com/v1"`

**Cause:** You're using standard Prometheus (not Prometheus Operator)

**Solution:** ServiceMonitor is not needed! Your setup uses annotation-based discovery instead. Prometheus automatically discovers pods with `prometheus.io/scrape: "true"` annotation. Skip the ServiceMonitor step and proceed to verify metrics below.

### No Redis Metrics in Grafana

**Check 1: Verify Redis Exporter is running**
```bash
kubectl logs -n lumo-system -l app=lumo-redis -c redis-exporter
# Should show: "Redis Exporter v1.55.0 starting..."
```

**Check 2: Test metrics endpoint**
```bash
kubectl exec -n lumo-system -it $(kubectl get pod -n lumo-system -l app=lumo-redis -o name) -c redis-exporter -- wget -O- http://localhost:9121/metrics
# Should return Prometheus format metrics
```

**Check 3: Verify Prometheus pod annotations**
```bash
kubectl get pod -n lumo-system -l app=lumo-redis -o jsonpath='{.items[0].metadata.annotations}' | jq
# Should show: prometheus.io/scrape: "true", port: "9121", path: "/metrics"
```

**Check 4: Wait for Prometheus to discover**
```bash
# Prometheus scrapes annotated pods every 30 seconds
# Wait 1-2 minutes after deploying, then check Grafana

# Or verify Prometheus has the target:
kubectl port-forward -n monitoring svc/prometheus-server 9090:80 &
# Open http://localhost:9090/targets
# Search for: "lumo-redis" (should be "UP" status)
```

**Check 5: Verify Grafana data source**
```bash
# Ensure Grafana is configured to use Prometheus
# Grafana → Configuration → Data Sources → Prometheus
# URL should be: http://prometheus-server.monitoring.svc.cluster.local:80
```

### High Memory Usage

**Symptom:** Memory usage > 90%

**Diagnosis:**
```bash
# Connect to Redis CLI
kubectl exec -n lumo-system -it $(kubectl get pod -n lumo-system -l app=lumo-redis -o name) -c redis -- redis-cli

# Check memory stats
INFO memory

# Check key distribution
INFO keyspace

# Find largest keys
MEMORY USAGE <key-name>
```

**Solutions:**
1. Increase memory limit in `redis.yaml`:
   ```yaml
   resources:
     limits:
       memory: 1Gi  # Increase from 512Mi
   ```

2. Enable eviction policy (add to redis container):
   ```yaml
   command:
     - redis-server
     - --maxmemory-policy
     - allkeys-lru
   ```

3. Reduce TTL for event debounce keys (default: 5 minutes)

### Low Cache Hit Rate

**Symptom:** Cache hit rate < 50%

**Possible Causes:**
1. **Insufficient cache size** - Keys being evicted before use
2. **Poor key distribution** - Some keys accessed frequently, others never
3. **TTL too short** - Keys expiring before second access

**Diagnosis:**
```bash
# Check eviction stats
kubectl exec -n lumo-system -it $(kubectl get pod -n lumo-system -l app=lumo-redis -o name) -c redis -- redis-cli INFO stats | grep evicted
```

**Solutions:**
1. Increase memory allocation
2. Review key naming patterns in application code
3. Adjust TTL values in agent configuration
4. Implement cache warming for frequently accessed keys

### High Eviction Rate

**Symptom:** `redis_evicted_keys_total` increasing rapidly

**Impact:** Cache effectiveness degraded, increased load on PostgreSQL

**Solutions:**
1. **Immediate:** Increase memory limit
   ```bash
   kubectl patch deployment lumo-redis -n lumo-system -p '{"spec":{"template":{"spec":{"containers":[{"name":"redis","resources":{"limits":{"memory":"1Gi"}}}]}}}}'
   ```

2. **Long-term:**
   - Implement cache tiering (hot vs cold data)
   - Review event debounce window (currently 45s)
   - Consider Redis cluster for horizontal scaling

## Integration with Lumo Components

### Event-Driven Agent

Uses Redis for:
- **Debouncing state**: Tracks events within 45s window (`internal/agent/eventdriven/debouncer.go`)
- **Deduplication**: Prevents duplicate event submissions
- **Event counting**: Tracks event frequency for severity escalation

**Key Patterns:**
- Key format: `debounce:<namespace>:<resource>:<name>`
- TTL: 5 minutes (configurable via `LUMO_AGENT_EVENT_DRIVEN_DEBOUNCE_WINDOW`)

### Agent Cache

Uses Redis for:
- **API response caching**: Reduces load on API server
- **State persistence**: Maintains agent state across restarts

**Metrics Impact:**
- High hit rate = Efficient caching, reduced API calls
- Low hit rate = Investigate cache key patterns

## Best Practices

1. **Monitor memory usage trends** - Set up alerts before hitting 90%
2. **Baseline your workload** - Establish normal patterns for your deployment profile
3. **Review cache efficiency weekly** - Target 80%+ hit rate
4. **Scale proactively** - Don't wait for OOM kills
5. **Use appropriate TTLs** - Balance freshness vs hit rate
6. **Monitor eviction rate** - Should be near zero under normal load

## Alert Rules (Recommended)

```yaml
# Add to Prometheus AlertManager
groups:
  - name: redis
    interval: 30s
    rules:
      - alert: RedisDown
        expr: redis_up{service="lumo-redis"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Redis is down"

      - alert: RedisHighMemory
        expr: 100 * (redis_memory_used_bytes / redis_memory_max_bytes) > 90
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Redis memory usage > 90%"

      - alert: RedisLowHitRate
        expr: 100 * (rate(redis_keyspace_hits_total[5m]) / (rate(redis_keyspace_hits_total[5m]) + rate(redis_keyspace_misses_total[5m]))) < 50
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Redis cache hit rate < 50%"

      - alert: RedisHighEvictionRate
        expr: rate(redis_evicted_keys_total[5m]) > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Redis evicting keys due to memory pressure"
```

## Resources

- **Redis Exporter Documentation**: https://github.com/oliver006/redis_exporter
- **Redis Metrics Guide**: https://redis.io/docs/management/optimization/metrics/
- **Prometheus Redis Dashboard**: https://grafana.com/grafana/dashboards/763
- **Lumo Event-Driven Documentation**: `EVENT_DRIVEN_IMPLEMENTATION.md`

## Support

For issues or questions:
- GitHub Issues: https://github.com/ignacio/lumo/issues
- Label: `component:redis` or `component:monitoring`
