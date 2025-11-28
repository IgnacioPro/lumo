# Lumo Monitoring Stack

This directory contains Grafana deployment manifests and dashboards for Lumo observability.

## Quick Start

### Prerequisites
- Kubernetes cluster with Helm 3.x
- Prometheus (either existing or deploy with `--with-prometheus` flag)

### Deploy

```bash
# Deploy Grafana only (assumes Prometheus already exists)
./deploy-monitoring.sh

# Deploy Grafana + Prometheus
./deploy-monitoring.sh --with-prometheus
```

### Access

```bash
# Port-forward Grafana
kubectl port-forward -n monitoring svc/grafana 3000:80

# Open in browser
open http://localhost:3000
```

Default credentials: `admin` / `admin`

## Components

### Grafana (`grafana/`)

| File | Description |
|------|-------------|
| `values.yaml` | Helm chart values with datasource and sidecar config |
| `dashboard-configmap.yaml` | Kubernetes ConfigMap with Lumo dashboard |
| `dashboards/lumo-overview.json` | Full dashboard JSON for import |
| `REDIS_MONITORING.md` | Complete Redis monitoring guide |
| `REDIS_QUICKSTART.md` | Quick start guide for Redis metrics |

### Dashboard Panels

The **Lumo Overview** dashboard includes:

| Section | Panels |
|---------|--------|
| Agent Overview | Active Agents, API Availability, K8s Events (1h), Critical Events (1h) |
| K8s Event Processing | Events by Type (Rate), Event Processing Duration (p50/p95) |
| Events by Severity & Namespace | Severity breakdown, Namespace breakdown |
| Agent Health & Connectivity | Cache Hit/Miss, Heartbeat Rate, Total Events Pie Chart |
| Agent Diagnostics | Diagnostic Runs, Duration by Checker, Errors by Checker |
| AI Analysis & Notifications | AI Analysis Rate, AI Duration, Notifications by Provider |
| API Server Metrics | API Events Processed, API AI Duration, API Notifications |
| Agent Info | Table of active agents with hostname, mode, platform, version |
| **Redis Metrics** | **Connected Clients, Memory %, Status, Memory Bytes, Total Keys, Commands/sec, Cache Hit Rate %, Network I/O, Keys by DB, Evicted/Expired Keys** |

### Metrics Visualized

| Metric | Type | Source | Description |
|--------|------|--------|-------------|
| `lumo_agent_info` | Gauge | Agent | Agent instances with labels |
| `lumo_agent_api_available` | Gauge | Agent | API connectivity status |
| `lumo_events_processed_total` | Counter | Agent | K8s events by type/severity/namespace |
| `lumo_event_processing_duration_seconds` | Histogram | Agent | Event processing latency |
| `lumo_agent_diagnostics_total` | Counter | Agent | Diagnostic runs by status |
| `lumo_agent_diagnostics_duration_seconds` | Histogram | Agent | Diagnostic latency by checker |
| `lumo_agent_diagnostics_errors_total` | Counter | Agent | Diagnostic errors by checker |
| `lumo_agent_cache_hits_total` | Counter | Agent | Cache efficiency |
| `lumo_agent_cache_misses_total` | Counter | Agent | Cache misses |
| `lumo_agent_heartbeats_total` | Counter | Agent | Agent health |
| `lumo_agent_heartbeat_errors_total` | Counter | Agent | Heartbeat failures |
| `lumo_ai_analysis_total` | Counter | Agent | AI analysis operations |
| `lumo_ai_analysis_duration_seconds` | Histogram | Agent | AI analysis latency |
| `lumo_notifications_sent_total` | Counter | Agent | Notification delivery |
| `lumo_api_events_processed_total` | Counter | API | API event pipeline |
| `lumo_api_ai_analysis_duration_seconds` | Histogram | API | API-side AI latency |
| `lumo_api_notifications_sent_total` | Counter | API | API notifications |
| `redis_up` | Gauge | Redis | Redis availability (1=up, 0=down) |
| `redis_connected_clients` | Gauge | Redis | Number of client connections |
| `redis_memory_used_bytes` | Gauge | Redis | Current memory usage |
| `redis_memory_max_bytes` | Gauge | Redis | Maximum memory limit |
| `redis_commands_total` | Counter | Redis | Total commands by type |
| `redis_keyspace_hits_total` | Counter | Redis | Cache hits |
| `redis_keyspace_misses_total` | Counter | Redis | Cache misses |
| `redis_db_keys` | Gauge | Redis | Number of keys per database |
| `redis_evicted_keys_total` | Counter | Redis | Keys evicted due to memory pressure |
| `redis_expired_keys_total` | Counter | Redis | Keys expired via TTL |
| `redis_net_input_bytes_total` | Counter | Redis | Network input bytes |
| `redis_net_output_bytes_total` | Counter | Redis | Network output bytes |

## Configuration

### Prometheus Datasource

Default datasource URL in `values.yaml`:
```yaml
url: http://prometheus-server.monitoring.svc.cluster.local:80
```

Update if your Prometheus is in a different namespace or has a different service name.

### Dashboard Sidecar

Grafana sidecar is configured to auto-load dashboards from ConfigMaps with label:
```yaml
grafana_dashboard: "1"
```

## Redis Monitoring

**Architecture Note:** Redis is deployed in the `lumo-system` namespace (via `deployments/kubernetes/kind/deploy-lumo.sh`), not in the `monitoring` namespace. The monitoring stack observes Redis via Prometheus auto-discovery using pod annotations.

### Redis in Lumo Architecture

Redis serves as the caching layer for:
- **Agent cache operations** - Reducing API server load
- **Event debouncing** - Preventing duplicate event submissions
- **Session management** - Temporary state storage

Redis deployment features:
- ✅ Deployed in `lumo-system` namespace with the main application
- ✅ Redis Exporter sidecar for Prometheus metrics (port 9121)
- ✅ Automatic Prometheus discovery via pod annotations
- ✅ Cross-namespace monitoring (Prometheus in `monitoring` scrapes Redis in `lumo-system`)

### Redis Metrics Dashboard

The "Redis Metrics" section in the Lumo Overview dashboard provides:

**Health & Status:**
- Redis availability (up/down indicator)
- Connected clients (current connections)
- Memory usage % (gauge with thresholds)

**Performance:**
- Commands/sec by type (GET, SET, DEL breakdown)
- Cache hit rate % (efficiency indicator)
- Network I/O (throughput monitoring)

**Capacity:**
- Memory usage in bytes (used vs max)
- Total keys across all databases
- Keys by database (distribution)
- Evicted/Expired keys (memory pressure indicators)

### Key Performance Indicators

| Metric | Good | Warning | Critical | Action |
|--------|------|---------|----------|--------|
| Cache Hit Rate | >80% | 50-80% | <50% | Increase TTL or memory |
| Memory Usage | <70% | 70-90% | >90% | Increase memory limit |
| Evicted Keys/sec | 0-10 | 10-100 | >100 | Scale Redis or reduce TTL |
| Connected Clients | <50 | 50-100 | >100 | Check connection pooling |

### Documentation

For detailed Redis monitoring information:
- **Quick Start**: [grafana/REDIS_QUICKSTART.md](grafana/REDIS_QUICKSTART.md)
- **Full Guide**: [grafana/REDIS_MONITORING.md](grafana/REDIS_MONITORING.md)

Includes:
- Architecture diagrams
- All available metrics
- Performance baselines
- Troubleshooting guide
- Alert rules

## Production Considerations

1. **Persistence**: Enable PVC for Grafana to preserve dashboards
2. **Secrets**: Use external secret manager for admin password
3. **Ingress**: Configure ingress for external access with TLS
4. **RBAC**: Restrict dashboard editing in production

## Troubleshooting

### init-chown-data Permission Denied

If you see `chown: /var/lib/grafana: Permission denied` errors:

```bash
# The values.yaml disables init-chown-data and uses fsGroup instead.
# If upgrading from an old deployment, delete the PVC first:
kubectl delete pvc grafana -n monitoring
helm upgrade --install grafana grafana/grafana -n monitoring -f values.yaml
```

### Check Pod Status

```bash
# Check Grafana pods
kubectl get pods -n monitoring -l app.kubernetes.io/name=grafana

# View Grafana logs
kubectl logs -n monitoring -l app.kubernetes.io/name=grafana

# View init container logs (if failing)
kubectl logs -n monitoring -l app.kubernetes.io/name=grafana -c init-chown-data

# Verify dashboard ConfigMap
kubectl get cm -n monitoring lumo-grafana-dashboards -o yaml
```

### Dashboard Not Loading

```bash
# Verify ConfigMap exists with correct label
kubectl get cm -n monitoring -l grafana_dashboard=1

# Check sidecar logs
kubectl logs -n monitoring -l app.kubernetes.io/name=grafana -c grafana-sc-dashboard
```
