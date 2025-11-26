# Contributing to Lumo

Thank you for your interest in contributing to Lumo! This guide will help you get started.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Pull Request Process](#pull-request-process)
- [Code Style](#code-style)
- [Testing](#testing)
- [Documentation](#documentation)

## Code of Conduct

We are committed to providing a welcoming and inclusive environment. Please be respectful and constructive in all interactions.

**Expected behavior:**
- Be respectful and inclusive
- Provide constructive feedback
- Focus on what's best for the community
- Show empathy towards others

**Unacceptable behavior:**
- Harassment, discrimination, or personal attacks
- Trolling or inflammatory comments
- Publishing others' private information

## Getting Started

### Prerequisites

- **Go 1.25+** - [Install Go](https://golang.org/doc/install)
- **Docker** - For local development environment
- **Make** - For running build commands

### Initial Setup

```bash
# Clone the repository
git clone https://github.com/IgnacioPro/lumo.git
cd lumo

# Run the setup script (installs tools, starts services)
make setup

# Verify everything works
make ci
```

### Project Structure

```
lumo/
├── cmd/              # CLI and Agent entry points
├── internal/         # Private application code (see package READMEs)
├── api/proto/        # Protocol Buffer definitions
├── configs/          # Example configurations
├── deployments/      # Kubernetes and systemd deployments
├── examples/         # End-to-end usage examples
├── tests/            # Integration and load tests
└── docs/             # Additional documentation
```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation only
- `refactor/` - Code refactoring
- `test/` - Test additions/improvements

### 2. Make Changes

- Keep changes focused and atomic
- Follow existing code patterns
- Add tests for new functionality
- Update documentation as needed

### 3. Run CI Checks Locally

**Always run before committing:**

```bash
make ci
```

This runs:
- `golangci-lint` - Code linting (50+ linters)
- `govulncheck` - Security vulnerability scanning
- `go test -race` - Tests with race detection
- `go build` - Build verification

### 4. Commit Your Changes

Write clear, descriptive commit messages:

```bash
# Good
git commit -m "Add circuit breaker to notification providers"
git commit -m "Fix race condition in event debouncer"

# Bad
git commit -m "fix stuff"
git commit -m "wip"
```

## Pull Request Process

### Before Submitting

1. ✅ Run `make ci` and ensure all checks pass
2. ✅ Add/update tests for your changes
3. ✅ Update documentation if needed
4. ✅ Rebase on latest `main` if needed

### PR Guidelines

- Fill out the PR template completely
- Link related issues using `Fixes #123` or `Relates to #123`
- Keep PRs focused - one feature/fix per PR
- Respond to review feedback promptly

### Review Process

1. A maintainer will review your PR
2. Address any requested changes
3. Once approved, a maintainer will merge

## Code Style

### Go Conventions

We follow standard Go conventions:

```go
// Package-level comments for godoc
package mypackage

// Exported functions have doc comments
// FunctionName does something important.
func FunctionName() error {
    // ...
}

// Error wrapping with context
if err != nil {
    return fmt.Errorf("context description: %w", err)
}

// Structured logging
log.WithFields(logrus.Fields{
    "key": value,
}).Info("descriptive message")
```

### Import Organization

```go
import (
    // Standard library
    "context"
    "fmt"

    // Third-party packages
    "github.com/sirupsen/logrus"

    // Internal packages
    "github.com/ignacio/lumo/internal/config"
)
```

### Linting

We use `golangci-lint` with the default configuration. Run locally:

```bash
make lint
```

## Testing

### Running Tests

```bash
# All tests
make test

# With verbose output
make test-verbose

# With coverage report
make coverage-html

# Integration tests (requires Docker)
go test -v ./tests/integration/...
```

### Writing Tests

- Use table-driven tests for multiple cases
- Use `testify/assert` for assertions
- Mock external dependencies
- Aim for meaningful coverage, not just numbers

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "foo", "FOO", false},
        {"empty input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            assert.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

## Documentation

### When to Update Docs

- Adding new features → Update README and relevant package docs
- Changing CLI commands → Update command help and examples
- Changing configuration → Update `configs/` examples
- API changes → Update API documentation

### Package Documentation

Each significant package should have a `README.md` explaining:
- Purpose of the package
- Key types and interfaces
- Usage examples

## Questions?

- Open a [GitHub Issue](https://github.com/IgnacioPro/lumo/issues) for bugs or features
- Check existing issues before creating new ones

Thank you for contributing! 🎉
