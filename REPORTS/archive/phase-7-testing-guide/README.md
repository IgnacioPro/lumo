# Phase 7: API Server & Agent Registration - Testing Guide

This directory contains comprehensive documentation and testing resources for **Phase 7: API Server Foundation**, which includes the complete agent registration system.

## 📁 Contents

### Documentation

- **[TESTING_GUIDE.md](TESTING_GUIDE.md)** - Complete guide for testing the agent registration API
  - Quick start instructions
  - Manual testing examples with curl
  - Database verification queries
  - Troubleshooting section
  - Testing Kubernetes agents
  - Re-registration testing

- **[API_REFERENCE.md](API_REFERENCE.md)** - Full API documentation
  - Authentication
  - All endpoint specifications
  - Request/response examples
  - Error handling

- **[openapi.yaml](openapi.yaml)** - OpenAPI 3.0 specification
  - Machine-readable API specification
  - Can be imported into Postman, Swagger UI, etc.

### Testing Resources

- **[test-agent-api.sh](test-agent-api.sh)** - Automated test script
  - Tests all 13 agent API operations
  - Registers Linux and Kubernetes agents
  - Tests filtering, pagination, updates, deletion
  - Requires `jq` for JSON formatting

## 🚀 Quick Start

### 1. Start Services

```bash
# From project root
docker-compose up -d
```

### 2. Run API Server

```bash
# Build and start
go build -o lumo ./cmd/lumo
./lumo serve
```

### 3. Create API Key

```bash
docker exec -it lumo-postgres psql -U lumo -d lumo -c "
INSERT INTO api_keys (id, name, key_hash, scopes, created_at, updated_at)
VALUES (
  gen_random_uuid(),
  'test-key',
  '5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8',
  ARRAY['diagnostics:run', 'jobs:read', 'jobs:write', 'agents:read', 'agents:write']::varchar[],
  NOW(),
  NOW()
);"
```

**API Key:** `test-key-123`

### 4. Run Tests

```bash
# From project root
./REPORTS/phase-7-testing-guide/test-agent-api.sh
```

## 📊 What Gets Tested

The automated test suite covers:

1. ✅ Health checks (`/health`, `/ready`, `/live`)
2. ✅ Agent registration (Linux, Kubernetes)
3. ✅ Heartbeat updates
4. ✅ Agent details retrieval
5. ✅ List agents with filtering
6. ✅ Agent statistics
7. ✅ Re-registration (updates)
8. ✅ Pagination
9. ✅ Platform filtering
10. ✅ Status filtering
11. ✅ Agent deletion
12. ✅ Deletion verification
13. ✅ Final statistics

## 🔗 API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/health` | Health check (no auth) |
| GET | `/api/v1/ready` | Readiness check (no auth) |
| GET | `/api/v1/live` | Liveness check (no auth) |
| POST | `/api/v1/agents/register` | Register/update agent |
| PUT | `/api/v1/agents/:id/heartbeat` | Send heartbeat |
| GET | `/api/v1/agents` | List agents (with filters) |
| GET | `/api/v1/agents/stats` | Get agent statistics |
| GET | `/api/v1/agents/:id` | Get agent details |
| DELETE | `/api/v1/agents/:id` | Delete agent |
| POST | `/api/v1/diagnostics` | Run diagnostics |
| GET | `/api/v1/jobs` | List jobs |
| GET | `/api/v1/jobs/:id` | Get job details |
| DELETE | `/api/v1/jobs/:id` | Delete job |

## 🔐 Authentication

All endpoints (except health checks) require an API key:

```bash
curl -H "X-API-Key: test-key-123" http://localhost:8080/api/v1/agents
```

## 📝 Example: Register an Agent

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

## 🔍 Database Verification

```bash
# Connect to database
docker exec -it lumo-postgres psql -U lumo -d lumo

# View all agents
SELECT id, name, hostname, platform, status, version, last_heartbeat_at
FROM agents
ORDER BY registered_at DESC;

# Count by status
SELECT status, COUNT(*) FROM agents GROUP BY status;

# View Kubernetes agents
SELECT name, hostname, kubernetes_metadata
FROM agents
WHERE platform = 'kubernetes';
```

## 🐛 Troubleshooting

### Server won't start
- Check PostgreSQL: `docker-compose ps postgres`
- Check logs: `docker-compose logs postgres`
- Verify connection: `psql -h localhost -U lumo -d lumo -c "SELECT version();"`

### "Unauthorized" errors
- Verify API key was created in database
- Check header: `X-API-Key: test-key-123`
- Verify API key has correct scopes

### Database connection errors
- Ensure docker-compose services are running
- Check `configs/config.example.yaml`
- Verify PostgreSQL is accessible on `localhost:5432`

## 📦 Phase 7 Implementation Status

**Status:** ✅ 100% Complete

**Completed:**
- ✅ REST API server with Chi router
- ✅ PostgreSQL database integration
- ✅ Redis cache integration
- ✅ Database migrations (jobs, api_keys, agents)
- ✅ API key authentication with scopes
- ✅ Health endpoints
- ✅ Job management system
- ✅ Diagnostic execution API
- ✅ Agent registration system
- ✅ Agent heartbeat tracking
- ✅ Agent statistics and filtering
- ✅ OpenAPI 3.0 specification
- ✅ Comprehensive testing guide
- ✅ Automated test suite

**Key Metrics:**
- 30+ files added
- 4,336 lines of code
- 6 API endpoints for agents
- 3 database migrations
- 13 automated tests

## 🎯 Next Steps

Once you've tested Phase 7, the next phases are:

- **Phase 8**: Agent Daemon implementation (`cmd/lumo-agent`)
- **Phase 9**: Kubernetes deployment (DaemonSet, Deployment, Helm)
- **Phase 10**: VM deployment (systemd, packages)
- **Phase 11**: Messaging integration (NATS, Kafka, RabbitMQ)
- **Phases 12-13**: Security hardening and production readiness

## 📚 Additional Resources

- **Project Documentation**: `CLAUDE.md` in project root
- **API Server Source**: `internal/api/`
- **Database Models**: `internal/database/models/`
- **Database Repositories**: `internal/database/repository/`
- **Migrations**: `internal/database/migrations/`

## 🤝 Contributing

When testing, if you find issues:

1. Check the troubleshooting section in `TESTING_GUIDE.md`
2. Verify all services are running with `docker-compose ps`
3. Check server logs for errors
4. Verify database connectivity
5. Report issues with full error messages and logs

## 📄 License

MIT License - See project root LICENSE file
