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

Define cloud infrastructure as code using Terraform with GCP as the primary cloud provider.

### Directory Structure

```
infrastructure/
├── terraform/
│   ├── modules/
│   │   ├── gke-cluster/               # GKE Autopilot or Standard cluster
│   │   │   ├── main.tf
│   │   │   ├── variables.tf
│   │   │   ├── outputs.tf
│   │   │   └── versions.tf
│   │   ├── cloud-sql/                 # Cloud SQL for PostgreSQL
│   │   │   ├── main.tf
│   │   │   ├── variables.tf
│   │   │   └── outputs.tf
│   │   ├── memorystore/               # Memorystore for Redis
│   │   │   ├── main.tf
│   │   │   ├── variables.tf
│   │   │   └── outputs.tf
│   │   ├── networking/                # VPC, Subnets, Firewall Rules
│   │   │   ├── main.tf
│   │   │   ├── variables.tf
│   │   │   └── outputs.tf
│   │   ├── observability/             # Cloud Monitoring, Cloud Logging
│   │   │   ├── main.tf
│   │   │   ├── variables.tf
│   │   │   └── outputs.tf
│   │   └── dns/                       # Cloud DNS
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

#### 2.1 GKE Cluster Module

**File:** `infrastructure/terraform/modules/gke-cluster/main.tf`

```hcl
# GKE Autopilot Cluster (recommended for simplicity)
resource "google_container_cluster" "lumo" {
  name     = var.cluster_name
  location = var.region

  # Autopilot mode - Google manages nodes
  enable_autopilot = var.autopilot_enabled

  # Network configuration
  network    = var.network
  subnetwork = var.subnetwork

  # Private cluster configuration
  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false
    master_ipv4_cidr_block  = var.master_ipv4_cidr_block
  }

  # IP allocation policy for VPC-native cluster
  ip_allocation_policy {
    cluster_secondary_range_name  = var.pods_range_name
    services_secondary_range_name = var.services_range_name
  }

  # Workload Identity for secure pod authentication
  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  # Release channel for automatic upgrades
  release_channel {
    channel = var.release_channel  # RAPID, REGULAR, or STABLE
  }

  # Maintenance window
  maintenance_policy {
    recurring_window {
      start_time = "2025-01-01T09:00:00Z"
      end_time   = "2025-01-01T17:00:00Z"
      recurrence = "FREQ=WEEKLY;BYDAY=SA,SU"
    }
  }

  # Binary Authorization (optional, for production)
  binary_authorization {
    evaluation_mode = var.environment == "production" ? "PROJECT_SINGLETON_POLICY_ENFORCE" : "DISABLED"
  }

  # Logging and monitoring
  logging_config {
    enable_components = ["SYSTEM_COMPONENTS", "WORKLOADS"]
  }

  monitoring_config {
    enable_components = ["SYSTEM_COMPONENTS"]
    managed_prometheus {
      enabled = true
    }
  }

  # Labels
  resource_labels = var.labels

  deletion_protection = var.environment == "production"
}

# Standard node pool (if not using Autopilot)
resource "google_container_node_pool" "lumo_nodes" {
  count = var.autopilot_enabled ? 0 : 1

  name       = "lumo-node-pool"
  location   = var.region
  cluster    = google_container_cluster.lumo.name

  initial_node_count = var.initial_node_count

  autoscaling {
    min_node_count = var.min_nodes
    max_node_count = var.max_nodes
  }

  node_config {
    machine_type = var.machine_type
    disk_size_gb = var.disk_size_gb
    disk_type    = "pd-ssd"

    # Workload Identity
    workload_metadata_config {
      mode = "GKE_METADATA"
    }

    # Security
    shielded_instance_config {
      enable_secure_boot          = true
      enable_integrity_monitoring = true
    }

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]

    labels = var.labels

    tags = ["lumo-node", var.environment]
  }

  management {
    auto_repair  = true
    auto_upgrade = true
  }
}
```

**Variables:**
```hcl
variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "cluster_name" {
  description = "GKE cluster name"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "us-central1"
}

variable "network" {
  description = "VPC network name"
  type        = string
}

variable "subnetwork" {
  description = "VPC subnetwork name"
  type        = string
}

variable "autopilot_enabled" {
  description = "Enable GKE Autopilot mode"
  type        = bool
  default     = true
}

variable "min_nodes" {
  description = "Minimum number of nodes (Standard mode only)"
  type        = number
  default     = 2
}

variable "max_nodes" {
  description = "Maximum number of nodes (Standard mode only)"
  type        = number
  default     = 10
}

variable "initial_node_count" {
  description = "Initial number of nodes (Standard mode only)"
  type        = number
  default     = 3
}

variable "machine_type" {
  description = "Machine type for nodes (Standard mode only)"
  type        = string
  default     = "e2-standard-4"
}

variable "disk_size_gb" {
  description = "Disk size in GB for nodes"
  type        = number
  default     = 100
}

variable "release_channel" {
  description = "GKE release channel (RAPID, REGULAR, STABLE)"
  type        = string
  default     = "REGULAR"
}

variable "master_ipv4_cidr_block" {
  description = "CIDR block for the master network"
  type        = string
  default     = "172.16.0.0/28"
}

variable "pods_range_name" {
  description = "Name of the secondary range for pods"
  type        = string
}

variable "services_range_name" {
  description = "Name of the secondary range for services"
  type        = string
}

variable "environment" {
  description = "Environment name (dev, staging, production)"
  type        = string
}

variable "labels" {
  description = "Labels to apply to all resources"
  type        = map(string)
  default     = {}
}
```

#### 2.2 Cloud SQL Module

**File:** `infrastructure/terraform/modules/cloud-sql/main.tf`

```hcl
# Cloud SQL for PostgreSQL
resource "google_sql_database_instance" "lumo" {
  name             = "${var.environment}-lumo-postgres"
  database_version = "POSTGRES_15"
  region           = var.region
  project          = var.project_id

  settings {
    tier              = var.tier
    availability_type = var.high_availability ? "REGIONAL" : "ZONAL"
    disk_size         = var.disk_size_gb
    disk_type         = "PD_SSD"
    disk_autoresize   = true

    # IP configuration
    ip_configuration {
      ipv4_enabled    = false
      private_network = var.network_id
      require_ssl     = true
    }

    # Backup configuration
    backup_configuration {
      enabled                        = true
      start_time                     = "03:00"
      point_in_time_recovery_enabled = true
      transaction_log_retention_days = var.backup_retention_days
      backup_retention_settings {
        retained_backups = var.backup_retention_days
        retention_unit   = "COUNT"
      }
    }

    # Maintenance window
    maintenance_window {
      day          = 7  # Sunday
      hour         = 4  # 4 AM
      update_track = "stable"
    }

    # Database flags
    database_flags {
      name  = "log_checkpoints"
      value = "on"
    }

    database_flags {
      name  = "log_connections"
      value = "on"
    }

    database_flags {
      name  = "log_disconnections"
      value = "on"
    }

    database_flags {
      name  = "log_lock_waits"
      value = "on"
    }

    # Insights (query performance)
    insights_config {
      query_insights_enabled  = true
      query_string_length     = 1024
      record_application_tags = true
      record_client_address   = true
    }

    user_labels = var.labels
  }

  deletion_protection = var.environment == "production"
}

# Database
resource "google_sql_database" "lumo" {
  name     = "lumo"
  instance = google_sql_database_instance.lumo.name
  project  = var.project_id
}

# Database user
resource "google_sql_user" "lumo" {
  name     = "lumo"
  instance = google_sql_database_instance.lumo.name
  password = var.db_password
  project  = var.project_id
}

# Private service connection (for VPC peering)
resource "google_compute_global_address" "private_ip_range" {
  name          = "${var.environment}-lumo-postgres-ip"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = var.network_id
  project       = var.project_id
}

resource "google_service_networking_connection" "private_vpc_connection" {
  network                 = var.network_id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_range.name]
}
```

#### 2.3 Memorystore Module

**File:** `infrastructure/terraform/modules/memorystore/main.tf`

```hcl
# Memorystore for Redis
resource "google_redis_instance" "lumo" {
  name           = "${var.environment}-lumo-redis"
  tier           = var.high_availability ? "STANDARD_HA" : "BASIC"
  memory_size_gb = var.memory_size_gb
  region         = var.region
  project        = var.project_id

  redis_version = "REDIS_7_0"

  # Network configuration
  authorized_network = var.network_id
  connect_mode       = "PRIVATE_SERVICE_ACCESS"

  # Auth
  auth_enabled = true

  # TLS
  transit_encryption_mode = "SERVER_AUTHENTICATION"

  # Maintenance window
  maintenance_policy {
    weekly_maintenance_window {
      day = "SUNDAY"
      start_time {
        hours   = 4
        minutes = 0
      }
    }
  }

  # Redis configuration
  redis_configs = {
    maxmemory-policy = "volatile-lru"
    notify-keyspace-events = "Ex"
  }

  labels = var.labels

  lifecycle {
    prevent_destroy = var.environment == "production"
  }
}

# Output the auth string
output "auth_string" {
  value     = google_redis_instance.lumo.auth_string
  sensitive = true
}

output "host" {
  value = google_redis_instance.lumo.host
}

output "port" {
  value = google_redis_instance.lumo.port
}
```

#### 2.4 Networking Module

**File:** `infrastructure/terraform/modules/networking/main.tf`

```hcl
# VPC Network
resource "google_compute_network" "lumo" {
  name                    = "${var.environment}-lumo-vpc"
  auto_create_subnetworks = false
  project                 = var.project_id
}

# Subnetwork for GKE
resource "google_compute_subnetwork" "lumo" {
  name          = "${var.environment}-lumo-subnet"
  ip_cidr_range = var.subnet_cidr
  region        = var.region
  network       = google_compute_network.lumo.id
  project       = var.project_id

  # Secondary ranges for GKE pods and services
  secondary_ip_range {
    range_name    = "pods"
    ip_cidr_range = var.pods_cidr
  }

  secondary_ip_range {
    range_name    = "services"
    ip_cidr_range = var.services_cidr
  }

  private_ip_google_access = true

  log_config {
    aggregation_interval = "INTERVAL_5_SEC"
    flow_sampling        = 0.5
    metadata             = "INCLUDE_ALL_METADATA"
  }
}

# Cloud NAT for private nodes
resource "google_compute_router" "lumo" {
  name    = "${var.environment}-lumo-router"
  region  = var.region
  network = google_compute_network.lumo.id
  project = var.project_id
}

resource "google_compute_router_nat" "lumo" {
  name                               = "${var.environment}-lumo-nat"
  router                             = google_compute_router.lumo.name
  region                             = var.region
  project                            = var.project_id
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}

# Firewall rules
resource "google_compute_firewall" "allow_internal" {
  name    = "${var.environment}-lumo-allow-internal"
  network = google_compute_network.lumo.name
  project = var.project_id

  allow {
    protocol = "tcp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "udp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "icmp"
  }

  source_ranges = [var.subnet_cidr, var.pods_cidr, var.services_cidr]
}

resource "google_compute_firewall" "allow_health_checks" {
  name    = "${var.environment}-lumo-allow-health-checks"
  network = google_compute_network.lumo.name
  project = var.project_id

  allow {
    protocol = "tcp"
  }

  # GCP health check ranges
  source_ranges = ["130.211.0.0/22", "35.191.0.0/16"]
  target_tags   = ["lumo-node"]
}
```

#### 2.5 Environment Configuration

**File:** `infrastructure/terraform/environments/production/main.tf`

```hcl
terraform {
  required_version = ">= 1.5.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
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

  backend "gcs" {
    bucket = "lumo-terraform-state"
    prefix = "production"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

locals {
  environment = "production"
  labels = {
    project     = "lumo"
    environment = "production"
    managed_by  = "terraform"
  }
}

# Networking
module "networking" {
  source = "../../modules/networking"

  project_id    = var.project_id
  environment   = local.environment
  region        = var.region
  subnet_cidr   = "10.0.0.0/20"
  pods_cidr     = "10.16.0.0/14"
  services_cidr = "10.20.0.0/20"
}

# GKE Cluster (Autopilot)
module "gke" {
  source = "../../modules/gke-cluster"

  project_id         = var.project_id
  cluster_name       = "lumo-production"
  region             = var.region
  environment        = local.environment
  autopilot_enabled  = true
  network            = module.networking.network_name
  subnetwork         = module.networking.subnetwork_name
  pods_range_name    = "pods"
  services_range_name = "services"
  release_channel    = "STABLE"
  labels             = local.labels

  depends_on = [module.networking]
}

# Cloud SQL for PostgreSQL
module "cloud_sql" {
  source = "../../modules/cloud-sql"

  project_id          = var.project_id
  environment         = local.environment
  region              = var.region
  network_id          = module.networking.network_id
  tier                = "db-custom-4-16384"  # 4 vCPU, 16GB RAM
  disk_size_gb        = 100
  high_availability   = true
  backup_retention_days = 30
  db_password         = var.db_password
  labels              = local.labels

  depends_on = [module.networking]
}

# Memorystore for Redis
module "memorystore" {
  source = "../../modules/memorystore"

  project_id       = var.project_id
  environment      = local.environment
  region           = var.region
  network_id       = module.networking.network_id
  memory_size_gb   = 5
  high_availability = true
  labels           = local.labels

  depends_on = [module.networking]
}

# Outputs for Helm/ArgoCD
output "cluster_name" {
  value = module.gke.cluster_name
}

output "cluster_endpoint" {
  value     = module.gke.cluster_endpoint
  sensitive = true
}

output "cluster_ca_certificate" {
  value     = module.gke.cluster_ca_certificate
  sensitive = true
}

output "database_connection_name" {
  value = module.cloud_sql.connection_name
}

output "database_private_ip" {
  value = module.cloud_sql.private_ip_address
}

output "redis_host" {
  value = module.memorystore.host
}

output "redis_port" {
  value = module.memorystore.port
}
```

**File:** `infrastructure/terraform/environments/production/variables.tf`

```hcl
variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "us-central1"
}

variable "db_password" {
  description = "Database password"
  type        = string
  sensitive   = true
}
```

**File:** `infrastructure/terraform/environments/production/terraform.tfvars`

```hcl
project_id = "lumo-production"
region     = "us-central1"
# db_password should be provided via TF_VAR_db_password environment variable
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

- [ ] **TF-2.2** GKE cluster module
  - [ ] Create `modules/gke-cluster/main.tf`
  - [ ] Create `modules/gke-cluster/variables.tf`
  - [ ] Create `modules/gke-cluster/outputs.tf`
  - [ ] Create `modules/gke-cluster/versions.tf`
  - [ ] Add Autopilot configuration
  - [ ] Add Standard node pool configuration (optional)
  - [ ] Add Workload Identity configuration
  - [ ] Add private cluster configuration

- [ ] **TF-2.3** Cloud SQL module
  - [ ] Create `modules/cloud-sql/main.tf`
  - [ ] Create `modules/cloud-sql/variables.tf`
  - [ ] Create `modules/cloud-sql/outputs.tf`
  - [ ] Add PostgreSQL 15 configuration
  - [ ] Add private service connection
  - [ ] Add backup configuration
  - [ ] Add Query Insights

- [ ] **TF-2.4** Memorystore module
  - [ ] Create `modules/memorystore/main.tf`
  - [ ] Create `modules/memorystore/variables.tf`
  - [ ] Create `modules/memorystore/outputs.tf`
  - [ ] Add Redis 7.0 configuration
  - [ ] Add HA configuration
  - [ ] Add TLS configuration

- [ ] **TF-2.5** Networking module
  - [ ] Create `modules/networking/main.tf`
  - [ ] Add VPC configuration
  - [ ] Add subnetwork with secondary ranges (pods, services)
  - [ ] Add Cloud NAT for private nodes
  - [ ] Add Cloud Router
  - [ ] Add firewall rules

- [ ] **TF-2.6** DNS module
  - [ ] Create `modules/dns/main.tf`
  - [ ] Add Cloud DNS zone
  - [ ] Add A records
  - [ ] Add CNAME records

- [ ] **TF-2.7** Environment configurations
  - [ ] Create `environments/dev/main.tf`
  - [ ] Create `environments/dev/terraform.tfvars`
  - [ ] Create `environments/staging/main.tf`
  - [ ] Create `environments/staging/terraform.tfvars`
  - [ ] Create `environments/production/main.tf`
  - [ ] Create `environments/production/terraform.tfvars`
  - [ ] Add GCS backend configurations

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

### External Services (Production - GCP)

| Service | GCP Product | Notes |
|---------|-------------|-------|
| Kubernetes | GKE Autopilot | Recommended for simplicity, or Standard for more control |
| PostgreSQL | Cloud SQL | Managed PostgreSQL 15 with HA |
| Redis | Memorystore | Managed Redis 7.0 with HA |
| DNS | Cloud DNS | Managed DNS zones |
| Secrets | Secret Manager | Native GCP secrets management |
| Container Registry | Artifact Registry | GCR replacement, recommended |
| Monitoring | Cloud Monitoring | Native integration with GKE |
| Logging | Cloud Logging | Native integration with GKE |

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
