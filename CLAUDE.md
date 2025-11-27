# CLAUDE.md - AI Assistant Guide for Lumo

> **Last Updated:** 2025-11-27 (Strategic Roadmap v1.1.0 Defined) | **Version:** 1.0.0 | **Status:** Phase 15 Complete ✅ | Phase 15b Complete ✅ | Phase 16 Complete ✅ | **Next: Phase 11c (Messaging) → v1.1.0** 🚀 | [Roadmap TODO](ROADMAP_TODO.md)

**Quick Links:** [Getting Started](docs/getting-started.md) | [Examples](examples/) | [Deployments](deployments/) | [API Docs](api/README.md)

---

## Contribution Guidelines

Always run `make ci` before committing (linters, security checks, tests, builds).

## Project Overview

**Lumo** - Intelligent SRE/DevOps automation platform in Go with 12 diagnostic checkers, 5 AI providers, auto-remediation, multi-platform notifications, and agent architecture (K8s + VM).

**Key Features:**
- Natural language interface (`lumo ask`) - translate queries to commands with AI
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
│   ├── lumo/                          # CLI: init, doctor, ask, diagnose, fix, serve, examples
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
│   │   └── eventdriven/               # Event-driven K8s monitoring (informers, watchers, debouncer)
│   ├── grpc/                          # gRPC server/client, handlers, interceptors, mTLS
│   ├── intelligence/                  # RAG: vectorstore, embeddings, ingestion
│   ├── doctor/                        # Health check system (6 checks)
│   ├── messaging/                     # Pub/sub framework (pending: NATS, Kafka, RabbitMQ, Redis)
│   ├── reliability/                   # Circuit breakers (100% tested)
│   └── observability/                 # OpenTelemetry tracing, structured observability
├── tests/
│   ├── integration/                   # API integration tests with testcontainers (PostgreSQL)
│   ├── load/                          # Load testing (50 concurrent workers, rate limiting)
│   └── testutil/                      # Shared test constants and utilities
├── deployments/
│   ├── kubernetes/                    # DaemonSet, Deployment, RBAC, Helm, kind
│   │   └── kind/                      # Local testing: deploy-lumo.sh, test-failure-scenarios.sh
│   └── systemd/                       # Service unit, install scripts, RPM/DEB packaging
├── examples/                          # 6 end-to-end examples (3,200+ LOC)
├── docs/                              # Getting started, competitive analysis, ROI, investor materials
├── website/                           # Next.js 16 landing page with TypeScript + Tailwind (ESLint 9 configured)
├── configs/                           # Example configurations
├── Dockerfile                         # Multi-stage alpine build for lumo CLI
├── Dockerfile.agent                   # Security-hardened build for lumo-agent (non-root)
└── docker-compose.yaml                # PostgreSQL + Redis for development

Total: 139 Go files + 83 test files | Coverage: 47.7% internal packages | Verified: 2025-11-26
```

---

## Technology Stack

**Core:** cobra, viper, logrus, x/crypto/ssh, backoff
**Database:** PostgreSQL (lib/pq), Redis (go-redis/v9), goose migrations
**AI:** Adapter pattern, custom HTTP client (no external SDKs), 5 providers
**API:** Chi router v5, JWT auth, rate limiting, request validation
**gRPC:** google.golang.org/grpc, Protocol Buffers, mTLS, JWT interceptors
**RAG:** chromem-go (local vector store), OpenAI embeddings
**Kubernetes:** k8s.io/client-go (native, no kubectl), SharedInformerFactory (event-driven)
**Agent:** robfig/cron/v3 (scheduling), Prometheus client (metrics), event-driven informers
**Observability:** OpenTelemetry (tracing), Prometheus (metrics), structured logging
**Notifications:** 4 providers (Slack, Telegram, Webhook, Email)

---

## Configuration System

**Hierarchy:** `--config` flag → `./config.yaml` → `~/.lumo/config.yaml`
**Loading:** `cfg, err := config.Load()` (searches hierarchy, auto-validates)
**Security:** API keys ONLY via env vars (LUMO_*_API_KEY), never in config files

**Viper Bindings (Nov 23, 2025):** 
- Explicit `viper.SetDefault()` calls added for database, AI, and agent config to enable environment variable reading in K8s deployments
- Fixed `viper.BindEnv()` to support multiple environment variable names (LUMO_AGENT_KUBERNETES_ENABLED, LUMO_DIAGNOSTICS_KUBERNETES_ENABLED)
- Added manual parsing of comma-separated `LUMO_AGENT_ENABLED_CHECKS` environment variable into slice (Viper doesn't auto-parse CSV to slices)

**Key Environment Variables:**
```bash
export LUMO_AI_PROVIDER=anthropic                    # anthropic|openai|ollama|gemini|openrouter
export LUMO_AI_ENABLED=false                         # Disable AI for testing
export LUMO_ANTHROPIC_API_KEY=sk-ant-...            # Provider-specific keys
export LUMO_RAG_ENABLED=true
export LUMO_AGENT_MODE=hybrid                       # scheduled|on-demand|continuous|hybrid|event-driven
export LUMO_AGENT_API_ENDPOINT=https://lumo-api...
export LUMO_AGENT_TOKEN=$JWT_TOKEN
export LUMO_AGENT_ENABLED_CHECKS=cpu,memory,disk,process,service,network,kubernetes
export LUMO_AGENT_KUBERNETES_ENABLED=true           # Enable Kubernetes diagnostics checker
export LUMO_AGENT_CACHE_PATH=/var/cache/lumo        # Agent cache directory
export LUMO_API_JWT_SECRET=secret-key               # Production required
export LUMO_DATABASE_HOST=postgres                  # Database host (K8s service name)
export LUMO_DATABASE_PORT=5432                      # Database port
export LUMO_DATABASE_PASSWORD=password              # DB password

# Event-driven mode configuration (K8s only)
export LUMO_AGENT_EVENT_DRIVEN_ENABLED=true         # Enable event-driven monitoring
export LUMO_AGENT_EVENT_DRIVEN_DEBOUNCE_WINDOW=45s  # Wait before processing events
export LUMO_AGENT_EVENT_DRIVEN_MAX_DEBOUNCE_WINDOW=3m  # Max wait (prevents infinite debouncing of continuous events)
export LUMO_AGENT_EVENT_DRIVEN_RESYNC_PERIOD=0s     # 0 = pure event-driven (no polling)
export LUMO_AGENT_EVENT_DRIVEN_GROUP_RELATED_EVENTS=true  # Batch related events
export LUMO_AGENT_EVENT_DRIVEN_MAX_EVENTS_PER_MIN=100     # Rate limiting
export LUMO_AGENT_EVENT_DRIVEN_MIN_SEVERITY=low     # low|medium|high|critical
export LUMO_AGENT_EVENT_DRIVEN_WATCH_POD_EVENTS=true      # Pod failures
export LUMO_AGENT_EVENT_DRIVEN_WATCH_WORKLOADS=true       # Deployments, StatefulSets, etc.
export LUMO_AGENT_EVENT_DRIVEN_WATCH_VOLUMES=true         # Volume issues
export LUMO_AGENT_EVENT_DRIVEN_WATCH_NODES=true           # Node conditions
export LUMO_AGENT_EVENT_DRIVEN_WATCH_EVENTS=true          # K8s Event resource
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
| `ask` | ✅ | Natural language interface - translate queries to commands |
| `connect` | ✅ | SSH connection |
| `diagnose` | ✅ | System diagnostics + AI analysis + RAG context |
| `diagnose --list-checks` | ✅ | List all available diagnostic checks |
| `events` | ✅ | Query Kubernetes events from PostgreSQL database |
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

**Coverage:** 47.7% internal packages (83 test files, 738 test functions) | Table-driven tests, mock executors

**Key Package Coverage:**
- `internal/reliability`: 100% (circuit breakers)
- `internal/diagnostics/formatters`: 97.4%
- `internal/diagnostics`: 88.3%
- `internal/diagnostics/checkers`: 78.3%
- `internal/api/auth`: 90.3%
- `internal/api/response`: 90.5%

**Phase 15 Complete (Nov 26, 2025):**
- ✅ Unit tests: 250+ test cases across 80 files
- ✅ Integration tests: API workflows with testcontainers
- ✅ Load tests: 50 concurrent workers, rate limiting verification
- ✅ Chaos engineering: 10+ K8s failure scenarios

**Phase 15b Complete (Nov 26, 2025):**
- ✅ Coverage improvements: 3 packages enhanced (+14.1%, +28.7%, +77.8%)
- ✅ New test files: reporter_test.go (637 LOC), middleware_test.go (391 LOC), tracing_test.go (93 LOC)
- ✅ Total new tests: 1,121 LOC, 19 new test functions
- ✅ All CI checks passing (golangci-lint, govulncheck, race detection)
- ✅ internal/agent: 17.8% → 31.9% (HTTP client, registration, retry logic)
- ✅ internal/api/middleware: 0% → 28.7% (rate limiting, auth, CORS)
- ✅ internal/observability: 0% → 77.8% (OpenTelemetry tracing)

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

**Integration Tests:**
```bash
# Run integration tests (requires Docker)
go test -v ./tests/integration/...

# Skip integration tests (short mode)
go test -short ./...
```
- Location: `tests/integration/` (testenv.go, api_test.go)
- Uses testcontainers-go for PostgreSQL
- Tests: Health endpoints, Agent lifecycle, Jobs CRUD, Authentication, JWT, Events API
- ~22 seconds execution time

**Chaos Engineering:**
```bash
# Run all failure scenarios (requires K8s cluster)
./deployments/kubernetes/kind/test-failure-scenarios.sh

# Run specific scenario
./test-failure-scenarios.sh --scenario oom-killed
./test-failure-scenarios.sh --list
```
- Location: `deployments/kubernetes/kind/test-failure-scenarios.sh` (781 lines)
- 10+ scenarios: ImagePullBackOff, CrashLoopBackOff, OOMKilled, DeploymentFailed, JobFailed, PVCProvisionFailed
- Validates full event pipeline: K8s failure → Agent → API → Database

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

**Checkers (12 total, 7 active in K8s agents):**
- Core (6): CPU, Memory, Disk, Process, Service, Network
- Security (4): Patch Status, Open Ports, SSH Security, Auth Failures
- Specialized (2): Kubernetes (native client, ✅ working in cluster deployments), Proxmox VE

**AI System:**
- Adapter pattern: HTTPClient, BaseProvider, StreamHandler (SSE + JSON-line)
- 5 providers: Anthropic, OpenAI, Gemini, Ollama, OpenRouter (via ProviderAdapter interface)
- Benefits: 27% code reduction, 5x easier maintenance

**Remediation:** executor, approval, audit, actions (disk, service, process, Kubernetes), suggestion engine - comprehensive test coverage with all tests passing (disk cleanup, log rotation, service management with command injection prevention)

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

**Observability (Phase 12 - Complete ✅):**
- OpenTelemetry tracing: `internal/observability/tracing.go` - Distributed trace collection with span creation
- Tracing spans in critical paths: API handlers, diagnostics runner, agent operations, scheduler tasks
- Span attributes: Job IDs, targets, metrics, durations, execution results
- Enhanced metrics: Full Prometheus instrumentation for diagnostics, heartbeats, cache, API availability
- Production readiness: Critical packages now tested (cache, database, doctor all 100%)
- Diagnostic API: Checker registration fully implemented

---

## Agent Architecture

**Two Deployment Models:**
1. **K8s Event-Driven Deployment:** Real-time Kubernetes monitoring (2+ replicas for HA, pure event reporting to API server)
2. **VM systemd:** System-level monitoring (non-root, CAP_NET_RAW, ProtectSystem=strict)

**Operational Modes:** 
- **K8s:** event-driven only (real-time informers, <60s detection)
- **VMs:** scheduled, on-demand, continuous, hybrid

**Communication:** Agents → API Server (registration, heartbeats, event submission)

**Architecture (Nov 24, 2025):** Centralized intelligence model
- **Agents:** Pure event reporters (no AI, no notifications)
- **API Server:** AI analysis + multi-channel notifications
- **Benefits:** ~2,000 LOC reduction, easier scaling, single source of truth

**Security:** JWT + mTLS, K8s RBAC (least-privilege), TLS 1.3, external secrets (Vault, AWS Secrets Manager)

**Resource Target:** 64-128 MB memory, <5% CPU avg, 1-10 KB/s network

**Status (Nov 24, 2025):** ✅ **Refactored to centralized architecture!** Single event-driven deployment model for K8s, all AI/notifications handled by API server.

**Deployment:**
```bash
# K8s - Full Stack (Recommended for testing)
cd deployments/kubernetes/kind
./deploy-lumo.sh  # Complete stack: DB + API + Agents + Tests

# K8s - Production (Event-Driven Only)
kubectl apply -f deployments/kubernetes/base/deployment-agent.yaml
helm install lumo-agent deployments/kubernetes/helm/lumo-agent

# VM
./deployments/systemd/install.sh
systemctl enable --now lumo-agent
```

See [deployments/kubernetes/README.md](deployments/kubernetes/README.md), [deployments/kubernetes/kind/FULL_STACK_DEPLOYMENT.md](deployments/kubernetes/kind/FULL_STACK_DEPLOYMENT.md), and [deployments/systemd/README.md](deployments/systemd/README.md) for complete guides.

**Docker Configuration (Nov 22, 2025):**
- Consolidated Dockerfiles to project root for improved CI/CD practices
- `Dockerfile`: Multi-stage alpine build for lumo CLI (minimal final image)
- `Dockerfile.agent`: Security-hardened build for lumo-agent (non-root user, minimal permissions)
- Build tools reference root-level files (e.g., `deployments/kubernetes/kind/build-and-load.sh`)

---

## Event-Driven Architecture (K8s)

**Status (Nov 24, 2025):** ✅ **Complete and Production-Ready** | 11 new files, ~3,500 LOC | Code review improvements applied | All CI checks passed

### Overview

Event-driven mode transforms Kubernetes monitoring from periodic polling (5-minute intervals) to **real-time reactive monitoring** using Kubernetes informers. This provides <60s detection latency, 90%+ reduction in API load, and intelligent debouncing to filter transient issues.

**Key Benefits:**
- **Zero polling overhead** - Pure event-driven using SharedInformerFactory
- **Real-time detection** - <60s latency (including 45s debounce window)
- **Intelligent filtering** - Debouncing eliminates transient failures
- **90%+ API load reduction** - Watch streams vs periodic List() calls
- **Focus on K8s issues** - No system metrics, only Kubernetes resources

### Architecture

```
K8s Event → Informer → Watcher → Debouncer → API Processor → API Server
                                                                 ↓
                                                      AI Analysis + Notifications
```

### Components

**Agent-Side (Pure Event Reporting):**

**Manager** (`internal/agent/eventdriven/manager.go` - 271 lines)
- SharedInformerFactory lifecycle management
- Coordinates 9 specialized watchers
- Graceful startup/shutdown with cache synchronization

**Watchers** (`internal/agent/eventdriven/watchers/` - 1,417 lines)
- **PodWatcher** (414 lines): ImagePullBackOff, CrashLoopBackOff, OOMKilled, high restarts, evictions
- **WorkloadWatcher** (543 lines): Deployments, StatefulSets, DaemonSets, Jobs
- **VolumeWatcher** (251 lines): PVC issues, FailedMount, FailedBinding
- **NodeWatcher** (209 lines): NotReady, MemoryPressure, DiskPressure, PIDPressure

**Debouncer** (`internal/agent/eventdriven/debouncer.go` - 274 lines)
- 45-second configurable wait window (filters transient failures)
- Redis-backed state tracking for deduplication
- Automatic event count tracking

**API Processor** (`internal/agent/eventdriven/api_processor.go` - 320 lines)
- Submits events to API server via HTTP POST
- Retry logic with exponential backoff
- Redis caching for deduplication
- Batch processing support

**API Server-Side (Centralized Intelligence):**

**Event Handler** (`internal/api/handlers/events.go` - 468 lines)
- Receives event submissions from agents
- AI-powered event analysis (all 5 providers supported)
- Multi-channel notifications (Slack, Telegram, Email, Webhook)
- Structured event storage and retrieval

**Types** (`internal/agent/eventdriven/types.go` - 277 lines)
- 17 event types with severity classification
- Event filtering by namespace, severity, labels
- Event grouping by owner UID

**Code Review Improvements (Nov 24, 2025):**
- Fixed EventGrouper race condition with sync.RWMutex
- Eliminated unsafe type assertions in all watchers (proper interface methods)
- Added 5 Prometheus metrics: events_processed_total, event_processing_duration_seconds, ai_analysis_total, ai_analysis_duration_seconds, notifications_sent_total

**Code Quality Improvements (Nov 25, 2025):**
- Removed debug print statements from production code (`internal/database/repository/agent.go`)
- Added proper error context wrapping in critical paths:
  - Agent registration, cache operations, gRPC streaming
  - All K8s informer event handler setup (node, volume, workload watchers)
- Zero linting issues, zero vulnerabilities (govulncheck clean)

**Bug Fixes (Nov 25, 2025):**
- **OOMKilled Detection**: Fixed PodWatcher to check BOTH `State.Terminated` (for restartPolicy: Never) and `LastTerminationState.Terminated` (after restart), with restart count tracking to prevent duplicate events
  - Location: `internal/agent/eventdriven/watchers/pod.go:106-138`
  - Issue: Only checked LastTerminationState, missing OOMKilled containers with restartPolicy: Never
- **PVC Provision Failed**: Enhanced detection logic to trigger on first observation if already pending >2 minutes, or when crossing the 2-minute threshold
  - Location: `internal/agent/eventdriven/watchers/volume.go:66-112`
  - Issue: Required status change to trigger, missing PVCs that stayed in Pending state during informer sync
  - Note: EventWatcher also detects these via Kubernetes "ProvisioningFailed" events as a backup detection method
- **Infinite Debouncing**: Added max debounce window (3 minutes) to prevent continuous events (like PVC ProvisioningFailed repeating every 15s) from infinitely resetting the debounce timer
  - Location: `internal/agent/eventdriven/debouncer.go` (MaxDebounceWindow field and logic), `internal/config/config.go` (config field), `internal/agent/agent.go` (initialization)
  - Issue: Events arriving frequently would reset the 45s debounce window continuously, preventing processing of persistent failures
  - Solution: If debounce timer is reset multiple times, max window ensures processing within 3 minutes while still filtering transient issues
  - Latency: OOMKilled (~50s), CrashLoopBackOff (~75s), ImagePullBackOff (~65s), PVC ProvisioningFailed (~240s with 3m max window)

### Event Types & Severity

| Event Type | Trigger | Severity |
|------------|---------|----------|
| `oom-killed` | Container OOMKilled | Critical |
| `pod-evicted` | Pod evicted from node | Critical |
| `node-not-ready` | NodeReady → NotReady | Critical |
| `job-failed` | BackoffLimitExceeded | Critical |
| `image-pull-backoff` | Image pull failures | High |
| `crash-loop-backoff` | Container crash loop | High |
| `deployment-failed` | ProgressDeadlineExceeded | High |
| `volume-failed-mount` | FailedMount | High |
| `pvc-provision-failed` | Provisioning failed | Medium |
| `pod-pending` | Pending >5 minutes | Medium |

### Configuration

See `deployments/kubernetes/base/configmap-agent.yaml` for complete configuration options.

**Key Settings:**
- `debounce_window: 45s` - Wait before processing (filters transients)
- `max_debounce_window: 3m` - Maximum wait before processing (prevents infinite debouncing of continuous events)
- `resync_period: 0s` - Pure event-driven (no polling)
- `group_related_events: true` - Batch related failures
- `max_events_per_min: 100` - Rate limiting
- `min_severity: low` - Process all severity levels

**Requirements:**
- **Redis** - Event state tracking (required)
- **Kubernetes RBAC** - Watch permissions on monitored resources
- **AI Provider API Key** - For event analysis (optional but recommended)

### Deployment

```bash
# Deploy Redis (required)
kubectl apply -f deployments/kubernetes/redis/

# Create secrets
kubectl create secret generic lumo-ai-secrets \
  --from-literal=anthropic-api-key=$ANTHROPIC_KEY \
  -n lumo-system

# Deploy event-driven agent (2-replica HA)
kubectl apply -f deployments/kubernetes/base/configmap-agent.yaml
kubectl apply -f deployments/kubernetes/base/deployment-agent.yaml

# Verify
kubectl logs -f -n lumo-system -l mode=event-driven
```

### Performance Characteristics

**Before (Periodic Polling):**
- Detection: 0-300s latency (avg 150s)
- API Load: List() every 5 minutes
- False Positives: ~30%

**After (Event-Driven):**
- Detection: <60s latency
- API Load: 90%+ reduction
- False Positives: <5%

### Documentation

See [EVENT_DRIVEN_IMPLEMENTATION.md](EVENT_DRIVEN_IMPLEMENTATION.md) for complete implementation details, testing results, and migration guide.

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

### Completed (Phase 12)

**Phase 12: Production Readiness - Phase 2** - COMPLETE ✅ (Nov 22, 2025)
- OpenTelemetry distributed tracing with full span creation in critical paths
- Tracing instrumentation added to:
  - API handlers (diagnostics, remediation) - `internal/api/handlers/`
  - Diagnostic runner and individual checks - `internal/diagnostics/diagnostics.go`
  - Agent reporter (register, heartbeat, submit) - `internal/agent/reporter.go`
  - Agent scheduler task execution - `internal/agent/scheduler.go`
- Comprehensive span attributes for observability (job IDs, targets, metrics, durations)
- Error recording and status tracking in all critical operations
- Enhanced Prometheus metrics for comprehensive monitoring
- Critical test coverage: cache, database, doctor packages (now 100% tested)
- Diagnostic checker registration fully implemented
- All tests passing with tracing enabled

### Completed (Phase 13)

**Phase 13: Circuit Breaker Integration** - COMPLETE ✅ (Nov 24, 2025)
- Circuit breakers integrated into all external service calls for resilience
- **AI Providers** (`internal/ai/base_provider.go`): Analyze(), Health(), Ask() methods protected
- **Notification Providers** (`internal/notifications/`): All Send() methods wrapped
- **gRPC Client** (`internal/grpc/client/client.go`): Key RPC methods protected
- **Test Coverage**: 100% for circuit breaker package (6 test cases)
- Benefits: Prevents cascading failures, fast-fail behavior, automatic recovery

### Completed (Phase 15)

**Phase 15: Testing & Quality** - COMPLETE ✅ (Nov 26, 2025)
- ✅ **Unit Test Expansion:** Added 8 comprehensive test files (1,536 LOC)
  - JWT authentication: token generation, validation, refresh (90.3% coverage)
  - API response helpers: all response types tested (90.5% coverage)
  - Handler validation: agents, approvals, auth, health, jobs, diagnostics
  - 250+ test cases covering edge cases and error paths
- ✅ **Code Quality:** Removed debug statements, improved error context wrapping
- ✅ **Integration Tests:** End-to-end API workflow testing with testcontainers
  - Location: `tests/integration/` (testenv.go, api_test.go, ~1,000 LOC)
  - Uses PostgreSQL testcontainers for real database testing
  - Tests: Health endpoints, Agent lifecycle, Jobs CRUD, Authentication, JWT, Events API
  - ~22 seconds execution time, Docker required
- ✅ **Load Testing:** Performance benchmarks (`tests/load/load_test.go`)
  - Health endpoint load testing (50 workers × 20 requests)
  - Rate limiting verification tests
- ✅ **Chaos Engineering:** Kubernetes failure scenario testing
  - Location: `deployments/kubernetes/kind/test-failure-scenarios.sh` (781 lines)
  - 10+ failure scenarios: ImagePullBackOff, CrashLoopBackOff, OOMKilled, DeploymentFailed, JobFailed, PVCProvisionFailed
  - Verifies event detection, database storage, and end-to-end flow
- **Overall Progress:** Internal package coverage 53.4%, 80 test files, all tests passing

### Completed (Phase 16)

**Phase 16: Event-Driven K8s Monitoring** - COMPLETE ✅ (Nov 24, 2025)
- Real-time event-driven Kubernetes monitoring via informers (replacing 5-min polling)
- 11 new files (~3,500 LOC): Manager, 9 specialized watchers, Debouncer, Processor, Types
- 17 event types with 4 severity levels (critical, high, medium, low)
- 45-second intelligent debouncing with Redis state tracking
- <60s detection latency vs 0-300s polling (150s avg)
- 90%+ Kubernetes API load reduction
- Code review: Race condition fix (sync.RWMutex), safe type assertions, 5 Prometheus metrics
- Location: `internal/agent/eventdriven/` with full documentation in `EVENT_DRIVEN_IMPLEMENTATION.md`
- All CI checks passed

### Strategic Roadmap (Nov 27, 2025)

**Current Maturity:** Enterprise-Ready v1.0.0 - Production deployment ready with comprehensive features

**Next Release: v1.1.0 (Target: January 2026)**

See [ROADMAP_TODO.md](ROADMAP_TODO.md) for detailed technical implementation tasks.

**Tier 1 - Critical Path (Immediate):**

**Phase 11c: Messaging Integration** ⚡ **PRIORITY #1** - [2-3 weeks]
- **Why Critical:** Blocks horizontal scaling beyond 10 agents, required for enterprise scale
- **Scope:** Pub/sub framework, NATS/Kafka/RabbitMQ/Redis Streams providers
- **Deliverables:**
  - `/internal/messaging/` package with provider abstraction
  - Event submission via message queue (replaces HTTP POST)
  - Dead-letter queues, replay capability, event routing
  - OpenTelemetry tracing for message flow
- **Success Metrics:** 1,000 events/sec sustained throughput
- **Status:** Foundation ready, pending implementation

**Phase 14: Advanced Reporting** 📊 **PRIORITY #2** - [2-3 weeks, parallel with 11c]
- **Why Important:** Market differentiator - executive visibility with AI-powered insights
- **Scope:** Report generation engine, PDF/HTML/CSV export, trend detection, scheduled delivery
- **Deliverables:**
  - `/internal/reporting/` package with template system
  - `lumo report` CLI command with time-window analysis
  - Database schema extensions for metrics history
  - Anomaly detection integration
- **Success Metrics:** <5 sec generation for 30-day reports
- **Status:** No blockers, can start immediately

**Tier 2 - Enterprise Features (v1.1.0 continued):**

**Phase 17a: Multi-Cluster Orchestration** 🌐 - [3-4 weeks]
- **Scope:** Central control plane, cross-cluster agent registration, unified alerting
- **Dependency:** Phase 11c (messaging) recommended first
- **Status:** Planned for January 2026

**Phase 17b: Anomaly Detection & Policy-as-Code** 🤖 - [4-5 weeks]
- **Scope:** ML baseline models, policy DSL, self-healing automation
- **Dependency:** Phase 11c (messaging required for policy routing)
- **Status:** Planned for v1.2.0

**Tier 3 - Quality & Performance (Ongoing):**

**Test Coverage Enhancement** - [1-2 weeks]
- **Current:** 47.7% overall
- **Target:** 70%+ overall, 90%+ critical packages
- **Focus Areas:** `/internal/database/repository/`, event-driven watchers, API handlers
- **Status:** Continuous improvement

**Performance Optimization** - [1 week]
- **Focus:** Database query caching, batch inserts, Redis hit rates
- **Expected Gain:** 30-50% API latency reduction
- **Status:** Profiling phase

**Documentation** - [Ongoing]
- Helm deployment guide (`docs/helm-deployment.md`)
- API authentication guide (`docs/api-auth-guide.md`)
- Troubleshooting playbook (`docs/troubleshooting.md`)

---

### Version Timeline

```
v1.0.0 [CURRENT - Nov 27, 2025]
└─ Enterprise-ready foundation with event-driven K8s monitoring

v1.1.0 [TARGET - January 2026]
├─ Phase 11c: Messaging Integration (NATS/Kafka/RabbitMQ/Redis)
├─ Phase 14: Advanced Reporting (PDF/HTML/CSV, trends)
├─ Phase 17a: Multi-Cluster Orchestration
├─ Test Coverage 70%+
└─ Performance optimizations (30-50% latency reduction)

v1.2.0 [Q1 2026]
├─ Phase 17b: Anomaly Detection + Policy-as-Code
├─ Backup/Recovery system
├─ RBAC fine-grained permissions
├─ Helm advanced features (ingress, TLS, secrets)
└─ API v2 with GraphQL option

v2.0.0 [Q2-Q3 2026]
├─ Distributed agent orchestration
├─ Advanced ML models (behavior-based anomaly detection)
├─ Custom plugin system
└─ SaaS multi-tenancy
```

---

### Enterprise Readiness Gap Analysis

**Strengths (Production Ready):**
- ✅ Security foundation (JWT, mTLS, rate limiting, RBAC)
- ✅ Multi-platform deployment (K8s + VMs)
- ✅ Event-driven architecture (real-time monitoring, <60s detection)
- ✅ Comprehensive diagnostics (12 checkers)
- ✅ AI integration (5 providers, RAG system)
- ✅ Observability (OpenTelemetry, Prometheus)
- ✅ Circuit breakers (fault tolerance)

**Critical Gaps (Must Close for Enterprise Scale):**

| Gap | Severity | Impact | ETA |
|-----|----------|--------|-----|
| Messaging Queue | HIGH | Can't scale beyond 10 agents reliably | Phase 11c (2-3w) |
| Advanced Reporting | MEDIUM | No stakeholder visibility | Phase 14 (2-3w) |
| Multi-Cluster Support | MEDIUM | Enterprise limitation | Phase 17a (3-4w) |
| Test Coverage (70%+) | MEDIUM | Risk in maintenance | 1-2 weeks |
| Helm Documentation | LOW | Deployment friction | Ongoing |
| Performance Tuning | LOW | Handles 100+ agents, needs optimization | 1 week |

---

## Key Features

**Localhost Auto-detection:** `diagnose` detects localhost patterns (`localhost`, `127.0.0.1`, `::1`, `0.0.0.0`) and runs directly (no SSH overhead)

**TOON Format:** Token-Oriented Object Notation - LLM-optimized reducing tokens by 30-60% vs JSON. Usage: `--format toon` or automatic with `--analyze`. Implementation: `formatters.NewToonFormatter()` via gotoon library

---

## Website Infrastructure

**Landing Page:** Next.js 16 + TypeScript + Tailwind CSS (in `/website/`)

**Tooling Setup (Nov 23, 2025):**
- ESLint 9 flat config: `website/eslint.config.mjs` with TypeScript, React, React Hooks, and accessibility plugins
- Package scripts: `npm run lint`, `npm run lint:fix`, `npm run type-check`
- All checks passing: lint, type-check, build
- Linting infrastructure for landing page components (Hero, Value Proposition, Features, Terminal demo)

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
lumo ask "check cpu usage"               # Natural language interface
lumo ask "why is the server slow?" -y    # Auto-execute without confirmation
lumo diagnose localhost --analyze --format toon
lumo events --severity critical --limit 10  # Query Kubernetes events
lumo events --type oom-killed --format json # Filter by event type
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
# Kubernetes - Event-Driven Mode (Single Deployment Model)
kubectl apply -f deployments/kubernetes/base/configmap-agent.yaml
kubectl apply -f deployments/kubernetes/base/deployment-agent.yaml
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
