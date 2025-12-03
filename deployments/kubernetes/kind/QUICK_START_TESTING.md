# Quick Start: Testing Multi-Tenant SaaS Deployment

## 30-Second Setup

```bash
cd deployments/kubernetes/kind

# 1. Deploy (takes ~5-10 minutes)
./deploy-saas.sh

# 2. Run a quick test (takes ~60 seconds)
./test-saas-failures.sh --scenario oom-killed

# 3. Watch notifications appear in your Slack/Telegram channel
```

## With Notifications Enabled (Recommended)

```bash
# Option 1: Slack Webhook (simplest)
export LUMO_SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
./deploy-saas.sh

# Option 2: Slack Bot API
export LUMO_SLACK_BOT_TOKEN="xoxb-..."
export LUMO_SLACK_CHANNEL_ID="C..."
./deploy-saas.sh

# Option 3: Telegram
export LUMO_TELEGRAM_BOT_TOKEN="123:ABC..."
export LUMO_TELEGRAM_CHAT_ID="-100..."
./deploy-saas.sh
```

## Run Tests

### Fastest Test (~30 seconds)
```bash
./test-saas-failures.sh --scenario oom-killed
```

### See all available tests
```bash
./test-saas-failures.sh --list
```

### Run all tests
```bash
./test-saas-failures.sh
```

### Test specific tenant
```bash
./test-saas-failures.sh --scenario crash-loop-backoff --tenant globex-ind
```

## What to Expect

### During Deployment
- ✅ Deployment messages sent to Slack/Telegram
- Shows progress: Cluster → Infrastructure → API → Tenants → Agents

### During Failure Test
- 🚨 Event detected by agent (within 45-75 seconds)
- 📊 Event submitted to API
- 💬 Notification sent with event details
- 📋 Incident created and correlated

### In Your Notifications Channel
```
✅ Lumo SaaS Deployment
Step 1/8: Cluster setup complete
Cluster: `lumo-saas-test` | Namespace: `lumo-system` | 2025-12-03 14:56:00 UTC
```

Then when failure occurs:
```
🚨 Lumo Event Analysis
OOMKilled detected in pod test-oom-killed

Severity: CRITICAL
Namespace: tenant-acme-corp
Root Cause: Memory limit exceeded
Recommendation: Increase memory limit or reduce memory usage

Cluster: `lumo-saas-test` | Namespace: `lumo-system`
```

## Verify Everything Works

### Check Events in API
```bash
# Port-forward
kubectl port-forward -n lumo-system svc/lumo-api 8080:8080 &

# View events
curl http://localhost:8080/api/v1/events | jq '.data | length'

# View specific event type
curl http://localhost:8080/api/v1/events?type=oom-killed | jq '.data'
```

### Check Events in Database
```bash
# Get PostgreSQL pod
PG_POD=$(kubectl get pods -n lumo-system -l app=postgres -o name | head -1)

# Count events
kubectl exec -n lumo-system ${PG_POD} -- \
  psql -U lumo -d lumo -c "SELECT type, COUNT(*) FROM events GROUP BY type;"
```

### Check Agent Logs
```bash
# Watch agent detecting events
kubectl logs -n tenant-acme-corp -l app=lumo-agent -f

# Or just the latest
kubectl logs -n tenant-acme-corp -l app=lumo-agent --tail=30
```

## Test Scenarios Available

| Scenario | Time | Severity | What It Tests |
|----------|------|----------|---------------|
| `oom-killed` | 50s | CRITICAL | Memory exhaustion detection |
| `image-pull-backoff` | 65s | HIGH | Image pull failure detection |
| `crash-loop-backoff` | 75s | HIGH | Container crash detection |
| `deployment-failed` | 90s | HIGH | Rollout timeout detection |
| `job-failed` | 60s | CRITICAL | Job failure detection |
| `pending-pod` | 65s | MEDIUM | Resource constraint detection |

## Troubleshooting

### "API not accessible"
```bash
# Check port-forward
kubectl port-forward -n lumo-system svc/lumo-api 8080:8080 &

# Or check API pod logs
kubectl logs -n lumo-system -l app=lumo-api --tail=20
```

### "No agents found"
```bash
# Check agents are running
kubectl get pods -n tenant-acme-corp,tenant-globex-ind,tenant-initech-sol -l app=lumo-agent

# Check agent logs
kubectl logs -n tenant-acme-corp -l app=lumo-agent --tail=50
```

### "Events not appearing"
```bash
# Enable debug output
NOTIFY_DEBUG=true ./test-saas-failures.sh --scenario oom-killed

# Check API is receiving events
curl http://localhost:8080/api/v1/events | jq '.data | length'

# Check database directly
kubectl exec -n lumo-system $(kubectl get pods -n lumo-system -l app=postgres -o name | head -1) -- \
  psql -U lumo -d lumo -c "SELECT COUNT(*) FROM events;"
```

### "Notifications not sending"
```bash
# Verify environment variables
echo $LUMO_SLACK_WEBHOOK_URL
echo $LUMO_SLACK_BOT_TOKEN
echo $LUMO_TELEGRAM_BOT_TOKEN

# Test Slack manually
curl -X POST $LUMO_SLACK_WEBHOOK_URL \
  -H 'Content-type: application/json' \
  -d '{"blocks":[{"type":"section","text":{"type":"mrkdwn","text":"Test message"}}]}'
```

## Next Steps

See [TEST_SAAS_FAILURES.md](./TEST_SAAS_FAILURES.md) for:
- Detailed scenario descriptions
- How to set up Slack/Telegram notifications
- Performance expectations
- Advanced usage and monitoring
- Complete troubleshooting guide

## Clean Up

```bash
# Delete tenant namespaces
kubectl delete namespace tenant-acme-corp tenant-globex-ind tenant-initech-sol

# Delete Lumo system
kubectl delete namespace lumo-system

# Delete kind cluster
kind delete cluster --name lumo-saas-test
```

---

**Duration**: ~15 minutes total (5min deploy + 2min tests + 8min waiting/monitoring)

**Resource Requirements**: 4GB RAM, 2 CPU cores, 10GB disk (kind cluster will use ~3GB)

**Success Criteria**:
- ✅ All 3 tenants created
- ✅ All 3 agents running
- ✅ Events detected and stored
- ✅ Notifications sent to Slack/Telegram
- ✅ Incidents created and correlated
