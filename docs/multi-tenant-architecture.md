# Multi-Tenant SaaS Architecture

> **Status:** Implemented (Phase 18)
> **Last Updated:** 2025-11-30

This document explains the practical aspects of Lumo's multi-tenant architecture for operators and developers.

---

## Overview

Lumo operates as a SaaS platform where:

- **You host** the Lumo API (control plane)
- **Customers deploy** Lumo Agents in their Kubernetes clusters (data plane)
- **Data is isolated** per customer (tenant)

```
┌─────────────────────────────────────────────────────────────┐
│                    YOUR INFRASTRUCTURE                       │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Lumo API  ──────────────────────────────────────────  │ │
│  │  PostgreSQL (tenant-aware) │ Redis │ AI Providers      │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
         ▲                    ▲                    ▲
         │ HTTPS              │ HTTPS              │ HTTPS
         │                    │                    │
    ┌─────────┐          ┌─────────┐          ┌─────────┐
    │ Acme    │          │ Globex  │          │ Initech │
    │ Agent   │          │ Agent   │          │ Agent   │
    │ Token:A │          │ Token:B │          │ Token:C │
    └─────────┘          └─────────┘          └─────────┘
    Customer A           Customer B           Customer C
    K8s Cluster          K8s Cluster          K8s Cluster
```

---

## Key Concepts

### Tenant

A tenant represents a customer organization. Each tenant has:

| Attribute | Description |
|-----------|-------------|
| `id` | Unique UUID |
| `slug` | URL-friendly identifier (e.g., `acme-corp`) |
| `plan` | Subscription tier (trial, starter, pro, enterprise) |
| `max_agents` | Maximum agents allowed |
| `max_events_per_day` | Daily event limit |
| `isolation_tier` | shared or dedicated infrastructure |

### Isolation Tiers

| Tier | Database | K8s Resources | Use Case |
|------|----------|---------------|----------|
| **Shared** | Single DB, tenant_id filtering | Shared API pods | Trial, Starter, Pro |
| **Dedicated** | Separate namespace, own pods | Dedicated resources | Enterprise |

---

## How Data Isolation Works

### Database Level

Every data table includes a `tenant_id` column:

```sql
-- Agents table
CREATE TABLE agents (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    hostname VARCHAR(255),
    ...
);

CREATE INDEX idx_agents_tenant ON agents(tenant_id);
```

All queries are scoped:

```sql
-- Listing agents for a tenant
SELECT * FROM agents WHERE tenant_id = $1;

-- Never exposed:
SELECT * FROM agents; -- Would return all tenants' data
```

### API Level

The API extracts `tenant_id` from the JWT and passes it to all repository calls:

```go
// Middleware extracts tenant from JWT
func TenantContext(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := auth.GetClaims(r.Context())
        ctx := context.WithValue(r.Context(), TenantKey, claims.TenantID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Handler uses tenant context
func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
    tenantID := middleware.GetTenantID(r.Context())
    agents, _ := h.repo.ListByTenant(r.Context(), tenantID)
    // ...
}
```

### Agent Level

Each agent's JWT contains tenant claims:

```json
{
  "sub": "agent:550e8400-e29b-41d4-a716-446655440000",
  "tenant_id": "123e4567-e89b-12d3-a456-426614174000",
  "tenant_slug": "acme-corp",
  "scopes": ["events:submit", "heartbeat"],
  "exp": 1732924800
}
```

When the agent submits events, the API:
1. Validates the JWT
2. Extracts `tenant_id` from claims
3. Stores events with that `tenant_id`
4. Agent cannot access other tenants' data

---

## Plans and Limits

### Pricing Tiers

| Plan | Monthly | Agents | Events/Day | Features |
|------|---------|--------|------------|----------|
| **Trial** | Free | 5 | 1,000 | Basic AI, Email notifications |
| **Starter** | $49 | 25 | 10,000 | + Slack notifications, 30-day retention |
| **Pro** | $199 | 100 | 100,000 | + All channels, 90-day retention |
| **Enterprise** | $999 | 1,000 | 1,000,000 | + Dedicated infrastructure, custom SLAs |

### Rate Limits

Per-tenant rate limits based on plan:

| Plan | Requests/Minute | Requests/Hour |
|------|-----------------|---------------|
| Trial | 30 | 500 |
| Starter | 60 | 3,600 |
| Pro | 300 | 18,000 |
| Enterprise | 1,000 | 100,000 |

Enterprise tenants with dedicated infrastructure skip shared rate limiting.

### Enforcement

When limits are exceeded:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 45
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1732924800

{
  "error": "Rate limit exceeded",
  "retry_after": 45
}
```

For event limits:

```http
HTTP/1.1 402 Payment Required
X-Events-Limit: 10000
X-Events-Used: 10000
X-Events-Remaining: 0

{
  "error": "Daily event limit reached",
  "upgrade_url": "https://lumo.io/upgrade"
}
```

---

## Customer Onboarding Flow

### 1. Create Tenant (Admin)

```bash
curl -X POST https://api.lumo.io/api/v1/admin/tenants \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corporation",
    "slug": "acme-corp",
    "plan": "pro",
    "contact_email": "ops@acme.com",
    "max_agents": 100,
    "max_events_per_day": 100000
  }'
```

Response:
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "name": "Acme Corporation",
  "slug": "acme-corp",
  "plan": "pro",
  "status": "active",
  "api_key": "lumo_acme_xxxxxxxxxxxxxxxxxxxx",
  "created_at": "2025-11-30T12:00:00Z"
}
```

### 2. Provision Agent (Admin or Customer)

```bash
curl -X POST https://api.lumo.io/api/v1/admin/tenants/{id}/provision-agent \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "hostname": "acme-prod-cluster",
    "cluster_name": "production",
    "namespace": "lumo-system"
  }'
```

Response:
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "agent_token": "eyJhbGciOiJIUzI1NiIs...",
  "api_endpoint": "https://api.lumo.io",
  "kubernetes_manifest": "apiVersion: v1\nkind: Secret\n..."
}
```

### 3. Deploy Agent (Customer)

Customer applies the manifest to their cluster:

```bash
# Save the manifest
echo "$KUBERNETES_MANIFEST" > lumo-agent.yaml

# Apply to cluster
kubectl apply -f lumo-agent.yaml
```

The manifest includes:
- Secret with agent token
- ConfigMap with API endpoint
- Deployment for lumo-agent
- ServiceAccount with RBAC

### 4. Verify Connection

```bash
# Check agent status
kubectl -n lumo-system get pods

# Check agent logs
kubectl -n lumo-system logs -l app=lumo-agent

# Verify in Lumo API
curl https://api.lumo.io/api/v1/portal/agents \
  -H "Authorization: Bearer $CUSTOMER_TOKEN"
```

---

## Customer Portal API

Customers can self-service through the portal API:

### Dashboard

```bash
GET /api/v1/portal/dashboard
```

Returns:
- Tenant summary (plan, limits, status)
- Usage stats (today, 7-day, 30-day)
- Agent health (online/offline counts)
- Recent events
- Alerts (usage warnings, limit approaching)

### Usage History

```bash
GET /api/v1/portal/usage?days=30
```

Returns daily usage breakdown for billing visibility.

### Agent Management

```bash
# List agents
GET /api/v1/portal/agents

# Get agent details
GET /api/v1/portal/agents/{id}

# Delete agent
DELETE /api/v1/portal/agents/{id}
```

### Billing

```bash
GET /api/v1/portal/billing
```

Returns:
- Current plan details
- Pricing information
- Available upgrade options

---

## Enterprise Dedicated Infrastructure

For enterprise customers, we provision dedicated Kubernetes resources:

### What Gets Created

```
Namespace: lumo-tenant-{slug}
├── ResourceQuota (CPU, memory, storage limits)
├── LimitRange (default container limits)
├── NetworkPolicy (tenant isolation)
├── ServiceAccount (workload identity)
├── Role + RoleBinding (RBAC)
├── Secret: db-credentials
└── Secret: redis-credentials
```

### Resource Quotas by Plan

| Resource | Starter | Pro | Enterprise |
|----------|---------|-----|------------|
| CPU Limit | 4 cores | 8 cores | 16 cores |
| Memory Limit | 8Gi | 16Gi | 32Gi |
| Storage Limit | 50Gi | 200Gi | 500Gi |
| Pod Limit | 50 | 100 | 200 |

### Network Isolation

Enterprise namespaces have NetworkPolicies that:
- Allow ingress from ingress-nginx (external traffic)
- Allow ingress from lumo-system (internal API)
- Allow egress to DNS
- Allow egress to lumo-system
- Allow egress to external HTTPS (AI providers, notifications)
- Block all other cross-namespace traffic

---

## Usage Tracking

### What's Tracked

| Metric | Description | Storage |
|--------|-------------|---------|
| `event_count` | Events submitted today | Redis (real-time) + PostgreSQL (daily) |
| `ai_analysis_count` | AI analyses performed | Redis + PostgreSQL |
| `agent_count` | Active agents | PostgreSQL |
| `notification_count` | Notifications sent | PostgreSQL |
| `peak_agents` | Max concurrent agents | PostgreSQL |

### Background Processing

The UsageTracker runs a background flush every minute:
1. Reads counters from Redis
2. Persists to `tenant_usage` table
3. Resets Redis counters

### Querying Usage

```sql
-- Today's usage
SELECT * FROM tenant_usage 
WHERE tenant_id = $1 AND date = CURRENT_DATE;

-- Last 30 days
SELECT * FROM tenant_usage 
WHERE tenant_id = $1 AND date >= CURRENT_DATE - INTERVAL '30 days'
ORDER BY date DESC;
```

---

## Operational Considerations

### Monitoring

Key metrics to monitor per-tenant:

```promql
# Events per tenant
sum by (tenant_id) (lumo_events_total)

# API latency per tenant
histogram_quantile(0.99, 
  sum by (tenant_id, le) (lumo_api_request_duration_seconds_bucket)
)

# Agent health per tenant
sum by (tenant_id) (lumo_agents_online)
```

### Alerting

Set up alerts for:
- Tenant approaching event limit (>80%)
- Tenant approaching agent limit (>80%)
- No heartbeats from tenant's agents (>5 min)
- Elevated error rates per tenant

### Backup Strategy

- **Shared tier**: Standard PostgreSQL backup covers all tenants
- **Enterprise tier**: Per-namespace backup policies possible

### Tenant Offboarding

When a tenant is deleted:
1. Soft-delete tenant record (status = 'cancelled')
2. Stop accepting new events
3. Schedule data retention cleanup (30 days grace)
4. For enterprise: deprovision namespace

---

## Security Checklist

- [ ] JWT tokens expire in 24 hours
- [ ] API keys are hashed (SHA-256) in database
- [ ] All queries include tenant_id filter
- [ ] Rate limiting prevents resource exhaustion
- [ ] Enterprise tenants have NetworkPolicy isolation
- [ ] Audit logs capture tenant context
- [ ] Secrets stored in Kubernetes Secrets (not ConfigMaps)
- [ ] TLS 1.3 for all API traffic

---

## Related Documentation

- [Getting Started](getting-started.md) - Agent deployment guide
- [API Authentication](api-auth-guide.md) - JWT and API key details
- [Event Types](event-types.md) - Kubernetes events detected
- [Database Schema](database-schema.md) - Full schema reference
