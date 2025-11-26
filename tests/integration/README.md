# Integration Tests

This directory contains end-to-end integration tests for the Lumo API server using real PostgreSQL databases via [testcontainers-go](https://github.com/testcontainers/testcontainers-go).

## Prerequisites

- **Docker**: Required for testcontainers to spin up PostgreSQL instances
- **Go 1.25+**: For running tests

## Running Tests

```bash
# Run all integration tests
go test -v ./tests/integration/...

# Run with count flag to avoid test caching
go test -v -count=1 ./tests/integration/...

# Skip integration tests (use short mode)
go test -short ./...
```

## Test Structure

### Test Environment (`testenv.go`)
- `NewTestEnv()`: Creates isolated test environment with PostgreSQL container
- `CreateRouter()`: Initializes API router with test database
- `CreateAPIKey()`: Helper to create test API keys
- `CreateTestAgent()`: Helper to create test agents
- `CreateTestJob()`: Helper to create test jobs
- `CleanupTables()`: Truncates all tables for test isolation

### Test Suites (`api_test.go`)

1. **TestEnvSetup**: Validates test environment creation
2. **TestHealthEndpoints**: Tests `/health`, `/ready`, `/live` endpoints
3. **TestAgentLifecycle**: Complete agent workflow (register, heartbeat, get, list, stats, delete)
4. **TestJobsLifecycle**: Jobs CRUD operations
5. **TestAuthenticationRequired**: Validates auth requirements on protected endpoints
6. **TestJWTAuthentication**: JWT token generation and validation
7. **TestEventSubmission**: Event listing API (event submission requires agent context)
8. **TestAgentReregistration**: Validates agent update on re-registration

## Architecture

```
Test → TestEnv → PostgreSQL Container → Database
                ↓
            API Router → Handlers → Repositories → Database
```

Each test:
1. Creates isolated PostgreSQL container (testcontainers)
2. Runs migrations
3. Executes test scenarios
4. Cleans up container

## Execution Time

- Full suite: ~22 seconds
- Per test: 2-3 seconds (includes container startup)

## Key Features

- **Real Database**: Uses actual PostgreSQL, not mocks
- **Isolation**: Each test gets fresh database
- **Automatic Cleanup**: Containers cleaned up after tests
- **Comprehensive Coverage**: Tests auth, CRUD, lifecycle workflows

## Testcontainers

Testcontainers automatically:
- Downloads postgres:15-alpine image (if not cached)
- Starts PostgreSQL container
- Waits for readiness
- Provides connection details
- Cleans up after test completion

## Troubleshooting

**Docker not running:**
```
Error: Cannot connect to the Docker daemon
```
→ Start Docker Desktop or Docker daemon

**Tests timeout:**
```
Test timed out after 2m
```
→ Increase timeout: `go test -timeout=5m`

**Port conflicts:**
Testcontainers uses dynamic ports, so conflicts are rare. If issues occur, ensure no stale containers: `docker ps -a`

## Future Enhancements

- Load testing with concurrent requests
- Chaos engineering (network failures, database unavailability)
- Event submission tests with agent context
- gRPC integration tests
