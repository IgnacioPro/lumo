# Database Connection Pool Tuning for Scale

This guide provides detailed instructions for tuning Lumo's PostgreSQL connection pool to support deployments ranging from small single-server setups to large-scale 1000+ agent deployments.

## Table of Contents

- [Overview](#overview)
- [Connection Pool Basics](#connection-pool-basics)
- [Sizing Guidelines](#sizing-guidelines)
- [PostgreSQL Server Configuration](#postgresql-server-configuration)
- [Lumo Configuration](#lumo-configuration)
- [Monitoring & Troubleshooting](#monitoring--troubleshooting)
- [Advanced: PgBouncer](#advanced-pgbouncer)

---

## Overview

**Problem:** Default database connection limits cannot scale to 1000+ concurrent agents.

**Solution:** Tune both Lumo's connection pool settings and PostgreSQL server configuration.

**Key Metrics:**
- **Small deployments (1-100 agents):** 25 connections sufficient
- **Medium deployments (100-500 agents):** 50 connections (default)
- **Large deployments (500-1000 agents):** 100 connections
- **Extra large (1000+ agents):** 200+ connections or PgBouncer

---

## Connection Pool Basics

### What is a Connection Pool?

A connection pool maintains a set of reusable database connections to avoid the overhead of creating new connections for each request.

**Key Parameters:**
- `max_connections`: Maximum number of open connections to PostgreSQL
- `max_idle`: Number of idle connections to keep warm (reduces latency)
- `conn_max_lifetime`: How long a connection can be reused before being closed

### How Many Connections Do I Need?

**Formula:**
```
max_connections = (concurrent_agents × requests_per_agent) / connection_sharing_factor
```

**Example (1000 agents):**
- 1000 agents × 2 concurrent requests/agent = 2000 concurrent requests
- 2000 / 10 (sharing factor) = 200 connections baseline
- Add 50% safety margin: 200 × 1.5 = **300 connections** recommended

**Lumo defaults:** 50 connections (good for 100-500 agents)

---

## Sizing Guidelines

| Deployment Size | Agents | max_connections | max_idle | conn_max_lifetime | Notes |
|-----------------|--------|-----------------|----------|-------------------|-------|
| **Small** | 1-100 | 25 | 5 | 30m | Conservative, low resource usage |
| **Medium** | 100-500 | 50 (default) | 12 | 30m | Balanced for most deployments |
| **Large** | 500-1000 | 100 | 25 | 30m | Requires PostgreSQL tuning |
| **Extra Large** | 1000+ | 200 | 50 | 30m | Consider PgBouncer |

### Agent Workload Patterns

**Heartbeats (30s interval):**
- 1000 agents = 33 req/sec sustained

**Diagnostics (5min interval):**
- 1000 agents = 3.3 req/sec sustained

**Peak burst:** 10x sustained rate (330 req/sec for 1000 agents)

---

## PostgreSQL Server Configuration

### 1. Edit PostgreSQL Configuration

**File:** `/etc/postgresql/*/main/postgresql.conf` (Linux) or `/usr/local/var/postgres/postgresql.conf` (Mac)

```conf
# ============================================================================
# Connection Settings
# ============================================================================

# Increase max connections (default: 100)
# Must be >= Lumo's max_connections setting
max_connections = 200

# Reserve connections for superuser emergencies
superuser_reserved_connections = 3

# ============================================================================
# Memory Settings
# ============================================================================

# Shared buffers: 25% of RAM (minimum)
# Example for 16GB server: 4GB
shared_buffers = 4GB

# Effective cache size: 50-75% of RAM
# Informs query planner about available cache
effective_cache_size = 12GB

# Work mem: Memory per operation (sort, hash join)
# Formula: (Total RAM - shared_buffers) / (max_connections * 2)
# Example: (16GB - 4GB) / (200 * 2) = 30MB
work_mem = 30MB

# ============================================================================
# Write-Ahead Logging (WAL)
# ============================================================================

# WAL buffers: -1 = auto (1/32 of shared_buffers, max 16MB)
wal_buffers = -1

# Checkpoint settings (reduce I/O spikes)
checkpoint_completion_target = 0.9
max_wal_size = 2GB

# ============================================================================
# Query Tuning
# ============================================================================

# Parallel query workers
max_parallel_workers_per_gather = 4
max_parallel_workers = 8

# Random page cost (lower for SSD)
random_page_cost = 1.1  # SSD
# random_page_cost = 4.0  # HDD
```

### 2. Validate Configuration

```bash
# Check syntax
sudo -u postgres postgres -C config_file

# Test configuration
sudo -u postgres postgres --config-file=/etc/postgresql/*/main/postgresql.conf --check

# View current settings
psql -U postgres -c "SHOW max_connections;"
psql -U postgres -c "SHOW shared_buffers;"
```

### 3. Restart PostgreSQL

```bash
# Linux (systemd)
sudo systemctl restart postgresql

# macOS (Homebrew)
brew services restart postgresql@14

# Docker
docker restart lumo-postgres
```

### 4. Verify Changes

```sql
-- Connect to PostgreSQL
psql -U lumo -d lumo

-- Check current settings
SHOW max_connections;
SHOW shared_buffers;
SHOW effective_cache_size;
SHOW work_mem;
```

---

## Lumo Configuration

### Option 1: Environment Variables (Recommended for Production)

```bash
# Set in systemd unit file or .env
export LUMO_DATABASE_MAX_CONNECTIONS=100
export LUMO_DATABASE_MAX_IDLE=25
export LUMO_DATABASE_CONN_MAX_LIFETIME=30m
```

**Kubernetes ConfigMap:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lumo-config
data:
  LUMO_DATABASE_MAX_CONNECTIONS: "100"
  LUMO_DATABASE_MAX_IDLE: "25"
  LUMO_DATABASE_CONN_MAX_LIFETIME: "30m"
```

### Option 2: Config File

**Edit:** `~/.lumo/config.yaml` or `/etc/lumo/config.yaml`

```yaml
database:
  host: localhost
  port: 5432
  name: lumo
  user: lumo
  # Password via LUMO_DATABASE_PASSWORD env var
  ssl_mode: require  # Use SSL in production

  # Connection pool settings
  max_connections: 100   # Match deployment size
  max_idle: 25           # 25% of max_connections
  conn_max_lifetime: 30m # Prevent stale connections
```

### Option 3: Multiple Environment Configs

**Production:** `config.production.yaml`
```yaml
environment: production
database:
  max_connections: 200
  max_idle: 50
  conn_max_lifetime: 30m
```

**Staging:** `config.staging.yaml`
```yaml
environment: staging
database:
  max_connections: 50
  max_idle: 12
  conn_max_lifetime: 30m
```

**Development:** `config.development.yaml`
```yaml
environment: development
database:
  max_connections: 25
  max_idle: 5
  conn_max_lifetime: 30m
```

---

## Monitoring & Troubleshooting

### 1. Check Lumo Pool Stats

Lumo automatically logs pool statistics. Check application logs:

```bash
# View recent pool stats
journalctl -u lumo-api -n 100 | grep "connection pool"

# Example log output:
# INFO Database connection pool configured max_open_conns=50 max_idle_conns=12
# INFO Database connection pool stats in_use=15 idle=10 wait_count=0 utilization_percent=30
# WARN Connection pool utilization high (>75%) - consider increasing max_connections
```

### 2. PostgreSQL Connection Monitoring

**Query active connections:**
```sql
-- Current connection count
SELECT count(*) as connections,
       max_conn,
       round(count(*) * 100.0 / max_conn, 2) as percent_used
FROM pg_stat_activity
CROSS JOIN (SELECT setting::int as max_conn FROM pg_settings WHERE name = 'max_connections') c
GROUP BY max_conn;

-- Connections by database
SELECT datname, count(*) as connections
FROM pg_stat_activity
WHERE datname IS NOT NULL
GROUP BY datname
ORDER BY connections DESC;

-- Connections by user
SELECT usename, count(*) as connections
FROM pg_stat_activity
WHERE usename IS NOT NULL
GROUP BY usename
ORDER BY connections DESC;
```

### 3. Identify Connection Saturation

**Symptoms:**
- ⚠️ Log warnings: "Connection pool saturation detected (>90% utilization)"
- 🐌 Slow API responses
- ❌ Connection timeout errors: "pq: sorry, too many clients already"
- 📈 High `wait_count` in pool stats

**Query waiting connections:**
```sql
-- Check for connection waits
SELECT pid, usename, application_name, state, wait_event_type, wait_event
FROM pg_stat_activity
WHERE wait_event IS NOT NULL;

-- Long-running queries (blocking connections)
SELECT pid, now() - pg_stat_activity.query_start AS duration, query, state
FROM pg_stat_activity
WHERE state != 'idle'
ORDER BY duration DESC
LIMIT 10;
```

### 4. Fix Connection Saturation

**Immediate mitigation:**
```bash
# Increase max_connections (requires PostgreSQL restart)
# Edit postgresql.conf:
max_connections = 300

sudo systemctl restart postgresql
```

**Tune Lumo:**
```bash
# Increase Lumo's pool size
export LUMO_DATABASE_MAX_CONNECTIONS=150
export LUMO_DATABASE_MAX_IDLE=40

# Restart Lumo API server
systemctl restart lumo-api
```

**Add jitter to agent heartbeats** (reduces burst load):
```yaml
# agent config
agent:
  heartbeat_seconds: 60  # Increase from 30s
  # Agent adds ±20% jitter automatically
```

---

## Advanced: PgBouncer

For 1000+ agents, use **PgBouncer** for connection pooling at the database layer.

### Benefits
- Reduces PostgreSQL connections by 10-100x
- Lumo can have 1000 connections to PgBouncer
- PgBouncer maintains only 50-100 to PostgreSQL
- Transaction pooling mode reuses connections efficiently

### Installation

```bash
# Ubuntu/Debian
sudo apt-get install pgbouncer

# RHEL/CentOS
sudo yum install pgbouncer

# macOS
brew install pgbouncer
```

### Configuration

**File:** `/etc/pgbouncer/pgbouncer.ini`

```ini
[databases]
lumo = host=localhost port=5432 dbname=lumo

[pgbouncer]
listen_addr = 127.0.0.1
listen_port = 6432
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt

# Pool settings
pool_mode = transaction          # Best for Lumo
max_client_conn = 1000           # Lumo connections to PgBouncer
default_pool_size = 50           # PgBouncer connections to PostgreSQL
reserve_pool_size = 25           # Reserve for bursts
max_db_connections = 100         # Total PostgreSQL connections

# Timeouts
server_idle_timeout = 600
query_timeout = 0
```

**File:** `/etc/pgbouncer/userlist.txt`
```
"lumo" "md5<password_hash>"
```

Generate MD5 hash:
```bash
echo -n "passwordlumo" | md5sum
```

### Start PgBouncer

```bash
sudo systemctl enable pgbouncer
sudo systemctl start pgbouncer
sudo systemctl status pgbouncer
```

### Update Lumo Configuration

```yaml
database:
  host: localhost
  port: 6432              # PgBouncer port (not 5432)
  name: lumo
  user: lumo
  max_connections: 500    # Can be higher (PgBouncer pools them)
  max_idle: 50
```

### Monitor PgBouncer

```bash
# Connect to PgBouncer admin
psql -h localhost -p 6432 -U pgbouncer -d pgbouncer

# Show pool stats
SHOW POOLS;

# Show client connections
SHOW CLIENTS;

# Show server connections (to PostgreSQL)
SHOW SERVERS;

# Example output:
#  database |   user   | cl_active | cl_waiting | sv_active | sv_idle
# ----------+----------+-----------+------------+-----------+---------
#  lumo     | lumo     |       500 |          0 |        50 |      10
```

---

## Best Practices

### ✅ Do

- **Start conservative:** Use defaults (50 connections) and scale up based on metrics
- **Monitor continuously:** Check pool utilization weekly
- **Use SSL in production:** Set `ssl_mode: require`
- **Set connection lifetime:** 30m prevents stale connections
- **Log pool stats:** Review logs for saturation warnings
- **Use environment-specific configs:** Different settings for dev/staging/prod

### ❌ Don't

- **Don't set max_connections > PostgreSQL max_connections:** Server will reject connections
- **Don't set max_idle > max_connections:** Inefficient
- **Don't use very low lifetimes (<5m):** Causes connection churn
- **Don't ignore saturation warnings:** Will cause cascading failures
- **Don't use shared passwords:** Use per-environment credentials

---

## Troubleshooting Guide

| Symptom | Likely Cause | Solution |
|---------|--------------|----------|
| "too many clients already" error | max_connections exhausted | Increase PostgreSQL max_connections |
| High wait_count in pool stats | Insufficient Lumo connections | Increase LUMO_DATABASE_MAX_CONNECTIONS |
| Slow startup connections | No warm idle connections | Increase max_idle |
| Connection refused | PostgreSQL not running | systemctl start postgresql |
| SSL error "certificate verify failed" | ssl_mode=verify-full but no CA | Set ssl_mode=require or provide CA cert |
| Stale connection errors | Connections held too long | Decrease conn_max_lifetime |

---

## Performance Benchmarks

**Environment:** 16GB RAM, 8 vCPU, PostgreSQL 14, 1000 agents

| max_connections | Throughput (req/sec) | P95 Latency (ms) | Connection Waits |
|-----------------|----------------------|------------------|------------------|
| 25 | 150 | 450 | 12000/hour |
| 50 (default) | 300 | 180 | 800/hour |
| 100 | 550 | 95 | 50/hour |
| 200 | 850 | 60 | 0/hour |
| 500 (PgBouncer) | 1200 | 45 | 0/hour |

**Conclusion:** 50 connections (default) supports 100-500 agents. For 1000+ agents, use 200 connections or PgBouncer.

---

## Additional Resources

- [PostgreSQL Connection Pooling](https://www.postgresql.org/docs/current/runtime-config-connection.html)
- [PgBouncer Documentation](https://www.pgbouncer.org/)
- [Lumo Configuration Guide](getting-started.md)
- [PostgreSQL Performance Tuning](https://wiki.postgresql.org/wiki/Tuning_Your_PostgreSQL_Server)

---

**Questions?** Open an issue at https://github.com/ignacio/lumo/issues
