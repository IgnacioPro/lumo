# GitOps Infrastructure & Customer Dashboard Plan

> **Version:** 1.0.0
> **Created:** 2025-12-01
> **Branch:** `feature/gitops-infra-standards`
> **Status:** Planning
> **Author:** Infrastructure Team

---

## Executive Summary

This document outlines the comprehensive plan to:

1. **Create a Customer Dashboard** - Grafana dashboard for per-tenant/namespace metrics
2. **Migrate to GitOps Architecture** - Terraform for infrastructure, Helm for applications, ArgoCD for continuous delivery
3. **Evolve deploy-saas.sh** - Split into production-ready components while maintaining kind testing

The goal is to establish infrastructure standards that enable:
- Reproducible infrastructure deployments
- Self-service tenant onboarding
- Per-customer observability
- Continuous deployment with rollback capabilities

---

## Table of Contents

1. [Current State Analysis](#current-state-analysis)
2. [Phase 1: Customer Dashboard](#phase-1-customer-dashboard)
3. [Phase 2: Terraform Infrastructure](#phase-2-terraform-infrastructure)
4. [Phase 3: Helm Charts](#phase-3-helm-charts)
5. [Phase 4: ArgoCD Integration](#phase-4-argocd-integration)
6. [Phase 5: Production Migration](#phase-5-production-migration)
7. [Implementation Timeline](#implementation-timeline)
8. [Detailed TODO List](#detailed-todo-list)

---

## Current State Analysis

### Existing Infrastructure

| Component | Current State | Location |
|-----------|---------------|----------|
| **Kind Deployment** | Bash script (988 LOC) | `deployments/kubernetes/kind/deploy-saas.sh` |
| **Monitoring** | Grafana + Prometheus | `deployments/kubernetes/monitoring/` |
| **Dashboard** | Single "Lumo Overview" | `deployments/kubernetes/monitoring/grafana/` |
| **Helm Charts** | `lumo-agent` (basic) | `deployments/kubernetes/helm/lumo-agent/` |
| **Base Manifests** | Raw YAML | `deployments/kubernetes/base/` |

### Key Observations

1. **deploy-saas.sh** handles everything:
   - Cluster creation (kind)
   - Infrastructure (PostgreSQL, Redis)
   - API server deployment
   - Multi-tenant setup (3 test tenants)
   - Agent provisioning per tenant
   - Validation tests

2. **Monitoring gaps**:
   - No per-tenant/customer filtering
   - No customer-facing dashboard
   - Single monolithic dashboard

3. **Missing production tooling**:
   - No Terraform for infrastructure
   - No ArgoCD for GitOps
   - No production Helm values
   - No environment separation

### Multi-Tenant Schema (Already Exists)

From `006_multi_tenant.sql`:
- `tenants` table with plan, limits, isolation tier
- `tenant_usage` table with daily metrics
- `tenant_api_keys` for agent provisioning
- `tenant_id` columns on agents, events, jobs, approvals

---

## Phase 1: Customer Dashboard

### Objective

Create a Grafana dashboard that allows selecting a customer namespace and viewing their specific metrics.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Grafana - Customer Dashboard                      │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │  Variables:                                                        │ │
│  │  [Namespace ▼] tenant-acme-corp                                   │ │
│  │  [Time Range ▼] Last 24h                                          │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐         │
│  │  Active Agents  │  │  Events Today   │  │  AI Analyses    │         │
│  │       3         │  │      247        │  │       89        │         │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘         │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  Events by Severity (24h)                                         │   │
│  │  ▓▓▓▓▓▓▓▓▓▓ Critical: 12                                         │   │
│  │  ▓▓▓▓▓▓▓▓▓▓▓▓▓ High: 45                                          │   │
│  │  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ Medium: 120                                 │   │
│  │  ▓▓▓▓▓▓▓▓▓▓▓▓ Low: 70                                            │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌────────────────────────────────┐  ┌────────────────────────────────┐ │
│  │  Event Rate (5m avg)          │  │  AI Analysis Duration          │ │
│  │  [Timeseries graph]           │  │  [p50, p95, p99 lines]         │ │
│  └────────────────────────────────┘  └────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

### Implementation Plan

#### 1.1 Dashboard Structure

**File:** `deployments/kubernetes/monitoring/grafana/dashboards/customer-dashboard.json`

**Panels to include:**

| Row | Panels | Metrics |
|-----|--------|---------|
| **Header Stats** | Active Agents, Events Today, AI Analyses, Usage % | `lumo_agent_info`, `lumo_events_processed_total`, `lumo_ai_analysis_total` |
| **Events Overview** | Events by Severity (bar), Events by Type (pie) | `lumo_events_processed_total{namespace=~"$namespace"}` |
| **Event Processing** | Event Rate (timeseries), Processing Duration (histogram) | `rate(lumo_events_processed_total[5m])`, `lumo_event_processing_duration_seconds` |
| **AI Analysis** | Analysis Rate, Duration (p50/p95/p99) | `lumo_ai_analysis_total`, `lumo_ai_analysis_duration_seconds` |
| **Agent Health** | Agent List (table), Heartbeat Rate, Cache Hit Rate | `lumo_agent_info`, `lumo_agent_heartbeats_total`, `lumo_agent_cache_hits_total` |
| **Resource Usage** | CPU/Memory by agent pods | `container_cpu_usage_seconds_total`, `container_memory_usage_bytes` |
| **Quota Status** | Events vs Limit gauge, Agents vs Limit gauge | Custom metrics from usage tracking |

#### 1.2 Template Variables

```json
{
  "templating": {
    "list": [
      {
        "name": "namespace",
        "type": "query",
        "query": "label_values(lumo_agent_info, namespace)",
        "regex": "/^tenant-.*/",
        "label": "Customer Namespace",
        "multi": false,
        "includeAll": false
      },
      {
        "name": "datasource",
        "type": "datasource",
        "query": "prometheus"
      }
    ]
  }
}
```

#### 1.3 Key Metrics Queries

```promql
# Active agents in namespace
count(lumo_agent_info{namespace=~"$namespace"})

# Events by severity (24h)
sum by (severity) (increase(lumo_events_processed_total{namespace=~"$namespace"}[24h]))

# Event rate (5m average)
sum(rate(lumo_events_processed_total{namespace=~"$namespace"}[5m]))

# AI analysis duration p95
histogram_quantile(0.95, sum by (le) (rate(lumo_ai_analysis_duration_seconds_bucket{namespace=~"$namespace"}[5m])))

# Agent heartbeat success rate
sum(rate(lumo_agent_heartbeats_total{namespace=~"$namespace", status="success"}[5m])) /
sum(rate(lumo_agent_heartbeats_total{namespace=~"$namespace"}[5m])) * 100

# Cache hit rate
sum(rate(lumo_agent_cache_hits_total{namespace=~"$namespace"}[5m])) /
(sum(rate(lumo_agent_cache_hits_total{namespace=~"$namespace"}[5m])) + 
 sum(rate(lumo_agent_cache_misses_total{namespace=~"$namespace"}[5m]))) * 100
```

#### 1.4 Files to Create/Modify

```
deployments/kubernetes/monitoring/
├── grafana/
│   ├── dashboards/
│   │   ├── lumo-overview.json          # Existing - keep as operator view
│   │   └── customer-dashboard.json     # NEW - per-customer view
│   ├── dashboard-configmap.yaml        # Modify to include new dashboard
│   └── values.yaml                     # Update with new dashboard folder
└── README.md                           # Update documentation
```

---

## Phase 2: Terraform Infrastructure

### Objective

Define cloud infrastructure as code using Terraform, supporting multiple cloud providers.

### Directory Structure

```
infrastructure/
├── terraform/
│   ├── modules/
│   │   ├── kubernetes-cluster/        # EKS/GKE/AKS cluster
│   │   │   ├── main.tf
│   │   │   ├── variables.tf
│   │   │   ├── outputs.tf
│   │   │   └── versions.tf
│   │   ├── postgresql/                # RDS/Cloud SQL/Azure PostgreSQL
│   │   │   ├── main.tf
│   │   │   ├── variables.tf
│   │   │   └── outputs.tf
│   │   ├── redis/                     # ElastiCache/Memorystore/Azure Cache
│   │   │   ├── main.tf
│   │   │   ├── variables.tf
│   │   │   └── outputs.tf
│   │   ├── networking/                # VPC/Subnets/Security Groups
│   │   │   ├── main.tf
│   │   │   ├── variables.tf
│   │   │   └── outputs.tf
│   │   ├── observability/             # CloudWatch/Stackdriver/Azure Monitor
│   │   │   ├── main.tf
│   │   │   ├── variables.tf
│   │   │   └── outputs.tf
│   │   └── dns/                       # Route53/Cloud DNS/Azure DNS
│   │       ├── main.tf
│   │       ├── variables.tf
│   │       └── outputs.tf
│   │
│   ├── environments/
│   │   ├── dev/                       # Development environment
│   │   │   ├── main.tf
│   │   │   ├── terraform.tfvars
│   │   │   └── backend.tf
│   │   ├── staging/                   # Staging environment
│   │   │   ├── main.tf
│   │   │   ├── terraform.tfvars
│   │   │   └── backend.tf
│   │   └── production/                # Production environment
│   │       ├── main.tf
│   │       ├── terraform.tfvars
│   │       └── backend.tf
│   │
│   └── README.md
│
├── kind/                              # Local development (keep existing)
│   └── ...
│
└── README.md
```

### Module Specifications

#### 2.1 Kubernetes Cluster Module

**File:** `infrastructure/terraform/modules/kubernetes-cluster/main.tf`

```hcl
# AWS EKS Example
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 19.0"

  cluster_name    = var.cluster_name
  cluster_version = var.kubernetes_version

  vpc_id     = var.vpc_id
  subnet_ids = var.subnet_ids

  eks_managed_node_groups = {
    default = {
      min_size     = var.min_nodes
      max_size     = var.max_nodes
      desired_size = var.desired_nodes

      instance_types = var.instance_types
      capacity_type  = var.capacity_type
    }

    # Dedicated node group for Lumo API (if needed)
    lumo-api = {
      min_size     = 2
      max_size     = 5
      desired_size = 2

      instance_types = ["t3.medium"]
      labels = {
        "lumo.io/role" = "api"
      }
      taints = []
    }
  }

  # Enable IRSA for pod IAM
  enable_irsa = true

  tags = var.tags
}
```

**Variables:**
```hcl
variable "cluster_name" {
  description = "EKS cluster name"
  type        = string
}

variable "kubernetes_version" {
  description = "Kubernetes version"
  type        = string
  default     = "1.29"
}

variable "vpc_id" {
  description = "VPC ID for the cluster"
  type        = string
}

variable "subnet_ids" {
  description = "Subnet IDs for the cluster"
  type        = list(string)
}

variable "min_nodes" {
  description = "Minimum number of nodes"
  type        = number
  default     = 2
}

variable "max_nodes" {
  description = "Maximum number of nodes"
  type        = number
  default     = 10
}

variable "desired_nodes" {
  description = "Desired number of nodes"
  type        = number
  default     = 3
}

variable "instance_types" {
  description = "EC2 instance types for nodes"
  type        = list(string)
  default     = ["t3.medium"]
}

variable "capacity_type" {
  description = "EC2 capacity type (ON_DEMAND or SPOT)"
  type        = string
  default     = "ON_DEMAND"
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default     = {}
}
```

#### 2.2 PostgreSQL Module

**File:** `infrastructure/terraform/modules/postgresql/main.tf`

```hcl
# AWS RDS PostgreSQL
resource "aws_db_instance" "lumo" {
  identifier = "${var.environment}-lumo-postgres"

  engine         = "postgres"
  engine_version = var.postgres_version
  instance_class = var.instance_class

  allocated_storage     = var.allocated_storage
  max_allocated_storage = var.max_allocated_storage
  storage_type          = "gp3"
  storage_encrypted     = true

  db_name  = "lumo"
  username = "lumo"
  password = var.db_password

  vpc_security_group_ids = [aws_security_group.rds.id]
  db_subnet_group_name   = aws_db_subnet_group.lumo.name

  # High availability
  multi_az = var.multi_az

  # Backups
  backup_retention_period = var.backup_retention_days
  backup_window           = "03:00-04:00"
  maintenance_window      = "Mon:04:00-Mon:05:00"

  # Performance Insights
  performance_insights_enabled          = true
  performance_insights_retention_period = 7

  # Parameters
  parameter_group_name = aws_db_parameter_group.lumo.name

  # Deletion protection
  deletion_protection = var.environment == "production"

  tags = var.tags
}

resource "aws_db_parameter_group" "lumo" {
  family = "postgres15"
  name   = "${var.environment}-lumo-postgres"

  parameter {
    name  = "shared_preload_libraries"
    value = "pg_stat_statements"
  }

  parameter {
    name  = "log_statement"
    value = "ddl"
  }
}
```

#### 2.3 Redis Module

**File:** `infrastructure/terraform/modules/redis/main.tf`

```hcl
# AWS ElastiCache Redis
resource "aws_elasticache_replication_group" "lumo" {
  replication_group_id = "${var.environment}-lumo-redis"
  description          = "Lumo Redis cluster"

  engine               = "redis"
  engine_version       = var.redis_version
  node_type            = var.node_type
  port                 = 6379

  # Cluster mode
  num_cache_clusters = var.num_cache_clusters
  automatic_failover_enabled = var.num_cache_clusters > 1

  # Security
  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token                 = var.auth_token

  subnet_group_name  = aws_elasticache_subnet_group.lumo.name
  security_group_ids = [aws_security_group.redis.id]

  # Maintenance
  maintenance_window       = "sun:05:00-sun:06:00"
  snapshot_window          = "00:00-01:00"
  snapshot_retention_limit = var.snapshot_retention_days

  tags = var.tags
}
```

#### 2.4 Environment Configuration

**File:** `infrastructure/terraform/environments/production/main.tf`

```hcl
terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.23"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.11"
    }
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "lumo"
      Environment = "production"
      ManagedBy   = "terraform"
    }
  }
}

# Networking
module "networking" {
  source = "../../modules/networking"

  environment     = "production"
  vpc_cidr        = "10.0.0.0/16"
  azs             = ["us-east-1a", "us-east-1b", "us-east-1c"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]
}

# Kubernetes Cluster
module "kubernetes" {
  source = "../../modules/kubernetes-cluster"

  cluster_name       = "lumo-production"
  kubernetes_version = "1.29"
  vpc_id             = module.networking.vpc_id
  subnet_ids         = module.networking.private_subnet_ids

  min_nodes     = 3
  max_nodes     = 20
  desired_nodes = 5

  instance_types = ["t3.large"]
  capacity_type  = "ON_DEMAND"
}

# PostgreSQL
module "postgresql" {
  source = "../../modules/postgresql"

  environment           = "production"
  vpc_id                = module.networking.vpc_id
  subnet_ids            = module.networking.private_subnet_ids
  postgres_version      = "15.4"
  instance_class        = "db.r6g.large"
  allocated_storage     = 100
  max_allocated_storage = 500
  multi_az              = true
  backup_retention_days = 30
  db_password           = var.db_password
}

# Redis
module "redis" {
  source = "../../modules/redis"

  environment            = "production"
  vpc_id                 = module.networking.vpc_id
  subnet_ids             = module.networking.private_subnet_ids
  redis_version          = "7.0"
  node_type              = "cache.r6g.large"
  num_cache_clusters     = 3
  snapshot_retention_days = 7
  auth_token             = var.redis_auth_token
}

# Outputs for Helm/ArgoCD
output "cluster_endpoint" {
  value = module.kubernetes.cluster_endpoint
}

output "cluster_ca_certificate" {
  value     = module.kubernetes.cluster_ca_certificate
  sensitive = true
}

output "database_endpoint" {
  value = module.postgresql.endpoint
}

output "redis_endpoint" {
  value = module.redis.endpoint
}
```

---

## Phase 3: Helm Charts

### Objective

Create comprehensive Helm charts for all Lumo components with production-ready defaults.

### Directory Structure

```
deployments/kubernetes/helm/
├── lumo-agent/                        # Existing - enhance
│   ├── Chart.yaml
│   ├── values.yaml
│   ├── values-production.yaml         # NEW
│   └── templates/
│       ├── deployment.yaml
│       ├── daemonset.yaml
│       ├── configmap.yaml
│       ├── serviceaccount.yaml
│       ├── rbac.yaml
│       ├── networkpolicy.yaml
│       └── servicemonitor.yaml        # NEW
│
├── lumo-api/                          # NEW - API server chart
│   ├── Chart.yaml
│   ├── values.yaml
│   ├── values-production.yaml
│   └── templates/
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── ingress.yaml
│       ├── configmap.yaml
│       ├── secret.yaml                # External secrets reference
│       ├── hpa.yaml
│       ├── pdb.yaml
│       ├── networkpolicy.yaml
│       └── servicemonitor.yaml
│
├── lumo-infrastructure/               # NEW - PostgreSQL, Redis
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── postgresql/
│       │   ├── statefulset.yaml
│       │   ├── service.yaml
│       │   ├── configmap.yaml
│       │   ├── secret.yaml
│       │   └── pvc.yaml
│       ├── redis/
│       │   ├── deployment.yaml
│       │   ├── service.yaml
│       │   └── configmap.yaml
│       └── NOTES.txt
│
├── lumo-monitoring/                   # NEW - Prometheus + Grafana
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── prometheus/
│       ├── grafana/
│       ├── dashboards/
│       └── alertmanager/
│
└── lumo-stack/                        # Umbrella chart
    ├── Chart.yaml
    ├── values.yaml
    ├── values-kind.yaml               # Local development
    ├── values-staging.yaml
    ├── values-production.yaml
    └── charts/                        # Subcharts
        ├── lumo-api/
        ├── lumo-agent/
        ├── lumo-infrastructure/
        └── lumo-monitoring/
```

### Helm Chart Specifications

#### 3.1 lumo-api Chart

**File:** `deployments/kubernetes/helm/lumo-api/Chart.yaml`

```yaml
apiVersion: v2
name: lumo-api
description: Lumo API Server - Multi-tenant SaaS control plane
type: application
version: 1.0.0
appVersion: "1.0.0"

keywords:
  - monitoring
  - api
  - multi-tenant
  - saas

home: https://github.com/ignacio/lumo
sources:
  - https://github.com/ignacio/lumo

maintainers:
  - name: Lumo Team
    email: lumo@example.com

dependencies:
  - name: postgresql
    version: "13.x.x"
    repository: "https://charts.bitnami.com/bitnami"
    condition: postgresql.enabled
  - name: redis
    version: "18.x.x"
    repository: "https://charts.bitnami.com/bitnami"
    condition: redis.enabled

kubeVersion: ">=1.24.0-0"
```

**File:** `deployments/kubernetes/helm/lumo-api/values.yaml`

```yaml
# Lumo API Server Helm Values

global:
  imagePullSecrets: []
  storageClass: ""

# Image configuration
image:
  repository: ghcr.io/ignacio/lumo
  pullPolicy: IfNotPresent
  tag: ""  # Defaults to appVersion

# Replica count
replicaCount: 2

# Service Account
serviceAccount:
  create: true
  annotations: {}
  name: ""

# Pod configuration
podAnnotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"

podSecurityContext:
  runAsNonRoot: true
  runAsUser: 65534
  fsGroup: 65534
  seccompProfile:
    type: RuntimeDefault

containerSecurityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  runAsUser: 65534
  capabilities:
    drop:
      - ALL

# Resource limits
resources:
  requests:
    cpu: 100m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 1Gi

# Autoscaling
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80

# Pod Disruption Budget
podDisruptionBudget:
  enabled: true
  minAvailable: 1

# Service configuration
service:
  type: ClusterIP
  port: 8080
  annotations: {}

# Ingress configuration
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/rate-limit: "100"
  hosts:
    - host: api.lumo.io
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: lumo-api-tls
      hosts:
        - api.lumo.io

# Health probes
livenessProbe:
  httpGet:
    path: /api/v1/health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /api/v1/health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3

startupProbe:
  httpGet:
    path: /api/v1/health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  failureThreshold: 30

# Application configuration
config:
  environment: production
  logLevel: info
  logFormat: json

# Database configuration
database:
  host: ""  # Set via external secret or values
  port: 5432
  name: lumo
  user: lumo
  sslmode: require
  # Password from secret
  existingSecret: lumo-db-credentials
  existingSecretKey: password
  # Connection pool
  maxOpenConns: 25
  maxIdleConns: 10
  connMaxLifetime: 5m

# Redis configuration
redis:
  host: ""  # Set via external secret or values
  port: 6379
  db: 0
  # Password from secret
  existingSecret: lumo-redis-credentials
  existingSecretKey: password

# API configuration
api:
  port: 8080
  host: "0.0.0.0"
  jwtSecret:
    existingSecret: lumo-api-secrets
    existingSecretKey: jwt-secret
  allowedOrigins:
    - "https://lumo.io"
    - "https://app.lumo.io"

# AI configuration
ai:
  enabled: true
  provider: anthropic
  # API key from secret
  existingSecret: lumo-ai-secrets
  existingSecretKey: anthropic-api-key

# Notification configuration
notifications:
  enabled: true
  slack:
    existingSecret: lumo-notification-secrets
    existingSecretKey: slack-webhook-url

# Prometheus ServiceMonitor
serviceMonitor:
  enabled: true
  interval: 30s
  scrapeTimeout: 10s
  labels: {}

# Network Policy
networkPolicy:
  enabled: true
  allowIngressFromNamespaces:
    - ingress-nginx
    - monitoring
  allowEgressToNamespaces:
    - lumo-system
    - monitoring

# Node selector, tolerations, affinity
nodeSelector: {}

tolerations: []

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/name
                operator: In
                values:
                  - lumo-api
          topologyKey: kubernetes.io/hostname

# External secrets (for production)
externalSecrets:
  enabled: false
  backend: ""  # vault, aws, gcp
  refreshInterval: 1h
```

#### 3.2 lumo-stack Umbrella Chart

**File:** `deployments/kubernetes/helm/lumo-stack/Chart.yaml`

```yaml
apiVersion: v2
name: lumo-stack
description: Complete Lumo SaaS Platform Stack
type: application
version: 1.0.0
appVersion: "1.0.0"

dependencies:
  - name: lumo-api
    version: "1.0.0"
    repository: "file://../lumo-api"
    condition: lumo-api.enabled

  - name: lumo-agent
    version: "1.0.0"
    repository: "file://../lumo-agent"
    condition: lumo-agent.enabled
    
  - name: lumo-infrastructure
    version: "1.0.0"
    repository: "file://../lumo-infrastructure"
    condition: lumo-infrastructure.enabled

  - name: lumo-monitoring
    version: "1.0.0"
    repository: "file://../lumo-monitoring"
    condition: lumo-monitoring.enabled

  - name: ingress-nginx
    version: "4.x.x"
    repository: "https://kubernetes.github.io/ingress-nginx"
    condition: ingress-nginx.enabled

  - name: cert-manager
    version: "1.x.x"
    repository: "https://charts.jetstack.io"
    condition: cert-manager.enabled
```

**File:** `deployments/kubernetes/helm/lumo-stack/values-kind.yaml`

```yaml
# Kind local development values

global:
  environment: development

lumo-api:
  enabled: true
  replicaCount: 1
  image:
    repository: lumo
    tag: local
    pullPolicy: Never
  ingress:
    enabled: false
  autoscaling:
    enabled: false
  resources:
    requests:
      cpu: 50m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 512Mi

lumo-agent:
  enabled: true
  deployment:
    replicas: 1
  daemonset:
    enabled: false
  image:
    repository: lumo-agent
    tag: local
    pullPolicy: Never

lumo-infrastructure:
  enabled: true
  postgresql:
    enabled: true
    persistence:
      enabled: false
  redis:
    enabled: true
    persistence:
      enabled: false

lumo-monitoring:
  enabled: true
  prometheus:
    enabled: true
  grafana:
    enabled: true

ingress-nginx:
  enabled: false

cert-manager:
  enabled: false
```

---

## Phase 4: ArgoCD Integration

### Objective

Implement GitOps continuous delivery using ArgoCD for declarative, version-controlled deployments.

### Directory Structure

```
deployments/argocd/
├── bootstrap/                         # ArgoCD itself + app-of-apps
│   ├── argocd-install.yaml           # ArgoCD installation
│   ├── argocd-cm.yaml                # ConfigMap
│   ├── argocd-secret.yaml            # Secrets (encrypted)
│   └── app-of-apps.yaml              # Root application
│
├── apps/                              # Application definitions
│   ├── lumo-system/                   # Lumo core apps
│   │   ├── lumo-api.yaml
│   │   ├── lumo-infrastructure.yaml
│   │   └── lumo-monitoring.yaml
│   │
│   ├── tenants/                       # Per-tenant applications
│   │   ├── tenant-template.yaml       # Template for new tenants
│   │   ├── tenant-acme-corp.yaml
│   │   ├── tenant-globex-ind.yaml
│   │   └── tenant-initech-sol.yaml
│   │
│   └── platform/                      # Platform components
│       ├── ingress-nginx.yaml
│       ├── cert-manager.yaml
│       ├── external-secrets.yaml
│       └── prometheus-stack.yaml
│
├── applicationsets/                   # Dynamic app generation
│   ├── tenant-agents.yaml            # Generate agent apps from Git
│   └── environments.yaml             # Multi-environment setup
│
├── projects/                          # ArgoCD projects
│   ├── platform.yaml
│   ├── lumo-system.yaml
│   └── tenants.yaml
│
└── README.md
```

### ArgoCD Configuration

#### 4.1 App-of-Apps Pattern

**File:** `deployments/argocd/bootstrap/app-of-apps.yaml`

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: lumo-platform
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: platform
  source:
    repoURL: https://github.com/ignacio/lumo.git
    targetRevision: HEAD
    path: deployments/argocd/apps
    directory:
      recurse: true
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
      allowEmpty: false
    syncOptions:
      - CreateNamespace=true
      - PrunePropagationPolicy=foreground
      - PruneLast=true
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 3m
```

#### 4.2 Lumo API Application

**File:** `deployments/argocd/apps/lumo-system/lumo-api.yaml`

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: lumo-api
  namespace: argocd
  labels:
    app.kubernetes.io/part-of: lumo
    app.kubernetes.io/component: api
spec:
  project: lumo-system
  source:
    repoURL: https://github.com/ignacio/lumo.git
    targetRevision: HEAD
    path: deployments/kubernetes/helm/lumo-api
    helm:
      releaseName: lumo-api
      valueFiles:
        - values.yaml
        - values-production.yaml
      parameters:
        - name: image.tag
          value: "1.0.0"
  destination:
    server: https://kubernetes.default.svc
    namespace: lumo-system
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jsonPointers:
        - /spec/replicas
```

#### 4.3 Tenant ApplicationSet

**File:** `deployments/argocd/applicationsets/tenant-agents.yaml`

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: tenant-agents
  namespace: argocd
spec:
  generators:
    - git:
        repoURL: https://github.com/ignacio/lumo.git
        revision: HEAD
        directories:
          - path: deployments/tenants/*
    - list:
        elements:
          - tenant: acme-corp
            namespace: tenant-acme-corp
            plan: pro
          - tenant: globex-ind
            namespace: tenant-globex-ind
            plan: starter
          - tenant: initech-sol
            namespace: tenant-initech-sol
            plan: trial
  template:
    metadata:
      name: "tenant-{{tenant}}-agent"
      namespace: argocd
      labels:
        app.kubernetes.io/part-of: lumo
        lumo.io/tenant: "{{tenant}}"
    spec:
      project: tenants
      source:
        repoURL: https://github.com/ignacio/lumo.git
        targetRevision: HEAD
        path: deployments/kubernetes/helm/lumo-agent
        helm:
          releaseName: "lumo-agent-{{tenant}}"
          values: |
            agent:
              tenantId: "{{tenant}}"
              apiEndpoint: https://api.lumo.io
            namespace: "{{namespace}}"
      destination:
        server: https://kubernetes.default.svc
        namespace: "{{namespace}}"
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        syncOptions:
          - CreateNamespace=true
```

#### 4.4 ArgoCD Projects

**File:** `deployments/argocd/projects/lumo-system.yaml`

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: lumo-system
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  description: Lumo core system components

  sourceRepos:
    - https://github.com/ignacio/lumo.git
    - https://charts.bitnami.com/bitnami
    - https://prometheus-community.github.io/helm-charts

  destinations:
    - namespace: lumo-system
      server: https://kubernetes.default.svc
    - namespace: monitoring
      server: https://kubernetes.default.svc

  clusterResourceWhitelist:
    - group: ""
      kind: Namespace
    - group: rbac.authorization.k8s.io
      kind: ClusterRole
    - group: rbac.authorization.k8s.io
      kind: ClusterRoleBinding
    - group: apiextensions.k8s.io
      kind: CustomResourceDefinition

  namespaceResourceBlacklist:
    - group: ""
      kind: ResourceQuota
    - group: ""
      kind: LimitRange

  roles:
    - name: admin
      description: Admin access to lumo-system
      policies:
        - p, proj:lumo-system:admin, applications, *, lumo-system/*, allow
      groups:
        - lumo-admins

    - name: developer
      description: Developer access (read-only sync)
      policies:
        - p, proj:lumo-system:developer, applications, get, lumo-system/*, allow
        - p, proj:lumo-system:developer, applications, sync, lumo-system/*, allow
      groups:
        - lumo-developers
```

---

## Phase 5: Production Migration

### Objective

Migrate from kind-based testing to production-ready infrastructure while maintaining backwards compatibility.

### Migration Strategy

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Migration Flow                                      │
│                                                                              │
│  deploy-saas.sh (current)                                                   │
│       │                                                                      │
│       ├──► Split into modules                                               │
│       │    ├── scripts/setup-kind.sh         # Kind cluster only            │
│       │    ├── scripts/deploy-infra.sh       # PostgreSQL, Redis            │
│       │    ├── scripts/deploy-api.sh         # Lumo API server              │
│       │    └── scripts/provision-tenants.sh  # Multi-tenant setup           │
│       │                                                                      │
│       ├──► Create Helm charts                                               │
│       │    ├── helm/lumo-api/                                               │
│       │    ├── helm/lumo-infrastructure/                                    │
│       │    └── helm/lumo-stack/                                             │
│       │                                                                      │
│       ├──► Add Terraform modules                                            │
│       │    └── terraform/modules/*                                          │
│       │                                                                      │
│       └──► Configure ArgoCD                                                 │
│            └── argocd/apps/*                                                │
│                                                                              │
│  Result:                                                                     │
│    - Kind: Use deploy-saas.sh OR helm install lumo-stack -f values-kind.yaml│
│    - Staging: terraform apply + argocd sync                                 │
│    - Production: terraform apply + argocd sync (with approvals)             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Script Refactoring

**Retain:** `deployments/kubernetes/kind/deploy-saas.sh` for testing

**Add:** Modular scripts for reuse

```bash
# New script structure
scripts/
├── kind/
│   ├── setup-cluster.sh          # Create kind cluster
│   ├── build-images.sh           # Build and load Docker images
│   └── teardown-cluster.sh       # Delete kind cluster
│
├── deploy/
│   ├── deploy-infrastructure.sh  # Deploy PostgreSQL + Redis
│   ├── deploy-api.sh             # Deploy Lumo API
│   ├── deploy-monitoring.sh      # Deploy Prometheus + Grafana
│   └── deploy-agents.sh          # Deploy agent per namespace
│
├── tenant/
│   ├── create-tenant.sh          # Create new tenant (DB + K8s)
│   ├── provision-agent.sh        # Provision agent for tenant
│   └── delete-tenant.sh          # Cleanup tenant resources
│
└── validate/
    ├── validate-deployment.sh    # Health checks
    └── run-e2e-tests.sh          # End-to-end tests
```

---

## Implementation Timeline

### Week 1: Customer Dashboard (Phase 1)

| Day | Task | Deliverable |
|-----|------|-------------|
| 1 | Design dashboard layout | `docs/dashboard-design.md` |
| 2-3 | Create customer-dashboard.json | `grafana/dashboards/customer-dashboard.json` |
| 4 | Create ConfigMap and update values | Updated `dashboard-configmap.yaml` |
| 5 | Test with multi-tenant deployment | Verified namespace filtering works |

### Week 2: Terraform Foundation (Phase 2)

| Day | Task | Deliverable |
|-----|------|-------------|
| 1-2 | Create module structure | `terraform/modules/` skeleton |
| 3 | Kubernetes cluster module | `modules/kubernetes-cluster/` |
| 4 | PostgreSQL module | `modules/postgresql/` |
| 5 | Redis module | `modules/redis/` |

### Week 3: Terraform Environments (Phase 2 continued)

| Day | Task | Deliverable |
|-----|------|-------------|
| 1 | Networking module | `modules/networking/` |
| 2 | DNS module | `modules/dns/` |
| 3 | Dev environment config | `environments/dev/` |
| 4 | Staging environment config | `environments/staging/` |
| 5 | Production environment config | `environments/production/` |

### Week 4: Helm Charts (Phase 3)

| Day | Task | Deliverable |
|-----|------|-------------|
| 1-2 | lumo-api chart | `helm/lumo-api/` |
| 3 | lumo-infrastructure chart | `helm/lumo-infrastructure/` |
| 4 | lumo-monitoring chart | `helm/lumo-monitoring/` |
| 5 | lumo-stack umbrella chart | `helm/lumo-stack/` |

### Week 5: ArgoCD Integration (Phase 4)

| Day | Task | Deliverable |
|-----|------|-------------|
| 1 | ArgoCD bootstrap | `argocd/bootstrap/` |
| 2 | App-of-apps setup | `argocd/apps/` |
| 3 | ApplicationSets for tenants | `argocd/applicationsets/` |
| 4 | Projects and RBAC | `argocd/projects/` |
| 5 | Documentation | `argocd/README.md` |

### Week 6: Integration & Testing (Phase 5)

| Day | Task | Deliverable |
|-----|------|-------------|
| 1-2 | Script refactoring | `scripts/` modules |
| 3-4 | End-to-end testing | Test reports |
| 5 | Documentation updates | Updated `CLAUDE.md`, `README.md` |

---

## Detailed TODO List

### Phase 1: Customer Dashboard

- [ ] **DASH-1.1** Create `customer-dashboard.json` file
  - [ ] Add namespace template variable with regex filter
  - [ ] Create header row with stat panels (agents, events, analyses)
  - [ ] Create events overview row (severity bar, type pie)
  - [ ] Create event processing row (rate, duration)
  - [ ] Create AI analysis row (rate, duration p50/95/99)
  - [ ] Create agent health row (table, heartbeats, cache)
  - [ ] Create resource usage row (CPU/memory)
  - [ ] Add quota status gauges

- [ ] **DASH-1.2** Update `dashboard-configmap.yaml`
  - [ ] Add customer-dashboard.json as data entry
  - [ ] Update labels for sidecar discovery

- [ ] **DASH-1.3** Update `values.yaml`
  - [ ] Add dashboard provider for customer folder
  - [ ] Configure dashboard ConfigMap reference

- [ ] **DASH-1.4** Update `README.md`
  - [ ] Document new customer dashboard
  - [ ] Add access instructions
  - [ ] Document namespace selection

- [ ] **DASH-1.5** Test with multi-tenant deployment
  - [ ] Deploy using deploy-saas.sh
  - [ ] Verify namespace filtering works
  - [ ] Verify metrics populate correctly
  - [ ] Screenshot for documentation

### Phase 2: Terraform Infrastructure

- [ ] **TF-2.1** Create module structure
  - [ ] Create `infrastructure/terraform/` directory
  - [ ] Create `modules/` subdirectory
  - [ ] Create `environments/` subdirectory
  - [ ] Add `.gitignore` for terraform state

- [ ] **TF-2.2** Kubernetes cluster module
  - [ ] Create `modules/kubernetes-cluster/main.tf`
  - [ ] Create `modules/kubernetes-cluster/variables.tf`
  - [ ] Create `modules/kubernetes-cluster/outputs.tf`
  - [ ] Create `modules/kubernetes-cluster/versions.tf`
  - [ ] Add EKS configuration
  - [ ] Add GKE configuration (optional)
  - [ ] Add node groups configuration

- [ ] **TF-2.3** PostgreSQL module
  - [ ] Create `modules/postgresql/main.tf`
  - [ ] Create `modules/postgresql/variables.tf`
  - [ ] Create `modules/postgresql/outputs.tf`
  - [ ] Add RDS configuration
  - [ ] Add parameter group
  - [ ] Add security group
  - [ ] Add subnet group

- [ ] **TF-2.4** Redis module
  - [ ] Create `modules/redis/main.tf`
  - [ ] Create `modules/redis/variables.tf`
  - [ ] Create `modules/redis/outputs.tf`
  - [ ] Add ElastiCache configuration
  - [ ] Add replication group
  - [ ] Add security group
  - [ ] Add subnet group

- [ ] **TF-2.5** Networking module
  - [ ] Create `modules/networking/main.tf`
  - [ ] Add VPC configuration
  - [ ] Add public subnets
  - [ ] Add private subnets
  - [ ] Add NAT gateway
  - [ ] Add internet gateway
  - [ ] Add route tables

- [ ] **TF-2.6** DNS module
  - [ ] Create `modules/dns/main.tf`
  - [ ] Add Route53 zone
  - [ ] Add A records
  - [ ] Add CNAME records

- [ ] **TF-2.7** Environment configurations
  - [ ] Create `environments/dev/main.tf`
  - [ ] Create `environments/dev/terraform.tfvars`
  - [ ] Create `environments/staging/main.tf`
  - [ ] Create `environments/staging/terraform.tfvars`
  - [ ] Create `environments/production/main.tf`
  - [ ] Create `environments/production/terraform.tfvars`
  - [ ] Add backend configurations (S3)

- [ ] **TF-2.8** Documentation
  - [ ] Create `infrastructure/terraform/README.md`
  - [ ] Document module usage
  - [ ] Document environment setup
  - [ ] Add architecture diagram

### Phase 3: Helm Charts

- [ ] **HELM-3.1** lumo-api chart
  - [ ] Create `Chart.yaml`
  - [ ] Create `values.yaml`
  - [ ] Create `values-production.yaml`
  - [ ] Create `templates/deployment.yaml`
  - [ ] Create `templates/service.yaml`
  - [ ] Create `templates/ingress.yaml`
  - [ ] Create `templates/configmap.yaml`
  - [ ] Create `templates/hpa.yaml`
  - [ ] Create `templates/pdb.yaml`
  - [ ] Create `templates/networkpolicy.yaml`
  - [ ] Create `templates/servicemonitor.yaml`
  - [ ] Create `templates/_helpers.tpl`
  - [ ] Create `templates/NOTES.txt`

- [ ] **HELM-3.2** Enhance lumo-agent chart
  - [ ] Add `values-production.yaml`
  - [ ] Add `templates/servicemonitor.yaml`
  - [ ] Update deployment template
  - [ ] Add tenant-aware configuration

- [ ] **HELM-3.3** lumo-infrastructure chart
  - [ ] Create `Chart.yaml`
  - [ ] Create `values.yaml`
  - [ ] Create `templates/postgresql/statefulset.yaml`
  - [ ] Create `templates/postgresql/service.yaml`
  - [ ] Create `templates/postgresql/configmap.yaml`
  - [ ] Create `templates/postgresql/pvc.yaml`
  - [ ] Create `templates/redis/deployment.yaml`
  - [ ] Create `templates/redis/service.yaml`

- [ ] **HELM-3.4** lumo-monitoring chart
  - [ ] Create `Chart.yaml`
  - [ ] Create `values.yaml`
  - [ ] Add Prometheus subchart reference
  - [ ] Add Grafana subchart reference
  - [ ] Add dashboard ConfigMaps
  - [ ] Add alerting rules

- [ ] **HELM-3.5** lumo-stack umbrella chart
  - [ ] Create `Chart.yaml` with dependencies
  - [ ] Create `values.yaml`
  - [ ] Create `values-kind.yaml`
  - [ ] Create `values-staging.yaml`
  - [ ] Create `values-production.yaml`
  - [ ] Test `helm dependency update`
  - [ ] Test `helm install` in kind

### Phase 4: ArgoCD Integration

- [ ] **ARGO-4.1** Bootstrap configuration
  - [ ] Create `argocd/bootstrap/argocd-install.yaml`
  - [ ] Create `argocd/bootstrap/argocd-cm.yaml`
  - [ ] Create `argocd/bootstrap/app-of-apps.yaml`
  - [ ] Test ArgoCD installation

- [ ] **ARGO-4.2** Application definitions
  - [ ] Create `argocd/apps/lumo-system/lumo-api.yaml`
  - [ ] Create `argocd/apps/lumo-system/lumo-infrastructure.yaml`
  - [ ] Create `argocd/apps/lumo-system/lumo-monitoring.yaml`
  - [ ] Create `argocd/apps/platform/ingress-nginx.yaml`
  - [ ] Create `argocd/apps/platform/cert-manager.yaml`

- [ ] **ARGO-4.3** Tenant ApplicationSets
  - [ ] Create `argocd/applicationsets/tenant-agents.yaml`
  - [ ] Create `argocd/applicationsets/environments.yaml`
  - [ ] Test dynamic application generation

- [ ] **ARGO-4.4** Projects and RBAC
  - [ ] Create `argocd/projects/platform.yaml`
  - [ ] Create `argocd/projects/lumo-system.yaml`
  - [ ] Create `argocd/projects/tenants.yaml`
  - [ ] Configure role bindings

- [ ] **ARGO-4.5** Documentation
  - [ ] Create `argocd/README.md`
  - [ ] Document application structure
  - [ ] Document sync policies
  - [ ] Add troubleshooting guide

### Phase 5: Production Migration

- [ ] **MIG-5.1** Script refactoring
  - [ ] Create `scripts/kind/setup-cluster.sh`
  - [ ] Create `scripts/kind/build-images.sh`
  - [ ] Create `scripts/deploy/deploy-infrastructure.sh`
  - [ ] Create `scripts/deploy/deploy-api.sh`
  - [ ] Create `scripts/tenant/create-tenant.sh`
  - [ ] Create `scripts/tenant/provision-agent.sh`
  - [ ] Create `scripts/validate/validate-deployment.sh`

- [ ] **MIG-5.2** Update deploy-saas.sh
  - [ ] Refactor to use modular scripts
  - [ ] Add `--use-helm` flag option
  - [ ] Maintain backwards compatibility
  - [ ] Update documentation

- [ ] **MIG-5.3** Makefile targets
  - [ ] Add `deploy-helm` target
  - [ ] Add `deploy-argocd` target
  - [ ] Add `terraform-plan` target
  - [ ] Add `terraform-apply` target

- [ ] **MIG-5.4** Documentation updates
  - [ ] Update `CLAUDE.md` with new infrastructure
  - [ ] Update `README.md` with deployment options
  - [ ] Create `docs/deployment-guide.md`
  - [ ] Create `docs/gitops-workflow.md`

- [ ] **MIG-5.5** Testing
  - [ ] Test kind deployment with Helm
  - [ ] Test Terraform plan in dev
  - [ ] Test ArgoCD sync
  - [ ] Run E2E tests
  - [ ] Document results

---

## Dependencies & Prerequisites

### Tools Required

| Tool | Version | Purpose |
|------|---------|---------|
| Terraform | >= 1.5.0 | Infrastructure as code |
| Helm | >= 3.12 | Kubernetes package manager |
| ArgoCD CLI | >= 2.8 | GitOps deployments |
| kubectl | >= 1.28 | Kubernetes CLI |
| kind | >= 0.20 | Local testing |
| Docker | >= 24.0 | Container runtime |

### External Services (Production)

| Service | Provider Options |
|---------|------------------|
| Kubernetes | EKS, GKE, AKS |
| PostgreSQL | RDS, Cloud SQL, Azure Database |
| Redis | ElastiCache, Memorystore, Azure Cache |
| DNS | Route53, Cloud DNS, Azure DNS |
| Secrets | Vault, AWS Secrets Manager, Azure Key Vault |
| Container Registry | ECR, GCR, ACR, GHCR |

---

## Success Criteria

### Phase 1 Complete When:
- [ ] Customer dashboard displays metrics for selected namespace
- [ ] All panels populate with correct data
- [ ] Namespace dropdown shows all tenant-* namespaces
- [ ] Documentation updated

### Phase 2 Complete When:
- [ ] `terraform plan` succeeds for all environments
- [ ] Dev environment can be created with `terraform apply`
- [ ] Infrastructure resources are tagged correctly
- [ ] Outputs available for Helm/ArgoCD

### Phase 3 Complete When:
- [ ] `helm install lumo-stack` works in kind
- [ ] All subcharts render correctly
- [ ] Production values include all security settings
- [ ] ServiceMonitors enable Prometheus scraping

### Phase 4 Complete When:
- [ ] ArgoCD syncs all applications successfully
- [ ] ApplicationSets generate tenant apps dynamically
- [ ] Sync policies enforce desired state
- [ ] RBAC restricts access appropriately

### Phase 5 Complete When:
- [ ] Both legacy script and new tooling work
- [ ] E2E tests pass with both approaches
- [ ] Documentation complete
- [ ] Team trained on new workflow

---

## References

- [ArgoCD Documentation](https://argo-cd.readthedocs.io/)
- [Terraform AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
- [Helm Best Practices](https://helm.sh/docs/chart_best_practices/)
- [Grafana Dashboard JSON Model](https://grafana.com/docs/grafana/latest/dashboards/json-model/)
- [Lumo Multi-Tenant Architecture](./multi-tenant-architecture.md)
- [Lumo Phase 18 Plan](./PHASE_18_MULTI_TENANT_SAAS.md)
