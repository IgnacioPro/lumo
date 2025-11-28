# Lumo Multi-Tenant SaaS Architecture Plan

## Executive Summary

Transform Lumo from a single-tenant deployment to a **multi-tenant SaaS platform** where:
- **Control Plane (Lumo-hosted)**: Dedicated database + Redis per customer, centralized AI/notification logic
- **Data Plane (Customer-hosted)**: Lightweight agents deployed to customer K8s clusters
- **Value Proposition**: AI-powered augmentation layer for Prometheus/Grafana, not a replacement
- **GitOps Integration**: AI suggests remediation changes, humans create PRs manually

## Strategic Decisions (User Confirmed)

1. **Multi-Tenancy Model**: Database-per-tenant (isolated PostgreSQL + Redis per customer)
2. **Monitoring Strategy**: Integrate with Prometheus AlertManager webhooks, not replace existing monitoring
3. **Remediation Approach**: AI suggests fixes, humans manually create GitOps PRs (safe, auditable)
4. **Rule Engine**: Ship with 10-15 toggle-able policy templates (faster MVP than full DSL)

---

## Architecture Overview

### Current State (Single-Tenant)
```
Customer Cluster                    Single Shared Infrastructure
┌─────────────────┐                ┌──────────────────────────┐
│ Lumo Agents     │────────────────▶│ API Server               │
│ (Event-Driven)  │  HTTP POST     │ PostgreSQL (shared)      │
└─────────────────┘                │ Redis (shared)           │
                                   │ AI Analysis (shared)     │
                                   └──────────────────────────┘
```

### Target State (Multi-Tenant SaaS)
```
Customer Cluster A           Lumo Control Plane (Hosted)        Customer Cluster B
┌─────────────────┐         ┌──────────────────────────────┐   ┌─────────────────┐
│ Agents (Tenant A)│────────▶│ API Router (Multi-Tenant)    │◀──│ Agents (Tenant B)│
│ - K8s Informers │ Auth:   │ ┌─────────────────────────┐  │   │ - K8s Informers │
│ - Agent Token   │ Tenant A│ │ Tenant A Infrastructure │  │   │ - Agent Token   │
└─────────────────┘         │ │ - PostgreSQL DB         │  │   └─────────────────┘
                            │ │ - Redis Cache           │  │
Prometheus/Grafana          │ │ - AI Provider           │  │   Prometheus/Grafana
┌─────────────────┐         │ └─────────────────────────┘  │   ┌─────────────────┐
│ AlertManager    │────────▶│ ┌─────────────────────────┐  │◀──│ AlertManager    │
│ Webhook to Lumo │         │ │ Tenant B Infrastructure │  │   │ Webhook to Lumo │
└─────────────────┘         │ │ - PostgreSQL DB         │  │   └─────────────────┘
                            │ │ - Redis Cache           │  │
                            │ │ - AI Provider           │  │
                            │ └─────────────────────────┘  │
                            └──────────────────────────────┘
```

---

## Phase 1: Multi-Tenant Foundation (3-4 weeks)

### 1.1 Tenant Management System

**New Components:**
- `internal/tenant/` package for tenant lifecycle management
- `internal/tenant/provisioner.go` - Database/Redis provisioning
- `internal/tenant/router.go` - Tenant context extraction from requests
- Database migrations for tenant metadata storage

**Tenant Model:**
```go
type Tenant struct {
    ID              string    // UUID
    Name            string    // "Acme Corp"
    Slug            string    // "acme-corp" (URL-safe)
    DatabaseDSN     string    // "postgres://tenant-acme:..."
    RedisDSN        string    // "redis://tenant-acme:..."
    APIKey          string    // Hashed agent authentication key
    Status          string    // "active", "suspended", "trial"
    CreatedAt       time.Time
    BillingTier     string    // "starter", "business", "enterprise"
    MaxAgents       int       // Quota enforcement
}
```

**Key Files to Create:**
- `internal/database/migrations/020_tenants.sql` - Tenant metadata table (stored in control plane DB)
- `internal/tenant/manager.go` - CRUD operations for tenants
- `internal/tenant/provisioner.go` - Automated DB/Redis provisioning

**Provisioning Flow:**
1. Admin creates tenant via control plane API: `POST /admin/v1/tenants`
2. Provisioner creates isolated PostgreSQL database: `CREATE DATABASE tenant_<slug>`
3. Provisioner runs migrations on new tenant database
4. Provisioner creates isolated Redis namespace: `tenant:<tenant_id>:*`
5. Provisioner generates API key for agent authentication
6. Returns connection details + agent deployment YAML

### 1.2 Request Router & Tenant Context

**Middleware Chain:**
```go
// internal/api/middleware/tenant.go
func TenantContextMiddleware(tenantManager *tenant.Manager) func(next http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract tenant identifier from:
            // 1. API key header (for agents)
            // 2. Subdomain (api-acme.lumo.sh)
            // 3. Custom header (X-Tenant-ID)

            tenantID := extractTenantID(r)
            tenant, err := tenantManager.GetTenant(tenantID)
            if err != nil {
                http.Error(w, "Invalid tenant", http.StatusUnauthorized)
                return
            }

            // Inject tenant-specific DB/Redis connections into context
            ctx := context.WithValue(r.Context(), TenantContextKey, tenant)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

**Key Files to Modify:**
- `internal/api/router.go` - Add TenantContextMiddleware
- `internal/api/middleware/tenant.go` - New file for tenant extraction
- `internal/database/postgres.go` - Support dynamic DSN per tenant
- `internal/cache/redis.go` - Support namespace prefixing per tenant

### 1.3 Connection Pooling Strategy

**Challenge**: 1000 customers = 1000 PostgreSQL connections if all pre-pooled

**Solution**: Lazy connection pooling with TTL-based eviction
```go
// internal/tenant/connection_pool.go
type ConnectionPool struct {
    pools map[string]*pgxpool.Pool // tenant_id -> connection pool
    mu    sync.RWMutex
    ttl   time.Duration             // 30 minutes idle timeout
}

func (cp *ConnectionPool) GetPool(tenant *Tenant) (*pgxpool.Pool, error) {
    cp.mu.RLock()
    pool, exists := cp.pools[tenant.ID]
    cp.mu.RUnlock()

    if exists {
        return pool, nil
    }

    // Lazy initialization
    cp.mu.Lock()
    defer cp.mu.Unlock()

    pool, err = pgxpool.New(context.Background(), tenant.DatabaseDSN)
    if err != nil {
        return nil, err
    }

    cp.pools[tenant.ID] = pool

    // Start TTL-based cleanup goroutine
    go cp.cleanupIdlePool(tenant.ID, cp.ttl)

    return pool, nil
}
```

**Key Files to Create:**
- `internal/tenant/connection_pool.go` - Lazy pool management
- `internal/tenant/connection_pool_test.go` - Test pool eviction

---

## Phase 2: Prometheus/Grafana Integration (2 weeks)

### 2.1 AlertManager Webhook Receiver

**New Endpoint:**
```
POST /api/v1/integrations/prometheus/webhook
```

**AlertManager Configuration (Customer-side):**
```yaml
# Customer's alertmanager.yml
receivers:
  - name: 'lumo-ai'
    webhook_configs:
      - url: 'https://api-acme.lumo.sh/api/v1/integrations/prometheus/webhook'
        send_resolved: true
        http_config:
          authorization:
            credentials: '<tenant-api-key>'

route:
  receiver: 'lumo-ai'
  group_by: ['alertname', 'cluster', 'namespace']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 12h
```

**Handler Implementation:**
```go
// internal/api/handlers/integrations/prometheus.go
type PrometheusWebhookHandler struct {
    aiProvider    ai.ProviderAdapter
    eventRepo     repository.EventRepository
    notifications notifications.Manager
}

func (h *PrometheusWebhookHandler) HandleAlert(w http.ResponseWriter, r *http.Request) {
    tenant := tenant.FromContext(r.Context())

    var alert alertmanager.Alert
    if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
        http.Error(w, "Invalid payload", http.StatusBadRequest)
        return
    }

    // Convert AlertManager alert to Lumo Event
    event := h.convertAlertToEvent(alert, tenant.ID)

    // Store in tenant-specific database
    db := h.getTenantDB(tenant)
    if err := h.eventRepo.Create(db, event); err != nil {
        log.WithError(err).Error("Failed to store event")
    }

    // Trigger AI analysis async
    go h.analyzeWithAI(event, tenant)

    w.WriteHeader(http.StatusAccepted)
}
```

**Key Files to Create:**
- `internal/api/handlers/integrations/prometheus.go` - Webhook handler
- `internal/api/handlers/integrations/prometheus_test.go` - Test AlertManager payload parsing
- `internal/integrations/prometheus/types.go` - AlertManager data structures

### 2.2 Hybrid Detection Strategy

**Keep Both:**
1. **Event-Driven K8s Monitoring** (existing) - Real-time semantic events (OOMKilled, CrashLoopBackOff)
2. **Prometheus Webhook** (new) - Metric-based alerts (CPU > 90%, latency spikes)

**Benefits:**
- Customers keep existing Grafana dashboards + alerts
- Lumo adds AI layer on top of existing monitoring
- Real-time K8s events provide faster detection than Prometheus scrapes
- Metric-based alerts capture resource exhaustion patterns

---

## Phase 3: Policy Templates (1-2 weeks)

### 3.1 Policy Definition System

**Location**: `internal/policies/`

**Policy Template Structure:**
```go
type PolicyTemplate struct {
    ID          string   // "high-memory-usage"
    Name        string   // "High Memory Usage Detection"
    Description string   // "Alert when containers exceed memory limits"
    Category    string   // "resource-management", "reliability", "security"
    Severity    string   // "medium"
    Enabled     bool     // Default false (customer toggles)

    // Filter conditions
    Conditions PolicyConditions

    // Actions to take when triggered
    Actions []PolicyAction
}

type PolicyConditions struct {
    EventTypes      []string            // ["oom-killed", "pod-evicted"]
    MinSeverity     string              // "high"
    Namespaces      []string            // ["production"]
    LabelSelectors  map[string]string   // {"env": "production"}

    // For Prometheus alerts
    AlertNames      []string            // ["HighMemoryUsage", "PodCrashLooping"]
}

type PolicyAction struct {
    Type   string   // "notify", "analyze-with-ai", "create-incident"
    Config map[string]interface{}
}
```

**Pre-Built Templates (10-15 to ship):**
1. **High Memory Usage** - OOMKilled + Prometheus MemoryUsage > 90%
2. **Frequent Restarts** - CrashLoopBackOff + restart count > 5
3. **Image Pull Failures** - ImagePullBackOff events
4. **Deployment Failures** - ProgressDeadlineExceeded
5. **Volume Issues** - PVC ProvisioningFailed, FailedMount
6. **Node Health** - NodeNotReady, MemoryPressure, DiskPressure
7. **Job Failures** - BackoffLimitExceeded
8. **Security Events** - Unauthorized access attempts (from Prometheus alerts)
9. **High Latency** - Prometheus latency > threshold
10. **Error Rate Spike** - Prometheus error rate > threshold

**Key Files to Create:**
- `internal/policies/templates.go` - Define 10-15 templates
- `internal/policies/engine.go` - Evaluate events against enabled policies
- `internal/policies/repository.go` - Per-tenant policy configuration storage
- `internal/api/handlers/policies.go` - CRUD endpoints for policy management

**API Endpoints:**
```
GET    /api/v1/policies/templates        # List available templates
GET    /api/v1/policies                  # List enabled policies for tenant
PUT    /api/v1/policies/:id/enable       # Enable a policy template
PUT    /api/v1/policies/:id/disable      # Disable a policy template
PATCH  /api/v1/policies/:id              # Customize policy conditions
```

### 3.2 Policy Evaluation Engine

**Integration Point**: `internal/api/handlers/events.go`

Modify existing event processing to evaluate policies:
```go
// Before AI analysis, check if event matches any enabled policies
func (h *EventsHandler) processEventWithPolicies(event *models.Event, tenant *tenant.Tenant) {
    db := h.getTenantDB(tenant)

    // Get enabled policies for this tenant
    policies, err := h.policyRepo.GetEnabledPolicies(db, tenant.ID)

    // Evaluate event against each policy
    for _, policy := range policies {
        if h.policyEngine.Matches(event, policy) {
            // Execute policy actions
            for _, action := range policy.Actions {
                switch action.Type {
                case "notify":
                    h.sendNotification(event, policy)
                case "analyze-with-ai":
                    h.analyzeWithAI(event, policy)
                case "create-incident":
                    h.createIncident(event, policy)
                }
            }
        }
    }
}
```

---

## Phase 4: GitOps Remediation Integration (2 weeks)

### 4.1 AI Remediation Suggestion Generator

**Enhancement**: Modify existing AI analysis to output structured GitOps changes

**Location**: `internal/ai/remediation.go` (new file)

```go
type RemediationSuggestion struct {
    EventID         string
    RootCause       string   // AI-generated explanation
    Impact          string   // "Pod restarting every 30s, affecting 10% of traffic"

    // Structured GitOps changes
    GitOpsChanges []GitOpsChange

    // Manual steps if GitOps not applicable
    ManualSteps []string
}

type GitOpsChange struct {
    FilePath    string   // "k8s/production/deployment.yaml"
    ChangeType  string   // "increase-memory-limit", "add-resource-request", "increase-replicas"

    // YAML path + new value
    YAMLPath    string   // "spec.template.spec.containers[0].resources.limits.memory"
    CurrentValue string  // "256Mi"
    ProposedValue string // "512Mi"

    // Human-readable explanation
    Explanation string   // "Increase memory limit to prevent OOMKilled errors"
    Confidence  float64  // 0.85 (85% confidence this will fix the issue)
}
```

**AI Prompt Enhancement:**
```go
func (p *BaseProvider) GenerateRemediationPrompt(event *models.Event) string {
    return fmt.Sprintf(`
Analyze this Kubernetes event and provide remediation suggestions:

Event Type: %s
Severity: %s
Resource: %s/%s in namespace %s
Message: %s

Provide:
1. Root cause analysis (why did this happen?)
2. Impact assessment (what's affected?)
3. GitOps-compatible remediation changes in this JSON format:
{
  "root_cause": "...",
  "impact": "...",
  "gitops_changes": [
    {
      "file_path": "k8s/production/deployment.yaml",
      "change_type": "increase-memory-limit",
      "yaml_path": "spec.template.spec.containers[0].resources.limits.memory",
      "current_value": "256Mi",
      "proposed_value": "512Mi",
      "explanation": "...",
      "confidence": 0.85
    }
  ],
  "manual_steps": ["If GitOps changes don't resolve, check application logs for memory leaks"]
}

Focus on common Kubernetes remediation patterns:
- Resource limit adjustments
- Replica count changes
- Liveness/readiness probe tuning
- Image version changes (if crashlooping)
`, event.EventType, event.Severity, event.ResourceKind, event.ResourceName,
   event.Namespace, event.Message)
}
```

### 4.2 Remediation UI/API Endpoints

**New API Endpoints:**
```
GET  /api/v1/events/:id/remediation           # Get AI remediation suggestion
POST /api/v1/events/:id/remediation/export    # Export as diff/patch file
```

**Response Example:**
```json
{
  "event_id": "evt_123",
  "root_cause": "Container exceeded memory limit due to Java heap misconfiguration",
  "impact": "Pod restarting every 45 seconds, 3 restarts in last 5 minutes",
  "gitops_changes": [
    {
      "file_path": "k8s/production/api-deployment.yaml",
      "change_type": "increase-memory-limit",
      "yaml_path": "spec.template.spec.containers[0].resources.limits.memory",
      "current_value": "512Mi",
      "proposed_value": "1Gi",
      "explanation": "Increase memory limit to accommodate Java heap + overhead",
      "confidence": 0.92
    }
  ],
  "manual_steps": [
    "After applying change, monitor memory usage in Grafana",
    "If issue persists, investigate application for memory leaks using heap dump"
  ],
  "export_formats": {
    "git_diff": "https://api.lumo.sh/api/v1/events/evt_123/remediation/export?format=diff",
    "kubectl_patch": "https://api.lumo.sh/api/v1/events/evt_123/remediation/export?format=kubectl",
    "yaml": "https://api.lumo.sh/api/v1/events/evt_123/remediation/export?format=yaml"
  }
}
```

**Export Formats:**
1. **Git Diff** - Patch file for manual `git apply`
2. **kubectl Patch** - `kubectl patch` command
3. **YAML** - Full updated YAML manifest

**Key Files to Create:**
- `internal/ai/remediation.go` - Generate structured remediation
- `internal/api/handlers/remediation.go` - Export endpoints
- `internal/remediation/exporter.go` - Generate diff/patch files

---

## Phase 5: Agent Deployment Automation (1 week)

### 5.1 Tenant-Specific Agent YAML Generator

**New Endpoint:**
```
GET /admin/v1/tenants/:id/agent-manifest
```

**Generated Output:**
```yaml
# Generated for tenant: acme-corp
apiVersion: v1
kind: Secret
metadata:
  name: lumo-agent-secret
  namespace: lumo-system
stringData:
  api-endpoint: "https://api-acme.lumo.sh"
  api-key: "<tenant-specific-api-key>"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lumo-agent
  namespace: lumo-system
spec:
  replicas: 2
  selector:
    matchLabels:
      app: lumo-agent
      tenant: acme-corp
  template:
    metadata:
      labels:
        app: lumo-agent
        tenant: acme-corp
    spec:
      serviceAccountName: lumo-agent
      containers:
      - name: agent
        image: lumo/lumo-agent:v1.1.0
        env:
        - name: LUMO_AGENT_API_ENDPOINT
          valueFrom:
            secretKeyRef:
              name: lumo-agent-secret
              key: api-endpoint
        - name: LUMO_AGENT_TOKEN
          valueFrom:
            secretKeyRef:
              name: lumo-agent-secret
              key: api-key
        - name: LUMO_AGENT_MODE
          value: "event-driven"
        - name: LUMO_AGENT_KUBERNETES_ENABLED
          value: "true"
---
# RBAC manifests...
```

**Key Files to Create:**
- `internal/tenant/manifest_generator.go` - Generate tenant-specific YAML
- `internal/api/handlers/admin/tenants.go` - Admin endpoints for tenant management

---

## Phase 6: Scalability & Infrastructure (2 weeks)

### 6.1 Database Provisioning Automation

**Options:**
1. **K8s Operator** - CloudNativePG, Zalando Postgres Operator (automated PostgreSQL provisioning)
2. **Cloud Provider** - AWS RDS, GCP Cloud SQL (managed PostgreSQL with API provisioning)
3. **Docker Compose** - Local development only

**Recommended for MVP**: Cloud Provider (AWS RDS)
- Automated provisioning via API
- Built-in backups, monitoring, HA
- Auto-scaling storage
- Per-tenant isolation via separate RDS instances

**Provisioner Implementation:**
```go
// internal/tenant/provisioner/rds.go
type RDSProvisioner struct {
    rdsClient *rds.Client
    region    string
}

func (p *RDSProvisioner) ProvisionTenantDatabase(tenant *Tenant) error {
    instanceID := fmt.Sprintf("lumo-tenant-%s", tenant.Slug)

    // Create RDS instance
    _, err := p.rdsClient.CreateDBInstance(context.Background(), &rds.CreateDBInstanceInput{
        DBInstanceIdentifier: aws.String(instanceID),
        DBInstanceClass:     aws.String("db.t3.micro"),  // Start small
        Engine:              aws.String("postgres"),
        EngineVersion:       aws.String("15.4"),
        AllocatedStorage:    aws.Int32(20),
        MasterUsername:      aws.String("lumo"),
        MasterUserPassword:  aws.String(generateSecurePassword()),
        BackupRetentionPeriod: aws.Int32(7),
        StorageEncrypted:    aws.Bool(true),
        Tags: []types.Tag{
            {Key: aws.String("TenantID"), Value: aws.String(tenant.ID)},
            {Key: aws.String("ManagedBy"), Value: aws.String("lumo-control-plane")},
        },
    })

    // Wait for instance to be available (async)
    go p.waitForInstanceAndRunMigrations(instanceID, tenant)

    return err
}
```

### 6.2 Redis Provisioning

**Options:**
1. **Shared Redis with namespace prefixing** - `tenant:<tenant_id>:*`
2. **Redis Cluster with tenant sharding** - Hash slot allocation per tenant
3. **Separate Redis instances** - ElastiCache per tenant

**Recommended for MVP**: Shared Redis with namespace prefixing
- Cost-effective for 100-1000 customers
- Upgrade to dedicated instances for enterprise tier

```go
// internal/cache/multi_tenant_redis.go
type MultiTenantRedis struct {
    client *redis.Client
}

func (r *MultiTenantRedis) GetTenantClient(tenantID string) *redis.Client {
    return r.client.WithContext(context.Background()).
        WithNamespace(fmt.Sprintf("tenant:%s", tenantID))
}
```

### 6.3 Resource Limits Per Tier

**Billing Tiers:**
```go
type BillingTier struct {
    Name              string
    MaxAgents         int
    MaxEventsPerDay   int
    AIAnalysisQuota   int  // AI calls per month
    DataRetentionDays int
    SupportLevel      string
}

var BillingTiers = map[string]BillingTier{
    "starter": {
        Name:              "Starter",
        MaxAgents:         5,
        MaxEventsPerDay:   1000,
        AIAnalysisQuota:   100,
        DataRetentionDays: 7,
        SupportLevel:      "community",
    },
    "business": {
        Name:              "Business",
        MaxAgents:         50,
        MaxEventsPerDay:   10000,
        AIAnalysisQuota:   1000,
        DataRetentionDays: 30,
        SupportLevel:      "email",
    },
    "enterprise": {
        Name:              "Enterprise",
        MaxAgents:         -1,  // unlimited
        MaxEventsPerDay:   -1,
        AIAnalysisQuota:   -1,
        DataRetentionDays: 90,
        SupportLevel:      "24/7",
    },
}
```

**Quota Enforcement:**
- `internal/api/middleware/quota.go` - Reject requests exceeding tier limits
- `internal/tenant/quota_tracker.go` - Track usage per tenant

---

## Implementation Roadmap

### Sprint 1 (Week 1-2): Multi-Tenant Foundation
- [ ] Create `internal/tenant/` package structure
- [ ] Add tenant model + metadata database
- [ ] Implement tenant context middleware
- [ ] Build lazy connection pool
- [ ] Test with 2-3 mock tenants

### Sprint 2 (Week 3): Provisioning Automation
- [ ] Implement RDS provisioner
- [ ] Add Redis namespace support
- [ ] Build agent manifest generator
- [ ] Test end-to-end tenant creation

### Sprint 3 (Week 4): Prometheus Integration
- [ ] Create AlertManager webhook handler
- [ ] Convert Prometheus alerts to Lumo events
- [ ] Test hybrid detection (K8s events + Prometheus)
- [ ] Document customer AlertManager configuration

### Sprint 4 (Week 5): Policy Templates
- [ ] Define 10-15 pre-built policy templates
- [ ] Build policy evaluation engine
- [ ] Create policy management API endpoints
- [ ] Add UI for toggling policies (future - CLI for MVP)

### Sprint 5 (Week 6): GitOps Remediation
- [ ] Enhance AI prompts for structured output
- [ ] Build remediation suggestion API
- [ ] Implement diff/patch export formats
- [ ] Test with real OOMKilled scenario

### Sprint 6 (Week 7-8): Scalability & Testing
- [ ] Load test with 10 concurrent tenants
- [ ] Implement quota enforcement
- [ ] Add billing tier logic
- [ ] End-to-end integration tests
- [ ] Documentation: customer onboarding guide

---

## Key Files to Create

### Core Multi-Tenancy
- `internal/tenant/manager.go` - Tenant CRUD operations
- `internal/tenant/provisioner/rds.go` - AWS RDS provisioning
- `internal/tenant/provisioner/redis.go` - Redis namespace provisioning
- `internal/tenant/connection_pool.go` - Lazy DB connection pooling
- `internal/api/middleware/tenant.go` - Tenant context extraction
- `internal/database/migrations/020_tenants.sql` - Tenant metadata table

### Integrations
- `internal/api/handlers/integrations/prometheus.go` - AlertManager webhook
- `internal/integrations/prometheus/types.go` - AlertManager data structures
- `internal/integrations/prometheus/converter.go` - Alert → Event conversion

### Policies
- `internal/policies/templates.go` - Pre-built policy definitions
- `internal/policies/engine.go` - Policy evaluation logic
- `internal/policies/repository.go` - Per-tenant policy storage
- `internal/api/handlers/policies.go` - Policy management endpoints

### Remediation
- `internal/ai/remediation.go` - Structured remediation generation
- `internal/remediation/exporter.go` - Diff/patch file generators
- `internal/api/handlers/remediation.go` - Remediation export endpoints

### Admin
- `internal/api/handlers/admin/tenants.go` - Tenant provisioning endpoints
- `internal/tenant/manifest_generator.go` - Agent YAML generator

### Quota & Billing
- `internal/api/middleware/quota.go` - Quota enforcement middleware
- `internal/tenant/quota_tracker.go` - Usage tracking per tenant
- `internal/tenant/billing_tiers.go` - Tier definitions

---

## Key Files to Modify

### Existing Components
- `internal/api/router.go` - Add tenant middleware to all routes
- `internal/database/postgres.go` - Support dynamic tenant DSN
- `internal/cache/redis.go` - Support tenant namespace prefixing
- `internal/api/handlers/events.go` - Integrate policy evaluation
- `internal/agent/reporter.go` - Use tenant-specific API endpoint
- `cmd/lumo/serve.go` - Initialize tenant connection pool

---

## Success Metrics

### Technical
- [ ] Support 100 tenants with <500ms P95 latency
- [ ] Zero cross-tenant data leakage (audit with automated tests)
- [ ] Tenant provisioning <2 minutes (RDS creation time)
- [ ] Connection pool eviction working (verify idle tenants don't hold connections)

### Business
- [ ] Customer onboarding <10 minutes (provision + agent deploy)
- [ ] AI suggestion acceptance rate >30% (customers apply suggested changes)
- [ ] Integration with 3+ Prometheus deployments (validation)
- [ ] Policy template usage >50% (customers enable at least 5 templates)

---

## Risk Mitigation

### Database-per-Tenant Costs
**Risk**: 1000 customers × $15/month (db.t3.micro) = $15K/month infrastructure
**Mitigation**:
- Start with shared database + RLS for starter tier
- Upgrade to dedicated database only for business/enterprise
- Implement tiered pricing that covers infrastructure costs

### RDS Provisioning Delays
**Risk**: RDS instance creation takes 5-10 minutes
**Mitigation**:
- Async provisioning with webhook callback
- Pre-provision pool of databases for instant activation
- Show "Provisioning in progress" status to customers

### Connection Pool Exhaustion
**Risk**: 1000 tenants × 10 connections = 10K connections
**Mitigation**:
- Lazy pool creation (only active tenants)
- TTL-based eviction (30-min idle)
- PgBouncer connection pooler (reduces RDS connections)

### Cross-Tenant Data Leaks
**Risk**: Bug in tenant context middleware exposes data
**Mitigation**:
- Automated tests with 2+ mock tenants
- Audit logs for all cross-tenant access attempts
- Row-level security (RLS) as fallback defense

---

## Open Questions for User

1. **Pricing Model**: How should we price? Per-agent, per-event, per-AI-analysis, or flat monthly?
2. **Self-Service Onboarding**: Should customers provision tenants themselves, or sales-assisted?
3. **GitOps Repository Access**: How do we get access to customer GitOps repos? GitHub App, PAT, manual?
4. **Multi-Cluster**: Should one tenant support multiple K8s clusters (common for prod/staging)?
5. **Data Residency**: EU customers require EU-hosted databases? (AWS regions)

---

## Next Steps

After plan approval:
1. Create `internal/tenant/` package structure
2. Build tenant metadata database migration
3. Implement tenant context middleware
4. Test with 2 mock tenants (validate isolation)
5. Document customer onboarding workflow
