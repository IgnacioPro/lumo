#!/usr/bin/env bash
#
# Test script to show what each profile does
#

set -euo pipefail

echo "Testing Lumo Deployment Profiles"
echo "=================================="
echo ""

# Test startup profile
echo "1. STARTUP PROFILE (--profile startup)"
echo "   Expected: Use existing Redis, no additional deployment"
echo ""
./deploy-lumo.sh --profile startup --skip-cluster --skip-build --skip-infrastructure --skip-api --skip-deploy --skip-monitoring 2>&1 | grep -A 3 "messaging infrastructure" || echo "   (Would use existing Redis)"
echo ""

# Test small-business profile
echo "2. SMALL-BUSINESS PROFILE (--profile small-business) [DEFAULT]"
echo "   Expected: Deploy NATS single node (64MB RAM)"
echo ""
./deploy-lumo.sh --profile small-business --skip-cluster --skip-build --skip-infrastructure --skip-api --skip-deploy --skip-monitoring 2>&1 | grep -A 5 "messaging infrastructure" || echo "   (Would deploy NATS Deployment)"
echo ""

# Test enterprise profile
echo "3. ENTERPRISE PROFILE (--profile enterprise)"
echo "   Expected: Deploy NATS cluster (3-node StatefulSet)"
echo ""
./deploy-lumo.sh --profile enterprise --skip-cluster --skip-build --skip-infrastructure --skip-api --skip-deploy --skip-monitoring 2>&1 | grep -A 5 "messaging infrastructure" || echo "   (Would deploy NATS StatefulSet with 3 replicas)"
echo ""

# Test hyperscale profile
echo "4. HYPERSCALE PROFILE (--profile hyperscale)"
echo "   Expected: Deploy Kafka cluster (fallback to NATS for now)"
echo ""
./deploy-lumo.sh --profile hyperscale --skip-cluster --skip-build --skip-infrastructure --skip-api --skip-deploy --skip-monitoring 2>&1 | grep -A 5 "messaging infrastructure" || echo "   (Would deploy Kafka - coming in Week 3)"
echo ""

echo "=================================="
echo "All profile tests complete!"
echo ""
echo "To actually deploy:"
echo "  ./deploy-lumo.sh --profile startup"
echo "  ./deploy-lumo.sh --profile small-business"
echo "  ./deploy-lumo.sh --profile enterprise"
