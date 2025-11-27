#!/usr/bin/env bash
#
# Set up Workload Identity for External Secrets Operator on GKE
# This allows ESO to access GCP Secret Manager without storing keys
#
# Prerequisites:
#   - gcloud CLI authenticated
#   - GKE cluster with Workload Identity enabled
#   - External Secrets Operator installed
#
# Usage:
#   export GCP_PROJECT_ID="your-project-id"
#   export GKE_CLUSTER_NAME="your-cluster-name"
#   export GKE_CLUSTER_LOCATION="us-central1"  # or zone like us-central1-a
#   ./setup-workload-identity.sh

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
GCP_PROJECT_ID="${GCP_PROJECT_ID:-}"
GKE_CLUSTER_NAME="${GKE_CLUSTER_NAME:-}"
GKE_CLUSTER_LOCATION="${GKE_CLUSTER_LOCATION:-}"
GCP_SA_NAME="lumo-secrets-accessor"
K8S_NAMESPACE="external-secrets"
K8S_SA_NAME="external-secrets"

# Check prerequisites
check_prerequisites() {
    if ! command -v gcloud &> /dev/null; then
        log_error "gcloud CLI not found"
        exit 1
    fi

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found"
        exit 1
    fi

    if [ -z "$GCP_PROJECT_ID" ]; then
        log_error "GCP_PROJECT_ID not set"
        exit 1
    fi

    if [ -z "$GKE_CLUSTER_NAME" ]; then
        log_error "GKE_CLUSTER_NAME not set"
        exit 1
    fi

    if [ -z "$GKE_CLUSTER_LOCATION" ]; then
        log_error "GKE_CLUSTER_LOCATION not set"
        exit 1
    fi
}

main() {
    echo "========================================"
    echo "  Workload Identity Setup for ESO"
    echo "========================================"
    echo ""

    check_prerequisites

    log_info "Configuration:"
    echo "  GCP Project:    $GCP_PROJECT_ID"
    echo "  GKE Cluster:    $GKE_CLUSTER_NAME"
    echo "  Cluster Region: $GKE_CLUSTER_LOCATION"
    echo "  GCP SA:         $GCP_SA_NAME"
    echo "  K8s Namespace:  $K8S_NAMESPACE"
    echo "  K8s SA:         $K8S_SA_NAME"
    echo ""

    # Step 1: Create GCP Service Account
    log_info "Step 1/4: Creating GCP Service Account..."
    if gcloud iam service-accounts describe "${GCP_SA_NAME}@${GCP_PROJECT_ID}.iam.gserviceaccount.com" --project="$GCP_PROJECT_ID" &>/dev/null; then
        log_warn "Service account already exists, skipping creation"
    else
        gcloud iam service-accounts create "$GCP_SA_NAME" \
            --display-name="Lumo Secrets Accessor" \
            --description="Service account for External Secrets Operator to access GCP Secret Manager" \
            --project="$GCP_PROJECT_ID"
        log_success "Created service account: $GCP_SA_NAME"
    fi

    # Step 2: Grant Secret Manager access
    log_info "Step 2/4: Granting Secret Manager access..."
    gcloud projects add-iam-policy-binding "$GCP_PROJECT_ID" \
        --member="serviceAccount:${GCP_SA_NAME}@${GCP_PROJECT_ID}.iam.gserviceaccount.com" \
        --role="roles/secretmanager.secretAccessor" \
        --condition=None \
        --quiet
    log_success "Granted secretmanager.secretAccessor role"

    # Step 3: Bind to Kubernetes Service Account (Workload Identity)
    log_info "Step 3/4: Setting up Workload Identity binding..."
    gcloud iam service-accounts add-iam-policy-binding \
        "${GCP_SA_NAME}@${GCP_PROJECT_ID}.iam.gserviceaccount.com" \
        --role="roles/iam.workloadIdentityUser" \
        --member="serviceAccount:${GCP_PROJECT_ID}.svc.id.goog[${K8S_NAMESPACE}/${K8S_SA_NAME}]" \
        --project="$GCP_PROJECT_ID" \
        --quiet
    log_success "Created Workload Identity binding"

    # Step 4: Annotate Kubernetes Service Account
    log_info "Step 4/4: Annotating Kubernetes Service Account..."
    
    # Get cluster credentials
    gcloud container clusters get-credentials "$GKE_CLUSTER_NAME" \
        --location="$GKE_CLUSTER_LOCATION" \
        --project="$GCP_PROJECT_ID"

    # Annotate the service account
    kubectl annotate serviceaccount "$K8S_SA_NAME" \
        --namespace="$K8S_NAMESPACE" \
        "iam.gke.io/gcp-service-account=${GCP_SA_NAME}@${GCP_PROJECT_ID}.iam.gserviceaccount.com" \
        --overwrite
    log_success "Annotated K8s service account with Workload Identity"

    echo ""
    echo "========================================"
    log_success "Workload Identity setup complete!"
    echo "========================================"
    echo ""
    log_info "Update cluster-secret-store.yaml with:"
    echo ""
    echo "  spec:"
    echo "    provider:"
    echo "      gcpsm:"
    echo "        projectID: $GCP_PROJECT_ID"
    echo "        auth:"
    echo "          workloadIdentity:"
    echo "            clusterLocation: $GKE_CLUSTER_LOCATION"
    echo "            clusterName: $GKE_CLUSTER_NAME"
    echo "            serviceAccountRef:"
    echo "              name: $K8S_SA_NAME"
    echo "              namespace: $K8S_NAMESPACE"
    echo ""
    log_info "Then apply:"
    echo "  kubectl apply -f cluster-secret-store.yaml"
    echo "  kubectl apply -f external-secrets.yaml"
    echo ""
}

main "$@"
