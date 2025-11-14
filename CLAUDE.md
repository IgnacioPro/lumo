# CLAUDE.md - AI Assistant Guide for Lumo

> **Last Updated:** 2025-11-14
> **Project Version:** 0.2.0
> **Current Phase:** Phase 3.1 (Diagnostic Foundation In Progress)

This document provides comprehensive guidance for AI assistants (like Claude) working on the Lumo codebase. It covers architecture, conventions, workflows, and best practices to ensure consistent, high-quality contributions.

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Codebase Structure](#codebase-structure)
3. [Technology Stack](#technology-stack)
4. [Development Workflow](#development-workflow)
5. [Code Organization Principles](#code-organization-principles)
6. [Configuration System](#configuration-system)
7. [CLI Command Structure](#cli-command-structure)
8. [Adding New Features](#adding-new-features)
9. [Error Handling Patterns](#error-handling-patterns)
10. [Logging Guidelines](#logging-guidelines)
11. [Testing Strategy](#testing-strategy)
12. [Common Tasks & Examples](#common-tasks--examples)
13. [Security Considerations](#security-considerations)
14. [Important Files Reference](#important-files-reference)
15. [Phase Roadmap](#phase-roadmap)
16. [Troubleshooting](#troubleshooting)

---

## Project Overview

**Lumo** is an intelligent SRE/DevOps automation agent written in Go that:

- **Connects** to remote servers via SSH
- **Diagnoses** system issues (CPU, memory, disk, processes, logs, network)
- **Analyzes** problems using AI (Anthropic Claude, OpenAI GPT, or local models)
- **Remediates** issues automatically with human-in-the-loop approval
- **Reports** findings in multiple formats (Markdown, JSON, YAML)
- **Serves** as both a CLI tool and REST API server

### Key Characteristics

- **Language:** Go 1.25.4
- **Module Path:** `github.com/ignacio/lumo`
- **Architecture:** Modular CLI with pluggable backends
- **Current Status:** Phase 2 complete (SSH), Phase 3.1 in progress (Diagnostic foundation)
- **License:** MIT

---

## Codebase Structure

```
lumo/
├── cmd/                           # Command-line interface entry points
│   └── lumo/                      # Main application (package main)
│       ├── main.go               # Entry point (5 lines)
│       ├── root.go               # Root command + global setup (89 lines)
│       ├── connect.go            # SSH connection command (stub)
│       ├── diagnose.go           # System diagnostics command (stub)
│       ├── fix.go                # Auto-remediation command (stub)
│       ├── report.go             # Report generation command (stub)
│       └── serve.go              # API server command (stub)
│
├── internal/                      # Private application packages
│   └── config/                    # Configuration management
│       └── config.go             # Config types, loading, validation (152 lines)
│
├── configs/                       # Configuration templates
│   └── config.example.yaml        # Example configuration file
│
├── .gitignore                     # Git ignore patterns (binaries, secrets, configs)
├── go.mod                         # Go module definition
├── go.sum                         # Dependency checksums
└── README.md                      # User-facing documentation

Total: 8 Go files, ~430 lines of code
```

### Directory Purposes

| Directory | Purpose | Visibility |
|-----------|---------|-----------|
| `cmd/lumo/` | CLI command definitions, flags, user-facing logic | Public (compiled to binary) |
| `internal/` | Reusable business logic, NOT importable by external projects | Private |
| `internal/config/` | Configuration loading, validation, defaults | Private |
| `configs/` | Example/template configuration files | Public (documentation) |

---

## Technology Stack

### Core Dependencies

| Library | Version | Purpose | Documentation |
|---------|---------|---------|---------------|
| **spf13/cobra** | v1.10.1 | CLI framework - commands, subcommands, flags | [cobra.dev](https://cobra.dev) |
| **spf13/viper** | v1.21.0 | Configuration management - YAML, env vars, defaults | [github.com/spf13/viper](https://github.com/spf13/viper) |
| **sirupsen/logrus** | v1.9.3 | Structured logging with levels | [github.com/sirupsen/logrus](https://github.com/sirupsen/logrus) |

### Supporting Dependencies

- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/spf13/afero` - Filesystem abstraction (used by Viper)
- `github.com/spf13/pflag` - POSIX-style flags (used by Cobra)
- `github.com/fsnotify/fsnotify` - Config file watching (used by Viper)

### Planned Dependencies (Future Phases)

- **Phase 2 (SSH):** `golang.org/x/crypto/ssh`
- **Phase 4 (AI):** Anthropic/OpenAI SDKs or custom HTTP clients
- **Phase 7 (API):** `gin-gonic/gin` or `labstack/echo` for REST API

---

## Development Workflow

### Initial Setup

```bash
# Clone the repository
git clone https://github.com/ignacio/lumo.git
cd lumo

# Verify Go version (requires 1.25.4+)
go version

# Download dependencies
go mod download

# Build the binary
go build -o lumo ./cmd/lumo

# Install globally (optional)
go install ./cmd/lumo
```

### Running Locally

```bash
# Run with default config search paths
./lumo diagnose

# Run with explicit config
./lumo --config ./configs/config.example.yaml diagnose

# Run with environment variable overrides
LUMO_SSH_PORT=2222 LUMO_LOGGING_LEVEL=debug ./lumo connect user@host

# Enable verbose logging
./lumo --verbose diagnose

# Dry-run mode (simulate without changes)
./lumo fix --dry-run
```

### Configuration Setup

```bash
# Create config directory
mkdir -p ~/.lumo

# Copy example config
cp configs/config.example.yaml ~/.lumo/config.yaml

# Edit configuration
nano ~/.lumo/config.yaml
```

### Build & Test Workflow

```bash
# Format code
go fmt ./...

# Lint (requires golangci-lint)
golangci-lint run

# Run tests (when implemented in Phase 8)
go test ./...

# Run tests with coverage
go test -cover ./...

# Build for production
go build -ldflags="-s -w" -o lumo ./cmd/lumo
```

---

## Code Organization Principles

### 1. Package Structure

**Follow Go's package organization best practices:**

- **`cmd/`**: Executable entry points (main packages)
  - Keep CLI-specific logic here (flag parsing, user output)
  - Delegate business logic to `internal/` packages

- **`internal/`**: Private packages (cannot be imported externally)
  - Place all reusable business logic here
  - Examples: `internal/ssh/`, `internal/diagnostics/`, `internal/ai/`

- **`pkg/`** (future): Public libraries if needed
  - Only if you want external projects to import your code

### 2. Import Organization

Organize imports in three groups (separated by blank lines):

```go
import (
    // Standard library
    "fmt"
    "os"
    "time"

    // Third-party packages
    "github.com/sirupsen/logrus"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"

    // Internal packages
    "github.com/ignacio/lumo/internal/config"
)
```

### 3. Naming Conventions

| Type | Convention | Example |
|------|-----------|---------|
| **Packages** | Lowercase, single word, no underscores | `config`, `ssh`, `diagnostics` |
| **Files** | Lowercase, underscores for separation | `config.go`, `ssh_client.go` |
| **Types** | PascalCase (exported), camelCase (private) | `Config`, `sshClient` |
| **Functions** | PascalCase (exported), camelCase (private) | `LoadConfig()`, `validatePort()` |
| **Variables** | camelCase | `cfgFile`, `maxRetries` |
| **Constants** | PascalCase or ALL_CAPS | `DefaultTimeout`, `MAX_RETRIES` |

### 4. File Responsibility

**One primary purpose per file:**

- **Commands**: One file per subcommand (`connect.go`, `diagnose.go`)
- **Types**: Group related types together (`config.go` has all config structs)
- **Interfaces**: Define in the package that uses them (consumer-driven)

---

## Configuration System

### Configuration Hierarchy

Lumo searches for configuration in this order (first found wins):

1. `--config` flag path (explicit override)
2. `./config.yaml` (current directory)
3. `~/.lumo/config.yaml` (home directory)

### Environment Variable Overrides

**Pattern:** `LUMO_<SECTION>_<KEY>`

```bash
# Override SSH port
export LUMO_SSH_PORT=2222

# Override AI provider
export LUMO_AI_PROVIDER=openai

# Override logging level
export LUMO_LOGGING_LEVEL=debug

# Override API TLS settings
export LUMO_API_TLS=true
export LUMO_API_CERT_FILE=/path/to/cert.pem
```

### Configuration Schema

**File:** `internal/config/config.go`

```go
type Config struct {
    SSH     SSHConfig     `mapstructure:"ssh"`
    AI      AIConfig      `mapstructure:"ai"`
    Logging LoggingConfig `mapstructure:"logging"`
    API     APIConfig     `mapstructure:"api"`
}
```

#### SSH Configuration

```yaml
ssh:
  timeout: 30s              # Connection timeout (time.Duration)
  port: 22                  # Default SSH port (1-65535)
  keepalive: 30s            # Keep-alive interval
  max_retries: 3            # Retry attempts
  retry_interval: 5s        # Delay between retries
```

#### AI Configuration

```yaml
ai:
  provider: anthropic       # Provider: anthropic|openai|local
  models:
    anthropic: claude-sonnet-4-5-20250929
    openai: gpt-4
  timeout: 60s              # API call timeout
  max_retries: 3            # Retry attempts
```

#### Logging Configuration

```yaml
logging:
  level: info              # Level: debug|info|warn|error
  format: text             # Format: text|json
  output: stdout           # Output: stdout|stderr|/path/to/file
```

#### API Configuration

```yaml
api:
  port: 8080               # REST API port (1-65535)
  host: 0.0.0.0            # Bind address
  tls: false               # Enable HTTPS
  cert_file: ""            # TLS certificate path (required if tls=true)
  key_file: ""             # TLS key path (required if tls=true)
  read_timeout: 15s        # Request read timeout
  write_timeout: 15s       # Response write timeout
  max_connections: 100     # Connection pool limit
```

### Loading Configuration

```go
import "github.com/ignacio/lumo/internal/config"

// Load with defaults
cfg := config.DefaultConfig()

// Load from file (searches hierarchy)
cfg, err := config.Load()
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}

// Validation is automatic in Load()
```

### Validation Rules

**Implemented in `config.Validate()`:**

- SSH port: 1-65535
- SSH timeout: must be positive
- AI provider: must be `anthropic`, `openai`, or `local`
- Logging level: must be `debug`, `info`, `warn`, or `error`
- Logging format: must be `text` or `json`
- API port: 1-65535
- TLS validation: if `tls=true`, both `cert_file` AND `key_file` must be provided

---

## CLI Command Structure

### Root Command

**File:** `cmd/lumo/root.go`

```go
var rootCmd = &cobra.Command{
    Use:   "lumo",
    Short: "Intelligent SRE/DevOps Agent",
    Long:  "Lumo automates system diagnostics and remediation...",
    Version: "0.1.0",
}
```

### Global Flags

Available to all subcommands via `PersistentFlags`:

```go
rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "simulate without changes")
```

### Subcommands

| Command | File | Status | Purpose | Key Flags |
|---------|------|--------|---------|-----------|
| `connect` | `connect.go` | Phase 2 stub | SSH connection | `--port`, `--user`, `--key` |
| `diagnose` | `diagnose.go` | Phase 3 stub | System health checks | `--checks`, `--all` |
| `fix` | `fix.go` | Phase 5 stub | Auto-remediation | `--auto-approve`, `--skip` |
| `report` | `report.go` | Phase 6 stub | Report generation | `--format`, `--output`, `--summary` |
| `serve` | `serve.go` | Phase 7 stub | API server | `--port`, `--host`, `--tls` |

### Command Registration Pattern

**Each command file follows this structure:**

```go
package main

import (
    "github.com/sirupsen/logrus"
    "github.com/spf13/cobra"
)

var cmdCmd = &cobra.Command{
    Use:   "command [args]",
    Short: "Brief description",
    Long:  "Detailed description...",
    Args:  cobra.MinimumNArgs(1),  // Argument validation
    Run: func(cmd *cobra.Command, args []string) {
        log.Info("Command executed")
        // Implementation here
    },
}

func init() {
    // Register with root command
    rootCmd.AddCommand(cmdCmd)

    // Define command-specific flags
    cmdCmd.Flags().StringP("flag", "f", "default", "help text")
    cmdCmd.Flags().BoolP("option", "o", false, "help text")
}
```

### PersistentPreRun Hook

**Runs before every command:**

```go
rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
    // Set log level based on --verbose flag
    if verbose {
        log.SetLevel(logrus.DebugLevel)
    }

    // Warn in dry-run mode
    if dryRun {
        log.Warn("DRY RUN MODE: No changes will be made")
    }
}
```

---

## Adding New Features

### Adding a New CLI Command

**Example: Adding `lumo backup` command**

1. **Create command file:** `cmd/lumo/backup.go`

```go
package main

import (
    "github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
    Use:   "backup [destination]",
    Short: "Backup system configuration",
    Long:  "Creates a backup of system configuration to the specified destination",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        destination := args[0]
        log.Infof("Backing up to: %s", destination)

        // Get flags
        compress, _ := cmd.Flags().GetBool("compress")
        exclude, _ := cmd.Flags().GetStringSlice("exclude")

        // Implementation
        if dryRun {
            log.Info("Would create backup (dry run)")
            return
        }

        // Actual backup logic here
    },
}

func init() {
    rootCmd.AddCommand(backupCmd)

    backupCmd.Flags().BoolP("compress", "c", true, "compress backup")
    backupCmd.Flags().StringSliceP("exclude", "e", []string{}, "exclude patterns")
}
```

2. **Build and test:**

```bash
go build -o lumo ./cmd/lumo
./lumo backup /tmp/backup --compress --exclude logs,tmp
```

### Adding a New Configuration Section

**Example: Adding database configuration**

1. **Define struct in `internal/config/config.go`:**

```go
type DatabaseConfig struct {
    Host     string `mapstructure:"host"`
    Port     int    `mapstructure:"port"`
    Username string `mapstructure:"username"`
    Password string `mapstructure:"password"`
    Database string `mapstructure:"database"`
}

type Config struct {
    SSH      SSHConfig      `mapstructure:"ssh"`
    AI       AIConfig       `mapstructure:"ai"`
    Logging  LoggingConfig  `mapstructure:"logging"`
    API      APIConfig      `mapstructure:"api"`
    Database DatabaseConfig `mapstructure:"database"`  // New!
}
```

2. **Add defaults in `DefaultConfig()`:**

```go
func DefaultConfig() *Config {
    return &Config{
        // ... existing defaults ...
        Database: DatabaseConfig{
            Host:     "localhost",
            Port:     5432,
            Database: "lumo",
        },
    }
}
```

3. **Add validation in `Validate()`:**

```go
func (c *Config) Validate() error {
    // ... existing validation ...

    if c.Database.Port < 1 || c.Database.Port > 65535 {
        return fmt.Errorf("invalid database port: %d", c.Database.Port)
    }

    return nil
}
```

4. **Update `configs/config.example.yaml`:**

```yaml
database:
  host: localhost
  port: 5432
  username: lumo_user
  password: ""  # Set via LUMO_DATABASE_PASSWORD
  database: lumo
```

### Adding a New Internal Package

**Example: Creating `internal/diagnostics` package**

1. **Create directory and file:**

```bash
mkdir -p internal/diagnostics
touch internal/diagnostics/diagnostics.go
```

2. **Define package with interfaces:**

```go
// internal/diagnostics/diagnostics.go
package diagnostics

import (
    "context"
    "time"
)

// Checker represents a diagnostic check
type Checker interface {
    Name() string
    Run(ctx context.Context) (*Result, error)
}

// Result represents diagnostic results
type Result struct {
    Name      string
    Status    Status
    Message   string
    Data      map[string]interface{}
    Timestamp time.Time
}

type Status string

const (
    StatusOK      Status = "ok"
    StatusWarning Status = "warning"
    StatusError   Status = "error"
)

// Runner executes diagnostic checks
type Runner struct {
    checks []Checker
}

func NewRunner(checks ...Checker) *Runner {
    return &Runner{checks: checks}
}

func (r *Runner) RunAll(ctx context.Context) ([]*Result, error) {
    results := make([]*Result, 0, len(r.checks))

    for _, check := range r.checks {
        result, err := check.Run(ctx)
        if err != nil {
            return nil, fmt.Errorf("check %s failed: %w", check.Name(), err)
        }
        results = append(results, result)
    }

    return results, nil
}
```

3. **Use in command:**

```go
// cmd/lumo/diagnose.go
import "github.com/ignacio/lumo/internal/diagnostics"

func runDiagnostics() {
    runner := diagnostics.NewRunner(
        &CPUCheck{},
        &MemoryCheck{},
        &DiskCheck{},
    )

    results, err := runner.RunAll(context.Background())
    // Handle results...
}
```

---

## Error Handling Patterns

### Error Wrapping

**Always wrap errors with context using `fmt.Errorf` with `%w`:**

```go
if err := viper.ReadInConfig(); err != nil {
    return nil, fmt.Errorf("failed to read config file: %w", err)
}
```

### Error Chain Unwrapping

```go
import "errors"

if errors.Is(err, os.ErrNotExist) {
    // Handle missing file
}

var pathErr *os.PathError
if errors.As(err, &pathErr) {
    // Handle path error
}
```

### Validation Errors

**Return descriptive errors with values:**

```go
func (c *Config) Validate() error {
    if c.SSH.Port < 1 || c.SSH.Port > 65535 {
        return fmt.Errorf("invalid SSH port: %d (must be 1-65535)", c.SSH.Port)
    }

    if c.SSH.Timeout <= 0 {
        return fmt.Errorf("SSH timeout must be positive, got: %v", c.SSH.Timeout)
    }

    return nil
}
```

### Command-Level Error Handling

```go
var cmdCmd = &cobra.Command{
    Run: func(cmd *cobra.Command, args []string) {
        if err := doSomething(); err != nil {
            log.Errorf("Operation failed: %v", err)
            os.Exit(1)  // Exit with error code
        }

        log.Info("Operation successful")
    },
}
```

### Deferred Cleanup

```go
func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("failed to open file: %w", err)
    }
    defer f.Close()  // Always cleanup

    // Process file...
    return nil
}
```

---

## Logging Guidelines

### Log Levels

Use appropriate log levels:

```go
log.Debug("Detailed diagnostic information")   // --verbose only
log.Info("Normal operational messages")        // Default level
log.Warn("Warning conditions")                 // Potential issues
log.Error("Error conditions")                  // Errors that need attention
log.Fatal("Fatal errors - exits program")      // Critical failures
```

### Structured Logging

**Use fields for structured data:**

```go
log.WithFields(logrus.Fields{
    "host": "example.com",
    "port": 22,
    "user": "admin",
}).Info("Connecting to SSH server")

log.WithField("duration", time.Since(start)).Info("Operation completed")
```

### Contextual Logging

**Create logger with persistent context:**

```go
cmdLog := log.WithFields(logrus.Fields{
    "command": "diagnose",
    "session": sessionID,
})

cmdLog.Info("Starting diagnostics")
cmdLog.Debug("Running CPU check")
cmdLog.Warn("High memory usage detected")
```

### Formatting

```go
// Simple message
log.Info("Connection established")

// Formatted message
log.Infof("Connected to %s:%d", host, port)

// With error
log.Errorf("Failed to connect: %v", err)
```

### Dry-Run Logging

```go
if dryRun {
    log.Info("DRY RUN: Would execute command: systemctl restart nginx")
    return nil
}

log.Info("Executing: systemctl restart nginx")
// Actual execution
```

---

## Testing Strategy

### Test File Organization

**Create `*_test.go` files alongside source files:**

```
internal/config/
├── config.go
└── config_test.go
```

### Unit Testing Pattern

```go
// internal/config/config_test.go
package config

import (
    "testing"
)

func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  *Config
        wantErr bool
    }{
        {
            name: "valid config",
            config: &Config{
                SSH: SSHConfig{Port: 22, Timeout: 30 * time.Second},
            },
            wantErr: false,
        },
        {
            name: "invalid port",
            config: &Config{
                SSH: SSHConfig{Port: 99999},
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...

# Run specific test
go test -run TestConfigValidation ./internal/config

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Table-Driven Tests

**Preferred pattern for multiple scenarios:**

```go
func TestPortValidation(t *testing.T) {
    tests := []struct {
        port     int
        expected bool
    }{
        {22, true},
        {2222, true},
        {0, false},
        {-1, false},
        {65535, true},
        {65536, false},
    }

    for _, tt := range tests {
        result := isValidPort(tt.port)
        if result != tt.expected {
            t.Errorf("isValidPort(%d) = %v, want %v", tt.port, result, tt.expected)
        }
    }
}
```

---

## Common Tasks & Examples

### Task 1: Adding a New Flag to Existing Command

```go
// In cmd/lumo/diagnose.go
func init() {
    rootCmd.AddCommand(diagnoseCmd)

    // Add new flag
    diagnoseCmd.Flags().BoolP("json", "j", false, "output in JSON format")
}

// Use in command
var diagnoseCmd = &cobra.Command{
    Run: func(cmd *cobra.Command, args []string) {
        jsonOutput, _ := cmd.Flags().GetBool("json")

        if jsonOutput {
            // Output JSON
        } else {
            // Output text
        }
    },
}
```

### Task 2: Reading Configuration Values

```go
import "github.com/ignacio/lumo/internal/config"

func someFunction() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    log.Infof("SSH port: %d", cfg.SSH.Port)
    log.Infof("AI provider: %s", cfg.AI.Provider)
    log.Infof("Log level: %s", cfg.Logging.Level)
}
```

### Task 3: Using Viper Directly for Config Values

```go
import "github.com/spf13/viper"

func init() {
    viper.SetDefault("custom.option", "default_value")
}

func someFunction() {
    value := viper.GetString("custom.option")
    port := viper.GetInt("ssh.port")
    timeout := viper.GetDuration("ssh.timeout")
}
```

### Task 4: Creating a Custom Logger

```go
import (
    "github.com/sirupsen/logrus"
    "os"
)

func setupLogger() *logrus.Logger {
    logger := logrus.New()

    logger.SetFormatter(&logrus.JSONFormatter{})
    logger.SetOutput(os.Stdout)
    logger.SetLevel(logrus.InfoLevel)

    if verbose {
        logger.SetLevel(logrus.DebugLevel)
    }

    return logger
}
```

### Task 5: Implementing a Cobra Subcommand with Validation

```go
var exampleCmd = &cobra.Command{
    Use:   "example <required-arg>",
    Short: "Example command",
    Args:  cobra.ExactArgs(1),  // Require exactly 1 argument
    PreRunE: func(cmd *cobra.Command, args []string) error {
        // Validate before running
        flag, _ := cmd.Flags().GetString("important")
        if flag == "" {
            return fmt.Errorf("--important flag is required")
        }
        return nil
    },
    RunE: func(cmd *cobra.Command, args []string) error {
        // Use RunE to return errors instead of handling manually
        return doSomething(args[0])
    },
}
```

---

## Security Considerations

### .gitignore Protection

**Never commit these files (already in `.gitignore`):**

- `config.yaml`, `lumo.yaml` - May contain secrets
- `*.pem`, `*.key` - TLS certificates/keys
- `id_rsa*` - SSH private keys
- `.env`, `.env.*.local` - Environment files
- Binaries: `lumo`, `*.exe`, `*.dll`

### Credential Handling

**Always use environment variables for secrets:**

```yaml
# configs/config.example.yaml
ai:
  api_key: ""  # Set via LUMO_AI_API_KEY environment variable

database:
  password: ""  # Set via LUMO_DATABASE_PASSWORD
```

```bash
# Never do this:
export LUMO_AI_API_KEY=sk-1234567890

# Better: Use secret management
export LUMO_AI_API_KEY=$(vault kv get -field=api_key secret/lumo)
```

### TLS Validation

**Config enforces TLS security:**

```go
if c.API.TLS {
    if c.API.CertFile == "" || c.API.KeyFile == "" {
        return fmt.Errorf("TLS enabled but cert_file or key_file not provided")
    }
}
```

### File Permissions

```bash
# Configuration files
chmod 600 ~/.lumo/config.yaml

# SSH keys
chmod 600 ~/.ssh/id_rsa

# TLS certificates
chmod 644 /etc/lumo/cert.pem
chmod 600 /etc/lumo/key.pem
```

### Input Validation

**Always validate user input:**

```go
func validateHost(host string) error {
    if host == "" {
        return fmt.Errorf("host cannot be empty")
    }

    // Additional validation (DNS, IP address, etc.)
    return nil
}
```

---

## Important Files Reference

### cmd/lumo/root.go (89 lines)

**Purpose:** Root command setup, global flags, configuration initialization

**Key Functions:**
- `Execute()` - Entry point called from `main.go`
- `initConfig()` - Loads configuration from file/env vars

**Global Variables:**
- `cfgFile` - Config file path (from `--config` flag)
- `verbose` - Debug logging flag
- `dryRun` - Simulation mode flag
- `log` - Global logrus.Logger instance

**Important Sections:**
- `init()` - Registers global flags (`--config`, `--verbose`, `--dry-run`)
- `PersistentPreRun` - Sets up logging before any command runs
- `cobra.OnInitialize(initConfig)` - Deferred config loading

### internal/config/config.go (152 lines)

**Purpose:** Configuration types, loading, validation, defaults

**Key Types:**
- `Config` - Root configuration struct
- `SSHConfig` - SSH connection settings
- `AIConfig` - AI provider settings
- `LoggingConfig` - Logger configuration
- `APIConfig` - API server settings

**Key Functions:**
- `DefaultConfig() *Config` - Returns config with defaults
- `Load() (*Config, error)` - Loads config from file/env
- `(c *Config) Validate() error` - Validates all settings

**Configuration Search Paths:**
1. `--config` flag value
2. `./config.yaml`
3. `~/.lumo/config.yaml`

### cmd/lumo/main.go (5 lines)

**Purpose:** Binary entry point

```go
package main

func main() {
    Execute()  // Defined in root.go
}
```

### configs/config.example.yaml

**Purpose:** Template configuration with all available options

**Usage:**
```bash
cp configs/config.example.yaml ~/.lumo/config.yaml
```

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

### Phase 3: Diagnostic System 🚧 IN PROGRESS

**Status:** Phase 3.1 Foundation Complete (2025-11-14)
**Target Package:** `internal/diagnostics/`
**Phase 3.1 Foundation ✅ Complete:**
- ✅ Core `Checker` interface and `DiagnosticRunner` architecture
- ✅ `CheckResult` types with JSON structures
- ✅ Severity classification system with configurable thresholds
- ✅ `ThresholdConfig` for all diagnostic categories
- ✅ Parallel and sequential check execution support
**Files:** `diagnostics.go`, `result.go`, `severity.go` (858 lines)

**Phase 3.2 Remaining:**
- ⏳ Implement 8 diagnostic checkers (CPU, memory, disk, process, logs, network, service, security)
- ⏳ Command output parsers for each check
- ⏳ Platform detection (Linux, macOS, BSD)
- ⏳ Output formatters (text, JSON, YAML)
- ⏳ Update `diagnose.go` command with full integration

### Phase 4: AI Integration Layer

**Status:** Planned
**Target Package:** `internal/ai/`
**Dependencies:** Anthropic/OpenAI SDKs
**Tasks:**
- Provider abstraction (interface)
- Anthropic Claude integration
- OpenAI GPT integration
- Local model support (ollama/llama.cpp)
- Prompt engineering for diagnostics analysis
- Response parsing and structured output

### Phase 5: Auto-Remediation & Approval

**Status:** Planned
**Target Package:** `internal/remediation/`
**Tasks:**
- Remediation action registry
- Risk classification (safe, moderate, critical)
- Human-in-the-loop approval workflow
- Action execution with rollback support
- Audit logging
- Update `fix.go` command

### Phase 6: Reporting & Logging

**Status:** Planned
**Target Package:** `internal/reporting/`
**Tasks:**
- Report generation (Markdown, JSON, YAML, HTML)
- Historical data storage
- Trend analysis
- Export functionality
- Update `report.go` command

### Phase 7: API Server

**Status:** Planned
**Target Package:** `internal/server/`
**Dependencies:** Gin or Echo framework
**Tasks:**
- REST API endpoints
- WebSocket for real-time logs
- Authentication/authorization
- Rate limiting
- API documentation (OpenAPI/Swagger)
- Update `serve.go` command

### Phase 8: Testing & Documentation

**Status:** Planned
**Tasks:**
- Unit tests for all packages (target: 80% coverage)
- Integration tests for CLI commands
- End-to-end tests
- Benchmarking for critical paths
- GoDoc comments for all exported symbols
- User guide and tutorials

---

## Troubleshooting

### Issue: Config file not found

**Problem:**
```
Error: failed to load config: Config File "config" Not Found in [...]
```

**Solution:**
```bash
# Create config directory
mkdir -p ~/.lumo

# Copy example config
cp configs/config.example.yaml ~/.lumo/config.yaml

# Or specify explicit path
lumo --config ./my-config.yaml diagnose
```

### Issue: Environment variables not working

**Problem:** Setting `LUMO_SSH_PORT` doesn't change the SSH port

**Solution:** Ensure environment variable naming is correct:

```bash
# Correct
export LUMO_SSH_PORT=2222

# Incorrect (no prefix)
export SSH_PORT=2222

# Verify Viper is reading it
lumo diagnose --verbose  # Check debug output
```

### Issue: Build fails with import errors

**Problem:**
```
go build: cannot find package "github.com/spf13/cobra"
```

**Solution:**
```bash
# Download dependencies
go mod download

# Or tidy and download
go mod tidy
```

### Issue: Command not recognized

**Problem:** `lumo: command not found`

**Solution:**
```bash
# Install globally
go install ./cmd/lumo

# Or use explicit path
./lumo diagnose

# Or add to PATH
export PATH=$PATH:$(go env GOPATH)/bin
```

---

## AI Assistant Best Practices

### DO:

✅ **Use existing patterns** - Follow established code structure and conventions
✅ **Validate inputs** - Always validate configuration and user input
✅ **Wrap errors** - Use `fmt.Errorf("context: %w", err)` for error chains
✅ **Log appropriately** - Use correct log levels (debug/info/warn/error)
✅ **Add tests** - Write unit tests for new functionality
✅ **Document exports** - Add GoDoc comments to public functions/types
✅ **Use interfaces** - Define interfaces in consumer packages
✅ **Handle dry-run** - Respect `dryRun` flag in destructive operations

### DON'T:

❌ **Commit secrets** - Never hardcode API keys, passwords, or private keys
❌ **Ignore errors** - Always handle or propagate errors
❌ **Use global state** - Minimize global variables (except logger, rootCmd)
❌ **Panic unnecessarily** - Use error returns instead of `panic()`
❌ **Skip validation** - Always validate before executing operations
❌ **Hardcode paths** - Use config values or flags for file paths
❌ **Mix concerns** - Keep CLI logic separate from business logic

### Code Review Checklist:

- [ ] Does it follow Go naming conventions?
- [ ] Are errors properly wrapped with context?
- [ ] Is logging used appropriately?
- [ ] Are there tests for new functionality?
- [ ] Is documentation updated (README, comments)?
- [ ] Does it handle dry-run mode?
- [ ] Are configuration changes reflected in `config.example.yaml`?
- [ ] Are new dependencies necessary and well-justified?
- [ ] Is the code formatted with `go fmt`?
- [ ] Does it pass `go vet` and linting?

---

## Quick Reference

### Key Commands

```bash
# Build
go build -o lumo ./cmd/lumo

# Run with verbose logging
./lumo --verbose diagnose

# Dry run
./lumo fix --dry-run

# Custom config
./lumo --config /path/to/config.yaml connect user@host

# Environment override
LUMO_SSH_PORT=2222 ./lumo connect user@host
```

### Key Files

| File | Purpose |
|------|---------|
| `cmd/lumo/root.go` | Root command, global flags, config loading |
| `cmd/lumo/main.go` | Entry point |
| `internal/config/config.go` | Configuration types and validation |
| `configs/config.example.yaml` | Configuration template |
| `.gitignore` | Protected files (secrets, binaries) |
| `go.mod` | Dependencies |

### Key Patterns

```go
// Error wrapping
return fmt.Errorf("context: %w", err)

// Logging
log.WithFields(logrus.Fields{"key": "value"}).Info("message")

// Config loading
cfg, err := config.Load()

// Command registration
rootCmd.AddCommand(myCmd)

// Flag definition
cmd.Flags().StringP("name", "n", "default", "help")
```

---

**End of CLAUDE.md**

> For questions or improvements to this guide, please open an issue at https://github.com/ignacio/lumo/issues
