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
| Agent Overview | Active Agents, API Availability, Diagnostics (1h), Errors (1h) |
| Diagnostics Performance | Run Rate, Duration (p95 by checker) |
| API Events & Processing | Events Processed Rate, AI Analysis Duration |
| Cache & Connectivity | Cache Hit/Miss, Heartbeats, Notifications |

### Metrics Visualized

| Metric | Type | Description |
|--------|------|-------------|
| `lumo_agent_info` | Gauge | Agent instances |
| `lumo_agent_api_available` | Gauge | API connectivity |
| `lumo_agent_diagnostics_total` | Counter | Diagnostic runs |
| `lumo_agent_diagnostics_duration_seconds` | Histogram | Diagnostic latency |
| `lumo_agent_cache_hits_total` | Counter | Cache efficiency |
| `lumo_agent_heartbeats_total` | Counter | Agent health |
| `lumo_api_events_processed_total` | Counter | Event pipeline |
| `lumo_api_ai_analysis_duration_seconds` | Histogram | AI provider latency |
| `lumo_api_notifications_sent_total` | Counter | Notification delivery |

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

```bash
# Check Grafana pods
kubectl get pods -n monitoring -l app.kubernetes.io/name=grafana

# View Grafana logs
kubectl logs -n monitoring -l app.kubernetes.io/name=grafana

# Verify dashboard ConfigMap
kubectl get cm -n monitoring lumo-grafana-dashboards -o yaml
```
