# CLAUDE.md - AI Assistant Guide for Lumo

> **Last Updated:** 2025-11-18 | **Version:** 1.1.0
> **Status:** Phases 1-10 Complete ✅ | Notification Integration Complete ✅ | K8s + VM Deployment Ready 🚀 | 50.4% test coverage

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
- **CLI Mode:** Pull model - CLI connects to targets via SSH or runs locally
- **Agent Mode:** ✅ Hybrid push/pull - agents run on infrastructure, report to API/messaging platform (Phase 8 complete)

---

## Codebase Structure

```
lumo/
├── cmd/
│   ├── lumo/              # CLI: main, root, connect, diagnose, fix, serve
│   └── lumo-agent/        # ✅ Agent daemon: scheduler, reporter, health, metrics
├── internal/
│   ├── config/            # Configuration management (includes API, DB, Cache, Notifications)
│   ├── ssh/               # SSH client (auth, health, retry)
│   ├── diagnostics/       # Runner + 12 checkers + formatters
│   │   ├── checkers/      # CPU, Memory, Disk, Process, Service, Network
│   │   │                  # Patch, Ports, SSH Security, Auth Failures
│   │   │                  # Kubernetes, Proxmox
│   │   └── formatters/    # text, JSON, TOON
│   ├── ai/                # 5 providers: Anthropic, OpenAI, Ollama, Gemini, OpenRouter
│   ├── remediation/       # Actions, executor, approval, audit
│   ├── notifications/     # ✅ Notification system - 5 providers
│   │   ├── providers/     # Slack, Teams, Telegram, Email, Webhook
│   │   ├── message.go     # Message builder and types
│   │   ├── notifier.go    # Routing and concurrent delivery
│   │   └── provider.go    # Provider interface
│   ├── api/               # ✅ (Phase 7) API server - 100% complete
│   │   ├── handlers/      # diagnostics, health, jobs, agents
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
│   ├── agent/             # ✅ Agent logic, scheduling, caching, reporter
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

### lumo CLI

| Command | Status | Purpose |
|---------|--------|---------|
| `connect` | ✅ | SSH connection |
| `diagnose` | ✅ | System diagnostics + AI analysis |
| `fix` | ✅ | Auto-remediation with approval |
| `serve` | ✅ | API server (Phase 7) |
| `report` | ⏳ | Report generation (planned) |

### lumo-agent Daemon

| Command | Status | Purpose |
|---------|--------|---------|
| `lumo-agent` | ✅ | Agent daemon with hybrid scheduled/on-demand/continuous modes |
| `lumo-agent version` | ✅ | Display agent version |
| `lumo-agent health` | ✅ | Check agent health status |

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
- Notification credentials: Environment variables only (LUMO_SLACK_WEBHOOK_URL, LUMO_TELEGRAM_BOT_TOKEN, etc.)

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

**Notification Providers (5):** Slack, Microsoft Teams, Telegram, Email/SMTP, Generic Webhook

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

### ✅ Completed Phases (1-8)

**Phase 1-2:** Foundation (Cobra CLI, Viper config, Logrus logging) + SSH (4 auth methods, retry logic, health monitoring)

**Phase 3:** Diagnostic System - Core runner with `Checker` interface, 6 core checkers (CPU, Memory, Disk, Process, Service, Network), cross-platform support

**Phase 4:** AI Integration - 5 providers (Anthropic, OpenAI, Ollama, Gemini, OpenRouter), streaming, TOON format support

**Phase 4.1:** Enhanced Diagnostics - Memory enhancements (top consumers, page faults, pressure indicators), cross-platform metrics

**Phase 4.2:** OpenRouter Integration - Added 5th AI provider with multi-model support, unified API access to multiple LLM providers

**Phase 5:** Security Diagnostics - 4 checkers (Patch Status, Open Ports, SSH Security, Auth Failures)

**Phase 5.1:** Specialized Checkers - Kubernetes (native client, 8 resource types), Proxmox VE (cluster, VMs, storage, HA)

**Phase 6:** Auto-Remediation - Action framework, human-in-the-loop approval, risk classification (safe/moderate/critical), audit logging, actions for disk/service/process management

### ✅ Completed - Phase 7: API Server Foundation

**Phase 7: API Server Foundation** (Weeks 1-3) - **100% COMPLETE** ✅
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
- ✅ Integration tests (api_integration_test.go) - skipped pending full DB setup
- ✅ OpenAPI 3.0 specification (api/openapi.yaml)
- ✅ API documentation (api/README.md)
- ✅ Comprehensive testing guide (`REPORTS/phase-7-testing-guide/`)
  - Complete testing documentation (TESTING_GUIDE.md)
  - Automated test script (test-agent-api.sh)
  - API reference (API_REFERENCE.md)
  - OpenAPI 3.0 spec (openapi.yaml)
- ✅ GitHub CI passing (all checks green)
  - Fixed go.mod dependencies
  - Fixed code formatting (gofmt)
  - Fixed config tests
  - All tests passing
- ⏳ JWT authentication (deferred to Phase 12 - Security Hardening)
- ⏳ mTLS support (deferred to Phase 12 - Security Hardening)
- ⏳ WebSocket support (deferred to Phase 13 - Production Readiness)
- **Status:** 35+ files, 4,800+ LOC | **Phase 8 complete!** ✅

### ✅ Completed - Phase 8: Agent Daemon (Weeks 4-6)

**Phase 8: Agent Daemon** - **100% COMPLETE** ✅
- ✅ Agent daemon binary (`cmd/lumo-agent`)
- ✅ Agent configuration in `internal/config/config.go` (AgentConfig struct)
- ✅ Scheduler for periodic diagnostics (cron-based via `robfig/cron/v3`)
- ✅ API reporter with retry/backoff (exponential backoff, 4 attempts)
- ✅ Local result caching (offline mode support)
- ✅ Health check HTTP endpoints (`:8080/health`, `/ready`, `/live`, `/status`)
- ✅ Prometheus metrics (`:9090/metrics` with 12+ metrics)
- ✅ Multiple operational modes: scheduled, on-demand, continuous, hybrid
- ✅ Agent registration with API server
- ✅ Heartbeat system (30s intervals)
- ✅ Graceful shutdown handling
- ✅ Version and health commands
- ✅ Comprehensive tests (cache_test.go, scheduler_test.go)
- **Deliverables:**
  - `cmd/lumo-agent/{main.go,root.go,version.go,health.go}`
  - `internal/agent/{agent.go,reporter.go,scheduler.go,cache.go,healthcheck.go,metrics.go}`
  - Binary size: 65MB
  - All tests passing ✅

### ✅ Completed - Phase 9: Kubernetes Deployment (Weeks 7-8)

**Phase 9: Kubernetes Deployment** - **100% COMPLETE** ✅
- ✅ DaemonSet manifest (per-node monitoring with hostNetwork, hostPID)
- ✅ Deployment manifest (cluster-wide monitoring, 2 replicas for HA)
- ✅ RBAC configuration (ServiceAccount, ClusterRole, ClusterRoleBinding)
- ✅ ConfigMap templates for agent configuration
- ✅ Secret templates with external secret manager examples
- ✅ Service manifests (headless for DaemonSet, ClusterIP for Deployment)
- ✅ NetworkPolicy for security controls (ingress/egress rules)
- ✅ Helm chart with comprehensive customization
- ✅ Kustomize base and overlay structure
- ✅ ServiceMonitor for Prometheus Operator
- ✅ Comprehensive deployment README
- **Deliverables:**
  - `deployments/kubernetes/base/{daemonset,deployment,rbac,configmap,secret,service,networkpolicy,kustomization}.yaml` (8 manifests)
  - `deployments/kubernetes/helm/lumo-agent/` (Chart.yaml, values.yaml, templates/, .helmignore)
  - `deployments/kubernetes/README.md` (complete usage guide)
  - 18 total files created
  - All manifests follow K8s best practices ✅

### ✅ Completed - Phase 10: VM Deployment (Weeks 9-10)

**Phase 10: VM Deployment** - **100% COMPLETE** ✅
- ✅ systemd service unit with comprehensive security hardening
  - `ProtectSystem=strict`, `PrivateTmp=true`, `NoNewPrivileges=true`
  - Minimal capabilities: `CAP_NET_RAW`, `CAP_SYS_PTRACE`, `CAP_DAC_READ_SEARCH`
  - System call filtering, resource limits (512M memory, 50% CPU)
- ✅ Installation script (`install.sh`) with auto-detection and flexible options
- ✅ Uninstallation script (`uninstall.sh`) with purge/keep options
- ✅ RPM package specification for RHEL/CentOS/Fedora
  - Complete spec file with pre/post install scripts
  - User/group management, systemd integration
  - Build script (`build-rpm.sh`)
- ✅ DEB package files for Ubuntu/Debian
  - debian/control, postinst, prerm, postrm, rules, changelog
  - debhelper integration, systemd service management
  - Build script (`build-deb.sh`)
- ✅ Cross-platform build script (`build.sh`)
  - Supports 6 platforms: linux (amd64/arm64/arm), darwin (amd64/arm64), windows (amd64)
  - Checksums generation (SHA256, MD5)
  - Package building integration
- ✅ Comprehensive README documentation
  - Installation methods (script, RPM, DEB, manual)
  - Configuration and operational modes
  - Service management, health checks, metrics
  - Security hardening details
  - Troubleshooting and uninstallation
- **Deliverables:**
  - `deployments/systemd/lumo-agent.service` (hardened systemd unit)
  - `deployments/systemd/{install.sh,uninstall.sh,build.sh}` (3 scripts)
  - `deployments/systemd/packaging/rpm/{lumo-agent.spec,build-rpm.sh}` (RPM)
  - `deployments/systemd/packaging/deb/debian/{control,postinst,prerm,postrm,rules,changelog,compat}` + `build-deb.sh` (DEB)
  - `deployments/systemd/README.md` (comprehensive guide)
  - 15 total files created
  - All scripts tested and documented ✅

### 🚧 In Progress - Agent Deployment (Weeks 11-16)

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

## Notification System

**Overview:** Multi-provider notification system for sending alerts about diagnostic results, remediation actions, and system events.

**Architecture:** Provider interface with severity-based routing, concurrent delivery, and retry logic with exponential backoff.

**Providers (5):**
1. **Slack** - Rich Block Kit formatting, severity color coding, channel routing
2. **Microsoft Teams** - Adaptive Cards with theme colors and facts
3. **Telegram** - Bot API with Markdown formatting, silent mode for low-priority
4. **Email** - SMTP with HTML/plain text multipart, template-based formatting
5. **Webhook** - Generic HTTP endpoint for custom integrations

**Key Features:**
- **Severity Routing:** Map severity levels (critical, error, warning, info, ok) to specific providers
- **Concurrent Delivery:** Send to multiple providers in parallel for speed
- **Retry Logic:** Exponential backoff with 3 retry attempts (2s, 4s, 8s delays)
- **Message Builder:** Fluent API for constructing notification messages
- **Rich Formatting:** Provider-specific formatting (Slack blocks, Teams cards, Telegram Markdown, HTML email)

**Configuration:**
```yaml
notifications:
  enabled: true
  routing:
    critical: [slack, email]  # Critical alerts go to Slack and Email
    error: [slack]             # Errors go to Slack only
    warning: [slack]           # Warnings go to Slack
  providers:
    slack:
      enabled: true
      webhook_url: env:LUMO_SLACK_WEBHOOK_URL
      channel: "#lumo-alerts"
```

**Environment Variables:**
```bash
# Slack
export LUMO_SLACK_WEBHOOK_URL=https://hooks.slack.com/services/...

# Teams
export LUMO_TEAMS_WEBHOOK_URL=https://outlook.office.com/webhook/...

# Telegram
export LUMO_TELEGRAM_BOT_TOKEN=123456:ABC-DEF...
export LUMO_TELEGRAM_CHAT_ID=-1001234567890

# Email
export LUMO_EMAIL_FROM=lumo@example.com
export LUMO_EMAIL_AUTH_USERNAME=user@example.com
export LUMO_EMAIL_AUTH_PASSWORD=app-password

# Webhook
export LUMO_WEBHOOK_URL=https://your-endpoint.com/webhook
```

**Usage (Future CLI Integration):**
```bash
# Test notification configuration
lumo notify test --provider slack

# Send custom notification
lumo notify send --provider slack --title "Test" --body "Message"

# Run diagnostics with notifications
lumo diagnose localhost --notify slack --notify-on critical,error
```

**Implementation Files:**
- `internal/notifications/provider.go` - Provider interface and types
- `internal/notifications/message.go` - Message builder with fluent API
- `internal/notifications/notifier.go` - Router and concurrent delivery
- `internal/notifications/providers/slack.go` - Slack Block Kit implementation
- `internal/notifications/providers/teams.go` - MS Teams Adaptive Cards
- `internal/notifications/providers/telegram.go` - Telegram Bot API
- `internal/notifications/providers/email.go` - SMTP with HTML templates
- `internal/notifications/providers/webhook.go` - Generic HTTP webhooks
- `internal/config/notifications.go` - Configuration structs and validation

**Total Code:** ~1,500 lines across 9 files

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

### API Server Mode (Phase 7 - 100% Complete) ✅

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
