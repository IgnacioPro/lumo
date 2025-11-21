# CLAUDE.md - AI Assistant Guide for Lumo

> **Last Updated:** 2025-11-21 | **Version:** 1.0.8 | **Status:** Phase 11b Complete ✅ | Phase 11c Pending ⏳ | All Core Systems Live 🚀

**Quick Links:** [Getting Started](docs/getting-started.md) | [Examples](examples/) | [Deployments](deployments/) | [API Docs](api/README.md)

---

## Contribution Guidelines

Always run `make ci` before committing (linters, security checks, tests, builds).

## Project Overview

**Lumo** - Intelligent SRE/DevOps automation platform in Go with 12 diagnostic checkers, 5 AI providers, auto-remediation, multi-platform notifications, and agent architecture (K8s + VM).

**Key Features:**
- System diagnostics (6 core + 4 security + 2 specialized checkers)
- AI analysis: Anthropic, OpenAI, Ollama, Gemini, OpenRouter (adapter pattern)
- Auto-remediation with human approval
- RAG system (87% MTTR reduction, 4,400% ROI)
- Multi-platform notifications: Slack, Telegram, Discord, Teams, Email
- TOON format (30-60% token reduction for AI analysis)
- CLI mode (SSH pull), Agent mode (K8s DaemonSet/Deployment + VM systemd)

**Tech:** Go 1.25.4 | PostgreSQL + Redis | gRPC + Protocol Buffers | Chromem-go RAG

---

## Codebase Structure

```
lumo/
├── cmd/
│   ├── lumo/                          # CLI: init, doctor, diagnose, fix, serve, examples
│   └── lumo-agent/                    # Agent daemon: scheduler, reporter, health, metrics
├── internal/
│   ├── config/                        # Configuration + env var hierarchy
│   ├── version/                       # Centralized version management (CLI + Agent)
│   ├── ssh/                           # SSH client (4 auth methods, retry)
│   ├── diagnostics/                   # Runner, 12 checkers, formatters (text/TOON/JSON)
│   ├── ai/                            # Adapter pattern: 5 providers + HTTP/streaming
│   ├── remediation/                   # Actions, executor, approval, audit
│   ├── notifications/                 # 4 providers: Slack, Telegram, Webhook, Email
│   ├── api/                           # REST server (Chi router, health, jobs, agents)
│   ├── database/                      # PostgreSQL: models, repos, migrations (goose)
│   ├── cache/                         # Redis client
│   ├── agent/                         # Scheduling, reporting, caching, health
│   ├── grpc/                          # gRPC server/client, handlers, interceptors, mTLS
│   ├── intelligence/                  # RAG: vectorstore, embeddings, ingestion
│   ├── doctor/                        # Health check system (6 checks)
│   └── messaging/                     # Pub/sub framework (pending: NATS, Kafka, RabbitMQ, Redis)
├── deployments/
│   ├── kubernetes/                    # DaemonSet, Deployment, RBAC, Helm, kind
│   └── systemd/                       # Service unit, install scripts, RPM/DEB packaging
├── examples/                          # 6 end-to-end examples (3,200+ LOC)
├── docs/                              # Getting started, competitive analysis, ROI, investor materials
├── configs/                           # Example configurations
└── docker-compose.yaml                # PostgreSQL + Redis for development

Total: 192 Go files + 70 test files | Coverage: 66.7% | Verified: 2025-11-21
```

---

## Technology Stack

**Core:** cobra, viper, logrus, x/crypto/ssh, backoff
**Database:** PostgreSQL (lib/pq), Redis (go-redis/v9), goose migrations
**AI:** Adapter pattern, custom HTTP client (no external SDKs), 5 providers
**API:** Chi router v5, JWT auth, rate limiting, request validation
**gRPC:** google.golang.org/grpc, Protocol Buffers, mTLS, JWT interceptors
**RAG:** chromem-go (local vector store), OpenAI embeddings
**Kubernetes:** k8s.io/client-go (native, no kubectl)
**Agent:** robfig/cron/v3 (scheduling), Prometheus client (metrics)
**Notifications:** 4 providers (Slack, Telegram, Webhook, Email)

---

## Configuration System

**Hierarchy:** `--config` flag → `./config.yaml` → `~/.lumo/config.yaml`
**Loading:** `cfg, err := config.Load()` (searches hierarchy, auto-validates)
**Security:** API keys ONLY via env vars (LUMO_*_API_KEY), never in config files

**Key Environment Variables:**
```bash
export LUMO_AI_PROVIDER=anthropic                    # anthropic|openai|ollama|gemini|openrouter
export LUMO_ANTHROPIC_API_KEY=sk-ant-...            # Provider-specific keys
export LUMO_RAG_ENABLED=true
export LUMO_AGENT_MODE=hybrid                       # scheduled|on-demand|continuous|hybrid
export LUMO_AGENT_API_ENDPOINT=https://lumo-api...
export LUMO_AGENT_TOKEN=$JWT_TOKEN
export LUMO_API_JWT_SECRET=secret-key               # Production required
export LUMO_DATABASE_PASSWORD=password              # DB password
```

**Config Options:**
- `api.allowed_origins`: CORS-allowed origins (configurable per environment, not hardcoded)

See [configs/config.example.yaml](configs/config.example.yaml) and [configs/notifications.example.yaml](configs/notifications.example.yaml) for complete options.

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
| `init` | ✅ | Interactive setup wizard (Usability Week 1) |
| `doctor` | ✅ | Validate configuration and dependencies (Usability Week 2) |
| `examples` | ✅ | Show usage examples and tutorials (Usability Week 1) |
| `connect` | ✅ | SSH connection |
| `diagnose` | ✅ | System diagnostics + AI analysis + RAG context |
| `diagnose --list-checks` | ✅ | List all available diagnostic checks |
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
**Structured Logging:** `log.WithFields(logrus.Fields{...}).Info("msg")`
**Dry-Run:** Check flag before executing destructive operations

## Health Check System (`lumo doctor`)

Validates configuration and dependencies before running. Checks: config file, AI provider, API keys, RAG system, system dependencies, updates.

```bash
lumo doctor        # Run all health checks
lumo doctor -v     # Verbose with timing
```

Implementation: `internal/doctor/{doctor.go,checks.go}`, `cmd/lumo/doctor.go`

---

## Testing & CI

**Coverage:** 66.7% (70 test files) | Table-driven tests, mock executors
**Run:** `go test ./...` | `make ci` (full local checks)

**Makefile Targets:**
```bash
make ci        # Linters, security checks, tests, builds (all locally)
make ci-lint   # Linters + security (golangci-lint, govulncheck)
make ci-test   # Tests with race detection
make ci-build  # Build CLI + Agent binaries
```

**CI Checks:**
- golangci-lint (gofmt, govet, 50+ linters)
- govulncheck (vulnerability scanning)
- Race detection tests
- Build verification
- Cross-platform builds (main branch only: linux/darwin × amd64/arm64)
- Path-based filtering: only runs on Go/Makefile/CI changes

---

## Security

**Status:** Phase 11b complete ✅ | All critical issues resolved

**Core Features:**
- JWT auth (24h expiration, configurable issuer)
- Rate limiting: Per-IP (60 req/min) + Per-user (3,600 req/hour)
- Database connection pooling with health monitoring
- mTLS for gRPC (Phase 11a)
- API key auth with scopes
- Command injection protection (sanitization, metacharacter filtering)
- SSH host key verification (default)
- Secrets via env vars only

**Never Commit:** `config.yaml`, `*.pem`, `*.key`, `id_rsa*`, `.env*`

**Key Env Variables:**
- `LUMO_API_JWT_SECRET` - JWT signing key (required production)
- `LUMO_DATABASE_PASSWORD` - DB password
- `LUMO_*_API_KEY` - Provider API keys

**File Permissions:** `chmod 600` for config, keys, certs

For rate limiting and DB pool config, see [configs/config.example.yaml](configs/config.example.yaml)

---

## Key Files & Components

**Checkers (12):**
- Core (6): CPU, Memory, Disk, Process, Service, Network
- Security (4): Patch Status, Open Ports, SSH Security, Auth Failures
- Specialized (2): Kubernetes (native client), Proxmox VE

**AI System:**
- Adapter pattern: HTTPClient, BaseProvider, StreamHandler (SSE + JSON-line)
- 5 providers: Anthropic, OpenAI, Gemini, Ollama, OpenRouter (via ProviderAdapter interface)
- Benefits: 27% code reduction, 5x easier maintenance

**Remediation:** executor, approval, audit, actions (disk, service, process)

**Notifications (4 providers):**
- Slack: Webhook + rich attachments
- Telegram: Bot API + Markdown
- Webhook: Generic (Discord, Teams, Mattermost)
- Email: SMTP + TLS + HTML
- Doc: `internal/notifications/README.md`

**Diagnostics Handler:**
- Fixed async execution with background method (executeDiagnostics)
- Accepts config and logger, improved UX with summary headers
- Shows checks, format, and AI analysis status

**API Configuration:**
- CORS: Configurable allowed_origins (no longer hardcoded "*")
- Version injection: Centralized via `internal/version` package, injected at build time

---

## Agent Architecture

**Three Deployment Models:**
1. **K8s DaemonSet:** Per-node monitoring (hostNetwork, hostPID, RBAC)
2. **K8s Deployment:** Cluster-wide monitoring (2+ replicas for HA, K8s API access)
3. **VM systemd:** System-level monitoring (non-root, CAP_NET_RAW, ProtectSystem=strict)

**Operational Modes:** scheduled, on-demand, continuous, hybrid (recommended)

**Communication:** Agents → API Server (registration, heartbeats, reports) + Message Queue (pending Phase 11c)

**Security:** JWT + mTLS, K8s RBAC (least-privilege), TLS 1.3, external secrets (Vault, AWS Secrets Manager)

**Resource Target:** 64-128 MB memory, <5% CPU avg, 1-10 KB/s network

**Deployment:**
```bash
# K8s
kubectl apply -f deployments/kubernetes/daemonset.yaml
helm install lumo-agent deployments/kubernetes/helm/lumo-agent

# VM
./deployments/systemd/install.sh
systemctl enable --now lumo-agent
```

See [deployments/kubernetes/README.md](deployments/kubernetes/README.md) and [deployments/systemd/README.md](deployments/systemd/README.md) for complete guides.

---

## Phase Roadmap

### Completed (Phases 1-10)

**Phases 1-2:** Foundation (Cobra, Viper, Logrus) + SSH (4 auth methods, retry, health monitoring)
**Phase 3:** Diagnostics runner + 6 core checkers
**Phase 4:** AI integration (5 providers, streaming, TOON format)
**Phase 5:** Security diagnostics (4 checkers) + Specialized checkers (K8s, Proxmox)
**Phase 6:** Auto-remediation (executor, approval, audit, actions)
**Phase 7:** API server (REST, PostgreSQL, Redis, gRPC, JWT, Rate limiting) ✅
**Phase 8:** Agent daemon (scheduler, reporter, health, metrics) ✅
**Phase 9:** K8s deployment (DaemonSet, Deployment, RBAC, Helm) ✅
**Phase 10:** VM deployment (systemd, install scripts, RPM/DEB packaging) ✅

**Post-Phase 10 Additions:**
- RAG system (chromem-go, OpenAI embeddings, hybrid ingestion): 87% MTTR reduction
- Diagnostic enhancements (`--list-checks`)
- Investor materials (demo, ROI calculator, competitive analysis)

### Current (Phase 11)

**Phase 11a: gRPC Foundation** - COMPLETE ✅ (Nov 20, 2025)
- Protocol Buffers, gRPC server/client, interceptors, mTLS, JWT auth
- Location: `internal/grpc/{server,client,handlers,interceptors}`

**Phase 11b: Security Hardening** - COMPLETE ✅ (Nov 21, 2025)
- Rate limiting (per-IP, per-user), DB connection pooling, JWT config
- Location: `internal/api/middleware/ratelimit.go`, `internal/database/postgres.go`

**Phase 11c: Messaging Integration** - PENDING ⏳
- Publisher/subscriber framework, NATS, Kafka, RabbitMQ, Redis
- Topic-based routing, agent integration, dead-letter queues
- Note: Foundation ready, can be implemented as standalone PR

### Future (Phases 12+)

**Phase 12:** Advanced Security - Certificate rotation, security audit, Vault integration
**Phase 13:** Production Readiness - Grafana dashboards, Prometheus alerts, load testing
**Phase 14:** Advanced Reporting - Multiple formats, historical data, trend analysis
**Phase 15:** Testing & Quality - Target 80% coverage, integration tests, chaos engineering
**Phase 16:** Advanced Features - Multi-cluster, anomaly detection, policy-as-code

---

## Key Features

**Localhost Auto-detection:** `diagnose` detects localhost patterns (`localhost`, `127.0.0.1`, `::1`, `0.0.0.0`) and runs directly (no SSH overhead)

**TOON Format:** Token-Oriented Object Notation - LLM-optimized reducing tokens by 30-60% vs JSON. Usage: `--format toon` or automatic with `--analyze`. Implementation: `formatters.NewToonFormatter()` via gotoon library

---

## Best Practices

**DO:** Wrap errors (`fmt.Errorf("context: %w", err)`), write tests, document exports, respect `dryRun` flag, update CLAUDE.md on phase completion

**DON'T:** Commit secrets, ignore errors, use global state, hardcode paths

**Code Review:** Go conventions, error wrapping, tests, `go fmt`, `go vet`

## Quick Reference

**Build & Test:**
```bash
make ci            # Full local checks (linters, security, tests, builds)
make build         # Build CLI and Agent
go test ./...      # Run tests
```

**CLI Usage:**
```bash
lumo doctor                              # Validate setup
lumo diagnose localhost --analyze --format toon
lumo fix localhost --dry-run
LUMO_ANTHROPIC_API_KEY=sk-ant-... lumo diagnose --analyze
```

**API Server:**
```bash
docker-compose up -d                     # PostgreSQL + Redis
lumo serve --config configs/config.example.yaml
curl http://localhost:8080/api/v1/health
```

**Agent Deployment:**
```bash
# Kubernetes
kubectl apply -f deployments/kubernetes/daemonset.yaml
helm install lumo-agent deployments/kubernetes/helm/lumo-agent

# VM
./deployments/systemd/install.sh
systemctl enable --now lumo-agent
```

**Key Code Patterns:**
```go
cfg, err := config.Load()                           // Load config
return fmt.Errorf("context: %w", err)               // Error wrapping
log.WithFields(logrus.Fields{...}).Info("msg")      // Logging
formatter := formatters.NewToonFormatter()           // TOON formatter
```

---

**For questions/improvements:** https://github.com/ignacio/lumo/issues

**RAG System Details:** See `internal/intelligence/` (chromem-go vector store, OpenAI embeddings, hybrid ingestion)
