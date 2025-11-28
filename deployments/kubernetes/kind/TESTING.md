# Lumo Testing Guide

Comprehensive testing suite for load testing, chaos engineering, and profile validation.

## Test Scripts

### 1. Messaging Load Tests (`test-messaging-load.sh`)

Tests NATS messaging infrastructure under heavy load.

**Usage:**
```bash
# Test XS profile (100 events/sec target)
./test-messaging-load.sh --profile xs

# Test S profile (1K events/sec target)
./test-messaging-load.sh --profile s

# Test M profile (10K events/sec target)
./test-messaging-load.sh --profile m

# Custom duration
DURATION=120 ./test-messaging-load.sh --profile m
```

**What It Tests:**
- Message throughput (events/sec)
- Message latency (p50, p95, p99)
- Resource usage (CPU, memory)
- Connection stability under load

**Expected Results:**
- S profile: 1,000+ msg/sec sustained
- M profile: 10,000+ msg/sec sustained
- Latency: <10ms p99

---

### 2. Chaos Engineering (`test-failure-scenarios.sh`)

Tests event-driven system resilience against Kubernetes failures.

**Usage:**
```bash
# Run all scenarios
./test-failure-scenarios.sh

# Run specific scenario
./test-failure-scenarios.sh --scenario oom-killed

# List available scenarios
./test-failure-scenarios.sh --list
```

**Scenarios Tested:**
1. **Pod Failures:**
   - ImagePullBackOff
   - CrashLoopBackOff
   - OOMKilled
   - Pod Pending

2. **Workload Failures:**
   - Deployment Failed
   - Job Failed

3. **Volume Failures:**
   - Volume Failed Mount
   - PVC Provision Failed

4. **Scheduling Failures:**
   - Node Selector Mismatch
   - Insufficient CPU

**Expected Results:**
- Events detected within 60s
- Events stored in database
- AI analysis triggered (if enabled)

---

### 3. Integration Tests (`tests/integration/`)

End-to-end API workflow tests with real PostgreSQL.

**Usage:**
```bash
# Run integration tests (requires Docker)
go test -v ./tests/integration/...

# Skip integration tests
go test -short ./...
```

**What It Tests:**
- Health endpoints
- Agent registration lifecycle
- Jobs CRUD operations
- Authentication & JWT
- Events API
- Database operations

**Duration:** ~22 seconds

---

### 4. Load Tests (`tests/load/`)

API endpoint load testing with concurrent workers.

**Usage:**
```bash
# Run load tests (requires PostgreSQL)
go test -v ./tests/load/...

# Test specific scenario
go test -v -run TestLoadHealthCheck ./tests/load/...
go test -v -run TestRateLimiting ./tests/load/...
```

**What It Tests:**
- Health endpoint: 50 workers × 20 requests
- Rate limiting verification
- Database connection pooling
- Concurrent request handling

**Expected Results:**
- 1,000 requests with 0 errors
- Rate limiting triggers at configured limits

---

## Quick Testing Workflows

### Test S Profile (Small)

```bash
# Deploy
make deploy-s

# Test messaging load
./deployments/kubernetes/kind/test-messaging-load.sh --profile s

# Test chaos scenarios
./deployments/kubernetes/kind/test-failure-scenarios.sh

# Expected: 1K msg/sec, all scenarios pass
```

### Test M Profile (Medium)

```bash
# Deploy
make deploy-m

# Test messaging load (higher throughput)
./deployments/kubernetes/kind/test-messaging-load.sh --profile m

# Test under failure conditions
./deployments/kubernetes/kind/test-failure-scenarios.sh

# Expected: 10K msg/sec, all scenarios pass
```

### Full CI Test Suite

```bash
# Run everything locally
make ci

# Individual CI stages
make ci-lint    # Linters + security
make ci-test    # Tests with race detection
make ci-build   # Build verification
```

---

## Performance Benchmarks

### S Profile (NATS 64MB)

### XS Profile (Redis Streams)
- **Throughput:** 100+ msg/sec sustained
- **Latency:** <2ms p99
- **CPU:** ~50m under load
- **Memory:** ~20MB used
- **Agents:** 0-10
- **Throughput:** 1,000+ msg/sec sustained
- **Latency:** <5ms p99
- **CPU:** ~200m under load
- **Memory:** ~50MB used
- **Agents:** 10-50

### M Profile (NATS 256MB)
- **Throughput:** 10,000+ msg/sec sustained
- **Latency:** <10ms p99
- **CPU:** ~500m under load
- **Memory:** ~150MB used
- **Agents:** 50-500

---

## Debugging Failed Tests

### Messaging Load Test Fails

```bash
# Check NATS logs
kubectl logs -n lumo-system -l app=nats --tail=50

# Check NATS stats
kubectl exec -n lumo-system $(kubectl get pod -l app=nats -o name | head -1) -- \
  wget -qO- http://localhost:8222/varz | jq .

# Check resource usage
kubectl top pod -n lumo-system -l app=nats
```

### Chaos Test Fails

```bash
# Check agent logs
kubectl logs -n lumo-system -l app=lumo-agent --tail=50

# Check API logs
kubectl logs -n lumo-system -l app=lumo-api --tail=50

# Verify events in database
kubectl exec -n lumo-system postgres-0 -- \
  psql -U lumo -d lumo -c "SELECT * FROM events ORDER BY created_at DESC LIMIT 10;"
```

### Integration Test Fails

```bash
# Verify PostgreSQL is running
docker ps | grep postgres

# Check database connection
docker exec <postgres-container> psql -U lumo -d lumo -c "SELECT 1;"

# Re-run with verbose output
go test -v -run <test-name> ./tests/integration/...
```

---

## Continuous Testing

### Pre-Commit

```bash
make ci    # Runs linters, tests, builds
```

### GitHub CI

Automatically runs on push:
- Linters (golangci-lint)
- Security (govulncheck)
- Tests (with race detection)
- Builds (all platforms)

---

## Resources

- Load tests: `tests/load/load_test.go`
- Integration tests: `tests/integration/api_test.go`
- Chaos tests: `deployments/kubernetes/kind/test-failure-scenarios.sh`
- Messaging tests: `deployments/kubernetes/kind/test-messaging-load.sh`
