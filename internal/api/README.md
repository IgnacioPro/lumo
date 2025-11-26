# API Package

The `api` package provides the REST API server for Lumo.

## Overview

Built on [Chi router](https://github.com/go-chi/chi), the API server provides:

- Agent management (registration, heartbeats)
- Job lifecycle management
- Diagnostic result submission
- Event processing (for event-driven mode)
- Health and metrics endpoints

## Starting the Server

```go
import (
    "github.com/ignacio/lumo/internal/api"
    "github.com/ignacio/lumo/internal/config"
    "github.com/ignacio/lumo/internal/database"
)

cfg, _ := config.Load()
db, _ := database.NewDB(cfg)
server, _ := api.NewServer(cfg, db, logger)

server.Start() // Blocks until shutdown signal
```

Or via CLI:

```bash
lumo serve --config config.yaml
```

## API Endpoints

### Health & Metrics

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Liveness probe |
| `/ready` | GET | Readiness probe (checks DB) |
| `/metrics` | GET | Prometheus metrics |

### Agents (`/api/v1/agents`)

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/register` | POST | - | Register new agent |
| `/heartbeat` | POST | JWT | Agent heartbeat |
| `/{id}` | GET | JWT | Get agent details |

### Jobs (`/api/v1/jobs`)

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/` | GET | JWT | List jobs |
| `/` | POST | JWT | Create job |
| `/{id}` | GET | JWT | Get job details |
| `/{id}/results` | POST | JWT | Submit results |

### Events (`/api/v1/events`)

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/` | POST | JWT | Submit K8s event |
| `/` | GET | JWT | List events |

## Authentication

JWT-based authentication with configurable secret:

```bash
export LUMO_API_JWT_SECRET=your-secret-key
```

Tokens are obtained via agent registration and included in requests:

```
Authorization: Bearer <token>
```

## Middleware

- **Rate Limiting** - Per-IP (60/min) and per-user (3600/hour)
- **CORS** - Configurable allowed origins
- **Request ID** - Unique ID for tracing
- **Logging** - Structured request logging

## Directory Structure

```
api/
├── server.go           # Server initialization
├── routes.go           # Route definitions
├── auth/               # JWT authentication
├── handlers/           # Request handlers
├── middleware/         # HTTP middleware
└── response/           # Response helpers
```

## Configuration

```yaml
api:
  address: ":8080"
  jwt_secret: ""  # Use LUMO_API_JWT_SECRET env var
  allowed_origins:
    - "http://localhost:3000"
  rate_limit:
    requests_per_minute: 60
    requests_per_hour: 3600
```
