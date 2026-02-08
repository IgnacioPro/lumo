#!/usr/bin/env bash
# create-tenant-issue-pods.sh - Script to create issue pods in tenant namespaces for Lumo monitoring
# 
# This script creates various types of problematic pods in tenant namespaces that can be
# monitored and detected by the Lumo system. It's designed to help test and validate
# the multi-tenant monitoring capabilities.

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Default values
ISSUE_TYPES=("crash-loop" "oom-kill" "pending" "image-pull-backoff")
DEFAULT_ISSUE_TYPE="crash-loop"

# Function to display usage
usage() {
    echo -e "${BOLD}Usage:${NC} $0 [OPTIONS] [TENANT_NAMESPACES...]"
    echo ""
    echo -e "${BOLD}OPTIONS:${NC}"
    echo "  -t, --type TYPE      Issue type to create (crash-loop, oom-kill, pending, image-pull-backoff)"
    echo "  -n, --number NUM     Number of issue pods to create per tenant (default: 1)"
    echo "  -p, --prefix PREFIX  Prefix for pod names (default: issue-pod)"
    echo "  -h, --help          Show this help message"
    echo ""
    echo -e "${BOLD}EXAMPLES:${NC}"
    echo "  $0 tenant-acme-corp"
    echo "  $0 -t oom-kill -n 2 tenant-acme-corp tenant-globex"
    echo "  $0 --type crash-loop tenant-*"
    echo ""
    echo -e "${BOLD}NOTE:${NC}"
    echo "  Only namespaces starting with 'tenant-' are allowed"
    echo ""
    echo -e "${BOLD}SUPPORTED ISSUE TYPES:${NC}"
    for type in "${ISSUE_TYPES[@]}"; do
        echo "  - ${type}"
    done
}

# Function to validate kubectl is available
check_kubectl() {
    if ! command -v kubectl &> /dev/null; then
        echo -e "${RED}✗ kubectl is not installed or not in PATH${NC}" >&2
        exit 1
    fi
}

# Function to check if a namespace exists
namespace_exists() {
    local namespace="$1"
    kubectl get namespace "$namespace" &> /dev/null
}

# Function to create crash loop pod
create_crash_loop_pod() {
    local namespace="$1"
    local pod_name="$2"

    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
  namespace: ${namespace}
  labels:
    critical: "true"
spec:
  containers:
  - name: crash-loop-container
    image: busybox
    command: ["/bin/sh", "-c", "exit 1"]
  restartPolicy: Always
EOF
}

# Function to create OOM kill pod
create_oom_kill_pod() {
    local namespace="$1"
    local pod_name="$2"

    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
  namespace: ${namespace}
  labels:
    critical: "true"
spec:
  containers:
  - name: oom-container
    image: polinux/stress
    resources:
      limits:
        memory: "128Mi"
        cpu: "500m"
      requests:
        memory: "128Mi"
        cpu: "250m"
    command: ["stress"]
    args: ["--vm", "1", "--vm-bytes", "200M", "--vm-hang", "1"]
EOF
}

# Function to create pending pod
create_pending_pod() {
    local namespace="$1"
    local pod_name="$2"

    # Create a pod that requires a non-existent node selector to make it stay pending
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
  namespace: ${namespace}
  labels:
    critical: "true"
spec:
  containers:
  - name: pending-container
    image: nginx
    resources:
      requests:
        cpu: "999"  # Impossible CPU request to make it pending
  tolerations:
  - key: "node"
    operator: "Equal"
    value: "nonexistent"
    effect: "NoSchedule"
EOF
}

# Function to create image pull backoff pod
create_image_pull_backoff_pod() {
    local namespace="$1"
    local pod_name="$2"

    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
  namespace: ${namespace}
  labels:
    critical: "true"
spec:
  containers:
  - name: invalid-image-container
    image: nonexistent-registry.invalid-domain/example/nonexistent:latest
EOF
}

# Function to create issue pod based on type
create_issue_pod() {
    local namespace="$1"
    local issue_type="$2"
    local pod_name="$3"
    
    echo -e "${BLUE}Creating ${issue_type} pod: ${pod_name} in namespace: ${namespace}${NC}"
    
    case "$issue_type" in
        "crash-loop")
            create_crash_loop_pod "$namespace" "$pod_name"
            ;;
        "oom-kill")
            create_oom_kill_pod "$namespace" "$pod_name"
            ;;
        "pending")
            create_pending_pod "$namespace" "$pod_name"
            ;;
        "image-pull-backoff")
            create_image_pull_backoff_pod "$namespace" "$pod_name"
            ;;
        *)
            echo -e "${RED}✗ Unsupported issue type: ${issue_type}${NC}" >&2
            return 1
            ;;
    esac
    
    echo -e "${GREEN}✓ Created ${issue_type} pod: ${pod_name}${NC}"
}

# Parse command line arguments
ISSUE_TYPE="$DEFAULT_ISSUE_TYPE"
POD_COUNT=1
POD_PREFIX="issue-pod"

while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--type)
            ISSUE_TYPE="$2"
            shift 2
            ;;
        -n|--number)
            POD_COUNT="$2"
            shift 2
            ;;
        -p|--prefix)
            POD_PREFIX="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -*)
            echo -e "${RED}Unknown option: $1${NC}" >&2
            usage
            exit 1
            ;;
        *)
            TENANT_NAMESPACES+=("$1")
            shift
            ;;
    esac
done

# Validate arguments
if [[ ${#TENANT_NAMESPACES[@]} -eq 0 ]]; then
    echo -e "${RED}✗ No tenant namespaces provided. Only namespaces starting with 'tenant-' are allowed.${NC}" >&2
    usage
    exit 1
fi

if [[ ! " ${ISSUE_TYPES[*]} " =~ " ${ISSUE_TYPE} " ]]; then
    echo -e "${RED}✗ Invalid issue type: ${ISSUE_TYPE}${NC}" >&2
    echo -e "${YELLOW}Supported types: ${ISSUE_TYPES[*]}${NC}" >&2
    exit 1
fi

if ! [[ "$POD_COUNT" =~ ^[0-9]+$ ]] || [ "$POD_COUNT" -le 0 ]; then
    echo -e "${RED}✗ Invalid pod count: ${POD_COUNT}${NC}" >&2
    exit 1
fi

# Validate kubectl is available
check_kubectl

# Banner
echo -e "${BLUE}${BOLD}"
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║              CREATE TENANT ISSUE PODS                        ║"
echo "║        Lumo Multi-Tenant Testing Tool (tenant-* only)        ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

echo -e "${BLUE}Settings:${NC}"
echo "  Issue Type: $ISSUE_TYPE"
echo "  Pod Count per Tenant: $POD_COUNT"
echo "  Pod Prefix: $POD_PREFIX"
echo "  Target Tenants: ${TENANT_NAMESPACES[*]}"
echo ""

# Process each tenant namespace
for tenant_namespace in "${TENANT_NAMESPACES[@]}"; do
    # Check if namespace starts with "tenant-"
    if [[ ! "$tenant_namespace" =~ ^tenant- ]]; then
        echo -e "${YELLOW}⚠️  Skipping non-customer namespace: ${tenant_namespace} (does not start with 'tenant-')${NC}"
        continue
    fi

    echo -e "${BLUE}Processing tenant: ${tenant_namespace}${NC}"

    # Check if namespace exists
    if ! namespace_exists "$tenant_namespace"; then
        echo -e "${YELLOW}⚠️  Namespace ${tenant_namespace} does not exist, creating it...${NC}"
        kubectl create namespace "$tenant_namespace"
        echo -e "${GREEN}✓ Created namespace: ${tenant_namespace}${NC}"
    fi

    # Create issue pods in the namespace
    for i in $(seq 1 $POD_COUNT); do
        pod_name="${POD_PREFIX}-${tenant_namespace}-${ISSUE_TYPE}-${i}"
        create_issue_pod "$tenant_namespace" "$ISSUE_TYPE" "$pod_name"
    done

    echo -e "${GREEN}✓ Completed processing for tenant: ${tenant_namespace}${NC}"
    echo ""
done

echo -e "${GREEN}${BOLD}✅ All issue pods created successfully!${NC}"
echo ""
echo -e "${BLUE}Summary:${NC}"
echo "  Issue Type: $ISSUE_TYPE"
echo "  Total Pods Created: $((${#TENANT_NAMESPACES[@]} * POD_COUNT))"
echo "  Target Tenants: ${#TENANT_NAMESPACES[@]}"
echo ""
echo -e "${BLUE}To verify pods:${NC}"
for tenant_namespace in "${TENANT_NAMESPACES[@]}"; do
    echo "  kubectl get pods -n $tenant_namespace | grep $POD_PREFIX"
done