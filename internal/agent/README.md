# Agent Package

The `agent` package implements the Lumo agent daemon for continuous monitoring.

## Overview

The agent runs as a daemon (K8s Deployment or systemd service) and:

- Performs scheduled/event-driven diagnostics
- Reports results to the central API server
- Caches results locally for resilience
- Exposes health and metrics endpoints

## Operational Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| `scheduled` | Runs checks on cron schedule | Regular monitoring |
| `on-demand` | Responds to API requests | Ad-hoc diagnostics |
| `continuous` | Runs checks in a loop | High-frequency monitoring |
| `hybrid` | Scheduled + on-demand | Default for VMs |
| `event-driven` | K8s informer-based | Kubernetes clusters |

## Creating an Agent

```go
import (
    "github.com/ignacio/lumo/internal/agent"
    "github.com/ignacio/lumo/internal/config"
)

cfg, _ := config.Load()
agent, _ := agent.New(cfg, logger)

// Start blocking
agent.Start()
```

Or via CLI:

```bash
lumo-agent start
```

## Event-Driven Mode (Kubernetes)

For Kubernetes clusters, the agent uses SharedInformerFactory to watch for:

- Pod failures (OOMKilled, CrashLoopBackOff, ImagePullBackOff)
- Workload issues (Deployment, StatefulSet, DaemonSet, Job)
- Volume problems (PVC provisioning, mount failures)
- Node conditions (NotReady, pressure conditions)

Events are debounced (45s window) and submitted to the API server for AI analysis.

See `internal/agent/eventdriven/` for implementation details.

## Components

| Component | File | Purpose |
|-----------|------|---------|
| Agent | `agent.go` | Main orchestrator |
| Scheduler | `scheduler.go` | Cron-based check scheduling |
| Reporter | `reporter.go` | API communication |
| Cache | `cache.go` | Local result caching |
| HealthCheck | `health.go` | Agent health status |
| Metrics | `metrics.go` | Prometheus metrics |

## Configuration

```yaml
agent:
  mode: event-driven
  api_endpoint: "https://lumo-api.example.com"
  token: ""  # Set via LUMO_AGENT_TOKEN
  enabled_checks:
    - cpu
    - memory
    - disk
    - kubernetes
  cache_path: /var/cache/lumo

  # Event-driven settings (K8s only)
  event_driven:
    enabled: true
    debounce_window: 45s
    max_events_per_min: 100
```

## Deployment

### Kubernetes

```bash
kubectl apply -f deployments/kubernetes/base/deployment-agent.yaml
```

### VM (systemd)

```bash
./deployments/systemd/install.sh
systemctl enable --now lumo-agent
```

## Directory Structure

```
agent/
├── agent.go            # Main agent struct
├── scheduler.go        # Check scheduling
├── reporter.go         # API communication
├── cache.go            # Local caching
├── health.go           # Health checks
├── metrics.go          # Prometheus metrics
└── eventdriven/        # K8s event-driven monitoring
    ├── manager.go      # Informer lifecycle
    ├── debouncer.go    # Event debouncing
    ├── api_processor.go # API submission
    └── watchers/       # Resource watchers
```
