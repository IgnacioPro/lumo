#!/usr/bin/env bash
#
# Lumo Agent Kubernetes Uninstallation Script
#
# This script removes Lumo Agent from a Kubernetes cluster.
#
# Usage:
#   ./uninstall.sh [OPTIONS]
#
# Options:
#   --namespace NAME    Namespace to uninstall from (default: lumo-system)
#   --delete-namespace  Delete the namespace after uninstalling
#   --force             Skip confirmation prompts
#   -h, --help          Show this help message
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
NAMESPACE="lumo-system"
DELETE_NAMESPACE=false
FORCE=false
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Helper functions
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
}

show_help() {
    grep '^#' "$0" | grep -v '#!/usr/bin/env' | sed 's/^# \?//'
    exit 0
}

confirm() {
    if [ "$FORCE" = true ]; then
        return 0
    fi

    read -p "$1 (yes/no): " -r
    if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
        return 1
    fi
    return 0
}

check_prerequisites() {
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl not found. Please install kubectl first."
        exit 1
    fi

    # Check cluster connection
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster. Please configure kubectl."
        exit 1
    fi

    # Check if namespace exists
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        print_error "Namespace $NAMESPACE does not exist"
        exit 1
    fi
}

show_current_state() {
    print_header "Current State"

    print_info "Resources in namespace $NAMESPACE:"
    echo ""

    kubectl get all -n "$NAMESPACE" -l app.kubernetes.io/name=lumo-agent 2>/dev/null || \
        print_warning "No Lumo Agent resources found"

    echo ""
}

uninstall_resources() {
    print_header "Uninstalling Resources"

    local base_dir="$SCRIPT_DIR/base"

    # Delete Deployment first (graceful shutdown)
    if kubectl get deployment lumo-agent-cluster -n "$NAMESPACE" &> /dev/null; then
        print_info "Deleting Deployment..."
        kubectl delete -f "$base_dir/deployment.yaml" --namespace="$NAMESPACE" --ignore-not-found=true
        print_success "Deployment deleted"
    fi

    # Delete DaemonSet
    if kubectl get daemonset lumo-agent-node -n "$NAMESPACE" &> /dev/null; then
        print_info "Deleting DaemonSet..."
        kubectl delete -f "$base_dir/daemonset.yaml" --namespace="$NAMESPACE" --ignore-not-found=true
        print_success "DaemonSet deleted"
    fi

    # Wait for pods to terminate
    print_info "Waiting for pods to terminate..."
    kubectl wait --for=delete pod -l app.kubernetes.io/name=lumo-agent \
        --namespace="$NAMESPACE" --timeout=60s 2>/dev/null || true

    # Delete Services
    print_info "Deleting Services..."
    kubectl delete -f "$base_dir/service.yaml" --namespace="$NAMESPACE" --ignore-not-found=true
    print_success "Services deleted"

    # Delete NetworkPolicy
    print_info "Deleting NetworkPolicy..."
    kubectl delete -f "$base_dir/networkpolicy.yaml" --namespace="$NAMESPACE" --ignore-not-found=true
    print_success "NetworkPolicy deleted"

    # Delete ConfigMap
    print_info "Deleting ConfigMap..."
    kubectl delete configmap lumo-agent-config --namespace="$NAMESPACE" --ignore-not-found=true
    print_success "ConfigMap deleted"

    # Delete Secret
    print_info "Deleting Secret..."
    kubectl delete secret lumo-agent-secret --namespace="$NAMESPACE" --ignore-not-found=true
    print_success "Secret deleted"

    # Delete RBAC (ClusterRole and ClusterRoleBinding)
    print_info "Deleting RBAC resources..."
    kubectl delete clusterrolebinding lumo-agent-reader-binding --ignore-not-found=true
    kubectl delete clusterrolebinding lumo-agent-remediator-binding --ignore-not-found=true
    kubectl delete clusterrole lumo-agent-reader --ignore-not-found=true
    kubectl delete clusterrole lumo-agent-remediator --ignore-not-found=true
    kubectl delete serviceaccount lumo-agent --namespace="$NAMESPACE" --ignore-not-found=true
    print_success "RBAC resources deleted"
}

delete_namespace() {
    if [ "$DELETE_NAMESPACE" = true ]; then
        print_header "Deleting Namespace"

        if confirm "Delete namespace $NAMESPACE?"; then
            kubectl delete namespace "$NAMESPACE"
            print_success "Namespace $NAMESPACE deleted"
        else
            print_info "Namespace $NAMESPACE preserved"
        fi
    fi
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --delete-namespace)
            DELETE_NAMESPACE=true
            shift
            ;;
        --force)
            FORCE=true
            shift
            ;;
        -h|--help)
            show_help
            ;;
        *)
            print_error "Unknown option: $1"
            show_help
            ;;
    esac
done

# Main execution
print_header "Lumo Agent Kubernetes Uninstallation"

check_prerequisites
show_current_state

if ! confirm "Proceed with uninstallation from namespace $NAMESPACE?"; then
    print_info "Uninstallation cancelled"
    exit 0
fi

uninstall_resources
delete_namespace

print_header "Uninstallation Complete"
print_success "Lumo Agent has been uninstalled from namespace: $NAMESPACE"

exit 0
