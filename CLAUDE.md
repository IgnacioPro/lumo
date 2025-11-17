# CLAUDE.md - AI Assistant Guide for Lumo

> **Last Updated:** 2025-11-16 (Phase 5 Complete - Enhanced Diagnostics including Kubernetes)
> **Project Version:** 0.5.0
> **Current Phase:** Phase 5 Complete (Security + Kubernetes Diagnostics) + Phase 8 In Progress (Testing)

This document provides comprehensive guidance for AI assistants working on the Lumo codebase.

**For detailed examples and tutorials, see [DEVELOPMENT.md](DEVELOPMENT.md)**

---

## Project Overview

**Lumo** is an intelligent SRE/DevOps automation agent written in Go that:
- **Connects** to remote servers via SSH
- **Diagnoses** system issues (CPU, memory, disk, processes, services, network, security, Kubernetes)
- **Analyzes** problems using AI (Anthropic, OpenAI, Ollama, Gemini)
- **Remediates** issues automatically with human-in-the-loop approval
- **Reports** findings in multiple formats (text, JSON, TOON)
- **Serves** as both a CLI tool and REST API server
- **Optimizes** AI token usage with TOON format (30-60% reduction)

**Key Characteristics:**
- **Language:** Go 1.25.4
- **Module Path:** `github.com/ignacio/lumo`
- **Architecture:** Modular CLI with pluggable backends
- **Current Status:** Phase 5 complete (SSH + 11 Diagnostic Checkers + AI with 4 providers + Security Checks + Kubernetes Native Diagnostics)
- **License:** MIT

---

## Codebase Structure

```
lumo/
├── cmd/lumo/                      # CLI commands (main package)
│   ├── main.go                   # Entry point
│   ├── root.go                   # Root command, global flags
│   ├── connect.go                # SSH connection ✅
│   ├── diagnose.go               # Diagnostics (localhost + remote) ✅
│   ├── fix.go                    # Auto-remediation (planned)
│   ├── report.go                 # Report generation (planned)
│   └── serve.go                  # API server (planned)
├── internal/
│   ├── config/                    # Configuration management
│   ├── ssh/                       # SSH client (8 files, 2,410 lines) ✅
│   ├── diagnostics/               # Diagnostic system (20+ files, 7,300+ lines) ✅
│   │   ├── checkers/             # 11 checkers (6 core + 4 security + 1 Kubernetes)
│   │   │                         # Core: CPU, Memory, Disk, Process, Service, Network
│   │   │                         # Security: Patch Status, Open Ports, SSH Security, Auth Failures
│   │   │                         # Kubernetes: Cluster diagnostics (nodes, pods, deployments, etc.)
│   │   └── formatters/           # Output formatters (text, JSON, TOON)
│   └── ai/                        # AI providers (7 files, 1,964 lines) ✅
│       ├── anthropic.go          # Claude integration
│       ├── openai.go             # GPT integration
│       ├── ollama.go             # Local model support
│       └── gemini.go             # Google Gemini integration
└── configs/
    └── config.example.yaml        # Configuration template

Total: 43+ Go files (~14,400+ lines) + 25 test files (~11,059 lines)
```

**Directory Purposes:**

| Directory | Purpose |
|-----------|---------|
| `cmd/lumo/` | CLI command definitions (public) |
| `internal/` | Private packages, not importable externally |
| `internal/config/` | Config loading, validation, defaults |
| `internal/ssh/` | SSH client with auth, health monitoring |
| `internal/diagnostics/` | Core diagnostic runner and interfaces |
| `internal/diagnostics/checkers/` | Individual check implementations |
| `internal/ai/` | AI provider integrations |

---

## Technology Stack

### Core Dependencies

| Library | Version | Purpose |
|---------|---------|---------|
| **spf13/cobra** | v1.10.1 | CLI framework |
| **spf13/viper** | v1.21.0 | Configuration (YAML + env vars) |
| **sirupsen/logrus** | v1.9.3 | Structured logging |
| **golang.org/x/crypto/ssh** | - | SSH client |
| **cenkalti/backoff/v4** | - | Retry logic |
| **k8s.io/client-go** | v0.31.3 | Kubernetes native client |
| **k8s.io/api** | v0.31.3 | Kubernetes API types |
| **k8s.io/apimachinery** | v0.31.3 | Kubernetes API machinery |

**AI Integration:** Custom HTTP clients for all providers (no external SDKs)
**Kubernetes:** Native client using official Kubernetes Go libraries (no kubectl dependency)

---

## Configuration System

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

# AI API key - Generic fallback (works but requires changing when switching providers)
export LUMO_AI_API_KEY=sk-...              # Fallback if provider-specific not set

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
- `provider`: anthropic | openai | ollama | gemini
- `models`: Per-provider model map (defaults in code)
- `temperature`: 1.0 (standardized across all providers)
- `max_tokens`: 4096

**API Key Security:** NEVER in config file, ONLY via environment variables:
- **Recommended:** Provider-specific env vars (`LUMO_ANTHROPIC_API_KEY`, `LUMO_OPENAI_API_KEY`, etc.)
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

### Loading Configuration

```go
import "github.com/ignacio/lumo/internal/config"

cfg, err := config.Load()  // Searches hierarchy, validates automatically
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}
```

---

## Code Organization Principles

### Package Structure
- **`cmd/`**: Executable entry points, CLI logic
- **`internal/`**: Reusable business logic (private)
- **`pkg/`**: Public libraries (future, if needed)

### Import Organization
```go
import (
    // Standard library
    "fmt"
    "time"

    // Third-party
    "github.com/spf13/cobra"

    // Internal
    "github.com/ignacio/lumo/internal/config"
)
```

### Naming Conventions
| Type | Convention | Example |
|------|-----------|---------|
| Packages | Lowercase, no underscores | `config`, `diagnostics` |
| Files | Lowercase, underscores OK | `config.go`, `ssh_client.go` |
| Exported Types | PascalCase | `Config`, `Provider` |
| Private Types | camelCase | `sshClient`, `configCache` |
| Functions | PascalCase/camelCase | `LoadConfig()`, `validatePort()` |

---

## CLI Command Structure

### Global Flags (Available to all commands)
- `--config` - Config file path
- `--verbose` / `-v` - Debug logging
- `--dry-run` - Simulate without changes

### Subcommands

| Command | Status | Purpose | Key Flags |
|---------|--------|---------|-----------|
| `connect` | ✅ Phase 2 | SSH connection | `--port`, `--user`, `--key` |
| `diagnose` | ✅ Phase 3+4 | System diagnostics + AI | `--checks`, `--format`, `--analyze` |
| `fix` | ⏳ Phase 5 | Auto-remediation | `--auto-approve`, `--dry-run` |
| `report` | ⏳ Phase 6 | Report generation | `--format`, `--output` |
| `serve` | ⏳ Phase 7 | API server | `--port`, `--tls` |

### Command Registration Pattern

```go
var myCmd = &cobra.Command{
    Use:   "command [args]",
    Short: "Brief description",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        // Implementation
    },
}

func init() {
    rootCmd.AddCommand(myCmd)
    myCmd.Flags().StringP("flag", "f", "default", "help")
}
```

---

## Error Handling Patterns

**Always wrap errors with context:**
```go
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}
```

**Validation errors:**
```go
if port < 1 || port > 65535 {
    return fmt.Errorf("invalid port: %d (must be 1-65535)", port)
}
```

**Command-level error handling:**
```go
Run: func(cmd *cobra.Command, args []string) {
    if err := execute(); err != nil {
        log.Errorf("Operation failed: %v", err)
        os.Exit(1)
    }
}
```

---

## Logging Guidelines

**Log Levels:**
```go
log.Debug("Detailed info")     // --verbose only
log.Info("Normal operations")  // Default
log.Warn("Warnings")           // Potential issues
log.Error("Errors")            // Need attention
log.Fatal("Fatal")             // Exit program
```

**Structured Logging:**
```go
log.WithFields(logrus.Fields{
    "host": "example.com",
    "port": 22,
}).Info("Connecting to SSH server")
```

**Dry-Run:**
```go
if dryRun {
    log.Info("DRY RUN: Would execute command")
    return nil
}
```

---

## Testing Strategy

**Test File Organization:**
```
internal/config/
├── config.go
└── config_test.go
```

**Table-Driven Tests (Preferred):**
```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  *Config
        wantErr bool
    }{
        {"valid", &Config{SSH: SSHConfig{Port: 22}}, false},
        {"invalid port", &Config{SSH: SSHConfig{Port: 99999}}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

**Mock Executors for Checkers:**
```go
// Mock executor for testing checkers without real commands
type mockExecutor struct {
    responses map[string]mockResponse
}

func (m *mockExecutor) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
    for pattern, resp := range m.responses {
        if strings.Contains(command, pattern) {
            return resp.stdout, resp.stderr, resp.exitCode, resp.err
        }
    }
    return "", "command not mocked", 127, nil
}
```

**Running Tests:**
```bash
go test ./...                    # All tests (currently 37.1% coverage)
go test -cover ./...             # With coverage report
go test -v ./internal/config     # Specific package verbose
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out  # HTML coverage
```

**Current Coverage by Package:**
- internal/diagnostics/formatters: 100.0%
- internal/config: 68.8%
- internal/diagnostics/checkers: 61.2%
- internal/diagnostics: 54.1%
- internal/ai: 27.0%
- internal/ssh: 16.3%
- cmd/lumo: 0.0%

---

## Security Considerations

**Security Audit Status**: ✅ All CRITICAL issues resolved (2025-11-16)
- See `REPORTS/security-fixes-2025-11-16.md` for details
- See `REPORTS/security-audit-2025-11-15.md` for full audit

**Never Commit (.gitignore protects):**
- `config.yaml` - May contain secrets
- `*.pem`, `*.key` - Certificates/keys
- `id_rsa*` - SSH keys
- `.env*` - Environment files

**Credential Handling:**
- AI API keys: Use provider-specific env vars (`LUMO_ANTHROPIC_API_KEY`, `LUMO_OPENAI_API_KEY`, `LUMO_GEMINI_API_KEY`)
- Fallback: Generic `LUMO_AI_API_KEY` (not recommended, requires changing when switching)
- SSH passwords: **NEVER use CLI flags** - use secure prompting or key-based auth only
- Password flag removed for security (prevents exposure in process lists/history)
- TLS: Config validation enforces cert+key requirement

**SSH Security (Enhanced 2025-11-16):**
- ✅ Host key verification **enabled by default** (StrictHostKeyChecking: true)
- ✅ Known_hosts auto-configured to `~/.ssh/known_hosts`
- ✅ Warnings displayed when disabling verification
- ✅ Command injection protection via WorkingDir sanitization
- ✅ Shell metacharacter filtering and path validation

**File Permissions:**
```bash
chmod 600 ~/.lumo/config.yaml
chmod 600 ~/.ssh/id_rsa
chmod 600 /etc/lumo/key.pem
```

---

## Important Files Reference

### Core Files

| File | Purpose | Key Points |
|------|---------|-----------|
| `cmd/lumo/root.go` | Root command, global setup | `Execute()`, `initConfig()`, global flags |
| `cmd/lumo/diagnose.go` | Diagnostics command | Localhost detection, SSH/local executor switching, AI integration |
| `internal/config/config.go` | Configuration system | `Load()`, `Validate()`, defaults |
| `internal/diagnostics/diagnostics.go` | Diagnostic orchestration | `Checker` interface, `Runner`, parallel execution |
| `internal/diagnostics/executor.go` | Command execution | `SSHExecutor`, `LocalExecutor` |

### Diagnostic Checkers (11 Total: 6 Core + 4 Security + 1 Kubernetes)

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

**Kubernetes Checker:**
| Checker | Key Metrics |
|---------|-------------|
| `kubernetes.go` | Nodes (ready/not-ready), Pods (phase, restarts), Deployments/StatefulSets/DaemonSets (replica health), Services (endpoints), PVCs (binding status), Events (recent warnings/errors) |

### AI Providers (All 4 Complete)

| Provider | Model | Key Features |
|----------|-------|-------------|
| `anthropic.go` | claude-sonnet-4-5-20250929 | Streaming, structured output |
| `openai.go` | gpt-4-turbo-preview | Function calling, streaming |
| `ollama.go` | llama3.1:8b | Self-hosted, no API key |
| `gemini.go` | gemini-2.0-flash-exp | SSE streaming, token tracking |

---

## Phase Roadmap

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

### ✅ Phase 5.1: Kubernetes Diagnostics (Complete - 2025-11-16)
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

### ⏳ Phase 6: Auto-Remediation (Planned)
- Remediation action registry
- Risk classification (safe, moderate, critical)
- Human-in-the-loop approval
- Rollback support, audit logging

### ⏳ Phase 6: Reporting (Planned)
- Multiple formats (Markdown, JSON, YAML, HTML)
- Historical data, trend analysis

### ⏳ Phase 7: API Server (Planned)
- REST API + WebSocket
- Authentication, rate limiting
- OpenAPI/Swagger docs

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

#### Test Files Added (25 files, 11,059 total lines):

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

**Diagnostics Tests (6,198 lines):**
- `internal/diagnostics/testing.go` - **MockChecker for integration testing** (65 lines)
- `internal/diagnostics/runner_test.go` - Runner orchestration, parallel/sequential execution (18 tests)
- `internal/diagnostics/executor_test.go` - LocalExecutor with context support (7 tests)
- `internal/diagnostics/checkers/cpu_test.go` - CPU checker with mock executor
- `internal/diagnostics/checkers/disk_test.go` - Disk checker tests
- `internal/diagnostics/checkers/memory_test.go` - Memory checker tests with security validations (52 test cases)
- `internal/diagnostics/checkers/network_test.go` - Network checker + Run() tests
- `internal/diagnostics/checkers/process_test.go` - Process checker tests
- `internal/diagnostics/checkers/service_test.go` - Service checker + Run() tests
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

## 🎯 Roadmap to 80% Coverage

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
- `Analyze()` methods for all 4 providers (Anthropic, OpenAI, Ollama, Gemini)
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

### ⏳ Phase 9: API Server (Planned)
- REST API + WebSocket
- Authentication, rate limiting
- OpenAPI/Swagger docs

---

## Localhost Execution

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

## TOON Format Integration

**TOON (Token-Oriented Object Notation)** is a compact, LLM-optimized data format that Lumo uses to reduce AI token costs by 30-60% compared to JSON.

### What is TOON?

TOON combines YAML's readability with CSV's tabular efficiency:
- **Uniform arrays** use tabular notation: `array[count]{field1,field2,...}:`
- **Each row** is comma-separated values
- **Non-uniform data** uses YAML-like `key: value` format
- **Strings** are only quoted when necessary

### Token Efficiency

**Benchmark Results (from real Lumo diagnostics):**
- **33% reduction** on typical diagnostic reports
- **42% reduction** on process/service lists
- **58% reduction** on metrics arrays

**Cost Savings:**
- Anthropic Claude Sonnet: ~$0.30 → $0.20 per diagnostic analysis
- OpenAI GPT-4 Turbo: ~$0.50 → $0.34 per analysis
- Cumulative savings on 1000 analyses: ~$160-$300

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

### Usage

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

### Test Coverage

- **TOON formatter:** 98.1% coverage
- **8 test cases** covering all scenarios
- **Token efficiency test** validates 33% reduction

---

## AI Assistant Best Practices

### DO:
✅ Use existing patterns and code structure
✅ Wrap errors with context: `fmt.Errorf("context: %w", err)`
✅ Use appropriate log levels
✅ Write tests for new functionality
✅ Document exported functions with GoDoc comments
✅ Respect `dryRun` flag in destructive operations
✅ Update CLAUDE.md when completing phases

### DON'T:
❌ Commit secrets (API keys, passwords)
❌ Ignore errors
❌ Use excessive global state
❌ Skip validation
❌ Hardcode paths (use config/flags)
❌ Mix CLI logic with business logic

### Code Review Checklist:
- [ ] Go naming conventions followed?
- [ ] Errors wrapped with context?
- [ ] Logging appropriate?
- [ ] Tests added?
- [ ] Documentation updated?
- [ ] Dry-run mode handled?
- [ ] Config changes in `config.example.yaml`?
- [ ] Formatted with `go fmt`?
- [ ] Passes `go vet`?

---

## Quick Reference

### Key Commands
```bash
# Build
go build -o lumo ./cmd/lumo

# Install globally
go install ./cmd/lumo

# Run with options
./lumo --verbose diagnose localhost
./lumo diagnose user@host --checks cpu,memory --analyze

# With AI analysis (provider-specific API keys)
LUMO_ANTHROPIC_API_KEY=sk-ant-... ./lumo diagnose --analyze
LUMO_OPENAI_API_KEY=sk-... LUMO_AI_PROVIDER=openai ./lumo diagnose --analyze
LUMO_GEMINI_API_KEY=... LUMO_AI_PROVIDER=gemini ./lumo diagnose --analyze

# Output formats
./lumo diagnose localhost --format text        # Human-readable (default)
./lumo diagnose localhost --format json        # JSON output
./lumo diagnose localhost --format toon        # TOON format (30-60% token reduction)

# Development
go fmt ./...
go vet ./...
go test ./...
```

### Key Patterns
```go
// Error wrapping
return fmt.Errorf("context: %w", err)

// Logging
log.WithFields(logrus.Fields{"key": "val"}).Info("msg")

// Config loading
cfg, err := config.Load()

// Command registration
rootCmd.AddCommand(myCmd)

// TOON formatter
formatter := formatters.NewToonFormatter()
toonOutput := formatter.FormatReport(report)

// AI prompt builder with TOON
builder := ai.NewPromptBuilder()  // TOON enabled by default
prompt, _ := builder.BuildAnalysisPrompt(req)
```

---

**End of CLAUDE.md**

> For questions or improvements, open an issue at https://github.com/ignacio/lumo/issues
