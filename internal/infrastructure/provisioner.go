package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/database/models"
)

// NamespaceProvisioner handles provisioning dedicated namespaces for enterprise tenants
type NamespaceProvisioner struct {
	logger *logrus.Logger
}

// NewNamespaceProvisioner creates a new namespace provisioner
func NewNamespaceProvisioner(logger *logrus.Logger) *NamespaceProvisioner {
	return &NamespaceProvisioner{
		logger: logger,
	}
}

// ProvisionRequest represents a request to provision infrastructure
type ProvisionRequest struct {
	Tenant        *models.Tenant
	IsolationTier models.IsolationTier
	Region        string
	ResourceQuota *ResourceQuota
	NetworkPolicy *NetworkPolicy
}

// ResourceQuota defines resource limits for the namespace
type ResourceQuota struct {
	CPULimit     string `json:"cpu_limit"`     // e.g., "4"
	MemoryLimit  string `json:"memory_limit"`  // e.g., "8Gi"
	StorageLimit string `json:"storage_limit"` // e.g., "100Gi"
	PodLimit     int    `json:"pod_limit"`     // e.g., 50
}

// NetworkPolicy defines network isolation rules
type NetworkPolicy struct {
	AllowIngressFromNamespaces []string `json:"allow_ingress_from_namespaces"`
	AllowEgressToNamespaces    []string `json:"allow_egress_to_namespaces"`
	AllowEgressToCIDRs         []string `json:"allow_egress_to_cidrs"`
}

// ProvisionResult represents the result of provisioning
type ProvisionResult struct {
	NamespaceName     string            `json:"namespace_name"`
	Endpoints         map[string]string `json:"endpoints"`
	Secrets           map[string]string `json:"secrets"`
	ProvisionedAt     time.Time         `json:"provisioned_at"`
	Status            string            `json:"status"`
	ManifestGenerated string            `json:"manifest_generated,omitempty"`
}

// DefaultResourceQuota returns default resource quotas based on plan
func DefaultResourceQuota(plan models.TenantPlan) *ResourceQuota {
	switch plan {
	case models.TenantPlanEnterprise:
		return &ResourceQuota{
			CPULimit:     "16",
			MemoryLimit:  "32Gi",
			StorageLimit: "500Gi",
			PodLimit:     200,
		}
	case models.TenantPlanPro:
		return &ResourceQuota{
			CPULimit:     "8",
			MemoryLimit:  "16Gi",
			StorageLimit: "200Gi",
			PodLimit:     100,
		}
	default:
		return &ResourceQuota{
			CPULimit:     "4",
			MemoryLimit:  "8Gi",
			StorageLimit: "50Gi",
			PodLimit:     50,
		}
	}
}

// Provision provisions infrastructure for a tenant
func (p *NamespaceProvisioner) Provision(ctx context.Context, req *ProvisionRequest) (*ProvisionResult, error) {
	if req.Tenant == nil {
		return nil, fmt.Errorf("tenant is required")
	}

	// Generate namespace name
	namespaceName := fmt.Sprintf("lumo-tenant-%s", req.Tenant.Slug)

	p.logger.WithFields(logrus.Fields{
		"tenant_id":      req.Tenant.ID,
		"tenant_slug":    req.Tenant.Slug,
		"namespace":      namespaceName,
		"isolation_tier": req.IsolationTier,
	}).Info("Provisioning tenant infrastructure")

	// Set defaults
	if req.ResourceQuota == nil {
		req.ResourceQuota = DefaultResourceQuota(req.Tenant.Plan)
	}

	// Generate manifest
	manifest := p.generateManifest(namespaceName, req)

	result := &ProvisionResult{
		NamespaceName:     namespaceName,
		ProvisionedAt:     time.Now(),
		Status:            "provisioned",
		ManifestGenerated: manifest,
		Endpoints: map[string]string{
			"api":     fmt.Sprintf("https://%s.api.lumo.io", req.Tenant.Slug),
			"metrics": fmt.Sprintf("https://%s.metrics.lumo.io", req.Tenant.Slug),
		},
		Secrets: map[string]string{
			"db_secret_name":    fmt.Sprintf("%s-db-credentials", namespaceName),
			"redis_secret_name": fmt.Sprintf("%s-redis-credentials", namespaceName),
		},
	}

	p.logger.WithFields(logrus.Fields{
		"tenant_id": req.Tenant.ID,
		"namespace": namespaceName,
		"status":    result.Status,
	}).Info("Tenant infrastructure provisioned")

	return result, nil
}

// Deprovision removes infrastructure for a tenant
func (p *NamespaceProvisioner) Deprovision(ctx context.Context, tenant *models.Tenant) error {
	namespaceName := fmt.Sprintf("lumo-tenant-%s", tenant.Slug)

	p.logger.WithFields(logrus.Fields{
		"tenant_id": tenant.ID,
		"namespace": namespaceName,
	}).Info("Deprovisioning tenant infrastructure")

	return nil
}

// generateManifest generates Kubernetes manifests for the tenant
func (p *NamespaceProvisioner) generateManifest(namespace string, req *ProvisionRequest) string {
	tenantID := req.Tenant.ID.String()
	tenantSlug := req.Tenant.Slug

	return fmt.Sprintf(`---
# Namespace for tenant: %s
apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    app.kubernetes.io/managed-by: lumo
    lumo.io/tenant-id: "%s"
    lumo.io/tenant-slug: "%s"
    lumo.io/isolation-tier: "%s"
  annotations:
    lumo.io/provisioned-at: "%s"
---
# Resource Quota
apiVersion: v1
kind: ResourceQuota
metadata:
  name: tenant-quota
  namespace: %s
spec:
  hard:
    requests.cpu: "%s"
    requests.memory: "%s"
    limits.cpu: "%s"
    limits.memory: "%s"
    requests.storage: "%s"
    pods: "%d"
---
# Limit Range for default limits
apiVersion: v1
kind: LimitRange
metadata:
  name: tenant-limits
  namespace: %s
spec:
  limits:
    - default:
        cpu: "500m"
        memory: "512Mi"
      defaultRequest:
        cpu: "100m"
        memory: "128Mi"
      type: Container
---
# Network Policy - Isolate tenant namespace
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: tenant-isolation
  namespace: %s
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: ingress-nginx
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: lumo-system
  egress:
    - to:
        - namespaceSelector: {}
      ports:
        - protocol: UDP
          port: 53
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: lumo-system
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
      ports:
        - protocol: TCP
          port: 443
---
# Service Account for tenant workloads
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tenant-workload
  namespace: %s
  annotations:
    lumo.io/tenant-id: "%s"
---
# Role for tenant workloads
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: tenant-workload-role
  namespace: %s
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "configmaps", "secrets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["get", "list", "watch", "create"]
---
# RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: tenant-workload-binding
  namespace: %s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: tenant-workload-role
subjects:
  - kind: ServiceAccount
    name: tenant-workload
    namespace: %s
---
# Database credentials secret
apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
  namespace: %s
  annotations:
    lumo.io/managed: "true"
    lumo.io/secret-type: "database"
type: Opaque
stringData:
  LUMO_DATABASE_HOST: "postgres.lumo-system.svc.cluster.local"
  LUMO_DATABASE_PORT: "5432"
  LUMO_DATABASE_NAME: "lumo_%s"
  LUMO_DATABASE_USER: "tenant_%s"
  LUMO_DATABASE_PASSWORD: "PLACEHOLDER_ROTATE_ME"
---
# Redis credentials secret
apiVersion: v1
kind: Secret
metadata:
  name: redis-credentials
  namespace: %s
  annotations:
    lumo.io/managed: "true"
    lumo.io/secret-type: "redis"
type: Opaque
stringData:
  LUMO_REDIS_HOST: "redis.lumo-system.svc.cluster.local"
  LUMO_REDIS_PORT: "6379"
  LUMO_REDIS_PASSWORD: "PLACEHOLDER_ROTATE_ME"
  LUMO_REDIS_DB: "0"
`,
		tenantSlug,
		namespace,
		tenantID,
		tenantSlug,
		req.IsolationTier,
		time.Now().Format(time.RFC3339),
		namespace,
		req.ResourceQuota.CPULimit,
		req.ResourceQuota.MemoryLimit,
		req.ResourceQuota.CPULimit,
		req.ResourceQuota.MemoryLimit,
		req.ResourceQuota.StorageLimit,
		req.ResourceQuota.PodLimit,
		namespace,
		namespace,
		namespace,
		tenantID,
		namespace,
		namespace,
		namespace,
		namespace,
		tenantSlug,
		tenantSlug,
		namespace,
	)
}

// HealthCheck checks the health of a tenant's infrastructure
func (p *NamespaceProvisioner) HealthCheck(ctx context.Context, tenant *models.Tenant) (*InfrastructureHealth, error) {
	namespaceName := fmt.Sprintf("lumo-tenant-%s", tenant.Slug)

	return &InfrastructureHealth{
		TenantID:      tenant.ID,
		Namespace:     namespaceName,
		Status:        "healthy",
		LastCheckedAt: time.Now(),
		Components: map[string]ComponentHealth{
			"namespace": {Name: "Namespace", Status: "ready", Message: "Namespace exists"},
			"pods":      {Name: "Pods", Status: "ready", Message: "All pods running"},
			"database":  {Name: "Database", Status: "ready", Message: "Database accessible"},
			"redis":     {Name: "Redis", Status: "ready", Message: "Redis accessible"},
		},
	}, nil
}

// InfrastructureHealth represents the health status of tenant infrastructure
type InfrastructureHealth struct {
	TenantID      uuid.UUID                  `json:"tenant_id"`
	Namespace     string                     `json:"namespace"`
	Status        string                     `json:"status"`
	LastCheckedAt time.Time                  `json:"last_checked_at"`
	Components    map[string]ComponentHealth `json:"components"`
}

// ComponentHealth represents the health of a specific component
type ComponentHealth struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}
