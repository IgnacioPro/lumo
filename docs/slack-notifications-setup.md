# Slack Notifications Setup Guide

This guide explains how to configure Slack notifications for Lumo to receive alerts when Kubernetes events are detected.

## Overview

The Lumo API server has built-in support for sending notifications to Slack when:
- Kubernetes events are submitted by agents
- Critical or high-severity issues are detected
- AI analysis is completed on events

All notification logic runs on the API server (centralized intelligence model), not on individual agents.

## Prerequisites

1. A Slack workspace where you have permission to add apps
2. Lumo API server running (either locally or in Kubernetes)

## Step 1: Create a Slack Webhook

### Option A: Using Slack App (Recommended)

1. Go to https://api.slack.com/apps
2. Click **"Create New App"** → **"From scratch"**
3. Give it a name like "Lumo Alerts" and select your workspace
4. In the left sidebar, click **"Incoming Webhooks"**
5. Toggle **"Activate Incoming Webhooks"** to **On**
6. Click **"Add New Webhook to Workspace"**
7. Select the channel where you want notifications (e.g., #ops-alerts)
8. Click **"Allow"**
9. Copy the webhook URL (looks like: `https://hooks.slack.com/services/T.../B.../XXX...`)

### Option B: Using Legacy Webhooks

1. Go to https://[your-workspace].slack.com/apps/manage/custom-integrations
2. Click **"Incoming WebHooks"**
3. Click **"Add Configuration"**
4. Select a channel and click **"Add Incoming WebHooks integration"**
5. Copy the Webhook URL

## Step 2: Configure Notifications

### For Local Development

1. **Set the environment variable:**
   ```bash
   export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
   ```

2. **Your config is already updated** at `~/.lumo/config.yaml`:
   ```yaml
   notifications:
     enabled: true
     notifiers:
       - name: slack-alerts
         type: slack
         enabled: true
         webhook_url: ${SLACK_WEBHOOK_URL}
         timeout: 30
   ```

3. **Start the API server:**
   ```bash
   # Make sure PostgreSQL is running
   docker-compose up -d postgres

   # Start the API server
   lumo serve --config ~/.lumo/config.yaml
   ```

### For Kubernetes Deployment

1. **Update the secret with your webhook URL:**
   ```bash
   # Edit the secret file
   vim deployments/kubernetes/kind/manifests/slack-secret.yaml

   # Replace REPLACE_WITH_YOUR_SLACK_WEBHOOK_URL with your actual URL
   ```

2. **Deploy or update the stack:**
   ```bash
   cd deployments/kubernetes/kind

   # If deploying fresh
   ./deploy-to-kind.sh

   # If updating existing deployment
   kubectl apply -f manifests/slack-secret.yaml
   kubectl rollout restart deployment/lumo-api -n lumo-system
   ```

3. **Verify the secret was created:**
   ```bash
   kubectl get secret lumo-secrets -n lumo-system
   kubectl describe secret lumo-secrets -n lumo-system
   ```

## Step 3: Test the Integration

### Option A: Using curl to simulate an event

```bash
# Get a JWT token first (if authentication is enabled)
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r .token)

# Submit a test event
curl -X POST http://localhost:8080/api/v1/events \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "events": [
      {
        "event_type": "pod-crash-loop",
        "severity": "high",
        "resource_kind": "Pod",
        "resource_name": "test-pod",
        "namespace": "default",
        "message": "Pod is in CrashLoopBackOff state. Container exited with code 1.",
        "event_timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
      }
    ]
  }'
```

### Option B: Trigger a real Kubernetes event

If you have the event-driven agent running:

```bash
# Create a pod that will fail
kubectl run test-oom --image=polinux/stress --restart=Never \
  --namespace=default \
  -- stress --vm 1 --vm-bytes 512M --timeout 10s \
  --limits='memory=128Mi'

# This will trigger an OOMKilled event that should be sent to Slack
```

### Option C: Check API server logs

```bash
# Local
lumo serve --config ~/.lumo/config.yaml -v

# Kubernetes
kubectl logs -f deployment/lumo-api -n lumo-system | grep -i notification
```

You should see log messages like:
```
INFO Sending event notifications event_id=<uuid>
INFO Notification sent successfully provider=slack
```

## What Notifications Look Like

Slack notifications include:

- **Severity emoji:** 🔴 Critical, 🟠 High, 🟡 Medium, 🔵 Low
- **Event type:** pod-crash-loop, oom-killed, image-pull-backoff, etc.
- **Resource details:** Kind, name, namespace
- **Timestamp:** When the event occurred
- **Message:** Detailed description
- **AI Analysis:** (if enabled) Root cause and remediation steps

Example:
```
🟠 Kubernetes Event: pod-crash-loop

Severity: high
Resource: Pod/test-pod
Namespace: default
Time: 2025-11-25T19:30:00Z

Message:
Pod is in CrashLoopBackOff state. Container exited with code 1.

AI Analysis:
The pod is failing repeatedly due to an application error...
```

## Filtering Events

By default, notifications are sent for **high-priority events only**. This is determined by:
- Severity: `high` or `critical`
- Event importance (defined in event handler logic)

To customize this, modify the `sendEventNotifications` logic in `internal/api/handlers/events.go`.

## Multiple Notification Channels

You can add multiple notification providers (Telegram, Discord, Email, etc.) by adding more entries to the `notifiers` list:

```yaml
notifications:
  enabled: true
  notifiers:
    - name: slack-critical
      type: slack
      enabled: true
      webhook_url: ${SLACK_WEBHOOK_URL}
      timeout: 30

    - name: telegram-ops
      type: telegram
      enabled: true
      bot_token: ${TELEGRAM_BOT_TOKEN}
      chat_id: ${TELEGRAM_CHAT_ID}
      timeout: 30

    - name: email-oncall
      type: email
      enabled: true
      smtp_host: smtp.gmail.com
      smtp_port: 587
      smtp_user: alerts@example.com
      smtp_pass: ${SMTP_PASSWORD}
      from: lumo@example.com
      to:
        - oncall@example.com
      use_tls: true
      timeout: 30
```

See `configs/notifications.example.yaml` for complete configuration examples.

## Troubleshooting

### No notifications received

1. **Check API server logs:**
   ```bash
   kubectl logs -f deployment/lumo-api -n lumo-system | grep -E "(notification|notifier)"
   ```

2. **Verify configuration:**
   ```bash
   kubectl get configmap lumo-config -n lumo-system -o yaml | grep -A 10 notifications
   ```

3. **Check secret:**
   ```bash
   kubectl get secret lumo-secrets -n lumo-system -o jsonpath='{.data.slack-webhook-url}' | base64 -d
   ```

4. **Test webhook directly:**
   ```bash
   curl -X POST "${SLACK_WEBHOOK_URL}" \
     -H "Content-Type: application/json" \
     -d '{"text":"Test from Lumo"}'
   ```

### Notifications sent but not appearing

- Check that the Slack webhook is still valid (webhooks can be revoked)
- Verify the channel still exists
- Check Slack app permissions

### "Failed to initialize notifier" error

- Verify the `SLACK_WEBHOOK_URL` environment variable is set correctly
- Check that the webhook URL format is correct (starts with `https://hooks.slack.com/`)
- Ensure the secret is properly mounted in the pod

## Security Notes

- **Never commit webhook URLs to version control**
- Use environment variables or Kubernetes secrets
- Rotate webhook URLs periodically
- Restrict webhook channel permissions appropriately
- Consider using Slack's audit logs to monitor webhook usage

## Architecture Notes

- **Centralized Intelligence:** All notifications originate from the API server
- **Agents are reporters:** Agents submit events via HTTP POST to `/api/v1/events`
- **Async processing:** Events are processed asynchronously (AI + notifications)
- **Circuit breakers:** Built-in resilience with automatic retries and circuit breakers
- **Multi-channel:** Single event can trigger notifications across multiple channels

For more details, see:
- `internal/api/handlers/events.go` - Event submission and notification logic
- `internal/notifications/` - Notification provider implementations
- `internal/api/router.go` - Notification initialization
