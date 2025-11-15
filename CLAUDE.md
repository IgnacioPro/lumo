# CLAUDE.md - AI Assistant Guide for Lumo

> **Last Updated:** 2025-11-14
> **Project Version:** 0.4.0
> **Current Phase:** Phase 3 Complete (All Six Core Diagnostic Checkers Complete)

This document provides guidance for AI assistants working on the Lumo codebase.

---

## Project Overview

**Lumo** is an intelligent SRE/DevOps automation agent (Go 1.25.4) that connects to remote servers via SSH, diagnoses system issues, analyzes problems using AI, and remediates issues with human approval.

**Current Status:** Phase 3 Complete - SSH + All 6 Core Diagnostic Checkers (CPU, Memory, Disk, Process, Service, Network)

---

## Codebase Structure

```
lumo/
├── cmd/                           # Command-line interface entry points
│   └── lumo/                      # Main application (package main)
│       ├── main.go               # Entry point (5 lines)
│       ├── root.go               # Root command + global setup (89 lines)
│       ├── connect.go            # SSH connection command (173 lines) ✅
│       ├── diagnose.go           # System diagnostics command (190 lines) ✅
│       ├── fix.go                # Auto-remediation command (stub)
│       ├── report.go             # Report generation command (stub)
│       └── serve.go              # API server command (stub)
│
├── internal/                      # Private application packages
│   ├── config/                    # Configuration management
│   │   └── config.go             # Config types, loading, validation (152 lines)
│   │
│   ├── ssh/                       # SSH client implementation ✅ Phase 2
│   │   ├── types.go              # Core types and interfaces (177 lines)
│   │   ├── errors.go             # Custom error types (89 lines)
│   │   ├── config.go             # SSH client configuration (219 lines)
│   │   ├── retry.go              # Retry logic with backoff (145 lines)
│   │   ├── auth.go               # Authentication methods (346 lines)
│   │   ├── health.go             # Health monitoring (187 lines)
│   │   ├── client.go             # Main SSH client (368 lines)
│   │   └── session.go            # Session & command execution (879 lines)
│   │
│   └── diagnostics/               # Diagnostic system ✅ Phase 3.1, 3.2, & 3.3
│       ├── diagnostics.go        # Core runner & interfaces (353 lines)
│       ├── result.go             # Result types & reporting (320 lines)
│       ├── severity.go           # Severity & thresholds (354 lines)
│       ├── executor.go           # SSH command executor (51 lines)
│       ├── checkers/             # Diagnostic checkers
│       │   ├── cpu.go            # CPU diagnostics (200 lines) ✅
│       │   ├── memory.go         # Memory diagnostics (338 lines) ✅
│       │   ├── disk.go           # Disk diagnostics (366 lines) ✅
│       │   ├── process.go        # Process diagnostics (298 lines) ✅
│       │   ├── service.go        # Service diagnostics (344 lines) ✅
│       │   └── network.go        # Network diagnostics (474 lines) ✅
│       └── formatters/           # Output formatters
│           └── text.go           # Text formatter (265 lines) ✅
│
├── configs/                       # Configuration templates
│   └── config.example.yaml        # Example configuration file
│
├── .gitignore                     # Git ignore patterns (binaries, secrets, configs)
├── go.mod                         # Go module definition
├── go.sum                         # Dependency checksums
├── CLAUDE.md                      # AI assistant guide (this file)
└── README.md                      # User-facing documentation

Total: 30 Go files, ~8,500 lines of code
Phase 2 (SSH): 8 files, 2,410 lines
Phase 3 (Diagnostics): 12 files, 3,835 lines
```

---

## Technology Stack

**Core:** Cobra (CLI), Viper (config), Logrus (logging), golang.org/x/crypto/ssh, cenkalti/backoff

---

## Development Workflow

```bash
# Setup
go mod download
go build -o lumo ./cmd/lumo

# Config
mkdir -p ~/.lumo
cp configs/config.example.yaml ~/.lumo/config.yaml

# Development
go fmt ./...
go test ./...
```

---

## Code Conventions

- **Packages:** `cmd/` for CLI, `internal/` for private logic
- **Imports:** Standard lib, third-party, internal (separated by blank lines)
- **Naming:** PascalCase exports, camelCase private, lowercase packages
- **Errors:** Always wrap with context: `fmt.Errorf("context: %w", err)`

---

## Configuration

**Search paths:** `--config` flag → `./config.yaml` → `~/.lumo/config.yaml`

**Env vars:** `LUMO_<SECTION>_<KEY>` (e.g., `LUMO_SSH_PORT=2222`)

**Sections:** `ssh`, `ai`, `logging`, `api` (see `configs/config.example.yaml`)

---

## CLI Commands

**Global flags:** `--config`, `--verbose`, `--dry-run`

**Commands:**
- `connect` ✅ - SSH connection (Phase 2)
- `diagnose` ✅ - System health checks (Phase 3)
- `fix` ⏳ - Auto-remediation (Phase 5)
- `report` ⏳ - Report generation (Phase 6)
- `serve` ⏳ - API server (Phase 7)

---



## Security

- Never commit secrets (see `.gitignore`)
- Use env vars for credentials: `LUMO_AI_API_KEY`, etc.
- Validate all user input
- Set proper file permissions (600 for keys/configs)

---

## Key Files

- `cmd/lumo/root.go` - Root command, global flags, config loading
- `internal/config/config.go` - Config types, validation
- `internal/ssh/` - SSH client (8 files, 2,410 lines) - Phase 2 ✅
- `internal/diagnostics/` - Diagnostic system (12 files, 3,835 lines) - Phase 3 ✅
  - `diagnostics.go`, `result.go`, `severity.go` - Core system
  - `checkers/` - CPU, Memory, Disk, Process, Service, Network
  - `formatters/text.go` - Text output formatter
- `cmd/lumo/diagnose.go` - Diagnose command with SSH integration

---

## Phase Roadmap

### Phase 1: Foundation & Project Setup ✅ COMPLETE

**Status:** Done
**Files:** `root.go`, `config.go`, all command stubs
**Features:**
- Cobra CLI framework integrated
- Viper configuration system with YAML + env vars
- Logrus structured logging
- All 5 subcommands registered (stubs)
- Global flags implemented

### Phase 2: SSH & Connection Management ✅ COMPLETE

**Status:** Done (2025-11-14)
**Target Package:** `internal/ssh/`
**Dependencies:** `golang.org/x/crypto/ssh`, `github.com/cenkalti/backoff/v4`
**Features Implemented:**
- ✅ All 4 authentication methods (SSH agent, key files, password, keyboard-interactive)
- ✅ Connection retry logic with exponential backoff
- ✅ Command execution with timeout and output capture
- ✅ Health monitoring with keep-alive
- ✅ Auto-reconnection on connection loss
- ✅ Full integration in `connect.go` command
- ✅ Comprehensive error handling with custom error types
**Total:** 8 new files, 2,410 lines of code

### Phase 3: Diagnostic System ✅ COMPLETE

**Status:** Phase 3 Complete (2025-11-14) - Ready for Phase 4
**Deliverables:** 12 files, 3,835 lines

**Core System:**
- ✅ `Checker` interface, `DiagnosticRunner`, severity classification
- ✅ Configurable thresholds, parallel/sequential execution
- ✅ Text & JSON output formatters

**All 6 Core Checkers:**
1. ✅ CPU - Load average, usage, core count
2. ✅ Memory - RAM, swap utilization (Linux/macOS)
3. ✅ Disk - Space, inodes, multi-filesystem support
4. ✅ Process - Count, zombies, top CPU/memory consumers
5. ✅ Service - Status monitoring (systemd/init/launchd)
6. ✅ Network - Interfaces, connectivity, DNS, statistics

**Integration:**
- ✅ Full SSH integration in `diagnose.go` command
- ✅ Cross-platform support (Linux, macOS, BSD)

### Phase 4: AI Integration ⏳

**Package:** `internal/ai/`
**Tasks:** Provider abstraction, Anthropic/OpenAI/local model integration, prompt engineering

### Phase 5: Auto-Remediation ⏳

**Package:** `internal/remediation/`
**Tasks:** Action registry, risk classification, human-in-loop approval, rollback, update `fix.go`

### Phase 6: Reporting ⏳

**Package:** `internal/reporting/`
**Tasks:** Multi-format reports (Markdown/JSON/YAML/HTML), historical data, update `report.go`

### Phase 7: API Server ⏳

**Package:** `internal/server/`
**Tasks:** REST API, WebSocket, auth, rate limiting, OpenAPI docs, update `serve.go`

### Phase 8: Testing & Docs ⏳

**Tasks:** Unit/integration/e2e tests (80% coverage), benchmarking, GoDoc, user guide

---

## Best Practices

**DO:**
- Follow Go naming conventions
- Wrap errors: `fmt.Errorf("context: %w", err)`
- Use structured logging: `log.WithFields()`
- Validate all inputs
- Handle dry-run mode
- Add tests for new features

**DON'T:**
- Commit secrets
- Ignore errors
- Use global state unnecessarily
- Hardcode paths
- Mix CLI and business logic

---

**End of CLAUDE.md**

> For questions or improvements to this guide, please open an issue at https://github.com/ignacio/lumo/issues
