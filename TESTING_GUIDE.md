# Agent Registration API - Testing Guide

This guide shows you how to test the agent registration system (Phase 7).

## Prerequisites

- Docker and Docker Compose installed
- Go 1.24+ installed
- `jq` for JSON formatting (optional but recommended)

## Quick Start

### 1. Start the Infrastructure

```bash
# Start PostgreSQL and Redis
docker-compose up -d

# Verify services are healthy
docker-compose ps
```

### 2. Run the API Server

```bash
# Build the server
go build -o lumo ./cmd/lumo

# Start the API server (runs migrations automatically)
./lumo serve
```

The server will start on `http://localhost:8080` and automatically:
- Run database migrations (creates `jobs`, `api_keys`, `agents` tables)
- Connect to PostgreSQL and Redis
- Set up all API routes

### 3. Create an API Key

In another terminal:

```bash
# Connect to the database
docker exec -it lumo-postgres psql -U lumo -d lumo

# Create a test API key
INSERT INTO api_keys (id, name, key_hash, scopes, created_at, updated_at)
VALUES (
  gen_random_uuid(),
  'test-key',
  '5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8',
  ARRAY['diagnostics:run', 'jobs:read', 'jobs:write', 'agents:read', 'agents:write']::varchar[],
  NOW(),
  NOW()
);

# Verify
SELECT id, name, scopes, created_at FROM api_keys;

# Exit
\q
```

**API Key:** `test-key-123` (SHA-256 hash stored in database)

### 4. Run the Test Suite

```bash
# Run the comprehensive test script
./test-agent-api.sh
```

This tests all agent endpoints:
- Health checks
- Agent registration
- Heartbeats
- Get agent details
- List agents (with filtering)
- Agent statistics
- Re-registration (updates existing agent)
- Kubernetes agent registration
- Agent deletion

## Manual Testing Examples

### Register an Agent

```bash
curl -X POST http://localhost:8080/api/v1/agents/register \
  -H "X-API-Key: test-key-123" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-agent",
    "hostname": "server-01",
    "ip_address": "10.0.1.5",
    "platform": "linux",
    "architecture": "amd64",
    "version": "1.0.0",
    "capabilities": ["cpu", "memory", "disk"],
    "labels": {
      "environment": "production",
      "region": "us-east-1"
    }
  }'
```

**Response:**
```json
{
  "status": "success",
  "data": {
    "agent_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "my-agent",
    "hostname": "server-01",
    "status": "online",
    "registered_at": "2025-11-18T15:30:00Z"
  }
}
```

### Send Heartbeat

```bash
curl -X PUT http://localhost:8080/api/v1/agents/{agent-id}/heartbeat \
  -H "X-API-Key: test-key-123"
```

### List All Agents

```bash
curl http://localhost:8080/api/v1/agents \
  -H "X-API-Key: test-key-123"
```

### Filter Agents

```bash
# By platform
curl "http://localhost:8080/api/v1/agents?platform=kubernetes" \
  -H "X-API-Key: test-key-123"

# By status
curl "http://localhost:8080/api/v1/agents?status=online" \
  -H "X-API-Key: test-key-123"

# With pagination
curl "http://localhost:8080/api/v1/agents?limit=10&offset=0" \
  -H "X-API-Key: test-key-123"
```

### Get Agent Details

```bash
curl http://localhost:8080/api/v1/agents/{agent-id} \
  -H "X-API-Key: test-key-123"
```

### Get Agent Statistics

```bash
curl http://localhost:8080/api/v1/agents/stats \
  -H "X-API-Key: test-key-123"
```

**Response:**
```json
{
  "status": "success",
  "data": {
    "total": 5,
    "online": 4,
    "offline": 1,
    "error": 0,
    "by_status": {
      "online": 4,
      "offline": 1
    }
  }
}
```

### Delete an Agent

```bash
curl -X DELETE http://localhost:8080/api/v1/agents/{agent-id} \
  -H "X-API-Key: test-key-123"
```

## Testing Kubernetes Agents

Register a Kubernetes agent with metadata:

```bash
curl -X POST http://localhost:8080/api/v1/agents/register \
  -H "X-API-Key: test-key-123" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "k8s-agent",
    "hostname": "k8s-node-01",
    "platform": "kubernetes",
    "architecture": "amd64",
    "version": "1.0.0",
    "capabilities": ["kubernetes", "pods", "deployments"],
    "kubernetes_metadata": {
      "cluster_name": "prod-cluster",
      "namespace": "monitoring",
      "pod_name": "lumo-agent-xyz",
      "node_name": "k8s-node-01"
    }
  }'
```

## Testing Re-registration

If you register an agent with an existing hostname, it will **update** the existing registration instead of creating a new one:

```bash
# First registration
curl -X POST http://localhost:8080/api/v1/agents/register \
  -H "X-API-Key: test-key-123" \
  -d '{"name":"agent-v1","hostname":"server-01",...}'
# Returns: 201 Created with agent_id

# Re-registration (same hostname)
curl -X POST http://localhost:8080/api/v1/agents/register \
  -H "X-API-Key: test-key-123" \
  -d '{"name":"agent-v2","hostname":"server-01",...}'
# Returns: 200 OK with SAME agent_id (updated)
```

## Verifying in the Database

```bash
# Connect to PostgreSQL
docker exec -it lumo-postgres psql -U lumo -d lumo

# View all agents
SELECT id, name, hostname, platform, status, version, last_heartbeat_at
FROM agents
ORDER BY registered_at DESC;

# View agent capabilities
SELECT name, hostname, capabilities, labels
FROM agents;

# View Kubernetes agents
SELECT name, hostname, kubernetes_metadata
FROM agents
WHERE platform = 'kubernetes';

# Count by status
SELECT status, COUNT(*)
FROM agents
GROUP BY status;
```

## Troubleshooting

### Server won't start
```bash
# Check if PostgreSQL is running
docker-compose ps postgres

# Check PostgreSQL logs
docker-compose logs postgres

# Verify connection
psql -h localhost -U lumo -d lumo -c "SELECT version();"
```

### "Unauthorized" errors
- Verify you created the API key in the database
- Verify you're using `X-API-Key: test-key-123` header
- Check API key has correct scopes (`agents:read`, `agents:write`)

### Database connection errors
- Ensure docker-compose services are running
- Check `configs/config.example.yaml` for database settings
- Verify PostgreSQL is accessible on `localhost:5432`

### Migrations not running
```bash
# Manually run migrations
./lumo serve --config configs/config.example.yaml

# Or connect to DB and check
docker exec -it lumo-postgres psql -U lumo -d lumo -c "\dt"
```

## Cleanup

```bash
# Stop the API server (Ctrl+C)

# Stop and remove containers
docker-compose down

# Remove volumes (WARNING: deletes all data)
docker-compose down -v
```

## Next Steps

Once you've tested the API locally:

1. **Phase 8**: Implement the agent daemon (`cmd/lumo-agent`)
2. **Phase 9**: Create Kubernetes manifests (DaemonSet, Deployment)
3. **Phase 10**: Create systemd service units for VMs
4. **Phase 11**: Add messaging integration (NATS/Kafka)
5. **Phases 12-13**: Security hardening and production readiness

## API Reference

See the complete OpenAPI specification: `api/openapi.yaml`

Or view the API documentation: `api/README.md`
