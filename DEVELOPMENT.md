# Development Guide

This guide provides instructions for developing, building, and testing Lumo.

## Prerequisites

- **Go**: 1.25 or higher
- **Docker**: For containerized builds and local environment
- **Make**: For running build commands

## Getting Started

1. **Clone the repository**
   ```bash
   git clone https://github.com/IgnacioPro/lumo.git
   cd lumo
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Run local environment**
   Lumo requires PostgreSQL and Redis for full functionality.
   ```bash
   docker-compose up -d
   ```

## Building

Build the CLI and Agent binaries:

```bash
make build
```

Binaries will be placed in the current directory:
- `lumo`: Main CLI and API server
- `lumo-agent`: Agent daemon

## Running

### API Server

Start the API server (requires DB/Redis):

```bash
./lumo serve
```

Configuration can be customized via `~/.lumo/config.yaml` or environment variables (see README.md).

### Agent

Start the agent:

```bash
./lumo-agent start
```

## Testing

### Unit & Integration Tests

Run the standard test suite:

```bash
make test
```

This runs all tests, including:
- Unit tests for core logic
- Integration tests for API endpoints (using in-memory/mock DB where possible)
- **Load Tests**: Basic load tests are included in the suite (`tests/load`)

### Load Testing

Load tests are located in `tests/load`. They verify system stability and performance.
They are integrated into `make test` but can be run individually:

```bash
go test -v ./tests/load/...
```

**Note**: Load tests require a running database (Postgres). The test harness automatically connects to the local Postgres instance defined in `config.yaml` (default: localhost:5432, user: lumo, pass: lumo_dev).

## Code Quality

Run linting and formatting:

```bash
make lint
make fmt
```

## Project Structure

- `cmd/`: Entry points for binaries
- `internal/`: Private application code
    - `api/`: HTTP API server and handlers
    - `ai/`: AI provider integrations
    - `diagnostics/`: System diagnostic logic
    - `reliability/`: Circuit breakers and resilience patterns
    - `observability/`: Tracing and metrics
- `pkg/`: Public libraries (if any)
- `tests/`: Integration and load tests
