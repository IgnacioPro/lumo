#!/usr/bin/env bash
#
# End-to-end workflow test: Agent -> Central API
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

CLUSTER_NAME="${KIND_CLUSTER_NAME:-lumo-test}"
NAMESPACE="${LUMO_NAMESPACE:-lumo-system}"

log_info() { echo -e "${BLUE}[INFO]${NC} $1" >&2; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1" >&2; }
log_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }

# Change to script directory
cd "$(dirname "$0")"

# 1. Setup Cluster
log_info "Step 1: Setting up cluster..."
./setup-kind-cluster.sh

# 2. Build and Load Images
log_info "Step 2: Building and loading images..."
./build-and-load.sh

# Create namespace
log_info "Creating namespace ${NAMESPACE}..."
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# 3. Deploy Postgres
log_info "Step 3: Deploying Postgres..."
kubectl apply -f manifests/postgres.yaml
kubectl rollout status deployment/postgres -n "${NAMESPACE}" --timeout=120s

# 4. Deploy API Server
log_info "Step 4: Deploying API Server..."
kubectl apply -f manifests/api-server.yaml
kubectl rollout status deployment/lumo-api -n "${NAMESPACE}" --timeout=120s

# 5. Deploy Agent
log_info "Step 5: Deploying Agent..."
export LUMO_API_ENDPOINT="http://lumo-api.${NAMESPACE}.svc.cluster.local:80"
export SKIP_CLUSTER_SETUP=true
export SKIP_BUILD=true
./deploy-to-kind.sh

# 6. Verify Workflow
log_info "Step 6: Verifying Workflow..."

# Wait a bit for agent to register
sleep 10

# Port forward API server to check agents
kubectl port-forward -n "${NAMESPACE}" svc/lumo-api 8081:80 >/dev/null 2>&1 &
PF_PID=$!
trap "kill $PF_PID" EXIT
sleep 2

# Query agents
log_info "Querying registered agents..."
RESPONSE=$(curl -s http://localhost:8081/api/v1/agents)
echo "Response: $RESPONSE"

# Simple check if response contains agents (this is a weak check, improve later)
# Assuming the API returns a JSON list or similar.
# If auth is required, we might need a token.
# The agent uses a token, but does the API require one to list agents?
# Let's assume for now we might need to authenticate or just check logs.

# Check API logs for registration
log_info "Checking API logs for agent registration..."
if kubectl logs -n "${NAMESPACE}" -l app=lumo-api | grep -q "Registering agent"; then
    log_success "Agent registration log found in API server"
else
    log_error "Agent registration log NOT found in API server"
    kubectl logs -n "${NAMESPACE}" -l app=lumo-api
    exit 1
fi

log_success "Workflow test passed!"
