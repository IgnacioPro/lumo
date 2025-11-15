# DEVELOPMENT.md - Developer Guide for Lumo

> **Companion to CLAUDE.md** - Detailed examples, tutorials, and troubleshooting

This document provides detailed examples and step-by-step tutorials for common development tasks in Lumo. For architectural overview and AI assistant guidance, see [CLAUDE.md](CLAUDE.md).

---

## Table of Contents

1. [Adding New Features](#adding-new-features)
2. [Common Tasks & Examples](#common-tasks--examples)
3. [Detailed File Reference](#detailed-file-reference)
4. [Troubleshooting](#troubleshooting)

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

---

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
    SSH         SSHConfig         `mapstructure:"ssh"`
    AI          AIConfig          `mapstructure:"ai"`
    Logging     LoggingConfig     `mapstructure:"logging"`
    API         APIConfig         `mapstructure:"api"`
    Diagnostics DiagnosticsConfig `mapstructure:"diagnostics"` // Already exists!
    Database    DatabaseConfig    `mapstructure:"database"`    // New!
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

---

### Adding a New Internal Package

**Example: Creating `internal/metrics` package**

1. **Create directory and file:**

```bash
mkdir -p internal/metrics
touch internal/metrics/metrics.go
```

2. **Define package with interfaces:**

```go
// internal/metrics/metrics.go
package metrics

import (
    "context"
    "time"
)

// Collector represents a metrics collector
type Collector interface {
    Name() string
    Collect(ctx context.Context) (*Metrics, error)
}

// Metrics represents collected metrics
type Metrics struct {
    Name      string
    Values    map[string]float64
    Timestamp time.Time
}

// Registry manages metric collectors
type Registry struct {
    collectors []Collector
}

func NewRegistry(collectors ...Collector) *Registry {
    return &Registry{collectors: collectors}
}

func (r *Registry) CollectAll(ctx context.Context) ([]*Metrics, error) {
    results := make([]*Metrics, 0, len(r.collectors))

    for _, collector := range r.collectors {
        metrics, err := collector.Collect(ctx)
        if err != nil {
            return nil, fmt.Errorf("collector %s failed: %w", collector.Name(), err)
        }
        results = append(results, metrics)
    }

    return results, nil
}
```

3. **Use in command:**

```go
// cmd/lumo/metrics.go
import "github.com/ignacio/lumo/internal/metrics"

func runMetrics() {
    registry := metrics.NewRegistry(
        &CPUMetrics{},
        &MemoryMetrics{},
    )

    results, err := registry.CollectAll(context.Background())
    // Handle results...
}
```

---

### Adding a New Diagnostic Checker

**Example: Creating a log checker**

1. **Create `internal/diagnostics/checkers/logs.go`:**

```go
package checkers

import (
    "context"
    "fmt"
    "time"

    "github.com/ignacio/lumo/internal/diagnostics"
)

type LogChecker struct {
    logPaths  []string
    threshold LogThreshold
}

type LogThreshold struct {
    MaxErrorRate float64 // Errors per minute
    MaxWarnings  int     // Total warnings in window
}

func NewLogChecker(paths []string, threshold LogThreshold) *LogChecker {
    return &LogChecker{
        logPaths:  paths,
        threshold: threshold,
    }
}

func (c *LogChecker) Name() string {
    return "logs"
}

func (c *LogChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
    result := &diagnostics.CheckResult{
        Name:      c.Name(),
        Category:  "logs",
        Timestamp: time.Now(),
        Metrics:   []diagnostics.Metric{},
    }

    // Check each log file
    for _, path := range c.logPaths {
        // Example: Count errors in last 1 hour
        cmd := fmt.Sprintf("grep -c ERROR %s | tail -n 60", path)
        stdout, _, exitCode, err := executor.Execute(cmd, 10*time.Second)

        if err != nil || exitCode != 0 {
            result.AddMetric("log_errors", 0, "errors/min")
            continue
        }

        // Parse and add metrics
        // ... implementation ...
    }

    // Evaluate severity
    result.Severity = c.evaluateSeverity(result)

    return result, nil
}

func (c *LogChecker) evaluateSeverity(result *diagnostics.CheckResult) diagnostics.Severity {
    // Implementation based on thresholds
    return diagnostics.SeverityOK
}
```

2. **Register in `cmd/lumo/diagnose.go`:**

```go
runner.RegisterCheckers(
    checkers.NewCPUChecker(thresholds.CPU),
    checkers.NewMemoryChecker(thresholds.Memory),
    checkers.NewLogChecker(
        []string{"/var/log/syslog", "/var/log/messages"},
        checkers.LogThreshold{MaxErrorRate: 10.0, MaxWarnings: 100},
    ),
)
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
            data, _ := json.MarshalIndent(results, "", "  ")
            fmt.Println(string(data))
        } else {
            // Output text
            formatter := formatters.NewTextFormatter(!noColor, verbose)
            fmt.Println(formatter.FormatReport(report))
        }
    },
}
```

---

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

    // Access nested config
    if cfg.AI.Enabled {
        log.Infof("Using model: %s", cfg.AI.GetModelForProvider(cfg.AI.Provider))
    }
}
```

---

### Task 3: Using Viper Directly for Config Values

```go
import "github.com/spf13/viper"

func init() {
    // Set defaults for custom config
    viper.SetDefault("custom.option", "default_value")
    viper.SetDefault("custom.timeout", 30*time.Second)
}

func someFunction() {
    // Read values directly
    value := viper.GetString("custom.option")
    port := viper.GetInt("ssh.port")
    timeout := viper.GetDuration("ssh.timeout")
    enabled := viper.GetBool("ai.enabled")

    log.Infof("Custom option: %s", value)
}
```

---

### Task 4: Creating a Custom Logger

```go
import (
    "github.com/sirupsen/logrus"
    "os"
)

func setupLogger(level string, format string) *logrus.Logger {
    logger := logrus.New()

    // Set formatter
    if format == "json" {
        logger.SetFormatter(&logrus.JSONFormatter{})
    } else {
        logger.SetFormatter(&logrus.TextFormatter{
            FullTimestamp: true,
        })
    }

    // Set output
    logger.SetOutput(os.Stdout)

    // Set level
    switch level {
    case "debug":
        logger.SetLevel(logrus.DebugLevel)
    case "info":
        logger.SetLevel(logrus.InfoLevel)
    case "warn":
        logger.SetLevel(logrus.WarnLevel)
    case "error":
        logger.SetLevel(logrus.ErrorLevel)
    default:
        logger.SetLevel(logrus.InfoLevel)
    }

    return logger
}

// Usage
customLog := setupLogger("debug", "json")
customLog.Info("Custom logger initialized")
```

---

### Task 5: Implementing a Cobra Subcommand with Validation

```go
var exampleCmd = &cobra.Command{
    Use:   "example <required-arg>",
    Short: "Example command with validation",
    Args:  cobra.ExactArgs(1),  // Require exactly 1 argument
    PreRunE: func(cmd *cobra.Command, args []string) error {
        // Validate before running
        flag, _ := cmd.Flags().GetString("important")
        if flag == "" {
            return fmt.Errorf("--important flag is required")
        }

        // Additional validation
        if len(args[0]) < 3 {
            return fmt.Errorf("argument must be at least 3 characters")
        }

        return nil
    },
    RunE: func(cmd *cobra.Command, args []string) error {
        // Use RunE to return errors instead of handling manually
        result, err := doSomething(args[0])
        if err != nil {
            return fmt.Errorf("failed to do something: %w", err)
        }

        fmt.Println(result)
        return nil
    },
}

func init() {
    rootCmd.AddCommand(exampleCmd)

    exampleCmd.Flags().String("important", "", "important flag (required)")
    exampleCmd.Flags().Bool("optional", false, "optional flag")
}
```

---

### Task 6: Working with SSH Executor

```go
import (
    "context"
    "time"

    "github.com/ignacio/lumo/internal/diagnostics"
    "github.com/ignacio/lumo/internal/ssh"
)

func runRemoteCommand(sshClient *ssh.Client, command string) error {
    // Create SSH executor
    executor := diagnostics.NewSSHExecutor(sshClient)

    // Execute with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, command)

    if err != nil {
        return fmt.Errorf("command execution failed: %w", err)
    }

    if exitCode != 0 {
        log.Warnf("Command exited with code %d: %s", exitCode, stderr)
    }

    log.Infof("Command output: %s", stdout)
    return nil
}
```

---

### Task 7: Working with Local Executor

```go
import (
    "context"
    "time"

    "github.com/ignacio/lumo/internal/diagnostics"
)

func runLocalCommand(command string) error {
    // Create local executor
    executor := diagnostics.NewLocalExecutor()

    // Execute with timeout
    stdout, stderr, exitCode, err := executor.Execute(command, 30*time.Second)

    if err != nil {
        return fmt.Errorf("local command failed: %w", err)
    }

    if exitCode != 0 {
        log.Warnf("Command exited with code %d: %s", exitCode, stderr)
        return fmt.Errorf("command failed")
    }

    log.Infof("Output: %s", stdout)
    return nil
}
```

---

### Task 8: Creating a Custom Formatter

```go
package formatters

import (
    "fmt"
    "strings"

    "github.com/ignacio/lumo/internal/diagnostics"
)

type CustomFormatter struct {
    color   bool
    verbose bool
}

func NewCustomFormatter(color, verbose bool) *CustomFormatter {
    return &CustomFormatter{
        color:   color,
        verbose: verbose,
    }
}

func (f *CustomFormatter) FormatReport(report *diagnostics.Report) string {
    var sb strings.Builder

    // Header
    sb.WriteString("=== CUSTOM DIAGNOSTIC REPORT ===\n\n")

    // Summary
    sb.WriteString(fmt.Sprintf("Total Checks: %d\n", report.Summary.TotalChecks))
    sb.WriteString(fmt.Sprintf("OK: %d, Warnings: %d, Errors: %d\n\n",
        report.Summary.OKCount,
        report.Summary.WarningCount,
        report.Summary.ErrorCount))

    // Results
    for _, result := range report.Results {
        sb.WriteString(f.formatResult(result))
        sb.WriteString("\n")
    }

    return sb.String()
}

func (f *CustomFormatter) formatResult(result *diagnostics.CheckResult) string {
    status := "✓"
    if result.Severity >= diagnostics.SeverityWarning {
        status = "⚠"
    }
    if result.Severity >= diagnostics.SeverityError {
        status = "✗"
    }

    return fmt.Sprintf("%s %s: %s", status, result.Name, result.Message)
}
```

---

### Task 9: Configuring Network Targets for Connectivity Testing

**Goal:** Configure custom network targets to test connectivity to your infrastructure (databases, APIs, internal services)

**Steps:**

1. **Create or edit your config file:**

```bash
# Copy example config if you don't have one
cp configs/config.example.yaml ~/.lumo/config.yaml

# Edit the diagnostics section
vim ~/.lumo/config.yaml
```

2. **Add network targets in config:**

```yaml
diagnostics:
  network:
    targets:
      # ICMP ping test (port 0 means ICMP-only)
      - host: 8.8.8.8
        port: 0
        protocol: icmp

      # Test database connectivity (TCP)
      - host: database.internal
        port: 5432
        protocol: tcp

      # Test API endpoint
      - host: api.myservice.com
        port: 443
        protocol: tcp

      # Test Redis cache
      - host: cache.internal
        port: 6379
        protocol: tcp
```

3. **Run network diagnostics:**

```bash
# Test all configured targets
lumo diagnose localhost --checks network

# Test on a remote server (targets are tested FROM that server)
lumo diagnose user@server --checks network

# Get detailed JSON output
lumo diagnose localhost --checks network --format json | jq '.results[0].data.target_results'
```

4. **Understand the results:**

Each target result includes:
- `reachable`: boolean - whether connection succeeded
- `latency_ms`: float - connection latency in milliseconds
- `error`: string - error classification (if failed):
  - `connection_refused`: Port is closed/nothing listening
  - `dns_failed`: Hostname couldn't be resolved
  - `timeout`: Connection timed out (host/port unreachable)
  - `unreachable`: General network unreachability

**Example JSON output:**
```json
{
  "target_results": [
    {
      "host": "database.internal",
      "port": 5432,
      "protocol": "tcp",
      "reachable": true,
      "latency_ms": 2.5
    },
    {
      "host": "api.myservice.com",
      "port": 443,
      "protocol": "tcp",
      "reachable": false,
      "latency_ms": 5001,
      "error": "timeout"
    }
  ]
}
```

**Use cases:**
- Monitor connectivity to critical infrastructure
- Test database accessibility before deployments
- Verify API endpoints are reachable
- Detect network isolation issues
- Measure network latency to services

---

## Detailed File Reference

### cmd/lumo/root.go

**Purpose:** Root command setup, global flags, configuration initialization

**Key Components:**

```go
// Global variables
var (
    cfgFile string        // Config file path (from --config flag)
    verbose bool          // Debug logging flag
    dryRun  bool          // Simulation mode flag
    log     *logrus.Logger // Global logger instance
)

// Root command definition
var rootCmd = &cobra.Command{
    Use:     "lumo",
    Short:   "Intelligent SRE/DevOps Agent",
    Version: "0.4.0",
}

// Execute is the entry point
func Execute() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}

// initConfig loads configuration
func initConfig() {
    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        viper.AddConfigPath(".")
        viper.AddConfigPath("$HOME/.lumo")
        viper.SetConfigName("config")
    }

    viper.AutomaticEnv()
    viper.SetEnvPrefix("LUMO")

    if err := viper.ReadInConfig(); err == nil {
        log.Infof("Using config file: %s", viper.ConfigFileUsed())
    }
}
```

**Initialization Flow:**
1. `cobra.OnInitialize(initConfig)` - Deferred config loading
2. `PersistentPreRun` - Sets up logging before any command runs
3. Command `Run` - Actual command execution

---

### internal/config/config.go

**Purpose:** Configuration types, loading, validation, defaults

**Configuration Loading:**

```go
func Load() (*Config, error) {
    cfg := DefaultConfig()

    if err := viper.Unmarshal(cfg); err != nil {
        return nil, fmt.Errorf("failed to unmarshal config: %w", err)
    }

    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }

    return cfg, nil
}
```

**Search Paths:**
1. `--config` flag value (explicit path)
2. `./config.yaml` (current directory)
3. `~/.lumo/config.yaml` (home directory)

**Environment Variables:**
- Pattern: `LUMO_<SECTION>_<KEY>`
- Example: `LUMO_SSH_PORT=2222`
- Automatically mapped by Viper

---

### internal/diagnostics/diagnostics.go

**Purpose:** Core diagnostic system orchestration

**Key Interfaces:**

```go
// Checker interface - all checkers must implement this
type Checker interface {
    Name() string
    Run(ctx context.Context, executor CommandExecutor) (*CheckResult, error)
}

// CommandExecutor interface - abstraction for SSH/local execution
type CommandExecutor interface {
    Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)
    ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error)
}
```

**Runner Implementation:**

```go
type Runner struct {
    config    *RunConfig
    thresholds *ThresholdConfig
    executor  CommandExecutor
    checkers  []Checker
    log       *logrus.Logger
}

func (r *Runner) RunAll(ctx context.Context) (*Report, error) {
    results := make([]*CheckResult, 0, len(r.checkers))

    for _, checker := range r.checkers {
        result, err := checker.Run(ctx, r.executor)
        if err != nil {
            return nil, fmt.Errorf("checker %s failed: %w", checker.Name(), err)
        }
        results = append(results, result)
    }

    return r.buildReport(results), nil
}
```

---

### internal/ai/types.go

**Purpose:** Core AI provider interfaces and types

**Provider Interface:**

```go
type Provider interface {
    Name() string
    Analyze(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error)
    AnalyzeStream(ctx context.Context, req *AnalysisRequest) (<-chan StreamChunk, error)
    Health(ctx context.Context) error
}
```

**Analysis Request/Response:**

```go
type AnalysisRequest struct {
    Report     *diagnostics.Report
    SystemInfo SystemInfo
    Focus      []string // Areas to focus analysis on
}

type AnalysisResponse struct {
    OverallHealth   HealthStatus
    Summary         string
    Findings        []Finding
    Recommendations []Recommendation
    Confidence      float64
    Provider        string
    Model           string
    Duration        time.Duration
    TokensUsed      *TokenUsage
}
```

---

## Troubleshooting

### Issue: Config file not found

**Problem:**
```
Error: failed to load config: Config File "config" Not Found in [...]
```

**Solution:**
```bash
# Option 1: Create config directory
mkdir -p ~/.lumo
cp configs/config.example.yaml ~/.lumo/config.yaml

# Option 2: Specify explicit path
lumo --config ./my-config.yaml diagnose

# Option 3: Use current directory
cp configs/config.example.yaml ./config.yaml
```

---

### Issue: Environment variables not working

**Problem:** Setting `LUMO_SSH_PORT` doesn't change the SSH port

**Root Cause:** Environment variable naming or Viper configuration issue

**Solution:**

```bash
# Correct - with LUMO prefix
export LUMO_SSH_PORT=2222

# Incorrect - no prefix
export SSH_PORT=2222

# Verify environment variable is set
echo $LUMO_SSH_PORT

# Verify Viper is reading it (with --verbose flag)
lumo --verbose diagnose
```

**Debug Steps:**
1. Check `viper.AutomaticEnv()` is called in `initConfig()`
2. Check `viper.SetEnvPrefix("LUMO")` is set
3. Enable verbose logging to see config values
4. Use `viper.AllSettings()` to debug loaded config

---

### Issue: Build fails with import errors

**Problem:**
```
go build: cannot find package "github.com/spf13/cobra"
```

**Root Cause:** Missing dependencies

**Solution:**
```bash
# Download all dependencies
go mod download

# Or tidy and download
go mod tidy

# Verify go.mod and go.sum are present
ls -la go.mod go.sum

# Clean module cache if corrupted
go clean -modcache
go mod download
```

---

### Issue: Command not recognized

**Problem:** `lumo: command not found`

**Root Cause:** Binary not in PATH or not installed

**Solution:**
```bash
# Option 1: Install globally
go install ./cmd/lumo

# Option 2: Use explicit path
./lumo diagnose

# Option 3: Add to PATH
export PATH=$PATH:$(go env GOPATH)/bin

# Verify installation
which lumo
lumo --version

# Check Go bin directory
ls -la $(go env GOPATH)/bin/lumo
```

---

### Issue: SSH connection fails

**Problem:** Cannot connect to remote host

**Troubleshooting Steps:**

```bash
# 1. Test SSH manually
ssh user@host -p 22

# 2. Check SSH config
cat ~/.ssh/config

# 3. Verify key permissions
ls -la ~/.ssh/id_rsa
chmod 600 ~/.ssh/id_rsa

# 4. Test with verbose logging
lumo --verbose connect user@host

# 5. Check SSH agent
ssh-add -l

# 6. Try with explicit key
lumo connect user@host --identity ~/.ssh/id_rsa
```

**Common Issues:**
- Wrong port (use `--port` flag)
- Key permissions too open (must be 600)
- Key not added to SSH agent
- Host key verification failed (check `~/.ssh/known_hosts`)

---

### Issue: AI analysis fails

**Problem:** AI analysis not working or returning errors

**Troubleshooting Steps:**

```bash
# 1. Verify API key is set (check provider-specific env var)
echo $LUMO_ANTHROPIC_API_KEY  # For Anthropic
echo $LUMO_OPENAI_API_KEY     # For OpenAI
echo $LUMO_GEMINI_API_KEY     # For Gemini
echo $LUMO_AI_API_KEY         # Generic fallback

# 2. Test provider health
# (check provider-specific health check endpoint)

# 3. Check network connectivity
ping api.anthropic.com   # For Anthropic
ping api.openai.com      # For OpenAI
ping generativelanguage.googleapis.com  # For Gemini

# 4. Verify config
cat ~/.lumo/config.yaml

# 5. Test with verbose logging (use provider-specific key)
LUMO_ANTHROPIC_API_KEY=sk-ant-... lumo --verbose diagnose localhost --analyze
LUMO_OPENAI_API_KEY=sk-... LUMO_AI_PROVIDER=openai lumo --verbose diagnose --analyze
LUMO_GEMINI_API_KEY=... LUMO_AI_PROVIDER=gemini lumo --verbose diagnose --analyze
```

**Common Issues:**
- API key not set or invalid (check provider-specific env var)
- Wrong provider selected in config
- Network connectivity issues
- Rate limiting (429 errors)
- Model not available
- Timeout too short (increase with `LUMO_AI_TIMEOUT`)

---

### Issue: Localhost detection not working

**Problem:** `lumo diagnose localhost` still trying to use SSH

**Debug:**

```go
// Check isLocalhost() function in diagnose.go
func isLocalhost(hostname string) bool {
    hostname = strings.ToLower(strings.TrimSpace(hostname))

    localhostPatterns := []string{
        "localhost",
        "127.0.0.1",
        "::1",
        "0.0.0.0",
        "",
        "localhost.localdomain",
    }

    for _, pattern := range localhostPatterns {
        if hostname == pattern {
            return true
        }
    }
    return false
}
```

**Solution:**
- Ensure hostname is exactly one of the supported patterns
- Check for extra whitespace or formatting
- Use `--verbose` to see executor selection

---

### Issue: Tests failing

**Problem:** `go test ./...` shows failures

**Common Causes:**

1. **Import cycle:**
```bash
# Error: import cycle not allowed
# Solution: Reorganize imports, move shared types to separate package
```

2. **Missing test dependencies:**
```bash
go get -t ./...
```

3. **Race conditions:**
```bash
# Run with race detector
go test -race ./...
```

4. **Platform-specific failures:**
```bash
# Test on specific platform
GOOS=linux go test ./...
GOOS=darwin go test ./...
```

---

### Issue: JSON output malformed

**Problem:** JSON output is not valid JSON

**Debug:**

```bash
# Test JSON output
lumo diagnose localhost --format json | jq .

# Check for mixed output (logs + JSON)
# Ensure logs go to stderr, JSON to stdout
```

**Solution:**
- Use `log.SetOutput(os.Stderr)` for JSON mode
- Ensure only JSON goes to stdout
- Validate JSON with `json.Valid()`

---

### Issue: High memory usage

**Problem:** Lumo consuming excessive memory

**Debug:**

```bash
# Profile memory usage
go test -memprofile=mem.prof ./...
go tool pprof mem.prof

# Check for leaks
go test -run=TestYourTest -memprofile=mem.prof
```

**Common Causes:**
- Large log files being read into memory
- Unbounded slices/maps
- Goroutine leaks
- Not closing connections

---

**End of DEVELOPMENT.md**

> See [CLAUDE.md](CLAUDE.md) for architectural overview and AI assistant guidance
