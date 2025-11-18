# CLAUDE.md - AI Assistant Guide for Lumo

> **Last Updated:** 2025-11-18 | **Version:** 0.7.1
> **Status:** Phases 1-6 Complete | Phase 7 (API Server) 90% Complete | 84 Go files, 50.4% test coverage

**For detailed examples and tutorials, see [DEVELOPMENT.md](DEVELOPMENT.md)**

---

## Project Overview

**Lumo** - Intelligent SRE/DevOps automation platform in Go:
- **Execution Models:** SSH connectivity, local execution, native agent deployment
- **Agent Architecture:** K8s DaemonSet/Deployment + VM systemd daemons (Phases 7-11)
- System diagnostics (12 checkers: core, security, specialized)
- AI analysis (5 providers: Anthropic, OpenAI, Ollama, Gemini, OpenRouter)
- Auto-remediation with human-in-the-loop approval
- Multiple output formats (text, JSON, TOON)
- TOON format for 30-60% AI token reduction

**Tech:** Go 1.25.4 | Module: `github.com/ignacio/lumo` | License: MIT

**Architecture Modes:**
- **CLI Mode (Current):** Pull model - CLI connects to targets via SSH or runs locally
- **Agent Mode (Planned):** Hybrid push/pull - agents run on infrastructure, report to API/messaging platform

---

## Codebase Structure

```
lumo/
├── cmd/
│   ├── lumo/              # CLI: main, root, connect, diagnose, fix, serve
│   └── lumo-agent/        # (Phase 8) Agent daemon: scheduler, reporter, health
├── internal/
│   ├── config/            # Configuration management (includes API, DB, Cache config)
│   ├── ssh/               # SSH client (auth, health, retry)
│   ├── diagnostics/       # Runner + 12 checkers + formatters
│   │   ├── checkers/      # CPU, Memory, Disk, Process, Service, Network
│   │   │                  # Patch, Ports, SSH Security, Auth Failures
│   │   │                  # Kubernetes, Proxmox
│   │   └── formatters/    # text, JSON, TOON
│   ├── ai/                # 5 providers: Anthropic, OpenAI, Ollama, Gemini, OpenRouter
│   ├── remediation/       # Actions, executor, approval, audit
│   ├── api/               # ✅ (Phase 7) API server - 75% complete
│   │   ├── handlers/      # diagnostics, health, jobs
│   │   ├── middleware/    # auth, logging, recovery, cors
│   │   ├── response/      # response utilities
│   │   ├── router.go      # Chi router with routes
│   │   └── server.go      # HTTP server with graceful shutdown
│   ├── database/          # ✅ (Phase 7) PostgreSQL integration
│   │   ├── models/        # Job, APIKey models
│   │   ├── repository/    # Job, APIKey repositories
│   │   ├── migrations/    # SQL migration files
│   │   ├── migrations.go  # Goose migration runner
│   │   └── postgres.go    # DB connection pool
│   ├── cache/             # ✅ (Phase 7) Redis client
│   │   └── redis.go       # Redis operations
│   ├── agent/             # (Phase 8) Agent logic, scheduling, caching
│   └── messaging/         # (Phase 11) Pub/sub: NATS, Kafka, RabbitMQ, Redis
├── deployments/
│   ├── kubernetes/        # (Phase 9) DaemonSet, Deployment, RBAC, Helm
│   └── systemd/           # (Phase 10) Service units, install scripts, packages
├── configs/config.example.yaml  # Updated with DB and Cache sections
└── docker-compose.yaml    # ✅ PostgreSQL + Redis for development

Total: 84 Go files (54 + 30 new) + 35 test files | Test Coverage: 50.4%
Phase 7: +4,336 LOC across 30 files (jobs, api_keys, agents systems)
```

---

## Technology Stack

**Core:** cobra (CLI), viper (config), logrus (logging), x/crypto/ssh, backoff (retry)
**Kubernetes:** k8s.io/client-go v0.31.3 (native client, no kubectl)
**AI:** Custom HTTP clients for all 5 providers (no external SDKs)
**API Server (Phase 7):** Chi router v5, PostgreSQL (lib/pq), Redis (go-redis/v9), goose migrations v3
**Data:** JSONB for flexible storage, UUID for primary keys, repository pattern

---

## Configuration System

**Hierarchy:** `--config` flag → `./config.yaml` → `~/.lumo/config.yaml`
**Env Pattern:** `LUMO_<SECTION>_<KEY>`

### Key Environment Variables

```bash
# AI Provider (anthropic|openai|ollama|gemini|openrouter)
export LUMO_AI_PROVIDER=anthropic

# API Keys (provider-specific recommended)
export LUMO_ANTHROPIC_API_KEY=sk-ant-...
export LUMO_OPENAI_API_KEY=sk-...
export LUMO_GEMINI_API_KEY=...
export LUMO_OPENROUTER_API_KEY=sk-or-...
export LUMO_AI_API_KEY=sk-...  # Generic fallback

# OpenAI reasoning models (o1, o3, gpt-5-nano)
export LUMO_AI_REASONING_EFFORT=medium  # low|medium|high
```

**Config Sections:** SSH, AI, Logging, API, Diagnostics, Agent (Phase 8)
**Loading:** `cfg, err := config.Load()` - searches hierarchy, validates automatically

**Security:** API keys ONLY via env vars, NEVER in config files

### Agent Configuration (Phase 8)

```bash
# Agent operational settings
export LUMO_AGENT_MODE=hybrid                  # scheduled|on-demand|continuous|hybrid
export LUMO_AGENT_SCHEDULE="*/5 * * * *"       # Cron expression
export LUMO_AGENT_API_ENDPOINT=https://lumo-api.example.com
export LUMO_AGENT_TOKEN=jwt-token-here         # JWT authentication
export LUMO_AGENT_MESSAGING_ENABLED=true       # Enable messaging (Phase 11)
export LUMO_AGENT_MESSAGING_PROVIDER=nats      # nats|kafka|rabbitmq|redis
```

---

## Code Organization

**Packages:** `cmd/` (CLI), `internal/` (private logic), `pkg/` (future public libs)
**Imports:** Standard → Third-party → Internal
**Naming:** packages (lowercase), exported (PascalCase), private (camelCase)

---

## CLI Commands

**Global Flags:** `--config`, `--verbose`/`-v`, `--dry-run`

### lumo CLI (Current)

| Command | Status | Purpose |
|---------|--------|---------|
| `connect` | ✅ | SSH connection |
| `diagnose` | ✅ | System diagnostics + AI analysis |
| `fix` | ✅ | Auto-remediation with approval |
| `report` | ⏳ | Report generation (planned) |
| `serve` | ⏳ | API server (Phase 7) |

### lumo-agent Daemon (Phase 8)

| Command | Status | Purpose |
|---------|--------|---------|
| `lumo-agent` | ⏳ | Agent daemon with scheduled/on-demand/continuous modes |
| `lumo-agent version` | ⏳ | Display agent version |
| `lumo-agent health` | ⏳ | Check agent health |

---

## Error Handling & Logging

**Error Wrapping:** Always use `fmt.Errorf("context: %w", err)`
**Log Levels:** Debug (verbose), Info (default), Warn, Error, Fatal
**Structured Logging:** `log.WithFields(logrus.Fields{...}).Info("msg")`
**Dry-Run:** Check flag before executing destructive operations

---

## Testing

**Current Coverage:** 50.4% (35 test files)
**Pattern:** Table-driven tests, mock executors for checkers
**Run:** `go test ./...` or `go test -cover ./...`

**Coverage by Package:**
- internal/diagnostics/formatters: 98.1%
- internal/diagnostics: 87.6%
- internal/config: 68.8%
- internal/diagnostics/checkers: 61.4%
- cmd/lumo: 61.6%
- internal/ssh: 30.5%
- internal/ai: 27.1%

---

## Security

**Audit Status:** ✅ All CRITICAL issues resolved (see `REPORTS/security-*.md`)

**Never Commit:** `config.yaml`, `*.pem`, `*.key`, `id_rsa*`, `.env*`

**Credentials:**
- API keys: Provider-specific env vars only (never in config files)
- SSH: Key-based auth or secure prompting (no password flags)
- Host key verification enabled by default
- Command injection protection: WorkingDir sanitization, shell metacharacter filtering

**File Permissions:** `chmod 600` for all config, key, and cert files

---

## Key Files & Components

**Core Files:** `cmd/lumo/root.go`, `diagnose.go`, `fix.go` | `internal/config/config.go`, `diagnostics/diagnostics.go`

**Checkers (12):**
- **Core (6):** CPU, Memory, Disk, Process, Service, Network
- **Security (4):** Patch Status, Open Ports, SSH Security, Auth Failures
- **Specialized (2):** Kubernetes (k8s native client), Proxmox VE

**AI Providers (5):** Anthropic (Claude), OpenAI (GPT), Ollama, Gemini, OpenRouter

**Remediation:** `executor.go`, `approval.go`, `audit.go`, `actions_*.go` (disk, service, process)

---

## Agent Architecture (Phases 7-11)

### Overview

Transform Lumo from **CLI pull model** (SSH to targets) to **hybrid push/pull model** (agents report to API/messaging).

**Code Reuse:** 85% of existing code reusable (diagnostics, remediation, AI, formatters)

### Agent Types

**1. Kubernetes DaemonSet** (per-node monitoring)
- Deployment: DaemonSet with `hostNetwork: true`, `hostPID: true`
- Scope: Node-level diagnostics (CPU, memory, disk, processes)
- Access: K8s API via RBAC (read-only + optional remediation)

**2. Kubernetes Deployment** (cluster-wide monitoring)
- Deployment: Standard Deployment (2+ replicas for HA)
- Scope: Cluster-level diagnostics via K8s API
- Focus: Pods, services, deployments, statefulsets, jobs

**3. VM systemd Daemon** (system-level monitoring)
- Deployment: systemd service unit
- User: Non-root with minimal capabilities (CAP_NET_RAW, CAP_SYS_PTRACE)
- Security: ProtectSystem=strict, PrivateTmp=true

### Operational Modes

- **Scheduled:** Periodic diagnostics (cron: `*/5 * * * *`)
- **On-Demand:** API-triggered diagnostics
- **Continuous:** WebSocket streaming (high-frequency checks)
- **Hybrid (Recommended):** Scheduled background + API + alerts

### Communication Patterns

```
Agents ──────────────▶ Lumo API Server (Phase 7)
  │                      │
  │                      ├─ Agent Registration
  │                      ├─ Heartbeats
  │                      ├─ Report Submission
  │                      └─ On-Demand Diagnostics
  │
  └─────────────────▶ Message Queue (Phase 11)
                         (NATS/Kafka/RabbitMQ/Redis)
```

### Security

- **Authentication:** JWT tokens + mTLS for production
- **K8s RBAC:** Least-privilege (read-only + selective remediation)
- **Encryption:** TLS 1.3 for all communication
- **Secrets:** K8s Secrets API or external (Vault, AWS Secrets Manager)

### Resource Footprint (Target)

- **Memory:** 64-128 MB baseline, 256 MB peak
- **CPU:** < 5% average, 50% peak during diagnostics
- **Disk:** 100 MB binary, 1 GB cache
- **Network:** 1-10 KB/s average, 100 KB/s peak

### Configuration

```yaml
agent:
  mode: hybrid                  # scheduled|on-demand|continuous|hybrid
  schedule: "*/5 * * * *"       # Cron expression
  api_endpoint: https://lumo-api.example.com
  token: ""                     # JWT (via env var)
  tls_enabled: true
  enabled_checks: [cpu, memory, disk, process, service, network]
  report_format: toon           # 30-60% token reduction
  offline_mode: true            # Continue if API unavailable
  health_check_port: 8080
  metrics_port: 9090            # Prometheus metrics

  kubernetes:
    enabled: true
    scope: node                 # node|cluster
```

### Deployment Examples

**Kubernetes DaemonSet:**
```bash
kubectl apply -f deployments/kubernetes/daemonset.yaml
# or
helm install lumo-agent deployments/kubernetes/helm/lumo-agent
```

**VM systemd:**
```bash
./deployments/systemd/install.sh
systemctl enable --now lumo-agent
```

---

## Phase Roadmap

### ✅ Completed Phases (1-6)

**Phase 1-2:** Foundation (Cobra CLI, Viper config, Logrus logging) + SSH (4 auth methods, retry logic, health monitoring)

**Phase 3:** Diagnostic System - Core runner with `Checker` interface, 6 core checkers (CPU, Memory, Disk, Process, Service, Network), cross-platform support

**Phase 4:** AI Integration - 5 providers (Anthropic, OpenAI, Ollama, Gemini, OpenRouter), streaming, TOON format support

**Phase 4.1:** Enhanced Diagnostics - Memory enhancements (top consumers, page faults, pressure indicators), cross-platform metrics

**Phase 4.2:** OpenRouter Integration - Added 5th AI provider with multi-model support, unified API access to multiple LLM providers

**Phase 5:** Security Diagnostics - 4 checkers (Patch Status, Open Ports, SSH Security, Auth Failures)

**Phase 5.1:** Specialized Checkers - Kubernetes (native client, 8 resource types), Proxmox VE (cluster, VMs, storage, HA)

**Phase 6:** Auto-Remediation - Action framework, human-in-the-loop approval, risk classification (safe/moderate/critical), audit logging, actions for disk/service/process management

### 🚧 In Progress - Agent Deployment (16 weeks)

**Phase 7: API Server Foundation** (Weeks 1-3) - **90% COMPLETE** ✅
- ✅ REST API server (`internal/api/`) with Chi router
- ✅ PostgreSQL database + Redis cache integration
- ✅ Database migrations (goose) - 3 migrations (jobs, api_keys, agents)
- ✅ API key authentication with scope-based authorization
- ✅ Health endpoints (`/health`, `/ready`, `/live`)
- ✅ Job management endpoints (create, list, get, delete)
- ✅ Diagnostic endpoint (POST /api/v1/diagnostics) with async execution
- ✅ Agent registration system (register, heartbeat, list, get, delete, stats)
- ✅ Repository pattern for database operations (Job, APIKey, Agent)
- ✅ docker-compose.yaml for local development
- ⏳ Integration tests (pending)
- ⏳ JWT authentication (optional - can defer to Phase 12)
- ⏳ mTLS support (optional - deferred to Phase 12)
- ⏳ WebSocket support (optional - deferred to Phase 13)
- ⏳ OpenAPI/Swagger documentation (pending)
- **Status:** 30 files (+4), 4,336 LOC (+939) | Phase 8 ready to start!

**Phase 8: Agent Daemon** (Weeks 4-6)
- Agent daemon binary (`cmd/lumo-agent`)
- Scheduler for periodic diagnostics (cron-based)
- API reporter with retry/backoff
- Local result caching (offline mode)
- Health check endpoints (`:8080/health`)
- Prometheus metrics (`:9090/metrics`)
- **Deliverables:** `cmd/lumo-agent/`, `internal/agent/{agent,config,reporter,scheduler,cache,healthcheck}.go`

**Phase 9: Kubernetes Deployment** (Weeks 7-8)
- DaemonSet manifest (per-node monitoring)
- Deployment manifest (cluster-wide monitoring)
- RBAC configuration (ServiceAccount, ClusterRole, ClusterRoleBinding)
- ConfigMap and Secret templates
- Service and NetworkPolicy
- Helm chart with values customization
- **Deliverables:** `deployments/kubernetes/{daemonset,deployment,rbac,configmap,secret,service,networkpolicy}.yaml`, Helm chart

**Phase 10: VM Deployment** (Weeks 9-10)
- systemd service unit (`lumo-agent.service`)
- Installation/uninstallation scripts
- RPM package (CentOS/RHEL/Fedora)
- DEB package (Ubuntu/Debian)
- Binary releases (Linux, macOS, Windows)
- **Deliverables:** `deployments/systemd/{lumo-agent.service,install.sh,uninstall.sh}`, packaging specs

**Phase 11: Messaging Integration** (Weeks 11-12)
- Messaging publisher/subscriber (`internal/messaging`)
- Provider implementations: NATS, Kafka, RabbitMQ, Redis
- Topic-based routing (diagnostics, remediation, alerts, lifecycle)
- Message serialization (JSON) and compression
- Retry and dead-letter handling
- **Deliverables:** `internal/messaging/{publisher,subscriber,providers/}.go`

**Phase 12: Security Hardening** (Weeks 13-14)
- mTLS implementation and testing
- Certificate rotation mechanisms
- Security audit (OWASP Top 10, CWE)
- Penetration testing (agent, API server)
- Secrets management integration (Vault, AWS Secrets Manager)
- **Deliverables:** Security audit report, mTLS implementation, cert management scripts

**Phase 13: Production Readiness** (Weeks 15-16)
- Performance optimization (profiling, benchmarking)
- Grafana dashboards (agent health, diagnostics, API metrics)
- Prometheus alerting rules
- Operator runbooks (deployment, troubleshooting, upgrades)
- Complete documentation (user guides, API reference, migration path)
- Load testing (1000+ agents, 10K+ req/min)
- **Deliverables:** Dashboards, runbooks, documentation, performance report

### ⏳ Future Phases

**Phase 14: Advanced Reporting**
- Multiple formats (Markdown, JSON, YAML, HTML, PDF)
- Historical data storage and querying
- Trend analysis and forecasting
- Report scheduling and distribution

**Phase 15: Testing & Quality**
- Target 70%+ test coverage (currently 50.4%)
- Priorities: internal/ai HTTP tests, internal/ssh client tests, checker edge cases
- Integration tests for agent workflows
- Chaos engineering tests (network partitions, failures)

**Phase 16: Advanced Features**
- Multi-cluster management
- Predictive analysis (ML-based anomaly detection)
- Policy-as-code for remediation
- Incident management integrations (PagerDuty, Opsgenie)

---

## Localhost Execution

**Auto-detection:** `diagnose` command detects localhost patterns (`localhost`, `127.0.0.1`, `::1`, `0.0.0.0`) and runs commands directly without SSH overhead using `LocalExecutor`

---

## TOON Format

**TOON (Token-Oriented Object Notation)** - LLM-optimized format reducing AI token costs by 30-60% vs JSON

**How it Works:**
- Uniform arrays use tabular notation: `array[count]{field1,field2}:`
- Rows are comma-separated values
- Non-uniform data uses YAML-like `key: value`

**Efficiency:** 33% on typical reports, 42% on process/service lists, 58% on metrics arrays

**Usage:**
- `--format toon` for TOON output
- `--analyze` automatically uses TOON internally for AI (user still sees text/JSON)
- Best for: process lists, service status, metrics arrays

**Implementation:** `formatters.NewToonFormatter()` | Uses `github.com/alpkeskin/gotoon`

---

## AI Assistant Best Practices

**DO:**
- Use existing patterns, wrap errors (`fmt.Errorf("context: %w", err)`), write tests
- Document exported functions, respect `dryRun` flag, update CLAUDE.md when completing phases

**DON'T:**
- Commit secrets, ignore errors, use excessive global state, skip validation, hardcode paths

**Code Review:** Go conventions, error wrapping, tests, docs updated, `go fmt`, `go vet`

---

## Quick Reference

### CLI Mode (Current)

```bash
# Build & Test
go build -o lumo ./cmd/lumo
go test ./... && go vet ./... && go fmt ./...

# Usage
lumo diagnose localhost --analyze --format toon
lumo fix localhost --dry-run
LUMO_ANTHROPIC_API_KEY=sk-ant-... lumo diagnose --analyze

# Key Patterns
cfg, err := config.Load()                      # Config
return fmt.Errorf("context: %w", err)          # Errors
log.WithFields(logrus.Fields{...}).Info()     # Logging
formatter := formatters.NewToonFormatter()     # TOON
builder := ai.NewPromptBuilder()               # AI (TOON enabled)
```

### API Server Mode (Phase 7 - 75% Complete)

```bash
# Start development environment (PostgreSQL + Redis)
docker-compose up -d

# Run API server
lumo serve --config configs/config.example.yaml

# API Usage Examples
# (Requires API key in database - see phase-9-api-server-planning.md)

# Health check
curl http://localhost:8080/api/v1/health

# Run diagnostics
curl -X POST http://localhost:8080/api/v1/diagnostics \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"target": "localhost", "checks": ["cpu", "memory"]}'

# Get job status
curl http://localhost:8080/api/v1/jobs/{job-id} \
  -H "X-API-Key: your-api-key"

# List jobs
curl http://localhost:8080/api/v1/jobs?status=completed&limit=10 \
  -H "X-API-Key: your-api-key"

# Agent registration
curl -X POST http://localhost:8080/api/v1/agents/register \
  -H "X-API-Key: your-api-key" \
  -d '{"name":"agent-01","hostname":"node01","platform":"linux","architecture":"amd64","version":"1.0.0","capabilities":["cpu","memory","disk"]}'

# Agent heartbeat
curl -X PUT http://localhost:8080/api/v1/agents/{agent-id}/heartbeat \
  -H "X-API-Key: your-api-key"

# List agents
curl http://localhost:8080/api/v1/agents?status=online \
  -H "X-API-Key: your-api-key"

# Agent stats
curl http://localhost:8080/api/v1/agents/stats \
  -H "X-API-Key: your-api-key"

# Database migrations (automatic on server start)
# Or manually: go run internal/database/migrations.go up
```

### Agent Mode (Phases 7-11)

```bash
# Build Agent
go build -o lumo-agent ./cmd/lumo-agent

# Deploy to Kubernetes
kubectl apply -f deployments/kubernetes/daemonset.yaml
helm install lumo-agent deployments/kubernetes/helm/lumo-agent

# Deploy to VM
./deployments/systemd/install.sh
systemctl enable --now lumo-agent
systemctl status lumo-agent

# Agent Operations
curl http://localhost:8080/health              # Health check
curl http://localhost:9090/metrics             # Prometheus metrics
journalctl -u lumo-agent -f                    # View logs (systemd)
kubectl logs -f daemonset/lumo-agent           # View logs (K8s)

# API Server (Phase 7)
lumo serve --port 8443 --tls-enabled
curl -H "Authorization: Bearer $TOKEN" https://lumo-api/v1/agents

# Agent Configuration
export LUMO_AGENT_MODE=hybrid
export LUMO_AGENT_SCHEDULE="*/5 * * * *"
export LUMO_AGENT_API_ENDPOINT=https://lumo-api.example.com
export LUMO_AGENT_TOKEN=$JWT_TOKEN
```

---

**For questions/improvements:** https://github.com/ignacio/lumo/issues
