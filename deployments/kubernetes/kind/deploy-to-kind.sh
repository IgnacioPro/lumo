#!/usr/bin/env bash
#
# Deploy Lumo Agent to kind cluster
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
NAMESPACE="${LUMO_NAMESPACE:-lumo-system}"
IMAGE_NAME="${LUMO_IMAGE_NAME:-lumo-agent}"
IMAGE_TAG="${LUMO_IMAGE_TAG:-local}"
FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

# API configuration (optional - for testing with actual API)
API_ENDPOINT="${LUMO_API_ENDPOINT:-http://lumo-api.lumo-system.svc.cluster.local:8080}"
AGENT_TOKEN="${LUMO_AGENT_TOKEN:-test-token-for-kind}"
AI_PROVIDER="${LUMO_AI_PROVIDER:-gemini}"
AI_API_KEY="${LUMO_AI_API_KEY:-dummy-key-for-testing}"

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

    # Check for kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed"
        exit 1
    fi

    # Check cluster connectivity
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi

    # Check if we're on the right cluster
    local current_context=$(kubectl config current-context)
    if [[ "$current_context" != "kind-${CLUSTER_NAME}" ]]; then
        log_warning "Current context is '${current_context}', expected 'kind-${CLUSTER_NAME}'"
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi

    log_success "Prerequisites satisfied"
}

create_namespace() {
    log_info "Creating namespace '${NAMESPACE}'..."

    kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

    log_success "Namespace ready"
}

create_secrets() {
    log_info "Creating secrets..."

    # Build secret data
    local secret_data="--from-literal=agent-token=${AGENT_TOKEN}"

    # Add AI API key if provided
    if [ -n "$AI_PROVIDER" ] && [ -n "$AI_API_KEY" ]; then
        case "$AI_PROVIDER" in
            anthropic)
                secret_data="${secret_data} --from-literal=anthropic-api-key=${AI_API_KEY}"
                ;;
            openai)
                secret_data="${secret_data} --from-literal=openai-api-key=${AI_API_KEY}"
                ;;
            gemini)
                secret_data="${secret_data} --from-literal=gemini-api-key=${AI_API_KEY}"
                ;;
            openrouter)
                secret_data="${secret_data} --from-literal=openrouter-api-key=${AI_API_KEY}"
                ;;
            *)
                log_warning "Unknown AI provider: ${AI_PROVIDER}"
                ;;
        esac
    fi

    # Delete existing secret to ensure update
    kubectl delete secret lumo-agent-secret --namespace="${NAMESPACE}" --ignore-not-found

    # Create secret
    kubectl create secret generic lumo-agent-secret \
        --namespace="${NAMESPACE}" \
        ${secret_data} \
        --dry-run=client -o yaml | kubectl apply -f -

    log_success "Secrets created"
}

update_manifests() {
    log_info "Preparing manifests for kind..."

    # Create temp directory for modified manifests
    # Create temp directory for modified manifests
    local temp_dir=$(mktemp -d)

    # Copy base manifests
    cp -r "$(dirname "$0")/../base/"*.yaml "${temp_dir}/"

    # Update image references in DaemonSet and Deployment
    for file in "${temp_dir}/daemonset.yaml" "${temp_dir}/deployment.yaml"; do
        if [ -f "$file" ]; then
            # Update image
            sed -i.bak "s|image:.*lumo-agent.*|image: ${FULL_IMAGE}|g" "$file"
            # Set imagePullPolicy to Never for kind
            sed -i.bak "s|imagePullPolicy:.*|imagePullPolicy: Never|g" "$file"
            # If no imagePullPolicy line exists, add it after image line
            if ! grep -q "imagePullPolicy:" "$file"; then
                sed -i.bak "/image: ${IMAGE_NAME}:${IMAGE_TAG}/a\        imagePullPolicy: Never" "$file"
            fi
            
            # Inject LUMO_AI_PROVIDER environment variable
            # Find the line with "# Secrets from Secret" and add the env var before it
            sed -i.bak "/# Secrets from Secret/i\\
            # AI Provider\\
            - name: LUMO_AI_PROVIDER\\
              value: \"${AI_PROVIDER}\"\\
" "$file"
            
            # Inject --config flag to tell agent to read from /etc/lumo/config.yaml
            # Find imagePullPolicy and add args section after it
            sed -i.bak "/imagePullPolicy:/a\\
          args:\\
            - --config=/etc/lumo/config.yaml\\
" "$file"
            
            rm -f "$file.bak"
        fi
    done

    # Update ConfigMap with API endpoint
    if [ -f "${temp_dir}/configmap.yaml" ]; then
        sed -i.bak "s|api_endpoint:.*|api_endpoint: \"${API_ENDPOINT}\"|g" "${temp_dir}/configmap.yaml"
        rm -f "${temp_dir}/configmap.yaml.bak"
    fi

    # Remove ServiceMonitor from service.yaml for kind (CRD not present)
    if [ -f "${temp_dir}/service.yaml" ]; then
        sed -i.bak '/# ServiceMonitor for Prometheus Operator/,$d' "${temp_dir}/service.yaml"
        rm -f "${temp_dir}/service.yaml.bak"
    fi

    # Update AI provider in ConfigMap
    if [ -f "${temp_dir}/configmap.yaml" ]; then
        # Update the provider field in the config.yaml section
        sed -i.bak "s|provider: \"\"|provider: \"${AI_PROVIDER}\"|g" "${temp_dir}/configmap.yaml"
        # Add cache_path to agent section (after offline_mode line)
        sed -i.bak "/offline_mode: true/a\\
      cache_path: /var/cache/lumo\\
" "${temp_dir}/configmap.yaml"
        rm -f "${temp_dir}/configmap.yaml.bak"
    fi

    echo "$temp_dir"
}

deploy_manifests() {
    log_info "Deploying Lumo Agent to kind cluster..."

    local manifest_dir=$(update_manifests)
    trap "rm -rf ${manifest_dir}" EXIT

    # Deploy in order
    local manifests=(
        "rbac.yaml"
        "configmap.yaml"
        "service.yaml"
        "daemonset.yaml"
        "deployment.yaml"
    )

    for manifest in "${manifests[@]}"; do
        local file="${manifest_dir}/${manifest}"
        if [ -f "$file" ]; then
            log_info "Applying ${manifest}..."
            kubectl apply -f "$file" -n "${NAMESPACE}"
        else
            log_warning "Manifest ${manifest} not found, skipping"
        fi
    done

    log_success "Manifests deployed"

    # Force restart to pick up new secrets/config
    log_info "Restarting pods to pick up configuration changes..."
    kubectl rollout restart daemonset/lumo-agent-node -n "${NAMESPACE}"
    kubectl rollout restart deployment/lumo-agent-cluster -n "${NAMESPACE}"
}

wait_for_pods() {
    log_info "Waiting for pods to be ready..."

    # Wait for DaemonSet
    log_info "Waiting for DaemonSet pods..."
    kubectl rollout status daemonset/lumo-agent-node -n "${NAMESPACE}" --timeout=300s || true

    # Wait for Deployment
    log_info "Waiting for Deployment pods..."
    kubectl rollout status deployment/lumo-agent-cluster -n "${NAMESPACE}" --timeout=300s || true

    echo ""
    log_info "Pod status:"
    kubectl get pods -n "${NAMESPACE}" -o wide
}

verify_deployment() {
    log_info "Verifying deployment..."

    # Check if pods are running
    local running_pods=$(kubectl get pods -n "${NAMESPACE}" -o json | jq -r '.items[] | select(.status.phase=="Running") | .metadata.name' | wc -l)
    local total_pods=$(kubectl get pods -n "${NAMESPACE}" -o json | jq -r '.items | length')

    echo ""
    log_info "Pods running: ${running_pods}/${total_pods}"

    if [ "$running_pods" -eq 0 ]; then
        log_warning "No pods are running yet. Check logs for issues."
        return 1
    fi

    # Show events
    echo ""
    log_info "Recent events:"
    kubectl get events -n "${NAMESPACE}" --sort-by='.lastTimestamp' | tail -10

    log_success "Deployment verification complete"
}

print_next_steps() {
    echo ""
    echo "=========================================="
    log_success "Lumo Agent deployed to kind!"
    echo "=========================================="
    echo ""
    echo "Namespace: ${NAMESPACE}"
    echo "Image: ${FULL_IMAGE}"
    echo ""
    echo "Useful commands:"
    echo ""
    echo "  View all pods:"
    echo "    ${BLUE}kubectl get pods -n ${NAMESPACE}${NC}"
    echo ""
    echo "  View DaemonSet logs:"
    echo "    ${BLUE}kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=node-monitor -f${NC}"
    echo ""
    echo "  View Deployment logs:"
    echo "    ${BLUE}kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=cluster-monitor -f${NC}"
    echo ""
    echo "  Check health endpoint:"
    echo "    ${BLUE}kubectl port-forward -n ${NAMESPACE} svc/lumo-agent-cluster 8080:8080${NC}"
    echo "    ${BLUE}curl http://localhost:8080/health${NC}"
    echo ""
    echo "  Check metrics:"
    echo "    ${BLUE}kubectl port-forward -n ${NAMESPACE} svc/lumo-agent-cluster 9090:9090${NC}"
    echo "    ${BLUE}curl http://localhost:9090/metrics${NC}"
    echo ""
    echo "  Exec into a pod:"
    echo "    ${BLUE}kubectl exec -it -n ${NAMESPACE} <pod-name> -- /bin/sh${NC}"
    echo ""
    echo "  Delete deployment:"
    echo "    ${BLUE}kubectl delete namespace ${NAMESPACE}${NC}"
    echo ""
}

main() {
    log_info "Deploying Lumo Agent to kind cluster '${CLUSTER_NAME}'"
    echo ""

    check_prerequisites
    create_namespace
    create_secrets
    deploy_manifests
    wait_for_pods
    verify_deployment
    print_next_steps
}

main "$@"
