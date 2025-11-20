# Remediation Plan

**Target:** Lumo Codebase
**Date:** 2026-03-15

This document tracks the remediation of issues identified during the production-readiness audit.

## 1. Completed Actions (P0/P1)

| Severity | Issue | Location | Fix | Status |
|----------|-------|----------|-----|--------|
| **P0** | SQL Injection (G201) | `internal/database/repository/job.go` | Verified strict allowlisting and added `// #nosec G201` suppression. | ✅ Fixed |
| **P0** | Integer Overflow (G115) | `internal/agent/reporter.go` | Added shift capping logic `if shift > 30`. | ✅ Fixed |
| **P0** | Integer Overflow (G115) | `internal/ssh/retry.go` | Verified `MaxAttempts > 0` check and added suppression. | ✅ Fixed |
| **P1** | File Perms (G301/G302) | `internal/remediation/audit.go` | Changed `0755` -> `0750`, `0644` -> `0600`. | ✅ Fixed |
| **P1** | File Perms (G306) | `internal/agent/cache.go` | Changed `0644` -> `0600` for cache files. | ✅ Fixed |
| **P1** | Path Traversal (G304) | `internal/ssh/auth.go` | Added `filepath.Clean()` to key path reading. | ✅ Fixed |
| **P1** | Path Traversal (G304) | `internal/remediation/audit.go` | Added `filepath.Clean()` to log path reading. | ✅ Fixed |
| **P1** | Build Security | `Makefile` / `Dockerfile` | Added `-trimpath`, `-s -w`, created distroless Dockerfile. | ✅ Fixed |
| **P1** | CI/CD Security | `.github/workflows/ci.yml` | Added `gosec` scanner and SBOM generation. | ✅ Fixed |

## 2. Pending / Long-Term Items (P2/P3)

| Severity | Issue | Recommendation | Owner |
|----------|-------|----------------|-------|
| **P2** | Test Coverage | Increase coverage for `cmd/lumo` and `internal/ssh` to >80%. | Dev Team |
| **P2** | Secret Mgmt | Integrate HashiCorp Vault or AWS Secrets Manager directly. | Dev Team |
| **P3** | Fuzzing | Add fuzz tests for `internal/api` input parsing. | QA/Sec |
| **P3** | Documentation | Add architectural decision records (ADRs) for security choices. | Tech Lead |

## 3. Deployment Checklist

- [x] Build container with new `Dockerfile`.
- [x] Verify `lumo --version` shows correct build info.
- [x] Run `lumo diagnose` locally to ensure no regression.
- [ ] Push image to registry.
- [ ] Update Kubernetes manifests to use new image tag.
- [ ] Verify metrics in Prometheus/Grafana.

## 4. Rollback Plan

If regressions are found:
1. Revert to previous docker image tag.
2. Revert `go.mod`/`go.sum` changes if dependency issues arise (unlikely).
3. `git revert` the PR merging these changes.
