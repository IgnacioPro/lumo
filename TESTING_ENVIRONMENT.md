# Lumo Testing Environment Guide

This guide details how to set up a complete testing environment for Lumo to verify all features, including the CLI, API Server, and Agent.

## 1. Prerequisites

Ensure you have the following installed:
- **Go 1.25+**: For building and running tests.
- **Docker & Docker Compose**: For running PostgreSQL and Redis dependencies.
- **jq**: Recommended for formatting JSON output in API tests.

## 2. Unit Testing

Run the standard Go test suite to verify core logic (diagnostics, config, etc.).

```bash
# Run all unit tests
go test ./...

# Run with race detector (recommended)
go test -race ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 3. API & Agent Integration Testing (Phase 7+)

The API server requires PostgreSQL and Redis.

### Step 3.1: Start Infrastructure

```bash
# Start DB and Cache
docker-compose up -d

# Verify services are running
docker-compose ps
```

### Step 3.2: Start API Server

Open a new terminal window to run the server. It will automatically run migrations on startup.

```bash
# Build and run the server
go run ./cmd/lumo serve
```
*Server listens on `http://localhost:8080`*

### Step 3.3: Create Test API Key

You need an API key to authenticate requests. Run this command to insert a key into the running Postgres container:

```bash
docker exec -i lumo-postgres psql -U lumo -d lumo <<EOF
INSERT INTO api_keys (id, name, key_hash, scopes, created_at, updated_at)
VALUES (
  gen_random_uuid(),
  'example-key',
  '5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8',
  ARRAY['diagnostics:run', 'jobs:read', 'jobs:write', 'agents:read', 'agents:write']::varchar[],
  NOW(),
  NOW()
);
EOF
```
*The API Key corresponding to this hash is: `example-key`*

### Step 3.4: Run Automated API Tests

Lumo includes a script to test the full Agent Registration flow.

```bash
# Make the script executable (if needed)
chmod +x REPORTS/phase-7-testing-guide/test-agent-api.sh

# Run the test script
./REPORTS/phase-7-testing-guide/test-agent-api.sh
```

**What this tests:**
- Health endpoints
- Agent registration (Linux & Kubernetes modes)
- Heartbeats
- Agent listing & filtering
- Agent deletion

## 4. CLI Manual Testing

Test the CLI diagnostics locally.

```bash
# Build the CLI
go build -o lumo ./cmd/lumo

# Run local diagnostics
./lumo diagnose localhost

# Run specific checks
./lumo diagnose localhost --checks cpu,memory,disk

# Test AI Analysis (requires API Key)
# export LUMO_ANTHROPIC_API_KEY=sk-...
# ./lumo diagnose localhost --analyze
```

## 5. Troubleshooting

- **Database Connection Failed**: Ensure `docker-compose up` is running and ports 5432/6379 are free.
- **Unauthorized**: Check that you are using `X-API-Key: test-key-123` in your requests (the test script handles this).
- **Migrations**: If the server fails to start due to DB errors, try resetting the volume: `docker-compose down -v && docker-compose up -d`.
