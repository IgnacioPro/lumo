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
