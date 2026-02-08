# Customer Onboarding Guide

> **Audience:** Lumo operators onboarding new customers
> **Last Updated:** 2025-11-30

This guide walks through the complete process of onboarding a new customer to Lumo.

---

## Prerequisites

Before onboarding a customer, ensure:

- [ ] Lumo API is deployed and accessible
- [ ] PostgreSQL and Redis are running
- [ ] Admin JWT token available
- [ ] Customer has provided:
  - Company name
  - Contact email
  - Desired plan (trial/starter/pro/enterprise)
  - Approximate number of agents needed

---

## Step 1: Create Tenant

Create a tenant record for the customer:

```bash
# Set your admin token
export ADMIN_TOKEN="your-admin-jwt-token"
export API_URL="https://api.lumo.io"

# Create tenant
curl -X POST "$API_URL/api/v1/admin/tenants" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corporation",
    "slug": "acme-corp",
    "plan": "pro",
    "contact_email": "devops@acme.com",
    "max_agents": 100,
    "max_events_per_day": 100000
  }' | jq .
```

**Expected Response:**

```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "name": "Acme Corporation",
  "slug": "acme-corp",
  "plan": "pro",
  "status": "active",
  "max_agents": 100,
  "max_events_per_day": 100000,
  "isolation_tier": "shared",
  "created_at": "2025-11-30T12:00:00Z"
}
```

**Note:** Save the tenant `id` for subsequent steps.

---

## Step 2: Provision Agent(s)

For each Kubernetes cluster the customer wants to monitor:

```bash
export TENANT_ID="123e4567-e89b-12d3-a456-426614174000"

curl -X POST "$API_URL/api/v1/admin/tenants/$TENANT_ID/provision-agent" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "hostname": "acme-production",
    "cluster_name": "prod-us-east-1",
    "namespace": "lumo-system"
  }' | jq .
```

**Expected Response:**

```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "agent_token": "<agent-token>",
  "api_endpoint": "https://api.lumo.io",
  "kubernetes_manifest": "---\napiVersion: v1\nkind: Namespace\nmetadata:\n  name: lumo-system\n---\napiVersion: v1\nkind: Secret\n..."
}
```

---

## Step 3: Send Deployment Instructions to Customer

Provide the customer with:

### Option A: Pre-generated Manifest

Save and send the manifest from the provisioning response:

```bash
# Extract manifest from response
echo "$RESPONSE" | jq -r '.kubernetes_manifest' > acme-lumo-agent.yaml

# Send to customer
# Email/Slack/etc with the file
```

### Option B: Quick Deploy Script

Send the customer this script with their token:

```bash
#!/bin/bash
# Lumo Agent Deployment Script for Acme Corporation

AGENT_TOKEN="<agent-token>"
API_ENDPOINT="https://api.lumo.io"

# Create namespace
kubectl create namespace lumo-system --dry-run=client -o yaml | kubectl apply -f -

# Create secret with agent token
kubectl create secret generic lumo-agent-credentials \
  --from-literal=token="$AGENT_TOKEN" \
  -n lumo-system \
  --dry-run=client -o yaml | kubectl apply -f -

# Create ConfigMap
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: lumo-agent-config
  namespace: lumo-system
data:
  config.yaml: |
    api:
      endpoint: $API_ENDPOINT
    agent:
      mode: event-driven
      enabled_checks:
        - kubernetes
      event_driven:
        enabled: true
        debounce_window: 45s
        max_debounce_window: 3m
EOF

# Deploy agent
kubectl apply -f https://raw.githubusercontent.com/your-org/lumo/main/deployments/kubernetes/base/deployment-agent.yaml
```

---

## Step 4: Verify Deployment

### From Your Side (API)

Check the agent registered:

```bash
curl "$API_URL/api/v1/admin/tenants/$TENANT_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.agents'
```

Expected:
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "hostname": "acme-production",
    "status": "online",
    "last_heartbeat_at": "2025-11-30T12:05:00Z"
  }
]
```

### From Customer Side

Have the customer verify:

```bash
# Check pods are running
kubectl -n lumo-system get pods

# Check logs
kubectl -n lumo-system logs -l app=lumo-agent --tail=50

# Look for successful registration
# Expected log: "Agent registered successfully"
# Expected log: "Event-driven monitoring started"
```

---

## Step 5: Configure Notifications (Optional)

If the customer wants notifications, update their tenant settings:

```bash
curl -X PUT "$API_URL/api/v1/admin/tenants/$TENANT_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "settings": {
      "notifications": {
        "slack": {
          "enabled": true,
          "webhook_url": "<slack-webhook-url>"
        },
        "email": {
          "enabled": true,
          "recipients": ["oncall@acme.com"]
        }
      },
      "ai_analysis": {
        "enabled": true,
        "provider": "anthropic"
      }
    }
  }'
```

---

## Step 6: Provide Portal Access

Generate a user token for the customer's admin:

```bash
curl -X POST "$API_URL/api/v1/admin/tenants/$TENANT_ID/users" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "name": "Acme Admin",
    "role": "owner"
  }'
```

Send the customer:
- Portal URL: `https://portal.lumo.io`
- Their login credentials (or SSO setup instructions)
- Link to documentation

---

## Quick Reference: Customer Communication Template

```
Subject: Your Lumo Account is Ready

Hi [Customer Name],

Your Lumo account has been created and is ready to use.

Account Details:
- Organization: Acme Corporation
- Plan: Pro (100 agents, 100K events/day)
- Tenant ID: 123e4567-e89b-12d3-a456-426614174000

Getting Started:

1. Deploy the Lumo Agent to your Kubernetes cluster:
   [Attached: acme-lumo-agent.yaml]
   
   kubectl apply -f acme-lumo-agent.yaml

2. Verify the agent is running:
   kubectl -n lumo-system get pods
   kubectl -n lumo-system logs -l app=lumo-agent

3. Access your dashboard:
   https://portal.lumo.io
   Login: admin@acme.com

Documentation:
- Getting Started: https://docs.lumo.io/getting-started
- Event Types: https://docs.lumo.io/event-types
- Troubleshooting: https://docs.lumo.io/troubleshooting

Need help? Reply to this email or reach us at support@lumo.io.

Best,
The Lumo Team
```

---

## Troubleshooting Onboarding Issues

### Agent Not Appearing Online

1. Check agent logs:
   ```bash
   kubectl -n lumo-system logs -l app=lumo-agent --tail=100
   ```

2. Common issues:
   - **Connection refused**: Check API endpoint in ConfigMap
   - **401 Unauthorized**: Token may be expired or invalid
   - **TLS errors**: Ensure cluster can reach API endpoint

3. Verify network connectivity:
   ```bash
   kubectl -n lumo-system run curl --rm -it --image=curlimages/curl -- \
     curl -v https://api.lumo.io/api/v1/health
   ```

### Events Not Being Detected

1. Check RBAC permissions:
   ```bash
   kubectl auth can-i list pods --as=system:serviceaccount:lumo-system:lumo-agent -n default
   ```

2. Verify event-driven mode is enabled:
   ```bash
   kubectl -n lumo-system get configmap lumo-agent-config -o yaml
   ```

3. Check for errors in agent logs related to informers

### Rate Limit Errors

If customer hits rate limits during initial setup:

```bash
# Temporarily increase limits
curl -X PUT "$API_URL/api/v1/admin/tenants/$TENANT_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"max_events_per_day": 200000}'
```

---

## Onboarding Checklist

- [ ] Tenant created with correct plan
- [ ] Agent token generated
- [ ] Manifest sent to customer
- [ ] Agent deployed and showing online
- [ ] Test event generated (optional: `kubectl delete pod test-pod`)
- [ ] Notifications configured (if requested)
- [ ] Portal access provided
- [ ] Welcome email sent
- [ ] Billing setup (Stripe subscription)

---

## Related Documentation

- [Multi-Tenant Architecture](multi-tenant-architecture.md) - How isolation works
- [Getting Started](getting-started.md) - Customer-facing deployment guide
- [Event Types](event-types.md) - What Lumo detects
