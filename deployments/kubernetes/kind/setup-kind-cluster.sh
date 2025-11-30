#!/usr/bin/env bash
#
# Setup kind cluster for testing Lumo Agent
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
KIND_VERSION="${KIND_VERSION:-v0.20.0}"
K8S_VERSION="${K8S_VERSION:-v1.33.0}"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check for Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        exit 1
    fi

    # Check if Docker daemon is running
    if ! docker info &> /dev/null; then
        log_error "Docker daemon is not running. Please start Docker."
        exit 1
    fi

    # Check for kind
    if ! command -v kind &> /dev/null; then
        log_warning "kind is not installed. Installing kind..."
        install_kind
    fi

    # Check for kubectl
    if ! command -v kubectl &> /dev/null; then
        log_warning "kubectl is not installed. Installing kubectl..."
        install_kubectl
    fi

    log_success "All prerequisites satisfied"
}

install_kind() {
    log_info "Installing kind ${KIND_VERSION}..."

    # Detect OS and architecture
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) log_error "Unsupported architecture: $ARCH"; exit 1 ;;
    esac

    # Download kind
    curl -Lo ./kind "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${OS}-${ARCH}"
    chmod +x ./kind
    sudo mv ./kind /usr/local/bin/kind

    log_success "kind installed successfully"
}

install_kubectl() {
    log_info "Installing kubectl..."

    # Detect OS and architecture
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
    esac

    # Download kubectl
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/${OS}/${ARCH}/kubectl"
    chmod +x kubectl
    sudo mv kubectl /usr/local/bin/kubectl

    log_success "kubectl installed successfully"
}

create_kind_config() {
    log_info "Creating kind cluster configuration..."

    cat > /tmp/kind-config.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${CLUSTER_NAME}
nodes:
  # Control plane node
  - role: control-plane
    image: kindest/node:${K8S_VERSION}
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "lumo.io/monitor=true"

  # Worker nodes
  - role: worker
    image: kindest/node:${K8S_VERSION}
    kubeadmConfigPatches:
      - |
        kind: JoinConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "lumo.io/monitor=true"

  - role: worker
    image: kindest/node:${K8S_VERSION}
    kubeadmConfigPatches:
      - |
        kind: JoinConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "lumo.io/monitor=true"

# Networking configuration
networking:
  apiServerAddress: "127.0.0.1"
  apiServerPort: 6443
  podSubnet: "10.244.0.0/16"
  serviceSubnet: "10.96.0.0/12"

# Port mappings (optional - for accessing services)
# containerdConfigPatches:
#   - |-
#     [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5000"]
#       endpoint = ["http://kind-registry:5000"]
EOF

    log_success "Kind config created at /tmp/kind-config.yaml"
}

create_cluster() {
    log_info "Creating kind cluster '${CLUSTER_NAME}'..."

    # Check if cluster already exists
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_warning "Cluster '${CLUSTER_NAME}' already exists"
        read -p "Do you want to delete and recreate it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Deleting existing cluster..."
            kind delete cluster --name "${CLUSTER_NAME}"
        else
            log_info "Using existing cluster"
            return 0
        fi
    fi

    # Create cluster
    kind create cluster --config /tmp/kind-config.yaml --wait 2m

    log_success "Cluster '${CLUSTER_NAME}' created successfully"
}

verify_cluster() {
    log_info "Verifying cluster status..."

    # Wait for nodes to be ready
    log_info "Waiting for nodes to be ready..."
    kubectl wait --for=condition=Ready nodes --all --timeout=300s

    # Show cluster info
    echo ""
    log_info "Cluster information:"
    kubectl cluster-info
    echo ""

    log_info "Nodes:"
    kubectl get nodes -o wide
    echo ""

    log_success "Cluster is ready!"
}

print_next_steps() {
    echo ""
    echo "=========================================="
    log_success "Kind cluster setup complete!"
    echo "=========================================="
    echo ""
    echo "Cluster Name: ${CLUSTER_NAME}"
    echo "Kubernetes Version: ${K8S_VERSION}"
    echo ""
    echo "Next steps:"
    echo "  1. Build the Lumo agent image:"
    echo "     ${BLUE}./build-and-load.sh${NC}"
    echo ""
    echo "  2. Deploy the agent:"
    echo "     ${BLUE}./deploy-to-kind.sh${NC}"
    echo ""
    echo "  3. Or do both in one step:"
    echo "     ${BLUE}./deploy-lumo.sh${NC}"
    echo ""
    echo "Useful commands:"
    echo "  - Get pods: ${BLUE}kubectl get pods -n lumo-system${NC}"
    echo "  - View logs: ${BLUE}kubectl logs -n lumo-system -l app.kubernetes.io/name=lumo-agent -f${NC}"
    echo "  - Delete cluster: ${BLUE}kind delete cluster --name ${CLUSTER_NAME}${NC}"
    echo ""
}

main() {
    log_info "Starting kind cluster setup for Lumo Agent testing"
    echo ""

    check_prerequisites
    create_kind_config
    create_cluster
    verify_cluster
    print_next_steps
}

main "$@"
