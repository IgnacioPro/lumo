#!/usr/bin/env bash
#
# Complete end-to-end test of Lumo Agent in kind
# This script: creates cluster → builds image → deploys agent → runs tests
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

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_banner() {
    echo ""
    echo "=========================================="
    echo "  Lumo Agent - kind Testing Suite"
    echo "=========================================="
    echo ""
}

setup_cluster() {
    if [ "$SKIP_CLUSTER_SETUP" = "true" ]; then
        log_info "Skipping cluster setup (SKIP_CLUSTER_SETUP=true)"
        return 0
    fi

    log_info "Step 1/4: Setting up kind cluster..."
    ./setup-kind-cluster.sh
}

build_and_load() {
    if [ "$SKIP_BUILD" = "true" ]; then
        log_info "Skipping image build (SKIP_BUILD=true)"
        return 0
    fi

    log_info "Step 2/4: Building and loading image..."
    ./build-and-load.sh
}

deploy_agent() {
    if [ "$SKIP_DEPLOY" = "true" ]; then
        log_info "Skipping deployment (SKIP_DEPLOY=true)"
        return 0
    fi

    log_info "Step 3/4: Deploying agent..."
    ./deploy-to-kind.sh
}

run_tests() {
    log_info "Step 4/4: Running tests..."
    echo ""

    # Test 1: Check if pods are running
    log_info "Test 1: Checking pod status..."
    local running_pods=$(kubectl get pods -n "${NAMESPACE}" -o json | jq -r '.items[] | select(.status.phase=="Running") | .metadata.name' | wc -l)
    local total_pods=$(kubectl get pods -n "${NAMESPACE}" -o json | jq -r '.items | length')

    if [ "$running_pods" -gt 0 ]; then
        log_success "✓ Pods are running (${running_pods}/${total_pods})"
    else
        log_error "✗ No pods are running"
        return 1
    fi

    # Test 2: Check health endpoint
    log_info "Test 2: Checking health endpoints..."
    local pod_name=$(kubectl get pods -n "${NAMESPACE}" -l app.kubernetes.io/component=cluster-monitor -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

    if [ -n "$pod_name" ]; then
        log_info "Testing health endpoint on pod: ${pod_name}"

        # Port-forward in background
        kubectl port-forward -n "${NAMESPACE}" pod/"${pod_name}" 8080:8080 >/dev/null 2>&1 &
        local pf_pid=$!
        sleep 2

        # Test endpoints
        if curl -s http://localhost:8080/health >/dev/null 2>&1; then
            log_success "✓ Health endpoint responding"
        else
            log_error "✗ Health endpoint not responding"
        fi

        if curl -s http://localhost:8080/ready >/dev/null 2>&1; then
            log_success "✓ Ready endpoint responding"
        else
            log_error "✗ Ready endpoint not responding"
        fi

        # Kill port-forward
        kill $pf_pid 2>/dev/null || true
        wait $pf_pid 2>/dev/null || true
    else
        log_error "✗ No cluster-monitor pod found"
    fi

    # Test 3: Check metrics endpoint
    log_info "Test 3: Checking metrics endpoint..."
    if [ -n "$pod_name" ]; then
        # Port-forward in background
        kubectl port-forward -n "${NAMESPACE}" pod/"${pod_name}" 9090:9090 >/dev/null 2>&1 &
        local pf_pid=$!
        sleep 2

        if curl -s http://localhost:9090/metrics | grep -q "lumo_agent"; then
            log_success "✓ Metrics endpoint responding with lumo_agent metrics"
        else
            log_error "✗ Metrics endpoint not responding correctly"
        fi

        # Kill port-forward
        kill $pf_pid 2>/dev/null || true
        wait $pf_pid 2>/dev/null || true
    fi

    # Test 4: Check logs for errors
    log_info "Test 4: Checking logs for critical errors..."
    local error_count=$(kubectl logs -n "${NAMESPACE}" -l app.kubernetes.io/name=lumo-agent --tail=100 2>/dev/null | grep -i "error\|fatal\|panic" | wc -l || echo "0")

    if [ "$error_count" -eq 0 ]; then
        log_success "✓ No critical errors in logs"
    else
        log_error "✗ Found ${error_count} error messages in logs"
        log_info "Showing recent errors:"
        kubectl logs -n "${NAMESPACE}" -l app.kubernetes.io/name=lumo-agent --tail=100 2>/dev/null | grep -i "error\|fatal\|panic" | head -5
    fi

    # Test 5: Check RBAC permissions
    log_info "Test 5: Checking RBAC permissions..."
    if kubectl auth can-i list nodes --as=system:serviceaccount:${NAMESPACE}:lumo-agent >/dev/null 2>&1; then
        log_success "✓ ServiceAccount has required permissions"
    else
        log_error "✗ ServiceAccount missing required permissions"
    fi

    # Test 6: Check DaemonSet scheduling
    log_info "Test 6: Checking DaemonSet scheduling..."
    local node_count=$(kubectl get nodes -o json | jq -r '.items | length')
    local ds_scheduled=$(kubectl get daemonset -n "${NAMESPACE}" lumo-agent-node -o json | jq -r '.status.numberReady // 0')

    if [ "$ds_scheduled" -eq "$node_count" ]; then
        log_success "✓ DaemonSet scheduled on all nodes (${ds_scheduled}/${node_count})"
    else
        log_error "✗ DaemonSet not fully scheduled (${ds_scheduled}/${node_count})"
    fi
}

print_summary() {
    echo ""
    echo "=========================================="
    log_success "Testing complete!"
    echo "=========================================="
    echo ""
    echo "Cluster: ${CLUSTER_NAME}"
    echo "Namespace: ${NAMESPACE}"
    echo ""
    log_info "View full logs:"
    echo "  ${BLUE}kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=lumo-agent -f${NC}"
    echo ""
    log_info "To tear down the test environment:"
    echo "  ${BLUE}kind delete cluster --name ${CLUSTER_NAME}${NC}"
    echo ""
}

show_usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Complete end-to-end testing of Lumo Agent in kind.

Options:
  --skip-cluster    Skip cluster creation (use existing)
  --skip-build      Skip image build (use existing image)
  --skip-deploy     Skip deployment (test existing deployment)
  --cluster-name    Name of kind cluster (default: lumo-test)
  --namespace       Kubernetes namespace (default: lumo-system)
  -h, --help        Show this help message

Environment variables:
  SKIP_CLUSTER_SETUP  Set to 'true' to skip cluster setup
  SKIP_BUILD          Set to 'true' to skip image build
  SKIP_DEPLOY         Set to 'true' to skip deployment
  KIND_CLUSTER_NAME   Name of kind cluster
  LUMO_NAMESPACE      Kubernetes namespace

Examples:
  # Full test (cluster creation, build, deploy, test)
  $0

  # Test with existing cluster and image
  $0 --skip-cluster --skip-build

  # Quick test of existing deployment
  SKIP_CLUSTER_SETUP=true SKIP_BUILD=true SKIP_DEPLOY=true $0

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

    deploy_agent
    echo ""

    run_tests
    echo ""

    print_summary
}

main "$@"
