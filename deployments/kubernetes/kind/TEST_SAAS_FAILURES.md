# Multi-Tenant SaaS - Failure Scenario Testing

This guide shows how to test the multi-tenant SaaS deployment with various failure scenarios and verify that:
1. Event-driven agents detect failures in real-time
2. Events are submitted to the API server
3. Notifications are sent (Slack/Telegram) if configured
4. Incidents are created and correlated
5. Postmortems are generated

## Quick Start

### 1. Deploy the SaaS environment

```bash
# With Slack webhook notifications
export LUMO_SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"

# OR with Slack Bot API
export LUMO_SLACK_BOT_TOKEN="xoxb-..."
export LUMO_SLACK_CHANNEL_ID="C..."

# OR with Telegram
export LUMO_TELEGRAM_BOT_TOKEN="123:ABC..."
export LUMO_TELEGRAM_CHAT_ID="-100..."

# Deploy
./deploy-saas.sh
```

### 2. Run a failure scenario

```bash
# List available scenarios
./test-saas-failures.sh --list

# Run ImagePullBackOff test (fastest, ~30 seconds)
./test-saas-failures.sh --scenario image-pull-backoff

# Run OOMKilled test
./test-saas-failures.sh --scenario oom-killed

# Run all tests sequentially
./test-saas-failures.sh

# Test against a specific tenant
./test-saas-failures.sh --scenario crash-loop-backoff --tenant globex-ind
```

## Available Failure Scenarios

### Pod Failures (45-75s detection latency)

**1. ImagePullBackOff** - Invalid container image
```bash
./test-saas-failures.sh --scenario image-pull-backoff
```
- Creates pod with non-existent image
- Agent detects pull failures
- Event: `image-pull-backoff` (severity: HIGH)
- Expected latency: ~65s (45s debounce + image pull retries)

**2. CrashLoopBackOff** - Container exits immediately
```bash
./test-saas-failures.sh --scenario crash-loop-backoff
```
- Creates pod that crashes on startup
- Agent detects restart failures
- Event: `crash-loop-backoff` (severity: HIGH)
- Expected latency: ~75s (45s debounce + multiple restarts)

**3. OOMKilled** - Memory limit exceeded
```bash
./test-saas-failures.sh --scenario oom-killed
```
- Creates pod with 16Mi limit, exits with OOMKilled
- Agent detects memory exhaustion
- Event: `oom-killed` (severity: CRITICAL)
- Expected latency: ~50s (45s debounce + OOM detection)

**4. Pod Pending** - Insufficient resources
```bash
./test-saas-failures.sh --scenario pending-pod
```
- Creates pod with 512Gi memory request
- Pod stays pending (not enough cluster resources)
- Event: `pod-pending` (severity: MEDIUM)
- Expected latency: ~65-120s (depends on pending duration)

### Workload Failures (60-90s detection latency)

**5. Deployment Failed** - Rollout timeout
```bash
./test-saas-failures.sh --scenario deployment-failed
```
- Creates deployment with 10s progress deadline
- Cannot pull image, exceeds deadline
- Event: `deployment-failed` (severity: HIGH)
- Expected latency: ~90s (10s deadline + 45s debounce)

**6. Job Failed** - Backoff limit exceeded
```bash
./test-saas-failures.sh --scenario job-failed
```
- Creates job with backoff limit 2
- Job exits with code 1, retries exhausted
- Event: `job-failed` (severity: CRITICAL)
- Expected latency: ~60s (retries + 45s debounce)

### Run All Tests

```bash
./test-saas-failures.sh
```

This runs all scenarios sequentially and reports:
- Number of tests passed/failed
- Event detection times
- Notification delivery status

## Testing Notifications

### Prerequisites

1. **Slack Webhook**
   - Create incoming webhook: https://api.slack.com/apps
   - New App → From scratch → Enable Webhooks
   - Copy webhook URL
   - Set: `export LUMO_SLACK_WEBHOOK_URL="https://hooks.slack.com/..."`

2. **Slack Bot API**
   - Create bot: https://api.slack.com/apps → Create New App
   - Install bot to workspace
   - Get Bot Token: `xoxb-...`
   - Get Channel ID: Right-click channel → Copy member ID
   - Set: `export LUMO_SLACK_BOT_TOKEN="xoxb-..."` `export LUMO_SLACK_CHANNEL_ID="C..."`

3. **Telegram**
   - Create bot: Message @BotFather on Telegram
   - Get bot token: `123:ABC...`
   - Get chat ID: Send message to bot, then run:
     ```bash
     curl "https://api.telegram.org/bot<TOKEN>/getUpdates"
     ```
   - Set: `export LUMO_TELEGRAM_BOT_TOKEN="123:ABC..."` `export LUMO_TELEGRAM_CHAT_ID="..."`

### Deploy with notifications

```bash
# Slack Webhook
LUMO_SLACK_WEBHOOK_URL="https://hooks.slack.com/..." ./deploy-saas.sh

# Slack Bot
LUMO_SLACK_BOT_TOKEN="xoxb-..." LUMO_SLACK_CHANNEL_ID="C..." ./deploy-saas.sh

# Telegram
LUMO_TELEGRAM_BOT_TOKEN="123:ABC..." LUMO_TELEGRAM_CHAT_ID="-100..." ./deploy-saas.sh

# Combined (all three channels)
export LUMO_SLACK_WEBHOOK_URL="https://hooks.slack.com/..."
export LUMO_SLACK_BOT_TOKEN="xoxb-..."
export LUMO_SLACK_CHANNEL_ID="C..."
export LUMO_TELEGRAM_BOT_TOKEN="123:ABC..."
export LUMO_TELEGRAM_CHAT_ID="-100..."
./deploy-saas.sh
```

### Monitor notifications

When you run failure scenarios, you should see notifications appear in:
- Slack channel (if webhook or bot configured)
- Telegram chat (if bot configured)

Example Slack message:
```
✅ Lumo SaaS Deployment
Step 7/8: Tenant agents deployed

Cluster: `lumo-saas-test` | Namespace: `lumo-system` | 2025-12-03 14:56:00 UTC
```

When failures occur:
```
🚨 Lumo Event Analysis
ImagePullBackOff detected

Event: test-image-pull-backoff pod in tenant-acme-corp
Severity: HIGH
Root Cause: Image not found at registry
Recommendation: Fix image URL or registry credentials

Cluster: `lumo-saas-test` | Namespace: `lumo-system`
```

## Verifying Event Flow

### 1. Check events in Slack/Telegram

Look for event notifications with severity emoji:
- ✅ Success notifications during deployment
- 🚨 Critical/High severity events when failures occur
- ⚠️ Warning for medium/low severity events

### 2. Check events in API

```bash
# Port-forward API
kubectl port-forward -n lumo-system svc/lumo-api 8080:8080 &

# Get all events
curl http://localhost:8080/api/v1/events | jq .

# Get specific event type
curl http://localhost:8080/api/v1/events?type=oom-killed | jq .

# Get events by tenant
curl http://localhost:8080/api/v1/events?tenant_id=<TENANT_ID> | jq .
```

### 3. Check events in Database

```bash
# Get PostgreSQL pod
PG_POD=$(kubectl get pods -n lumo-system -l app=postgres -o name | head -1)

# View events table
kubectl exec -n lumo-system ${PG_POD} -- \
  psql -U lumo -d lumo -c "SELECT id, type, severity, metadata FROM events LIMIT 10;"

# Count events by type
kubectl exec -n lumo-system ${PG_POD} -- \
  psql -U lumo -d lumo -c "SELECT type, COUNT(*) FROM events GROUP BY type;"

# View incidents
kubectl exec -n lumo-system ${PG_POD} -- \
  psql -U lumo -d lumo -c "SELECT id, category, severity, status FROM incidents LIMIT 10;"
```

### 4. Check agent logs

```bash
# View agent logs for a tenant
kubectl logs -n tenant-acme-corp -l app=lumo-agent -f

# Look for event detection
grep -i "event\|oom\|crash\|image" <(kubectl logs -n tenant-acme-corp -l app=lumo-agent --tail=100)

# Watch real-time
kubectl logs -n tenant-acme-corp -l app=lumo-agent -f --timestamps=true
```

### 5. Check API health

```bash
# API health
curl http://localhost:8080/api/v1/health | jq .

# Agent registration
curl http://localhost:8080/api/v1/agents | jq '.data[] | {id, hostname, status, last_heartbeat}'

# Incidents
curl http://localhost:8080/api/v1/incidents | jq '.data[] | {id, category, severity, status}'
```

## End-to-End Test Procedure

This verifies the complete pipeline from failure to notification:

```bash
# 1. Deploy with notifications
export LUMO_SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
./deploy-saas.sh

# 2. Wait for all agents to be ready
sleep 30
kubectl get pods -n tenant-acme-corp,tenant-globex-ind,tenant-initech-sol -l app=lumo-agent

# 3. Run OOMKilled test (fastest, ~50s)
./test-saas-failures.sh --scenario oom-killed

# 4. Watch notifications appear in Slack/Telegram

# 5. Check API for events
curl http://localhost:8080/api/v1/events?type=oom-killed | jq .

# 6. Check database for incidents
PG_POD=$(kubectl get pods -n lumo-system -l app=postgres -o name | head -1)
kubectl exec -n lumo-system ${PG_POD} -- \
  psql -U lumo -d lumo -c "SELECT * FROM incidents WHERE category = 'memory';"

# 7. Clean up
kubectl delete pod test-oom-killed -n tenant-acme-corp --ignore-not-found
```

## Troubleshooting

### Events not appearing

1. Check agent is running:
   ```bash
   kubectl get pods -n tenant-acme-corp -l app=lumo-agent
   ```

2. Check agent logs:
   ```bash
   kubectl logs -n tenant-acme-corp -l app=lumo-agent --tail=50
   ```

3. Check event-driven mode is enabled:
   ```bash
   kubectl exec -n tenant-acme-corp -it $(kubectl get pods -n tenant-acme-corp -l app=lumo-agent -o name | head -1) -- env | grep EVENT_DRIVEN
   ```

4. Verify API endpoint:
   ```bash
   kubectl get configmap -n tenant-acme-corp lumo-agent-config -o yaml | grep endpoint
   ```

### Notifications not sent

1. Check notification configuration:
   ```bash
   env | grep LUMO_SLACK\|LUMO_TELEGRAM
   ```

2. Check API logs for notification errors:
   ```bash
   kubectl logs -n lumo-system -l app=lumo-api --tail=50 | grep -i notification\|slack\|telegram
   ```

3. Enable debug mode:
   ```bash
   export NOTIFY_DEBUG=true
   ./test-saas-failures.sh --scenario image-pull-backoff
   ```

4. Test notification manually:
   ```bash
   curl -X POST https://slack.com/api/chat.postMessage \
     -H "Authorization: Bearer $LUMO_SLACK_BOT_TOKEN" \
     -H "Content-type: application/json" \
     -d '{"channel":"'${LUMO_SLACK_CHANNEL_ID}'","text":"Test message"}'
   ```

### Test timeout

If tests timeout waiting for events:

1. Increase poll time:
   ```bash
   POLL_MAX_ATTEMPTS=120 ./test-saas-failures.sh --scenario oom-killed
   ```

2. Check debounce settings:
   ```bash
   kubectl get configmap -n tenant-acme-corp lumo-agent-config -o yaml | grep debounce
   ```

3. Verify agent is connected to API:
   ```bash
   curl http://localhost:8080/api/v1/agents | jq '.data | length'
   ```

## Performance Expectations

| Scenario | Detection Time | Notes |
|----------|---|---|
| ImagePullBackOff | ~65s | Image pull retries take time |
| CrashLoopBackOff | ~75s | Multiple restart cycles |
| OOMKilled | ~50s | Fastest detection |
| Pod Pending | ~65-120s | Depends on pending duration |
| Deployment Failed | ~90s | Includes progress deadline |
| Job Failed | ~60s | Retry backoff delays |

The latency includes:
- 0-5s: Failure occurs
- 0-10s: Agent detects via watcher
- 45s: Debounce window (configurable)
- 5-10s: API processing + notification

Total: ~50-120 seconds depending on scenario and debounce settings

## Advanced Usage

### Test with custom tenant

```bash
./test-saas-failures.sh --scenario oom-killed --tenant globex-ind
```

### Run specific test only

```bash
./test-saas-failures.sh --scenario crash-loop-backoff
```

### Run all tests with verbose output

```bash
./test-saas-failures.sh 2>&1 | tee test-results.log
```

### Test multiple tenants

```bash
for tenant in acme-corp globex-ind initech-sol; do
  echo "Testing $tenant..."
  ./test-saas-failures.sh --scenario oom-killed --tenant $tenant
  sleep 10
done
```

## Notes

- Tests use the `critical: "true"` label to bypass debouncing for immediate notification
- Each test cleans up after itself (deletes test resources)
- Tests run sequentially; if one fails, subsequent tests still run
- Event detection latency varies based on cluster load and network conditions
- Notifications are sent asynchronously and may take 1-5 seconds to appear

## See Also

- [Multi-Tenant SaaS Deployment](./FULL_STACK_DEPLOYMENT.md)
- [Event-Driven Architecture](../../EVENT_DRIVEN_IMPLEMENTATION.md)
- [Incident Correlation Engine](../../PHASE_19_INCIDENT_CORRELATION.md)
