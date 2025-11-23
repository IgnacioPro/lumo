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

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
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
    ./setup-kind-cluster.sh
}

build_and_load() {
    if [ "$SKIP_BUILD" = "true" ]; then
        log_info "Skipping image build (SKIP_BUILD=true)"
        return 0
    fi

    log_info "Step 2/7: Building and loading images (API + Agent)..."
    ./build-and-load.sh
}

deploy_infrastructure() {
    if [ "$SKIP_INFRASTRUCTURE" = "true" ]; then
        log_info "Skipping infrastructure deployment (SKIP_INFRASTRUCTURE=true)"
        return 0
    fi

    log_info "Step 3/7: Deploying infrastructure (PostgreSQL)..."
    
    # Create namespace first
    kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
    
    # Deploy PostgreSQL
    log_info "Deploying PostgreSQL..."
    kubectl apply -f manifests/postgres.yaml
    
    # Wait for PostgreSQL to be ready
    log_info "Waiting for PostgreSQL to be ready (timeout: 120s)..."
    kubectl rollout status deployment/postgres -n "${NAMESPACE}" --timeout=120s
    
    # Verify PostgreSQL is accessible
    log_info "Verifying PostgreSQL connection..."
    local pod_name=$(kubectl get pods -n "${NAMESPACE}" -l app=postgres -o jsonpath='{.items[0].metadata.name}')
    if kubectl exec -n "${NAMESPACE}" "${pod_name}" -- psql -U lumo -d lumo -c "SELECT 1" >/dev/null 2>&1; then
        log_success "✓ PostgreSQL is ready and accepting connections"
    else
        log_error "✗ PostgreSQL connection test failed"
        return 1
    fi
}

deploy_api_server() {
    if [ "$SKIP_API" = "true" ]; then
        log_info "Skipping API server deployment (SKIP_API=true)"
        return 0
    fi

    log_info "Step 4/7: Deploying Lumo API Server..."
    
    # Deploy API server
    kubectl apply -f manifests/api-server.yaml
    
    # Wait for API server to be ready
    log_info "Waiting for API server to be ready (timeout: 120s)..."
    kubectl rollout status deployment/lumo-api -n "${NAMESPACE}" --timeout=120s
    
    # Verify API server health
    log_info "Verifying API server health..."
    local pod_name=$(kubectl get pods -n "${NAMESPACE}" -l app=lumo-api -o jsonpath='{.items[0].metadata.name}')
    
    # Port-forward in background for health check
    kubectl port-forward -n "${NAMESPACE}" "pod/${pod_name}" 8081:8080 >/dev/null 2>&1 &
    local pf_pid=$!
    sleep 3
    
    if curl -s http://localhost:8081/api/v1/health >/dev/null 2>&1; then
        log_success "✓ API server is healthy and responding"
    else
        log_error "✗ API server health check failed"
        kill $pf_pid 2>/dev/null || true
        return 1
    fi
    
    # Kill port-forward
    kill $pf_pid 2>/dev/null || true
    wait $pf_pid 2>/dev/null || true
}

deploy_agent() {
    if [ "$SKIP_DEPLOY" = "true" ]; then
        log_info "Skipping agent deployment (SKIP_DEPLOY=true)"
        return 0
    fi

    log_info "Step 5/7: Deploying agents (DaemonSet + Deployment)..."
    
    # Set API endpoint to point to our in-cluster API server
    export LUMO_API_ENDPOINT="http://lumo-api.${NAMESPACE}.svc.cluster.local:8080"
    export SKIP_CLUSTER_SETUP=true
    export SKIP_BUILD=true
    
    ./deploy-to-kind.sh
}

run_component_tests() {
    log_info "Step 6/7: Running component tests..."
    echo ""

    # Test 1: Check if all pods are running
    log_info "Test 1: Checking pod status..."
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
    if kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- psql -U lumo -d lumo -c "SELECT COUNT(*) FROM information_schema.tables" >/dev/null 2>&1; then
        log_success "✓ PostgreSQL is accessible and functional"
    else
        log_error "✗ PostgreSQL connection failed"
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
    local agent_pod=$(kubectl get pods -n "${NAMESPACE}" -l app.kubernetes.io/component=cluster-monitor -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

    if [ -n "$agent_pod" ]; then
        kubectl port-forward -n "${NAMESPACE}" pod/"${agent_pod}" 8082:8080 >/dev/null 2>&1 &
        local pf_pid=$!
        sleep 2

        if curl -s http://localhost:8082/health >/dev/null 2>&1; then
            log_success "✓ Agent health endpoint responding"
        else
            log_error "✗ Agent health endpoint not responding"
        fi

        if curl -s http://localhost:8082/ready >/dev/null 2>&1; then
            log_success "✓ Agent ready endpoint responding"
        else
            log_error "✗ Agent ready endpoint not responding"
        fi

        kill $pf_pid 2>/dev/null || true
        wait $pf_pid 2>/dev/null || true
    else
        log_error "✗ No cluster-monitor pod found"
    fi

    # Test 5: Check agent metrics
    log_info "Test 5: Checking agent metrics endpoint..."
    if [ -n "$agent_pod" ]; then
        kubectl port-forward -n "${NAMESPACE}" pod/"${agent_pod}" 9090:9090 >/dev/null 2>&1 &
        local pf_pid=$!
        sleep 2

        if curl -s http://localhost:9090/metrics | grep -q "lumo_agent"; then
            log_success "✓ Metrics endpoint responding with lumo_agent metrics"
        else
            log_error "✗ Metrics endpoint not responding correctly"
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

    # Test 7: Check DaemonSet scheduling
    log_info "Test 7: Checking DaemonSet scheduling..."
    local node_count=$(kubectl get nodes -o json | jq -r '.items | length')
    local ds_scheduled=$(kubectl get daemonset -n "${NAMESPACE}" lumo-agent-node -o json | jq -r '.status.numberReady // 0')

    if [ "$ds_scheduled" -eq "$node_count" ]; then
        log_success "✓ DaemonSet scheduled on all nodes (${ds_scheduled}/${node_count})"
    else
        log_error "✗ DaemonSet not fully scheduled (${ds_scheduled}/${node_count})"
    fi
}

run_integration_tests() {
    log_info "Step 7/7: Running integration tests..."
    echo ""

    # Test 1: Check agent registration with API
    log_info "Test 1: Checking agent registration..."
    
    # Give agents time to register
    sleep 5
    
    local api_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=lumo-api -o jsonpath='{.items[0].metadata.name}')
    
    # Check API logs for agent registration
    if kubectl logs -n "${NAMESPACE}" "${api_pod}" --tail=100 | grep -iq "agent.*register"; then
        log_success "✓ Agent registration activity found in API logs"
    else
        log_error "✗ No agent registration found in API logs"
        log_info "API server logs (last 20 lines):"
        kubectl logs -n "${NAMESPACE}" "${api_pod}" --tail=20
    fi

    # Test 2: Check agent is attempting to communicate with API
    log_info "Test 2: Checking agent → API communication..."
    local agent_pod=$(kubectl get pods -n "${NAMESPACE}" -l app.kubernetes.io/component=cluster-monitor -o jsonpath='{.items[0].metadata.name}')
    
    if kubectl logs -n "${NAMESPACE}" "${agent_pod}" --tail=50 | grep -iq "api\|register\|heartbeat"; then
        log_success "✓ Agent is attempting API communication"
    else
        log_error "✗ No API communication attempts in agent logs"
        log_info "Agent logs (last 20 lines):"
        kubectl logs -n "${NAMESPACE}" "${agent_pod}" --tail=20
    fi

    # Test 3: Check for critical errors in any component
    log_info "Test 3: Checking for critical errors across all components..."
    local error_count=$(kubectl logs -n "${NAMESPACE}" --all-containers --tail=200 2>/dev/null | grep -i "fatal\|panic" | wc -l || echo "0")

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
    echo "  ✓ Lumo API Server"
    echo "  ✓ Lumo Agents (DaemonSet + Deployment)"
    echo ""
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
  --cluster-name         Name of kind cluster (default: lumo-test)
  --namespace            Kubernetes namespace (default: lumo-system)
  -h, --help             Show this help message

Environment variables:
  SKIP_CLUSTER_SETUP      Set to 'true' to skip cluster setup
  SKIP_BUILD              Set to 'true' to skip image build
  SKIP_INFRASTRUCTURE     Set to 'true' to skip PostgreSQL deployment
  SKIP_API                Set to 'true' to skip API server deployment
  SKIP_DEPLOY             Set to 'true' to skip agent deployment
  KIND_CLUSTER_NAME       Name of kind cluster
  LUMO_NAMESPACE          Kubernetes namespace

Examples:
  # Full stack deployment (recommended)
  $0

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

    deploy_agent
    echo ""

    run_component_tests
    echo ""

    run_integration_tests
    echo ""

    print_summary
}

main "$@"
