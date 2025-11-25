# 🧪 Comprehensive Testing Checklist for Lumo

## 1. CLI Commands (cmd/lumo)

### `lumo doctor`
- All 6 health checks pass with valid config
- Detects missing config file
- Detects invalid AI provider configuration
- Detects missing API keys
- Detects missing system dependencies
- Validates RAG system when enabled
- Checks for available updates
- Verbose mode shows timing for each check

### `lumo init`
- Creates config file in correct location
- Interactive prompts work correctly
- Validates AI provider selection
- Securely handles API key input
- Creates necessary directories
- Doesn't overwrite existing config without confirmation

### `lumo diagnose`
- Localhost auto-detection (localhost, 127.0.0.1, ::1, 0.0.0.0)
- SSH connection with each auth method (agent, key, password, interactive)
- All 12 checkers run successfully
- `--list-checks` displays all available checks
- `--checks` flag filters specific checkers
- Output formats: text, TOON, JSON
- `--analyze` flag triggers AI analysis
- AI analysis works with each provider (Anthropic, OpenAI, Gemini, Ollama, OpenRouter)
- RAG context enhances AI analysis when enabled
- Streaming output displays correctly
- `--dry-run` shows what would be executed

### `lumo fix`
- Detects fixable issues from diagnostics
- Presents remediation plan for approval
- Executes approved actions
- Respects `--dry-run` flag
- Creates audit log entries
- Handles action failures gracefully
- SSH-based remediation works
- Localhost remediation works

### `lumo serve`
- API server starts on configured port
- PostgreSQL connection established
- Redis connection established
- Health endpoint responds
- Metrics endpoint exposes Prometheus data
- CORS configuration works
- Graceful shutdown on SIGTERM or SIGINT

### `lumo connect`
- Establishes SSH connection
- All 4 auth methods work
- Connection retry on failure
- Interactive shell session

### `lumo examples`
- Displays usage examples
- Shows tutorials

---

## 2. Agent Daemon (cmd/lumo-agent)

### Core Functionality
- Agent starts successfully
- Registers with API server on startup
- Heartbeat mechanism works
- Health check endpoint responds (:8081/health)
- Metrics endpoint works (:8081/metrics)
- Version command shows correct version

### Operational Modes
- Scheduled mode: Runs diagnostics at configured intervals
- On-demand mode: Waits for API triggers
- Continuous mode: Runs diagnostics continuously with minimal delay
- Hybrid mode: Combines scheduled and on-demand

### Offline Mode
- Caches diagnostics when API unavailable
- Replays cached diagnostics when API returns
- TTL-based cache expiration works

### Error Handling
- Handles API server unreachability
- Retries failed operations with backoff
- Logs errors appropriately

---

## 3. Diagnostic Checkers (12 Total)

### Core Checkers (6)
- CPU: Detects high usage, load averages, per-core utilization
- Memory: Detects high RAM or swap usage, identifies top consumers
- Disk: Detects low space, inode exhaustion, high I/O wait
- Process: Detects zombie processes, resource hogs
- Service: Checks systemd or init service status
- Network: Validates interface status, connectivity, DNS resolution

### Security Checkers (4)
- Patch Status: Detects pending security updates
- Open Ports: Lists listening services, identifies unexpected ports
- SSH Security: Checks PermitRootLogin, PasswordAuthentication, etc.
- Auth Failures: Detects failed login attempts

### Specialized Checkers (2)
- Kubernetes: Checks pods, nodes, services, deployments
- Proxmox VE: Checks cluster status, VMs, storage, HA state

### Cross-Cutting
- Severity levels assigned correctly (INFO, WARNING, ERROR, CRITICAL)
- Multiple issues detected simultaneously
- Checkers handle command failures gracefully
- Timeout handling for long-running checks

---

## 4. AI Providers (5 Total)

### Provider Integration
- Anthropic: Claude models work, streaming enabled
- OpenAI: GPT models work, streaming enabled
- Gemini: Google models work, streaming enabled
- Ollama: Local models work, streaming enabled
- OpenRouter: Multi-provider routing works

### AI Features
- Provider switching via config or environment variable
- Streaming output displays in real time
- Token usage tracked and reported
- Custom prompts work
- Error handling for API failures
- Rate limiting handled gracefully
- Timeout handling

---

## 5. RAG System (internal/intelligence)
- Vector store initialization
- Document ingestion (scan plus API)
- Embedding generation (OpenAI)
- Similar document retrieval
- Context enhancement in AI analysis
- MTTR reduction measurable
- Handles large document collections

---

## 6. SSH System (internal/ssh)

### Authentication Methods
- SSH agent authentication
- Private key authentication (RSA, ECDSA, Ed25519)
- Password authentication
- Interactive authentication

### Connection Features
- Retry with exponential backoff
- Connection pooling
- Health monitoring
- Session management
- Command execution with timeout
- Host key verification
- Known hosts file handling

### Error Scenarios
- Connection refused
- Authentication failure
- Network timeout
- Command execution failure
- SSH agent unavailable

---

## 7. API Server (internal/api)

### Endpoints
- `GET /health` - Health check
- `GET /api/v1/health` - API health
- `POST /api/v1/jobs` - Create diagnostic job
- `GET /api/v1/jobs/:id` - Get job status
- `POST /api/v1/agents` - Register agent
- `POST /api/v1/agents/:id/heartbeat` - Agent heartbeat
- `GET /api/v1/agents` - List agents
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/apikeys` - Generate API key

### Security
- JWT authentication works
- JWT expiration enforced
- API key authentication works
- API key scopes enforced
- Rate limiting per IP (60 req per min)
- Rate limiting per user (3600 req per hour)
- CORS configuration respected
- TLS or HTTPS in production

### Middleware
- Request logging
- Recovery from panics
- Request validation
- Response headers set correctly

---

## 8. gRPC System (internal/grpc)

### Server and Client
- gRPC server starts successfully
- Client connection established
- Request and response serialization
- Streaming RPCs work
- Concurrent requests handled

### Security
- mTLS enabled and working
- Certificate validation
- JWT interceptor authentication
- Token expiration handled

### Handlers
- Diagnostics handler
- Agent registration handler
- Health check handler

---

## 9. Notifications (4 Providers)

### Providers
- Slack: Webhook delivery, rich attachments
- Telegram: Bot API works, Markdown formatting
- Webhook: Generic webhook (Discord, Teams, Mattermost)
- Email: SMTP with TLS, HTML templates

### Notification Triggers
- Critical severity findings
- Remediation approval requests
- Agent offline alerts
- Successful remediation completion

---

## 10. Remediation System (internal/remediation)

### Actions
- Disk: Clean temp files, rotate logs, clear cache
- Service: Restart services
- Process: Kill processes
- Security: Apply patches, harden SSH config

### Workflow
- Action suggestion from diagnostics
- Human approval flow
- Action execution
- Audit logging
- Rollback on failure
- Idempotency

---

## 11. Database (PostgreSQL and Redis)

### PostgreSQL
- Connection pooling works
- Health monitoring active
- Migrations run successfully
- Models CRUD operations
- Repository layer works
- Connection recovery after failure

### Redis
- Connection established
- Caching works
- TTL expiration
- Cache invalidation

---

## 12. Configuration System

### Hierarchy
- `--config` flag takes precedence
- `./config.yaml` loaded if no flag
- `~/.lumo/config.yaml` as fallback
- Environment variables override config
- API keys only via environment variables

### Validation
- Required fields validated
- Invalid values rejected
- JWT secret length enforced (32+ chars)
- Provider-specific validation

---

## 13. Deployment Scenarios

### Kubernetes DaemonSet
- Deploys to all nodes
- hostNetwork and hostPID access
- RBAC permissions work
- Pod health probes respond
- Metrics scraped by Prometheus
- Node diagnostics work

### Kubernetes Deployment
- Multi-replica HA works
- Cluster-level diagnostics
- Service discovery
- ConfigMap or Secret mounting

### Helm Chart
- Chart installs successfully
- Values customization works
- Upgrade or rollback works

### systemd (VM)
- Service installs correctly
- Service starts on boot
- Non-root execution
- Capabilities (CAP_NET_RAW)
- Log rotation
- RPM or DEB packaging

---

## 14. Security Testing

### Authentication and Authorization
- Unauthenticated requests rejected
- Expired JWT tokens rejected
- Invalid API keys rejected
- Scope enforcement works

### Command Injection Protection
- Remediation actions sanitized
- Shell metacharacters filtered
- Path traversal prevented

### Secrets Management
- API keys not in config files
- Logs don't expose secrets
- File permissions correct (600)

### Rate Limiting
- Per-IP limits enforced
- Per-user limits enforced
- Burst allowance works
- 429 responses returned

---

## 15. Performance and Load Testing
- 100+ concurrent diagnostic requests
- 1000+ agents reporting simultaneously
- Database connection pool under load
- Redis cache performance
- Memory usage within bounds (64 to 128 MB for agent)
- CPU usage reasonable (<5 percent average for agent)
- API response times acceptable (<200 ms p95)

---

## 16. Error Handling and Edge Cases

### Network Failures
- SSH connection timeout
- API server unreachable
- Database connection lost
- Redis unavailable
- AI provider timeout

### Invalid Input
- Malformed config files
- Invalid SSH credentials
- Non-existent hosts
- Invalid checker names

### Resource Exhaustion
- Disk full scenarios
- Memory pressure
- Connection pool exhaustion
- Rate limit exceeded

---

## 17. Observability
- Structured logging works
- Log levels configurable
- Prometheus metrics exposed
- Health check endpoints respond
- Version information available
- Audit trails created

---

## 18. Cross-Platform Testing
- Linux (systemd, various distros)
- macOS (local execution)
- Different architectures (amd64, arm64)

---

## 19. Upgrade and Migration
- Database migrations run successfully
- Config backward compatibility
- Agent version mismatch handling
- Rolling updates in Kubernetes

---

## 20. Documentation Validation
- README examples work
- Getting started guide accurate
- API documentation matches implementation
- Example configs valid

---

## 📊 Testing Priorities

### 🔴 Critical (Must Test)
- Core diagnostic checkers (12)
- SSH authentication all methods
- API authentication and rate limiting
- Agent registration and heartbeat
- Remediation approval workflow

### 🟡 Important (Should Test)
- AI provider integration (5 providers)
- Notification delivery
- Database operations
- Kubernetes deployment
- systemd deployment

### 🟢 Nice to Have (Could Test)
- RAG system performance
- Load testing
- Chaos engineering
- Security penetration testing

---

## Current Status
65 percent test coverage with 619+ test functions.  
Main gaps: API integration tests (DB-dependent), RAG system, and agent reporter.



