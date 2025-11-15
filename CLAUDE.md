# CLAUDE.md - AI Assistant Guide for Lumo

> **Last Updated:** 2025-11-15
> **Project Version:** 0.4.0
> **Current Phase:** Phase 4 Complete (AI Integration with 4 Providers)

This document provides comprehensive guidance for AI assistants working on the Lumo codebase.

**For detailed examples and tutorials, see [DEVELOPMENT.md](DEVELOPMENT.md)**

---

## Project Overview

**Lumo** is an intelligent SRE/DevOps automation agent written in Go that:
- **Connects** to remote servers via SSH
- **Diagnoses** system issues (CPU, memory, disk, processes, services, network)
- **Analyzes** problems using AI (Anthropic, OpenAI, Ollama, Gemini)
- **Remediates** issues automatically with human-in-the-loop approval
- **Reports** findings in multiple formats (text, JSON)
- **Serves** as both a CLI tool and REST API server

**Key Characteristics:**
- **Language:** Go 1.25.4
- **Module Path:** `github.com/ignacio/lumo`
- **Architecture:** Modular CLI with pluggable backends
- **Current Status:** Phase 4 complete (SSH + Diagnostics + AI with 4 providers + Local execution)
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
│   ├── diagnostics/               # Diagnostic system (12 files, 3,835 lines) ✅
│   │   ├── checkers/             # 6 core checkers (CPU, Memory, Disk, Process, Service, Network)
│   │   └── formatters/           # Output formatters (text, JSON)
│   └── ai/                        # AI providers (7 files, 1,964 lines) ✅
│       ├── anthropic.go          # Claude integration
│       ├── openai.go             # GPT integration
│       ├── ollama.go             # Local model support
│       └── gemini.go             # Google Gemini integration
└── configs/
    └── config.example.yaml        # Configuration template

Total: 37 Go files, ~10,600 lines
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

**Running Tests:**
```bash
go test ./...                    # All tests
go test -cover ./...             # With coverage
go test -v ./internal/config     # Specific package
```

---

## Security Considerations

**Never Commit (.gitignore protects):**
- `config.yaml` - May contain secrets
- `*.pem`, `*.key` - Certificates/keys
- `id_rsa*` - SSH keys
- `.env*` - Environment files

**Credential Handling:**
- AI API keys: Use provider-specific env vars (`LUMO_ANTHROPIC_API_KEY`, `LUMO_OPENAI_API_KEY`, `LUMO_GEMINI_API_KEY`)
- Fallback: Generic `LUMO_AI_API_KEY` (not recommended, requires changing when switching)
- SSH passwords: Avoid, use key-based auth
- TLS: Config validation enforces cert+key requirement

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
| `memory.go` | RAM/swap usage %, cross-platform |
| `disk.go` | Space usage %, inode usage %, multi-filesystem |
| `process.go` | Process count, zombies, top consumers |
| `service.go` | Failed services, systemd/init/launchd support |
| `network.go` | Interfaces (Linux `ip`, macOS/BSD `ifconfig`), connectivity (ICMP/TCP), configurable targets, latency, error classification, status tracking |

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

### ⏳ Phase 5: Auto-Remediation (Planned)
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

### ⏳ Phase 8: Testing & Documentation (Planned)
- 80% test coverage target
- Integration + E2E tests
- GoDoc comments, user guides

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
```

---

**End of CLAUDE.md**

> For questions or improvements, open an issue at https://github.com/ignacio/lumo/issues
