# CLAUDE.md Detailed Archive

> **Date:** 2025-11-17
> **Purpose:** Historical archive of detailed content removed from CLAUDE.md to reduce token usage
> **Original Size:** 1,213 lines → **Trimmed to:** 255 lines (79% reduction)

This document contains the detailed phase descriptions, testing roadmaps, and verbose examples that were removed from CLAUDE.md for token efficiency. The main CLAUDE.md now contains only essential information for current development work.

---

## Table of Contents

1. [Detailed Phase Descriptions (1-6)](#detailed-phase-descriptions)
2. [Testing & Documentation Deep Dive](#testing--documentation-deep-dive)
3. [Roadmap to 80% Coverage](#roadmap-to-80-coverage)
4. [Detailed Configuration Examples](#detailed-configuration-examples)
5. [Component Reference Tables](#component-reference-tables)
6. [TOON Format Examples](#toon-format-examples)
7. [Phase 10 Detailed Plan](#phase-10-detailed-plan)

---

## Detailed Phase Descriptions

### ✅ Phase 1: Foundation (Complete)
- Cobra CLI + Viper config + Logrus logging
- All command stubs registered

### ✅ Phase 2: SSH & Connection (Complete - 2025-11-14)
- 4 auth methods (agent, key, password, interactive)
- Retry logic, health monitoring, auto-reconnect
- 8 files, 2,410 lines

### ✅ Phase 3: Diagnostic System (Complete - 2025-11-14)
- Core runner architecture with `Checker` interface
- All 6 core checkers (CPU, Memory, Disk, Process, Service, Network)
- Severity system, thresholds, formatters (text, JSON)
- Cross-platform support (Linux, macOS, BSD)
- 12 files, 3,835 lines

### ✅ Phase 4: AI Integration (Complete - 2025-11-15)
- 4 AI providers (Anthropic, OpenAI, Ollama, Gemini)
- Streaming + non-streaming analysis
- Prompt engineering for SRE diagnostics
- Token usage tracking
- Temperature standardized to 1.0
- Local execution support (no SSH for localhost)
- 7 files, 1,964 lines (includes tests)

### ✅ Phase 4.2: OpenRouter Integration (Complete - 2025-11-17)
- ✅ **OpenRouter Provider:** Added 5th AI provider with multi-model support
- ✅ **Unified API:** Access multiple LLM providers through single OpenRouter endpoint
- ✅ **Model Flexibility:** Support for Claude, GPT-4, Gemini, and other models via OpenRouter
- ✅ **Configuration:** Added `LUMO_OPENROUTER_API_KEY` environment variable support
- ✅ **Default Model:** Configured `anthropic/claude-sonnet-4.5` as default model
- ✅ **Testing:** Comprehensive tests following existing provider patterns (8 test cases, 100% passing)
- ✅ **Documentation:** Updated config.example.yaml and CLAUDE.md with usage examples
- Delivered: 2 files (openrouter.go: 508 lines, openrouter_test.go: 511 lines)

### ✅ Phase 4.1: Enhanced Diagnostics (Complete - 2025-11-16)
- ✅ **Memory enhancements:** Top 10 memory consumers per process
- ✅ **Page fault tracking:** Total, major, and minor page faults (Linux: /proc/vmstat, macOS: vm_stat)
- ✅ **Memory pressure indicators:** Pages throttled, compressions/decompressions, swapins/swapouts, purged pages
- ✅ **Deep-dive metrics:** Active/Inactive/Wired memory breakdown, File-backed vs Anonymous pages
- ✅ **Pressure warnings:** Automatic alerts in text format when memory pressure is significant
- ✅ **Cross-platform:** Full support for both Linux (/proc/meminfo + /proc/vmstat) and macOS (vm_stat)

### ✅ Phase 5: Security Diagnostics (Complete - 2025-11-16)
- ✅ **Patch Status Checker:** Detects available system updates and security patches across multiple package managers (apt, yum, dnf, apk, pacman)
- ✅ **Open Ports Checker:** Identifies listening services, public vs localhost ports, unexpected ports, with configurable whitelisting
- ✅ **SSH Security Checker:** Validates SSH key permissions (600 for private keys), analyzes sshd_config for security issues (PermitRootLogin, PasswordAuthentication, etc.)
- ✅ **Auth Failures Checker:** Parses authentication logs to detect failed login attempts, brute force attacks, and suspicious IPs
- ✅ **Configuration:** Security settings in config.yaml (whitelisted_ports, lookback_hours, failure_threshold)
- ✅ **Integration:** All 4 checkers registered and working with existing diagnostic framework
- 4 new checkers, ~1,400 lines of code

### ✅ Phase 5.1: Specialized Platform Checkers (Complete - 2025-11-16)

#### Kubernetes Diagnostics
- ✅ **Native Kubernetes Client:** Direct API access using k8s.io/client-go (no kubectl dependency)
- ✅ **8 Resource Types Monitored:** Nodes, Pods, Deployments, StatefulSets, DaemonSets, Services, PVCs, Events
- ✅ **Interface-Based Design:** Uses kubernetes.Interface for testability with fake clients
- ✅ **Comprehensive Health Checks:**
  - Node readiness and conditions
  - Pod phase tracking and restart counts
  - Workload replica health (Deployments, StatefulSets, DaemonSets)
  - Service endpoint validation
  - PVC binding status
  - Recent event monitoring (configurable lookback)
- ✅ **Granular Configuration:** Individual check toggles, namespace filtering, event lookback period
- ✅ **Conditional Registration:** Opt-in model, disabled by default, zero overhead when not used
- ✅ **Full Testing:** 13 test cases (100% passing), mock-based testing with fake Kubernetes clientset
- ✅ **Documentation:** Comprehensive implementation report (1,717 lines) in REPORTS/
- 1 new checker, 846 lines production code + 660 lines tests
- See: REPORTS/kubernetes-diagnostics-implementation-2025-11-16.md

#### Proxmox VE Platform Support
- ✅ **Proxmox VE Checker:** Comprehensive monitoring for Proxmox Virtual Environment
  - Cluster health: Quorum status, node membership, online/offline detection
  - VM/Container status: Running/stopped VMs and LXC containers with resource metrics
  - Storage monitoring: Pool health, usage percentages, active/inactive detection
  - Replication jobs: Status tracking, error detection
  - Backup validation: Recent backup success/failure tracking
  - High Availability: HA service status and managed VM tracking
  - Daemon health: Critical Proxmox service monitoring (pve-cluster, pvedaemon, pveproxy, etc.)
- ✅ **Auto-detection:** Automatically skips check if Proxmox not installed
- ✅ **Selective checking:** Granular control over which Proxmox components to check
- ✅ **Comprehensive tests:** 12 test cases covering all parsing functions and edge cases
- 1 new checker (~900 lines implementation + ~750 lines tests)

### ✅ Phase 6: Auto-Remediation (Complete - 2025-11-17)
**Full auto-remediation system with human-in-the-loop approval and safety controls**

**Core Infrastructure:**
- ✅ **Action Interface:** Standardized interface for all remediation actions (Execute, Validate, Rollback)
- ✅ **Action Registry:** Pluggable registry for managing available actions
- ✅ **Executor System:** Robust execution engine with error handling and state tracking
- ✅ **Approval System:** Human-in-the-loop approval for moderate/critical actions
- ✅ **Audit Logging:** Complete audit trail of all remediation attempts to `~/.lumo/remediation-audit.log`
- ✅ **Suggester:** Intelligent action generation from diagnostic reports

**Risk Classification:**
- ✅ **Safe Actions:** Auto-approved based on policy (cleaning temp files, logs)
- ✅ **Moderate Actions:** User approval required (restarting services, killing processes)
- ✅ **Critical Actions:** Always require explicit approval (system configuration changes)

**Action Categories (11 files, 3,395 lines):**
- ✅ **Disk Actions** (`actions_disk.go`): Clean log files, temp directories, package caches, journal logs
- ✅ **Service Actions** (`actions_service.go`): Restart failed systemd/init/launchd services
- ✅ **Process Actions** (`actions_process.go`): Kill zombie processes, terminate high CPU/memory processes

**CLI Features:**
```bash
lumo fix localhost                    # Detect issues and suggest fixes
lumo fix host --auto-approve         # Auto-approve safe operations
lumo fix host --dry-run              # Simulate without executing
lumo fix host --list-actions         # Show plan without executing
lumo fix host --skip service,disk    # Skip specific categories
lumo fix host --audit-log /path      # Custom audit log location
```

**Safety Features:**
- ✅ Reversible actions track rollback data
- ✅ Dry-run mode for testing
- ✅ Per-action validation before execution
- ✅ Execution reports with duration, changes, errors
- ✅ Status tracking (pending, approved, rejected, success, failed, rolled_back)

**Testing:**
- ✅ E2E integration tests (`e2e_test.go`)
- ✅ Action-specific unit tests
- ✅ Full test coverage of suggestion logic

---

## Testing & Documentation Deep Dive

### ✅ Phase 8: Testing & Documentation (In Progress - Critical Gaps Closed)
**Status:** Critical production CLI paths now fully tested with integration coverage

**Overall Test Coverage: 50.4%** (11,059 lines of test code) - **Updated 2025-11-16 (Final)**

#### 🎯 Critical Production Gaps RESOLVED:

| Critical Handler | Before | After | Impact |
|-----------------|--------|-------|--------|
| **runDiagnostics()** | 0.0% | **62.9%** ✅ | Users running `lumo diagnose` |
| **runAIAnalysis()** | 0.0% | **86.7%** ✅ | Users running `--analyze` |
| **cmd/lumo CLI package** | 34.1% | **61.6%** ✅ | Overall CLI coverage |

#### Package Coverage Status:

| Package | Coverage | Test Files | Status |
|---------|----------|------------|--------|
| **internal/diagnostics/formatters** | 98.1% | text_test.go | ✅ Excellent |
| **internal/diagnostics** | 87.6% | result, severity, runner, executor, testing.go | ✅ Excellent |
| **internal/config** | 68.8% | config_test.go, config_api_key_test.go | ✅ Good |
| **internal/diagnostics/checkers** | 61.4% | cpu, disk, memory, network, process, service tests | ✅ Good |
| **cmd/lumo** | 61.6% | diagnose_test.go, diagnose_integration_test.go, ai_integration_test.go | ✅ Good |
| **internal/ssh** | 30.5% | config, errors, types, retry, session, auth tests | ⚠️ Needs expansion |
| **internal/ai** | 27.1% | prompts_test.go, provider_test.go, testing.go | ⚠️ Needs expansion |

#### Test Files Added (26 files, 11,800+ total lines):

**CMD Package Tests (1,177 lines, 86 test cases):**
- `cmd/lumo/diagnose_test.go` - CLI formatting functions, localhost detection, AI analysis output (67 test cases)
- `cmd/lumo/diagnose_integration_test.go` - **Full integration tests for runDiagnostics** (354 lines, 16 tests)
- `cmd/lumo/ai_integration_test.go` - **Integration tests for runAIAnalysis** (152 lines, 3 tests)

**SSH Package Tests (2,302 lines):**
- `internal/ssh/config_test.go` - ClientConfig validation, key handling
- `internal/ssh/errors_test.go` - All error types, helper functions
- `internal/ssh/types_test.go` - Enums, CommandResult, constants
- `internal/ssh/retry_test.go` - Retry logic, exponential backoff, context handling (23 tests)
- `internal/ssh/session_test.go` - Security tests for shellQuote and sanitizeWorkingDir
- `internal/ssh/auth_test.go` - Key validation, key type detection (18 tests)

**Diagnostics Tests (6,950+ lines):**
- `internal/diagnostics/testing.go` - **MockChecker for integration testing** (65 lines)
- `internal/diagnostics/runner_test.go` - Runner orchestration, parallel/sequential execution (18 tests)
- `internal/diagnostics/executor_test.go` - LocalExecutor with context support (7 tests)
- `internal/diagnostics/checkers/cpu_test.go` - CPU checker with mock executor
- `internal/diagnostics/checkers/disk_test.go` - Disk checker tests
- `internal/diagnostics/checkers/memory_test.go` - Memory checker tests with security validations (52 test cases)
- `internal/diagnostics/checkers/network_test.go` - Network checker + Run() tests
- `internal/diagnostics/checkers/process_test.go` - Process checker tests
- `internal/diagnostics/checkers/service_test.go` - Service checker + Run() tests
- `internal/diagnostics/checkers/proxmox_test.go` - **Proxmox checker with comprehensive tests** (750+ lines, 12 test cases)
- `internal/diagnostics/formatters/text_test.go` - Text formatter (98.1% coverage)
- `internal/diagnostics/result_test.go` - CheckResult and Report types
- `internal/diagnostics/severity_test.go` - Severity and threshold types

**AI Package Tests (895 lines):**
- `internal/ai/testing.go` - **MockProvider for AI testing** (128 lines)
- `internal/ai/prompts_test.go` - PromptBuilder, formatters, parsers
- `internal/ai/provider_test.go` - Provider factory, type validation

**Config Package Tests (237 lines):**
- `internal/config/config_test.go` - Config loading, validation
- `internal/config/config_api_key_test.go` - API key handling

#### Test Patterns Established:

✅ **Mock Executors:** Comprehensive mock command executor for checkers
✅ **Table-Driven Tests:** Used throughout for clarity and coverage
✅ **Error Wrapping Tests:** All custom error types tested
✅ **Method Chaining:** Builder pattern tests verify fluent APIs
✅ **Edge Cases:** Nil values, empty strings, boundary conditions

#### Recent Security & Testing Enhancements:

**Memory Checker Refactoring (2025-11-15):**
- ✅ Fixed command injection vulnerability by extracting shell commands into testable functions
- ✅ Added comprehensive input validation (PID, memory %, RSS values)
- ✅ Implemented command sanitization (null bytes, newlines, truncation)
- ✅ Introduced named constants for all magic numbers
- ✅ Created 48 test cases with mock executor pattern
- ✅ All tests passing, functionality preserved

#### Recent Test Additions (2025-11-16):

**Coverage Improvement: 37.1% → 48.8% (+11.7%)**

**Phase 1 (Morning):**
- ✅ SSH retry tests: 100% coverage on retry.go (exponential backoff, context handling)
- ✅ SSH session helper tests: Security validations for command injection prevention
- ✅ Diagnostics runner tests: 92.1% package coverage (orchestration, parallel/sequential execution)
- ✅ Diagnostics executor tests: LocalExecutor with full context and timeout support
- Coverage: 37.1% → 44.7% (+7.6%)

**Phase 2 (Afternoon):**
- ✅ CMD/lumo diagnostic tests: 67 test cases covering formatting functions, localhost detection, AI analysis output
  - TestIsLocalhost: 14 cases
  - TestFormatHealthStatus: 8 cases
  - TestFormatSeverity: 10 cases
  - TestFormatPriority: 10 cases
  - TestFormatRisk: 12 cases
  - TestFormatAIAnalysisJSON: 3 cases
  - TestFormatAIAnalysisText: 8 cases (including TokensUsed, Commands, EstimatedImpact)
- ✅ SSH auth helper tests: 18 test cases for validateKeyFile and detectKeyType
- ✅ Memory checker getter tests: 4 test cases for trivial interface methods
- Coverage: 44.7% → 48.8% (+4.1%)

**Test Files Created:**
1. `cmd/lumo/diagnose_test.go` (671 lines, 67 tests)
2. `internal/ssh/auth_test.go` (183 lines, 18 tests)
3. Memory checker enhancements (43 lines, 4 tests)

**Phase 2 (Afternoon) - Integration Tests:**
- ✅ **cmd/lumo integration tests**: 506 lines, 19 test cases
  - diagnose_integration_test.go: Full runDiagnostics flow (354 lines, 16 tests)
  - ai_integration_test.go: runAIAnalysis integration (152 lines, 3 tests)
- ✅ **Test infrastructure**: 193 lines
  - internal/diagnostics/testing.go: MockChecker (65 lines)
  - internal/ai/testing.go: MockProvider + helpers (128 lines)
- Coverage: 48.8% → 50.4% (+1.6%)

**Total Added This Session: 1,483 lines, 108 test cases**

**Overall Progress: 37.1% (session start) → 50.4% (current) = +13.3 percentage points**

---

## Roadmap to 80% Coverage

**Current:** 50.4%
**Target:** 80.0%
**Gap:** **+29.6 percentage points needed**

### What's Already Protected ✅

**Production Critical Paths (DONE):**
- ✅ `lumo diagnose localhost` → 62.9% tested
- ✅ `lumo diagnose --analyze` → 86.7% tested
- ✅ CLI flag parsing → Integration tested
- ✅ Localhost detection → 100% tested
- ✅ Diagnostic runner → 87.6% tested
- ✅ All formatters → 98.1% tested

### Remaining High-Impact Work

#### Priority 1: Internal/AI Package (27.1% → 70%+)
**Estimated Impact:** +6-8% overall coverage
**Effort:** ~500-700 lines of tests

**What to Test:**
- `Analyze()` methods for all 5 providers (Anthropic, OpenAI, Ollama, Gemini, OpenRouter)
- `Health()` check integration with HTTP
- `AnalyzeStream()` streaming responses
- HTTP request/response handling with mock servers

**Implementation:**
```go
// Example: internal/ai/anthropic_test.go
func TestAnthropicProvider_Analyze(t *testing.T) {
    server := httptest.NewServer(/* mock Anthropic API */)
    provider := NewAnthropicProvider(config, logger)
    response, err := provider.Analyze(ctx, request)
    // Verify response parsing, token counting, error handling
}
```

**Files to Create:**
- `internal/ai/anthropic_test.go` (150-200 lines)
- `internal/ai/openai_test.go` (150-200 lines)
- `internal/ai/ollama_test.go` (100-150 lines)
- `internal/ai/gemini_test.go` (150-200 lines)
- `internal/ai/openrouter_test.go` (150-200 lines)

#### Priority 2: Internal/SSH Package (30.5% → 70%+)
**Estimated Impact:** +4-5% overall coverage
**Effort:** ~400-500 lines of tests

**What to Test:**
- `Client.Connect()` / `Disconnect()` with mock SSH
- `Client.Execute()` command execution
- `HealthChecker` Start/Stop/monitoring
- Connection state management
- Retry logic integration

**Implementation Pattern:**
```go
// Example: internal/ssh/client_test.go
func TestClient_Connect(t *testing.T) {
    // Mock SSH server or connection
    client := NewClient(config, logger)
    err := client.Connect("test-host", 22, "user")
    // Verify connection state, auth attempts, health checker started
}
```

**Files to Create:**
- `internal/ssh/client_test.go` (200-250 lines)
- `internal/ssh/health_test.go` (150-200 lines)

#### Priority 3: Checkers Edge Cases (61.4% → 85%+)
**Estimated Impact:** +2-3% overall coverage
**Effort:** ~300-400 lines of tests

**What to Test:**
- Parser error handling (malformed command output)
- Cross-platform edge cases (macOS vs Linux output differences)
- Boundary conditions (0%, 100% usage)
- Missing/incomplete data scenarios

**Files to Expand:**
- Enhance existing checker test files with edge cases
- Add platform-specific parsing tests

#### Priority 4: CMD/Lumo Remaining (61.6% → 75%+)
**Estimated Impact:** +1-2% overall coverage
**Effort:** ~200-300 lines of tests

**What to Test:**
- `runConnect()` integration tests
- More `runDiagnostics()` edge cases
- Error path coverage

### Estimated Total to 80%

| Task | Lines | Impact | Status |
|------|-------|--------|--------|
| Internal/AI HTTP tests | 500-700 | +6-8% | ⏳ Not started |
| Internal/SSH client tests | 400-500 | +4-5% | ⏳ Not started |
| Checker edge cases | 300-400 | +2-3% | ⏳ Not started |
| CMD/lumo remaining | 200-300 | +1-2% | ⏳ Not started |
| **TOTAL** | **1,400-1,900** | **+13-18%** | **Reaches 63-68%** |

**Note:** Getting from 68% → 80% would require additional work on lower-impact areas or E2E tests.

### Recommended Next Steps

**Option A: Reach 60%+ (Easiest Path)**
1. Focus on internal/ai HTTP tests (Priority 1)
2. ~500 lines of tests
3. Gets to ~57-58% coverage
4. All critical production paths already protected

**Option B: Reach 70%+ (Moderate Effort)**
1. Complete Priority 1 (AI) and Priority 2 (SSH)
2. ~900-1,200 lines of tests
3. Gets to ~65-68% coverage
4. Comprehensive coverage of all major subsystems

**Option C: Reach 80% (Significant Effort)**
1. Complete all 4 priorities
2. ~1,400-1,900 lines of tests
3. May need additional E2E/integration tests
4. Diminishing returns on critical path protection

### Current Status Summary

**✅ Accomplished:**
- Critical CLI handlers: 0% → 60-87%
- Integration test framework established
- Mock infrastructure created
- Production user flows protected

**⏳ Remaining for 80%:**
- Primarily HTTP and SSH mocking
- Edge case coverage
- ~1,400-1,900 more lines of tests

**💡 Recommendation:**
The critical production gaps are now closed. The remaining work to 80% is primarily testing internal HTTP/SSH implementation details that are less critical than the user-facing CLI paths we've already covered.

---

## Detailed Configuration Examples

### Configuration Hierarchy

Searches in this order (first wins):
1. `--config` flag path
2. `./config.yaml`
3. `~/.lumo/config.yaml`

### Environment Variable Overrides

**Pattern:** `LUMO_<SECTION>_<KEY>`

```bash
# SSH settings
export LUMO_SSH_PORT=2222

# AI provider settings
export LUMO_AI_PROVIDER=openai

# AI API keys - Provider-specific (recommended - allows switching without changing keys)
export LUMO_ANTHROPIC_API_KEY=sk-ant-...   # For Anthropic Claude
export LUMO_OPENAI_API_KEY=sk-...          # For OpenAI GPT
export LUMO_GEMINI_API_KEY=...             # For Google Gemini
export LUMO_OLLAMA_API_KEY=...             # For Ollama (usually not needed)
export LUMO_OPENROUTER_API_KEY=sk-or-...   # For OpenRouter

# AI API key - Generic fallback (works but requires changing when switching providers)
export LUMO_AI_API_KEY=sk-...              # Fallback if provider-specific not set

# AI reasoning effort (for OpenAI reasoning models: o1, o3, gpt-5-nano)
export LUMO_AI_REASONING_EFFORT=low        # low, medium, or high

# Logging
export LUMO_LOGGING_LEVEL=debug
```

### Configuration Sections

```go
type Config struct {
    SSH         SSHConfig         // timeout, port, keepalive, retries
    AI          AIConfig          // provider, models, timeout, temperature
    Logging     LoggingConfig     // level, format, output
    API         APIConfig         // port, host, TLS settings
    Diagnostics DiagnosticsConfig // network targets, thresholds
}
```

**Key AI Settings:**
- `provider`: anthropic | openai | ollama | gemini | openrouter
- `models`: Per-provider model map (defaults in code)
- `temperature`: 1.0 (standardized across all providers)
- `max_tokens`: 4096 (16384 recommended for OpenAI reasoning models)
- `reasoning_effort`: low | medium | high (OpenAI reasoning models only: o1, o3, gpt-5-nano)
  - **Purpose:** Controls reasoning token usage for OpenAI reasoning models
  - **low:** ~1-2k reasoning tokens, faster responses
  - **medium:** ~2-4k reasoning tokens (default if not specified)
  - **high:** ~4-8k reasoning tokens, most thorough analysis
  - **Note:** Only affects OpenAI reasoning models, ignored by other providers

**API Key Security:** NEVER in config file, ONLY via environment variables:
- **Recommended:** Provider-specific env vars (`LUMO_ANTHROPIC_API_KEY`, `LUMO_OPENAI_API_KEY`, `LUMO_GEMINI_API_KEY`, `LUMO_OPENROUTER_API_KEY`, etc.)
- **Fallback:** Generic `LUMO_AI_API_KEY` (requires changing when switching providers)

**Diagnostics Configuration:**
- `network.targets`: List of network endpoints to test connectivity from diagnosed server
  - Each target specifies: `host`, `port`, `protocol` (tcp/icmp)
  - Port 0 = ICMP-only (ping test)
  - TCP targets test port connectivity with latency & error classification
  - Defaults: 8.8.8.8 and google.com via ICMP if no targets configured
  - Example use cases: Test database connectivity, API availability, DNS resolution, cache connectivity
  - Error types: `connection_refused`, `dns_failed`, `timeout`, `unreachable`

**Network Metrics Collected:**
- Interface detection with state tracking (up/down/inactive)
- Active connection counting (Linux: `ss`, macOS/BSD: `netstat`)
- Per-target latency and reachability
- Interface statistics: RX/TX bytes, packets, errors
- Dynamic AI analysis: Only analyzes selected checks, avoids false warnings

---

## Component Reference Tables

### Diagnostic Checkers (12 Total: 6 Core + 4 Security + 2 Specialized)

**Core Checkers:**
| Checker | Key Metrics |
|---------|-------------|
| `cpu.go` | Load average, usage %, core count |
| `memory.go` | RAM/swap usage %, top memory consumers, cross-platform |
| `disk.go` | Space usage %, inode usage %, multi-filesystem |
| `process.go` | Process count, zombies, top consumers |
| `service.go` | Failed services, systemd/init/launchd support |
| `network.go` | Interfaces (Linux `ip`, macOS/BSD `ifconfig` with proper UP/status parsing), connectivity (ICMP/TCP), configurable targets, latency, error classification, active connection counting |

**Security Checkers:**
| Checker | Key Metrics |
|---------|-------------|
| `patch_status.go` | Available updates, security patches (apt/yum/dnf/apk/pacman) |
| `open_ports.go` | Listening services, public vs localhost, whitelisting |
| `ssh_security.go` | SSH key permissions, sshd_config security analysis |
| `auth_failures.go` | Failed login attempts, brute force detection |

**Specialized Checkers:**
| Checker | Key Metrics |
|---------|-------------|
| `kubernetes.go` | Nodes (ready/not-ready), Pods (phase, restarts), Deployments/StatefulSets/DaemonSets (replica health), Services (endpoints), PVCs (binding status), Events (recent warnings/errors) |
| `proxmox.go` | **Core:** Cluster status (quorum, nodes), VMs/containers (running/stopped), storage (usage, health), replication jobs, backup status, HA services, Proxmox daemon health<br>**Enhanced:** Subscription validation, update detection, task history, per-VM/CT performance metrics, boot configuration (OnBoot settings), network interface statistics |

### AI Providers (5 Complete)

| Provider | Model | Key Features | Status |
|----------|-------|-------------|--------|
| `anthropic.go` | claude-sonnet-4-5-20250929 | Streaming, structured output | ✅ Complete |
| `openai.go` | gpt-4-turbo-preview | Function calling, streaming | ✅ Complete |
| `ollama.go` | llama3.1:8b | Self-hosted, no API key | ✅ Complete |
| `gemini.go` | gemini-2.0-flash-exp | SSE streaming, token tracking | ✅ Complete |
| `openrouter.go` | anthropic/claude-sonnet-4.5 | Multi-provider routing, unified API, OpenAI-compatible | ✅ Complete |

### Remediation System (Phase 6 - Complete)

| File | Purpose | Key Points |
|------|---------|-----------|
| `cmd/lumo/fix.go` | Fix command CLI | Flag handling, plan display, execution orchestration |
| `remediation.go` | Core types | `Action` interface, `RiskLevel`, `ActionStatus`, `RemediationPlan` |
| `executor.go` | Execution engine | Action execution, approval integration, error handling, audit logging |
| `approval.go` | Human-in-the-loop | Interactive approval prompts, auto-approve logic for safe actions |
| `audit.go` | Audit logging | JSON-formatted audit trail to `~/.lumo/remediation-audit.log` |
| `suggestions.go` | AI suggester | Generate actions from diagnostic reports, severity-based filtering |
| `registry.go` | Action registry | Register/lookup actions, default registry with all actions |
| `actions_disk.go` | Disk cleanup actions | Clean logs, temp files, package caches, journal logs |
| `actions_service.go` | Service management | Restart failed services (systemd, init, launchd) |
| `actions_process.go` | Process management | Kill zombies, terminate high resource consumers |
| `e2e_test.go` | Integration tests | End-to-end workflow testing |

**Total:** 11 files, 3,395 lines

---

## TOON Format Examples

### Example TOON Output

**JSON (1622 bytes):**
```json
{
  "results": [
    {"name": "cpu_check", "severity": "ok", "message": "CPU normal", ...},
    {"name": "memory_check", "severity": "warning", "message": "High memory", ...}
  ],
  "metrics": [
    {"name": "cpu_usage", "value": 45.5, "unit": "percent"},
    {"name": "memory_usage", "value": 71.2, "unit": "percent"}
  ]
}
```

**TOON (1086 bytes = 33% smaller):**
```toon
results[2]:
  - name: cpu_check
    severity: ok
    message: CPU normal
  - name: memory_check
    severity: warning
    message: High memory
metrics[2]{name,value,unit}:
  cpu_usage,45.5,percent
  memory_usage,71.2,percent
```

### Usage Examples

**User-Selectable Format:**
```bash
lumo diagnose localhost --format toon          # TOON output to user
lumo diagnose user@host --format json          # JSON output
lumo diagnose --format text                    # Human-readable (default)
```

**Automatic AI Optimization:**
When `--analyze` is used, Lumo **automatically** uses TOON format internally for AI prompts while still displaying text/JSON to the user based on `--format`:

```bash
lumo diagnose localhost --analyze              # User sees text, AI gets TOON
lumo diagnose --format json --analyze          # User sees JSON, AI gets TOON
```

### Implementation Details

**Formatter:**
```go
// internal/diagnostics/formatters/toon.go
formatter := formatters.NewToonFormatter()
output := formatter.FormatReport(report)
```

**AI Integration:**
```go
// internal/ai/prompts.go
// TOON is enabled by default for all AI providers
builder := ai.NewPromptBuilder()  // useTOON: true by default
builder.WithTOON(false)           // Disable if needed
```

**Library:**
- Uses `github.com/alpkeskin/gotoon` for encoding
- Automatic type conversion (structs → maps → TOON)
- Deterministic output (sorted keys)

### When TOON Helps Most

✅ **High value:**
- Process lists (100+ processes)
- Service status (50+ services)
- Metrics arrays (10+ metrics per check)
- Network interface data
- Top memory/CPU consumers

⚠️ **Low value:**
- Single check results
- Deeply nested structures
- Already minimal data

---

## Phase 10 Detailed Plan

### ⏳ Phase 10: Messaging & Notifications (Planned)
**Cross-cutting notification system for alerts and team communication**

**Supported Providers:**
- **Chat:** Slack, Microsoft Teams, Discord, Mattermost, Rocket.Chat
- **Messaging:** Telegram (Bot API)
- **Email:** SMTP with HTML/plain text
- **Incident Management:** PagerDuty, Opsgenie
- **Generic:** Custom webhooks

**Key Features:**
- Severity-based routing (critical → PagerDuty+Slack, warning → Slack only)
- Provider-based interface (similar to AI providers)
- Rich message formatting (Slack blocks, Teams adaptive cards, Markdown)
- Retry logic with exponential backoff
- Integration with diagnostics, remediation, and API events
- Environment variable configuration for all credentials

**CLI Integration:**
```bash
lumo diagnose host --notify slack              # Send results to Slack
lumo diagnose host --notify-on critical        # Only notify on critical issues
lumo fix host --interactive --notify teams     # Send approval requests to Teams
lumo notify test --provider slack              # Test notification config
```

**Configuration:**
```yaml
notifications:
  enabled: true
  routing:
    critical: [pagerduty, slack]
    error: [slack, email]
    warning: [slack]
  providers:
    slack:
      webhook_url: env:LUMO_SLACK_WEBHOOK_URL
      channel: "#lumo-alerts"
    teams:
      webhook_url: env:LUMO_TEAMS_WEBHOOK_URL
    telegram:
      bot_token: env:LUMO_TELEGRAM_BOT_TOKEN
      chat_id: env:LUMO_TELEGRAM_CHAT_ID
    pagerduty:
      integration_key: env:LUMO_PAGERDUTY_KEY
```

**Estimated Deliverables:**
- 10 provider implementations (~2,500 lines)
- Core notifier + routing (~500 lines)
- Message formatters (~800 lines)
- Tests (~2,000 lines)
- **Total: ~5,800 lines**

---

## Localhost Execution Details

**Automatic Detection:**
The `diagnose` command automatically detects localhost patterns and runs commands directly without SSH:

**Localhost Patterns:**
- `localhost`
- `127.0.0.1`
- `::1` (IPv6)
- `0.0.0.0`
- Empty hostname
- `localhost.localdomain`

**Implementation:**
```go
// diagnose.go
isLocal := isLocalhost(hostname)

if isLocal {
    executor = diagnostics.NewLocalExecutor()  // Direct execution
} else {
    executor = diagnostics.NewSSHExecutor(sshClient)  // SSH
}
```

**Usage:**
```bash
lumo diagnose localhost                    # No SSH overhead
lumo diagnose localhost --analyze          # Local with AI
lumo diagnose user@remote.server           # SSH connection
```

---

**End of Detailed Archive**

> This archive preserves the detailed documentation removed from CLAUDE.md on 2025-11-17 for token efficiency. The main CLAUDE.md now contains only essential current development information.
