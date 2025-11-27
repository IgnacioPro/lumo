# Phase 9: API Server Implementation Plan

**Date:** 2025-11-18
**Status:** Planning Complete - Ready for Implementation
**Version:** 1.0
**Estimated Timeline:** 9-10 weeks

---

## Executive Summary

Phase 9 introduces a comprehensive API server and distributed agent system to Lumo, transforming it from a CLI-only tool to a full-featured SRE/DevOps automation platform. The implementation is divided into three sub-phases:

- **Phase 9.1:** Core API Server Foundation (~2,250 LOC, 3 weeks)
- **Phase 9.2:** Distributed Agent System (~3,650 LOC, 4 weeks)
- **Phase 9.3:** Advanced Features & Polish (~2,850 LOC, 2-3 weeks)

**Total New Code:** ~8,750 lines across ~45 new files

---

## Current State Analysis

### Codebase Metrics
- **Total Files:** 89 Go files (54 source + 35 test)
- **Test Coverage:** 50.4%
- **Architecture:** Well-structured with `internal/` organization
- **Patterns:** Interface-based design, executor pattern, parallel processing

### Existing Infrastructure (Reusable)
✅ Config system with validation (Viper)
✅ Structured logging (Logrus)
✅ SSH client with connection pooling
✅ Diagnostics runner with parallel execution (12 checkers)
✅ Remediation with approval workflow & audit logging
✅ TOON format implementation (30-60% token reduction)
✅ APIConfig structure already exists (`internal/config/config.go:55-65`)

### Missing Components (To Implement)
❌ HTTP server and REST API
❌ Database layer (PostgreSQL)
❌ Caching layer (Redis)
❌ Agent daemon system
❌ WebSocket communication
❌ Job queue and dispatching
❌ Authentication (API keys, JWT)
❌ API documentation (OpenAPI/Swagger)

---

## Phase 9.1: Core API Server (Foundation)

**Timeline:** 3 weeks
**Lines of Code:** ~2,250
**Dependencies:** Chi, PostgreSQL, Redis, goose

### Architecture

```
internal/
├── api/
│   ├── server.go              # HTTP server setup with graceful shutdown
│   ├── router.go              # Chi route definitions
│   ├── middleware/
│   │   ├── auth.go            # API key authentication
│   │   ├── logging.go         # Request/response logging
│   │   ├── recovery.go        # Panic recovery
│   │   └── cors.go            # CORS handling
│   ├── handlers/
│   │   ├── diagnostics.go     # POST /api/v1/diagnostics
│   │   ├── remediation.go     # POST /api/v1/remediation
│   │   ├── health.go          # GET /api/v1/health
│   │   ├── status.go          # GET /api/v1/status
│   │   └── jobs.go            # GET /api/v1/jobs, GET /api/v1/jobs/:id
│   └── response/
│       ├── response.go        # Standard JSON response wrapper
│       └── errors.go          # Error codes and handling
├── database/
│   ├── postgres.go            # PostgreSQL connection pool
│   ├── migrations/            # SQL migration files
│   │   ├── 001_initial.sql
│   │   ├── 002_api_keys.sql
│   │   └── 003_jobs_indexes.sql
│   ├── models/
│   │   ├── job.go             # Job database model
│   │   ├── api_key.go         # API key model
│   │   └── result.go          # Diagnostic result model
│   └── repository/
│       ├── job.go             # Job CRUD operations
│       └── api_key.go         # API key management
└── cache/
    └── redis.go               # Redis client wrapper
```

### Database Schema

#### Migration 001: Initial Schema
```sql
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(50) NOT NULL,      -- 'diagnostic' or 'remediation'
    status VARCHAR(20) NOT NULL,     -- 'pending', 'running', 'completed', 'failed'
    target VARCHAR(255) NOT NULL,    -- hostname or IP
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_by VARCHAR(255),         -- API key identifier
    result JSONB,                    -- Diagnostic/remediation results
    error TEXT,                      -- Error message if failed
    metadata JSONB                   -- Additional context
);

CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_type ON jobs(type);
CREATE INDEX idx_jobs_created_at ON jobs(created_at DESC);
CREATE INDEX idx_jobs_target ON jobs(target);
```

#### Migration 002: API Keys
```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash VARCHAR(64) UNIQUE NOT NULL,  -- SHA-256 hash
    name VARCHAR(255) NOT NULL,            -- Friendly name
    scopes TEXT[] NOT NULL DEFAULT '{}',   -- Permissions
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    revoked BOOLEAN DEFAULT FALSE
);

CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_revoked ON api_keys(revoked) WHERE revoked = FALSE;
```

### API Endpoints (Phase 9.1)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/health` | None | Health check (200 OK) |
| GET | `/api/v1/status` | API Key | Server status & version |
| POST | `/api/v1/diagnostics` | API Key | Run diagnostic checks |
| POST | `/api/v1/remediation` | API Key | Execute remediation actions |
| GET | `/api/v1/jobs` | API Key | List jobs (paginated) |
| GET | `/api/v1/jobs/:id` | API Key | Get job status & result |

### Request/Response Examples

#### POST /api/v1/diagnostics
```json
{
  "target": "localhost",
  "checks": ["cpu", "memory", "disk"],
  "format": "toon",
  "analyze": true,
  "dry_run": false
}
```

**Response:**
```json
{
  "success": true,
  "job_id": "123e4567-e89b-12d3-a456-426614174000",
  "status": "pending",
  "created_at": "2025-11-18T10:00:00Z"
}
```

#### GET /api/v1/jobs/:id
```json
{
  "success": true,
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "type": "diagnostic",
    "status": "completed",
    "target": "localhost",
    "created_at": "2025-11-18T10:00:00Z",
    "started_at": "2025-11-18T10:00:01Z",
    "completed_at": "2025-11-18T10:00:15Z",
    "result": {
      "timestamp": "2025-11-18T10:00:01Z",
      "duration": "14.2s",
      "summary": {
        "total_checks": 12,
        "ok_count": 10,
        "warning_count": 2,
        "critical_count": 0
      },
      "results": [ /* ... */ ]
    }
  }
}
```

### Configuration Updates

**configs/config.example.yaml:**
```yaml
# Database Configuration
database:
  host: localhost
  port: 5432
  name: lumo
  user: lumo
  # Password via LUMO_DATABASE_PASSWORD env var
  ssl_mode: disable
  max_connections: 25
  max_idle: 5
  conn_max_lifetime: 5m

# Cache Configuration
cache:
  enabled: true
  redis_url: redis://localhost:6379/0
  # Password via LUMO_CACHE_PASSWORD env var
  max_retries: 3
  pool_size: 10
  ttl: 1h
```

**internal/config/config.go additions:**
```go
type Config struct {
    SSH         SSHConfig         `mapstructure:"ssh"`
    AI          AIConfig          `mapstructure:"ai"`
    Logging     LoggingConfig     `mapstructure:"logging"`
    API         APIConfig         `mapstructure:"api"`
    Diagnostics DiagnosticsConfig `mapstructure:"diagnostics"`
    Database    DatabaseConfig    `mapstructure:"database"`    // NEW
    Cache       CacheConfig       `mapstructure:"cache"`       // NEW
}

type DatabaseConfig struct {
    Host            string        `mapstructure:"host"`
    Port            int           `mapstructure:"port"`
    Name            string        `mapstructure:"name"`
    User            string        `mapstructure:"user"`
    Password        string        `mapstructure:"password"`
    SSLMode         string        `mapstructure:"ssl_mode"`
    MaxConnections  int           `mapstructure:"max_connections"`
    MaxIdle         int           `mapstructure:"max_idle"`
    ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type CacheConfig struct {
    Enabled    bool          `mapstructure:"enabled"`
    RedisURL   string        `mapstructure:"redis_url"`
    Password   string        `mapstructure:"password"`
    MaxRetries int           `mapstructure:"max_retries"`
    PoolSize   int           `mapstructure:"pool_size"`
    TTL        time.Duration `mapstructure:"ttl"`
}
```

### Implementation Tasks (Phase 9.1)

#### Week 1: Infrastructure
- [x] Add Go dependencies (Chi, lib/pq, go-redis, goose)
- [ ] Create directory structure
- [ ] Database connection pool implementation
- [ ] Migration system setup (goose)
- [ ] Initial migrations (jobs, api_keys)
- [ ] Redis client wrapper
- [ ] Config updates (DatabaseConfig, CacheConfig)

#### Week 2: HTTP Server & Handlers
- [ ] HTTP server with graceful shutdown
- [ ] Chi router setup
- [ ] Middleware: logging, recovery, CORS
- [ ] API key authentication middleware
- [ ] Health check handler
- [ ] Status handler
- [ ] Diagnostics handler
- [ ] Jobs handler (list, get)
- [ ] Response utilities

#### Week 3: Testing & Polish
- [ ] Unit tests (handlers, repos, middleware)
- [ ] Integration tests (full HTTP flow)
- [ ] TOON format support in API responses
- [ ] Error handling improvements
- [ ] Update serve.go command
- [ ] Documentation (inline + DEVELOPMENT.md)

### Dependencies Added

```go
// go.mod additions
require (
    github.com/go-chi/chi/v5 v5.0.11
    github.com/go-chi/cors v1.2.1
    github.com/lib/pq v1.10.9
    github.com/redis/go-redis/v9 v9.4.0
    github.com/pressly/goose/v3 v3.17.0
)
```

---

## Phase 9.2: Distributed Agent System

**Timeline:** 4 weeks
**Lines of Code:** ~3,650
**Dependencies:** gorilla/websocket

### Architecture

```
internal/
├── agent/
│   ├── agent.go               # Main agent coordinator
│   ├── registration.go        # Server registration logic
│   ├── heartbeat.go           # Keepalive mechanism
│   ├── poller.go              # Job polling (fallback)
│   ├── executor.go            # Local job execution
│   ├── reporter.go            # Result reporting
│   └── config.go              # Agent-specific config
├── websocket/
│   ├── server.go              # WebSocket server
│   ├── client.go              # WebSocket client (agent-side)
│   ├── hub.go                 # Connection hub & routing
│   └── message.go             # Message protocol types
└── queue/
    ├── queue.go               # Redis-backed job queue
    ├── consumer.go            # Queue consumer
    └── producer.go            # Queue producer

cmd/lumo/
├── agent.go                   # `lumo agent` command
└── serve.go                   # Enhanced with agent endpoints
```

### Database Schema

#### Migration 004: Agents Table
```sql
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hostname VARCHAR(255) NOT NULL,
    ip_address VARCHAR(45) NOT NULL,
    status VARCHAR(20) NOT NULL,         -- 'online', 'offline', 'error'
    version VARCHAR(50),                 -- Lumo version
    platform VARCHAR(50),                -- OS/arch
    registered_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_heartbeat TIMESTAMP NOT NULL DEFAULT NOW(),
    last_job_id UUID,
    metadata JSONB,                      -- Custom tags, capabilities
    tags TEXT[]                          -- Searchable tags
);

CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_last_heartbeat ON agents(last_heartbeat DESC);
CREATE INDEX idx_agents_tags ON agents USING GIN(tags);

-- Add agent_id to jobs table
ALTER TABLE jobs ADD COLUMN agent_id UUID REFERENCES agents(id);
CREATE INDEX idx_jobs_agent_id ON jobs(agent_id);
```

### API Endpoints (Phase 9.2 additions)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/agents/register` | API Key | Agent registration |
| POST | `/api/v1/agents/:id/heartbeat` | API Key | Heartbeat ping |
| GET | `/api/v1/agents` | API Key | List agents (with filters) |
| GET | `/api/v1/agents/:id` | API Key | Get agent details |
| GET | `/api/v1/agents/:id/jobs` | API Key | Get jobs for agent |
| WS | `/api/v1/ws/agents/:id` | API Key | WebSocket connection |

### WebSocket Message Protocol

```json
{
  "type": "job_assigned|job_result|heartbeat|ping|pong",
  "timestamp": "2025-11-18T10:00:00Z",
  "agent_id": "uuid",
  "payload": {
    "job_id": "uuid",
    "data": { /* type-specific payload */ }
  }
}
```

**Message Types:**
- `job_assigned`: Server → Agent (new job)
- `job_result`: Agent → Server (job completion)
- `heartbeat`: Agent → Server (keepalive)
- `ping/pong`: Bidirectional (connection check)

### Agent Workflow

```
1. Agent Startup:
   - Load config
   - Register with server (POST /api/v1/agents/register)
   - Receive agent_id + API key
   - Establish WebSocket connection

2. Active Loop:
   - Send heartbeat every 30s
   - Listen for job_assigned messages
   - Execute jobs locally (reuse diagnostics/remediation)
   - Send job_result messages
   - Reconnect WebSocket if disconnected

3. Shutdown:
   - Send final heartbeat with status='offline'
   - Close WebSocket gracefully
   - Clean up resources
```

### Implementation Tasks (Phase 9.2)

#### Week 1: Agent Foundation
- [ ] Agent structure and lifecycle
- [ ] Registration logic
- [ ] Heartbeat mechanism
- [ ] Job polling (HTTP fallback)
- [ ] Agent config loading
- [ ] Agent database schema
- [ ] Agent API endpoints

#### Week 2: WebSocket Server
- [ ] WebSocket server setup
- [ ] Connection hub (routing)
- [ ] Message protocol definitions
- [ ] Authentication for WebSocket
- [ ] Broadcast capabilities
- [ ] Connection lifecycle management

#### Week 3: WebSocket Client & Queue
- [ ] WebSocket client (agent-side)
- [ ] Reconnection logic with backoff
- [ ] Redis job queue implementation
- [ ] Queue producer (API → Queue)
- [ ] Queue consumer (Queue → WebSocket)
- [ ] Multi-instance coordination (Redis pub/sub)

#### Week 4: Testing & Edge Cases
- [ ] Agent unit tests
- [ ] WebSocket integration tests
- [ ] Connection drop/reconnect tests
- [ ] Load testing (100+ agents)
- [ ] Queue backlog handling
- [ ] Documentation & examples

### Dependencies Added

```go
require (
    github.com/gorilla/websocket v1.5.1
)
```

---

## Phase 9.3: Advanced Features (Polish)

**Timeline:** 2-3 weeks
**Lines of Code:** ~2,850
**Dependencies:** JWT, Swagger, Prometheus

### Features

1. **JWT Authentication** (for web UI)
   - User login endpoint
   - Token generation & refresh
   - JWT validation middleware
   - User management

2. **OpenAPI/Swagger Documentation**
   - Swagger annotations in handlers
   - Auto-generated OpenAPI spec
   - Swagger UI at `/api/v1/docs`

3. **Rate Limiting**
   - Redis-backed rate limiter
   - Per-API-key limits
   - Configurable thresholds

4. **Metrics & Observability**
   - Prometheus metrics endpoint
   - Custom collectors (jobs, agents, queue)
   - Request duration histograms

5. **RBAC** (Role-Based Access Control)
   - Roles table (admin, user, viewer)
   - Permission checks
   - Scope-based API key access

### API Endpoints (Phase 9.3 additions)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/auth/login` | None | Login (get JWT) |
| POST | `/api/v1/auth/refresh` | JWT | Refresh access token |
| GET | `/api/v1/users` | JWT (admin) | List users |
| POST | `/api/v1/users` | JWT (admin) | Create user |
| GET | `/api/v1/metrics` | API Key | Prometheus metrics |
| GET | `/api/v1/docs` | None | Swagger UI |
| GET | `/api/v1/openapi.json` | None | OpenAPI spec |

### Database Schema

#### Migration 005: Users
```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_login TIMESTAMP,
    active BOOLEAN DEFAULT TRUE
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
```

#### Migration 006: RBAC
```sql
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    permissions TEXT[] NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE user_roles (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

-- Seed default roles
INSERT INTO roles (name, permissions) VALUES
    ('admin', ARRAY['*']),
    ('user', ARRAY['diagnostics:read', 'diagnostics:write', 'jobs:read']),
    ('viewer', ARRAY['diagnostics:read', 'jobs:read']);
```

### Implementation Tasks (Phase 9.3)

#### Week 1: Authentication
- [ ] JWT middleware
- [ ] Login endpoint
- [ ] Token refresh endpoint
- [ ] User model & repository
- [ ] Password hashing (bcrypt)
- [ ] User management endpoints

#### Week 2: Documentation & Limits
- [ ] Swagger annotations
- [ ] OpenAPI spec generation
- [ ] Swagger UI integration
- [ ] Rate limiting middleware
- [ ] Rate limit config
- [ ] Rate limit tests

#### Week 3: Observability & RBAC
- [ ] Prometheus metrics setup
- [ ] Custom collectors
- [ ] Metrics endpoint
- [ ] RBAC database schema
- [ ] Permission checking middleware
- [ ] RBAC tests
- [ ] Security audit

### Dependencies Added

```go
require (
    github.com/golang-jwt/jwt/v5 v5.2.0
    github.com/swaggo/swag v1.16.2
    github.com/swaggo/http-swagger v1.3.4
    github.com/prometheus/client_golang v1.18.0
    golang.org/x/crypto v0.18.0  // Already present
)
```

---

## Dependencies & Risks

### External Service Dependencies

| Service | Version | Purpose | HA Strategy |
|---------|---------|---------|-------------|
| PostgreSQL | >= 12.0 | Persistent storage | Master-replica replication |
| Redis | >= 6.0 | Cache, queue, pub/sub | Sentinel or Cluster |

### Implementation Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Database schema breaking changes | High | Medium | Versioned migrations, rollback scripts |
| WebSocket connection storms | High | Medium | Connection limits, rate limiting |
| Redis single point of failure | High | Low | Redis Sentinel/Cluster, AOF persistence |
| API key leakage | Critical | Low | SHA-256 hashing, rotation, audit logs |
| Agent authentication bypass | Critical | Low | Consider mTLS for production |
| Job queue backlog | Medium | Medium | Queue monitoring, alerts, auto-scaling |
| Migration failures | High | Low | Staging environment, backup before migrate |
| Memory leaks in agents | Medium | Medium | Resource limits, profiling |

### Testing Requirements

**Phase 9.1:**
- Unit tests: Handlers, middleware, repositories
- Integration tests: Full HTTP request flow
- Load tests: 100+ concurrent requests
- Security tests: Auth bypass, SQL injection

**Phase 9.2:**
- Unit tests: Agent logic, queue operations
- Integration tests: Registration → execution → result
- Stress tests: 100+ agents, 1000+ jobs
- Network tests: Connection drops, reconnection

**Phase 9.3:**
- Unit tests: JWT validation, rate limiting
- Integration tests: Full auth flow
- Load tests: Rate limits under load
- Security audit: OWASP Top 10

---

## Timeline & Milestones

### Phase 9.1-alpha (2 weeks)
**Goal:** Working API server with basic endpoints

**Milestones:**
- ✅ Database connected, migrations run
- ✅ HTTP server starts on port 8080
- ✅ Health check returns 200 OK
- ✅ Can run diagnostic via API
- ✅ Results stored in PostgreSQL

### Phase 9.1-beta (1 week)
**Goal:** Production-ready foundation

**Milestones:**
- ✅ Redis caching enabled
- ✅ TOON format in API responses
- ✅ Tests >= 50% coverage
- ✅ Error handling robust
- ✅ API key authentication working

### Phase 9.2-alpha (2 weeks)
**Goal:** Agents can execute jobs

**Milestones:**
- ✅ Agent daemon runs
- ✅ Registration successful
- ✅ Heartbeat working
- ✅ Job polling functional
- ✅ Jobs execute and report

### Phase 9.2-beta (2 weeks)
**Goal:** Real-time WebSocket communication

**Milestones:**
- ✅ WebSocket server running
- ✅ Agents connect via WebSocket
- ✅ Real-time job dispatching
- ✅ Multi-instance coordination
- ✅ Queue handles 1000+ jobs

### Phase 9.3 (2-3 weeks)
**Goal:** Production-grade features

**Milestones:**
- ✅ JWT authentication
- ✅ Swagger UI at /api/v1/docs
- ✅ Rate limiting prevents abuse
- ✅ Prometheus metrics exposed
- ✅ Security audit passes

**Total Timeline:** 9-10 weeks

---

## Development Environment Setup

### Docker Compose
Create `docker-compose.yaml`:

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: lumo
      POSTGRES_PASSWORD: lumo_dev
      POSTGRES_DB: lumo
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U lumo"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
  redis_data:
```

### Environment Variables
```bash
# Database
export LUMO_DATABASE_HOST=localhost
export LUMO_DATABASE_PORT=5432
export LUMO_DATABASE_NAME=lumo
export LUMO_DATABASE_USER=lumo
export LUMO_DATABASE_PASSWORD=lumo_dev

# Cache
export LUMO_CACHE_ENABLED=true
export LUMO_CACHE_REDIS_URL=redis://localhost:6379/0

# API (existing)
export LUMO_API_PORT=8080
export LUMO_API_HOST=0.0.0.0
```

### Quick Start Commands
```bash
# Start services
docker-compose up -d

# Run migrations
go run cmd/lumo/main.go migrate up

# Start API server
go run cmd/lumo/main.go serve --port 8080

# Start agent
go run cmd/lumo/main.go agent --server http://localhost:8080
```

---

## Success Criteria

### Phase 9.1 Complete When:
- ✅ `lumo serve --port 8080` starts successfully
- ✅ `curl http://localhost:8080/api/v1/health` returns 200 OK
- ✅ Can create diagnostic job via API
- ✅ Results stored in PostgreSQL
- ✅ API key authentication blocks unauthorized requests
- ✅ Tests >= 50% coverage

### Phase 9.2 Complete When:
- ✅ `lumo agent --server http://localhost:8080` registers successfully
- ✅ Server dispatches jobs to connected agents
- ✅ Agents execute jobs locally
- ✅ WebSocket maintains connection for 24h+
- ✅ 100+ agents can connect simultaneously
- ✅ Queue handles 1000+ jobs without blocking

### Phase 9.3 Complete When:
- ✅ JWT login/refresh flow works
- ✅ Swagger UI accessible at `/api/v1/docs`
- ✅ Rate limiting prevents >100 req/min per key
- ✅ Prometheus metrics exposed at `/api/v1/metrics`
- ✅ Security scan (gosec) passes
- ✅ Load test: 1000 req/s for 5 minutes

---

## Code Quality Gates

Before merging each sub-phase:
- [ ] All tests pass: `go test ./...`
- [ ] No linter errors: `golangci-lint run`
- [ ] Test coverage >= 50% (maintain current level)
- [ ] No security vulnerabilities: `gosec ./...`
- [ ] Documentation updated (CLAUDE.md, DEVELOPMENT.md)
- [ ] Example usage documented

---

## Documentation Updates Required

### CLAUDE.md
- Update "Codebase Structure" section with new directories
- Update "Technology Stack" with new dependencies
- Mark Phase 9 sub-phases as complete when done
- Update CLI commands table with `serve` and `agent`

### DEVELOPMENT.md
- Add API usage examples
- Document authentication flow
- Add agent setup guide
- Include curl/Postman examples

### README.md
- Highlight API server capability
- Add "Quick Start API" section
- Link to API documentation

### New: API_REFERENCE.md
- Full endpoint documentation
- Request/response schemas
- Authentication guide
- Error codes reference

---

## Recommendations

### Implementation Strategy
**Recommended:** Sequential implementation with alpha/beta iterations

**Rationale:**
- Each phase builds on previous infrastructure
- Easier to test incrementally
- Lower risk of integration issues
- Clear milestones for progress tracking

### Alternative Approaches Considered

**Option 2: Parallel Implementation**
- Pros: Faster delivery, can utilize multiple developers
- Cons: Higher coordination overhead, integration challenges
- Decision: Not recommended unless team size > 3

**Option 3: MVP-First**
- Pros: Fastest to working system, early validation
- Cons: Technical debt, may need refactoring
- Decision: Not recommended for production-grade system

### Post-Phase 9 Considerations

**Scalability:**
- Add load balancer support (multiple API servers)
- Database read replicas
- Redis Cluster for HA
- Horizontal agent scaling

**Security Enhancements:**
- mTLS for agent authentication
- API key rotation automation
- Intrusion detection
- Audit log analysis

**Monitoring:**
- Distributed tracing (OpenTelemetry)
- Centralized logging (ELK stack)
- Alert rules (PagerDuty integration)
- Dashboard (Grafana)

---

## Appendix: File Count Estimate

### New Files by Phase

**Phase 9.1:** ~18 files
- internal/api: 5 files (server, router, etc.)
- internal/api/middleware: 4 files
- internal/api/handlers: 5 files
- internal/api/response: 2 files
- internal/database: 1 file
- internal/database/models: 3 files
- internal/database/repository: 2 files
- internal/database/migrations: 3 files
- internal/cache: 1 file

**Phase 9.2:** ~15 files
- internal/agent: 6 files
- internal/websocket: 4 files
- internal/queue: 3 files
- cmd/lumo: 1 file (agent.go)
- migrations: 1 file

**Phase 9.3:** ~12 files
- internal/api/handlers: 2 files (auth, users)
- internal/api/middleware: 3 files (jwt, ratelimit, metrics)
- internal/metrics: 2 files
- internal/database/models: 2 files
- migrations: 2 files
- docs: 1 file (swagger)

**Total New Files:** ~45

---

**Document Version:** 1.0
**Last Updated:** 2025-11-18
**Next Review:** After Phase 9.1 completion
