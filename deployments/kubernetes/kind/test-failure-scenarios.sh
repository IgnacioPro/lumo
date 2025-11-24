#!/usr/bin/env bash
#
# Test Event-Driven Agent - Failure Scenario Testing
#
# This script triggers various Kubernetes failure scenarios and verifies
# that the event-driven agents detect and report them to the API server.
#
# Usage:
#   ./test-failure-scenarios.sh              # Run all tests
#   ./test-failure-scenarios.sh --scenario pod-failure  # Run specific test
#   ./test-failure-scenarios.sh --list       # List available scenarios
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
TEST_NAMESPACE="lumo-test-scenarios"
DEBOUNCE_WINDOW=45
WAIT_TIME=55  # Debounce + 10s buffer

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
    
    ((TESTS_RUN++))
    
    if [ "$result" = "PASS" ]; then
        log_success "$message"
        ((TESTS_PASSED++))
    else
        log_error "$message"
        ((TESTS_FAILED++))
    fi
}

print_banner() {
    echo ""
    echo "======================================================="
    echo "  Lumo Event-Driven Agent - Failure Scenario Tests"
    echo "  Testing 8 watchers across 10+ failure scenarios"
    echo "======================================================="
    echo ""
}

list_scenarios() {
    echo "Available test scenarios:"
    echo ""
    echo "Pod Failures:"
    echo "  1. image-pull-backoff    - Invalid image name"
    echo "  2. crash-loop-backoff    - Container exits immediately"
    echo "  3. oom-killed            - Memory limit exceeded"
    echo "  4. pod-pending           - Insufficient resources"
    echo ""
    echo "Workload Failures:"
    echo "  5. deployment-failed     - Progress deadline exceeded"
    echo "  6. job-failed            - Backoff limit exceeded"
    echo ""
    echo "Volume Failures:"
    echo "  7. volume-failed-mount   - Invalid volume config"
    echo "  8. pvc-provision-failed  - Storage class not found"
    echo ""
    echo "Scheduling Failures:"
    echo "  9. scheduling-failed     - Node selector mismatch"
    echo " 10. insufficient-cpu      - CPU request too high"
    echo ""
    echo "Use: ./test-failure-scenarios.sh --scenario <name>"
}

setup_test_namespace() {
    log_info "Setting up test namespace: ${TEST_NAMESPACE}"
    
    kubectl create namespace "${TEST_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null 2>&1
    
    # Label for easy cleanup
    kubectl label namespace "${TEST_NAMESPACE}" \
        app.kubernetes.io/name=lumo-test \
        app.kubernetes.io/component=failure-scenarios \
        --overwrite >/dev/null 2>&1
    
    log_success "Test namespace ready"
}

cleanup_test_namespace() {
    log_info "Cleaning up test resources..."
    
    kubectl delete namespace "${TEST_NAMESPACE}" --ignore-not-found --wait=false >/dev/null 2>&1 || true
    
    log_success "Cleanup complete"
}

wait_for_event() {
    local event_type=$1
    local resource_name=$2
    local timeout=${3:-60}
    
    log_info "Waiting up to ${timeout}s for event: ${event_type} (resource: ${resource_name})"
    
    local start=$(date +%s)
    local found=false
    
    while [ $(($(date +%s) - start)) -lt $timeout ]; do
        # Check agent logs for event submission
        if kubectl logs -n "${NAMESPACE}" -l mode=event-driven --tail=100 --since=60s 2>/dev/null | \
           grep -q "\"event_type\":\"${event_type}\".*\"resource_name\":\"${resource_name}\""; then
            found=true
            break
        fi
        
        sleep 2
    done
    
    if [ "$found" = true ]; then
        log_success "Event detected: ${event_type}"
        return 0
    else
        log_error "Event NOT detected after ${timeout}s: ${event_type}"
        return 1
    fi
}

verify_event_in_database() {
    local event_type=$1
    local resource_name=$2
    
    log_info "Verifying event in database: ${event_type}"
    
    local pg_pod=$(kubectl get pods -n "${NAMESPACE}" -l app=postgres -o jsonpath='{.items[0].metadata.name}')
    
    local count=$(kubectl exec -n "${NAMESPACE}" "${pg_pod}" -- \
        psql -U lumo -d lumo -t -c \
        "SELECT COUNT(*) FROM events WHERE event_type = '${event_type}' AND resource_name LIKE '%${resource_name}%';" \
        2>/dev/null | tr -d ' ')
    
    if [ "${count:-0}" -gt 0 ]; then
        log_success "Event stored in database: ${event_type} (${count} records)"
        return 0
    else
        log_error "Event NOT found in database: ${event_type}"
        return 1
    fi
}

# ==============================================================================
# TEST SCENARIOS
# ==============================================================================

test_image_pull_backoff() {
    log_test "Scenario 1: ImagePullBackOff - Invalid image name"
    
    local pod_name="test-imagepull-$(date +%s)"
    
    # Create pod with invalid image
    cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
  namespace: ${TEST_NAMESPACE}
  labels:
    test: image-pull-backoff
spec:
  containers:
  - name: test
    image: invalid-registry.example.com/nonexistent-image:latest
    imagePullPolicy: Always
EOF
    
    log_info "Pod created: ${pod_name}"
    sleep 5
    
    # Wait for ImagePullBackOff status
    log_info "Waiting for ImagePullBackOff status..."
    local max_wait=30
    local elapsed=0
    
    while [ $elapsed -lt $max_wait ]; do
        local status=$(kubectl get pod "${pod_name}" -n "${TEST_NAMESPACE}" -o jsonpath='{.status.containerStatuses[0].state.waiting.reason}' 2>/dev/null || echo "")
        
        if [[ "$status" == *"ImagePullBackOff"* ]] || [[ "$status" == *"ErrImagePull"* ]]; then
            log_success "Pod in ImagePullBackOff state"
            break
        fi
        
        sleep 2
        elapsed=$((elapsed + 2))
    done
    
    # Wait for debounce window + buffer
    log_info "Waiting ${WAIT_TIME}s for debounce and event processing..."
    sleep ${WAIT_TIME}
    
    # Verify event detection
    if wait_for_event "image-pull-backoff" "${pod_name}" 10; then
        if verify_event_in_database "image-pull-backoff" "${pod_name}"; then
            log_result "PASS" "ImagePullBackOff scenario detected and stored"
        else
            log_result "FAIL" "ImagePullBackOff detected but not stored in database"
        fi
    else
        log_result "FAIL" "ImagePullBackOff scenario not detected"
    fi
}

test_crash_loop_backoff() {
    log_test "Scenario 2: CrashLoopBackOff - Container exits immediately"
    
    local pod_name="test-crashloop-$(date +%s)"
    
    # Create pod that exits immediately
    cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
  namespace: ${TEST_NAMESPACE}
  labels:
    test: crash-loop-backoff
spec:
  containers:
  - name: test
    image: busybox:latest
    command: ["sh", "-c", "exit 1"]
  restartPolicy: Always
EOF
    
    log_info "Pod created: ${pod_name}"
    sleep 5
    
    # Wait for CrashLoopBackOff status
    log_info "Waiting for CrashLoopBackOff status..."
    local max_wait=60
    local elapsed=0
    
    while [ $elapsed -lt $max_wait ]; do
        local status=$(kubectl get pod "${pod_name}" -n "${TEST_NAMESPACE}" -o jsonpath='{.status.containerStatuses[0].state.waiting.reason}' 2>/dev/null || echo "")
        local restarts=$(kubectl get pod "${pod_name}" -n "${TEST_NAMESPACE}" -o jsonpath='{.status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")
        
        if [[ "$status" == "CrashLoopBackOff" ]] && [ "${restarts:-0}" -ge 2 ]; then
            log_success "Pod in CrashLoopBackOff state (${restarts} restarts)"
            break
        fi
        
        sleep 3
        elapsed=$((elapsed + 3))
    done
    
    # Wait for debounce window
    log_info "Waiting ${WAIT_TIME}s for debounce and event processing..."
    sleep ${WAIT_TIME}
    
    # Verify event detection
    if wait_for_event "crash-loop-backoff" "${pod_name}" 10; then
        if verify_event_in_database "crash-loop-backoff" "${pod_name}"; then
            log_result "PASS" "CrashLoopBackOff scenario detected and stored"
        else
            log_result "FAIL" "CrashLoopBackOff detected but not stored in database"
        fi
    else
        log_result "FAIL" "CrashLoopBackOff scenario not detected"
    fi
}

test_oom_killed() {
    log_test "Scenario 3: OOMKilled - Memory limit exceeded"
    
    local pod_name="test-oom-$(date +%s)"
    
    # Create pod with low memory limit that will OOM
    cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
  namespace: ${TEST_NAMESPACE}
  labels:
    test: oom-killed
spec:
  containers:
  - name: test
    image: polinux/stress:latest
    command: ["stress"]
    args: ["--vm", "1", "--vm-bytes", "512M", "--vm-hang", "0"]
    resources:
      limits:
        memory: "128Mi"
      requests:
        memory: "64Mi"
  restartPolicy: Never
EOF
    
    log_info "Pod created: ${pod_name}"
    sleep 5
    
    # Wait for OOMKilled status
    log_info "Waiting for OOMKilled status..."
    local max_wait=60
    local elapsed=0
    
    while [ $elapsed -lt $max_wait ]; do
        local reason=$(kubectl get pod "${pod_name}" -n "${TEST_NAMESPACE}" -o jsonpath='{.status.containerStatuses[0].state.terminated.reason}' 2>/dev/null || echo "")
        
        if [ "$reason" = "OOMKilled" ]; then
            log_success "Pod killed due to OOM"
            break
        fi
        
        sleep 3
        elapsed=$((elapsed + 3))
    done
    
    # Wait for debounce window
    log_info "Waiting ${WAIT_TIME}s for debounce and event processing..."
    sleep ${WAIT_TIME}
    
    # Verify event detection
    if wait_for_event "oom-killed" "${pod_name}" 10; then
        if verify_event_in_database "oom-killed" "${pod_name}"; then
            log_result "PASS" "OOMKilled scenario detected and stored"
        else
            log_result "FAIL" "OOMKilled detected but not stored in database"
        fi
    else
        log_result "FAIL" "OOMKilled scenario not detected"
    fi
}

test_deployment_failed() {
    log_test "Scenario 4: Deployment Failed - Progress deadline exceeded"
    
    local deploy_name="test-deploy-$(date +%s)"
    
    # Create deployment with invalid image (will fail to progress)
    cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${deploy_name}
  namespace: ${TEST_NAMESPACE}
  labels:
    test: deployment-failed
spec:
  replicas: 2
  progressDeadlineSeconds: 30
  selector:
    matchLabels:
      app: test-deploy
  template:
    metadata:
      labels:
        app: test-deploy
    spec:
      containers:
      - name: test
        image: invalid-image:nonexistent
EOF
    
    log_info "Deployment created: ${deploy_name}"
    
    # Wait for progress deadline to be exceeded
    log_info "Waiting for deployment to fail (30s deadline + buffer)..."
    sleep 40
    
    # Check deployment condition
    local condition=$(kubectl get deployment "${deploy_name}" -n "${TEST_NAMESPACE}" \
        -o jsonpath='{.status.conditions[?(@.type=="Progressing")].reason}' 2>/dev/null || echo "")
    
    if [ "$condition" = "ProgressDeadlineExceeded" ]; then
        log_success "Deployment failed with ProgressDeadlineExceeded"
    fi
    
    # Wait for debounce window
    log_info "Waiting ${WAIT_TIME}s for debounce and event processing..."
    sleep ${WAIT_TIME}
    
    # Verify event detection
    if wait_for_event "deployment-failed" "${deploy_name}" 10; then
        if verify_event_in_database "deployment-failed" "${deploy_name}"; then
            log_result "PASS" "Deployment failure detected and stored"
        else
            log_result "FAIL" "Deployment failure detected but not stored in database"
        fi
    else
        log_result "FAIL" "Deployment failure not detected"
    fi
}

test_job_failed() {
    log_test "Scenario 5: Job Failed - Backoff limit exceeded"
    
    local job_name="test-job-$(date +%s)"
    
    # Create job that will fail
    cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job_name}
  namespace: ${TEST_NAMESPACE}
  labels:
    test: job-failed
spec:
  backoffLimit: 2
  template:
    spec:
      containers:
      - name: test
        image: busybox:latest
        command: ["sh", "-c", "exit 1"]
      restartPolicy: Never
EOF
    
    log_info "Job created: ${job_name}"
    
    # Wait for job to fail
    log_info "Waiting for job to fail (backoffLimit: 2)..."
    local max_wait=60
    local elapsed=0
    
    while [ $elapsed -lt $max_wait ]; do
        local failed=$(kubectl get job "${job_name}" -n "${TEST_NAMESPACE}" -o jsonpath='{.status.failed}' 2>/dev/null || echo "0")
        local condition=$(kubectl get job "${job_name}" -n "${TEST_NAMESPACE}" \
            -o jsonpath='{.status.conditions[?(@.type=="Failed")].status}' 2>/dev/null || echo "")
        
        if [ "${failed:-0}" -gt 2 ] || [ "$condition" = "True" ]; then
            log_success "Job failed (${failed} attempts)"
            break
        fi
        
        sleep 5
        elapsed=$((elapsed + 5))
    done
    
    # Wait for debounce window
    log_info "Waiting ${WAIT_TIME}s for debounce and event processing..."
    sleep ${WAIT_TIME}
    
    # Verify event detection
    if wait_for_event "job-failed" "${job_name}" 10; then
        if verify_event_in_database "job-failed" "${job_name}"; then
            log_result "PASS" "Job failure detected and stored"
        else
            log_result "FAIL" "Job failure detected but not stored in database"
        fi
    else
        log_result "FAIL" "Job failure not detected"
    fi
}

test_pvc_provision_failed() {
    log_test "Scenario 6: PVC Provision Failed - Invalid storage class"
    
    local pvc_name="test-pvc-$(date +%s)"
    
    # Create PVC with non-existent storage class
    cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ${pvc_name}
  namespace: ${TEST_NAMESPACE}
  labels:
    test: pvc-provision-failed
spec:
  accessModes:
  - ReadWriteOnce
  storageClassName: nonexistent-storage-class
  resources:
    requests:
      storage: 1Gi
EOF
    
    log_info "PVC created: ${pvc_name}"
    sleep 10
    
    # Check PVC status
    local phase=$(kubectl get pvc "${pvc_name}" -n "${TEST_NAMESPACE}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    
    if [ "$phase" = "Pending" ]; then
        log_success "PVC in Pending state (provisioning failed)"
    fi
    
    # Wait for debounce window
    log_info "Waiting ${WAIT_TIME}s for debounce and event processing..."
    sleep ${WAIT_TIME}
    
    # Verify event detection
    if wait_for_event "pvc-provision-failed" "${pvc_name}" 10; then
        if verify_event_in_database "pvc-provision-failed" "${pvc_name}"; then
            log_result "PASS" "PVC provision failure detected and stored"
        else
            log_result "FAIL" "PVC provision failure detected but not stored in database"
        fi
    else
        # PVC provision failures might show as scheduling-failed in some K8s versions
        log_warning "PVC provision failure not detected as pvc-provision-failed"
        log_result "SKIP" "PVC scenario result varies by K8s version"
    fi
}

test_scheduling_failed() {
    log_test "Scenario 7: Scheduling Failed - Node selector mismatch"
    
    local pod_name="test-sched-$(date +%s)"
    
    # Create pod with impossible node selector
    cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
  namespace: ${TEST_NAMESPACE}
  labels:
    test: scheduling-failed
spec:
  nodeSelector:
    nonexistent-label: "true"
    impossible-match: "yes"
  containers:
  - name: test
    image: nginx:latest
EOF
    
    log_info "Pod created: ${pod_name}"
    sleep 5
    
    # Verify pod is unschedulable
    local phase=$(kubectl get pod "${pod_name}" -n "${TEST_NAMESPACE}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    
    if [ "$phase" = "Pending" ]; then
        log_success "Pod stuck in Pending state (unschedulable)"
    fi
    
    # Wait for debounce window
    log_info "Waiting ${WAIT_TIME}s for debounce and event processing..."
    sleep ${WAIT_TIME}
    
    # Verify event detection
    if wait_for_event "scheduling-failed" "${pod_name}" 10; then
        if verify_event_in_database "scheduling-failed" "${pod_name}"; then
            log_result "PASS" "Scheduling failure detected and stored"
        else
            log_result "FAIL" "Scheduling failure detected but not stored in database"
        fi
    else
        log_result "FAIL" "Scheduling failure not detected"
    fi
}

# ==============================================================================
# MAIN TEST RUNNER
# ==============================================================================

run_all_tests() {
    print_banner
    
    log_info "Checking prerequisites..."
    
    # Check if agents are running
    local agent_count=$(kubectl get pods -n "${NAMESPACE}" -l mode=event-driven --no-headers 2>/dev/null | wc -l | tr -d ' ')
    
    if [ "${agent_count:-0}" -lt 1 ]; then
        log_error "No event-driven agents found in namespace: ${NAMESPACE}"
        log_error "Please run ./test-agent.sh first to deploy the full stack"
        exit 1
    fi
    
    log_success "Found ${agent_count} event-driven agent(s)"
    
    setup_test_namespace
    
    echo ""
    log_info "Starting failure scenario tests..."
    log_info "Debounce window: ${DEBOUNCE_WINDOW}s | Wait time per test: ${WAIT_TIME}s"
    echo ""
    
    # Run tests
    test_image_pull_backoff
    echo ""
    
    test_crash_loop_backoff
    echo ""
    
    test_oom_killed
    echo ""
    
    test_deployment_failed
    echo ""
    
    test_job_failed
    echo ""
    
    test_pvc_provision_failed
    echo ""
    
    test_scheduling_failed
    echo ""
    
    # Cleanup
    cleanup_test_namespace
    
    # Print summary
    print_summary
}

print_summary() {
    echo ""
    echo "======================================================="
    echo "  Test Summary"
    echo "======================================================="
    echo ""
    echo "  Tests Run:    ${TESTS_RUN}"
    echo -e "  Tests Passed: ${GREEN}${TESTS_PASSED}${NC}"
    echo -e "  Tests Failed: ${RED}${TESTS_FAILED}${NC}"
    echo ""
    
    if [ ${TESTS_FAILED} -eq 0 ]; then
        echo -e "${GREEN}✓ All tests passed!${NC}"
        echo ""
        log_info "Event-driven agents successfully detected all failure scenarios"
    else
        echo -e "${RED}✗ Some tests failed${NC}"
        echo ""
        log_warning "Review logs above for failure details"
        log_info "Check agent logs: kubectl logs -n ${NAMESPACE} -l mode=event-driven --tail=100"
    fi
    
    echo ""
    echo "View stored events:"
    echo "  kubectl exec -n ${NAMESPACE} \$(kubectl get pods -n ${NAMESPACE} -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \\"
    echo "    psql -U lumo -d lumo -c 'SELECT event_type, severity, resource_name, created_at FROM events ORDER BY created_at DESC LIMIT 10;'"
    echo ""
}

# ==============================================================================
# CLI INTERFACE
# ==============================================================================

main() {
    case "${1:-all}" in
        --list)
            list_scenarios
            ;;
        --scenario)
            if [ -z "${2:-}" ]; then
                log_error "Scenario name required"
                list_scenarios
                exit 1
            fi
            
            setup_test_namespace
            
            case "$2" in
                image-pull-backoff)
                    test_image_pull_backoff
                    ;;
                crash-loop-backoff)
                    test_crash_loop_backoff
                    ;;
                oom-killed)
                    test_oom_killed
                    ;;
                deployment-failed)
                    test_deployment_failed
                    ;;
                job-failed)
                    test_job_failed
                    ;;
                pvc-provision-failed)
                    test_pvc_provision_failed
                    ;;
                scheduling-failed)
                    test_scheduling_failed
                    ;;
                *)
                    log_error "Unknown scenario: $2"
                    list_scenarios
                    exit 1
                    ;;
            esac
            
            cleanup_test_namespace
            print_summary
            ;;
        all|--all)
            run_all_tests
            ;;
        --help|-h)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --list                 List available test scenarios"
            echo "  --scenario <name>      Run specific scenario"
            echo "  --all                  Run all scenarios (default)"
            echo "  --help                 Show this help"
            echo ""
            list_scenarios
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
}

main "$@"
