# CLAUDE.md - AI Assistant Guide for Lumo

> **Last Updated:** 2025-11-16 (Phase 5 Complete - Security Diagnostics)
> **Project Version:** 0.4.2
> **Current Phase:** Phase 5 Complete (Security Diagnostics) + Phase 8 In Progress (Testing)

This document provides comprehensive guidance for AI assistants working on the Lumo codebase.

**For detailed examples and tutorials, see [DEVELOPMENT.md](DEVELOPMENT.md)**

---

## Project Overview

**Lumo** is an intelligent SRE/DevOps automation agent written in Go that:
- **Connects** to remote servers via SSH
- **Diagnoses** system issues (CPU, memory, disk, processes, services, network)
- **Analyzes** problems using AI (Anthropic, OpenAI, Ollama, Gemini)
- **Remediates** issues automatically with human-in-the-loop approval
- **Reports** findings in multiple formats (text, JSON, TOON)
- **Serves** as both a CLI tool and REST API server
- **Optimizes** AI token usage with TOON format (30-60% reduction)

**Key Characteristics:**
- **Language:** Go 1.25.4
- **Module Path:** `github.com/ignacio/lumo`
- **Architecture:** Modular CLI with pluggable backends
- **Current Status:** Phase 5 complete (SSH + Enhanced Diagnostics + AI with 4 providers + Security Checks + Advanced Memory Metrics)
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
│   ├── diagnostics/               # Diagnostic system (18 files, 5,900+ lines) ✅
│   │   ├── checkers/             # 10 checkers (6 core + 4 security)
│   │   │                         # Core: CPU, Memory, Disk, Process, Service, Network
│   │   │                         # Security: Patch Status, Open Ports, SSH Security, Auth Failures
│   │   └── formatters/           # Output formatters (text, JSON, TOON)
│   └── ai/                        # AI providers (7 files, 1,964 lines) ✅
│       ├── anthropic.go          # Claude integration
│       ├── openai.go             # GPT integration
│       ├── ollama.go             # Local model support
│       └── gemini.go             # Google Gemini integration
└── configs/
    └── config.example.yaml        # Configuration template

Total: 41 Go files (~12,000 lines) + 14 test files (~5,246 lines)
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

**AI Integration:** Custom HTTP clients for all providers (no external SDKs)

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

### Diagnostic Checkers (All 6 Complete)

| Checker | Key Metrics |
|---------|-------------|
| `cpu.go` | Load average, usage %, core count |
| `memory.go` | RAM/swap usage %, top memory consumers, cross-platform |
| `disk.go` | Space usage %, inode usage %, multi-filesystem |
| `process.go` | Process count, zombies, top consumers |
| `service.go` | Failed services, systemd/init/launchd support |
| `network.go` | Interfaces (Linux `ip`, macOS/BSD `ifconfig` with proper UP/status parsing), connectivity (ICMP/TCP), configurable targets, latency, error classification, active connection counting |

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

### ⏳ Phase 6: Auto-Remediation (Planned)
- Remediation action registry
- Risk classification (safe, moderate, critical)
- Human-in-the-loop approval
- Rollback support, audit logging

### ⏳ Phase 7: Reporting (Planned)
- Multiple formats (Markdown, JSON, YAML, HTML)
- Historical data, trend analysis

### ✅ Phase 8: Testing & Documentation (Substantially Complete)
**Status:** Comprehensive test coverage achieved across all core packages

**Overall Test Coverage: 37.1%** (5,917 lines of test code)

#### Package Coverage Status:

| Package | Coverage | Test Files | Status |
|---------|----------|------------|--------|
| **internal/diagnostics/formatters** | 100.0% | text_test.go | ✅ Complete |
| **internal/config** | 68.8% | config_test.go, config_api_key_test.go | ✅ Good |
| **internal/diagnostics/checkers** | 61.2% | cpu, disk, memory, network, process, service tests | ✅ Good |
| **internal/diagnostics** | 54.1% | result_test.go, severity_test.go | ✅ Solid |
| **internal/ai** | 27.0% | prompts_test.go, provider_test.go | ⚠️ Needs expansion |
| **internal/ssh** | 16.3% | config_test.go, errors_test.go, types_test.go | ⚠️ Needs expansion |
| **cmd/lumo** | 0.0% | (none) | ❌ Not started |

#### Test Files Added (15 files, 5,917 total lines):

**SSH Package Tests (1,230 lines):**
- `internal/ssh/config_test.go` - ClientConfig validation, key handling
- `internal/ssh/errors_test.go` - All error types, helper functions
- `internal/ssh/types_test.go` - Enums, CommandResult, constants

**Diagnostics Tests (4,207 lines):**
- `internal/diagnostics/checkers/cpu_test.go` - CPU checker with mock executor
- `internal/diagnostics/checkers/disk_test.go` - Disk checker tests
- `internal/diagnostics/checkers/memory_test.go` - Memory checker tests with security validations (48 test cases covering command injection prevention, input validation, sanitization)
- `internal/diagnostics/checkers/network_test.go` - Network checker + Run() tests
- `internal/diagnostics/checkers/process_test.go` - Process checker tests
- `internal/diagnostics/checkers/service_test.go` - Service checker + Run() tests
- `internal/diagnostics/formatters/text_test.go` - Text formatter (100% coverage)
- `internal/diagnostics/result_test.go` - CheckResult and Report types
- `internal/diagnostics/severity_test.go` - Severity and threshold types

**AI Package Tests (767 lines):**
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

#### Remaining Work for Higher Coverage:

**High Priority:**
1. **cmd/lumo (0%)** - CLI command tests
2. **AI providers (27%)** - HTTP client tests for Anthropic, OpenAI, Ollama, Gemini
3. **SSH package (16.3%)** - Auth flow tests, retry logic tests

**Medium Priority:**
4. **Checkers (61.2%)** - Additional edge case coverage for parsing functions
5. **Diagnostics core (54.1%)** - Runner orchestration and parallel execution tests

**Future:**
6. Integration tests with real SSH connections
7. E2E tests for full diagnostic flows
8. GoDoc comments for exported functions

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
