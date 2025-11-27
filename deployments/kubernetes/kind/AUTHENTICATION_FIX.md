# Authentication Fix for Event-Driven Agents

## Problem

Agents were unable to submit events to the API server, receiving **401 Unauthorized** errors:
- Agents had valid bearer tokens but no corresponding API keys in database
- Events handler required `agent_id` in context, but API key auth didn't provide it
- Events table had foreign key constraint on `agent_id` requiring valid agent

## Root Cause

1. **Missing API Keys**: No API keys were created during deployment
2. **Agent ID Requirement**: Events handler expected `agent_id` from agent registration
3. **Foreign Key Constraint**: Events table required valid agent reference

## Solution

### 1. Bootstrap API Key Creation (`deploy-lumo.sh`)

Added `bootstrap_api_key()` function that:
- Creates API key from agent token using SHA-256 hash
- Grants full agent permissions: `agents:read`, `agents:write`, `events:write`, `jobs:read`, `diagnostics:write`
- Creates system agent (`uuid.Nil`) for API key authenticated events

### 2. Events Handler Update (`internal/api/handlers/events.go`)

Modified `SubmitEvents()` to:
- Make `agent_id` optional (defaults to `uuid.Nil` if not present)
- Log authentication method (agent vs API key)
- Accept events from both registered agents and API key auth

### 3. System Agent Creation

Created special system agent with:
- ID: `00000000-0000-0000-0000-000000000000` (uuid.Nil)
- Name: `system-api-key`
- Platform: `kubernetes`
- Status: `online`
- Labels: `{"type": "system", "auth": "api-key"}`

## Testing

```bash
cd deployments/kubernetes/kind
./deploy-lumo.sh
```

## Verification

✅ **All Tests Passing:**
- API key authentication working (no more 401 errors)
- Events successfully submitted to API server
- Events stored in PostgreSQL database
- HTTP 201 responses for event submissions
- 5 events stored with proper metadata

## Files Changed

1. `deployments/kubernetes/kind/deploy-lumo.sh` - Added `bootstrap_api_key()` function
2. `internal/api/handlers/events.go` - Made `agent_id` optional in SubmitEvents

## Deployment Flow

1. Deploy PostgreSQL + Redis
2. **Bootstrap API key + system agent** ← New step
3. Deploy API server
4. Deploy event-driven agents
5. Agents authenticate with API key
6. Events submitted successfully

## Database Schema

```sql
-- API Keys
CREATE TABLE api_keys (
    id uuid PRIMARY KEY,
    key_hash varchar UNIQUE,
    name varchar,
    scopes text[],
    ...
);

-- Agents (including system agent)
CREATE TABLE agents (
    id uuid PRIMARY KEY,
    name varchar,
    platform varchar CHECK (platform IN ('linux', 'darwin', 'windows', 'kubernetes')),
    status varchar CHECK (status IN ('online', 'offline', 'error')),
    ...
);

-- Events (references agent)
CREATE TABLE events (
    id uuid PRIMARY KEY,
    agent_id uuid REFERENCES agents(id),
    event_type varchar,
    ...
);
```

## Security Notes

- API keys hashed with SHA-256 before storage
- Token transmitted via `Authorization: Bearer` header
- API key provides scoped permissions
- System agent has metadata indicating auth method

## Next Steps

- ✅ Authentication working
- ⏳ Enable AI analysis for events
- ⏳ Configure multi-channel notifications
- ⏳ Add event filtering by severity/namespace
