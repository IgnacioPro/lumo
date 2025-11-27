#!/usr/bin/env bash
#
# Deploy Lumo with GCP Secret Manager
# Complete setup: ESO installation + Secret Store + External Secrets
#
# Prerequisites:
#   - kubectl configured for your cluster
#   - GCP credentials (Workload Identity or service account key)
#   - GCP_PROJECT_ID environment variable set
#
# Usage:
#   export GCP_PROJECT_ID="your-project-id"
#   ./deploy-with-gcp-secrets.sh
#
# Options:
#   --skip-eso        Skip External Secrets Operator installation
#   --skip-secrets    Skip GCP secret creation (use existing)
#   --workload-identity  Use Workload Identity (GKE only)
#   --sa-key FILE     Use service account key file

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAMESPACE="${LUMO_NAMESPACE:-lumo-system}"
ESO_NAMESPACE="external-secrets"
SKIP_ESO="${SKIP_ESO:-false}"
SKIP_SECRETS="${SKIP_SECRETS:-false}"
USE_WORKLOAD_IDENTITY="${USE_WORKLOAD_IDENTITY:-false}"
SA_KEY_FILE="${SA_KEY_FILE:-}"

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-eso)
                SKIP_ESO=true
                shift
                ;;
            --skip-secrets)
                SKIP_SECRETS=true
                shift
                ;;
            --workload-identity)
                USE_WORKLOAD_IDENTITY=true
                shift
                ;;
            --sa-key)
                SA_KEY_FILE="$2"
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

Deploy Lumo with GCP Secret Manager integration.

Options:
  --skip-eso           Skip External Secrets Operator installation
  --skip-secrets       Skip GCP secret creation (use existing)
  --workload-identity  Use Workload Identity authentication (GKE only)
  --sa-key FILE        Use service account key file for authentication
  -h, --help           Show this help message

Environment Variables:
  GCP_PROJECT_ID       GCP project ID (required)
  LUMO_NAMESPACE       Kubernetes namespace (default: lumo-system)
  GKE_CLUSTER_NAME     GKE cluster name (for Workload Identity)
  GKE_CLUSTER_LOCATION GKE cluster location (for Workload Identity)

Examples:
  # Using service account key
  export GCP_PROJECT_ID="my-project"
  $0 --sa-key ./gcp-sa-key.json

  # Using Workload Identity (GKE)
  export GCP_PROJECT_ID="my-project"
  export GKE_CLUSTER_NAME="my-cluster"
  export GKE_CLUSTER_LOCATION="us-central1"
  $0 --workload-identity
EOF
}

check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found"
        exit 1
    fi

    if ! command -v helm &> /dev/null; then
        log_error "helm not found. Install from: https://helm.sh/docs/intro/install/"
        exit 1
    fi

    if [ -z "${GCP_PROJECT_ID:-}" ]; then
        log_error "GCP_PROJECT_ID environment variable not set"
        exit 1
    fi

    # Check cluster connectivity
    if ! kubectl cluster-info &>/dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi

    log_success "Prerequisites OK"
}

install_external_secrets_operator() {
    if [ "$SKIP_ESO" = "true" ]; then
        log_info "Skipping ESO installation (--skip-eso)"
        return 0
    fi

    log_info "Installing External Secrets Operator..."

    # Add helm repo
    helm repo add external-secrets https://charts.external-secrets.io 2>/dev/null || true
    helm repo update

    # Check if already installed
    if helm status external-secrets -n "$ESO_NAMESPACE" &>/dev/null; then
        log_warn "External Secrets Operator already installed, upgrading..."
        helm upgrade external-secrets external-secrets/external-secrets \
            -n "$ESO_NAMESPACE" \
            --set installCRDs=true \
            --wait
    else
        # Install
        kubectl create namespace "$ESO_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
        helm install external-secrets external-secrets/external-secrets \
            -n "$ESO_NAMESPACE" \
            --set installCRDs=true \
            --wait
    fi

    log_success "External Secrets Operator installed"

    # Wait for CRDs
    log_info "Waiting for CRDs to be ready..."
    kubectl wait --for=condition=Established crd/externalsecrets.external-secrets.io --timeout=60s
    kubectl wait --for=condition=Established crd/clustersecretstores.external-secrets.io --timeout=60s
    log_success "CRDs ready"
}

setup_gcp_credentials() {
    log_info "Setting up GCP credentials..."

    if [ "$USE_WORKLOAD_IDENTITY" = "true" ]; then
        log_info "Using Workload Identity authentication"
        
        if [ -z "${GKE_CLUSTER_NAME:-}" ] || [ -z "${GKE_CLUSTER_LOCATION:-}" ]; then
            log_error "GKE_CLUSTER_NAME and GKE_CLUSTER_LOCATION required for Workload Identity"
            exit 1
        fi

        # Run Workload Identity setup
        "$SCRIPT_DIR/scripts/setup-workload-identity.sh"

    elif [ -n "$SA_KEY_FILE" ]; then
        log_info "Using service account key file: $SA_KEY_FILE"
        
        if [ ! -f "$SA_KEY_FILE" ]; then
            log_error "Service account key file not found: $SA_KEY_FILE"
            exit 1
        fi

        # Create K8s secret with the key
        kubectl create secret generic gcp-secret-manager-credentials \
            --from-file=secret-access-credentials="$SA_KEY_FILE" \
            -n "$ESO_NAMESPACE" \
            --dry-run=client -o yaml | kubectl apply -f -
        
        log_success "GCP credentials secret created"
    else
        log_error "No authentication method specified"
        echo "Use --workload-identity for GKE or --sa-key FILE for service account key"
        exit 1
    fi
}

create_gcp_secrets() {
    if [ "$SKIP_SECRETS" = "true" ]; then
        log_info "Skipping GCP secret creation (--skip-secrets)"
        return 0
    fi

    log_info "Creating secrets in GCP Secret Manager..."
    "$SCRIPT_DIR/scripts/create-gcp-secrets.sh"
}

deploy_secret_store() {
    log_info "Deploying ClusterSecretStore..."

    # Create namespace
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

    # Generate ClusterSecretStore with correct project ID
    local store_file="$SCRIPT_DIR/cluster-secret-store.yaml"
    
    if [ "$USE_WORKLOAD_IDENTITY" = "true" ]; then
        # Use Workload Identity configuration
        cat <<EOF | kubectl apply -f -
apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: gcp-secret-manager
spec:
  provider:
    gcpsm:
      projectID: ${GCP_PROJECT_ID}
      auth:
        workloadIdentity:
          clusterLocation: ${GKE_CLUSTER_LOCATION}
          clusterName: ${GKE_CLUSTER_NAME}
          serviceAccountRef:
            name: external-secrets
            namespace: ${ESO_NAMESPACE}
EOF
    else
        # Use service account key configuration
        cat <<EOF | kubectl apply -f -
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
    fi

    log_success "ClusterSecretStore deployed"

    # Wait for SecretStore to be ready
    log_info "Waiting for SecretStore to be ready..."
    local max_wait=60
    local elapsed=0
    
    while [ $elapsed -lt $max_wait ]; do
        local status=$(kubectl get clustersecretstore gcp-secret-manager -o jsonpath='{.status.conditions[0].status}' 2>/dev/null || echo "Unknown")
        
        if [ "$status" = "True" ]; then
            log_success "ClusterSecretStore is ready"
            return 0
        fi
        
        sleep 5
        elapsed=$((elapsed + 5))
        log_info "Waiting for SecretStore... (${elapsed}s/${max_wait}s)"
    done

    log_error "SecretStore not ready after ${max_wait}s"
    kubectl describe clustersecretstore gcp-secret-manager
    exit 1
}

deploy_external_secrets() {
    log_info "Deploying ExternalSecrets..."

    kubectl apply -f "$SCRIPT_DIR/external-secrets.yaml"

    log_success "ExternalSecrets deployed"

    # Wait for secrets to sync
    log_info "Waiting for secrets to sync..."
    sleep 10

    # Check status
    kubectl get externalsecrets -n "$NAMESPACE"
}

verify_secrets() {
    log_info "Verifying secrets were created..."

    local secrets=("lumo-api-secret" "lumo-ai-secrets" "lumo-agent-secret" "lumo-notification-secrets")
    local all_found=true

    for secret in "${secrets[@]}"; do
        if kubectl get secret "$secret" -n "$NAMESPACE" &>/dev/null; then
            log_success "✓ $secret exists"
        else
            log_warn "✗ $secret not found (may be optional)"
            # Don't fail - some secrets may be optional
        fi
    done

    # At minimum, API secret should exist
    if kubectl get secret lumo-api-secret -n "$NAMESPACE" &>/dev/null; then
        log_success "Core secrets verified"
    else
        log_error "Core secrets not created. Check ExternalSecret status:"
        kubectl describe externalsecret lumo-api-secrets -n "$NAMESPACE"
        exit 1
    fi
}

print_summary() {
    echo ""
    echo "========================================"
    log_success "GCP Secret Manager Integration Complete!"
    echo "========================================"
    echo ""
    echo "Project: ${GCP_PROJECT_ID}"
    echo "Namespace: ${NAMESPACE}"
    echo ""
    log_info "Secrets synced from GCP:"
    kubectl get secrets -n "$NAMESPACE" -l app.kubernetes.io/name=lumo
    echo ""
    log_info "Check ExternalSecret status:"
    echo "  kubectl get externalsecrets -n ${NAMESPACE}"
    echo ""
    log_info "View secret store status:"
    echo "  kubectl get clustersecretstore gcp-secret-manager"
    echo ""
    log_info "Now deploy Lumo:"
    echo "  kubectl apply -k ../base/"
    echo ""
}

main() {
    parse_args "$@"

    echo "========================================"
    echo "  Lumo + GCP Secret Manager Deployment"
    echo "========================================"
    echo ""

    check_prerequisites
    install_external_secrets_operator
    create_gcp_secrets
    setup_gcp_credentials
    deploy_secret_store
    deploy_external_secrets
    verify_secrets
    print_summary
}

main "$@"
