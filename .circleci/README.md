# CircleCI Configuration

This directory contains the CircleCI configuration for the Lumo project.

## Overview

The CircleCI pipeline provides continuous integration and testing for Lumo, running in parallel with GitHub Actions. It provides:

- **Linting & Security Checks**: golangci-lint and govulncheck
- **Testing**: Go tests with race detection
- **Building**: CLI and Agent binaries
- **Cross-Platform Builds**: Linux and macOS for amd64/arm64 (main/master only)
- **Nightly Integration Tests**: Full CI suite runs daily

## Workflows

### 1. Main CI Workflow (`ci`)

Runs on **all branches** for every push:

```
lint-and-security
       │
       ├─── golangci-lint
       └─── govulncheck

test
       │
       └─── go test -race

build (requires lint + test)
       │
       ├─── Build CLI
       └─── Build Agent
```

**Triggers**: Push to any branch

### 2. Cross-Platform Builds (`build-all-platforms`)

Runs only on **main/master branches**:

- Linux: amd64, arm64
- Darwin (macOS): amd64, arm64

**Triggers**: Push to main/master branches only

### 3. Nightly Integration Test (`nightly`)

Runs full CI suite (`make ci`) every night at midnight UTC.

**Triggers**: Cron schedule `0 0 * * *` on main/master

## Local Development

To run the same checks locally before pushing:

```bash
# Run all CI checks (matches CircleCI workflow)
make ci

# Run individual checks
make ci-lint    # Linting + security
make ci-test    # Tests with race detection
make ci-build   # Build both binaries
```

## Setup Instructions

### 1. Connect Repository to CircleCI

1. Go to [CircleCI](https://circleci.com/)
2. Sign in with GitHub
3. Click "Set Up Project" for the `lumo` repository
4. CircleCI will automatically detect the `.circleci/config.yml` file

### 2. Environment Variables (Optional)

No environment variables are required for basic CI. However, you may want to add:

- `GITHUB_TOKEN`: For accessing private dependencies (if any)
- `CODECOV_TOKEN`: If integrating with Codecov for test coverage

Set these in: **Project Settings → Environment Variables**

### 3. Status Badge

Add to README.md:

```markdown
[![CircleCI](https://dl.circleci.com/status-badge/img/gh/ignacio/lumo/tree/main.svg?style=shield)](https://dl.circleci.com/status-badge/link/gh/ignacio/lumo/tree/main)
```

## Configuration Details

### Executor

Uses official CircleCI Go image:
- **Image**: `cimg/go:1.25.4`
- **Features**: Pre-installed Go, git, Docker

### Caching

CircleCI automatically caches:
- Go module downloads
- Build cache

### Artifacts

Build artifacts are stored for:
- Test results (`/tmp/test-results`)
- Cross-platform binaries (lumo-*, lumo-agent-*)

**Retention**: 30 days (CircleCI default)

### Parallelism

Jobs run in parallel where possible:
- `lint-and-security` and `test` run concurrently
- Cross-platform builds run in parallel (4 jobs)

## Comparison with GitHub Actions

| Feature | GitHub Actions | CircleCI |
|---------|---------------|----------|
| **Trigger** | Push, PR | Push only |
| **Platforms** | linux, darwin | linux, darwin |
| **Runner** | Self-hosted | Cloud |
| **Caching** | Manual setup | Automatic |
| **Artifacts** | 90 days | 30 days |
| **Cost** | Free (self-hosted) | Free tier: 6,000 credits/month |

**Recommendation**: Keep both CI systems for redundancy and cloud/self-hosted coverage.

## Troubleshooting

### Build Fails on Dependency Download

**Symptom**: `go mod download` fails
**Solution**: Check network connectivity or add `GOPROXY` environment variable:

```bash
GOPROXY=https://proxy.golang.org,direct
```

### golangci-lint Timeout

**Symptom**: Linter exceeds 5-minute timeout
**Solution**: Increase timeout in `.circleci/config.yml`:

```yaml
- run:
    name: Run golangci-lint
    command: golangci-lint run --timeout=10m
```

### Race Detector Fails

**Symptom**: Tests fail only with `-race` flag
**Solution**: Genuine race condition detected. Fix the code, don't disable the check.

### Cross-Platform Build Missing

**Symptom**: Build doesn't appear in CircleCI
**Solution**: Ensure you're pushing to `main` or `master` branch (cross-platform builds only run there).

## Resources

- [CircleCI Documentation](https://circleci.com/docs/)
- [CircleCI Go Language Guide](https://circleci.com/docs/language-go/)
- [Lumo Makefile Targets](../Makefile)
- [GitHub Actions Comparison](.github/workflows/ci.yml)

## Maintenance

**Config File**: `.circleci/config.yml`
**Last Updated**: 2025-11-21
**CircleCI Config Version**: 2.1
**Go Version**: 1.25.4

When updating Go version, update in:
1. `.circleci/config.yml` (executor image)
2. `.github/workflows/ci.yml`
3. `.github/workflows/release.yml`
4. `go.mod`
