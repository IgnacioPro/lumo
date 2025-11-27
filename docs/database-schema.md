# Database Schema

This document describes the PostgreSQL database schema used by Lumo.

## Overview

Lumo uses PostgreSQL for persistent storage of:
- Agent registrations and heartbeats
- Job records and status
- Kubernetes events
- Approval workflows
- API keys

## Tables

### agents

Stores registered Lumo agents and their status.

```sql
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    hostname VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    mode VARCHAR(50) NOT NULL,           -- scheduled, on-demand, continuous, hybrid, event-driven
    enabled_checks TEXT[],               -- Array of enabled diagnostic checks
    labels JSONB DEFAULT '{}'::jsonb,    -- Key-value labels for filtering
    status VARCHAR(50) DEFAULT 'unknown', -- online, offline, unknown
    last_heartbeat TIMESTAMP,
    registered_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_last_heartbeat ON agents(last_heartbeat);
CREATE INDEX idx_agents_mode ON agents(mode);
CREATE INDEX idx_agents_labels ON agents USING GIN (labels);
```

**Key Fields:**
- `mode`: Operating mode (scheduled, on-demand, continuous, hybrid, event-driven)
- `enabled_checks`: List of diagnostic checks the agent runs
- `status`: Current agent status based on heartbeat recency
- `labels`: Custom labels for organizing and filtering agents

---

### jobs

Stores diagnostic job records and results.

```sql
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID REFERENCES agents(id),
    target VARCHAR(255) NOT NULL,        -- Hostname or IP
    checks TEXT[],                       -- List of checks executed
    status VARCHAR(50) NOT NULL,         -- pending, running, completed, failed
    results JSONB,                       -- Diagnostic results
    ai_analysis TEXT,                    -- AI-generated analysis
    error TEXT,                          -- Error message if failed
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_jobs_agent_id ON jobs(agent_id);
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_created_at ON jobs(created_at DESC);
CREATE INDEX idx_jobs_target ON jobs(target);
```

**Status Values:**
- `pending`: Job queued for execution
- `running`: Job currently executing
- `completed`: Job finished successfully
- `failed`: Job encountered an error

---

### events

Stores Kubernetes events reported by agents.

```sql
CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,    -- oom-killed, crash-loop-backoff, etc.
    severity VARCHAR(20) NOT NULL,       -- low, medium, high, critical
    resource_kind VARCHAR(50) NOT NULL,  -- Pod, Deployment, Node, etc.
    resource_name VARCHAR(255) NOT NULL,
    resource_uid VARCHAR(255),           -- Kubernetes UID
    namespace VARCHAR(255),              -- Nullable for cluster-scoped resources
    message TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    event_timestamp TIMESTAMP NOT NULL,
    ai_analysis TEXT,
    ai_analyzed_at TIMESTAMP,
    notification_sent BOOLEAN DEFAULT FALSE,
    notification_sent_at TIMESTAMP,
    notification_channels TEXT[],
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT events_severity_check 
        CHECK (severity IN ('low', 'medium', 'high', 'critical'))
);

-- Primary indexes
CREATE INDEX idx_events_agent_id ON events(agent_id);
CREATE INDEX idx_events_event_type ON events(event_type);
CREATE INDEX idx_events_severity ON events(severity);
CREATE INDEX idx_events_resource_kind ON events(resource_kind);
CREATE INDEX idx_events_namespace ON events(namespace);
CREATE INDEX idx_events_resource_uid ON events(resource_uid);
CREATE INDEX idx_events_event_timestamp ON events(event_timestamp DESC);
CREATE INDEX idx_events_created_at ON events(created_at DESC);

-- Composite indexes for common query patterns
CREATE INDEX idx_events_agent_timestamp ON events(agent_id, event_timestamp DESC);
CREATE INDEX idx_events_severity_timestamp ON events(severity, event_timestamp DESC) 
    WHERE severity IN ('high', 'critical');
CREATE INDEX idx_events_namespace_timestamp ON events(namespace, event_timestamp DESC);

-- Partial index for pending notifications
CREATE INDEX idx_events_notification_sent ON events(notification_sent) 
    WHERE notification_sent = FALSE;

-- GIN index for JSONB metadata queries
CREATE INDEX idx_events_metadata ON events USING GIN (metadata);
```

**Severity Levels:**
- `low`: Informational, no immediate action required
- `medium`: Should be reviewed soon
- `high`: Requires prompt attention
- `critical`: Immediate action required

See [event-types.md](event-types.md) for detailed event type descriptions.

---

### approvals

Stores remediation approval requests and decisions.

```sql
CREATE TABLE approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID REFERENCES jobs(id),
    action_type VARCHAR(100) NOT NULL,   -- disk_cleanup, service_restart, etc.
    target VARCHAR(255) NOT NULL,
    description TEXT,
    risk_level VARCHAR(50) NOT NULL,     -- low, medium, high, critical
    status VARCHAR(50) DEFAULT 'pending', -- pending, approved, rejected, expired
    requested_by VARCHAR(255),
    approved_by VARCHAR(255),
    approved_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_approvals_job_id ON approvals(job_id);
CREATE INDEX idx_approvals_status ON approvals(status);
CREATE INDEX idx_approvals_expires_at ON approvals(expires_at);
```

---

### api_keys

Stores API keys for authentication.

```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,      -- SHA-256 hash of API key
    scopes TEXT[] DEFAULT '{}',          -- read, write, admin
    description TEXT,
    expires_at TIMESTAMP,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_expires_at ON api_keys(expires_at);
```

---

## Migrations

Migrations are managed using [goose](https://github.com/pressly/goose).

**Location:** `internal/database/migrations/`

**Migration Files:**
1. `001_init_schema.sql` - Base schema (agents, jobs)
2. `002_api_keys.sql` - API key authentication
3. `003_agents.sql` - Agent enhancements
4. `004_approvals.sql` - Approval workflow
5. `005_events.sql` - Kubernetes events

**Running Migrations:**

```bash
# Apply all pending migrations
goose -dir internal/database/migrations postgres "$DATABASE_URL" up

# Rollback last migration
goose -dir internal/database/migrations postgres "$DATABASE_URL" down

# Check migration status
goose -dir internal/database/migrations postgres "$DATABASE_URL" status
```

## Connection Configuration

Configure database connection in `config.yaml`:

```yaml
database:
  host: localhost
  port: 5432
  name: lumo
  user: lumo
  password: ${LUMO_DATABASE_PASSWORD}  # Use env var
  sslmode: require  # disable, require, verify-full
  max_connections: 25
  max_idle: 5
  conn_max_lifetime: 1h
```

**Environment Variables:**
```bash
export LUMO_DATABASE_HOST=localhost
export LUMO_DATABASE_PORT=5432
export LUMO_DATABASE_NAME=lumo
export LUMO_DATABASE_USER=lumo
export LUMO_DATABASE_PASSWORD=secret
export LUMO_DATABASE_SSLMODE=require
```

## Performance Tuning

See [database-tuning.md](database-tuning.md) for PostgreSQL optimization recommendations.

### Recommended Settings

```sql
-- Connection pooling (in config.yaml)
max_connections: 25
max_idle: 5
conn_max_lifetime: 1h

-- Query timeout
statement_timeout = '30s'

-- Work memory for complex queries
work_mem = '64MB'
```

### Index Maintenance

```sql
-- Analyze tables regularly
ANALYZE events;
ANALYZE agents;
ANALYZE jobs;

-- Reindex if needed
REINDEX TABLE events;
```

## Backup and Recovery

### Backup

```bash
# Full backup
pg_dump -Fc lumo > lumo_backup.dump

# Schema only
pg_dump -Fc --schema-only lumo > lumo_schema.dump

# Data only
pg_dump -Fc --data-only lumo > lumo_data.dump
```

### Restore

```bash
# Restore from backup
pg_restore -d lumo lumo_backup.dump

# Restore to new database
createdb lumo_restored
pg_restore -d lumo_restored lumo_backup.dump
```

## Security

1. **Credentials**: Store database password in environment variable `LUMO_DATABASE_PASSWORD`
2. **SSL**: Enable `sslmode: require` or `verify-full` for production
3. **Access**: Limit database user permissions to required operations
4. **Audit**: Enable PostgreSQL logging for audit trails

```sql
-- Create limited user for production
CREATE USER lumo_app WITH PASSWORD 'xxx';
GRANT CONNECT ON DATABASE lumo TO lumo_app;
GRANT USAGE ON SCHEMA public TO lumo_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO lumo_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO lumo_app;
```
