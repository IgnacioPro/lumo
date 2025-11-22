# CircleCI Configuration

This directory contains the CircleCI configuration for the Lumo project.

## Overview

The CircleCI pipeline provides continuous integration and testing for Lumo, running in parallel with GitHub Actions. **Optimized for free tier usage** with caching, branch filtering, and reduced build frequency.

- **Linting & Security Checks**: golangci-lint and govulncheck (cached)
- **Testing**: Go tests with race detection
- **Building**: CLI and Agent binaries
- **Cross-Platform Builds**: Linux and macOS for amd64/arm64 (main/master only)
- **Weekly Integration Tests**: Full CI suite runs Sundays (reduced from daily)

### Cost Optimizations

To stay within free tier limits (6,000 credits/month), this config implements:

1. **Branch filtering**: Only runs on `main`/`master` (not all branches) - saves ~90% of credits
2. **Dependency caching**: Go modules and tools cached - saves ~40% per run
3. **Resource sizing**: Uses `small` resource class - saves 50% credits per job
4. **Merged workflows**: Cross-platform builds require main build - prevents duplicate runs
5. **Reduced frequency**: Weekly integration tests instead of daily - saves 85% on scheduled runs

## Workflows

### 1. Main CI Workflow (`ci`)

Runs **ONLY on main/master branches** (not feature branches):

```
lint-and-security (cached)
       │
       ├─── golangci-lint
       └─── govulncheck

test (cached)
       │
       └─── go test -race

build (requires lint + test, cached)
       │
       ├─── Build CLI
       └─── Build Agent
       │
       └─── Cross-platform builds (4 parallel jobs)
            ├─── Linux: amd64, arm64
            └─── Darwin: amd64, arm64
```

**Triggers**: Push to main/master only (saves ~90% credits vs all branches)

### 2. Weekly Integration Test (`weekly`)

Runs full CI suite (`make ci`) every Sunday at midnight UTC.

**Triggers**: Cron schedule `0 0 * * 0` on main/master (reduced from daily)

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

Aggressive caching to reduce build times and credits:
- **Go modules**: Cached by `go.sum` checksum (~40% savings)
- **Build tools**: golangci-lint, govulncheck cached by architecture
- **Build cache**: Go build cache automatically preserved

Cache keys:
- `go-mod-v1-{{ checksum "go.sum" }}` - Go dependencies
- `tools-v1-{{ arch }}` - golangci-lint, govulncheck

**Tip**: Bump version (`v1` → `v2`) in cache keys to invalidate all caches

### Artifacts

Build artifacts are stored for:
- Test results (`/tmp/test-results`)
- Cross-platform binaries (lumo-*, lumo-agent-*)

**Retention**: 30 days (CircleCI default)

### Parallelism

Jobs run in parallel where possible:
- `lint-and-security` and `test` run concurrently
- Cross-platform builds run in parallel (4 jobs) after main build succeeds

### Credit Usage (Free Tier: 6,000/month)

**Per main branch push** (~150-200 credits):
- lint-and-security: ~30 credits (small, cached)
- test: ~30 credits (small, cached)
- build: ~30 credits (small, cached)
- Cross-platform builds: ~60-80 credits (4 × small)

**Weekly integration**: ~50 credits/week = ~200 credits/month

**Expected monthly usage**: ~1,500-2,000 credits (well within free tier)
- Assumes ~10 pushes to main/month
- Weekly integration tests
- All with caching enabled

**Previous usage**: ~6,000+ credits (was running on ALL branches + daily integration)

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

## Monitoring Credit Usage

Check your credit consumption: **Project Settings → Plan → Usage**

If credits are still too high:
1. Reduce cross-platform builds (keep only linux-amd64 + darwin-arm64)
2. Disable weekly integration test
3. Use GitHub Actions exclusively (it has path filtering built-in)

## Maintenance

**Config File**: `.circleci/config.yml`
**Last Updated**: 2025-11-22 (optimized for free tier)
**CircleCI Config Version**: 2.1
**Go Version**: 1.25.4
**Resource Class**: `small` (saves 50% credits vs `medium`)

**Optimizations Applied**:
- Branch filtering (main/master only)
- Dependency caching (Go modules + tools)
- Merged workflows (prevents duplicate runs)
- Reduced scheduled tests (weekly vs daily)
- Smaller resource class

When updating Go version, update in:
1. `.circleci/config.yml` (executor image)
2. `.github/workflows/ci.yml`
3. `.github/workflows/release.yml`
4. `go.mod`
