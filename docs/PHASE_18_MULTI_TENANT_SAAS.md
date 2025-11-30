# Phase 18: Multi-Tenant SaaS Architecture

> **Version:** 1.0.0 Draft
> **Author:** Lumo Team
> **Created:** 2025-11-30
> **Status:** Proposal
> **Timeline:** 6-8 weeks
> **Priority:** Strategic - Enables commercial deployment

---

## Executive Summary

Transform Lumo from a self-hosted solution to a **SaaS platform** where:

1. **Lumo API (Control Plane)** - Hosted and managed by us
2. **Lumo Agents (Data Plane)** - Deployed in customer Kubernetes clusters

This architecture enables:
- Centralized management and billing
- Simplified customer onboarding
- Consistent updates and security patches
- Multi-tenant data isolation
- Revenue generation through subscription model

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           LUMO CLOUD (Our Infrastructure)                    │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                         Control Plane                                │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │    │
│  │  │   API GW     │  │  Lumo API    │  │  AI Service  │               │    │
│  │  │  (Kong/Nginx)│  │  (Multi-     │  │  (Shared)    │               │    │
│  │  │  Rate Limit  │──│  Tenant)     │──│  5 Providers │               │    │
│  │  │  Auth        │  │              │  │              │               │    │
│  │  └──────────────┘  └──────────────┘  └──────────────┘               │    │
│  │         │                 │                 │                        │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │    │
│  │  │  PostgreSQL  │  │    Redis     │  │ Notification │               │    │
│  │  │  (Per-tenant │  │  (Caching +  │  │   Service    │               │    │
│  │  │   schemas)   │  │   Pub/Sub)   │  │              │               │    │
│  │  └──────────────┘  └──────────────┘  └──────────────┘               │    │
│  │                                                                      │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │    │
│  │  │  Customer    │  │   Billing    │  │  Audit Log   │               │    │
│  │  │  Portal      │  │   Service    │  │   Service    │               │    │
│  │  │  (Dashboard) │  │  (Stripe)    │  │              │               │    │
│  │  └──────────────┘  └──────────────┘  └──────────────┘               │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                    HTTPS (TLS 1.3) │ Outbound only from customer clusters
                                    │
        ┌───────────────────────────┼───────────────────────────┐
        │                           │                           │
        ▼                           ▼                           ▼
┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│  Customer A      │    │  Customer B      │    │  Customer C      │
│  K8s Cluster     │    │  K8s Cluster     │    │  K8s Cluster     │
│  ┌────────────┐  │    │  ┌────────────┐  │    │  ┌────────────┐  │
│  │Lumo Agent  │  │    │  │Lumo Agent  │  │    │  │Lumo Agent  │  │
│  │(Event-     │  │    │  │(Event-     │  │    │  │(Event-     │  │
│  │ Driven)    │  │    │  │ Driven)    │  │    │  │ Driven)    │  │
│  │            │  │    │  │            │  │    │  │            │  │
│  │ Token: A   │  │    │  │ Token: B   │  │    │  │ Token: C   │  │
│  └────────────┘  │    └────────────┘  │    │  └────────────┘  │
└──────────────────┘    └──────────────────┘    └──────────────────┘
```

---

## Key Design Decisions

### 1. Agent Communication Model

**Decision: Outbound-Only HTTPS from Customer Clusters**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| Agents → API (HTTPS) | No firewall changes, simple | Polling overhead | ✅ Selected |
| API → Agents (Push) | Real-time | Requires ingress, security risk | ❌ |
| Bidirectional gRPC | Efficient | Complex firewall rules | ❌ |
| WebSocket long-poll | Real-time, outbound-only | Connection management | 🔄 Phase 2 |

**Rationale:** Customers won't need to open inbound ports. Agents initiate all connections outbound to our API. Event-driven mode already uses HTTP POST for event submission.

### 2. Multi-Tenancy Model

**Decision: Tiered Isolation Based on Plan**

| Plan | Database | K8s Namespace | API Pods | Use Case |
|------|----------|---------------|----------|----------|
| **Trial/Starter** | Shared (schema-per-tenant) | `lumo-shared` | Shared | SMB, startups |
| **Pro** | Shared (schema-per-tenant) | `lumo-shared` | Shared | Growing companies |
| **Enterprise** | Dedicated PostgreSQL | `lumo-ent-{customer}` | Dedicated | Large orgs, compliance |

**Why Tiered:**
- **Cost efficiency** for smaller customers (shared infra)
- **Full isolation** for Enterprise (compliance, SLA guarantees)
- **Upsell path** - customers can upgrade isolation level
- **Operational sanity** - only manage N dedicated namespaces for N enterprise customers

**Shared Tier (Trial/Starter/Pro):**
```sql
-- Schema-per-tenant in shared database
CREATE SCHEMA tenant_acme;
CREATE SCHEMA tenant_globex;

-- Tables duplicated per schema
tenant_acme.agents
tenant_acme.events
tenant_acme.jobs
```

**Enterprise Tier:**
```yaml
# Dedicated namespace per enterprise customer
apiVersion: v1
kind: Namespace
metadata:
  name: lumo-ent-acme
  labels:
    lumo.io/tier: enterprise
    lumo.io/tenant: acme
---
# Dedicated PostgreSQL (via CloudNativePG or managed RDS)
# Dedicated Redis
# Dedicated API deployment
# NetworkPolicy for complete isolation
```

### 3. Authentication Model

**Decision: Hierarchical JWT with API Keys**

```
┌─────────────────────────────────────────────────┐
│              Authentication Hierarchy            │
├─────────────────────────────────────────────────┤
│  Level 1: Tenant API Key                        │
│  - Long-lived key for tenant identification     │
│  - Used to provision agent tokens               │
│  - Stored in customer's secret manager          │
│                                                 │
│  Level 2: Agent JWT Token                       │
│  - Short-lived (24h), auto-refreshed            │
│  - Contains tenant_id, agent_id, scopes         │
│  - Per-agent granular permissions               │
│                                                 │
│  Level 3: User JWT Token (Portal)               │
│  - For customer portal access                   │
│  - Role-based permissions (admin, viewer)       │
└─────────────────────────────────────────────────┘
```

---

## Database Schema Changes

### New Tables

```sql
-- ============================================
-- TENANT MANAGEMENT
-- ============================================

-- Core tenant table (in public schema)
CREATE TABLE public.tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,  -- URL-friendly identifier
    display_name VARCHAR(255) NOT NULL,
    
    -- Billing & Subscription
    plan VARCHAR(50) NOT NULL DEFAULT 'trial',  -- trial, starter, pro, enterprise
    plan_started_at TIMESTAMP WITH TIME ZONE,
    plan_expires_at TIMESTAMP WITH TIME ZONE,
    stripe_customer_id VARCHAR(255),
    stripe_subscription_id VARCHAR(255),
    
    -- Limits (based on plan)
    max_agents INTEGER NOT NULL DEFAULT 5,
    max_events_per_day INTEGER NOT NULL DEFAULT 10000,
    max_users INTEGER NOT NULL DEFAULT 3,
    ai_analysis_enabled BOOLEAN DEFAULT true,
    retention_days INTEGER NOT NULL DEFAULT 30,
    
    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'active',  -- active, suspended, cancelled
    suspended_reason TEXT,
    
    -- Metadata
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT tenants_plan_check CHECK (plan IN ('trial', 'starter', 'pro', 'enterprise')),
    CONSTRAINT tenants_status_check CHECK (status IN ('active', 'suspended', 'cancelled', 'pending'))
);

CREATE INDEX idx_tenants_slug ON public.tenants(slug);
CREATE INDEX idx_tenants_status ON public.tenants(status);
CREATE INDEX idx_tenants_stripe_customer ON public.tenants(stripe_customer_id);

-- Tenant API Keys (for agent provisioning)
CREATE TABLE public.tenant_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(64) NOT NULL,  -- SHA-256 hash of the API key
    key_prefix VARCHAR(12) NOT NULL,  -- First 12 chars for identification (lumo_xxx...)
    scopes TEXT[] NOT NULL DEFAULT ARRAY['agent:register', 'agent:read'],
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_by UUID,  -- User who created this key
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT tenant_api_keys_unique_prefix UNIQUE (key_prefix)
);

CREATE INDEX idx_tenant_api_keys_tenant ON public.tenant_api_keys(tenant_id);
CREATE INDEX idx_tenant_api_keys_prefix ON public.tenant_api_keys(key_prefix);

-- Tenant Users (for portal access)
CREATE TABLE public.tenant_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255),  -- NULL if SSO-only
    name VARCHAR(255),
    role VARCHAR(50) NOT NULL DEFAULT 'member',  -- owner, admin, member, viewer
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    
    -- SSO
    sso_provider VARCHAR(50),  -- google, github, okta, saml
    sso_id VARCHAR(255),
    
    -- MFA
    mfa_enabled BOOLEAN DEFAULT false,
    mfa_secret VARCHAR(255),
    
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT tenant_users_role_check CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    CONSTRAINT tenant_users_unique_email_tenant UNIQUE (tenant_id, email)
);

CREATE INDEX idx_tenant_users_tenant ON public.tenant_users(tenant_id);
CREATE INDEX idx_tenant_users_email ON public.tenant_users(email);

-- ============================================
-- USAGE TRACKING & BILLING
-- ============================================

-- Daily usage aggregation per tenant
CREATE TABLE public.tenant_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    
    -- Counts
    agent_count INTEGER DEFAULT 0,
    event_count INTEGER DEFAULT 0,
    ai_analysis_count INTEGER DEFAULT 0,
    notification_count INTEGER DEFAULT 0,
    
    -- Storage (bytes)
    storage_used_bytes BIGINT DEFAULT 0,
    
    -- Computed metrics
    peak_agents INTEGER DEFAULT 0,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT tenant_usage_unique_date UNIQUE (tenant_id, date)
);

CREATE INDEX idx_tenant_usage_tenant_date ON public.tenant_usage(tenant_id, date DESC);

-- Audit log (cross-tenant, for compliance)
CREATE TABLE public.audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES public.tenants(id) ON DELETE SET NULL,
    user_id UUID,
    agent_id UUID,
    
    action VARCHAR(100) NOT NULL,  -- tenant.created, agent.registered, event.submitted, etc.
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    
    ip_address INET,
    user_agent TEXT,
    
    details JSONB,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_audit_log_tenant ON public.audit_log(tenant_id, created_at DESC);
CREATE INDEX idx_audit_log_action ON public.audit_log(action, created_at DESC);

-- ============================================
-- PER-TENANT SCHEMA TEMPLATE
-- ============================================

-- Function to create tenant schema with all required tables
CREATE OR REPLACE FUNCTION create_tenant_schema(tenant_slug VARCHAR)
RETURNS VOID AS $$
DECLARE
    schema_name VARCHAR := 'tenant_' || tenant_slug;
BEGIN
    -- Create schema
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);
    
    -- Create agents table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.agents (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(255) NOT NULL,
            hostname VARCHAR(255) NOT NULL,
            ip_address INET,
            platform VARCHAR(50) NOT NULL,
            architecture VARCHAR(50) NOT NULL,
            version VARCHAR(50) NOT NULL,
            status VARCHAR(50) NOT NULL DEFAULT ''online'',
            capabilities TEXT[],
            labels JSONB DEFAULT ''{}'',
            kubernetes_metadata JSONB,
            last_heartbeat_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            registered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            CONSTRAINT agents_hostname_unique UNIQUE (hostname)
        )', schema_name);
    
    -- Create events table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.events (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            agent_id UUID REFERENCES %I.agents(id) ON DELETE CASCADE,
            type VARCHAR(100) NOT NULL,
            severity VARCHAR(50) NOT NULL,
            title VARCHAR(500) NOT NULL,
            message TEXT,
            resource_kind VARCHAR(100),
            resource_name VARCHAR(255),
            resource_namespace VARCHAR(255),
            metadata JSONB DEFAULT ''{}'',
            ai_analysis TEXT,
            ai_analysis_at TIMESTAMP WITH TIME ZONE,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            processed_at TIMESTAMP WITH TIME ZONE
        )', schema_name, schema_name);
    
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_events_agent ON %I.events(agent_id)', schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_events_created ON %I.events(created_at DESC)', schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_events_type ON %I.events(type)', schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_events_severity ON %I.events(severity)', schema_name);
    
    -- Create jobs table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.jobs (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            agent_id UUID REFERENCES %I.agents(id) ON DELETE CASCADE,
            type VARCHAR(50) NOT NULL,
            status VARCHAR(50) NOT NULL DEFAULT ''pending'',
            config JSONB DEFAULT ''{}'',
            result JSONB,
            error_message TEXT,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            started_at TIMESTAMP WITH TIME ZONE,
            completed_at TIMESTAMP WITH TIME ZONE
        )', schema_name, schema_name);
    
    -- Create notification_history table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.notification_history (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            event_id UUID REFERENCES %I.events(id) ON DELETE CASCADE,
            channel VARCHAR(50) NOT NULL,
            status VARCHAR(50) NOT NULL,
            sent_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            error_message TEXT,
            metadata JSONB DEFAULT ''{}''
        )', schema_name, schema_name);
    
END;
$$ LANGUAGE plpgsql;

-- Function to drop tenant schema (for cleanup)
CREATE OR REPLACE FUNCTION drop_tenant_schema(tenant_slug VARCHAR)
RETURNS VOID AS $$
DECLARE
    schema_name VARCHAR := 'tenant_' || tenant_slug;
BEGIN
    EXECUTE format('DROP SCHEMA IF EXISTS %I CASCADE', schema_name);
END;
$$ LANGUAGE plpgsql;
```

---

## API Changes

### New Endpoints

#### Tenant Management (Internal/Admin)

```
POST   /api/v1/admin/tenants              # Create tenant (provisions schema)
GET    /api/v1/admin/tenants              # List all tenants
GET    /api/v1/admin/tenants/:id          # Get tenant details
PUT    /api/v1/admin/tenants/:id          # Update tenant
DELETE /api/v1/admin/tenants/:id          # Soft-delete tenant (schedule data cleanup)
POST   /api/v1/admin/tenants/:id/suspend  # Suspend tenant
POST   /api/v1/admin/tenants/:id/activate # Reactivate tenant
```

#### Tenant Self-Service (Customer Portal)

```
GET    /api/v1/tenant                     # Get current tenant info
PUT    /api/v1/tenant/settings            # Update tenant settings
GET    /api/v1/tenant/usage               # Get usage statistics
GET    /api/v1/tenant/billing             # Get billing info

# API Key Management
POST   /api/v1/tenant/api-keys            # Create new API key
GET    /api/v1/tenant/api-keys            # List API keys
DELETE /api/v1/tenant/api-keys/:id        # Revoke API key

# User Management
POST   /api/v1/tenant/users               # Invite user
GET    /api/v1/tenant/users               # List users
PUT    /api/v1/tenant/users/:id           # Update user role
DELETE /api/v1/tenant/users/:id           # Remove user
```

#### Agent Provisioning

```
# Called by customer's CI/CD or kubectl to provision agent
POST   /api/v1/provision/agent
Headers:
  X-API-Key: lumo_xxxx...  # Tenant API key

Request:
{
  "cluster_name": "prod-us-east",
  "namespace": "lumo-system"
}

Response:
{
  "agent_token": "eyJ...",       # JWT for agent auth
  "api_endpoint": "https://api.lumo.cloud",
  "config": {
    "event_driven": true,
    "debounce_window": "45s",
    ...
  },
  "manifest_url": "https://api.lumo.cloud/v1/provision/manifest?token=xxx"
}
```

### Modified Existing Endpoints

All existing endpoints need tenant context:

```go
// Before (single-tenant)
GET /api/v1/agents

// After (multi-tenant) - tenant extracted from JWT
GET /api/v1/agents
Headers:
  Authorization: Bearer <agent_token or user_token>

// Server extracts tenant_id from token, queries tenant_<slug>.agents
```

---

## Implementation Plan

### Phase 18a: Multi-Tenant Foundation (Week 1-2)

**Goal:** Database schema isolation and tenant context propagation

#### Tasks

1. **Database Schema Migration**
   - [ ] Create migration `017_multi_tenant_foundation.sql`
   - [ ] Add `public.tenants` table
   - [ ] Add `public.tenant_api_keys` table
   - [ ] Add `public.tenant_users` table
   - [ ] Create `create_tenant_schema()` function

2. **Tenant Context Middleware**
   - [ ] Create `/internal/api/middleware/tenant.go`
   ```go
   func TenantContextMiddleware(next http.Handler) http.Handler {
       return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           // Extract tenant from JWT claims
           claims := auth.GetClaims(r.Context())
           tenantID := claims.TenantID
           
           // Set tenant context
           ctx := context.WithValue(r.Context(), TenantContextKey, tenantID)
           
           // Set database schema search path
           // This ensures all queries use tenant_<slug> schema
           next.ServeHTTP(w, r.WithContext(ctx))
       })
   }
   ```

3. **Repository Layer Updates**
   - [ ] Add schema-aware database connection pooling
   - [ ] Update all repositories to use tenant schema
   ```go
   func (r *AgentRepository) setTenantSchema(ctx context.Context) error {
       tenantSlug := GetTenantSlug(ctx)
       _, err := r.db.ExecContext(ctx, 
           fmt.Sprintf("SET search_path TO tenant_%s, public", tenantSlug))
       return err
   }
   ```

4. **JWT Claims Extension**
   - [ ] Extend `Claims` struct in `/internal/api/auth/jwt.go`
   ```go
   type Claims struct {
       UserID   string   `json:"user_id,omitempty"`
       Username string   `json:"username,omitempty"`
       TenantID string   `json:"tenant_id"`           // NEW
       TenantSlug string `json:"tenant_slug"`         // NEW
       AgentID  string   `json:"agent_id,omitempty"`  // NEW (for agent tokens)
       Scopes   []string `json:"scopes,omitempty"`
       jwt.RegisteredClaims
   }
   ```

**Deliverables:**
- Tenant creation with schema provisioning
- Tenant context extracted from JWT
- Repositories query correct tenant schema

---

### Phase 18b: Agent Provisioning & Auth (Week 3-4)

**Goal:** Customer can generate agent tokens and deploy agents

#### Tasks

1. **Tenant API Key System**
   - [ ] Create `/internal/api/handlers/tenant_api_keys.go`
   - [ ] Implement API key generation (prefix + hash)
   - [ ] Implement key validation middleware

2. **Agent Provisioning API**
   - [ ] Create `/internal/api/handlers/provision.go`
   ```go
   func (h *ProvisionHandler) ProvisionAgent(w http.ResponseWriter, r *http.Request) {
       // Validate tenant API key
       apiKey := r.Header.Get("X-API-Key")
       tenant, err := h.validateAPIKey(apiKey)
       
       // Check tenant limits
       agentCount, _ := h.agentRepo.CountByTenant(tenant.ID)
       if agentCount >= tenant.MaxAgents {
           response.Error(w, "Agent limit reached", 402)
           return
       }
       
       // Generate agent JWT
       token, _ := h.jwtManager.GenerateAgentToken(tenant.ID, tenant.Slug, agentID)
       
       // Return provisioning response
       response.Success(w, ProvisionResponse{
           AgentToken: token,
           APIEndpoint: cfg.PublicAPIEndpoint,
           Config: h.getAgentConfig(tenant),
       })
   }
   ```

3. **Agent Auth Updates**
   - [ ] Update `/internal/agent/reporter.go` to use provisioned token
   - [ ] Add token refresh logic (before 24h expiry)
   - [ ] Handle 401 responses (re-provision needed)

4. **Manifest Generation**
   - [ ] Create `/internal/api/handlers/manifest.go`
   - [ ] Generate Kubernetes manifests with embedded config
   - [ ] Include ConfigMap with API endpoint and token secret reference

**Deliverables:**
- `POST /api/v1/provision/agent` working
- Agent can register with tenant-scoped token
- Kubernetes manifests generated on-demand

---

### Phase 18c: Usage Tracking & Rate Limiting (Week 5)

**Goal:** Track per-tenant usage and enforce limits

#### Tasks

1. **Usage Tracking Service**
   - [ ] Create `/internal/usage/tracker.go`
   ```go
   type UsageTracker struct {
       redis    *redis.Client
       db       *sql.DB
   }
   
   func (t *UsageTracker) IncrementEventCount(ctx context.Context, tenantID string) error {
       key := fmt.Sprintf("usage:%s:%s:events", tenantID, time.Now().Format("2006-01-02"))
       return t.redis.Incr(ctx, key).Err()
   }
   
   func (t *UsageTracker) CheckLimit(ctx context.Context, tenantID string, limitType string) (bool, error) {
       // Check current usage against tenant limits
   }
   
   func (t *UsageTracker) FlushToDB(ctx context.Context) error {
       // Periodic job to persist Redis counters to PostgreSQL
   }
   ```

2. **Per-Tenant Rate Limiting**
   - [ ] Extend `/internal/api/middleware/ratelimit.go`
   ```go
   func TenantRateLimiter(redis *redis.Client, tenantRepo *TenantRepository) func(http.Handler) http.Handler {
       return func(next http.Handler) http.Handler {
           return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
               tenantID := GetTenantID(r.Context())
               tenant, _ := tenantRepo.Get(tenantID)
               
               // Rate limit based on plan
               limit := getPlanRateLimit(tenant.Plan)
               
               // Use sliding window counter in Redis
               if exceeded := checkRateLimit(redis, tenantID, limit); exceeded {
                   response.TooManyRequests(w, "Rate limit exceeded")
                   return
               }
               
               next.ServeHTTP(w, r)
           })
       }
   }
   ```

3. **Limit Enforcement**
   - [ ] Block event submission when daily limit reached
   - [ ] Block agent registration when max agents reached
   - [ ] Return 402 Payment Required with upgrade prompt

**Deliverables:**
- Real-time usage tracking in Redis
- Daily aggregation to PostgreSQL
- Plan-based limits enforced

---

### Phase 18d: Customer Portal Backend (Week 6)

**Goal:** APIs for customer self-service dashboard

#### Tasks

1. **User Authentication**
   - [ ] Create `/internal/api/handlers/portal_auth.go`
   - [ ] Implement email/password login
   - [ ] Implement OAuth (Google, GitHub)
   - [ ] Session management with refresh tokens

2. **Dashboard APIs**
   - [ ] `GET /api/v1/tenant/dashboard` - Overview stats
   - [ ] `GET /api/v1/tenant/agents` - Agent list with health
   - [ ] `GET /api/v1/tenant/events` - Recent events
   - [ ] `GET /api/v1/tenant/events/:id` - Event detail with AI analysis

3. **Settings APIs**
   - [ ] Notification channel configuration
   - [ ] AI provider preferences (which model, custom prompts)
   - [ ] Alert thresholds

4. **Billing Integration (Stripe)**
   - [ ] Create `/internal/billing/stripe.go`
   - [ ] Webhook handling for subscription events
   - [ ] Usage-based billing reporting

**Deliverables:**
- Portal authentication working
- Dashboard data APIs
- Stripe integration for billing

---

### Phase 18e: Production Infrastructure (Week 7-8)

**Goal:** Deploy control plane infrastructure

#### Tasks

1. **Shared Tier Kubernetes Deployment**
   - [ ] Create `/deployments/kubernetes/saas/` directory
   - [ ] Namespace: `lumo-shared` for Trial/Starter/Pro customers
   - [ ] Multi-replica API deployment (3+ pods)
   - [ ] PostgreSQL with read replicas
   - [ ] Redis cluster
   - [ ] Ingress with TLS (api.lumo.cloud)

2. **Enterprise Tier Provisioning System**
   - [ ] Create `/internal/provisioning/enterprise.go`
   ```go
   type EnterpriseProvisioner struct {
       k8sClient  kubernetes.Interface
       pgOperator *cloudnativepg.Client  // or Terraform/Crossplane
   }
   
   func (p *EnterpriseProvisioner) ProvisionTenant(tenant *Tenant) error {
       // 1. Create dedicated namespace
       ns := &corev1.Namespace{
           ObjectMeta: metav1.ObjectMeta{
               Name: fmt.Sprintf("lumo-ent-%s", tenant.Slug),
               Labels: map[string]string{
                   "lumo.io/tier":   "enterprise",
                   "lumo.io/tenant": tenant.Slug,
               },
           },
       }
       
       // 2. Create NetworkPolicy for isolation
       // 3. Deploy dedicated PostgreSQL (CloudNativePG CRD)
       // 4. Deploy dedicated Redis
       // 5. Deploy dedicated API pods
       // 6. Create Ingress (tenant.api.lumo.cloud)
       // 7. Store connection info in tenant record
   }
   ```
   
   - [ ] Create `/deployments/kubernetes/saas/enterprise-template/`
   ```yaml
   # namespace.yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: lumo-ent-{{TENANT_SLUG}}
     labels:
       lumo.io/tier: enterprise
       lumo.io/tenant: "{{TENANT_SLUG}}"
   ---
   # network-policy.yaml - Complete isolation
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: deny-cross-tenant
     namespace: lumo-ent-{{TENANT_SLUG}}
   spec:
     podSelector: {}
     policyTypes:
       - Ingress
       - Egress
     ingress:
       - from:
         - namespaceSelector:
             matchLabels:
               lumo.io/tenant: "{{TENANT_SLUG}}"
         - namespaceSelector:
             matchLabels:
               name: ingress-nginx  # Allow ingress controller
     egress:
       - to:
         - namespaceSelector:
             matchLabels:
               lumo.io/tenant: "{{TENANT_SLUG}}"
       - to:  # Allow external (customer agents, AI providers)
         - ipBlock:
             cidr: 0.0.0.0/0
             except:
               - 10.0.0.0/8  # Block internal cluster traffic
   ---
   # postgres.yaml (CloudNativePG)
   apiVersion: postgresql.cnpg.io/v1
   kind: Cluster
   metadata:
     name: postgres
     namespace: lumo-ent-{{TENANT_SLUG}}
   spec:
     instances: 3
     storage:
       size: 100Gi
       storageClass: fast-ssd
     resources:
       requests:
         memory: "2Gi"
         cpu: "1"
       limits:
         memory: "4Gi"
         cpu: "2"
   ---
   # api-deployment.yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: lumo-api
     namespace: lumo-ent-{{TENANT_SLUG}}
   spec:
     replicas: 2
     selector:
       matchLabels:
         app: lumo-api
     template:
       spec:
         containers:
         - name: lumo-api
           image: lumo/api:latest
           env:
           - name: LUMO_TENANT_MODE
             value: "dedicated"
           - name: LUMO_TENANT_ID
             value: "{{TENANT_ID}}"
           - name: LUMO_DATABASE_HOST
             value: "postgres-rw.lumo-ent-{{TENANT_SLUG}}.svc"
   ```

3. **Observability**
   - [ ] Per-tenant metrics labels (`tenant_id`, `tenant_tier`)
   - [ ] Tenant dashboards in Grafana
   - [ ] Alert routing per tenant (PagerDuty/Opsgenie integration)
   - [ ] Dedicated dashboards for Enterprise customers

4. **Security Hardening**
   - [ ] Network policies (cross-namespace isolation)
   - [ ] Pod security standards (restricted)
   - [ ] Secret management (Vault with per-tenant paths)
   - [ ] WAF rules (SQL injection, XSS)
   - [ ] Audit logging for compliance

5. **Documentation**
   - [ ] Customer onboarding guide
   - [ ] Agent deployment guide
   - [ ] API reference
   - [ ] Enterprise upgrade guide
   - [ ] Troubleshooting guide

**Deliverables:**
- Shared tier: `lumo-shared` namespace with multi-tenant API
- Enterprise tier: Automated namespace provisioning
- Network isolation between enterprise tenants
- Monitoring and alerting per tenant
- Security audit passed
- Documentation complete

---

## Pricing Tiers

| Feature | Trial | Starter | Pro | Enterprise |
|---------|-------|---------|-----|------------|
| **Agents** | 3 | 10 | 50 | Unlimited |
| **Events/day** | 1,000 | 10,000 | 100,000 | Unlimited |
| **Users** | 1 | 3 | 10 | Unlimited |
| **Retention** | 7 days | 30 days | 90 days | 1 year+ |
| **AI Analysis** | ✓ | ✓ | ✓ | ✓ + Custom prompts |
| **Notifications** | Email | +Slack | +All channels | +Custom webhooks |
| **Support** | Community | Email | Priority | Dedicated CSM |
| **SLA** | - | - | 99.9% | 99.99% |
| **Price** | Free | $99/mo | $499/mo | Custom |
| | | | | |
| **Infrastructure** | | | | |
| Database | Shared (schema) | Shared (schema) | Shared (schema) | **Dedicated** |
| K8s Namespace | `lumo-shared` | `lumo-shared` | `lumo-shared` | **`lumo-ent-{customer}`** |
| API Pods | Shared | Shared | Shared | **Dedicated** |
| Network Isolation | Logical | Logical | Logical | **Physical (NetworkPolicy)** |
| Custom Domain | ❌ | ❌ | ❌ | ✓ `{customer}.api.lumo.cloud` |
| Data Residency | US | US | US | **Region choice** |
| Compliance | - | - | SOC2 | SOC2 + HIPAA + Custom |

---

## Migration Path

### For Existing Self-Hosted Users

1. **Export data** using new CLI command: `lumo export --format json`
2. **Create tenant** in Lumo Cloud
3. **Import data**: `lumo import --token <tenant_token> data.json`
4. **Update agents** with new API endpoint and token
5. **Verify** events flowing through

### Backwards Compatibility

- Self-hosted mode remains fully supported
- Single-tenant mode = implicit "default" tenant
- No breaking changes to existing API

---

## Security Considerations

### Data Isolation (Tiered)

**Shared Tier (Trial/Starter/Pro):**
- **Schema separation**: Each tenant's data in separate PostgreSQL schema
- **Query enforcement**: All queries include tenant context via middleware
- **Connection pooling**: Shared pool with `SET search_path` per request
- **No cross-tenant access**: Schema isolation prevents SQL injection cross-access

**Enterprise Tier:**
- **Database separation**: Dedicated PostgreSQL instance per customer
- **Namespace isolation**: Dedicated K8s namespace with NetworkPolicies
- **Network segmentation**: No pod-to-pod communication across tenants
- **Dedicated secrets**: Per-tenant Vault paths

### Secrets Management

```yaml
# Customer's Secret (in their cluster)
apiVersion: v1
kind: Secret
metadata:
  name: lumo-agent-token
  namespace: lumo-system
type: Opaque
data:
  token: <base64-encoded-jwt>
```

- Agent tokens stored in customer's Kubernetes secrets
- Tokens rotatable without agent restart (watch for secret changes)
- API keys hashed with SHA-256, prefix stored for identification

### Audit Trail

- All API calls logged with tenant context
- Retention: 1 year for compliance
- Searchable by tenant, user, action, resource

---

## Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Tenant provisioning | < 30 seconds | Time from signup to first agent |
| Agent registration | < 5 seconds | Time from deploy to online |
| Event latency | < 2 seconds | Time from K8s event to stored |
| API availability | 99.9% | Uptime monitoring |
| API latency (p99) | < 200ms | Prometheus metrics |

---

## Open Questions

1. **Regional deployment**: Single region initially or multi-region from start?
   - Recommendation: Start with single region (us-east-1), add regions based on customer demand

2. **Data residency**: Some customers require data in specific regions
   - Recommendation: Phase 2 - add region selector during tenant creation

3. **On-premise option**: Some enterprises want control plane on their infra
   - Recommendation: Phase 3 - offer "Lumo Enterprise" self-hosted license

4. **White-labeling**: Partners want to resell under their brand
   - Recommendation: Phase 3 - add customization options (logo, domain)

---

## Dependencies

| Dependency | Purpose | Status |
|------------|---------|--------|
| Phase 11c: Messaging | Event queue between agents and API | ✅ Complete |
| Phase 14: Reporting | Tenant usage reports | ⏳ Planned |
| PostgreSQL 15+ | Schema-per-tenant support | ✅ Available |
| Redis 7+ | Usage tracking, rate limiting | ✅ Available |
| Stripe | Billing integration | 🔄 New |
| Auth0/Clerk | User authentication (optional) | 🔄 New |

---

## Timeline Summary

| Week | Phase | Deliverables |
|------|-------|--------------|
| 1-2 | 18a: Foundation | Tenant schema, context middleware |
| 3-4 | 18b: Provisioning | Agent tokens, manifest generation |
| 5 | 18c: Usage | Tracking, rate limiting, limits |
| 6 | 18d: Portal | Auth, dashboard APIs, Stripe |
| 7-8 | 18e: Production | K8s deployment, security, docs |

**Total: 6-8 weeks**

---

## Next Steps

1. [ ] Review and approve this proposal
2. [ ] Create GitHub milestone for Phase 18
3. [ ] Create individual issues for each phase (18a-18e)
4. [ ] Set up Stripe account for billing
5. [ ] Design customer portal UI (Figma)
6. [ ] Begin Phase 18a implementation

---

## References

- [CLAUDE.md](../CLAUDE.md) - Current architecture
- [ROADMAP_TODO.md](../ROADMAP_TODO.md) - Technical roadmap
- [EVENT_DRIVEN_IMPLEMENTATION.md](../EVENT_DRIVEN_IMPLEMENTATION.md) - Agent architecture
- [deployments/kubernetes/](../deployments/kubernetes/) - Current K8s manifests
