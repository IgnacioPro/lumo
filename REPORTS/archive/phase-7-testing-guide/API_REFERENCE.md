# Lumo API Documentation

This directory contains the OpenAPI 3.0 specification and documentation for the Lumo API.

## API Specification

The complete API specification is available in `openapi.yaml`. This specification documents all available endpoints, request/response schemas, authentication requirements, and error responses.

## Viewing the API Documentation

You can view the API documentation in several ways:

### 1. Online Swagger Editor

Visit [Swagger Editor](https://editor.swagger.io/) and paste the contents of `openapi.yaml`.

### 2. Swagger UI (Local)

```bash
# Using Docker
docker run -p 8081:8080 -e SWAGGER_JSON=/api/openapi.yaml \
  -v $(pwd)/api:/api swaggerapi/swagger-ui

# Then visit: http://localhost:8081
```

### 3. Redoc (Local)

```bash
# Using npx
npx @redocly/cli preview-docs api/openapi.yaml

# Or using Docker
docker run -p 8081:80 -e SPEC_URL=openapi.yaml \
  -v $(pwd)/api:/usr/share/nginx/html/api redocly/redoc
```

## Quick Start

### 1. Start the API Server

```bash
# Start development environment
docker-compose up -d

# Run API server
go run cmd/lumo/main.go serve
```

### 2. Create an API Key

API keys are stored in the database. You'll need to create one manually or via a setup script:

```sql
-- Connect to PostgreSQL
psql -U lumo -d lumo

-- Create API key (example)
INSERT INTO api_keys (key_hash, name, scopes, created_at)
VALUES (
  encode(digest('your-secret-key', 'sha256'), 'hex'),
  'dev-key',
  ARRAY['diagnostics:read', 'diagnostics:write', 'agents:read', 'agents:write'],
  NOW()
);
```

**Note**: The API key you use in requests is `your-secret-key`, not the hash.

### 3. Make API Requests

```bash
# Health check (no auth required)
curl http://localhost:8080/api/v1/health

# Register an agent
curl -X POST http://localhost:8080/api/v1/agents/register \
  -H "X-API-Key: your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "agent-01",
    "hostname": "node-01",
    "platform": "linux",
    "architecture": "amd64",
    "version": "1.0.0",
    "capabilities": ["cpu", "memory", "disk"]
  }'

# List agents
curl http://localhost:8080/api/v1/agents \
  -H "X-API-Key: your-secret-key"

# Run diagnostics
curl -X POST http://localhost:8080/api/v1/diagnostics \
  -H "X-API-Key: your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{
    "target": "localhost",
    "checks": ["cpu", "memory", "disk"]
  }'

# Get job status
curl http://localhost:8080/api/v1/jobs/{job-id} \
  -H "X-API-Key: your-secret-key"
```

## API Endpoints

### Health
- `GET /api/v1/health` - Health check (no auth)
- `GET /api/v1/ready` - Readiness check (no auth)
- `GET /api/v1/live` - Liveness check (no auth)

### Agents
- `POST /api/v1/agents/register` - Register agent
- `PUT /api/v1/agents/{id}/heartbeat` - Record heartbeat
- `GET /api/v1/agents` - List agents
- `GET /api/v1/agents/stats` - Get statistics
- `GET /api/v1/agents/{id}` - Get agent details
- `DELETE /api/v1/agents/{id}` - Deregister agent

### Diagnostics
- `POST /api/v1/diagnostics` - Run diagnostics

### Jobs
- `GET /api/v1/jobs` - List jobs
- `GET /api/v1/jobs/{id}` - Get job details
- `DELETE /api/v1/jobs/{id}` - Delete job

## Authentication

All authenticated endpoints require an API key passed via one of:
- Header: `X-API-Key: <your-api-key>`
- Header: `Authorization: Bearer <your-api-key>`

API keys support scope-based permissions:
- `diagnostics:read` - Read diagnostic results
- `diagnostics:write` - Run diagnostics
- `agents:read` - List and view agents
- `agents:write` - Register and manage agents
- `jobs:read` - View jobs
- `jobs:write` - Create and delete jobs

## Rate Limiting

API requests are rate-limited per API key:
- **Rate**: 60 requests per minute
- **Burst**: 100 requests

Rate limit headers are included in responses:
- `X-RateLimit-Limit` - Maximum requests per minute
- `X-RateLimit-Remaining` - Remaining requests in current window
- `X-RateLimit-Reset` - Time when rate limit resets (Unix timestamp)

## Error Responses

All error responses follow this format:

```json
{
  "success": false,
  "error": {
    "code": "error_code",
    "message": "Human-readable error message"
  }
}
```

Common error codes:
- `invalid_request` - Malformed request
- `authentication_required` - Missing API key
- `invalid_api_key` - Invalid or expired API key
- `not_found` - Resource not found
- `rate_limit_exceeded` - Too many requests
- `internal_error` - Server error

## Versioning

The API uses URL-based versioning (`/api/v1/`). Breaking changes will result in a new API version (e.g., `/api/v2/`).

## Support

For issues, questions, or feature requests:
- GitHub Issues: https://github.com/ignacio/lumo/issues
- Documentation: https://github.com/ignacio/lumo/blob/main/CLAUDE.md

## Development

To regenerate API clients from the OpenAPI spec:

```bash
# Generate Go client
openapi-generator-cli generate -i api/openapi.yaml \
  -g go -o clients/go

# Generate Python client
openapi-generator-cli generate -i api/openapi.yaml \
  -g python -o clients/python

# Generate TypeScript client
openapi-generator-cli generate -i api/openapi.yaml \
  -g typescript-axios -o clients/typescript
```

## License

MIT License - See LICENSE file for details
