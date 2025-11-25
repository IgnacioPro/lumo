#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Lumo Slack Notifications Test ===${NC}\n"

# Check if API is running
echo -e "${YELLOW}1. Checking API server health...${NC}"
API_URL="${LUMO_API_URL:-http://localhost:8080}"
if curl -s -f "${API_URL}/api/v1/health" > /dev/null; then
    echo -e "${GREEN}✓ API server is healthy${NC}\n"
else
    echo -e "${RED}✗ API server is not responding at ${API_URL}${NC}"
    echo "Make sure the API server is running:"
    echo "  - Local: lumo serve --config ~/.lumo/config.yaml"
    echo "  - K8s: kubectl port-forward svc/lumo-api 8080:8080 -n lumo-system"
    exit 1
fi

# Check if notifications are configured
echo -e "${YELLOW}2. Checking notification configuration...${NC}"
if [ -z "$SLACK_WEBHOOK_URL" ]; then
    echo -e "${RED}✗ SLACK_WEBHOOK_URL environment variable is not set${NC}"
    echo ""
    echo "Set your Slack webhook URL:"
    echo "  export SLACK_WEBHOOK_URL='https://hooks.slack.com/services/YOUR/WEBHOOK/URL'"
    echo ""
    echo "Get a webhook URL from: https://api.slack.com/messaging/webhooks"
    exit 1
else
    echo -e "${GREEN}✓ SLACK_WEBHOOK_URL is configured${NC}\n"
fi

# Test webhook directly
echo -e "${YELLOW}3. Testing Slack webhook directly...${NC}"
DIRECT_TEST=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${SLACK_WEBHOOK_URL}" \
    -H "Content-Type: application/json" \
    -d '{"text":"🧪 Test from Lumo Slack notifications setup"}')

if [ "$DIRECT_TEST" = "200" ]; then
    echo -e "${GREEN}✓ Slack webhook is working (check your Slack channel)${NC}\n"
else
    echo -e "${RED}✗ Slack webhook test failed (HTTP $DIRECT_TEST)${NC}"
    echo "Verify your webhook URL is correct"
    exit 1
fi

# Submit a test event to the API
echo -e "${YELLOW}4. Submitting test event to API...${NC}"

# Create test payload
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
PAYLOAD=$(cat <<EOF
{
  "events": [
    {
      "event_type": "test-notification",
      "severity": "high",
      "resource_kind": "Pod",
      "resource_name": "test-notification-pod",
      "namespace": "default",
      "message": "This is a test notification from the Lumo Slack integration setup. If you see this in Slack, notifications are working correctly! 🎉",
      "event_timestamp": "${TIMESTAMP}",
      "metadata": {
        "test": true,
        "source": "test-slack-notifications.sh"
      }
    }
  ]
}
EOF
)

# Try to get a token (may not be needed if auth is disabled)
echo "Attempting to authenticate..."
TOKEN=$(curl -s -X POST "${API_URL}/api/v1/auth/token" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin"}' 2>/dev/null | jq -r '.token // empty')

# Submit event with or without token
if [ -n "$TOKEN" ]; then
    echo "Using JWT token for authentication"
    RESPONSE=$(curl -s -X POST "${API_URL}/api/v1/events" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD")
else
    echo "No authentication required or auth failed, trying without token"
    RESPONSE=$(curl -s -X POST "${API_URL}/api/v1/events" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD")
fi

# Check response
if echo "$RESPONSE" | jq -e '.accepted > 0' > /dev/null 2>&1; then
    EVENT_ID=$(echo "$RESPONSE" | jq -r '.event_ids[0]')
    echo -e "${GREEN}✓ Event submitted successfully (ID: ${EVENT_ID})${NC}\n"
else
    echo -e "${RED}✗ Failed to submit event${NC}"
    echo "Response: $RESPONSE"
    exit 1
fi

# Wait for async processing
echo -e "${YELLOW}5. Waiting for notification processing (5 seconds)...${NC}"
sleep 5

# Check API logs for notification status
echo -e "\n${YELLOW}6. Checking API server logs for notification status...${NC}"
if command -v kubectl &> /dev/null; then
    echo "Kubernetes logs:"
    kubectl logs -n lumo-system deployment/lumo-api --tail=20 | grep -i notification || true
fi

echo -e "\n${GREEN}=== Test Complete ===${NC}"
echo ""
echo "Expected results:"
echo "  1. ✓ You should see a test message in your Slack channel"
echo "  2. ✓ A formatted event notification with the test message"
echo ""
echo "If you received both notifications in Slack, the integration is working! 🎉"
echo ""
echo "To trigger real events, run:"
echo "  ./test-failure-scenarios.sh"
