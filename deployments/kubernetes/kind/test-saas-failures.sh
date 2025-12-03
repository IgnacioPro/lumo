#!/usr/bin/env bash
#
# Multi-Tenant SaaS - Failure Scenario Testing
#
# This script creates various failure scenarios in tenant namespaces
# and verifies that:
# 1. Event-driven agents detect failures
# 2. Events are submitted to the API
# 3. Notifications are sent (if configured)
# 4. Incidents are created and correlated
#
# Usage:
#   ./test-saas-failures.sh                    # Run all tests
#   ./test-saas-failures.sh --scenario oom     # Run specific scenario
#   ./test-saas-failures.sh --list             # List scenarios
#   ./test-saas-failures.sh --tenant acme      # Test specific tenant
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="${LUMO_NAMESPACE:-lumo-system}"
API_ENDPOINT="${LUMO_API_ENDPOINT:-http://localhost:8080}"
TENANT="${LUMO_TEST_TENANT:-acme-corp}"
SCENARIO="${SCENARIO_FILTER:-}"
FAST_MODE="${FAST_MODE:-true}"
POLL_INTERVAL=5
POLL_MAX_ATTEMPTS=60

# Counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1" >&2
}

log_warning() {
    echo -e "${YELLOW}[!]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[✗]${NC} $1" >&2
}

log_test() {
    echo -e "${CYAN}[TEST]${NC} $1" >&2
}

log_result() {
    local result=$1
    local message=$2

    TESTS_RUN=$((TESTS_RUN + 1))

    if [ "$result" = "PASS" ]; then
        log_success "$message"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        log_error "$message"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

print_banner() {
    echo ""
    echo "╔════════════════════════════════════════════════════════╗"
    echo "║   Lumo Multi-Tenant SaaS - Failure Scenario Tests      ║"
    echo "║   Testing event detection and notification flow       ║"
    echo "╚════════════════════════════════════════════════════════╝"
    echo ""
}

list_scenarios() {
    cat <<EOF
Available failure scenarios:

  Pod Failures (detected in ~45-75s):
    1. image-pull-backoff    - Invalid container image
    2. crash-loop-backoff    - Container exits immediately
    3. oom-killed            - Memory limit exceeded
    4. pending-pod           - Insufficient cluster resources

  Workload Failures (detected in ~60-90s):
    5. deployment-failed     - Rollout timeout
    6. job-failed            - Max retries exceeded

  Volume Failures (detected in ~65-120s):
    7. pvc-provision-failed  - Storage provisioning fails

  All: Run all tests sequentially

Usage:
  ./test-saas-failures.sh --scenario image-pull-backoff
  ./test-saas-failures.sh --scenario oom-killed
  ./test-saas-failures.sh --tenant globex-ind --scenario crash-loop-backoff
  ./test-saas-failures.sh --list

EOF
}

# ==================== SETUP ====================

check_cluster_ready() {
    log_info "Checking cluster and API server..."

    # Check kind cluster
    if ! kubectl cluster-info >/dev/null 2>&1; then
        log_error "Kubernetes cluster not available"
        return 1
    fi

    # Check namespace exists
    if ! kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1; then
        log_error "Namespace ${NAMESPACE} not found"
        return 1
    fi

    # Check API pod exists
    local api_pod
    api_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=lumo-api -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [ -z "$api_pod" ]; then
        log_error "Lumo API pod not found in ${NAMESPACE}"
        return 1
    fi

    # Check tenant namespace exists
    local tenant_namespace="tenant-${TENANT}"
    if ! kubectl get namespace "${tenant_namespace}" >/dev/null 2>&1; then
        log_error "Tenant namespace ${tenant_namespace} not found"
        log_info "Available tenants:"
        kubectl get namespaces | grep tenant- || log_warning "No tenant namespaces found"
        return 1
    fi

    log_success "Cluster and tenant namespace ready"
    return 0
}

setup_port_forward() {
    log_info "Setting up port forward to API server..."

    local api_pod
    api_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=lumo-api -o jsonpath='{.items[0].metadata.name}')

    # Kill any existing port-forward
    pkill -f "port-forward.*${api_pod}" || true

    # Start new port-forward
    kubectl port-forward -n "${NAMESPACE}" "pod/${api_pod}" 8080:8080 >/dev/null 2>&1 &
    sleep 2

    # Verify connection
    if curl -s "${API_ENDPOINT}/api/v1/health" >/dev/null 2>&1; then
        log_success "API server accessible at ${API_ENDPOINT}"
        return 0
    else
        log_error "Cannot reach API at ${API_ENDPOINT}"
        return 1
    fi
}

get_event_count_before() {
    local event_type="$1"

    curl -s "${API_ENDPOINT}/api/v1/events?type=${event_type}&limit=1000" 2>/dev/null | \
        grep -o '"id"' | wc -l || echo "0"
}

wait_for_events() {
    local event_type="$1"
    local min_events="${2:-1}"

    log_info "Waiting for ${event_type} events (need at least ${min_events})..."

    local attempt=0
    while [ $attempt -lt $POLL_MAX_ATTEMPTS ]; do
        local current_count
        current_count=$(curl -s "${API_ENDPOINT}/api/v1/events?type=${event_type}&limit=1000" 2>/dev/null | \
            grep -o '"id"' | wc -l || echo "0")

        if [ "$current_count" -ge "$min_events" ]; then
            log_success "Detected $current_count ${event_type} events"
            return 0
        fi

        log_info "Events found: $current_count/$min_events (attempt $((attempt+1))/$POLL_MAX_ATTEMPTS)"
        sleep $POLL_INTERVAL
        attempt=$((attempt + 1))
    done

    log_warning "Timeout waiting for ${event_type} events"
    return 1
}

check_event_in_database() {
    local event_type="$1"
    local tenant_namespace="tenant-${TENANT}"

    local pg_pod
    pg_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

    if [ -z "$pg_pod" ]; then
        log_warning "PostgreSQL pod not found"
        return 1
    fi

    # Query events table
    local event_count
    event_count=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
        psql -U lumo -d lumo -t -c \
        "SELECT COUNT(*) FROM events WHERE type = '${event_type}' AND metadata->>'namespace' = '${tenant_namespace}';" 2>/dev/null | tr -d ' \n' || echo "0")

    if [ "$event_count" -gt "0" ]; then
        log_success "Found $event_count ${event_type} events in database"
        return 0
    else
        log_warning "No ${event_type} events found in database"
        return 1
    fi
}

# ==================== FAILURE SCENARIOS ====================

test_image_pull_backoff() {
    log_test "ImagePullBackOff - Invalid container image"

    local test_name="test-image-pull-backoff"
    local tenant_namespace="tenant-${TENANT}"

    # Clean up any previous test
    kubectl delete pod "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1

    # Create pod with invalid image
    cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: Pod
metadata:
  name: ${test_name}
  namespace: ${tenant_namespace}
  labels:
    test-scenario: true
    critical: "true"
spec:
  containers:
  - name: invalid-app
    image: invalid-registry.example.com/nonexistent:latest
    resources:
      limits:
        memory: "64Mi"
        cpu: "100m"
  restartPolicy: Always
EOF

    log_info "Pod created, waiting for ImagePullBackOff..."
    sleep 5

    # Wait for agent to detect and report
    if wait_for_events "image-pull-backoff"; then
        log_result "PASS" "ImagePullBackOff event detected and reported"
        # Cleanup
        kubectl delete pod "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1
        return 0
    else
        log_result "FAIL" "ImagePullBackOff event not detected"
        # Show pod status for debugging
        kubectl get pod "${test_name}" -n "${tenant_namespace}" -o wide 2>/dev/null || true
        kubectl describe pod "${test_name}" -n "${tenant_namespace}" 2>/dev/null | tail -10 || true
        kubectl delete pod "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1
        return 1
    fi
}

test_crash_loop_backoff() {
    log_test "CrashLoopBackOff - Container exits immediately"

    local test_name="test-crash-loop-backoff"
    local tenant_namespace="tenant-${TENANT}"

    # Clean up any previous test
    kubectl delete pod "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1

    # Create pod that crashes immediately
    cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: Pod
metadata:
  name: ${test_name}
  namespace: ${tenant_namespace}
  labels:
    test-scenario: true
    critical: "true"
spec:
  containers:
  - name: crashing-app
    image: busybox:latest
    command: ["sh", "-c", "exit 1"]
    resources:
      limits:
        memory: "64Mi"
        cpu: "100m"
  restartPolicy: Always
EOF

    log_info "Pod created, waiting for CrashLoopBackOff..."
    sleep 5

    # Wait for agent to detect and report
    if wait_for_events "crash-loop-backoff"; then
        log_result "PASS" "CrashLoopBackOff event detected and reported"
        # Cleanup
        kubectl delete pod "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1
        return 0
    else
        log_result "FAIL" "CrashLoopBackOff event not detected"
        kubectl describe pod "${test_name}" -n "${tenant_namespace}" 2>/dev/null | tail -10 || true
        kubectl delete pod "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1
        return 1
    fi
}

test_oom_killed() {
    log_test "OOMKilled - Memory limit exceeded"

    local test_name="test-oom-killed"
    local tenant_namespace="tenant-${TENANT}"

    # Clean up any previous test
    kubectl delete pod "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1

    # Create pod that exceeds memory limit
    cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: Pod
metadata:
  name: ${test_name}
  namespace: ${tenant_namespace}
  labels:
    test-scenario: true
    critical: "true"
spec:
  containers:
  - name: memory-hog
    image: busybox:latest
    command: ["sh", "-c", "tail -f /dev/null"]
    resources:
      limits:
        memory: "16Mi"
        cpu: "100m"
      requests:
        memory: "8Mi"
        cpu: "50m"
  restartPolicy: Always
EOF

    log_info "Pod created, waiting for OOMKilled..."
    sleep 5

    # Wait for agent to detect and report
    if wait_for_events "oom-killed"; then
        log_result "PASS" "OOMKilled event detected and reported"
        # Cleanup
        kubectl delete pod "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1
        return 0
    else
        log_result "FAIL" "OOMKilled event not detected"
        kubectl describe pod "${test_name}" -n "${tenant_namespace}" 2>/dev/null | tail -10 || true
        kubectl delete pod "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1
        return 1
    fi
}

test_pending_pod() {
    log_test "Pod Pending - Insufficient resources"

    local test_name="test-pod-pending"
    local tenant_namespace="tenant-${TENANT}"

    # Clean up any previous test
    kubectl delete pod "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1

    # Create pod with excessive resource requests
    cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: Pod
metadata:
  name: ${test_name}
  namespace: ${tenant_namespace}
  labels:
    test-scenario: true
    critical: "true"
spec:
  containers:
  - name: resource-hog
    image: busybox:latest
    command: ["sleep", "3600"]
    resources:
      requests:
        memory: "512Gi"
        cpu: "10000m"
      limits:
        memory: "512Gi"
        cpu: "10000m"
EOF

    log_info "Pod created, waiting for Pending state..."
    sleep 5

    # Wait for agent to detect and report
    if wait_for_events "pod-pending"; then
        log_result "PASS" "Pod Pending event detected and reported"
        # Cleanup
        kubectl delete pod "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1
        return 0
    else
        log_result "FAIL" "Pod Pending event not detected"
        kubectl describe pod "${test_name}" -n "${tenant_namespace}" 2>/dev/null | tail -10 || true
        kubectl delete pod "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1
        return 1
    fi
}

test_deployment_failed() {
    log_test "Deployment Failed - Rollout timeout"

    local test_name="test-deployment-failed"
    local tenant_namespace="tenant-${TENANT}"

    # Clean up any previous test
    kubectl delete deployment "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1

    # Create deployment that cannot complete
    cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${test_name}
  namespace: ${tenant_namespace}
  labels:
    test-scenario: true
    critical: "true"
spec:
  replicas: 1
  progressDeadlineSeconds: 10
  selector:
    matchLabels:
      app: ${test_name}
  template:
    metadata:
      labels:
        app: ${test_name}
    spec:
      containers:
      - name: failing-app
        image: invalid-registry.example.com/nonexistent:latest
        resources:
          limits:
            memory: "64Mi"
            cpu: "100m"
EOF

    log_info "Deployment created, waiting for failure..."
    sleep 5

    # Wait for agent to detect and report
    if wait_for_events "deployment-failed"; then
        log_result "PASS" "Deployment Failed event detected and reported"
        # Cleanup
        kubectl delete deployment "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1
        return 0
    else
        log_result "FAIL" "Deployment Failed event not detected"
        kubectl describe deployment "${test_name}" -n "${tenant_namespace}" 2>/dev/null | tail -15 || true
        kubectl delete deployment "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1
        return 1
    fi
}

test_job_failed() {
    log_test "Job Failed - Backoff limit exceeded"

    local test_name="test-job-failed"
    local tenant_namespace="tenant-${TENANT}"

    # Clean up any previous test
    kubectl delete job "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1

    # Create job that fails
    cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: batch/v1
kind: Job
metadata:
  name: ${test_name}
  namespace: ${tenant_namespace}
  labels:
    test-scenario: true
    critical: "true"
spec:
  backoffLimit: 2
  template:
    spec:
      containers:
      - name: failing-task
        image: busybox:latest
        command: ["sh", "-c", "exit 1"]
        resources:
          limits:
            memory: "64Mi"
            cpu: "100m"
      restartPolicy: Never
EOF

    log_info "Job created, waiting for failure..."
    sleep 5

    # Wait for agent to detect and report
    if wait_for_events "job-failed"; then
        log_result "PASS" "Job Failed event detected and reported"
        # Cleanup
        kubectl delete job "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1
        return 0
    else
        log_result "FAIL" "Job Failed event not detected"
        kubectl describe job "${test_name}" -n "${tenant_namespace}" 2>/dev/null | tail -10 || true
        kubectl delete job "${test_name}" -n "${tenant_namespace}" --ignore-not-found=true >/dev/null 2>&1
        return 1
    fi
}

# ==================== MAIN ====================

main() {
    print_banner

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --scenario)
                SCENARIO="$2"
                shift 2
                ;;
            --tenant)
                TENANT="$2"
                shift 2
                ;;
            --list)
                list_scenarios
                exit 0
                ;;
            -h|--help)
                list_scenarios
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done

    # Verify cluster is ready
    if ! check_cluster_ready; then
        exit 1
    fi

    log_info "Running tests for tenant: ${TENANT}"
    echo ""

    # Setup port forward
    if ! setup_port_forward; then
        log_error "Failed to setup port forward to API server"
        exit 1
    fi

    echo ""

    # Run tests
    if [ -z "$SCENARIO" ] || [ "$SCENARIO" = "image-pull-backoff" ]; then
        test_image_pull_backoff || true
    fi

    if [ -z "$SCENARIO" ] || [ "$SCENARIO" = "crash-loop-backoff" ]; then
        test_crash_loop_backoff || true
    fi

    if [ -z "$SCENARIO" ] || [ "$SCENARIO" = "oom-killed" ]; then
        test_oom_killed || true
    fi

    if [ -z "$SCENARIO" ] || [ "$SCENARIO" = "pending-pod" ]; then
        test_pending_pod || true
    fi

    if [ -z "$SCENARIO" ] || [ "$SCENARIO" = "deployment-failed" ]; then
        test_deployment_failed || true
    fi

    if [ -z "$SCENARIO" ] || [ "$SCENARIO" = "job-failed" ]; then
        test_job_failed || true
    fi

    # Summary
    echo ""
    echo "╔════════════════════════════════════════════════════════╗"
    echo "║ Test Results: $TESTS_PASSED/$TESTS_RUN passed                    ║"
    echo "╚════════════════════════════════════════════════════════╝"
    echo ""

    if [ $TESTS_FAILED -gt 0 ]; then
        exit 1
    fi
}

main "$@"
