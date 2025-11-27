#!/usr/bin/env bash
#
# Create Lumo secrets in GCP Secret Manager
# Run this once to set up your secrets
#
# Prerequisites:
#   - gcloud CLI authenticated
#   - Secret Manager API enabled
#   - GCP_PROJECT_ID environment variable set
#
# Usage:
#   export GCP_PROJECT_ID="your-project-id"
#   ./create-gcp-secrets.sh
#
# To update a secret value:
#   echo -n "new-value" | gcloud secrets versions add SECRET_NAME --data-file=-

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

# Check prerequisites
check_prerequisites() {
    if ! command -v gcloud &> /dev/null; then
        log_error "gcloud CLI not found. Install from: https://cloud.google.com/sdk/docs/install"
        exit 1
    fi

    if [ -z "${GCP_PROJECT_ID:-}" ]; then
        log_error "GCP_PROJECT_ID environment variable not set"
        echo "Usage: export GCP_PROJECT_ID=\"your-project-id\" && $0"
        exit 1
    fi

    # Check if Secret Manager API is enabled
    if ! gcloud services list --enabled --project="$GCP_PROJECT_ID" 2>/dev/null | grep -q "secretmanager.googleapis.com"; then
        log_warn "Secret Manager API not enabled. Enabling now..."
        gcloud services enable secretmanager.googleapis.com --project="$GCP_PROJECT_ID"
        log_success "Secret Manager API enabled"
    fi
}

# Create or update a secret
create_secret() {
    local secret_name="$1"
    local description="$2"
    local prompt="$3"
    local default_value="${4:-}"

    echo ""
    log_info "Secret: ${secret_name}"
    echo "  Description: ${description}"

    # Check if secret already exists
    if gcloud secrets describe "$secret_name" --project="$GCP_PROJECT_ID" &>/dev/null; then
        log_warn "Secret '$secret_name' already exists"
        read -p "  Update with new value? (y/N): " update
        if [[ ! "$update" =~ ^[Yy]$ ]]; then
            log_info "Skipping '$secret_name'"
            return 0
        fi
    fi

    # Prompt for value
    if [ -n "$default_value" ]; then
        read -p "  ${prompt} [default: ${default_value:0:20}...]: " value
        value="${value:-$default_value}"
    else
        read -sp "  ${prompt}: " value
        echo ""
    fi

    if [ -z "$value" ]; then
        log_warn "Empty value provided, skipping '$secret_name'"
        return 0
    fi

    # Create or update secret
    if gcloud secrets describe "$secret_name" --project="$GCP_PROJECT_ID" &>/dev/null; then
        echo -n "$value" | gcloud secrets versions add "$secret_name" \
            --data-file=- \
            --project="$GCP_PROJECT_ID"
        log_success "Updated secret '$secret_name'"
    else
        echo -n "$value" | gcloud secrets create "$secret_name" \
            --data-file=- \
            --replication-policy="automatic" \
            --labels="app=lumo,managed-by=lumo-setup" \
            --project="$GCP_PROJECT_ID"
        log_success "Created secret '$secret_name'"
    fi
}

# Generate a random JWT secret
generate_jwt_secret() {
    openssl rand -base64 32 2>/dev/null || head -c 32 /dev/urandom | base64
}

# Main
main() {
    echo "========================================"
    echo "  GCP Secret Manager Setup for Lumo"
    echo "========================================"
    echo ""
    echo "Project: ${GCP_PROJECT_ID}"
    echo ""

    check_prerequisites

    log_info "This script will create the following secrets in GCP Secret Manager:"
    echo "  - lumo-jwt-secret (JWT signing key)"
    echo "  - lumo-database-password (PostgreSQL password)"
    echo "  - lumo-agent-token (Agent API token)"
    echo "  - lumo-anthropic-api-key (Anthropic AI API key)"
    echo "  - lumo-openai-api-key (OpenAI API key)"
    echo "  - lumo-slack-webhook-url (Slack notifications)"
    echo ""
    
    read -p "Continue? (Y/n): " confirm
    if [[ "$confirm" =~ ^[Nn]$ ]]; then
        log_info "Aborted"
        exit 0
    fi

    # Generate default JWT secret
    local default_jwt=$(generate_jwt_secret)

    # API Server secrets
    create_secret "lumo-jwt-secret" \
        "JWT signing key for API authentication" \
        "Enter JWT secret (or press Enter for random)" \
        "$default_jwt"

    create_secret "lumo-database-password" \
        "PostgreSQL database password" \
        "Enter database password"

    # Agent secrets
    create_secret "lumo-agent-token" \
        "API token for agent authentication" \
        "Enter agent token (or press Enter to generate)" \
        "$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p)"

    # AI Provider secrets
    create_secret "lumo-anthropic-api-key" \
        "Anthropic Claude API key (optional)" \
        "Enter Anthropic API key (sk-ant-...)"

    create_secret "lumo-openai-api-key" \
        "OpenAI API key (optional)" \
        "Enter OpenAI API key (sk-...)"

    # Notification secrets
    create_secret "lumo-slack-webhook-url" \
        "Slack webhook URL for notifications (optional)" \
        "Enter Slack webhook URL"

    echo ""
    echo "========================================"
    log_success "Secret setup complete!"
    echo "========================================"
    echo ""
    log_info "List your secrets:"
    echo "  gcloud secrets list --project=$GCP_PROJECT_ID --filter='labels.app=lumo'"
    echo ""
    log_info "View a secret value:"
    echo "  gcloud secrets versions access latest --secret=SECRET_NAME --project=$GCP_PROJECT_ID"
    echo ""
    log_info "Next steps:"
    echo "  1. Set up Workload Identity or create service account key"
    echo "  2. Install External Secrets Operator in your cluster"
    echo "  3. Apply cluster-secret-store.yaml and external-secrets.yaml"
    echo ""
}

main "$@"
