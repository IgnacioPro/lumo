# Developer Checklist: Production Readiness

**Goal:** Maintain high security and quality standards for future Lumo development.

## 1. Code Quality & Security
- [ ] **Run Security Scan:** `gosec ./...` must pass (0 issues) before merge.
- [ ] **Run Linters:** `golangci-lint run ./...` must pass.
- [ ] **No Data Races:** `go test -race ./...` must pass.
- [ ] **Input Validation:**
  - [ ] All user inputs (API, CLI flags) must be validated.
  - [ ] File paths must be cleaned via `filepath.Clean`.
  - [ ] SQL queries must use placeholders (`$1`) or strict allowlisting.
- [ ] **Safe Math:** Check for integer overflows in retry/backoff logic.

## 2. Testing
- [ ] **Unit Tests:** New packages must have >80% coverage.
- [ ] **Integration Tests:** Add tests for new API endpoints in `internal/api`.
- [ ] **Mocks:** Use `sqlmock` or interfaces for external dependencies.

## 3. Build & Deployment
- [ ] **Dockerfile:** Ensure no new root privileges are added.
- [ ] **Dependencies:** Run `go mod tidy` and `go mod verify`.
- [ ] **Secrets:** NEVER commit secrets to git. Use env vars or secret managers.

## 4. Observability
- [ ] **Logs:** Use structured logging (`logrus.WithFields`).
- [ ] **Metrics:** Add Prometheus counters for new features/errors.
- [ ] **Tracing:** Ensure context is propagated for distributed tracing (future).

## 5. Reproducing Checks Locally
```bash
# Run all tests
make test

# Run race detection
go test -race ./...

# Run security scan (requires gosec installed)
gosec -no-fail ./...

# Build production binary
make build

# Build container
docker build -t lumo:latest .
```
