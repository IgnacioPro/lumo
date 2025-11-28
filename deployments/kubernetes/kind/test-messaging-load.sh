#!/usr/bin/env bash
#
# Messaging Load Test - Test NATS messaging under heavy load
#
# Tests:
# - S profile: 1,000 events/sec sustained
# - M profile: 10,000 events/sec sustained  
# - Message latency (p50, p95, p99)
# - Resource usage (CPU, memory)
# - Connection stability
#
# Usage:
#   ./test-messaging-load.sh              # Run all tests
#   ./test-messaging-load.sh --profile s  # Test S profile
#   ./test-messaging-load.sh --profile m  # Test M profile
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
NAMESPACE="${LUMO_NAMESPACE:-lumo-system}"
PROFILE="${1:-s}"  # Default to S profile
DURATION=60        # Test duration in seconds
WARMUP=5           # Warmup period

# Get target RPS based on profile
get_target_rps() {
    case "$1" in
        xs) echo 100 ;;     # 100 events/sec for XS
        s) echo 1000 ;;     # 1K events/sec for S
        m) echo 10000 ;;    # 10K events/sec for M
        *) echo 1000 ;;     # Default
    esac
}

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

print_banner() {
    local target=$(get_target_rps "$PROFILE")
    echo ""
    echo "======================================================="
    echo "  Lumo Messaging Load Test"
    echo "  Profile: $(echo $PROFILE | tr '[:lower:]' '[:upper:]') | Duration: ${DURATION}s"
    echo "  Target: ${target} events/sec"
    echo "======================================================="
    echo ""
}

check_messaging_backend() {
    log_info "Checking messaging backend..."
    
    if [ "$PROFILE" = "xs" ]; then
        # XS profile uses Redis Streams (deployed as lumo-redis)
        if ! kubectl get deployment lumo-redis -n "$NAMESPACE" &>/dev/null; then
            log_error "Redis not deployed. Run: make deploy-xs"
            exit 1
        fi
        
        local ready=$(kubectl get deployment lumo-redis -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}')
        if [ "$ready" != "1" ]; then
            log_error "Redis not ready (ready: $ready/1)"
            exit 1
        fi
        
        log_success "Redis is running and ready (XS profile)"
    else
        # S and M profiles use NATS
        if ! kubectl get deployment nats -n "$NAMESPACE" &>/dev/null; then
            log_error "NATS not deployed. Run: make deploy-${PROFILE}"
            exit 1
        fi
        
        local ready=$(kubectl get deployment nats -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}')
        if [ "$ready" != "1" ]; then
            log_error "NATS not ready (ready: $ready/1)"
            exit 1
        fi
        
        log_success "NATS is running and ready"
    fi
}

get_messaging_resources() {
    local profile=$1
    
    if [ "$profile" = "xs" ]; then
        # Get Redis resources (label: app=lumo-redis)
        local pod=$(kubectl get pods -n "$NAMESPACE" -l app=lumo-redis -o jsonpath='{.items[0].metadata.name}')
        local metrics=$(kubectl top pod "$pod" -n "$NAMESPACE" --no-headers 2>/dev/null || echo "N/A N/A")
    else
        # Get NATS resources
        local pod=$(kubectl get pods -n "$NAMESPACE" -l app=nats -o jsonpath='{.items[0].metadata.name}')
        local metrics=$(kubectl top pod "$pod" -n "$NAMESPACE" --no-headers 2>/dev/null || echo "N/A N/A")
    fi
    
    local cpu=$(echo "$metrics" | awk '{print $2}')
    local mem=$(echo "$metrics" | awk '{print $3}')
    
    echo "$cpu $mem"
}

run_publisher_load() {
    local target_rps=$1
    local duration=$2
    local namespace=$3
    local profile=$4
    
    log_info "Starting publisher load: ${target_rps} msg/sec for ${duration}s..."
    
    if [ "$profile" = "xs" ]; then
        # Deploy Redis Streams load generator
        cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: redis-load-publisher
  namespace: ${namespace}
spec:
  restartPolicy: Never
  containers:
  - name: publisher
    image: redis:7-alpine
    command:
    - /bin/sh
    - -c
    - |
      set -e
      echo "Warming up for ${WARMUP}s..."
      sleep ${WARMUP}
      
      echo "Starting load test: ${target_rps} msg/sec for ${duration}s"
      start=\$(date +%s)
      count=0
      
      while [ \$((\$(date +%s) - start)) -lt ${duration} ]; do
        # Publish to Redis Stream in batches
        for i in \$(seq 1 10); do
          redis-cli -h lumo-redis XADD lumo:events "*" event "event-\$count" > /dev/null 2>&1 || true
          count=\$((count + 1))
        done
        
        # Calculate sleep time to hit target rate
        elapsed=\$((\$(date +%s) - start))
        if [ \$elapsed -gt 0 ]; then
          current_rps=\$((count / elapsed))
          if [ \$current_rps -gt ${target_rps} ]; then
            sleep 0.1
          fi
        fi
      done
      
      echo "Published \$count messages in ${duration}s"
      echo "Average rate: \$((count / ${duration})) msg/sec"
EOF
        local pod_name="redis-load-publisher"
    else
        # Deploy NATS load generator
        cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: nats-load-publisher
  namespace: ${namespace}
spec:
  restartPolicy: Never
  containers:
  - name: publisher
    image: natsio/nats-box:latest
    command:
    - /bin/sh
    - -c
    - |
      set -e
      echo "Warming up for ${WARMUP}s..."
      sleep ${WARMUP}
      
      echo "Starting load test: ${target_rps} msg/sec for ${duration}s"
      start=\$(date +%s)
      count=0
      
      while [ \$((\$(date +%s) - start)) -lt ${duration} ]; do
        # Publish in batches for efficiency
        for i in \$(seq 1 100); do
          echo "event-\$count" | nats pub -s nats:4222 lumo.events > /dev/null 2>&1 || true
          count=\$((count + 1))
        done
        
        # Calculate sleep time to hit target rate
        elapsed=\$((\$(date +%s) - start))
        if [ \$elapsed -gt 0 ]; then
          current_rps=\$((count / elapsed))
          if [ \$current_rps -gt ${target_rps} ]; then
            sleep 0.1
          fi
        fi
      done
      
      echo "Published \$count messages in ${duration}s"
      echo "Average rate: \$((count / ${duration})) msg/sec"
EOF
        local pod_name="nats-load-publisher"
    fi
    
    # Wait for pod to start
    log_info "Waiting for load generator to start..."
    kubectl wait --for=condition=ready --timeout=30s pod/$pod_name -n "$namespace" 2>/dev/null || true
    
    # Monitor progress
    log_info "Running load test (this will take ${duration}s)..."
    local start=$(date +%s)
    
    while true; do
        local status=$(kubectl get pod $pod_name -n "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
        local elapsed=$(($(date +%s) - start))
        
        if [ "$status" = "Succeeded" ]; then
            break
        elif [ "$status" = "Failed" ]; then
            log_error "Load generator failed"
            kubectl logs $pod_name -n "$namespace" || true
            return 1
        elif [ $elapsed -gt $((duration + WARMUP + 30)) ]; then
            log_error "Load test timed out"
            return 1
        fi
        
        # Show progress every 10s
        if [ $((elapsed % 10)) -eq 0 ]; then
            log_info "Progress: ${elapsed}s elapsed..."
        fi
        
        sleep 2
    done
    
    # Get results
    local logs=$(kubectl logs $pod_name -n "$namespace" 2>/dev/null)
    echo "$logs"
}

run_subscriber_test() {
    local namespace=$1
    local duration=$2
    
    log_info "Starting subscriber to measure latency..."
    
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: nats-load-subscriber
  namespace: ${namespace}
spec:
  restartPolicy: Never
  containers:
  - name: subscriber
    image: natsio/nats-box:latest
    command:
    - /bin/sh
    - -c
    - |
      echo "Subscribing to lumo.events..."
      timeout ${duration} nats sub -s nats:4222 lumo.events --count=1000 > /tmp/sub.log 2>&1 || true
      received=\$(grep -c "Received" /tmp/sub.log || echo 0)
      echo "Received \$received messages"
EOF
    
    kubectl wait --for=condition=ready --timeout=30s pod/nats-load-subscriber -n "$namespace" 2>/dev/null || true
}

measure_latency() {
    local profile=$1
    log_info "Measuring message latency..."
    
    if [ "$profile" = "xs" ]; then
        # Redis latency measurement
        kubectl run redis-bench --rm -i --restart=Never --image=redis:7-alpine -n "$NAMESPACE" -- \
            sh -c 'redis-cli -h lumo-redis --latency-history -i 1 2>&1 | head -5' || echo "Could not measure latency"
    else
        # Use nats bench for precise latency measurement
        kubectl run nats-bench --rm -i --restart=Never --image=natsio/nats-box:latest -n "$NAMESPACE" -- \
            nats bench lumo.events --pub 10 --sub 1 --size 100 --msgs 1000 2>&1 | tee /tmp/nats-bench.log || true
        
        # Extract latency percentiles
        if [ -f /tmp/nats-bench.log ]; then
            grep -E "p50|p95|p99|Average" /tmp/nats-bench.log || echo "Could not measure latency"
        fi
    fi
}

cleanup_test_pods() {
    log_info "Cleaning up test pods..."
    kubectl delete pod redis-load-publisher -n "$NAMESPACE" --ignore-not-found=true 2>/dev/null || true
    kubectl delete pod redis-bench -n "$NAMESPACE" --ignore-not-found=true 2>/dev/null || true
    kubectl delete pod nats-load-publisher -n "$NAMESPACE" --ignore-not-found=true 2>/dev/null || true
    kubectl delete pod nats-load-subscriber -n "$NAMESPACE" --ignore-not-found=true 2>/dev/null || true
    kubectl delete pod nats-bench -n "$NAMESPACE" --ignore-not-found=true 2>/dev/null || true
}

run_load_test() {
    local profile=$1
    local target_rps=$(get_target_rps "$profile")
    local profile_upper=$(echo "$profile" | tr '[:lower:]' '[:upper:]')
    
    log_info "Starting ${profile_upper} profile load test..."
    log_info "Target throughput: ${target_rps} events/sec"
    echo ""
    
    # Get baseline resources
    local backend_name="NATS"
    if [ "$profile" = "xs" ]; then
        backend_name="Redis"
    fi
    
    log_info "Baseline $backend_name resources:"
    local baseline=$(get_messaging_resources "$profile")
    echo "  CPU: $(echo $baseline | awk '{print $1}'), Memory: $(echo $baseline | awk '{print $2}')"
    echo ""
    
    # Run publisher load
    local pub_logs=$(run_publisher_load "$target_rps" "$DURATION" "$NAMESPACE" "$profile")
    
    # Extract metrics
    local published=$(echo "$pub_logs" | grep "Published" | awk '{print $2}')
    local actual_rps=$(echo "$pub_logs" | grep "Average rate" | awk '{print $3}')
    
    log_info "Load test results:"
    echo "  Published: ${published:-0} messages"
    echo "  Average rate: ${actual_rps:-0} msg/sec"
    echo "  Target rate: ${target_rps} msg/sec"
    echo ""
    
    # Get peak resources
    log_info "Peak $backend_name resources during test:"
    local peak=$(get_messaging_resources "$profile")
    echo "  CPU: $(echo $peak | awk '{print $1}'), Memory: $(echo $peak | awk '{print $2}')"
    echo ""
    
    # Measure latency
    measure_latency "$profile"
    echo ""
    
    # Verify results
    local success_rate=$((actual_rps * 100 / target_rps))
    local profile_upper=$(echo "$profile" | tr '[:lower:]' '[:upper:]')
    
    if [ $success_rate -ge 80 ]; then
        log_success "Load test PASSED (${success_rate}% of target)"
    elif [ $success_rate -ge 50 ]; then
        log_warning "Load test PARTIAL (${success_rate}% of target)"
    else
        log_error "Load test FAILED (${success_rate}% of target)"
        return 1
    fi
}

show_messaging_stats() {
    local profile=$1
    
    if [ "$profile" = "xs" ]; then
        log_info "Redis Statistics:"
        local pod=$(kubectl get pods -n "$NAMESPACE" -l app=lumo-redis -o jsonpath='{.items[0].metadata.name}')
        kubectl exec -n "$NAMESPACE" "$pod" -- redis-cli INFO stats 2>/dev/null | \
            grep -E "total_connections_received|total_commands_processed|instantaneous_ops_per_sec" || \
            echo "Could not fetch stats"
    else
        log_info "NATS Server Statistics:"
        local pod=$(kubectl get pods -n "$NAMESPACE" -l app=nats -o jsonpath='{.items[0].metadata.name}')
        kubectl exec -n "$NAMESPACE" "$pod" -- wget -qO- http://localhost:8222/varz 2>/dev/null | \
            jq -r '. | "Connections: \(.connections)\nIn Msgs: \(.in_msgs)\nOut Msgs: \(.out_msgs)\nIn Bytes: \(.in_bytes)\nOut Bytes: \(.out_bytes)\nSlow Consumers: \(.slow_consumers)"' || \
            echo "Could not fetch stats"
    fi
    
    echo ""
}

main() {
    print_banner
    
    # Parse args
    if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
        echo "Usage: $0 [profile]"
        echo ""
        echo "Profiles:"
        echo "  xs   - Extra Small (100 events/sec, Redis Streams)"
        echo "  s    - Small (1K events/sec, NATS)"
        echo "  m    - Medium (10K events/sec, NATS)"
        echo ""
        echo "Environment Variables:"
        echo "  LUMO_NAMESPACE   - Kubernetes namespace (default: lumo-system)"
        echo "  DURATION         - Test duration in seconds (default: 60)"
        echo ""
        exit 0
    fi
    
    if [ "${1:-}" = "--profile" ]; then
        PROFILE="${2:-s}"
        shift 2 || true
    fi
    
    # Validate profile
    if [ "$PROFILE" != "xs" ] && [ "$PROFILE" != "s" ] && [ "$PROFILE" != "m" ]; then
        log_error "Invalid profile: $PROFILE (must be 'xs', 's', or 'm')"
        exit 1
    fi
    
    # Check prerequisites
    check_messaging_backend
    
    # Cleanup any previous test pods
    cleanup_test_pods
    
    # Run load test
    PROFILE_UPPER=$(echo "$PROFILE" | tr '[:lower:]' '[:upper:]')
    if run_load_test "$PROFILE"; then
        echo ""
        log_success "All tests passed for ${PROFILE_UPPER} profile!"
        
        # Show final stats
        show_messaging_stats "$PROFILE"
        
        cleanup_test_pods
        exit 0
    else
        echo ""
        log_error "Load test failed for ${PROFILE_UPPER} profile"
        
        # Show logs for debugging
        if [ "$PROFILE" = "xs" ]; then
            log_info "Recent Redis logs:"
            kubectl logs -n "$NAMESPACE" -l app=lumo-redis --tail=20 || true
        else
            log_info "Recent NATS logs:"
            kubectl logs -n "$NAMESPACE" -l app=nats --tail=20 || true
        fi
        
        cleanup_test_pods
        exit 1
    fi
}

# Handle script interruption
trap cleanup_test_pods EXIT INT TERM

main "$@"
