#!/usr/bin/env bash
#
# Lumo Agent Kubernetes Installation Script
#
# This script deploys Lumo Agent to a Kubernetes cluster with both
# DaemonSet (per-node) and Deployment (cluster-wide) components.
#
# Usage:
#   ./install.sh [OPTIONS]
#
# Options:
#   --namespace NAME         Namespace to deploy to (default: lumo-system)
#   --api-endpoint URL       Lumo API endpoint (required)
#   --agent-token TOKEN      Agent authentication token (required)
#   --ai-provider PROVIDER   AI provider: anthropic|openai|gemini|openrouter (default: anthropic)
#   --ai-api-key KEY         AI provider API key (optional)
#   --enable-remediation     Enable auto-remediation permissions (default: disabled)
#   --daemonset-only         Deploy only DaemonSet (node monitoring)
#   --deployment-only        Deploy only Deployment (cluster monitoring)
#   --skip-secret            Skip secret creation (use existing)
#   --dry-run                Show what would be deployed without applying
#   --uninstall              Uninstall Lumo Agent
#   -h, --help               Show this help message
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
API_ENDPOINT=""
AGENT_TOKEN=""
AI_PROVIDER="anthropic"
AI_API_KEY=""
ENABLE_REMEDIATION=false
DEPLOY_DAEMONSET=true
DEPLOY_DEPLOYMENT=true
SKIP_SECRET=false
DRY_RUN=false
UNINSTALL=false
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

check_prerequisites() {
    print_header "Checking Prerequisites"

    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl not found. Please install kubectl first."
        exit 1
    fi
    print_success "kubectl found: $(kubectl version --client --short 2>/dev/null || kubectl version --client)"

    # Check cluster connection
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster. Please configure kubectl."
        exit 1
    fi
    print_success "Connected to cluster: $(kubectl config current-context)"

    # Check required parameters
    if [ "$UNINSTALL" = false ]; then
        if [ -z "$API_ENDPOINT" ]; then
            print_error "API endpoint is required. Use --api-endpoint"
            exit 1
        fi

        if [ -z "$AGENT_TOKEN" ] && [ "$SKIP_SECRET" = false ]; then
            print_error "Agent token is required. Use --agent-token or --skip-secret"
            exit 1
        fi
    fi
}

create_namespace() {
    print_header "Creating Namespace"

    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        print_warning "Namespace $NAMESPACE already exists"
    else
        if [ "$DRY_RUN" = true ]; then
            print_info "Would create namespace: $NAMESPACE"
        else
            kubectl create namespace "$NAMESPACE"
            print_success "Namespace $NAMESPACE created"
        fi
    fi
}

create_secret() {
    if [ "$SKIP_SECRET" = true ]; then
        print_info "Skipping secret creation (--skip-secret)"
        return
    fi

    print_header "Creating Secret"

    local secret_args="--namespace=$NAMESPACE --from-literal=agent-token=$AGENT_TOKEN"

    # Add AI provider API key if provided
    if [ -n "$AI_API_KEY" ]; then
        case "$AI_PROVIDER" in
            anthropic)
                secret_args="$secret_args --from-literal=anthropic-api-key=$AI_API_KEY"
                ;;
            openai)
                secret_args="$secret_args --from-literal=openai-api-key=$AI_API_KEY"
                ;;
            gemini)
                secret_args="$secret_args --from-literal=gemini-api-key=$AI_API_KEY"
                ;;
            openrouter)
                secret_args="$secret_args --from-literal=openrouter-api-key=$AI_API_KEY"
                ;;
        esac
    fi

    if [ "$DRY_RUN" = true ]; then
        print_info "Would create secret: lumo-agent-secret"
        echo "  kubectl create secret generic lumo-agent-secret $secret_args"
    else
        # Delete existing secret if present
        kubectl delete secret lumo-agent-secret --namespace="$NAMESPACE" &> /dev/null || true

        # Create secret
        eval "kubectl create secret generic lumo-agent-secret $secret_args"
        print_success "Secret lumo-agent-secret created"
    fi
}

update_configmap() {
    print_header "Updating ConfigMap"

    local configmap="$SCRIPT_DIR/base/configmap.yaml"
    local temp_configmap=$(mktemp)

    # Update API endpoint in ConfigMap
    sed "s|agent.api-endpoint: \"https://lumo-api.example.com\"|agent.api-endpoint: \"$API_ENDPOINT\"|g" \
        "$configmap" > "$temp_configmap"

    if [ "$DRY_RUN" = true ]; then
        print_info "Would apply ConfigMap with API endpoint: $API_ENDPOINT"
        kubectl apply --dry-run=client -f "$temp_configmap" --namespace="$NAMESPACE"
    else
        kubectl apply -f "$temp_configmap" --namespace="$NAMESPACE"
        print_success "ConfigMap applied"
    fi

    rm -f "$temp_configmap"
}

apply_rbac() {
    print_header "Applying RBAC"

    local rbac="$SCRIPT_DIR/base/rbac.yaml"

    if [ "$ENABLE_REMEDIATION" = true ]; then
        print_warning "Remediation permissions ENABLED (write access to cluster resources)"
        local temp_rbac=$(mktemp)
        # Uncomment remediation ClusterRole and ClusterRoleBinding
        sed 's/^# apiVersion: rbac.authorization.k8s.io\/v1$/apiVersion: rbac.authorization.k8s.io\/v1/g' "$rbac" | \
        sed 's/^# kind: ClusterRole$/kind: ClusterRole/g' | \
        sed 's/^# metadata:$/metadata:/g' | \
        sed 's/^#   name:/  name:/g' | \
        sed 's/^#   labels:/  labels:/g' | \
        sed 's/^#     app/    app/g' | \
        sed 's/^# roleRef:/roleRef:/g' | \
        sed 's/^# subjects:/subjects:/g' | \
        sed 's/^#   - kind:/  - kind:/g' | \
        sed 's/^#     /    /g' > "$temp_rbac"
        rbac="$temp_rbac"
    fi

    if [ "$DRY_RUN" = true ]; then
        print_info "Would apply RBAC"
        kubectl apply --dry-run=client -f "$rbac" --namespace="$NAMESPACE"
    else
        kubectl apply -f "$rbac" --namespace="$NAMESPACE"
        print_success "RBAC applied"
    fi

    [ "$ENABLE_REMEDIATION" = true ] && rm -f "$temp_rbac"
}

deploy_components() {
    print_header "Deploying Components"

    local base_dir="$SCRIPT_DIR/base"

    # Deploy Service
    if [ "$DRY_RUN" = true ]; then
        print_info "Would apply Service"
        kubectl apply --dry-run=client -f "$base_dir/service.yaml" --namespace="$NAMESPACE"
    else
        kubectl apply -f "$base_dir/service.yaml" --namespace="$NAMESPACE"
        print_success "Service applied"
    fi

    # Deploy NetworkPolicy
    if [ "$DRY_RUN" = true ]; then
        print_info "Would apply NetworkPolicy"
        kubectl apply --dry-run=client -f "$base_dir/networkpolicy.yaml" --namespace="$NAMESPACE"
    else
        kubectl apply -f "$base_dir/networkpolicy.yaml" --namespace="$NAMESPACE"
        print_success "NetworkPolicy applied"
    fi

    # Deploy DaemonSet
    if [ "$DEPLOY_DAEMONSET" = true ]; then
        if [ "$DRY_RUN" = true ]; then
            print_info "Would apply DaemonSet (per-node monitoring)"
            kubectl apply --dry-run=client -f "$base_dir/daemonset.yaml" --namespace="$NAMESPACE"
        else
            kubectl apply -f "$base_dir/daemonset.yaml" --namespace="$NAMESPACE"
            print_success "DaemonSet applied (per-node monitoring)"
        fi
    fi

    # Deploy Deployment
    if [ "$DEPLOY_DEPLOYMENT" = true ]; then
        if [ "$DRY_RUN" = true ]; then
            print_info "Would apply Deployment (cluster-wide monitoring)"
            kubectl apply --dry-run=client -f "$base_dir/deployment.yaml" --namespace="$NAMESPACE"
        else
            kubectl apply -f "$base_dir/deployment.yaml" --namespace="$NAMESPACE"
            print_success "Deployment applied (cluster-wide monitoring)"
        fi
    fi
}

verify_deployment() {
    if [ "$DRY_RUN" = true ]; then
        print_info "Skipping verification in dry-run mode"
        return
    fi

    print_header "Verifying Deployment"

    print_info "Waiting for pods to be ready (timeout: 60s)..."

    # Wait for DaemonSet
    if [ "$DEPLOY_DAEMONSET" = true ]; then
        kubectl rollout status daemonset/lumo-agent-node --namespace="$NAMESPACE" --timeout=60s || \
            print_warning "DaemonSet rollout did not complete within timeout"
    fi

    # Wait for Deployment
    if [ "$DEPLOY_DEPLOYMENT" = true ]; then
        kubectl rollout status deployment/lumo-agent-cluster --namespace="$NAMESPACE" --timeout=60s || \
            print_warning "Deployment rollout did not complete within timeout"
    fi

    echo ""
    print_info "Current pod status:"
    kubectl get pods --namespace="$NAMESPACE" -l app.kubernetes.io/name=lumo-agent
}

show_status() {
    if [ "$DRY_RUN" = true ]; then
        return
    fi

    print_header "Installation Complete"

    print_success "Lumo Agent deployed to namespace: $NAMESPACE"

    echo ""
    echo "Deployed components:"
    [ "$DEPLOY_DAEMONSET" = true ] && echo "  ✓ DaemonSet (per-node monitoring)"
    [ "$DEPLOY_DEPLOYMENT" = true ] && echo "  ✓ Deployment (cluster-wide monitoring)"
    echo "  ✓ RBAC (ServiceAccount, ClusterRole, ClusterRoleBinding)"
    echo "  ✓ ConfigMap"
    echo "  ✓ Secret"
    echo "  ✓ Service"
    echo "  ✓ NetworkPolicy"

    echo ""
    echo "Next steps:"
    echo ""
    echo "  # Check pod status"
    echo "  kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=lumo-agent"
    echo ""
    echo "  # View logs (DaemonSet)"
    echo "  kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=node-monitor --tail=50"
    echo ""
    echo "  # View logs (Deployment)"
    echo "  kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=cluster-monitor --tail=50"
    echo ""
    echo "  # Access health endpoint"
    echo "  kubectl port-forward -n $NAMESPACE service/lumo-agent-cluster 8080:8080"
    echo "  curl http://localhost:8080/health"
    echo ""
    echo "  # Access metrics"
    echo "  kubectl port-forward -n $NAMESPACE service/lumo-agent-cluster 9090:9090"
    echo "  curl http://localhost:9090/metrics"
    echo ""
}

uninstall_lumo() {
    print_header "Uninstalling Lumo Agent"

    print_warning "This will delete all Lumo Agent resources from namespace: $NAMESPACE"

    if [ "$DRY_RUN" = true ]; then
        print_info "Would delete resources"
        return
    fi

    read -p "Are you sure? (yes/no): " -r
    if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
        print_info "Uninstall cancelled"
        exit 0
    fi

    local base_dir="$SCRIPT_DIR/base"

    # Delete in reverse order
    kubectl delete -f "$base_dir/deployment.yaml" --namespace="$NAMESPACE" --ignore-not-found=true
    kubectl delete -f "$base_dir/daemonset.yaml" --namespace="$NAMESPACE" --ignore-not-found=true
    kubectl delete -f "$base_dir/networkpolicy.yaml" --namespace="$NAMESPACE" --ignore-not-found=true
    kubectl delete -f "$base_dir/service.yaml" --namespace="$NAMESPACE" --ignore-not-found=true
    kubectl delete -f "$base_dir/rbac.yaml" --ignore-not-found=true
    kubectl delete -f "$base_dir/configmap.yaml" --namespace="$NAMESPACE" --ignore-not-found=true
    kubectl delete -f "$base_dir/secret.yaml" --namespace="$NAMESPACE" --ignore-not-found=true

    print_success "Lumo Agent uninstalled"

    read -p "Delete namespace $NAMESPACE? (yes/no): " -r
    if [[ $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
        kubectl delete namespace "$NAMESPACE"
        print_success "Namespace $NAMESPACE deleted"
    fi
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --api-endpoint)
            API_ENDPOINT="$2"
            shift 2
            ;;
        --agent-token)
            AGENT_TOKEN="$2"
            shift 2
            ;;
        --ai-provider)
            AI_PROVIDER="$2"
            shift 2
            ;;
        --ai-api-key)
            AI_API_KEY="$2"
            shift 2
            ;;
        --enable-remediation)
            ENABLE_REMEDIATION=true
            shift
            ;;
        --daemonset-only)
            DEPLOY_DAEMONSET=true
            DEPLOY_DEPLOYMENT=false
            shift
            ;;
        --deployment-only)
            DEPLOY_DAEMONSET=false
            DEPLOY_DEPLOYMENT=true
            shift
            ;;
        --skip-secret)
            SKIP_SECRET=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --uninstall)
            UNINSTALL=true
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
print_header "Lumo Agent Kubernetes Installation"

check_prerequisites

if [ "$UNINSTALL" = true ]; then
    uninstall_lumo
    exit 0
fi

create_namespace
create_secret
apply_rbac
update_configmap
deploy_components
verify_deployment
show_status

exit 0
