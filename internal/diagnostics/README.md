# Diagnostics Package

The `diagnostics` package provides system health checks and diagnostic reporting.

## Overview

This package implements a pluggable checker system with 12 built-in checks across 4 categories:

- **Core (6):** CPU, Memory, Disk, Process, Service, Network
- **Security (4):** Patch Status, Open Ports, SSH Security, Auth Failures  
- **Specialized (2):** Kubernetes, Proxmox VE

## Key Types

### Checker Interface

```go
type Checker interface {
    Name() string
    Category() CheckCategory
    Description() string
    Run(ctx context.Context, exec CommandExecutor) (*CheckResult, error)
}
```

### Runner

The `Runner` orchestrates check execution with parallel processing:

```go
runner := diagnostics.NewRunner(logger)

// Register checks
runner.RegisterChecker(checkers.NewCPUChecker())
runner.RegisterChecker(checkers.NewMemoryChecker())

// Run diagnostics
report, err := runner.Run(ctx, executor, &diagnostics.RunOptions{
    Checks:  []string{"cpu", "memory"},
    Timeout: 30 * time.Second,
})
```

## Output Formats

Three formatters are available via `internal/diagnostics/formatters`:

| Format | Use Case |
|--------|----------|
| `text` | Human-readable terminal output |
| `json` | Machine-parseable, API responses |
| `toon` | Token-optimized for AI analysis (30-60% reduction) |

```go
formatter := formatters.NewToonFormatter()
output, err := formatter.Format(report)
```

## Check Results

Each check returns structured data:

```go
type CheckResult struct {
    Name        string
    Category    CheckCategory
    Status      CheckStatus      // completed, failed, skipped, timeout
    Metrics     map[string]any   // Numeric data
    Issues      []Issue          // Detected problems
    Suggestions []string         // Recommendations
    Duration    time.Duration
}
```

## Adding New Checks

1. Create a new file in `internal/diagnostics/checkers/`
2. Implement the `Checker` interface
3. Register in the runner

```go
type MyChecker struct{}

func (c *MyChecker) Name() string { return "mycheck" }
func (c *MyChecker) Category() CheckCategory { return CategoryCustom }
func (c *MyChecker) Description() string { return "My custom check" }
func (c *MyChecker) Run(ctx context.Context, exec CommandExecutor) (*CheckResult, error) {
    // Implementation
}
```

## Directory Structure

```
diagnostics/
├── diagnostics.go      # Runner and core types
├── formatters/         # Output formatters (text, JSON, TOON)
└── checkers/           # Individual check implementations
    ├── cpu.go
    ├── memory.go
    ├── disk.go
    ├── kubernetes.go
    └── ...
```
