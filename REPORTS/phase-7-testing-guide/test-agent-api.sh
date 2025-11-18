#!/bin/bash

# Test Agent Registration API
# Usage: ./test-agent-api.sh

set -e

API_KEY="test-key-123"
BASE_URL="http://localhost:8080/api/v1"

echo "=== Testing Agent Registration API ==="
echo

# 1. Health check
echo "1. Testing health endpoint..."
curl -s "$BASE_URL/health" | jq .
echo -e "\n"

# 2. Register an agent
echo "2. Registering agent-01..."
AGENT_RESPONSE=$(curl -s -X POST "$BASE_URL/agents/register" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "agent-01",
    "hostname": "node-01.example.com",
    "ip_address": "192.168.1.10",
    "platform": "linux",
    "architecture": "amd64",
    "version": "1.0.0",
    "capabilities": ["cpu", "memory", "disk", "process"],
    "labels": {
      "environment": "production",
      "region": "us-west-2",
      "datacenter": "dc1"
    }
  }')

echo "$AGENT_RESPONSE" | jq .
AGENT_ID=$(echo "$AGENT_RESPONSE" | jq -r '.data.agent_id')
echo "Agent ID: $AGENT_ID"
echo -e "\n"

# 3. Send heartbeat
echo "3. Sending heartbeat..."
curl -s -X PUT "$BASE_URL/agents/$AGENT_ID/heartbeat" \
  -H "X-API-Key: $API_KEY" | jq .
echo -e "\n"

# 4. Get agent details
echo "4. Getting agent details..."
curl -s "$BASE_URL/agents/$AGENT_ID" \
  -H "X-API-Key: $API_KEY" | jq .
echo -e "\n"

# 5. List all agents
echo "5. Listing all agents..."
curl -s "$BASE_URL/agents" \
  -H "X-API-Key: $API_KEY" | jq .
echo -e "\n"

# 6. Get agent stats
echo "6. Getting agent stats..."
curl -s "$BASE_URL/agents/stats" \
  -H "X-API-Key: $API_KEY" | jq .
echo -e "\n"

# 7. Register another agent (same hostname - should update)
echo "7. Re-registering agent with updated info..."
curl -s -X POST "$BASE_URL/agents/register" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "agent-01-updated",
    "hostname": "node-01.example.com",
    "ip_address": "192.168.1.10",
    "platform": "linux",
    "architecture": "amd64",
    "version": "1.1.0",
    "capabilities": ["cpu", "memory", "disk", "process", "network"],
    "labels": {
      "environment": "production",
      "region": "us-west-2",
      "datacenter": "dc1",
      "updated": "true"
    }
  }' | jq .
echo -e "\n"

# 8. Register a Kubernetes agent
echo "8. Registering Kubernetes agent..."
K8S_RESPONSE=$(curl -s -X POST "$BASE_URL/agents/register" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "k8s-agent-01",
    "hostname": "k8s-node-01",
    "platform": "kubernetes",
    "architecture": "amd64",
    "version": "1.0.0",
    "capabilities": ["kubernetes", "pods", "deployments"],
    "labels": {
      "cluster": "prod-cluster",
      "namespace": "monitoring"
    },
    "kubernetes_metadata": {
      "cluster_name": "prod-cluster",
      "namespace": "monitoring",
      "pod_name": "lumo-agent-abc123",
      "node_name": "k8s-node-01"
    }
  }')

echo "$K8S_RESPONSE" | jq .
K8S_AGENT_ID=$(echo "$K8S_RESPONSE" | jq -r '.data.agent_id')
echo "K8s Agent ID: $K8S_AGENT_ID"
echo -e "\n"

# 9. Filter agents by platform
echo "9. Filtering agents by platform (kubernetes)..."
curl -s "$BASE_URL/agents?platform=kubernetes" \
  -H "X-API-Key: $API_KEY" | jq .
echo -e "\n"

# 10. Filter agents by status
echo "10. Filtering agents by status (online)..."
curl -s "$BASE_URL/agents?status=online" \
  -H "X-API-Key: $API_KEY" | jq .
echo -e "\n"

# 11. Delete an agent
echo "11. Deleting agent..."
curl -s -X DELETE "$BASE_URL/agents/$K8S_AGENT_ID" \
  -H "X-API-Key: $API_KEY" | jq .
echo -e "\n"

# 12. Verify deletion
echo "12. Verifying agent was deleted..."
curl -s "$BASE_URL/agents/$K8S_AGENT_ID" \
  -H "X-API-Key: $API_KEY" | jq .
echo -e "\n"

# 13. Final stats
echo "13. Final agent stats..."
curl -s "$BASE_URL/agents/stats" \
  -H "X-API-Key: $API_KEY" | jq .
echo -e "\n"

echo "=== Test Complete ==="
