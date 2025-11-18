# CLAUDE.md - AI Assistant Guide for Lumo

> **Last Updated:** 2025-11-17 | **Version:** 0.5.0
> **Status:** Phases 1-6 Complete | Testing In Progress

**For detailed examples and tutorials, see [DEVELOPMENT.md](DEVELOPMENT.md)**

---

## Project Overview

**Lumo** - Intelligent SRE/DevOps automation agent in Go:
- SSH connectivity + local execution
- System diagnostics (12 checkers: core, security, specialized)
- AI analysis (5 providers: Anthropic, OpenAI, Ollama, Gemini, OpenRouter)
- Auto-remediation with human-in-the-loop approval
- Multiple output formats (text, JSON, TOON)
- TOON format for 30-60% AI token reduction

**Tech:** Go 1.25.4 | Module: `github.com/ignacio/lumo` | License: MIT

---

## Codebase Structure

```
lumo/
├── cmd/lumo/              # CLI: main, root, connect, diagnose, fix
├── internal/
│   ├── config/            # Configuration management
│   ├── ssh/               # SSH client (auth, health, retry)
│   ├── diagnostics/       # Runner + 12 checkers + formatters
│   │   ├── checkers/      # CPU, Memory, Disk, Process, Service, Network
│   │   │                  # Patch, Ports, SSH Security, Auth Failures
│   │   │                  # Kubernetes, Proxmox
│   │   └── formatters/    # text, JSON, TOON
│   ├── ai/                # 5 providers: Anthropic, OpenAI, Ollama, Gemini, OpenRouter
│   ├── remediation/       # Actions, executor, approval, audit
│   └── notifications/     # (planned) Slack, Teams, etc.
└── configs/config.example.yaml

Total: 54 Go files + 35 test files | Test Coverage: 50.4%
```

---

## Technology Stack

**Core:** cobra (CLI), viper (config), logrus (logging), x/crypto/ssh, backoff (retry)
**Kubernetes:** k8s.io/client-go v0.31.3 (native client, no kubectl)
**AI:** Custom HTTP clients for all 5 providers (no external SDKs)

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

**Config Sections:** SSH, AI, Logging, API, Diagnostics
**Loading:** `cfg, err := config.Load()` - searches hierarchy, validates automatically

**Security:** API keys ONLY via env vars, NEVER in config files

---

## Code Organization

**Packages:** `cmd/` (CLI), `internal/` (private logic), `pkg/` (future public libs)
**Imports:** Standard → Third-party → Internal
**Naming:** packages (lowercase), exported (PascalCase), private (camelCase)

---

## CLI Commands

**Global Flags:** `--config`, `--verbose`/`-v`, `--dry-run`

| Command | Status | Purpose |
|---------|--------|---------|
| `connect` | ✅ | SSH connection |
| `diagnose` | ✅ | System diagnostics + AI analysis |
| `fix` | ✅ | Auto-remediation with approval |
| `report` | ⏳ | Report generation (planned) |
| `serve` | ⏳ | API server (planned) |
| `notify` | ⏳ | Notifications (planned) |

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

### ⏳ Planned Phases

**Phase 7:** Reporting - Multiple formats (Markdown, JSON, YAML, HTML), historical data, trend analysis

**Phase 8:** Testing - Target 70%+ coverage (currently 50.4%). Priorities: internal/ai HTTP tests, internal/ssh client tests, checker edge cases

**Phase 9:** API Server - REST API + WebSocket, authentication, rate limiting, OpenAPI/Swagger docs

**Phase 10:** Messaging & Notifications - Multi-provider support (Slack, Teams, PagerDuty, etc.), severity-based routing, rich formatting

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

---

**For questions/improvements:** https://github.com/ignacio/lumo/issues
