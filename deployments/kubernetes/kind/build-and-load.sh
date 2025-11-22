#!/usr/bin/env bash
#
# Build Lumo Agent Docker image and load it into kind
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="${KIND_CLUSTER_NAME:-lumo-test}"
IMAGE_NAME="${LUMO_IMAGE_NAME:-lumo-agent}"
IMAGE_TAG="${LUMO_IMAGE_TAG:-local}"
FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check for Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi

    # Check for kind
    if ! command -v kind &> /dev/null; then
        log_error "kind is not installed. Run ./setup-kind-cluster.sh first"
        exit 1
    fi

    # Check if cluster exists
    if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_error "Kind cluster '${CLUSTER_NAME}' does not exist. Run ./setup-kind-cluster.sh first"
        exit 1
    fi

    log_success "Prerequisites satisfied"
}

build_image() {
    log_info "Building Docker image: ${FULL_IMAGE}"
    echo ""

    # Change to project root (3 levels up from kind/)
    cd "$(dirname "$0")/../../.."

    # Build image with Dockerfile.agent from root
    docker build \
        -f Dockerfile.agent \
        -t "${FULL_IMAGE}" \
        --progress=plain \
        .

    echo ""
    log_success "Image built successfully"

    # Show image details
    log_info "Image details:"
    docker images "${IMAGE_NAME}" | head -2
}

load_image() {
    log_info "Loading image into kind cluster '${CLUSTER_NAME}'..."

    kind load docker-image "${FULL_IMAGE}" --name "${CLUSTER_NAME}"

    log_success "Image loaded into kind cluster"
}

verify_image() {
    log_info "Verifying image in kind cluster..."

    # Check if image is available in kind nodes
    local node_image=$(docker exec "${CLUSTER_NAME}-control-plane" crictl images | grep "${IMAGE_NAME}" || true)

    if [ -n "$node_image" ]; then
        log_success "Image is available in cluster nodes"
        echo "$node_image"
    else
        log_error "Image not found in cluster nodes"
        exit 1
    fi
}

print_next_steps() {
    echo ""
    echo "=========================================="
    log_success "Image build and load complete!"
    echo "=========================================="
    echo ""
    echo "Image: ${FULL_IMAGE}"
    echo "Cluster: ${CLUSTER_NAME}"
    echo ""
    echo "Next steps:"
    echo "  1. Deploy the agent:"
    echo "     ${BLUE}./deploy-to-kind.sh${NC}"
    echo ""
    echo "  2. Or update existing deployment:"
    echo "     ${BLUE}kubectl rollout restart daemonset/lumo-agent-node -n lumo-system${NC}"
    echo "     ${BLUE}kubectl rollout restart deployment/lumo-agent-cluster -n lumo-system${NC}"
    echo ""
}

main() {
    log_info "Building and loading Lumo Agent image for kind"
    echo ""

    check_prerequisites
    build_image
    load_image
    verify_image
    print_next_steps
}

main "$@"
