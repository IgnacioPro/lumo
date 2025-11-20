# gRPC Testing Guide

This document describes the testing strategy and available tests for Lumo's gRPC implementation.

## Test Structure

```
internal/grpc/
├── client/
│   └── client_test.go         # Unit tests for gRPC client
├── handlers/
│   └── handlers_test.go       # Unit tests for service handlers
└── integration_test.go        # Integration tests (client + server)
```

## Test Coverage

### 1. Client Unit Tests (`client/client_test.go`)

Tests for the gRPC client library with in-memory (bufconn) connections.

**Test Cases:**
- `TestNewClient` - Client creation with various options
- `TestClient_Close` - Connection cleanup
- `TestClient_HealthCheck` - Health check RPC
- `TestClient_Live` - Liveness probe RPC
- `TestClient_Ready` - Readiness probe RPC
- `TestClient_Ping` - Connectivity check
- `TestClient_RegisterAgent` - Agent registration (valid/invalid)
- `TestClient_SendHeartbeat` - Heartbeat sending (valid/invalid)
- `TestClient_GetAgentStats` - Agent statistics retrieval
- `TestClient_RunDiagnostics` - Diagnostic job creation (valid/invalid)
- `TestClient_GetDiagnosticsResult` - Diagnostic result retrieval
- `TestClient_WithAuth` - JWT authentication injection
- `TestClient_GetConnectionState` - Connection state monitoring

**Coverage:**
- ✅ All service methods (Health, Agents, Diagnostics)
- ✅ Error handling and validation
- ✅ Context management
- ✅ Connection state tracking
- ✅ Authentication metadata injection

### 2. Handler Unit Tests (`handlers/handlers_test.go`)

Tests for gRPC service handlers with mock repositories.

**Health Service Tests:**
- `TestHealthHandler_Check` - Comprehensive health check
- `TestHealthHandler_Ready` - Readiness probe
- `TestHealthHandler_Live` - Liveness probe

**Agents Service Tests:**
- `TestAgentsHandler_RegisterAgent` - Agent registration with validation
- `TestAgentsHandler_SendHeartbeat` - Heartbeat with valid/invalid IDs
- `TestAgentsHandler_ListAgents` - Agent listing with pagination
- `TestAgentsHandler_GetAgent` - Get agent by ID
- `TestAgentsHandler_DeleteAgent` - Agent deletion
- `TestAgentsHandler_GetAgentStats` - Aggregate statistics

**Diagnostics Service Tests:**
- `TestDiagnosticsHandler_RunDiagnostics` - Job creation with validation
- `TestDiagnosticsHandler_GetDiagnosticsResult` - Result retrieval
- `TestDiagnosticsHandler_ListDiagnostics` - Job listing with pagination

**Helper Function Tests:**
- `TestToProtoJobStatus` - Model to proto conversion
- `TestToProtoJobType` - Job type conversion
- `TestToProtoAgent` - Agent model conversion

**Coverage:**
- ✅ Request validation (missing fields, invalid formats)
- ✅ Error codes (InvalidArgument, NotFound, Internal)
- ✅ Repository integration
- ✅ Response formatting
- ✅ Timestamp handling

### 3. Integration Tests (`integration_test.go`)

End-to-end tests with full client-server communication using bufconn.

**Test Cases:**
- `TestIntegration_HealthCheck` - Full health check flow (Live, Ready, Check)
- `TestIntegration_ConcurrentRequests` - 50 concurrent requests
- `TestIntegration_Timeouts` - Context timeouts and cancellation
- `TestIntegration_ErrorHandling` - Error propagation
- `TestIntegration_ConnectionManagement` - Multiple connections, connection reuse

**Benchmarks:**
- `BenchmarkGRPCHealthCheck` - Single-threaded performance
- `BenchmarkGRPCConcurrentHealthCheck` - Concurrent request performance

**Coverage:**
- ✅ Full RPC flow (client → server → response)
- ✅ Concurrent request handling
- ✅ Context cancellation and timeouts
- ✅ Connection pooling
- ✅ Performance baselines

## Running Tests

### Run All Tests

```bash
# Run all gRPC tests
go test ./internal/grpc/...

# With verbose output
go test -v ./internal/grpc/...

# With coverage
go test -cover ./internal/grpc/...

# Generate coverage report
go test -coverprofile=coverage.out ./internal/grpc/...
go tool cover -html=coverage.out -o coverage.html
```

### Run Specific Test Suites

```bash
# Client tests only
go test ./internal/grpc/client

# Handler tests only
go test ./internal/grpc/handlers

# Integration tests only
go test ./internal/grpc -run Integration
```

### Run Tests with Race Detection

```bash
# Detect data races
go test -race ./internal/grpc/...
```

### Run Short Tests (Skip Integration)

```bash
# Skip long-running integration tests
go test -short ./internal/grpc/...
```

### Run Benchmarks

```bash
# Run all benchmarks
go test -bench=. ./internal/grpc/...

# Run specific benchmark
go test -bench=BenchmarkGRPCHealthCheck ./internal/grpc

# With memory allocation stats
go test -bench=. -benchmem ./internal/grpc/...

# Run for longer duration
go test -bench=. -benchtime=10s ./internal/grpc/...
```

## Test Utilities

### bufconn (In-Memory Connections)

All tests use `google.golang.org/grpc/test/bufconn` for in-memory gRPC connections. This provides:
- Fast test execution (no network I/O)
- Isolation (no port conflicts)
- Reliability (no network flakiness)

### Mock Repositories

Tests use in-memory mock repositories instead of real databases:
- `mockJobRepository` - Job CRUD operations
- `mockAgentRepository` - Agent CRUD operations

These mocks implement the same interfaces as real repositories but store data in memory maps.

## Writing New Tests

### Example: Adding a New Handler Test

```go
func TestNewHandler_NewMethod(t *testing.T) {
    // Create mock repository
    repo := newMockRepository()
    handler := NewHandler(repo)

    // Setup test cases
    tests := []struct {
        name    string
        req     *lumov1.Request
        wantErr bool
        errCode codes.Code
    }{
        {
            name: "valid request",
            req:  &lumov1.Request{Field: "value"},
            wantErr: false,
        },
        {
            name: "invalid request",
            req:  &lumov1.Request{},
            wantErr: true,
            errCode: codes.InvalidArgument,
        },
    }

    // Run test cases
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := context.Background()
            resp, err := handler.NewMethod(ctx, tt.req)

            if tt.wantErr {
                assert.Error(t, err)
                st, ok := status.FromError(err)
                require.True(t, ok)
                assert.Equal(t, tt.errCode, st.Code())
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, resp)
            }
        })
    }
}
```

### Example: Adding an Integration Test

```go
func TestIntegration_NewFeature(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Setup server
    lis := bufconn.Listen(bufSize)
    defer lis.Close()

    srv := grpc.NewServer()
    defer srv.Stop()

    // Register services
    handler := handlers.NewHandler(mockRepo)
    lumov1.RegisterServiceServer(srv, handler)

    go func() {
        if err := srv.Serve(lis); err != nil {
            t.Logf("Server error: %v", err)
        }
    }()

    // Create client
    ctx := context.Background()
    conn, err := grpc.DialContext(ctx, "bufnet",
        grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
            return lis.Dial()
        }),
        grpc.WithInsecure(),
    )
    require.NoError(t, err)
    defer conn.Close()

    client := lumov1.NewServiceClient(conn)

    // Test your feature
    resp, err := client.Method(ctx, &lumov1.Request{})
    require.NoError(t, err)
    assert.NotNil(t, resp)
}
```

## CI/CD Integration

These tests are automatically run in CI pipelines:

```yaml
# .github/workflows/test.yml
- name: Run gRPC tests
  run: go test -v -race -cover ./internal/grpc/...

- name: Run benchmarks
  run: go test -bench=. -benchtime=5s ./internal/grpc/...
```

## Test Coverage Goals

- **Target:** 80%+ code coverage
- **Current Status:** Run `go test -cover ./internal/grpc/...` to check

Coverage breakdown:
- Client library: >90% (high priority)
- Handlers: >85% (business logic)
- Integration: >70% (happy paths + errors)

## Known Limitations

1. **No TLS/mTLS Tests:** Current tests use insecure connections. TLS/mTLS testing requires certificate setup and is covered in manual testing.

2. **No Streaming Tests:** The `StreamDiagnostics` RPC is not fully tested. Future work: Add streaming tests with mock data.

3. **No Load Tests:** Benchmarks test single-machine performance. For load testing, use separate tools (e.g., ghz).

## Troubleshooting

### Tests Hang

**Issue:** Tests hang indefinitely

**Solution:** Check for:
- Missing `defer server.Stop()` or `defer conn.Close()`
- Blocked channels without timeout
- Context without deadline

### Flaky Tests

**Issue:** Tests pass/fail randomly

**Solution:**
- Use `t.Parallel()` cautiously with shared resources
- Add appropriate timeouts (`context.WithTimeout`)
- Check for race conditions (`go test -race`)

### Import Errors

**Issue:** `cannot find package`

**Solution:**
```bash
# Ensure proto files are generated
make proto-gen

# Download dependencies
go mod download

# Tidy dependencies
go mod tidy
```

## Performance Baseline

Run benchmarks to establish performance baselines:

```bash
go test -bench=. -benchmem ./internal/grpc/... > bench.txt
```

**Expected Results (bufconn):**
- Health Check (single): ~5-10 µs/op
- Health Check (concurrent): ~50-100 µs/op (with contention)

Real network performance will be 10-100x slower depending on latency.

## Future Testing Work

1. **TLS/mTLS Integration Tests** - Test certificate validation and mutual auth
2. **Streaming RPC Tests** - Test `StreamDiagnostics` with event handlers
3. **Interceptor Tests** - Test auth, logging, and recovery interceptors in isolation
4. **Load Tests** - Use [ghz](https://ghz.sh/) for realistic load testing
5. **Chaos Tests** - Test behavior under network failures, timeouts, etc.

## Resources

- [gRPC Go Testing](https://github.com/grpc/grpc-go/blob/master/Documentation/testing.md)
- [bufconn Documentation](https://godoc.org/google.golang.org/grpc/test/bufconn)
- [testify Documentation](https://github.com/stretchr/testify)
- [Go Testing Best Practices](https://golang.org/doc/tutorial/add-a-test)
