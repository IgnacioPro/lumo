# Incident Correlation Engine

> **Status:** Phase 19 - Core Implementation Complete
> **Created:** 2025-12-02
> **Package:** `internal/correlation/`

## Overview

The Incident Correlation Engine transforms Lumo from an "alerting tool that sends hundreds of individual notifications" to an **"incident intelligence platform that sends ONE comprehensive incident report."**

### The Problem

Before correlation:
```
12:01:05 → Alert: OOMKilled (pod-abc)
12:01:08 → Alert: CrashLoopBackOff (pod-abc)
12:01:30 → Alert: PodEvicted (pod-abc)
12:02:00 → Alert: NodeMemoryPressure (node-1)
12:02:15 → Alert: OOMKilled (pod-xyz)
12:02:20 → Alert: CrashLoopBackOff (pod-xyz)
... (50+ more alerts)
```

**Result:** Alert fatigue. On-call engineer gets paged 50 times for the same incident.

### The Solution

After correlation:
```
┌─────────────────────────────────────────────────────────────────┐
│ 🔴 INCIDENT: Memory Exhaustion on node-1                        │
├─────────────────────────────────────────────────────────────────┤
│ Duration: 5m 23s | Events: 47 | Resources: 12 pods              │
│ Namespace: production                                            │
├─────────────────────────────────────────────────────────────────┤
│ 🔍 ROOT CAUSE:                                                   │
│ Memory leak in payment-service v2.3.1 (deployed 2h ago)         │
│ caused node-1 memory pressure, triggering pod evictions.        │
├─────────────────────────────────────────────────────────────────┤
│ 📅 TIMELINE:                                                     │
│ • 12:01:05 OOMKilled payment-service-abc                        │
│ • 12:01:30 NodeMemoryPressure node-1                            │
│ • 12:02:00 Pod evictions began (12 pods affected)               │
├─────────────────────────────────────────────────────────────────┤
│ 🛠 IMMEDIATE ACTIONS:                                            │
│ 1. Rollback payment-service: kubectl rollout undo deploy/...   │
│ 2. Cordon affected node: kubectl cordon node-1                  │
│ 3. Scale up healthy nodes: kubectl scale nodes...               │
└─────────────────────────────────────────────────────────────────┘
```

**Result:** ONE notification with complete context, root cause, and remediation steps.

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         CORRELATION ENGINE                                │
│                                                                          │
│  ┌────────────┐    ┌────────────────┐    ┌─────────────────────────┐    │
│  │   Events   │───▶│ Correlation    │───▶│ Open Incidents         │    │
│  │ (from API) │    │ Rules Engine   │    │ (by correlation key)   │    │
│  └────────────┘    └────────────────┘    └─────────────────────────┘    │
│                           │                         │                    │
│                           │ Match rules             │ Correlation window │
│                           │ Generate key            │ expires (5 min)    │
│                           ▼                         ▼                    │
│                    ┌────────────────┐    ┌─────────────────────────┐    │
│                    │ Categories:    │    │ Context Gatherer       │    │
│                    │ • memory       │    │ • Pod logs             │    │
│                    │ • crash        │    │ • K8s events           │    │
│                    │ • image        │    │ • Node conditions      │    │
│                    │ • storage      │    │ • Metrics              │    │
│                    │ • node         │    │ • Error patterns       │    │
│                    │ • scheduling   │    └─────────────────────────┘    │
│                    │ • deployment   │              │                    │
│                    └────────────────┘              ▼                    │
│                                         ┌─────────────────────────┐    │
│                                         │ AI Analyzer            │    │
│                                         │ • Root cause analysis  │    │
│                                         │ • Impact assessment    │    │
│                                         │ • Remediation steps    │    │
│                                         └─────────────────────────┘    │
│                                                    │                    │
│                                                    ▼                    │
│                                         ┌─────────────────────────┐    │
│                                         │ Incident Notifier      │    │
│                                         │ ONE comprehensive      │    │
│                                         │ notification           │    │
│                                         └─────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────────┘
```

## Components

### 1. Correlation Engine (`engine.go`)

The main orchestrator that:
- Receives events from the API
- Matches events to correlation rules
- Groups events into incidents by correlation key
- Manages correlation windows (default: 5 minutes)
- Triggers context gathering and AI analysis when window closes

### 2. Correlation Rules

Rules define how events are grouped:

| Category | Events | Correlation Key |
|----------|--------|-----------------|
| **Memory** | OOMKilled, MemoryPressure, Evicted | By node name |
| **Crash** | CrashLoopBackOff | By owner workload UID |
| **Image** | ImagePullBackOff | By image name (without tag) |
| **Storage** | VolumeFailedMount, PVCProvisionFailed | By storage class |
| **Node** | NodeNotReady, DiskPressure, etc. | By node name |
| **Scheduling** | PodPending, InsufficientMemory | By namespace |
| **Deployment** | DeploymentFailed, JobFailed | By resource UID |

### 3. Context Gatherer (`context.go`)

Enriches incidents with:
- **Pod Logs**: Last 100 lines from affected containers
- **K8s Events**: Related Warning events from the same time window
- **Node Conditions**: Ready, MemoryPressure, DiskPressure, etc.
- **Resource Metrics**: CPU/memory usage (if metrics-server available)
- **Error Patterns**: Detected patterns like OOM, timeout, connection refused

### 4. AI Analyzer (`analyzer.go`)

Generates structured analysis:
- **Root Cause**: What actually went wrong (not just symptoms)
- **Impact Assessment**: User impact, service degradation, data loss
- **Immediate Actions**: Priority-ordered remediation with kubectl commands
- **Long-term Prevention**: Architectural recommendations
- **Monitoring Recommendations**: What to alert on

### 5. Incident Notifier (`notifier.go`)

Sends ONE notification per incident containing:
- Incident summary and metadata
- Root cause (if AI analysis available)
- Timeline of key events
- Immediate action items
- Link to full incident details

## Configuration

```go
config := &correlation.EngineConfig{
    // How long to wait for related events before closing incident
    CorrelationWindow: 5 * time.Minute,
    
    // Minimum events to form an incident (1 = single critical events count)
    MinEventsForIncident: 1,
    
    // Maximum events per incident (prevents runaway correlation)
    MaxEventsPerIncident: 100,
    
    // Timeout for gathering context (logs, metrics)
    ContextGatherTimeout: 30 * time.Second,
    
    // Timeout for AI analysis
    AIAnalysisTimeout: 60 * time.Second,
    
    // Log lines to fetch per pod
    LogLinesPerPod: 100,
    
    // How far back to look for metrics
    MetricsLookback: 15 * time.Minute,
    
    // Enable/disable features
    EnableContextGathering: true,
    EnableAIAnalysis: true,
    
    // Suppress duplicate incidents for this duration
    SuppressDuplicateWindow: 1 * time.Hour,
}
```

## Usage

### Initialize the Engine

```go
// Create dependencies
contextGatherer := correlation.NewK8sContextGatherer(k8sClient, metricsClient, logger, nil)
aiAnalyzer := correlation.NewAIIncidentAnalyzer(aiProvider, logger)
notifier := correlation.NewIncidentNotifier(notifiers, apiBaseURL, logger)
repo := correlation.NewInMemoryIncidentRepository(logger)

// Create engine
engine := correlation.NewEngine(
    config,
    contextGatherer,
    aiAnalyzer,
    notifier,
    repo,
    logger,
)

// Start background processes
engine.Start()
defer engine.Stop()
```

### Process Events

```go
// In your event handler
func (h *EventsHandler) SubmitEvents(w http.ResponseWriter, r *http.Request) {
    // ... validate and store events ...
    
    // Process through correlation engine
    for _, event := range events {
        if err := h.correlationEngine.ProcessEvent(ctx, event); err != nil {
            log.WithError(err).Warn("Failed to correlate event")
        }
    }
}
```

## Data Flow

```
1. Event arrives at API
   ↓
2. Engine.ProcessEvent() called
   ↓
3. Find matching correlation rule
   ↓
4. Generate correlation key (e.g., "memory:node:node-1")
   ↓
5. Find or create incident with that key
   ↓
6. Add event to incident
   ↓
7. Reset/start correlation window timer (5 minutes)
   ↓
   ... more events may arrive and get added ...
   ↓
8. Timer expires → Close incident
   ↓
9. Gather context (logs, metrics, related events)
   ↓
10. AI analysis (root cause, remediation)
   ↓
11. Send ONE notification
   ↓
12. Add to suppression cache (1 hour)
```

## Metrics

The engine exposes Prometheus metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `lumo_incidents_created_total` | Counter | Total incidents created (by category, severity) |
| `lumo_incidents_resolved_total` | Counter | Total incidents resolved (by category, severity) |
| `lumo_events_correlated_total` | Counter | Total events correlated into incidents |
| `lumo_incident_duration_seconds` | Histogram | Duration of incidents (first to last event) |
| `lumo_open_incidents` | Gauge | Current number of open incidents |
| `lumo_correlation_window_duration_seconds` | Histogram | Time spent in correlation window |

## Incident Categories

| Category | Emoji | Common Root Causes |
|----------|-------|-------------------|
| `memory` | 💾 | Memory leaks, insufficient limits, node pressure |
| `crash` | 💥 | Application bugs, dependency failures, config errors |
| `image` | 🖼 | Registry issues, auth failures, missing images |
| `storage` | 💿 | PV provisioning, mount failures, disk full |
| `node` | 🖥 | Hardware issues, kubelet problems, network |
| `scheduling` | 📋 | Resource exhaustion, affinity/taints, quotas |
| `deployment` | 🚀 | Rollout failures, readiness probe failures |
| `network` | 🌐 | DNS issues, service mesh, network policies |

## Example Incident

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "category": "memory",
  "severity": "critical",
  "state": "resolved",
  "title": "Memory Exhaustion: node-1 in production",
  "root_cause": "Memory leak in payment-service v2.3.1 caused node memory pressure",
  "duration": "5m23s",
  "events": 47,
  "affected_resources": [
    {"kind": "Pod", "name": "payment-service-abc", "namespace": "production"},
    {"kind": "Pod", "name": "payment-service-xyz", "namespace": "production"},
    {"kind": "Node", "name": "node-1"}
  ],
  "timeline": [
    {"timestamp": "2025-12-02T12:01:05Z", "type": "oom-killed", "resource": "Pod/payment-service-abc"},
    {"timestamp": "2025-12-02T12:01:30Z", "type": "node-memory-pressure", "resource": "Node/node-1"},
    {"timestamp": "2025-12-02T12:02:00Z", "type": "pod-evicted", "resource": "Pod/cache-service-123"}
  ],
  "ai_analysis": {
    "root_cause": {
      "summary": "Memory leak in payment-service v2.3.1",
      "explanation": "The payment-service deployment from 2 hours ago introduced a memory leak...",
      "confidence": 85
    },
    "immediate_actions": [
      {"title": "Rollback payment-service", "command": "kubectl rollout undo deployment/payment-service -n production"},
      {"title": "Cordon affected node", "command": "kubectl cordon node-1"}
    ]
  },
  "context": {
    "error_patterns": [
      {"pattern": "OutOfMemory", "occurrences": 23},
      {"pattern": "ConnectionRefused", "occurrences": 15}
    ],
    "node_conditions": {
      "node-1": [
        {"type": "Ready", "status": "False"},
        {"type": "MemoryPressure", "status": "True"}
      ]
    }
  }
}
```

## Integration Points

### With Event Handler

The correlation engine integrates with the existing events API handler:

```go
// internal/api/handlers/events.go

func (h *EventsHandler) SubmitEvents(w http.ResponseWriter, r *http.Request) {
    // ... existing event storage logic ...
    
    // NEW: Process through correlation engine
    if h.correlationEngine != nil {
        for _, event := range events {
            _ = h.correlationEngine.ProcessEvent(r.Context(), event)
        }
    }
}
```

### With Notifications

The incident notifier uses the existing notifications package, so all configured channels (Slack, Telegram, Email, Webhook) receive incident notifications.

### With AI Providers

Uses the existing AI adapter pattern, supporting all 5 providers (Anthropic, OpenAI, Gemini, Ollama, OpenRouter).

## Future Enhancements

1. **PostgreSQL Repository**: Persist incidents to database for history and dashboards
2. **Incident API**: REST endpoints for listing, querying, and managing incidents
3. **Runbook Integration**: Link incidents to runbooks and playbooks
4. **Anomaly Detection**: ML-based baseline deviation detection
5. **Auto-Remediation**: Trigger approved remediation actions automatically
6. **SLA Tracking**: Track incident impact on SLO/SLA metrics

## Files

| File | Lines | Purpose |
|------|-------|---------|
| `types.go` | ~400 | Data types, configurations, incident structure |
| `engine.go` | ~550 | Main correlation engine and rules |
| `context.go` | ~400 | Kubernetes context gathering |
| `analyzer.go` | ~500 | AI-powered incident analysis |
| `notifier.go` | ~180 | Incident notification |
| `repository.go` | ~120 | In-memory incident storage |

**Total:** ~2,150 lines of code

## Testing

```bash
# Build the package
go build ./internal/correlation/...

# Run tests (when added)
go test ./internal/correlation/... -v

# Lint
golangci-lint run ./internal/correlation/...
```
