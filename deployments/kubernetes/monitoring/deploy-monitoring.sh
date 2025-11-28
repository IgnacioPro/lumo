#!/bin/bash
# Deploy Grafana with Prometheus for Lumo observability
# Usage: ./deploy-monitoring.sh [--with-prometheus]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAMESPACE="monitoring"
WITH_PROMETHEUS=false

# Pin chart versions to avoid warnings and ensure reproducible deployments
PROMETHEUS_CHART_VERSION="27.46.0"
GRAFANA_CHART_VERSION="10.2.0"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --with-prometheus)
            WITH_PROMETHEUS=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "=== Lumo Monitoring Stack Deployment ==="

# Create namespace
echo "Creating namespace..."
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# Add Helm repos
echo "Adding Helm repositories..."
helm repo add grafana https://grafana.github.io/helm-charts 2>/dev/null || true
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
helm repo update

# Deploy Prometheus if requested
if [ "$WITH_PROMETHEUS" = true ]; then
    echo "Deploying Prometheus (chart version ${PROMETHEUS_CHART_VERSION})..."
    helm upgrade --install prometheus prometheus-community/prometheus \
        --namespace "${NAMESPACE}" \
        --version "${PROMETHEUS_CHART_VERSION}" \
        --set alertmanager.enabled=false \
        --set prometheus-pushgateway.enabled=false \
        --set server.persistentVolume.size=2Gi \
        --wait --timeout 5m
fi

# Deploy dashboard ConfigMap
echo "Deploying Lumo dashboards..."
kubectl apply -f "${SCRIPT_DIR}/grafana/dashboard-configmap.yaml"

# Deploy Grafana
echo "Deploying Grafana (chart version ${GRAFANA_CHART_VERSION})..."
helm upgrade --install grafana grafana/grafana \
    --namespace "${NAMESPACE}" \
    --version "${GRAFANA_CHART_VERSION}" \
    -f "${SCRIPT_DIR}/grafana/values.yaml" \
    --set adminPassword=admin \
    --wait --timeout 5m

# Get Grafana admin password
echo ""
echo "=== Deployment Complete ==="
echo ""
echo "Components deployed:"
echo "  ✓ Grafana (with Lumo dashboards)"
if [ "$WITH_PROMETHEUS" = true ]; then
    echo "  ✓ Prometheus"
fi
echo ""
echo "Grafana admin password: admin"
echo ""
echo "Access Grafana:"
echo "  kubectl port-forward -n ${NAMESPACE} svc/grafana 3000:80"
echo "  Then open: http://localhost:3000"
echo ""

if [ "$WITH_PROMETHEUS" = true ]; then
    echo "Access Prometheus:"
    echo "  kubectl port-forward -n ${NAMESPACE} svc/prometheus-server 9090:80"
    echo "  Then open: http://localhost:9090"
    echo ""
fi

echo "Redis Monitoring:"
echo "  Redis is deployed in lumo-system namespace (via deploy-lumo.sh)"
echo "  Prometheus auto-discovers Redis metrics via pod annotations"
echo "  Access Redis: kubectl port-forward -n lumo-system svc/lumo-redis 6379:6379"
echo "  Redis metrics: kubectl port-forward -n lumo-system svc/lumo-redis 9121:9121"
echo "  Then: curl http://localhost:9121/metrics"
echo ""
echo "Lumo Dashboard: Navigate to Dashboards > Lumo > Lumo Overview"
echo "  - Scroll to 'Redis Metrics' section for comprehensive Redis monitoring"
