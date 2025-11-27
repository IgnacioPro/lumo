# Example 7: Querying Kubernetes Events

This example demonstrates how to query and analyze Kubernetes events stored in the database using the `lumo events` command.

## What You'll Learn

- Query events from the PostgreSQL database
- Filter events by severity, type, namespace, and time
- Understand event types and severity levels
- Export events in different formats (table, JSON, YAML)
- Integrate event queries into scripts and workflows

## Prerequisites

- Lumo configured with database access
- Kubernetes agents deployed and reporting events
- PostgreSQL database with events table populated

## Basic Usage

### View Recent Events

```bash
# Show last 20 events (default)
lumo events

# Show last 50 events
lumo events --limit 50

# Show last 100 events
lumo events --limit 100
```

**Example Output:**
```
┌────────────────────┬───────────────────────┬──────────┬───────────────────────┬─────────────────────────────────────┐
│ TIME               │ TYPE                  │ SEVERITY │ RESOURCE              │ AI ANALYSIS                         │
├────────────────────┼───────────────────────┼──────────┼───────────────────────┼─────────────────────────────────────┤
│ 2025-11-27 15:42   │ oom-killed            │ critical │ nginx-pod             │ Container exceeded 256Mi limit...   │
│ 2025-11-27 15:38   │ crash-loop-backoff    │ high     │ api-server            │ Exit code 1, check logs for...      │
│ 2025-11-27 15:30   │ image-pull-backoff    │ high     │ frontend-v2           │ Image not found: myrepo/front...    │
│ 2025-11-27 15:22   │ pvc-provision-failed  │ medium   │ data-pvc              │ StorageClass 'fast' not found...    │
└────────────────────┴───────────────────────┴──────────┴───────────────────────┴─────────────────────────────────────┘

Showing 4 of 4 events
```

## Filtering Events

### By Severity

```bash
# Critical events only
lumo events --severity critical

# High and critical events
lumo events --severity high,critical

# All severities (default)
lumo events --severity low,medium,high,critical
```

**Severity Levels:**
| Level | Description | Examples |
|-------|-------------|----------|
| `critical` | Immediate action required | OOMKilled, Node NotReady, Pod Evicted |
| `high` | Needs attention soon | CrashLoopBackOff, ImagePullBackOff, Deployment Failed |
| `medium` | Should investigate | PVC Provision Failed, Pod Pending |
| `low` | Informational | Minor warnings |

### By Event Type

```bash
# OOMKilled events
lumo events --type oom-killed

# CrashLoopBackOff events
lumo events --type crash-loop-backoff

# Image pull failures
lumo events --type image-pull-backoff

# Deployment failures
lumo events --type deployment-failed

# Job failures
lumo events --type job-failed

# Volume issues
lumo events --type pvc-provision-failed
lumo events --type volume-failed-mount

# Node issues
lumo events --type node-not-ready
```

**Available Event Types:**
- `oom-killed` - Container killed due to memory limit
- `pod-evicted` - Pod evicted from node
- `crash-loop-backoff` - Container in crash loop
- `image-pull-backoff` - Failed to pull container image
- `deployment-failed` - Deployment progress deadline exceeded
- `job-failed` - Job exceeded backoff limit
- `pvc-provision-failed` - PersistentVolumeClaim provisioning failed
- `volume-failed-mount` - Volume mount failure
- `node-not-ready` - Node became NotReady
- `pod-pending` - Pod stuck in Pending state

### By Namespace

```bash
# Events from specific namespace
lumo events --namespace production

# Events from kube-system
lumo events --namespace kube-system

# Combine with other filters
lumo events --namespace production --severity critical
```

### By Resource Kind

```bash
# Pod events only
lumo events --resource Pod

# Deployment events
lumo events --resource Deployment

# PersistentVolumeClaim events
lumo events --resource PersistentVolumeClaim
```

### By Time Range

```bash
# Last 24 hours
lumo events --since 24h

# Last 7 days
lumo events --since 7d

# Last 1 hour
lumo events --since 1h

# Last 30 minutes
lumo events --since 30m
```

## Output Formats

### Table (Default)

Human-readable table format:

```bash
lumo events --format table
```

### JSON

Machine-readable JSON for scripting:

```bash
lumo events --format json
```

**Example JSON Output:**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "event_type": "oom-killed",
    "severity": "critical",
    "namespace": "production",
    "resource_kind": "Pod",
    "resource_name": "nginx-pod",
    "message": "Container 'nginx' was OOMKilled",
    "detected_at": "2025-11-27T15:42:00Z",
    "ai_analysis": "Container exceeded memory limit of 256Mi. Consider increasing memory limit or optimizing application memory usage.",
    "metadata": {
      "container_name": "nginx",
      "exit_code": 137,
      "restart_count": 3
    }
  }
]
```

### YAML

YAML format for configuration workflows:

```bash
lumo events --format yaml
```

## Display Options

### Hide AI Analysis

For compact output without AI analysis column:

```bash
lumo events --no-analysis
```

### Show Metadata

Include event metadata in table output:

```bash
lumo events --show-metadata
```

## Advanced Usage

### Scripting Examples

#### Alert on Critical Events

```bash
#!/bin/bash
# Check for critical events in last hour

CRITICAL_COUNT=$(lumo events --severity critical --since 1h --format json | jq length)

if [ "$CRITICAL_COUNT" -gt 0 ]; then
    echo "ALERT: $CRITICAL_COUNT critical events in the last hour!"
    lumo events --severity critical --since 1h
    # Send to Slack, PagerDuty, etc.
fi
```

#### Daily Event Summary

```bash
#!/bin/bash
# Generate daily event summary

echo "=== Daily Event Summary $(date +%Y-%m-%d) ==="
echo ""
echo "Critical Events:"
lumo events --severity critical --since 24h --format table
echo ""
echo "High Severity Events:"
lumo events --severity high --since 24h --format table
echo ""
echo "Event Counts by Type:"
lumo events --since 24h --format json | jq 'group_by(.event_type) | map({type: .[0].event_type, count: length})'
```

#### Export Events for Analysis

```bash
# Export last 7 days of events to JSON
lumo events --since 7d --limit 1000 --format json > events-weekly.json

# Analyze with jq
cat events-weekly.json | jq 'group_by(.severity) | map({severity: .[0].severity, count: length})'

# Find most common event types
cat events-weekly.json | jq 'group_by(.event_type) | map({type: .[0].event_type, count: length}) | sort_by(.count) | reverse'
```

### Integration with Monitoring

#### Prometheus Alert Rule

```yaml
# prometheus-rules.yaml
groups:
  - name: lumo-events
    rules:
      - alert: HighCriticalEventRate
        expr: increase(lumo_events_processed_total{severity="critical"}[1h]) > 5
        for: 5m
        labels:
          severity: page
        annotations:
          summary: "High rate of critical Kubernetes events"
          description: "More than 5 critical events in the last hour"
```

#### Grafana Dashboard Query

```bash
# Get event counts for Grafana
lumo events --since 24h --format json | \
  jq 'group_by(.event_type) | map({type: .[0].event_type, count: length})'
```

### Cron Jobs

```bash
# crontab -e

# Hourly critical event check
0 * * * * /usr/local/bin/lumo events --severity critical --since 1h --format json | jq length > /var/log/lumo/critical-count.txt

# Daily event export
0 0 * * * /usr/local/bin/lumo events --since 24h --format json > /var/log/lumo/events-$(date +\%Y\%m\%d).json

# Weekly summary report
0 9 * * 1 /usr/local/bin/lumo events --since 7d --limit 500 | mail -s "Weekly K8s Events" ops@example.com
```

## Real-World Examples

### Example 1: Incident Investigation

```bash
# 1. Check what happened in the last hour
lumo events --since 1h

# 2. Focus on critical events
lumo events --severity critical --since 1h

# 3. Look at specific namespace
lumo events --namespace production --since 1h

# 4. Check OOMKilled events specifically
lumo events --type oom-killed --since 24h

# 5. Export for detailed analysis
lumo events --since 1h --format json > incident-events.json
```

### Example 2: Capacity Planning

```bash
# Check OOM events over the past week
lumo events --type oom-killed --since 7d --format json | \
  jq 'group_by(.resource_name) | map({pod: .[0].resource_name, oom_count: length}) | sort_by(.oom_count) | reverse'

# Check PVC provisioning failures
lumo events --type pvc-provision-failed --since 30d --format json | \
  jq 'length'
```

### Example 3: Deployment Validation

```bash
# After deploying, check for issues
sleep 300  # Wait 5 minutes for events to propagate
lumo events --namespace my-app --since 10m --severity high,critical

# If no critical events, deployment is likely healthy
if [ $(lumo events --namespace my-app --since 10m --severity critical --format json | jq length) -eq 0 ]; then
    echo "Deployment healthy - no critical events"
else
    echo "WARNING: Critical events detected after deployment"
    lumo events --namespace my-app --since 10m --severity critical
fi
```

## Troubleshooting

### "Failed to connect to database"

```bash
# Check database configuration
cat ~/.lumo/config.yaml | grep -A 5 database

# Verify PostgreSQL is running
docker-compose ps

# Test connection
psql -h localhost -U lumo -d lumo -c "SELECT COUNT(*) FROM events;"
```

### "No events found"

```bash
# Check if agents are running
kubectl get pods -n lumo-system -l app=lumo-agent

# Check agent logs for event processing
kubectl logs -n lumo-system -l app=lumo-agent | grep -i event

# Verify events are being stored
lumo events --limit 100 --format json | jq length
```

### Events Not Updated

```bash
# Check agent connectivity to API
kubectl logs -n lumo-system -l app=lumo-agent | grep -i "submit\|post\|api"

# Check API server logs
kubectl logs -n lumo-system -l app=lumo-api | grep -i event

# Verify database has recent entries
psql -h localhost -U lumo -d lumo -c "SELECT MAX(detected_at) FROM events;"
```

## Next Steps

- **[Example 5: K8s Agent Deployment](../05-agent-deployment-k8s/)** - Set up event-driven agents
- **[Example 3: AI Analysis](../03-ai-analysis/)** - Understand AI-powered event analysis
- **[Event-Driven Implementation](../../EVENT_DRIVEN_IMPLEMENTATION.md)** - Technical architecture details

## Additional Resources

- [API Events Endpoint](../../docs/api-reference.md#events)
- [Event Types Reference](../../docs/event-types.md)
- [Database Schema](../../docs/database-schema.md)
