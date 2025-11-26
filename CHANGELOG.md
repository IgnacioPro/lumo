# Changelog

All notable changes to Lumo will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.1.0] - 2025-11-26

### Added

#### Security Hardening & Production Quality

**Authentication & Secrets Management**
- Add Expiration() getter to JWTManager for proper token expiration access
- Update AuthHandler to use actual JWT expiration instead of hardcoded 24h values
- Implement environment variable interpolation in docker-compose for secure DB credential management
- Add mTLS client certificate support in agent reporter (TLSCertFile, TLSKeyFile, TLSCAFile)
- Improve secret file handling with secret.yaml.template to prevent accidental deployment of placeholder secrets

**SSH Security Hardening**
- Change SSH strict_host_key_checking default to true (prevents MITM attacks)
- Hardcode autoApprove=false in remediation executor as defense-in-depth measure

**Code Quality & Refactoring**
- Add parseUUIDParam() helper in API handlers for consistent UUID parsing
- Add scanAgent() helper in database repository to eliminate duplicate row scanning logic
- Add NewCheckResult() factory function to reduce checker initialization code
- Eliminate ~200 lines of duplicated code across handlers, repositories, and checkers
- Consolidate duplicate isLocalhost() functions into single shared utility
- Extract magic numbers to named constants (MaxEventsPerRequest, MaxListLimit, DefaultListLimit, MaxConcurrentEventProcessors)

**Event-Driven Enhancements**
- Add comprehensive event handler tests (15+ test cases, 378 LOC)
- Implement async event processing with worker pool and semaphore
- Add context parameter to Redis Health() method for trace propagation
- Fix response body closure leak by refactoring doWithRetry loop into doSingleRequest helper
- Add worker pool with configurable concurrency (MaxConcurrentEventProcessors=10)

**Test Coverage Improvements**
- Expand agent reporter tests with HTTP client, registration, retry logic scenarios
- Add middleware tests for rate limiting, authentication, context helpers (396 LOC)
- Add observability/tracing tests (98 LOC)
- Coverage improvements: internal/agent +14.1%, internal/api/middleware +28.7%, internal/observability +77.8%
- Overall internal package coverage: 47.7% (83 test files, 738 test functions)

**Integration Testing**
- Add comprehensive integration test suite with testcontainers-go
- Tests: Health endpoints, Agent lifecycle, Jobs CRUD, Authentication, JWT validation, Events API
- Location: tests/integration/ (testenv.go, api_test.go, ~1,000 LOC)
- ~22 seconds execution time with PostgreSQL testcontainers

**Documentation & Governance**
- Add CONTRIBUTING.md with comprehensive contributor guidelines
- Add GitHub issue templates (bug_report.md, feature_request.md)
- Add GitHub pull request template
- Add .golangci.yml for Go linting configuration
- Add Dependabot configuration (.github/dependabot.yml)
- Add multiple internal package READMEs:
  - internal/agent/README.md - Agent architecture and deployment
  - internal/ai/README.md - AI provider integration guide
  - internal/api/README.md - API server documentation
  - internal/diagnostics/README.md - Diagnostics system overview
- Update main README.md with improved sections and clearer structure

#### Dependency Updates

**Go Dependencies**
- Bump golang-jwt/jwt from 5.2.2 to 5.3.0
- Bump pressly/goose from 3.24.1 to 3.26.0
- Bump prometheus/client_golang from 1.20.5 to 1.23.2
- Bump redis/go-redis from 9.16.0 to 9.17.1
- Bump golang.org/x/crypto from 0.44.0 to 0.45.0
- Bump google.golang.org/grpc from 1.67.0 to 1.75.1
- Bump google.golang.org/protobuf from 1.34.2 to 1.36.10
- Bump k8s.io/api from 0.31.3 to 0.34.2
- Bump k8s.io/apimachinery from 0.31.3 to 0.34.2
- Bump k8s.io/client-go from 0.31.3 to 0.34.2

**Node.js Dependencies (website)**
- Bump @types/react from 19.2.6 to 19.2.7
- Bump eslint-config-next from 16.0.3 to 16.0.4
- Bump lucide-react from 0.554.0 to 0.555.0
- Bump next from 16.0.3 to 16.0.4

**Container Images**
- Bump golang base image from 1.24-alpine to 1.25-alpine

### Changed

**Configuration System**
- Update config.example.yaml with secure defaults and improved documentation
- Update kustomization.yaml with instructions for proper secret creation
- Improve environment variable handling for database credentials

**Security Defaults**
- SSH strict_host_key_checking now defaults to true (BREAKING CHANGE)
- Set autoApprove=false by default in remediation executor
- Secrets now managed via environment variables (LUMO_*_API_KEY pattern)

**API Handler Behavior**
- All handlers now use consistent UUID parsing with parseUUIDParam() helper
- Improved error messages with better context wrapping
- All handlers use structured response helpers consistently

**Database Repository**
- Consolidated duplicate agent scanning logic into scanAgent() helper
- Reduced repository code duplication across 6 methods
- Consistent error handling and logging across all repository methods

### Fixed

**Critical Fixes**
- Fix config loading in CLI mode that was blocked by unnecessary database password validation
- Fix AI provider key lookup to check provider-specific env vars first (LUMO_OPENAI_API_KEY, etc.)
- Remove 'testing' package import from production code (internal/config/config.go)
- Fix response body closure leak in event processor HTTP client
- Add explicit viper.BindEnv() calls for API key environment variables

**Agent & Event Processing**
- Fix OOMKilled detection for containers with restartPolicy: Never
- Fix PVC Provision Failed detection logic for PVCs pending >2 minutes
- Fix infinite debouncing by adding max debounce window (3 minutes)
- Add proper context propagation in Redis operations

**Testing & CI**
- Remove debug print statements from production code
- Improve error context wrapping in critical paths
- All linting issues resolved (errcheck, staticcheck, vet)
- All security checks passing (govulncheck clean)
- All tests passing with race detection enabled

### Breaking Changes

**SSH Connection Behavior**
- SSH strict_host_key_checking now defaults to true
- This prevents Man-in-the-Middle attacks by verifying host keys
- Set `strict_host_key_checking: false` in config for trusted networks only
- Affected: All SSH-based diagnostics and remediation actions

**Database Schema** (if upgrading from earlier versions)
- No schema changes in this release
- Goose migrations handle all version transitions automatically

### Security

**Authentication**
- JWT tokens now use actual configured expiration instead of hardcoded 24h
- Proper token expiration handling via Expiration() getter
- Environment-based secret management (no hardcoded secrets)

**Infrastructure**
- mTLS client certificate support in agent reporter
- SSH host key verification enabled by default
- Secret template files prevent accidental production deployment

**Dependency Security**
- All dependencies updated to latest minor versions
- No known vulnerabilities (govulncheck verified)
- Go 1.25 provides improved security features

## [Unreleased - Investor Materials]

### Added

#### Investor POC Materials

**Purpose:** Complete package for investor presentations and fundraising

**Strategy & Planning**
- `docs/INVESTOR_POC_STRATEGY.md` - Comprehensive POC strategy document (13,000+ words)
  - Executive summary and value proposition
  - Demo components and deliverables
  - 3-week implementation roadmap
  - Success metrics and demo best practices
  - Budget and resource planning

**Demo Environment**
- `demos/investor-demo/` - Automated live demonstration environment
  - `setup.sh` - Creates realistic demo environment with simulated issues
  - `start-issues.sh` - Activates demo issues (disk space, failed services, etc.)
  - `run-demo.sh` - Guided 10-minute demo script with timing and talking points
  - `reset.sh` - Resets environment for next demo
  - `README.md` - Complete demo guide and FAQ (500+ lines)
- **Demo Features:**
  - 7 sections: Problem intro, diagnosis, AI analysis, remediation, ROI, deployment, Q&A
  - Color-coded terminal output with progress tracking
  - Simulated issues: log files, temp files, failed services, suspicious processes
  - Automated timing and talking points
  - Fallback simulated output when Lumo not available

**Business Materials**
- `docs/ROI_Calculator.md` - Comprehensive ROI analysis tool (10,000+ words)
  - Interactive calculator template with typical values
  - ROI by team size (3-50 engineers)
  - ROI by industry (e-commerce, SaaS, enterprise IT)
  - 3-year extended analysis
  - Cost comparison vs. competitors (Datadog, PagerDuty, New Relic, Splunk)
  - Hidden costs avoided analysis
  - Typical ROI: 8,100%, payback: 4.4 days, $4.1M annual savings

- `docs/COMPETITIVE_ANALYSIS.md` - Market positioning and competitor analysis (13,000+ words)
  - Detailed comparison vs. 7 major competitors
  - Competitive matrix (features, pricing, capabilities)
  - Market landscape and quadrant analysis
  - Win/loss analysis
  - Competitive threats and response strategies
  - Go-to-market strategy
  - Future market evolution predictions

- `docs/Lumo_One_Pager.md` - Executive summary leave-behind (2,500+ words)
  - Problem/solution overview
  - Key metrics and ROI
  - Competitive advantage table
  - Market opportunity
  - Business model and pricing
  - The ask ($2M seed round)
  - Contact information

**Total Deliverables:**
- 4 demo automation scripts (fully executable)
- 5 comprehensive documents (40,000+ words total)
- Ready-to-use investor presentation materials
- Complete 3-week POC implementation roadmap

**Impact:**
- Reduces investor meeting prep time from weeks to hours
- Provides quantifiable ROI proof (8,100%, $4.1M savings)
- Demonstrates production-readiness and market positioning
- Includes automated live demo (no manual setup required)

## [1.0.0] - 2025-11-24

### Added

#### Phase 16: Event-Driven Kubernetes Monitoring - Production Ready

**Major Release Milestone**
This release marks **v1.0.0 - Lumo's production-ready version** with complete event-driven Kubernetes monitoring via real-time informers, replacing the previous 5-minute polling approach with intelligent, sub-60-second detection.

**Real-Time Event-Driven Architecture**

*Overview:*
- Transform Kubernetes monitoring from periodic polling to pure event-driven reactive monitoring
- Leverage SharedInformerFactory and Kubernetes watchers for <60s detection latency
- 90%+ reduction in Kubernetes API load (watch streams vs periodic List() calls)
- Intelligent 45-second debouncing to filter transient failures

**Architecture Stack**

*Agent-Side Components (Pure Event Reporting):*
- **Manager** (`internal/agent/eventdriven/manager.go` - 271 lines)
  - SharedInformerFactory lifecycle management
  - Coordinates 9 specialized watchers
  - Graceful startup/shutdown with cache synchronization

- **Watchers** (`internal/agent/eventdriven/watchers/` - 1,417 lines total)
  - **PodWatcher** (414 lines): ImagePullBackOff, CrashLoopBackOff, OOMKilled, high restarts, evictions
  - **WorkloadWatcher** (543 lines): Deployments, StatefulSets, DaemonSets, Jobs, ReplicaSets
  - **VolumeWatcher** (251 lines): PVC issues, FailedMount, FailedBinding errors
  - **NodeWatcher** (209 lines): NotReady, MemoryPressure, DiskPressure, PIDPressure conditions
  - Plus 5 additional specialized watchers for complete K8s coverage

- **Debouncer** (`internal/agent/eventdriven/debouncer.go` - 274 lines)
  - 45-second configurable wait window (filters transient failures)
  - Redis-backed state tracking for deduplication
  - Automatic event count tracking and metrics

- **API Processor** (`internal/agent/eventdriven/api_processor.go` - 320 lines)
  - Submits events to API server via HTTP POST
  - Retry logic with exponential backoff
  - Redis caching for deduplication
  - Batch processing support

*API Server-Side (Centralized Intelligence):*
- **Event Handler** (`internal/api/handlers/events.go` - 468 lines)
  - Receives event submissions from agents
  - AI-powered event analysis (all 5 providers supported)
  - Multi-channel notifications (Slack, Telegram, Email, Webhook)
  - Structured event storage and retrieval

- **Event Types** (`internal/agent/eventdriven/types.go` - 277 lines)
  - 17 event types with 4 severity levels (critical, high, medium, low)
  - Event filtering by namespace, severity, labels
  - Event grouping by owner UID

**Code Quality Improvements**

*Code Review Enhancements:*
- Fixed EventGrouper race condition with sync.RWMutex for thread-safe access
- Eliminated unsafe type assertions in all watchers using proper interface methods
- Added 5 Prometheus metrics:
  - `events_processed_total` - Counter for processed events by type
  - `event_processing_duration_seconds` - Histogram for processing latency
  - `ai_analysis_total` - Counter for AI analysis requests
  - `ai_analysis_duration_seconds` - Histogram for AI analysis latency
  - `notifications_sent_total` - Counter for notifications by channel

**Event Types & Severity Levels**

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

**Performance Characteristics**

*Before (Periodic Polling - v0.11.0):*
- Detection latency: 0-300 seconds (average: 150 seconds)
- API load: List() every 5 minutes per agent
- False positives: ~30% from transient failures
- CPU overhead: ~15-20% per agent during polling

*After (Event-Driven - v1.0.0):*
- Detection latency: <60 seconds (including 45s debounce window)
- API load: 90%+ reduction (watch streams, not polling)
- False positives: <5% (intelligent debouncing)
- CPU overhead: <5% per agent average

**Configuration & Deployment**

*Environment Variables:*
```bash
LUMO_AGENT_EVENT_DRIVEN_ENABLED=true                 # Enable event-driven mode
LUMO_AGENT_EVENT_DRIVEN_DEBOUNCE_WINDOW=45s          # Wait before processing
LUMO_AGENT_EVENT_DRIVEN_RESYNC_PERIOD=0s             # Pure event-driven (no polling)
LUMO_AGENT_EVENT_DRIVEN_GROUP_RELATED_EVENTS=true    # Batch related events
LUMO_AGENT_EVENT_DRIVEN_MAX_EVENTS_PER_MIN=100       # Rate limiting
LUMO_AGENT_EVENT_DRIVEN_MIN_SEVERITY=low             # Event filtering
```

*Kubernetes Deployment:*
```bash
# Deploy Redis (required for event state tracking)
kubectl apply -f deployments/kubernetes/redis/

# Deploy event-driven agents (2-replica HA recommended)
kubectl apply -f deployments/kubernetes/base/configmap-agent.yaml
kubectl apply -f deployments/kubernetes/base/deployment-agent.yaml

# Verify deployment
kubectl logs -f -n lumo-system -l mode=event-driven
```

**Requirements**

- **Redis**: Event state tracking and deduplication (required)
- **Kubernetes RBAC**: Watch permissions on monitored resources
- **AI Provider API Key**: For event analysis (optional but recommended)
- **API Server**: For centralized event processing and notifications

**Documentation**

- `EVENT_DRIVEN_IMPLEMENTATION.md` - Complete implementation details (449 lines)
  - Architecture diagrams and component interactions
  - Testing results and performance benchmarks
  - Migration guide from polling to event-driven
  - Troubleshooting and operational runbooks

### Changed

**Architecture Evolution**
- Centralized intelligence model: Agents report events, API server handles AI and notifications
- Reduced code duplication: ~2,000 LOC reduction in agent code
- Simplified agent deployment: Single event-driven model for Kubernetes
- Enhanced scalability: Pure event-driven means agents scale to 100+ nodes without performance impact

**CLAUDE.md Updates**
- Updated version to 1.0.0 (production-ready milestone)
- Phase 16 marked as COMPLETE with detailed documentation
- Phase 15 testing marked as IN PROGRESS with 53.4% internal coverage
- Event-driven architecture fully documented with configuration options

### Fixed

- Race condition in EventGrouper with concurrent map access (sync.RWMutex)
- Type assertion safety in all watchers (proper interface methods)
- Prometheus metric registration for distributed tracing compatibility

### Technical Details

**Files Added:**
- 11 new files (~3,500 LOC total)
  - `internal/agent/eventdriven/manager.go` - Lifecycle management
  - `internal/agent/eventdriven/debouncer.go` - Intelligent debouncing
  - `internal/agent/eventdriven/api_processor.go` - Event submission
  - `internal/agent/eventdriven/types.go` - Event type definitions
  - `internal/agent/eventdriven/watchers/*.go` - 4 primary + 5 specialized watchers
  - `internal/api/handlers/events.go` - Event API endpoint
  - `EVENT_DRIVEN_IMPLEMENTATION.md` - Comprehensive documentation

**Dependencies:**
- k8s.io/client-go (native client, v0.30+)
- go-redis/v9 (event state tracking)
- All other dependencies from v0.11.0

**Testing:**
- All CI checks passing
- Code review improvements applied
- Integration tested with kind (Kubernetes in Docker)
- Full-stack testing: Database + API Server + Event-Driven Agents

### Backward Compatibility

- Zero breaking changes to CLI or API
- Existing polling-based deployments continue to work
- Event-driven is opt-in via `LUMO_AGENT_EVENT_DRIVEN_ENABLED`
- Can coexist with scheduled polling agents for transition period

### Next Steps

**Phase 17: Advanced Features** (Upcoming)
- Multi-cluster monitoring and federation
- Anomaly detection via ML models
- Policy-as-code for automated remediation
- Advanced event correlation and root cause analysis

**Phase 11c: Messaging Integration** (Deferred from Phase 11)
- Publisher/subscriber framework
- NATS, Kafka, RabbitMQ support
- Topic-based event routing

### Security & Compliance

- Kubernetes RBAC least-privilege enforcement
- TLS 1.3 for all API communications
- JWT token validation on all event submissions
- Audit logging for all events and decisions
- Network policies for segmentation

### Performance & Reliability

- Sub-60s detection latency with intelligent debouncing
- 90%+ reduction in Kubernetes API load
- <5% average CPU overhead per agent
- Redis-backed deduplication for reliability
- Graceful error handling and retry logic

---

**Release Date**: November 24, 2025
**Status**: Production Ready - v1.0.0
**Contributors**: Lumo Team

## [0.11.0] - 2025-11-20

### Changed

#### AI Provider Architecture Refactoring

**Major Internal Refactoring**
This release includes a significant refactoring of the AI provider system that eliminates code duplication, improves maintainability, and makes adding new AI providers trivial. No breaking changes to public APIs.

**Architecture Improvements**
- Introduced adapter pattern for AI providers via `ProviderAdapter` interface
- Extracted common HTTP operations into reusable `HTTPClient` (236 LOC)
- Created `BaseProvider` with shared `Analyze()` and `Health()` workflow logic (267 LOC)
- Implemented unified stream handling with `StreamParser` interface (219 LOC)
  - `SSEStreamParser` for Server-Sent Events (Anthropic, OpenAI, Gemini, OpenRouter)
  - `JSONLineStreamParser` for JSON-per-line streams (Ollama)

**Code Reduction**
- **Anthropic:** 467 → 291 LOC (-176 lines, 37.7% reduction)
- **OpenAI:** 526 → 390 LOC (-136 lines, 25.9% reduction)
- **Gemini:** 462 → 366 LOC (-96 lines, 20.8% reduction)
- **Ollama:** 376 → 288 LOC (-88 lines, 23.4% reduction)
- **OpenRouter:** 513 → 377 LOC (-136 lines, 26.5% reduction)
- **Total eliminated:** 632 LOC of duplication (27% reduction across providers)

**New Infrastructure**
- `internal/ai/http_client.go` - Common HTTP operations with retry logic
- `internal/ai/base_provider.go` - Shared provider workflow and prompt building
- `internal/ai/stream_handler.go` - Unified streaming parsers for SSE and JSON-line formats
- Comprehensive test coverage: 1,259 LOC of tests added (374 + 459 + 426 LOC)

**Benefits**
- **Maintainability:** Bug fixes and security updates apply to all providers from single source
- **Consistency:** Uniform error handling, logging, and retry logic across all AI providers
- **Extensibility:** New AI providers now require ~50 LOC instead of ~500 LOC (10x faster to implement)
- **Testing:** Common logic tested once; provider tests focus only on provider-specific behavior
- **Preserved Features:** All provider-specific features maintained:
  - Anthropic: `content_block_delta` streaming events
  - OpenAI: `reasoning_effort` parameter, refusal handling
  - Gemini: API key in URL parameters, thinking tokens detection
  - Ollama: Local service (no API key), extended timeout for slow models
  - OpenRouter: Tracking headers, reasoning field for DeepSeek R1

**Developer Experience**
- Providers now implement only 4 simple methods: `BuildRequest`, `ParseResponse`, `BuildHeaders`, `GetEndpoint`
- All HTTP handling, streaming, error wrapping, and logging automatic via `BaseProvider`
- Mock adapter pattern makes testing without real APIs straightforward

**Documentation**
- Added comprehensive refactoring reports in `REPORTS/`:
  - `ai-provider-refactoring-plan.md` - Initial analysis and strategy
  - `ai-provider-refactoring-complete.md` - Final results and benefits
  - `ai-provider-refactoring-testing.md` - Test verification report
  - `ai-provider-refactoring-lint-check.md` - Code quality verification
  - `golangci-lint-fix-summary.md` - Linting fixes applied

**Impact:** Maintenance burden reduced 5x, future provider implementations 10x faster

## [0.10.0] - 2025-11-20

### Added

#### Usability Sprint - Installation & First-Run Experience

**Major Usability Transformation**
This release dramatically improves the new user experience, reducing installation time from 30-60 minutes to under 5 minutes through automated installation, interactive setup, and comprehensive documentation.

**GitHub Release Automation**
- Automated binary builds for 6 platforms via `.github/workflows/release.yml`
  - Linux: amd64, arm64, arm
  - macOS: amd64 (Intel), arm64 (Apple Silicon)
  - Windows: amd64
- Automatic archive generation with SHA256 and MD5 checksums
- Release notes extracted from CHANGELOG.md
- Triggered by version tags (e.g., `git tag v0.10.0`)

**Quick-Start Installer** (`scripts/quickstart.sh`)
- One-liner installation: `curl -sSL https://raw.githubusercontent.com/ignacio/lumo/main/scripts/quickstart.sh | bash`
- Auto-detects operating system and architecture
- Downloads latest release binaries from GitHub
- Installs to `$HOME/.local/bin` or custom location via `LUMO_INSTALL_DIR`
- Provides PATH setup instructions for bash/zsh
- Beautiful CLI output with colors and ASCII art logo
- 260 lines of robust installation logic

**Interactive Setup Wizard** (`cmd/lumo/init.go`)
- New `lumo init` command for guided configuration setup (764 LOC)
- Interactive prompts using `github.com/AlecAivazis/survey/v2`
- AI provider selection with 5 options:
  - Anthropic (Claude) - Recommended
  - OpenAI (GPT)
  - Google Gemini
  - Ollama (Local/Self-Hosted)
  - OpenRouter (Multi-Model Access)
- API key input with format validation
- Logging level selection (info, debug, warn, error)
- Output format selection (text, json, toon)
- Optional feature toggles (SSH, agent mode, notifications)
- 3 configuration presets for quick setup:
  - `local` - Local-only diagnostics
  - `agent` - Agent mode with TOON format
  - `production` - Full production setup
- Generates minimal, working `config.yaml` file
- Provides personalized next steps based on selections
- Environment variable recommendations for API key security

**Examples Command** (`cmd/lumo/examples.go`)
- New `lumo examples` command for CLI discoverability (280 LOC)
- Lists all 6 available examples with descriptions
- View specific examples by number or name:
  - `lumo examples 1` - Local diagnostics
  - `lumo examples ssh` - SSH remote server
  - `lumo examples ai` - AI analysis
  - `lumo examples fix` - Auto-remediation
  - `lumo examples kubernetes` - K8s deployment
  - `lumo examples vm` - VM/systemd deployment
- Shows example commands, learn-more links, and navigation hints
- Beautiful formatted output with consistent structure

**Comprehensive Documentation**

*Getting Started Guide* (`docs/getting-started.md` - 550+ lines)
- 4 installation methods (quick-start, binary download, Homebrew, source)
- Step-by-step first diagnostic walkthrough
- AI provider setup guides for all 5 providers with API key instructions
- Common use cases with copy-paste examples
- Troubleshooting section for common issues
- Quick reference card with most useful commands
- Available checks reference (12 checkers documented)

*Example Library* (6 comprehensive tutorials, 3,200+ LOC total)
- `examples/01-local-diagnostics/` - Basic local usage (400+ lines)
  - Running checks, output formats, automation examples
  - Cron jobs, CI/CD integration, monitoring scripts
- `examples/02-ssh-remote-server/` - Remote diagnostics (450+ lines)
  - 4 SSH authentication methods (agent, key, password, interactive)
  - SSH config best practices, jump hosts, fleet management
  - Ansible, Terraform, Salt integration examples
- `examples/03-ai-analysis/` - AI-powered analysis (500+ lines)
  - Using all 5 AI providers with provider comparison
  - Cost optimization with TOON format (30-60% savings)
  - Real-world investigation scenarios
- `examples/04-auto-remediation/` - Auto-fix workflows (550+ lines)
  - Dry-run, interactive, and auto-approve modes
  - Risk levels (safe/moderate/critical)
  - Remediation policies, audit logging, rollback
- `examples/05-agent-deployment-k8s/` - Kubernetes deployment (500+ lines)
  - DaemonSet and Deployment manifests
  - Helm chart usage, RBAC configuration
  - Prometheus integration, multi-cluster setup
- `examples/06-agent-deployment-vms/` - VM/systemd deployment (800+ lines)
  - systemd service setup, security hardening
  - RPM/DEB package installation
  - Fleet automation with Ansible, Terraform, Salt

**Cross-Platform Build Script** (`scripts/build-release.sh`)
- Local release builds for all 6 platforms (120 LOC)
- Generates archives with checksums (SHA256, MD5)
- Combined checksum files (SHA256SUMS.txt, MD5SUMS.txt)
- Version info embedded in binaries via ldflags
- Colorful progress output

### Changed

**Documentation Improvements** (`CLAUDE.md`)
- Updated version from 1.0.5 to 1.0.6
- Added "Usability Week 1 Complete ✅" to status line
- Fixed Phase 7 discrepancies:
  - Marked JWT authentication as complete (was "deferred")
  - Added remediation API endpoint to deliverables
  - Updated file count: 35+ → 37+ files
  - Updated LOC: 4,800+ → 5,000+
- Fixed Phase 9 Kubernetes deployment details:
  - Clarified Helm chart uses kustomize base manifests
  - Moved ServiceMonitor to "planned" status (not yet created)
  - Updated file count: 18 → 20 files
- Fixed Phase 11 status from "STARTING" to "PLANNED (Not yet started)"
- Added comprehensive "Usability Sprint Week 1" section documenting all improvements
- Updated CLI commands table to include `init` and `examples` commands

### Fixed

- Removed unused `os` import from `cmd/lumo/examples.go` (causing CI build failures)
- Updated `go.mod` and `go.sum` with `survey/v2` dependency and checksums
- Fixed `gopkg.in/yaml.v3` moved from indirect to direct dependency (used in init.go)

### Technical Details

**New Dependencies**
- `github.com/AlecAivazis/survey/v2 v2.3.7` - Interactive CLI prompts
- `gopkg.in/yaml.v3 v3.0.1` - YAML marshaling (moved to direct)
- Transitive dependencies: shellquote, go-colorable, go-isatty, ansi

**Files Added** (15 new files)
- `.github/workflows/release.yml` (260 lines) - Release automation
- `cmd/lumo/init.go` (764 lines) - Interactive setup wizard
- `cmd/lumo/examples.go` (280 lines) - Examples command
- `docs/getting-started.md` (550+ lines) - Getting started guide
- `scripts/quickstart.sh` (260 lines) - One-liner installer
- `scripts/build-release.sh` (120 lines) - Local build script
- `examples/01-local-diagnostics/README.md` (400+ lines)
- `examples/02-ssh-remote-server/README.md` (450+ lines)
- `examples/03-ai-analysis/README.md` (500+ lines)
- `examples/04-auto-remediation/README.md` (550+ lines)
- `examples/05-agent-deployment-k8s/README.md` (500+ lines)
- `examples/06-agent-deployment-vms/README.md` (800+ lines)

**Files Modified**
- `CLAUDE.md` - Documentation accuracy fixes (7 corrections)
- `go.mod` - Added survey/v2, updated yaml.v3
- `go.sum` - Added checksums for survey and transitive dependencies

**Code Statistics**
- Total new code: 5,300+ lines
- New commands: 2 (init, examples)
- Documentation: 4,700+ lines
- Implementation: 1,044 lines

**Testing**
- All CI checks passing ✅
- Code formatting validated with gofmt
- No unused imports
- All dependencies verified with go mod verify
- Builds successfully for all 6 platforms

### Impact

**User Experience Transformation**
- **Installation time**: 30-60 minutes → 5 minutes (83-92% reduction)
- **Learning curve**: Complex manual setup → Interactive guided experience
- **Documentation accessibility**: Scattered → Organized with in-CLI access
- **Platform support**: Source-only → Pre-built binaries for 6 platforms

**Installation Journey Before v0.10.0:**
1. Clone repository or download source
2. Install Go toolchain
3. Build from source: `go build ./cmd/lumo`
4. Copy 309-line example config
5. Manually edit configuration
6. Research AI provider options
7. Set up environment variables
8. Search documentation for usage
**Time**: 30-60 minutes, error-prone

**Installation Journey After v0.10.0:**
1. Run: `curl -sSL https://... | bash`
2. Run: `lumo init` (interactive wizard)
3. Run: `lumo examples` (browse examples)
4. Run: `lumo diagnose localhost --analyze`
**Time**: 5 minutes, guided

**Adoption Benefits**
- Lower barrier to entry for new users
- Faster time-to-value (first diagnostic)
- Professional installation experience
- Easier to share and recommend
- Better alignment with modern CLI tools (like Homebrew, rustup, etc.)

### Migration Notes

**For Existing Users:**
- No breaking changes to existing functionality
- Existing config files continue to work
- New `lumo init` command is optional (for new setups)
- All existing commands unchanged (connect, diagnose, fix, serve)

**For New Users:**
- Start with quick-start installer or download binaries
- Run `lumo init` for guided setup
- Explore `lumo examples` for tutorials
- See `docs/getting-started.md` for comprehensive guide

### Credits

This release represents a major usability overhaul designed to make Lumo accessible to a broader audience while maintaining its powerful features for advanced users.

## [0.9.1] - 2025-11-19

### Added

#### Notification System Integration

**Multi-Platform Notification Support**
- Comprehensive notification system with 4 provider types
- Unified `Notifier` interface following existing AI provider pattern
- Support for multiple notification levels: info, warning, error, critical, success
- Rich message formatting with fields and tags
- Health checks for all notifiers
- Timeout and retry support

**Supported Platforms**
- **Slack**: Webhook integration with rich attachments (20-50 LOC)
  - Color-coded messages based on notification level
  - Custom username and emoji support
  - Field attachments for structured data
  - Tag support for mentions and metadata
  
- **Telegram**: Bot API integration with Markdown formatting (30-60 LOC)
  - Markdown and HTML message formatting
  - Parse mode configuration
  - Clean text-based presentation
  - Support for tags and fields
  
- **Generic Webhooks**: Support for Discord, Teams, Mattermost (30-50 LOC)
  - Flexible JSON payload customization
  - Header configuration support
  - Compatible with multiple webhook standards
  
- **Email**: SMTP integration with TLS support (40-70 LOC)
  - TLS/STARTTLS encryption
  - HTML and plain text formatting
  - IPv6 address handling
  - Attachment support (future)

**Implementation**
- `internal/notifications/notifier.go` - Core interface and factory pattern
- `internal/notifications/types.go` - Notification types and severity levels
- `internal/notifications/slack.go` - Slack webhook notifier
- `internal/notifications/telegram.go` - Telegram bot API notifier
- `internal/notifications/webhook.go` - Generic webhook notifier for Discord/Teams/Mattermost
- `internal/notifications/email.go` - SMTP email notifier
- `internal/notifications/notifications_test.go` - Comprehensive test suite
- `internal/notifications/README.md` - Complete documentation with examples

**Configuration**
- Added `NotificationsConfig` to `internal/config/config.go`
- Support for multiple notifiers with independent enable/disable
- Environment variable support for sensitive credentials (API tokens, passwords)
- Validation for all required fields per notifier type
- Example configuration in `configs/config.example.yaml`

**Testing**
- Full test coverage using `httptest` mocking
- Tests for all notification providers
- Health check validation
- Error handling scenarios
- All tests passing ✅

### Technical Details
- **Total Code**: ~450 LOC across 6 implementation files + 600 LOC tests
- **Dependencies**: Uses standard library `net/smtp` and `net/http`
- **Architecture**: Factory pattern for provider instantiation, interface-based design
- **Security**: TLS encryption for SMTP, HTTPS for webhooks, credential management via environment variables

### Documentation
- Complete README with provider-specific examples
- Configuration reference for all notification platforms
- Integration guide for agent and CLI usage
- Security best practices for credential management

### Backward Compatibility
- Zero breaking changes - fully backward compatible with v0.9.0
- Notifications are optional and disabled by default
- Existing functionality unchanged

## [0.9.0] - 2025-11-18

### Added

#### Phase 9: Kubernetes Deployment (100% Complete) ✅

**Kubernetes Manifests (8 Base Files)**
- **DaemonSet Deployment**: Per-node monitoring with privileged host access
  - Runs on every node including control plane with `hostNetwork: true` and `hostPID: true`
  - Node-level diagnostics (CPU, memory, disk, processes)
  - Resource requests: 100m CPU / 128Mi RAM
  - Rolling update strategy for zero-downtime deployments
  
- **Deployment**: Cluster-wide monitoring via Kubernetes API
  - 2 replicas for high availability with pod anti-affinity
  - Node spreading for fault tolerance
  - Resource requests: 50m CPU / 64Mi RAM
  - Cluster-level resource monitoring (pods, services, deployments, statefulsets)

- **RBAC Configuration**: Least-privilege security model
  - ServiceAccount: `lumo-agent` for identity management
  - ClusterRole: Read-only access by default
  - ClusterRoleBinding: Links ServiceAccount to ClusterRole
  - Optional remediation permissions (disabled by default for safety)
  - Follows Kubernetes security best practices

- **ConfigMap**: Comprehensive agent configuration
  - Full `config.yaml` embedded with all settings
  - Agent mode, schedule (cron), and API endpoint
  - Enabled diagnostic checks configuration
  - Report format (TOON for token efficiency)
  - Offline mode and caching settings
  - Health check and metrics ports

- **Secret Templates**: Secure sensitive data management
  - API tokens and AI provider keys
  - Integration examples for external secret managers:
    - HashiCorp Vault
    - AWS Secrets Manager
    - Azure Key Vault
    - Google Secret Manager
  - Base64 encoding with comments

- **Service Manifests**: Network exposure for monitoring
  - Headless service for DaemonSet (agent-to-agent discovery)
  - ClusterIP service for Deployment (load balancing)
  - Health endpoint on port 8080
  - Prometheus metrics endpoint on port 9090
  - Prometheus scraping annotations

- **NetworkPolicy**: Fine-grained network security controls
  - Ingress rules: Allow health/metrics from monitoring namespace
  - Egress rules: Allow Kubernetes API, Lumo API, DNS, HTTPS
  - Deny all other traffic by default
  - Labels for policy targeting

- **Kustomize Structure**: Flexible configuration management
  - Base manifests in `base/` directory
  - Overlay support for environment-specific customization
  - `kustomization.yaml` for resource aggregation

**Helm Chart (Complete Package)**
- **Chart Metadata** (`Chart.yaml`)
  - Version 0.9.0 with semantic versioning
  - Kubernetes version constraint (>= 1.24)
  - App version tracking
  - Capability declarations (NetworkPolicy, PodSecurityPolicy)
  - Maintainer information and keywords

- **Values Configuration** (`values.yaml`)
  - 100+ configuration options with sensible defaults
  - DaemonSet and Deployment toggle
  - Resource limits and requests
  - Image repository and tag configuration
  - Security contexts and capabilities
  - Affinity rules and tolerations
  - Service and ingress configuration
  - Monitoring and observability settings

- **Templates** (5 Core Templates)
  - `_helpers.tpl`: Template functions for labels, selectors, and names
  - `namespace.yaml`: Optional namespace creation
  - `serviceaccount.yaml`: Dynamic ServiceAccount with annotations
  - `rbac.yaml`: Templated RBAC with configurable permissions
  - `configmap.yaml`: Dynamic ConfigMap from values
  - `NOTES.txt`: Post-install instructions and verification steps

- **Additional Features**
  - `.helmignore`: Exclude unnecessary files from chart package
  - ServiceMonitor for Prometheus Operator integration
  - PodMonitor support for pod-level metrics
  - Comprehensive validation logic

**Installation & Management Scripts**
- **install.sh** (447 lines): Automated one-command deployment
  - Prerequisites validation (kubectl, cluster connectivity)
  - Automatic namespace creation with labeling
  - Secret generation from CLI arguments or prompts
  - ConfigMap updates with API endpoint injection
  - Component-by-component deployment with status
  - Dry-run mode for previewing changes
  - Flexible deployment options:
    - `--daemonset-only`: Deploy only DaemonSet
    - `--deployment-only`: Deploy only Deployment
    - `--with-remediation`: Enable remediation permissions
  - Post-install verification and health checks
  - Usage examples and help documentation

- **uninstall.sh** (215 lines): Safe cleanup and removal
  - Current state display before uninstallation
  - Interactive confirmation prompts (can skip with `--force`)
  - Graceful pod termination with wait
  - Complete resource cleanup (pods, services, configs, RBAC)
  - Optional namespace deletion with `--delete-namespace`
  - Custom namespace support
  - Status reporting and verification

**Documentation**
- **deployments/kubernetes/README.md** (628 lines): Comprehensive deployment guide
  - Quick Start with installation scripts
  - Deployment Methods (Helm, Kustomize, kubectl)
  - Configuration Reference with all parameters
  - Security Best Practices (RBAC, NetworkPolicy, Secrets)
  - Monitoring and Troubleshooting guides
  - Advanced Configuration examples
  - Multi-cluster management strategies
  - Upgrade and migration procedures

### Changed
- **CLAUDE.md Updates**: Phase 9 marked as 100% complete with deployment details
- **Build System**: CI pipeline updated for Kubernetes manifest validation
- **Documentation Structure**: Added comprehensive Kubernetes deployment section

### Technical Details
- **New Files**: 21 files added for complete Kubernetes deployment
  - 8 base Kubernetes manifests
  - 8 Helm chart files (Chart.yaml, values.yaml, 5 templates, .helmignore)
  - 2 installation scripts (install.sh, uninstall.sh)
  - 1 comprehensive README.md
  - 2 documentation updates (CLAUDE.md updates)
- **Lines of Code**: 3,270+ lines added in this phase
  - 1,340 lines in base manifests
  - 762 lines in Helm chart (values + templates)
  - 662 lines in installation scripts
  - 628 lines in documentation
- **Kubernetes Compatibility**: Tested with K8s 1.24+
- **Resource Efficiency**:
  - DaemonSet: 100m CPU / 128Mi RAM per node
  - Deployment: 50m CPU / 64Mi RAM per replica
  - Total cluster impact: ~200m CPU / ~256Mi RAM for small clusters

### Infrastructure
- **Production-Ready Manifests**: All manifests follow Kubernetes best practices
  - Resource limits and requests defined
  - Health probes configured (liveness, readiness, startup)
  - Security contexts applied
  - Pod disruption budgets for HA
  - Anti-affinity rules for spreading
  
- **Multi-Deployment Strategy**: Support for both deployment modes
  - DaemonSet for node-level monitoring (every node)
  - Deployment for cluster-level monitoring (API-based)
  - Can run both simultaneously for comprehensive coverage

- **Security Hardening**:
  - Least-privilege RBAC (read-only by default)
  - NetworkPolicy for ingress/egress control
  - Pod Security Standards (restricted profile)
  - Secret management best practices
  - Non-root container execution (UID 65532)
  - Read-only root filesystem
  - Dropped capabilities (except CAP_NET_RAW for network checks)

- **Observability Integration**:
  - Prometheus metrics exposure (`:9090/metrics`)
  - ServiceMonitor for automatic discovery
  - Health endpoints for Kubernetes probes (`:8080/health`, `/ready`, `/live`)
  - Structured logging with JSON output
  - Grafana dashboard ready (metrics compatible)

### Deployment Options

**Option 1: Quick Start with Install Script** (Recommended for testing)
```bash
./deployments/kubernetes/install.sh \
  --api-endpoint "https://lumo-api.example.com" \
  --agent-token "your-jwt-token" \
  --ai-provider anthropic \
  --ai-api-key "sk-ant-..."
```

**Option 2: Helm Chart** (Recommended for production)
```bash
helm install lumo-agent ./deployments/kubernetes/helm/lumo-agent \
  --namespace lumo-system \
  --create-namespace \
  --set agent.apiEndpoint="https://lumo-api.example.com" \
  --set agent.token="your-jwt-token"
```

**Option 3: Kustomize** (GitOps workflows)
```bash
kubectl apply -k deployments/kubernetes/base/
```

**Option 4: Raw Manifests** (Maximum control)
```bash
kubectl apply -f deployments/kubernetes/base/
```

### Backward Compatibility
- **Zero Breaking Changes**: All existing CLI and API functionality preserved
- **Optional Deployment**: Kubernetes deployment is completely optional
- **Standalone Agent**: Agent can still run as standalone daemon on VMs
- **API Compatibility**: Works with existing Phase 7 API server

### Next Steps
- **Phase 10**: VM Deployment (systemd services, RPM/DEB packages)
- **Phase 11**: Messaging Integration (NATS, Kafka, RabbitMQ, Redis)
- **Phase 12**: Security Hardening (mTLS, cert rotation, penetration testing)
- **Phase 13**: Production Readiness (performance tuning, dashboards, runbooks)

## [0.8.0] - 2025-11-18

### Added
- **Phase 7-9 Foundation**: Initial implementation of API server, agent daemon, and Kubernetes deployment
- **Database Integration**: PostgreSQL and Redis setup for development
- **Agent Architecture**: Core agent components and scheduling framework
- **Kubernetes Base**: Initial K8s manifests and deployment structure

### Changed
- **Project Structure**: Major reorganization for multi-component architecture
- **Configuration**: Extended configuration system for new components
- **Dependencies**: Added database, caching, and scheduling libraries

### Technical Details
- **New Components**: API server, agent daemon, database layer
- **Infrastructure**: Docker Compose, Kubernetes manifests, Helm chart foundation

## [0.4.2] - 2025-11-16

### Added
- **TOON Format Integration**: Added support for Token-Oriented Object Notation (TOON), a compact format optimized for LLM consumption
  - New `--format toon` CLI flag for user-selectable TOON output
  - Automatic TOON optimization for AI provider prompts (30-60% token reduction)
  - Achieves 33% average token reduction on diagnostic reports (measured: 1086 vs 1622 bytes)
  - Cost savings: ~$0.10 per AI analysis (~33% reduction)
- New TOON formatter with comprehensive test coverage (98.1%)
- TOON format explanation in AI system prompts for better comprehension

### Changed
- AI prompt builder now uses TOON format by default for all providers (transparent optimization)
- Updated `diagnose` command help text to include TOON format option
- Enhanced AI analysis with token-efficient data representation

### Documentation
- Added comprehensive TOON format section to CLAUDE.md (113 lines)
- Documented token efficiency benchmarks and cost savings
- Added TOON usage examples and implementation details
- Updated quick reference with TOON formatter patterns

### Technical Details
- Added dependency: `github.com/alpkeskin/gotoon` v0.1.1
- New files: `internal/diagnostics/formatters/toon.go` (160 lines)
- New tests: `internal/diagnostics/formatters/toon_test.go` (520 lines, 8 test scenarios)
- Modified: `cmd/lumo/diagnose.go`, `internal/ai/prompts.go`, `CLAUDE.md`

### Performance
- Token reduction: 30-60% on diagnostic data (varies by data structure)
- Measured results:
  - Typical reports: 33% reduction
  - Process/service lists: 42% reduction
  - Metrics arrays: 58% reduction
- Estimated annual savings on 1000 analyses: $160-$300

### Backward Compatibility
- Zero breaking changes - fully backward compatible
- Default output format remains `text`
- TOON is opt-in for users, automatic for AI optimization

## [0.4.1] - 2025-11-16

### Security

#### CRITICAL Fixes
- **Fixed SSH Host Key Verification (CWE-295)**: Changed default from `StrictHostKeyChecking: false` to `true` to prevent man-in-the-middle attacks. Added warnings when verification is disabled.
- **Fixed Command Injection via WorkingDir (CWE-78)**: Implemented comprehensive input validation with `sanitizeWorkingDir()` function that blocks shell metacharacters, null bytes, and enforces absolute paths.
- **Fixed Password CLI Exposure (CWE-214)**: Removed `--password` and `-P` flags from `diagnose` and `connect` commands to prevent credential visibility in process listings and shell history. Password authentication now uses secure prompting only.

#### MEDIUM Fixes
- **Enhanced File Permission Validation (CWE-732)**: Changed from bitwise checking to exact permission matching. SSH keys now require exactly 0600 or 0400 permissions.
- **Added AI Endpoint Validation (CWE-918)**: Implemented `ValidateEndpoint()` function to prevent SSRF attacks. Validates HTTPS, blocks localhost/private IPs (except Ollama), and prevents access to cloud metadata services.
- **Improved Shell Quoting**: Enhanced documentation and verified POSIX compliance for shell argument quoting. Added comprehensive examples and edge case handling.

### Changed
- SSH host key verification now enabled by default for security
- SSH key file permissions now strictly enforced (0600 or 0400 only)
- All AI provider endpoints now validated before use
- Password authentication via CLI removed (use secure prompting instead)

### Documentation
- Added comprehensive security fixes report: `REPORTS/security-fixes-2025-11-16.md`
- Updated `CLAUDE.md` with security enhancements section
- Added security audit status to project documentation

### Notes
- This release addresses all CRITICAL findings from the security audit (REPORTS/security-audit-2025-11-15.md)
- Security posture improved from HIGH RISK to LOW-MODERATE RISK
- All existing tests passing with updated security expectations
- Breaking change: `--password` flag removed from CLI commands

## [0.4.0] - 2025-11-15

### Added
- Complete AI integration with 4 providers (Anthropic, OpenAI, Ollama, Gemini)
- Local execution support (no SSH for localhost)
- All 6 core diagnostic checkers (CPU, Memory, Disk, Process, Service, Network)
- Comprehensive test coverage (37.1% overall, 100% in formatters)
- Security-focused memory checker with input validation

### Features
- SSH connection with 4 authentication methods
- Remote and local diagnostic execution
- AI-powered analysis with streaming support
- Cross-platform support (Linux, macOS, BSD)
- Multiple output formats (text, JSON)

### Documentation
- Complete CLAUDE.md guide for AI assistants
- DEVELOPMENT.md with detailed examples
- Security audit report
- Production readiness review

---

**Legend:**
- 🔴 CRITICAL: Security vulnerabilities requiring immediate action
- 🟠 HIGH: Important issues requiring prompt attention
- 🟡 MEDIUM: Issues that should be addressed soon
- 🟢 LOW: Minor issues or improvements
