#!/usr/bin/env bash
#
# Complete end-to-end test of Lumo Full Stack in kind
# This script: creates cluster → builds images → deploys DB → deploys API → deploys agents → runs tests
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="${KIND_CLUSTER_NAME:-lumo-test}"
NAMESPACE="${LUMO_NAMESPACE:-lumo-system}"
SKIP_CLUSTER_SETUP="${SKIP_CLUSTER_SETUP:-false}"
SKIP_BUILD="${SKIP_BUILD:-false}"
SKIP_DEPLOY="${SKIP_DEPLOY:-false}"
SKIP_INFRASTRUCTURE="${SKIP_INFRASTRUCTURE:-false}"
SKIP_API="${SKIP_API:-false}"
SKIP_MONITORING="${SKIP_MONITORING:-false}"
WITH_MONITORING="${WITH_MONITORING:-false}"
WITH_GCP_SECRETS="${WITH_GCP_SECRETS:-false}"
GCP_SA_KEY_FILE="${GCP_SA_KEY_FILE:-}"

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

print_banner() {
    echo ""
    echo "================================================"
    echo "  Lumo Full Stack - kind Testing Suite"
    echo "  DB → API Server → Agents → Integration Tests"
    echo "================================================"
    echo ""
}

setup_cluster() {
    if [ "$SKIP_CLUSTER_SETUP" = "true" ]; then
        log_info "Skipping cluster setup (SKIP_CLUSTER_SETUP=true)"
        return 0
    fi

    log_info "Step 1/7: Setting up kind cluster..."
    
    # Check if cluster already exists
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_success "✓ Cluster '${CLUSTER_NAME}' already exists, skipping creation"
        return 0
    fi
    
    ./setup-kind-cluster.sh
}

build_and_load() {
    if [ "$SKIP_BUILD" = "true" ]; then
        log_info "Skipping image build (SKIP_BUILD=true)"
        return 0
    fi

    log_info "Step 2/7: Building and loading images (API + Agent)..."
    
    # Check if images are already loaded in kind cluster
    local has_lumo=false
    local has_agent=false
    
    if docker exec "${CLUSTER_NAME}-control-plane" crictl images 2>/dev/null | grep -q "lumo.*local"; then
        has_lumo=true
    fi
    
    if docker exec "${CLUSTER_NAME}-control-plane" crictl images 2>/dev/null | grep -q "lumo-agent.*local"; then
        has_agent=true
    fi
    
    if [ "$has_lumo" = true ] && [ "$has_agent" = true ]; then
        log_success "✓ Images already loaded in cluster (lumo:local, lumo-agent:local), skipping build"
        return 0
    fi
    
    ./build-and-load.sh
}

deploy_infrastructure() {
    if [ "$SKIP_INFRASTRUCTURE" = "true" ]; then
        log_info "Skipping infrastructure deployment (SKIP_INFRASTRUCTURE=true)"
        return 0
    fi

    log_info "Step 3/7: Deploying infrastructure (PostgreSQL + Redis)..."
    
    # Create namespace first
    kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
    
    # ==================== PostgreSQL ====================
    
    # Check if PostgreSQL is already running and healthy
    local pg_status=$(kubectl get deployment/postgres -n "${NAMESPACE}" -o jsonpath='{.status.availableReplicas}' 2>/dev/null || echo "0")
    
    if [ "$pg_status" != "0" ]; then
        log_info "PostgreSQL deployment already exists, checking health..."
        local pg_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

        if [ -n "$pg_pod" ]; then
            # Try connection with retries (up to 15s)
            local max_attempts=5
            local attempt=1
            while [ $attempt -le $max_attempts ]; do
                if kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -c "SELECT 1" >/dev/null 2>&1; then
                    log_success "✓ PostgreSQL already running and healthy, skipping deployment"
                    break
                fi
                if [ $attempt -lt $max_attempts ]; then
                    sleep 3
                fi
                attempt=$((attempt + 1))
            done
            
            if [ $attempt -gt $max_attempts ]; then
                log_info "Existing PostgreSQL not responding, will redeploy..."
                pg_status="0"
            fi
        fi
    fi
    
    # Deploy PostgreSQL if not healthy
    if [ "$pg_status" = "0" ]; then
        log_info "Deploying PostgreSQL..."
        kubectl apply -f manifests/postgres.yaml
        
        # Wait for PostgreSQL to be ready
        log_info "Waiting for PostgreSQL to be ready (timeout: 120s)..."
        kubectl rollout status deployment/postgres -n "${NAMESPACE}" --timeout=120s
        
        # Verify PostgreSQL is accessible with retries
        log_info "Verifying PostgreSQL connection (will retry up to 30s)..."
        local pg_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=postgres -o jsonpath='{.items[0].metadata.name}')
        local max_attempts=10
        local attempt=1

        while [ $attempt -le $max_attempts ]; do
            if kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -c "SELECT 1" >/dev/null 2>&1; then
                log_success "✓ PostgreSQL is ready and accepting connections (attempt ${attempt}/${max_attempts})"
                break
            fi

            if [ $attempt -lt $max_attempts ]; then
                log_info "PostgreSQL not ready yet, waiting 3s before retry (attempt ${attempt}/${max_attempts})..."
                sleep 3
            fi
            attempt=$((attempt + 1))
        done

        if [ $attempt -gt $max_attempts ]; then
            log_error "✗ PostgreSQL connection test failed after ${max_attempts} attempts"
            log_info "PostgreSQL pod logs (last 20 lines):"
            kubectl logs -n "${NAMESPACE}" "${pg_pod}" --tail=20
            return 1
        fi
    fi
    
    # ==================== Redis ====================
    
    # Check if Redis is already running and healthy
    local redis_status=$(kubectl get deployment/lumo-redis -n "${NAMESPACE}" -o jsonpath='{.status.availableReplicas}' 2>/dev/null || echo "0")
    
    if [ "$redis_status" != "0" ]; then
        log_info "Redis deployment already exists, checking health..."
        local redis_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=lumo-redis -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

        if [ -n "$redis_pod" ]; then
            # Try connection with retries (up to 15s)
            local max_attempts=5
            local attempt=1
            while [ $attempt -le $max_attempts ]; do
                if kubectl exec -n "${NAMESPACE}" "${redis_pod}" -- redis-cli ping >/dev/null 2>&1; then
                    log_success "✓ Redis already running and healthy, skipping deployment"
                    return 0
                fi
                if [ $attempt -lt $max_attempts ]; then
                    sleep 3
                fi
                attempt=$((attempt + 1))
            done
            
            log_info "Existing Redis not responding, will redeploy..."
        fi
    fi
    
    # Deploy Redis
    log_info "Deploying Redis..."
    kubectl apply -f manifests/redis.yaml
    
    # Wait for Redis to be ready
    log_info "Waiting for Redis to be ready (timeout: 60s)..."
    kubectl rollout status deployment/lumo-redis -n "${NAMESPACE}" --timeout=60s
    
    # Verify Redis is accessible with retries
    log_info "Verifying Redis connection (will retry up to 20s)..."
    local redis_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=lumo-redis -o jsonpath='{.items[0].metadata.name}')
    local max_attempts=7
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if kubectl exec -n "${NAMESPACE}" "${redis_pod}" -- redis-cli ping >/dev/null 2>&1; then
            log_success "✓ Redis is ready and accepting connections (attempt ${attempt}/${max_attempts})"
            return 0
        fi

        if [ $attempt -lt $max_attempts ]; then
            log_info "Redis not ready yet, waiting 3s before retry (attempt ${attempt}/${max_attempts})..."
            sleep 3
        fi
        attempt=$((attempt + 1))
    done

    log_error "✗ Redis connection test failed after ${max_attempts} attempts"
    log_info "Redis pod logs (last 20 lines):"
    kubectl logs -n "${NAMESPACE}" "${redis_pod}" --tail=20
    return 1
}

bootstrap_api_key() {
    log_info "Step 3.5/7: Bootstrapping API key and system agent..."

    # Token that agents will use (matches deploy-to-kind.sh)
    local AGENT_TOKEN="test-token-for-kind"

    # Hash the token using SHA-256 (matches internal/database/models/api_key.go)
    local KEY_HASH=$(echo -n "$AGENT_TOKEN" | sha256sum | awk '{print $1}')

    log_info "Creating API key with hash: ${KEY_HASH:0:16}..."

    # Get PostgreSQL pod name
    local pg_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=postgres -o jsonpath='{.items[0].metadata.name}')

    if [ -z "$pg_pod" ]; then
        log_error "✗ PostgreSQL pod not found"
        return 1
    fi

    # Check if API key already exists
    local existing_key=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
        psql -U lumo -d lumo -t -c "SELECT COUNT(*) FROM api_keys WHERE key_hash = '$KEY_HASH';" 2>/dev/null | tr -d ' \n')

    # Default to 0 if empty
    existing_key=${existing_key:-0}

    if [ "$existing_key" -gt 0 ]; then
        log_success "✓ API key already exists in database, skipping creation"
    else
        log_info "Creating new API key in database..."
        # Create API key in database with full agent permissions
        # Scopes: agents:read, agents:write, events:write, jobs:read, diagnostics:write
        kubectl exec -i -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -v ON_ERROR_STOP=1 <<-EOF
            INSERT INTO api_keys (
                id,
                key_hash,
                name,
                scopes,
                created_at,
                revoked
            ) VALUES (
                gen_random_uuid(),
                '$KEY_HASH',
                'kind-test-agent-key',
                ARRAY['agents:read', 'agents:write', 'events:write', 'jobs:read', 'diagnostics:write'],
                NOW(),
                false
            );
EOF

        if [ $? -ne 0 ]; then
            log_error "✗ Failed to execute API key insert command"
            return 1
        fi

        # Verify the API key was actually created
        log_info "Verifying API key creation..."
        local verify_key=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
            psql -U lumo -d lumo -t -c "SELECT COUNT(*) FROM api_keys WHERE key_hash = '$KEY_HASH';" 2>/dev/null | tr -d ' \n')

        verify_key=${verify_key:-0}

        if [ "$verify_key" -gt 0 ]; then
            log_success "✓ API key created and verified in database"
        else
            log_error "✗ API key creation failed - key not found in database after insert"
            log_info "Checking database connection and tables..."
            kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -c "\dt api_keys"
            return 1
        fi
    fi

    # Create system agent for API key authenticated events
    log_info "Creating system agent for API key authentication..."
    kubectl exec -i -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -v ON_ERROR_STOP=1 <<-EOF
        INSERT INTO agents (
            id,
            name,
            hostname,
            ip_address,
            platform,
            architecture,
            version,
            status,
            capabilities,
            labels,
            registered_at,
            last_heartbeat_at
        ) VALUES (
            '00000000-0000-0000-0000-000000000000',
            'system-api-key',
            'api-server',
            '0.0.0.0',
            'kubernetes',
            'any',
            'n/a',
            'online',
            ARRAY[]::text[],
            '{"type": "system", "auth": "api-key"}'::jsonb,
            NOW(),
            NOW()
        ) ON CONFLICT (id) DO UPDATE SET
            last_heartbeat_at = NOW(),
            status = 'online';
EOF

    if [ $? -ne 0 ]; then
        log_error "✗ Failed to execute system agent insert command"
        return 1
    fi

    # Verify the system agent was created
    log_info "Verifying system agent creation..."
    local verify_agent=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
        psql -U lumo -d lumo -t -c "SELECT COUNT(*) FROM agents WHERE id = '00000000-0000-0000-0000-000000000000';" 2>/dev/null | tr -d ' \n')

    verify_agent=${verify_agent:-0}

    if [ "$verify_agent" -gt 0 ]; then
        log_success "✓ System agent created and verified in database"
    else
        log_error "✗ System agent creation failed - agent not found in database after insert"
        log_info "Checking database connection and tables..."
        kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -c "\dt agents"
        return 1
    fi

    # Final verification: Show what was created
    log_info "Bootstrap summary:"
    kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -c \
        "SELECT name, scopes FROM api_keys WHERE key_hash = '$KEY_HASH';" 2>/dev/null | head -3
    kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -c \
        "SELECT id, name, status FROM agents WHERE id = '00000000-0000-0000-0000-000000000000';" 2>/dev/null | head -3
}

deploy_api_server() {
    if [ "$SKIP_API" = "true" ]; then
        log_info "Skipping API server deployment (SKIP_API=true)"
        return 0
    fi

    log_info "Step 4/7: Deploying Lumo API Server..."
    
    # Check if API server is already running and healthy
    local api_status=$(kubectl get deployment/lumo-api -n "${NAMESPACE}" -o jsonpath='{.status.availableReplicas}' 2>/dev/null || echo "0")
    
    if [ "$api_status" != "0" ]; then
        log_info "API server deployment already exists, checking health..."
        local pod_name=$(kubectl get pods -n "${NAMESPACE}" -l app=lumo-api -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
        
        if [ -n "$pod_name" ]; then
            kubectl port-forward -n "${NAMESPACE}" "pod/${pod_name}" 8081:8080 >/dev/null 2>&1 &
            local pf_pid=$!
            sleep 3
            
            if curl -s http://localhost:8081/api/v1/health >/dev/null 2>&1; then
                log_success "✓ API server already running and healthy, skipping deployment"
                kill $pf_pid 2>/dev/null || true
                wait $pf_pid 2>/dev/null || true
                return 0
            fi
            
            kill $pf_pid 2>/dev/null || true
            wait $pf_pid 2>/dev/null || true
        fi
    fi
    
    # Deploy API server
    kubectl apply -f manifests/api-server.yaml
    
    # Wait for API server to be ready
    log_info "Waiting for API server to be ready (timeout: 120s)..."
    kubectl rollout status deployment/lumo-api -n "${NAMESPACE}" --timeout=120s
    
    # Verify API server health with retries
    log_info "Verifying API server health (will retry up to 30s)..."
    local pod_name=$(kubectl get pods -n "${NAMESPACE}" -l app=lumo-api -o jsonpath='{.items[0].metadata.name}')

    # Port-forward in background for health check
    kubectl port-forward -n "${NAMESPACE}" "pod/${pod_name}" 8081:8080 >/dev/null 2>&1 &
    local pf_pid=$!
    sleep 3

    local max_attempts=10
    local attempt=1
    local health_ok=false

    while [ $attempt -le $max_attempts ]; do
        if curl -s http://localhost:8081/api/v1/health >/dev/null 2>&1; then
            log_success "✓ API server is healthy and responding (attempt ${attempt}/${max_attempts})"
            health_ok=true
            break
        fi

        if [ $attempt -lt $max_attempts ]; then
            log_info "API server not ready yet, waiting 3s before retry (attempt ${attempt}/${max_attempts})..."
            sleep 3
        fi
        attempt=$((attempt + 1))
    done

    # Kill port-forward
    kill $pf_pid 2>/dev/null || true
    wait $pf_pid 2>/dev/null || true

    if [ "$health_ok" = false ]; then
        log_error "✗ API server health check failed after ${max_attempts} attempts"
        log_info "API server logs (last 20 lines):"
        kubectl logs -n "${NAMESPACE}" "${pod_name}" --tail=20
        return 1
    fi
}

deploy_agent() {
    if [ "$SKIP_DEPLOY" = "true" ]; then
        log_info "Skipping agent deployment (SKIP_DEPLOY=true)"
        return 0
    fi

    log_info "Step 5/7: Deploying event-driven agent..."

    # Check if agents are already deployed and running
    local deploy_ready=$(kubectl get deployment -n "${NAMESPACE}" lumo-agent -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")

    if [ "$deploy_ready" -gt "0" ]; then
        log_success "✓ Agent already deployed and running (${deploy_ready} replicas ready), skipping deployment"
        return 0
    fi

    # Set API endpoint to point to our in-cluster API server
    export LUMO_API_ENDPOINT="http://lumo-api.${NAMESPACE}.svc.cluster.local:8080"
    export SKIP_CLUSTER_SETUP=true
    export SKIP_BUILD=true

    ./deploy-to-kind.sh

    # Wait for event-driven agent deployment to be ready
    log_info "Waiting for agent deployment to be ready (timeout: 120s)..."
    local max_wait=120
    local elapsed=0
    local interval=5

    while [ $elapsed -lt $max_wait ]; do
        local deploy_ready=$(kubectl get deployment -n "${NAMESPACE}" lumo-agent -o jsonpath='{.status.readyReplicas}' 2>/dev/null)
        local deploy_desired=$(kubectl get deployment -n "${NAMESPACE}" lumo-agent -o jsonpath='{.spec.replicas}' 2>/dev/null)
        
        # Default to 0 if empty
        deploy_ready=${deploy_ready:-0}
        deploy_desired=${deploy_desired:-0}

        if [ "$deploy_ready" -eq "$deploy_desired" ] && [ "$deploy_ready" -gt "0" ]; then
            log_success "✓ Agent deployment ready: ${deploy_ready}/${deploy_desired} pods"
            break
        fi

        log_info "Agent deployment not ready yet: ${deploy_ready}/${deploy_desired} pods ready (${elapsed}s/${max_wait}s)"
        sleep $interval
        elapsed=$((elapsed + interval))
    done

    if [ $elapsed -ge $max_wait ]; then
        log_error "✗ Agent deployment did not become ready within ${max_wait}s"
        kubectl get deployment -n "${NAMESPACE}" lumo-agent -o wide
        return 1
    fi

    # Give agents a moment to fully initialize
    log_info "Waiting for agents to initialize (5s)..."
    sleep 5
}

run_component_tests() {
    log_info "Step 6/7: Running component tests..."
    echo ""

    # Wait for all pods to be stable
    log_info "Waiting for all pods to stabilize (max 60s)..."
    local max_wait=60
    local elapsed=0
    local interval=5
    local all_running=false

    while [ $elapsed -lt $max_wait ]; do
        local running_pods=$(kubectl get pods -n "${NAMESPACE}" -o json | jq -r '.items[] | select(.status.phase=="Running") | .metadata.name' | wc -l)
        local total_pods=$(kubectl get pods -n "${NAMESPACE}" -o json | jq -r '.items | length')

        if [ "$running_pods" -eq "$total_pods" ] && [ "$running_pods" -gt 0 ]; then
            log_success "✓ All pods stabilized (${running_pods}/${total_pods}) after ${elapsed}s"
            all_running=true
            break
        fi

        log_info "Pods not all running yet: ${running_pods}/${total_pods} (${elapsed}s/${max_wait}s)"
        sleep $interval
        elapsed=$((elapsed + interval))
    done

    if [ "$all_running" = false ]; then
        log_error "✗ Not all pods running after ${max_wait}s"
        kubectl get pods -n "${NAMESPACE}"
        return 1
    fi

    # Give pods a moment to fully initialize
    log_info "Waiting for pod initialization (5s)..."
    sleep 5

    # Test 1: Check if all pods are running
    log_info "Test 1: Verifying final pod status..."
    local running_pods=$(kubectl get pods -n "${NAMESPACE}" -o json | jq -r '.items[] | select(.status.phase=="Running") | .metadata.name' | wc -l)
    local total_pods=$(kubectl get pods -n "${NAMESPACE}" -o json | jq -r '.items | length')

    if [ "$running_pods" -eq "$total_pods" ] && [ "$running_pods" -gt 0 ]; then
        log_success "✓ All pods are running (${running_pods}/${total_pods})"
        kubectl get pods -n "${NAMESPACE}" -o wide | grep -v "NAME"
    else
        log_error "✗ Not all pods are running (${running_pods}/${total_pods})"
        kubectl get pods -n "${NAMESPACE}"
        return 1
    fi

    # Test 2: Check PostgreSQL
    log_info "Test 2: Checking PostgreSQL..."
    local pg_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=postgres -o jsonpath='{.items[0].metadata.name}')

    # Retry connection check (up to 15s)
    local max_attempts=5
    local attempt=1
    local connected=false

    while [ $attempt -le $max_attempts ]; do
        if kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -c "SELECT COUNT(*) FROM information_schema.tables" >/dev/null 2>&1; then
            log_success "✓ PostgreSQL is accessible and functional (attempt ${attempt}/${max_attempts})"
            connected=true
            break
        fi
        if [ $attempt -lt $max_attempts ]; then
            sleep 3
        fi
        attempt=$((attempt + 1))
    done

    if [ "$connected" = false ]; then
        log_error "✗ PostgreSQL connection failed after ${max_attempts} attempts"
    fi

    # Test 3: Check API server health
    log_info "Test 3: Checking API server health..."
    local api_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=lumo-api -o jsonpath='{.items[0].metadata.name}')
    
    kubectl port-forward -n "${NAMESPACE}" "pod/${api_pod}" 8081:8080 >/dev/null 2>&1 &
    local pf_pid=$!
    sleep 2

    local health_response=$(curl -s http://localhost:8081/api/v1/health || echo "")
    if echo "$health_response" | grep -q "healthy\|ok"; then
        log_success "✓ API server health endpoint responding: ${health_response}"
    else
        log_error "✗ API server health check failed"
        kubectl logs -n "${NAMESPACE}" "${api_pod}" --tail=20
    fi
    
    kill $pf_pid 2>/dev/null || true
    wait $pf_pid 2>/dev/null || true

    # Test 4: Check agent health endpoints
    log_info "Test 4: Checking agent health endpoints..."
    local agent_pod=$(kubectl get pods -n "${NAMESPACE}" -l mode=event-driven -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

    if [ -n "$agent_pod" ]; then
        kubectl port-forward -n "${NAMESPACE}" pod/"${agent_pod}" 8082:8080 >/dev/null 2>&1 &
        local pf_pid=$!
        sleep 3

        # Check health endpoint with retries
        local max_attempts=5
        local attempt=1
        local health_ok=false

        while [ $attempt -le $max_attempts ]; do
            if curl -s http://localhost:8082/health >/dev/null 2>&1; then
                log_success "✓ Agent health endpoint responding (attempt ${attempt}/${max_attempts})"
                health_ok=true
                break
            fi
            if [ $attempt -lt $max_attempts ]; then
                sleep 2
            fi
            attempt=$((attempt + 1))
        done

        if [ "$health_ok" = false ]; then
            log_error "✗ Agent health endpoint not responding after ${max_attempts} attempts"
        fi

        # Check ready endpoint with retries
        attempt=1
        local ready_ok=false

        while [ $attempt -le $max_attempts ]; do
            if curl -s http://localhost:8082/ready >/dev/null 2>&1; then
                log_success "✓ Agent ready endpoint responding (attempt ${attempt}/${max_attempts})"
                ready_ok=true
                break
            fi
            if [ $attempt -lt $max_attempts ]; then
                sleep 2
            fi
            attempt=$((attempt + 1))
        done

        if [ "$ready_ok" = false ]; then
            log_error "✗ Agent ready endpoint not responding after ${max_attempts} attempts"
        fi

        kill $pf_pid 2>/dev/null || true
        wait $pf_pid 2>/dev/null || true
    else
        log_error "✗ No event-driven agent pod found"
    fi

    # Test 5: Check agent metrics
    log_info "Test 5: Checking agent metrics endpoint..."
    if [ -n "$agent_pod" ]; then
        kubectl port-forward -n "${NAMESPACE}" pod/"${agent_pod}" 9090:9090 >/dev/null 2>&1 &
        local pf_pid=$!
        sleep 3

        # Check metrics endpoint with retries
        local max_attempts=5
        local attempt=1
        local metrics_ok=false

        while [ $attempt -le $max_attempts ]; do
            if curl -s http://localhost:9090/metrics | grep -q "lumo_agent"; then
                log_success "✓ Metrics endpoint responding with lumo_agent metrics (attempt ${attempt}/${max_attempts})"
                metrics_ok=true
                break
            fi
            if [ $attempt -lt $max_attempts ]; then
                sleep 2
            fi
            attempt=$((attempt + 1))
        done

        if [ "$metrics_ok" = false ]; then
            log_error "✗ Metrics endpoint not responding correctly after ${max_attempts} attempts"
        fi

        kill $pf_pid 2>/dev/null || true
        wait $pf_pid 2>/dev/null || true
    fi

    # Test 6: Check RBAC permissions
    log_info "Test 6: Checking RBAC permissions..."
    if kubectl auth can-i list nodes --as=system:serviceaccount:${NAMESPACE}:lumo-agent >/dev/null 2>&1; then
        log_success "✓ ServiceAccount has required permissions"
    else
        log_error "✗ ServiceAccount missing required permissions"
    fi

    # Test 7: Check agent deployment replicas
    log_info "Test 7: Checking agent deployment replicas..."
    local deploy_ready=$(kubectl get deployment -n "${NAMESPACE}" lumo-agent -o json | jq -r '.status.readyReplicas // 0')
    local deploy_desired=$(kubectl get deployment -n "${NAMESPACE}" lumo-agent -o json | jq -r '.spec.replicas // 0')

    if [ "$deploy_ready" -eq "$deploy_desired" ] && [ "$deploy_ready" -gt "0" ]; then
        log_success "✓ Agent deployment has all replicas ready (${deploy_ready}/${deploy_desired})"
    else
        log_error "✗ Agent deployment replicas not ready (${deploy_ready}/${deploy_desired})"
    fi
}

run_integration_tests() {
    log_info "Step 7/7: Running integration tests..."
    echo ""

    # Test 1: Check agent registration with API
    log_info "Test 1: Checking agent registration (will retry up to 30s)..."

    local api_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=lumo-api -o jsonpath='{.items[0].metadata.name}')

    # Wait for agent registration with retries
    local max_attempts=10
    local attempt=1
    local registration_found=false

    while [ $attempt -le $max_attempts ]; do
        # Check API logs for agent registration
        if kubectl logs -n "${NAMESPACE}" "${api_pod}" --tail=100 | grep -iq "agent.*register"; then
            log_success "✓ Agent registration activity found in API logs (attempt ${attempt}/${max_attempts})"
            registration_found=true
            break
        fi

        if [ $attempt -lt $max_attempts ]; then
            log_info "No agent registration yet, waiting 3s before retry (attempt ${attempt}/${max_attempts})..."
            sleep 3
        fi
        attempt=$((attempt + 1))
    done

    if [ "$registration_found" = false ]; then
        log_error "✗ No agent registration found in API logs after ${max_attempts} attempts"
        log_info "API server logs (last 30 lines):"
        kubectl logs -n "${NAMESPACE}" "${api_pod}" --tail=30
    fi

    # Test 2: Check agent is attempting to communicate with API
    log_info "Test 2: Checking agent → API communication (will retry up to 20s)..."
    local agent_pod=$(kubectl get pods -n "${NAMESPACE}" -l mode=event-driven -o jsonpath='{.items[0].metadata.name}')

    max_attempts=7
    attempt=1
    local comm_found=false

    while [ $attempt -le $max_attempts ]; do
        if kubectl logs -n "${NAMESPACE}" "${agent_pod}" --tail=50 | grep -iq "api\|register\|heartbeat"; then
            log_success "✓ Agent is attempting API communication (attempt ${attempt}/${max_attempts})"
            comm_found=true
            break
        fi

        if [ $attempt -lt $max_attempts ]; then
            log_info "No API communication yet, waiting 3s before retry (attempt ${attempt}/${max_attempts})..."
            sleep 3
        fi
        attempt=$((attempt + 1))
    done

    if [ "$comm_found" = false ]; then
        log_error "✗ No API communication attempts in agent logs after ${max_attempts} attempts"
        log_info "Agent logs (last 30 lines):"
        kubectl logs -n "${NAMESPACE}" "${agent_pod}" --tail=30
    fi

    # Test 3: Check for critical errors in any component
    log_info "Test 3: Checking for critical errors across all components..."
    local error_count=$(kubectl logs -n "${NAMESPACE}" --all-containers --tail=200 2>/dev/null | grep -i "fatal\|panic" | wc -l | tr -d ' ')
    error_count=${error_count:-0}

    if [ "$error_count" -eq 0 ]; then
        log_success "✓ No fatal errors found in any component"
    else
        log_error "✗ Found ${error_count} fatal error messages"
        log_info "Showing recent fatal errors:"
        kubectl logs -n "${NAMESPACE}" --all-containers --tail=200 2>/dev/null | grep -i "fatal\|panic" | head -5
    fi
}

print_summary() {
    echo ""
    echo "========================================================"
    log_success "Lumo Full Stack Testing Complete!"
    echo "========================================================"
    echo ""
    echo "Cluster: ${CLUSTER_NAME}"
    echo "Namespace: ${NAMESPACE}"
    echo ""
    echo "Deployed Components:"
    echo "  ✓ PostgreSQL (Database)"
    echo "  ✓ Redis (Cache)"
    echo "  ✓ Lumo API Server"
    echo "  ✓ Lumo Event-Driven Agent (Deployment)"
    if [ "$WITH_GCP_SECRETS" = "true" ]; then
        echo "  ✓ External Secrets Operator"
        echo "  ✓ GCP Secret Manager Integration"
    fi
    if [ "$WITH_MONITORING" = "true" ]; then
        echo "  ✓ Prometheus + Grafana"
    fi
    echo ""
    if [ "$WITH_GCP_SECRETS" = "true" ]; then
        log_info "Secrets managed by GCP Secret Manager:"
        echo "  ${BLUE}kubectl get externalsecrets -n ${NAMESPACE}${NC}"
        echo "  ${BLUE}kubectl get clustersecretstore gcp-secret-manager${NC}"
        echo ""
    fi
    log_info "View component logs:"
    echo "  PostgreSQL:  ${BLUE}kubectl logs -n ${NAMESPACE} -l app=postgres -f${NC}"
    echo "  API Server:  ${BLUE}kubectl logs -n ${NAMESPACE} -l app=lumo-api -f${NC}"
    echo "  Agents:      ${BLUE}kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=lumo-agent -f${NC}"
    echo ""
    log_info "Access services:"
    echo "  API Server:  ${BLUE}kubectl port-forward -n ${NAMESPACE} svc/lumo-api 8080:8080${NC}"
    echo "               ${BLUE}curl http://localhost:8080/api/v1/health${NC}"
    echo ""
    echo "  PostgreSQL:  ${BLUE}kubectl port-forward -n ${NAMESPACE} svc/postgres 5432:5432${NC}"
    echo "               ${BLUE}PGPASSWORD=lumo psql -h localhost -U lumo -d lumo${NC}"
    echo ""
    log_info "Check all pods:"
    echo "  ${BLUE}kubectl get pods -n ${NAMESPACE} -o wide${NC}"
    echo ""
    log_info "To tear down the test environment:"
    echo "  ${BLUE}kind delete cluster --name ${CLUSTER_NAME}${NC}"
    echo ""
}

show_usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Complete end-to-end testing of Lumo Full Stack in kind.
Deploys: PostgreSQL → API Server → Agents → Runs Integration Tests

Options:
  --skip-cluster         Skip cluster creation (use existing)
  --skip-build           Skip image build (use existing images)
  --skip-infrastructure  Skip PostgreSQL deployment
  --skip-api             Skip API server deployment
  --skip-deploy          Skip agent deployment
  --with-monitoring      Deploy Prometheus + Grafana monitoring stack
  --skip-monitoring      Skip monitoring deployment (if --with-monitoring is set)
  --with-gcp-secrets     Use GCP Secret Manager (fully automated setup)
  --gcp-sa-key FILE      Use existing GCP SA key file (optional, auto-creates if not provided)
  --cluster-name         Name of kind cluster (default: lumo-test)
  --namespace            Kubernetes namespace (default: lumo-system)
  -h, --help             Show this help message

Environment variables:
  SKIP_CLUSTER_SETUP      Set to 'true' to skip cluster setup
  SKIP_BUILD              Set to 'true' to skip image build
  SKIP_INFRASTRUCTURE     Set to 'true' to skip PostgreSQL deployment
  SKIP_API                Set to 'true' to skip API server deployment
  SKIP_DEPLOY             Set to 'true' to skip agent deployment
  WITH_MONITORING         Set to 'true' to deploy monitoring stack
  WITH_GCP_SECRETS        Set to 'true' to use GCP Secret Manager
  GCP_PROJECT_ID          GCP project ID (uses gcloud default if not set)
  GCP_SA_KEY_FILE         Path to existing GCP service account key file
  LUMO_ANTHROPIC_API_KEY  Anthropic API key (stored in GCP Secret Manager)
  LUMO_OPENAI_API_KEY     OpenAI API key (stored in GCP Secret Manager)
  LUMO_SLACK_WEBHOOK_URL  Slack webhook URL (stored in GCP Secret Manager)
  KIND_CLUSTER_NAME       Name of kind cluster
  LUMO_NAMESPACE          Kubernetes namespace

Examples:
  # Full stack deployment with hardcoded test secrets (local dev)
  $0

  # Full stack with monitoring (Prometheus + Grafana)
  $0 --with-monitoring

  # Full stack with GCP Secret Manager (fully automated!)
  # Just needs: gcloud auth login && gcloud config set project YOUR_PROJECT
  $0 --with-gcp-secrets

  # With real AI keys stored in GCP
  export LUMO_ANTHROPIC_API_KEY="sk-ant-..."
  $0 --with-gcp-secrets

  # Use existing cluster but rebuild everything
  $0 --skip-cluster

  # Use existing infrastructure, only redeploy agents
  $0 --skip-cluster --skip-build --skip-infrastructure --skip-api

  # Quick test of existing full deployment
  SKIP_CLUSTER_SETUP=true SKIP_BUILD=true SKIP_INFRASTRUCTURE=true SKIP_API=true SKIP_DEPLOY=true $0

EOF
}

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
            --skip-api)
                SKIP_API=true
                shift
                ;;
            --skip-deploy)
                SKIP_DEPLOY=true
                shift
                ;;
            --skip-monitoring)
                SKIP_MONITORING=true
                shift
                ;;
            --with-monitoring)
                WITH_MONITORING=true
                shift
                ;;
            --with-gcp-secrets)
                WITH_GCP_SECRETS=true
                shift
                ;;
            --gcp-sa-key)
                GCP_SA_KEY_FILE="$2"
                shift 2
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

deploy_monitoring() {
    if [ "$WITH_MONITORING" != "true" ]; then
        log_info "Skipping monitoring (use --with-monitoring to enable)"
        return 0
    fi

    if [ "$SKIP_MONITORING" = "true" ]; then
        log_info "Skipping monitoring deployment (SKIP_MONITORING=true)"
        return 0
    fi

    echo "========================================"
    echo "  Deploying Monitoring Stack"
    echo "========================================"

    local SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local MONITORING_DIR="${SCRIPT_DIR}/../monitoring"

    if [ ! -f "${MONITORING_DIR}/deploy-monitoring.sh" ]; then
        log_error "Monitoring deploy script not found at ${MONITORING_DIR}/deploy-monitoring.sh"
        return 1
    fi

    log_info "Deploying Prometheus and Grafana..."
    bash "${MONITORING_DIR}/deploy-monitoring.sh" --with-prometheus

    # Wait for Grafana to be ready
    log_info "Waiting for Grafana to be ready..."
    kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=grafana -n monitoring --timeout=120s 2>/dev/null || {
        log_error "Grafana failed to become ready"
        kubectl get pods -n monitoring
        return 1
    }

    log_success "Monitoring stack deployed successfully"

    echo ""
    log_info "Access Grafana:"
    echo "  kubectl port-forward -n monitoring svc/grafana 3000:80"
    echo "  Open: http://localhost:3000 (admin/admin)"
    echo ""
}

deploy_gcp_secrets() {
    if [ "$WITH_GCP_SECRETS" != "true" ]; then
        log_info "Using hardcoded secrets (use --with-gcp-secrets for GCP Secret Manager)"
        return 0
    fi

    echo "========================================"
    echo "  Setting Up GCP Secret Manager"
    echo "  (Fully automated setup)"
    echo "========================================"

    # Check for gcloud CLI
    if ! command -v gcloud &> /dev/null; then
        log_error "gcloud CLI not found. Install from: https://cloud.google.com/sdk/docs/install"
        return 1
    fi

    # Check gcloud authentication
    if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" 2>/dev/null | grep -q .; then
        log_error "Not authenticated with gcloud. Run: gcloud auth login"
        return 1
    fi

    # Get or set project ID
    if [ -z "${GCP_PROJECT_ID:-}" ]; then
        GCP_PROJECT_ID=$(gcloud config get-value project 2>/dev/null)
        if [ -z "$GCP_PROJECT_ID" ]; then
            log_error "GCP_PROJECT_ID not set and no default project configured"
            log_info "Run: export GCP_PROJECT_ID=\"your-project-id\""
            log_info "Or:  gcloud config set project your-project-id"
            return 1
        fi
        log_info "Using default GCP project: $GCP_PROJECT_ID"
    fi

    export GCP_PROJECT_ID

    # Step 1: Enable Secret Manager API
    log_info "Step 1/5: Enabling Secret Manager API..."
    if ! gcloud services list --enabled --project="$GCP_PROJECT_ID" 2>/dev/null | grep -q "secretmanager.googleapis.com"; then
        gcloud services enable secretmanager.googleapis.com --project="$GCP_PROJECT_ID" --quiet
        log_success "Secret Manager API enabled"
    else
        log_success "Secret Manager API already enabled"
    fi

    # Step 2: Create service account if needed
    local SA_NAME="lumo-secrets-accessor"
    local SA_EMAIL="${SA_NAME}@${GCP_PROJECT_ID}.iam.gserviceaccount.com"

    log_info "Step 2/5: Setting up service account..."
    if ! gcloud iam service-accounts describe "$SA_EMAIL" --project="$GCP_PROJECT_ID" &>/dev/null; then
        gcloud iam service-accounts create "$SA_NAME" \
            --display-name="Lumo Secrets Accessor" \
            --description="Service account for Lumo to access GCP Secret Manager" \
            --project="$GCP_PROJECT_ID" --quiet
        log_success "Created service account: $SA_NAME"
    else
        log_success "Service account already exists: $SA_NAME"
    fi

    # Grant Secret Manager access
    gcloud projects add-iam-policy-binding "$GCP_PROJECT_ID" \
        --member="serviceAccount:${SA_EMAIL}" \
        --role="roles/secretmanager.secretAccessor" \
        --condition=None \
        --quiet 2>/dev/null
    log_success "Granted secretmanager.secretAccessor role"

    # Step 3: Create or download service account key
    log_info "Step 3/5: Setting up service account key..."
    local KEY_DIR="${HOME}/.lumo/gcp"
    local KEY_FILE="${KEY_DIR}/sa-key-${GCP_PROJECT_ID}.json"

    mkdir -p "$KEY_DIR"
    chmod 700 "$KEY_DIR"

    if [ -n "$GCP_SA_KEY_FILE" ] && [ -f "$GCP_SA_KEY_FILE" ]; then
        log_success "Using provided key file: $GCP_SA_KEY_FILE"
        KEY_FILE="$GCP_SA_KEY_FILE"
    elif [ -f "$KEY_FILE" ]; then
        log_success "Using existing key file: $KEY_FILE"
    else
        log_info "Creating new service account key..."
        gcloud iam service-accounts keys create "$KEY_FILE" \
            --iam-account="$SA_EMAIL" \
            --project="$GCP_PROJECT_ID" --quiet
        chmod 600 "$KEY_FILE"
        log_success "Created key file: $KEY_FILE"
    fi

    # Step 4: Create secrets in GCP Secret Manager
    log_info "Step 4/5: Creating secrets in GCP Secret Manager..."
    
    create_gcp_secret_if_missing() {
        local secret_name="$1"
        local secret_value="$2"
        
        if gcloud secrets describe "$secret_name" --project="$GCP_PROJECT_ID" &>/dev/null; then
            log_info "  ✓ $secret_name (exists)"
        else
            echo -n "$secret_value" | gcloud secrets create "$secret_name" \
                --data-file=- \
                --replication-policy="automatic" \
                --labels="app=lumo,managed-by=deploy-lumo" \
                --project="$GCP_PROJECT_ID" --quiet
            log_success "  ✓ $secret_name (created)"
        fi
    }

    # Generate secure random values for secrets
    local JWT_SECRET=$(openssl rand -base64 32 2>/dev/null || head -c 32 /dev/urandom | base64)
    local DB_PASSWORD=$(openssl rand -base64 16 2>/dev/null || head -c 16 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9')
    local AGENT_TOKEN=$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p | tr -d '\n')

    create_gcp_secret_if_missing "lumo-jwt-secret" "$JWT_SECRET"
    create_gcp_secret_if_missing "lumo-database-password" "$DB_PASSWORD"
    create_gcp_secret_if_missing "lumo-agent-token" "$AGENT_TOKEN"
    
    # Optional secrets - create with placeholder if not exists
    create_gcp_secret_if_missing "lumo-anthropic-api-key" "${LUMO_ANTHROPIC_API_KEY:-placeholder}"
    create_gcp_secret_if_missing "lumo-openai-api-key" "${LUMO_OPENAI_API_KEY:-placeholder}"
    create_gcp_secret_if_missing "lumo-slack-webhook-url" "${LUMO_SLACK_WEBHOOK_URL:-placeholder}"

    # Step 5: Deploy External Secrets Operator and sync secrets
    log_info "Step 5/5: Deploying External Secrets Operator..."

    # Install ESO via Helm
    if ! command -v helm &> /dev/null; then
        log_error "helm not found. Install from: https://helm.sh/docs/intro/install/"
        return 1
    fi

    helm repo add external-secrets https://charts.external-secrets.io 2>/dev/null || true
    helm repo update --quiet

    local ESO_NAMESPACE="external-secrets"
    kubectl create namespace "$ESO_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

    if helm status external-secrets -n "$ESO_NAMESPACE" &>/dev/null; then
        log_info "External Secrets Operator already installed, upgrading..."
        helm upgrade external-secrets external-secrets/external-secrets \
            -n "$ESO_NAMESPACE" \
            --set installCRDs=true \
            --wait --quiet
    else
        helm install external-secrets external-secrets/external-secrets \
            -n "$ESO_NAMESPACE" \
            --set installCRDs=true \
            --wait --quiet
    fi
    log_success "External Secrets Operator installed"

    # Wait for CRDs
    kubectl wait --for=condition=Established crd/externalsecrets.external-secrets.io --timeout=60s >/dev/null
    kubectl wait --for=condition=Established crd/clustersecretstores.external-secrets.io --timeout=60s >/dev/null

    # Create K8s secret with GCP credentials
    kubectl create secret generic gcp-secret-manager-credentials \
        --from-file=secret-access-credentials="$KEY_FILE" \
        -n "$ESO_NAMESPACE" \
        --dry-run=client -o yaml | kubectl apply -f - >/dev/null
    log_success "GCP credentials secret created in cluster"

    # Create namespace for Lumo
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

    # Deploy ClusterSecretStore
    cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: gcp-secret-manager
spec:
  provider:
    gcpsm:
      projectID: ${GCP_PROJECT_ID}
      auth:
        secretRef:
          secretAccessKeySecretRef:
            name: gcp-secret-manager-credentials
            key: secret-access-credentials
            namespace: ${ESO_NAMESPACE}
EOF
    log_success "ClusterSecretStore deployed"

    # Wait for SecretStore to be ready
    local max_wait=30
    local elapsed=0
    while [ $elapsed -lt $max_wait ]; do
        local status=$(kubectl get clustersecretstore gcp-secret-manager -o jsonpath='{.status.conditions[0].status}' 2>/dev/null || echo "Unknown")
        if [ "$status" = "True" ]; then
            log_success "ClusterSecretStore is ready"
            break
        fi
        sleep 3
        elapsed=$((elapsed + 3))
    done

    if [ $elapsed -ge $max_wait ]; then
        log_error "ClusterSecretStore not ready after ${max_wait}s"
        kubectl describe clustersecretstore gcp-secret-manager
        return 1
    fi

    # Deploy ExternalSecrets
    cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: lumo-api-secrets
  namespace: ${NAMESPACE}
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: gcp-secret-manager
    kind: ClusterSecretStore
  target:
    name: lumo-api-secret
    creationPolicy: Owner
  data:
    - secretKey: jwt-secret
      remoteRef:
        key: lumo-jwt-secret
    - secretKey: database-password
      remoteRef:
        key: lumo-database-password
---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: lumo-ai-secrets
  namespace: ${NAMESPACE}
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: gcp-secret-manager
    kind: ClusterSecretStore
  target:
    name: lumo-ai-secrets
    creationPolicy: Owner
  data:
    - secretKey: anthropic-api-key
      remoteRef:
        key: lumo-anthropic-api-key
    - secretKey: openai-api-key
      remoteRef:
        key: lumo-openai-api-key
---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: lumo-agent-secrets
  namespace: ${NAMESPACE}
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: gcp-secret-manager
    kind: ClusterSecretStore
  target:
    name: lumo-agent-secret
    creationPolicy: Owner
  data:
    - secretKey: agent-token
      remoteRef:
        key: lumo-agent-token
---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: lumo-notification-secrets
  namespace: ${NAMESPACE}
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: gcp-secret-manager
    kind: ClusterSecretStore
  target:
    name: lumo-notification-secrets
    creationPolicy: Owner
  data:
    - secretKey: slack-webhook-url
      remoteRef:
        key: lumo-slack-webhook-url
EOF
    log_success "ExternalSecrets deployed"

    # Wait for secrets to sync
    log_info "Waiting for secrets to sync from GCP..."
    max_wait=60
    elapsed=0
    while [ $elapsed -lt $max_wait ]; do
        if kubectl get secret lumo-api-secret -n "${NAMESPACE}" &>/dev/null; then
            log_success "✓ Secrets synced from GCP Secret Manager"
            echo ""
            log_info "GCP Secret Manager setup complete!"
            echo "  Project: $GCP_PROJECT_ID"
            echo "  Key file: $KEY_FILE"
            echo ""
            return 0
        fi
        sleep 5
        elapsed=$((elapsed + 5))
        log_info "Waiting for secrets... (${elapsed}s/${max_wait}s)"
    done

    log_error "Secrets not synced after ${max_wait}s"
    kubectl get externalsecrets -n "${NAMESPACE}"
    return 1
}

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

    # Deploy GCP secrets before API server (if enabled)
    deploy_gcp_secrets
    echo ""

    deploy_api_server
    echo ""

    bootstrap_api_key
    echo ""

    deploy_agent
    echo ""

    deploy_monitoring
    echo ""

    run_component_tests
    echo ""

    run_integration_tests
    echo ""

    print_summary
}

main "$@"
