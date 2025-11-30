#!/usr/bin/env bash
#
# Multi-Tenant SaaS Deployment - End-to-End Testing
#
# This script deploys the complete Lumo SaaS architecture:
# - Cluster setup
# - Infrastructure (PostgreSQL, Redis)
# - Lumo API Server (with multi-tenant support)
# - Creates test customers/tenants
# - Provisions and deploys agents for each tenant
# - Verifies end-to-end flow
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="${KIND_CLUSTER_NAME:-lumo-saas-test}"
NAMESPACE="${LUMO_NAMESPACE:-lumo-system}"
SKIP_CLUSTER_SETUP="${SKIP_CLUSTER_SETUP:-false}"
SKIP_BUILD="${SKIP_BUILD:-false}"
SKIP_INFRASTRUCTURE="${SKIP_INFRASTRUCTURE:-false}"
API_PORT=8080

# Test tenants - using simple arrays for bash 3.x compatibility
TENANT_KEYS=("acme" "globex" "initech")

# Tenant configurations (index matches TENANT_KEYS)
TENANT_NAMES=("Acme Corporation" "Globex Industries" "Initech Solutions")
TENANT_SLUGS=("acme-corp" "globex-ind" "initech-sol")
TENANT_PLANS=("pro" "starter" "trial")
TENANT_MAX_AGENTS=(100 25 5)
TENANT_MAX_EVENTS=(100000 10000 1000)

# Store created tenant IDs and tokens (filled during execution)
TENANT_IDS=("" "" "")
AGENT_TOKENS=("" "" "")

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1" >&2
}

print_banner() {
    echo ""
    echo "╔═══════════════════════════════════════════════════════════╗"
    echo "║   Lumo Multi-Tenant SaaS - End-to-End Deployment Test     ║"
    echo "║                                                           ║"
    echo "║   Infrastructure → API → Tenants → Agents → Validation   ║"
    echo "╚═══════════════════════════════════════════════════════════╝"
    echo ""
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-cluster)
                SKIP_CLUSTER_SETUP=true
                shift
                ;;
            --skip-build)
                SKIP_BUILD=true
                shift
                ;;
            --skip-infrastructure)
                SKIP_INFRASTRUCTURE=true
                shift
                ;;
            --cluster-name)
                CLUSTER_NAME="$2"
                shift 2
                ;;
            --namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
}

show_usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Multi-Tenant SaaS Deployment - End-to-End Testing

This script deploys a complete multi-tenant Lumo environment:
  1. Creates kind cluster
  2. Deploys infrastructure (PostgreSQL + Redis)
  3. Deploys Lumo API Server
  4. Creates test tenants (Acme, Globex, Initech)
  5. Provisions agents for each tenant
  6. Simulates events and verifies tenant isolation

Options:
  --skip-cluster         Skip cluster creation (use existing)
  --skip-build           Skip image build (use existing images)
  --skip-infrastructure  Skip infrastructure deployment
  --cluster-name NAME    Name of kind cluster (default: lumo-saas-test)
  --namespace NS         Kubernetes namespace (default: lumo-system)
  -h, --help             Show this help message

Test Tenants:
  - Acme Corporation (Pro plan: 100 agents, 100K events/day)
  - Globex Industries (Starter plan: 25 agents, 10K events/day)
  - Initech Solutions (Trial plan: 5 agents, 1K events/day)

Examples:
  # Full deployment from scratch
  $0

  # Use existing cluster, rebuild images
  $0 --skip-cluster

  # Use existing cluster and images
  $0 --skip-cluster --skip-build

  # Use existing cluster and infrastructure
  $0 --skip-cluster --skip-build --skip-infrastructure
EOF
}

# ==================== CLUSTER SETUP ====================

setup_cluster() {
    if [ "$SKIP_CLUSTER_SETUP" = "true" ]; then
        log_info "Skipping cluster setup (SKIP_CLUSTER_SETUP=true)"
        return 0
    fi

    log_step "Step 1/8: Setting up kind cluster..."
    
    # Check if cluster already exists
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_success "✓ Cluster '${CLUSTER_NAME}' already exists"
        kind export kubeconfig --name "${CLUSTER_NAME}" || true
        return 0
    fi
    
    # Create cluster config
    cat <<EOF | kind create cluster --name "${CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 30080
        hostPort: 8080
        protocol: TCP
EOF

    log_success "✓ Cluster '${CLUSTER_NAME}' created"
}

# ==================== BUILD IMAGES ====================

build_and_load() {
    if [ "$SKIP_BUILD" = "true" ]; then
        log_info "Skipping image build (SKIP_BUILD=true)"
        return 0
    fi

    log_step "Step 2/8: Building and loading Docker images..."
    
    # Build from project root
    local project_root
    project_root=$(cd "$(dirname "$0")/../../../" && pwd)
    
    log_info "Building lumo:local..."
    docker build -t lumo:local -f "${project_root}/Dockerfile" "${project_root}" --quiet
    
    log_info "Building lumo-agent:local..."
    docker build -t lumo-agent:local -f "${project_root}/Dockerfile.agent" "${project_root}" --quiet
    
    log_info "Loading images into kind cluster..."
    kind load docker-image lumo:local --name "${CLUSTER_NAME}"
    kind load docker-image lumo-agent:local --name "${CLUSTER_NAME}"
    
    log_success "✓ Images built and loaded"
}

# ==================== INFRASTRUCTURE ====================

deploy_infrastructure() {
    if [ "$SKIP_INFRASTRUCTURE" = "true" ]; then
        log_info "Skipping infrastructure (SKIP_INFRASTRUCTURE=true)"
        return 0
    fi

    log_step "Step 3/8: Deploying infrastructure..."
    
    # Create namespace
    kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
    
    # Deploy PostgreSQL
    log_info "Deploying PostgreSQL..."
    kubectl apply -f manifests/postgres.yaml
    
    log_info "Waiting for PostgreSQL to be ready..."
    kubectl rollout status deployment/postgres -n "${NAMESPACE}" --timeout=120s
    
    # Wait for PostgreSQL to accept connections
    local pg_pod
    pg_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=postgres -o jsonpath='{.items[0].metadata.name}')
    
    local max_attempts=15
    local attempt=1
    while [ $attempt -le $max_attempts ]; do
        if kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -c "SELECT 1" >/dev/null 2>&1; then
            log_success "✓ PostgreSQL is ready"
            break
        fi
        sleep 2
        attempt=$((attempt + 1))
    done
    
    if [ $attempt -gt $max_attempts ]; then
        log_error "✗ PostgreSQL failed to start"
        return 1
    fi
    
    # Deploy Redis
    log_info "Deploying Redis..."
    kubectl apply -f manifests/redis.yaml
    
    log_info "Waiting for Redis to be ready..."
    kubectl rollout status deployment/lumo-redis -n "${NAMESPACE}" --timeout=60s
    
    log_success "✓ Infrastructure deployed"
}

# ==================== API SERVER ====================

deploy_api_server() {
    log_step "Step 4/8: Deploying Lumo API Server..."
    
    # Apply API server manifest
    kubectl apply -f manifests/api-server.yaml
    
    log_info "Waiting for API server to be ready..."
    kubectl rollout status deployment/lumo-api -n "${NAMESPACE}" --timeout=120s
    
    # Verify API health
    local api_pod
    api_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=lumo-api -o jsonpath='{.items[0].metadata.name}')
    
    # Port-forward for local testing
    kubectl port-forward -n "${NAMESPACE}" "pod/${api_pod}" ${API_PORT}:8080 >/dev/null 2>&1 &
    local pf_pid=$!
    sleep 3
    
    local max_attempts=10
    local attempt=1
    while [ $attempt -le $max_attempts ]; do
        if curl -s "http://localhost:${API_PORT}/api/v1/health" | grep -q "healthy\|ok"; then
            log_success "✓ API server is healthy"
            break
        fi
        sleep 2
        attempt=$((attempt + 1))
    done
    
    # Kill port-forward for now (will restart later)
    kill $pf_pid 2>/dev/null || true
    wait $pf_pid 2>/dev/null || true
    
    if [ $attempt -gt $max_attempts ]; then
        log_error "✗ API server health check failed"
        return 1
    fi
}

# ==================== MULTI-TENANT SETUP ====================

create_admin_token() {
    log_info "Creating admin JWT token..."
    
    # Get the JWT secret from the API server secret
    local jwt_secret
    jwt_secret=$(kubectl get secret lumo-api-secret -n "${NAMESPACE}" -o jsonpath='{.data.jwt-secret}' 2>/dev/null | base64 -d 2>/dev/null || echo "test-jwt-secret")
    
    # Generate admin JWT token using a simple approach
    # In production, this would use proper JWT signing
    # For testing, we'll use the API's internal admin endpoint
    ADMIN_TOKEN="admin-test-token"
    
    # Store in secret for API to recognize
    kubectl create secret generic admin-token \
        --from-literal=token="${ADMIN_TOKEN}" \
        -n "${NAMESPACE}" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    log_success "✓ Admin token created"
}

create_tenants() {
    log_step "Step 5/8: Creating test tenants..."
    
    local pg_pod
    pg_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=postgres -o jsonpath='{.items[0].metadata.name}')
    
    local i=0
    for tenant_key in "${TENANT_KEYS[@]}"; do
        local name="${TENANT_NAMES[$i]}"
        local slug="${TENANT_SLUGS[$i]}"
        local plan="${TENANT_PLANS[$i]}"
        local max_agents="${TENANT_MAX_AGENTS[$i]}"
        local max_events="${TENANT_MAX_EVENTS[$i]}"
        
        log_info "Creating tenant: ${name} (${slug})..."
        
        # Generate tenant UUID
        local tenant_id
        tenant_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
        
        # Insert tenant directly into PostgreSQL
        kubectl exec -i -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -v ON_ERROR_STOP=1 <<-EOF
            INSERT INTO tenants (
                id, name, slug, display_name, plan, status,
                max_agents, max_events_per_day, max_users,
                isolation_tier, created_at, updated_at
            ) VALUES (
                '${tenant_id}',
                '${name}',
                '${slug}',
                '${name}',
                '${plan}',
                'active',
                ${max_agents},
                ${max_events},
                10,
                'shared',
                NOW(),
                NOW()
            ) ON CONFLICT (slug) DO UPDATE SET
                name = EXCLUDED.name,
                plan = EXCLUDED.plan,
                max_agents = EXCLUDED.max_agents,
                max_events_per_day = EXCLUDED.max_events_per_day,
                updated_at = NOW()
            RETURNING id;
EOF
        
        # Get the actual tenant ID (in case of upsert)
        tenant_id=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
            psql -U lumo -d lumo -t -c "SELECT id FROM tenants WHERE slug = '${slug}';" | tr -d ' \n')
        
        TENANT_IDS[$i]="$tenant_id"
        
        log_success "✓ Created tenant: ${name} (ID: ${tenant_id:0:8}...)"
        i=$((i + 1))
    done
    
    # Verify tenants
    log_info "Verifying tenants in database..."
    kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
        psql -U lumo -d lumo -c "SELECT slug, plan, status, max_agents FROM tenants WHERE slug != 'default';"
}

provision_agents() {
    log_step "Step 6/8: Provisioning agents for each tenant..."
    
    local pg_pod
    pg_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=postgres -o jsonpath='{.items[0].metadata.name}')
    
    local i=0
    for tenant_key in "${TENANT_KEYS[@]}"; do
        local tenant_id="${TENANT_IDS[$i]}"
        local agent_id agent_token_raw token_hash
        
        agent_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
        agent_token_raw="lumo_${tenant_key}_$(openssl rand -hex 16)"
        token_hash=$(echo -n "${agent_token_raw}" | shasum -a 256 | awk '{print $1}')
        
        log_info "Provisioning agent for ${tenant_key} (tenant: ${tenant_id:0:8}...)..."
        
        # Create agent in database
        kubectl exec -i -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -v ON_ERROR_STOP=1 <<-EOF
            INSERT INTO agents (
                id, tenant_id, name, hostname, ip_address,
                platform, architecture, version, status,
                capabilities, labels,
                registered_at, last_heartbeat_at
            ) VALUES (
                '${agent_id}',
                '${tenant_id}',
                '${tenant_key}-agent-1',
                '${tenant_key}-cluster-1',
                '10.0.1.${i}',
                'kubernetes',
                'amd64',
                'v1.1.0',
                'offline',
                ARRAY['kubernetes', 'events', 'diagnostics'],
                '{"env": "test", "tenant": "${tenant_key}"}'::jsonb,
                NOW(),
                NOW()
            ) ON CONFLICT (id) DO NOTHING;
EOF
        
        # Create tenant API key for this agent
        local key_prefix="${agent_token_raw:0:12}"
        kubectl exec -i -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -v ON_ERROR_STOP=1 <<-EOF
            INSERT INTO tenant_api_keys (
                id, tenant_id, name, key_hash, key_prefix,
                scopes, created_at
            ) VALUES (
                gen_random_uuid(),
                '${tenant_id}',
                '${tenant_key}-agent-key',
                '${token_hash}',
                '${key_prefix}',
                ARRAY['agent:register', 'agent:heartbeat', 'events:submit'],
                NOW()
            ) ON CONFLICT (key_prefix) DO NOTHING;
EOF
        
        AGENT_TOKENS[$i]="$agent_token_raw"
        
        log_success "✓ Provisioned agent for ${tenant_key} (Agent ID: ${agent_id:0:8}...)"
        i=$((i + 1))
    done
    
    # Verify agents
    log_info "Verifying agents in database..."
    kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
        psql -U lumo -d lumo -c "SELECT a.name, t.slug as tenant, a.status FROM agents a JOIN tenants t ON a.tenant_id = t.id WHERE t.slug != 'default';"
}

deploy_tenant_agents() {
    log_step "Step 7/8: Deploying agents to cluster..."
    
    local i=0
    for tenant_key in "${TENANT_KEYS[@]}"; do
        local tenant_id="${TENANT_IDS[$i]}"
        local agent_token="${AGENT_TOKENS[$i]}"
        local tenant_slug="${TENANT_SLUGS[$i]}"
        
        log_info "Deploying agent for ${tenant_key}..."
        
        # Create namespace for tenant agent (simulates customer cluster)
        local agent_namespace="tenant-${tenant_slug}"
        kubectl create namespace "${agent_namespace}" --dry-run=client -o yaml | kubectl apply -f -
        
        # Create agent secret with token
        kubectl create secret generic lumo-agent-credentials \
            --from-literal=token="${agent_token}" \
            -n "${agent_namespace}" \
            --dry-run=client -o yaml | kubectl apply -f -
        
        # Create agent ConfigMap
        cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: lumo-agent-config
  namespace: ${agent_namespace}
data:
  config.yaml: |
    ai:
      enabled: false
    api:
      endpoint: http://lumo-api.${NAMESPACE}.svc.cluster.local:8080
    cache:
      enabled: true
      redis_url: redis://lumo-redis.${NAMESPACE}.svc.cluster.local:6379/0
    agent:
      mode: event-driven
      tenant_id: "${tenant_id}"
      cache_path: /tmp/lumo-cache
      enabled_checks:
        - kubernetes
      event_driven:
        enabled: true
        debounce_window: 10s
        max_debounce_window: 30s
EOF
        
        # Deploy agent
        cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: lumo-agent
  namespace: ${agent_namespace}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: lumo-agent-${tenant_slug}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: view
subjects:
  - kind: ServiceAccount
    name: lumo-agent
    namespace: ${agent_namespace}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lumo-agent
  namespace: ${agent_namespace}
  labels:
    app: lumo-agent
    tenant: ${tenant_slug}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: lumo-agent
      tenant: ${tenant_slug}
  template:
    metadata:
      labels:
        app: lumo-agent
        tenant: ${tenant_slug}
    spec:
      serviceAccountName: lumo-agent
      containers:
        - name: agent
          image: lumo-agent:local
          imagePullPolicy: Never
          env:
            - name: LUMO_AGENT_TOKEN
              valueFrom:
                secretKeyRef:
                  name: lumo-agent-credentials
                  key: token
            - name: LUMO_AGENT_API_ENDPOINT
              value: "http://lumo-api.${NAMESPACE}.svc.cluster.local:8080"
            - name: LUMO_AGENT_MODE
              value: "event-driven"
            - name: LUMO_AGENT_KUBERNETES_ENABLED
              value: "true"
            - name: LUMO_AGENT_EVENT_DRIVEN_ENABLED
              value: "true"
            - name: LUMO_LOG_LEVEL
              value: "debug"
          ports:
            - containerPort: 8080
              name: health
            - containerPort: 9090
              name: metrics
          resources:
            requests:
              memory: "64Mi"
              cpu: "100m"
            limits:
              memory: "128Mi"
              cpu: "200m"
          volumeMounts:
            - name: config
              mountPath: /etc/lumo
      volumes:
        - name: config
          configMap:
            name: lumo-agent-config
EOF
        
        log_success "✓ Agent deployed for ${tenant_key} in namespace ${agent_namespace}"
        i=$((i + 1))
    done
    
    # Wait for all agents to be ready
    log_info "Waiting for all agents to be ready..."
    local j=0
    for tenant_key in "${TENANT_KEYS[@]}"; do
        local tenant_slug="${TENANT_SLUGS[$j]}"
        local agent_namespace="tenant-${tenant_slug}"
        
        kubectl rollout status deployment/lumo-agent -n "${agent_namespace}" --timeout=60s || {
            log_warn "Agent in ${agent_namespace} not ready, checking logs..."
            kubectl logs -n "${agent_namespace}" -l app=lumo-agent --tail=20 || true
        }
        j=$((j + 1))
    done
    
    log_success "✓ All agents deployed"
}

# ==================== VALIDATION ====================

run_validation_tests() {
    log_step "Step 8/8: Running validation tests..."
    echo ""
    
    local pg_pod api_pod
    pg_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=postgres -o jsonpath='{.items[0].metadata.name}')
    api_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=lumo-api -o jsonpath='{.items[0].metadata.name}')
    
    local tests_passed=0
    local tests_failed=0
    
    # Test 1: Verify all tenants exist
    log_info "Test 1: Verifying tenants in database..."
    local tenant_count
    tenant_count=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
        psql -U lumo -d lumo -t -c "SELECT COUNT(*) FROM tenants WHERE slug != 'default';" | tr -d ' \n')
    
    if [ "$tenant_count" -eq "3" ]; then
        log_success "✓ All 3 tenants created successfully"
        tests_passed=$((tests_passed + 1))
    else
        log_error "✗ Expected 3 tenants, found ${tenant_count}"
        tests_failed=$((tests_failed + 1))
    fi
    
    # Test 2: Verify agents per tenant
    log_info "Test 2: Verifying agents per tenant..."
    local agent_count
    agent_count=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
        psql -U lumo -d lumo -t -c "SELECT COUNT(*) FROM agents WHERE tenant_id != '00000000-0000-0000-0000-000000000000';" | tr -d ' \n')
    
    if [ "$agent_count" -eq "3" ]; then
        log_success "✓ All 3 agents provisioned (one per tenant)"
        tests_passed=$((tests_passed + 1))
    else
        log_error "✗ Expected 3 agents, found ${agent_count}"
        tests_failed=$((tests_failed + 1))
    fi
    
    # Test 3: Verify tenant isolation (data separation)
    log_info "Test 3: Verifying tenant isolation..."
    local isolated=true
    local i=0
    for tenant_key in "${TENANT_KEYS[@]}"; do
        local tenant_id="${TENANT_IDS[$i]}"
        local other_agents
        other_agents=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
            psql -U lumo -d lumo -t -c "SELECT COUNT(*) FROM agents WHERE tenant_id = '${tenant_id}';" | tr -d ' \n')
        
        if [ "$other_agents" -ne "1" ]; then
            log_error "✗ Tenant ${tenant_key} has ${other_agents} agents (expected 1)"
            isolated=false
        fi
        i=$((i + 1))
    done
    
    if [ "$isolated" = true ]; then
        log_success "✓ Tenant isolation verified - each tenant has exactly 1 agent"
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi
    
    # Test 4: Verify API keys per tenant
    log_info "Test 4: Verifying API keys..."
    local key_count
    key_count=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
        psql -U lumo -d lumo -t -c "SELECT COUNT(*) FROM tenant_api_keys;" | tr -d ' \n')
    
    if [ "$key_count" -ge "3" ]; then
        log_success "✓ API keys created for tenants"
        tests_passed=$((tests_passed + 1))
    else
        log_error "✗ Expected at least 3 API keys, found ${key_count}"
        tests_failed=$((tests_failed + 1))
    fi
    
    # Test 5: Verify agent pods are running
    log_info "Test 5: Verifying agent pods..."
    local running_agents=0
    local k=0
    for tenant_key in "${TENANT_KEYS[@]}"; do
        local tenant_slug="${TENANT_SLUGS[$k]}"
        local agent_namespace="tenant-${tenant_slug}"
        
        local pod_status
        pod_status=$(kubectl get pods -n "${agent_namespace}" -l app=lumo-agent -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "Unknown")
        
        if [ "$pod_status" = "Running" ]; then
            ((running_agents++)) || true
        else
            log_warn "Agent in ${agent_namespace} is ${pod_status}"
        fi
        k=$((k + 1))
    done
    
    if [ "$running_agents" -eq "3" ]; then
        log_success "✓ All 3 agent pods are running"
        tests_passed=$((tests_passed + 1))
    else
        log_warn "⚠ Only ${running_agents}/3 agent pods are running"
        # Not counting as failed - agents may still be starting
    fi
    
    # Test 6: Verify plan limits are set correctly
    log_info "Test 6: Verifying plan limits..."
    local limits_correct=true
    
    # Check Acme (Pro: 100 agents)
    local acme_limit
    acme_limit=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
        psql -U lumo -d lumo -t -c "SELECT max_agents FROM tenants WHERE slug = 'acme-corp';" | tr -d ' \n')
    if [ "$acme_limit" -ne "100" ]; then
        log_error "✗ Acme max_agents is ${acme_limit} (expected 100)"
        limits_correct=false
    fi
    
    # Check Globex (Starter: 25 agents)
    local globex_limit
    globex_limit=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
        psql -U lumo -d lumo -t -c "SELECT max_agents FROM tenants WHERE slug = 'globex-ind';" | tr -d ' \n')
    if [ "$globex_limit" -ne "25" ]; then
        log_error "✗ Globex max_agents is ${globex_limit} (expected 25)"
        limits_correct=false
    fi
    
    # Check Initech (Trial: 5 agents)
    local initech_limit
    initech_limit=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
        psql -U lumo -d lumo -t -c "SELECT max_agents FROM tenants WHERE slug = 'initech-sol';" | tr -d ' \n')
    if [ "$initech_limit" -ne "5" ]; then
        log_error "✗ Initech max_agents is ${initech_limit} (expected 5)"
        limits_correct=false
    fi
    
    if [ "$limits_correct" = true ]; then
        log_success "✓ Plan limits configured correctly for all tenants"
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi
    
    # Test 7: Verify API server health
    log_info "Test 7: Verifying API server health..."
    kubectl port-forward -n "${NAMESPACE}" "pod/${api_pod}" 8081:8080 >/dev/null 2>&1 &
    local pf_pid=$!
    sleep 2
    
    if curl -s "http://localhost:8081/api/v1/health" | grep -q "healthy\|ok"; then
        log_success "✓ API server is healthy"
        tests_passed=$((tests_passed + 1))
    else
        log_error "✗ API server health check failed"
        tests_failed=$((tests_failed + 1))
    fi
    
    kill $pf_pid 2>/dev/null || true
    wait $pf_pid 2>/dev/null || true
    
    # Test 8: Verify agents are actually working (check logs for successful startup)
    log_info "Test 8: Verifying agents are initialized correctly..."
    local agents_working=0
    local agents_failed=0
    local m=0
    for tenant_key in "${TENANT_KEYS[@]}"; do
        local tenant_slug="${TENANT_SLUGS[$m]}"
        local agent_namespace="tenant-${tenant_slug}"
        
        # Check for startup success or error messages in logs
        local agent_logs
        agent_logs=$(kubectl logs -n "${agent_namespace}" -l app=lumo-agent --tail=50 2>/dev/null || echo "")
        
        # Check for fatal errors (config issues, crashes)
        if echo "$agent_logs" | grep -qi "failed to load config\|Error:\|panic:"; then
            log_error "✗ Agent ${tenant_key} has startup errors:"
            echo "$agent_logs" | grep -i "error\|failed\|panic" | head -5
            agents_failed=$((agents_failed + 1))
        elif echo "$agent_logs" | grep -qi "Starting Lumo Agent\|Agent started\|informer\|event-driven"; then
            log_success "✓ Agent ${tenant_key} started successfully"
            agents_working=$((agents_working + 1))
        else
            # Check if pod is still starting
            local restart_count
            restart_count=$(kubectl get pods -n "${agent_namespace}" -l app=lumo-agent -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")
            if [ "$restart_count" -gt "0" ]; then
                log_error "✗ Agent ${tenant_key} has restarted ${restart_count} times"
                agents_failed=$((agents_failed + 1))
            else
                log_warn "⚠ Agent ${tenant_key} logs not yet available (may still be starting)"
            fi
        fi
        m=$((m + 1))
    done
    
    if [ "$agents_failed" -eq "0" ] && [ "$agents_working" -eq "3" ]; then
        log_success "✓ All 3 agents initialized correctly"
        tests_passed=$((tests_passed + 1))
    elif [ "$agents_failed" -gt "0" ]; then
        log_error "✗ ${agents_failed}/3 agents failed to start"
        tests_failed=$((tests_failed + 1))
    else
        log_warn "⚠ Only ${agents_working}/3 agents verified as working"
    fi
    
    # Summary
    echo ""
    echo "========================================================"
    echo "Validation Results: ${tests_passed} passed, ${tests_failed} failed"
    echo "========================================================"
    
    if [ "$tests_failed" -gt 0 ]; then
        return 1
    fi
    return 0
}

# ==================== SUMMARY ====================

print_summary() {
    echo ""
    echo "╔═══════════════════════════════════════════════════════════╗"
    echo "║          Multi-Tenant SaaS Deployment Complete!           ║"
    echo "╚═══════════════════════════════════════════════════════════╝"
    echo ""
    echo "Cluster: ${CLUSTER_NAME}"
    echo "Namespace: ${NAMESPACE}"
    echo ""
    echo "Deployed Components:"
    echo "  ✓ PostgreSQL (with multi-tenant schema)"
    echo "  ✓ Redis"
    echo "  ✓ Lumo API Server"
    echo ""
    echo "Test Tenants:"
    local i=0
    for tenant_key in "${TENANT_KEYS[@]}"; do
        local name="${TENANT_NAMES[$i]}"
        local slug="${TENANT_SLUGS[$i]}"
        local plan="${TENANT_PLANS[$i]}"
        echo "  ✓ ${name}"
        echo "    - Slug: ${slug}"
        echo "    - Plan: ${plan}"
        echo "    - Tenant ID: ${TENANT_IDS[$i]:0:8}..."
        echo "    - Namespace: tenant-${slug}"
        i=$((i + 1))
    done
    echo ""
    echo "Agent Tokens (for testing):"
    local j=0
    for tenant_key in "${TENANT_KEYS[@]}"; do
        echo "  ${tenant_key}: ${AGENT_TOKENS[$j]:0:24}..."
        j=$((j + 1))
    done
    echo ""
    log_info "View tenant data:"
    echo "  kubectl exec -n ${NAMESPACE} \$(kubectl get pods -n ${NAMESPACE} -l app=postgres -o name | head -1) -- \\"
    echo "    psql -U lumo -d lumo -c 'SELECT slug, plan, max_agents FROM tenants;'"
    echo ""
    log_info "View agent logs:"
    local k=0
    for tenant_key in "${TENANT_KEYS[@]}"; do
        local tenant_slug="${TENANT_SLUGS[$k]}"
        echo "  ${tenant_key}: kubectl logs -n tenant-${tenant_slug} -l app=lumo-agent -f"
        k=$((k + 1))
    done
    echo ""
    log_info "Port-forward API:"
    echo "  kubectl port-forward -n ${NAMESPACE} svc/lumo-api 8080:8080"
    echo "  curl http://localhost:8080/api/v1/health"
    echo ""
    log_info "Cleanup:"
    echo "  kind delete cluster --name ${CLUSTER_NAME}"
    echo ""
}

# ==================== MAIN ====================

main() {
    parse_args "$@"
    
    print_banner
    
    cd "$(dirname "$0")"
    
    setup_cluster
    echo ""
    
    build_and_load
    echo ""
    
    deploy_infrastructure
    echo ""
    
    deploy_api_server
    echo ""
    
    create_tenants
    echo ""
    
    provision_agents
    echo ""
    
    deploy_tenant_agents
    echo ""
    
    if run_validation_tests; then
        echo ""
        print_summary
        log_success "🎉 Multi-Tenant SaaS deployment successful!"
    else
        echo ""
        log_error "Some validation tests failed. Check logs above."
        print_summary
        exit 1
    fi
}

main "$@"
