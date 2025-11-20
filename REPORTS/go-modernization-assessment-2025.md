# Go Modernization Assessment: 2025 Stack Migration

**Project:** Lumo - Intelligent SRE/DevOps Automation Platform
**Assessment Date:** 2025-11-20
**Current State:** Phase 10 Complete (97 Go files, 66.7% test coverage)
**Target Stack:** Restate + gRPC/mTLS + Sandboxing + Chromem-go RAG

---

## Executive Summary

Lumo's codebase demonstrates **excellent architectural separation** with 85%+ of business logic decoupled from transport and execution layers. The modernization to the proposed 2025 stack is **VIABLE** with varying effort levels across components:

| Component | Current State | Target State | Viability | Effort | Risk |
|-----------|--------------|--------------|-----------|--------|------|
| **Transport** | Chi HTTP + JSON | gRPC + mTLS + Protobuf | ✅ High | **Medium** | Low |
| **Execution** | os/exec + SSH | Sandboxed (namespaces/cgroups) | ⚠️ Moderate | **High** | Medium |
| **State/Reliability** | PostgreSQL + Agent | Restate durable execution | ✅ High | **Medium-High** | Medium |
| **Intelligence** | AI Analysis (5 providers) | Local RAG (Chromem-go) | ✅ High | **Low-Medium** | Low |

**Overall Assessment:** Migration is architecturally sound with manageable risks. Recommended approach: **Incremental adoption** starting with gRPC, then RAG, then Restate, and finally sandboxing.

---

## 1. Transport Layer Assessment: gRPC + mTLS

### Current HTTP Implementation

**Framework:** Chi v5 router with net/http
**Architecture:** Clean separation - API handlers in `internal/api/`, zero HTTP dependencies in business logic
**Code Distribution:**
- API Layer: ~2,200 LOC (8 handlers, 5 middleware files)
- Business Logic: ~30,000 LOC (diagnostics, remediation, SSH, AI) - **100% reusable**

**Key Files:**
```
internal/api/
├── handlers/          # 1,709 LOC - diagnostics, remediation, agents, jobs, health
├── middleware/        # 363 LOC - auth, JWT, logging, CORS, recovery
├── router.go          # 87 LOC - Chi route definitions
├── server.go          # 148 LOC - HTTP server with graceful shutdown
└── response/          # Standardized JSON responses
```

### Current Strengths

1. **Interface-Based Execution:**
   ```go
   type CommandExecutor interface {
       ExecuteWithContext(ctx context.Context, command string, opts CommandOptions) (CommandResult, error)
   }
   // Implementations: LocalExecutor, SSHExecutor
   ```

2. **Repository Pattern:** Database access abstracted through repositories:
   ```go
   type JobRepository interface {
       Create(ctx context.Context, job *Job) error
       Get(ctx context.Context, id uuid.UUID) (*Job, error)
       List(ctx context.Context, filters JobFilters) ([]*Job, error)
   }
   ```

3. **Async Job Pattern:** Handlers return immediately, execution runs in goroutines:
   ```go
   // HTTP handler
   job := createJob()
   h.jobRepo.Create(ctx, job)
   go h.executeDiagnostics(context.Background(), job, req)
   return jobID  // Immediate response
   ```

4. **Protocol-Agnostic Auth:** JWT manager in `internal/api/auth/jwt.go` has no HTTP dependencies

### gRPC Migration Assessment

#### Coupling Analysis
**Score: 9/10** - Excellent separation

- ✅ Zero HTTP imports in business logic packages
- ✅ Diagnostics engine (`internal/diagnostics/`) completely transport-agnostic
- ✅ Remediation engine (`internal/remediation/`) no HTTP dependencies
- ✅ SSH client (`internal/ssh/`) no HTTP dependencies
- ✅ AI providers (`internal/ai/`) use standard HTTP clients (easily replaceable)
- ⚠️ Only API layer (`internal/api/`) tightly coupled to HTTP

#### What Needs Replacement

| Current | Target | Effort |
|---------|--------|--------|
| Chi router | gRPC server | Medium |
| JSON request/response | Protocol Buffers | Medium |
| HTTP middleware | gRPC interceptors | Low |
| REST endpoints (8) | gRPC service methods | Medium |
| `net/http` imports | `google.golang.org/grpc` | Low |

**Estimated LOC Changes:** ~2,200 LOC (API layer only) out of 32,000+ total

#### What Can Be Reused (85%+)

✅ **Core Business Logic (No Changes Required):**
- All 12 diagnostic checkers (`internal/diagnostics/checkers/`)
- Diagnostic runner and formatters
- Remediation executor and actions
- SSH client and session management
- AI provider clients (5 providers)
- Database repositories (job, agent, API key)
- Configuration system (Viper-based)
- Logging (logrus structured logging)

✅ **Partially Reusable with Minor Adaptation:**
- JWT auth logic (add gRPC metadata support)
- Async execution pattern (maps well to gRPC streaming)
- Health checks (gRPC health protocol)

#### Proposed gRPC Service Definition

```protobuf
syntax = "proto3";

service LumoAPI {
  // Diagnostics
  rpc RunDiagnostics(DiagnosticsRequest) returns (JobResponse);
  rpc GetDiagnosticsResult(JobIDRequest) returns (DiagnosticsResult);
  rpc StreamDiagnostics(DiagnosticsRequest) returns (stream DiagnosticsEvent);

  // Remediation
  rpc RunRemediation(RemediationRequest) returns (JobResponse);
  rpc GetRemediationResult(JobIDRequest) returns (RemediationResult);

  // Agents
  rpc RegisterAgent(RegisterAgentRequest) returns (AgentResponse);
  rpc SendHeartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc ListAgents(ListAgentsRequest) returns (ListAgentsResponse);

  // Jobs
  rpc ListJobs(ListJobsRequest) returns (ListJobsResponse);
  rpc CancelJob(JobIDRequest) returns (JobResponse);

  // Health
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}

message DiagnosticsRequest {
  string target = 1;
  repeated string checks = 2;
  bool analyze = 3;
  OutputFormat format = 4;
  map<string, string> metadata = 5;
}

message JobResponse {
  string job_id = 1;
  JobStatus status = 2;
  google.protobuf.Timestamp created_at = 3;
}
```

#### Migration Strategy

**Option 1: Dual Protocol (Recommended)**
- Run HTTP and gRPC servers side-by-side
- Share business logic between both
- Gradual client migration
- Use gRPC-Gateway for HTTP→gRPC translation during transition

```go
// Shared business logic
type DiagnosticsService struct {
    executor diagnostics.Runner
    jobRepo  repository.JobRepository
}

// HTTP handler
func (h *HTTPHandler) RunDiagnostics(w http.ResponseWriter, r *http.Request) {
    result := h.service.Execute(ctx, req)
    json.NewEncoder(w).Encode(result)
}

// gRPC handler
func (g *GRPCHandler) RunDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.JobResponse, error) {
    result := g.service.Execute(ctx, req)
    return toProto(result), nil
}
```

**Option 2: Complete Replacement**
- Rewrite API layer entirely in gRPC
- Remove HTTP dependencies
- Faster but higher risk
- No backward compatibility

#### mTLS Implementation

**Current Security:**
- API key authentication (database-backed)
- JWT tokens for agent auth
- TLS support (flag: `tls_enabled`)

**Target mTLS Setup:**
```go
// Server-side
creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
opts := []grpc.ServerOption{
    grpc.Creds(creds),
    grpc.UnaryInterceptor(authInterceptor),
}
grpcServer := grpc.NewServer(opts...)

// Client-side (agents)
creds, err := credentials.NewClientTLSFromFile(caFile, serverName)
conn, err := grpc.Dial(target, grpc.WithTransportCredentials(creds))
```

**Auth Interceptor:**
```go
func authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    // Extract JWT from metadata
    md, ok := metadata.FromIncomingContext(ctx)
    token := md.Get("authorization")[0]

    // Validate (reuse existing JWT manager)
    claims, err := jwtManager.Validate(token)

    // Inject into context
    ctx = context.WithValue(ctx, "claims", claims)
    return handler(ctx, req)
}
```

### Viability Assessment

**Overall Viability: ✅ HIGH**

**Strengths:**
- Clean architecture with excellent separation
- Business logic 100% transport-agnostic
- Repository pattern facilitates testing
- Async pattern maps naturally to gRPC streaming

**Challenges:**
- ~2,200 LOC to rewrite (API layer)
- Protocol Buffer schema design
- Client migration (if external clients exist)
- Dual protocol complexity during transition

**Risks:**
- 🟢 **Low:** Breaking business logic (well-isolated)
- 🟡 **Medium:** Client compatibility during migration
- 🟢 **Low:** Performance degradation (gRPC is typically faster)

### Refactor Effort: **MEDIUM**

**Breakdown:**
- Protocol Buffer definitions: 1-2 days
- gRPC server implementation: 3-5 days
- Interceptor migration (auth, logging, recovery): 2-3 days
- Testing and validation: 3-4 days
- Documentation updates: 1 day

**Total Estimate:** 10-15 developer days

**Dependencies:**
```bash
go get google.golang.org/grpc
go get google.golang.org/protobuf
go get github.com/grpc-ecosystem/grpc-gateway/v2  # Optional, for HTTP→gRPC
```

---

## 2. Execution Safety Assessment: Sandboxing

### Current Execution Model

**Architecture:** Dual-mode execution with interface abstraction

```go
type CommandExecutor interface {
    ExecuteWithContext(ctx context.Context, command string, opts CommandOptions) (CommandResult, error)
}

// Implementations
type LocalExecutor struct {}   // os/exec for localhost
type SSHExecutor struct {}     // SSH for remote hosts
```

**Auto-Detection:**
```go
if isLocalhost(hostname) {
    executor = diagnostics.NewLocalExecutor()  // No SSH overhead
} else {
    executor = diagnostics.NewSSHExecutor(sshClient)
}
```

### os/exec Usage Analysis

**LocalExecutor** (`internal/diagnostics/executor.go:56-106`):
```go
func (e *LocalExecutor) ExecuteWithContext(ctx context.Context, command string, opts CommandOptions) (CommandResult, error) {
    cmd := exec.CommandContext(ctx, "sh", "-c", command)  // ⚠️ Shell execution

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if opts.WorkingDir != "" {
        cmd.Dir = opts.WorkingDir  // Working directory support
    }

    if opts.Env != nil {
        cmd.Env = opts.Env  // Environment variable support
    }

    err := cmd.Run()
    return CommandResult{
        Stdout:   stdout.String(),
        Stderr:   stderr.String(),
        ExitCode: cmd.ProcessState.ExitCode(),
    }, err
}
```

**Pattern Found:** All commands run through `sh -c` for shell feature compatibility (pipes, redirects, etc.)

### Command Patterns from Checkers

**Simple Commands:**
```bash
nproc --all
free -b
df -h
ps aux
systemctl list-units
```

**Complex Pipelines:**
```bash
top -bn2 -d 0.5 | grep '^%Cpu' | tail -1 | awk '{print $2}' | tr -d 'us,'
ps aux --sort=-%mem | head -11 | tail -10
tail -n 1000 /var/log/auth.log | grep 'Failed password' | wc -l
```

**Network Commands:**
```bash
nc -w 5 -zv host port || bash -c 'cat < /dev/null > /dev/tcp/host/port'
curl -I --max-time 5 http://example.com
```

### Privilege Analysis

**RequiresRoot() Interface:**
All 12 checkers implement this metadata method:

```go
type Checker interface {
    Name() string
    Category() CheckCategory
    RequiresRoot() bool  // ⚠️ Metadata only, not enforced
    Run(ctx context.Context, executor CommandExecutor) (*CheckResult, error)
}
```

**Current Privilege Requirements:**

| Checker | Requires Root | Notes |
|---------|---------------|-------|
| CPU | ❌ No | `/proc/stat`, `nproc` |
| Memory | ❌ No | `/proc/meminfo`, `free` |
| Disk | ❌ No | `df`, user-accessible paths |
| Process | ❌ No | `ps aux` (all processes visible) |
| Service | ❌ No | `systemctl --user` fallback |
| Network | ❌ No | Non-privileged ports only |
| Patch Status | ❌ No | `apt list`, `yum list` |
| Open Ports | ❌ No | `ss -tulpn` (may need root for PID mapping) |
| SSH Security | ❌ No | Reads world-readable configs |
| Auth Failures | ❌ No | Log files typically readable by adm group |
| Kubernetes | ❌ No | Uses kubeconfig auth |
| Proxmox | ✅ **YES** | `pvecm`, `pvesh` commands need root |

**Key Finding:** Only 1 of 12 checkers requires root. System designed for **unprivileged operation**.

### Context Timeout Usage

**✅ Comprehensive Context Support:**

```go
// All execution paths use context
cmd := exec.CommandContext(ctx, "sh", "-c", command)  // Native timeout

// SSH execution
func (s *SSHExecutor) ExecuteWithContext(ctx context.Context, command string, opts CommandOptions) (CommandResult, error) {
    session, err := s.client.NewSession()

    // Goroutine with timeout
    done := make(chan error, 1)
    go func() { done <- session.Run(command) }()

    select {
    case <-ctx.Done():
        session.Close()
        return CommandResult{}, ctx.Err()
    case err := <-done:
        return result, err
    }
}
```

**Timeout Hierarchy:**
1. User-specified via flags
2. Checker default: 30 seconds
3. Executor fallback: 30 seconds
4. Context cancellation propagates to child processes (SIGKILL)

### Sandboxing Assessment

#### Architectural Compatibility

**Current Execution Assumptions:**
- ✅ Context timeouts already implemented
- ✅ Stdout/stderr capture via buffers
- ✅ Exit code handling
- ✅ Working directory support (`cmd.Dir`)
- ✅ Environment variables (`cmd.Env`)
- ⚠️ Runs through shell (`sh -c`) for compatibility
- ⚠️ No privilege dropping (runs as current user)
- ⚠️ No resource limits (memory, CPU, file descriptors)

#### Proposed Sandboxing Interface

```go
type SandboxedExecutor struct {
    enableNamespaces bool
    enableCgroups    bool
    limits           ResourceLimits
}

type ResourceLimits struct {
    MemoryMB      uint64
    CPUShares     uint64
    MaxProcesses  int
    MaxOpenFiles  int
    TimeoutSec    int
}

func (e *SandboxedExecutor) ExecuteWithContext(ctx context.Context, command string, opts CommandOptions) (CommandResult, error) {
    cmd := exec.CommandContext(ctx, "sh", "-c", command)

    // Apply Linux namespace isolation
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Cloneflags: syscall.CLONE_NEWUSER |  // User namespace (no root needed)
                   syscall.CLONE_NEWPID |   // PID namespace
                   syscall.CLONE_NEWNS |    // Mount namespace
                   syscall.CLONE_NEWNET,    // Network namespace (⚠️ breaks connectivity)
        UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
        GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
    }

    // Apply cgroup limits
    applyResourceLimits(cmd.Process.Pid, e.limits)

    return runAndCapture(cmd)
}
```

#### Compatibility Challenges

**1. Shell Dependency:**
- Current: All commands run through `sh -c`
- Problem: Complex pipelines require shell features
- Solution: Keep shell, sandbox the shell process

**2. Network Access:**
- Current: Many checkers need network (curl, nc, SSH)
- Problem: `CLONE_NEWNET` creates isolated network namespace
- Solution: Don't use network namespace, or use veth pairs

**3. File System Access:**
- Current: Checkers read from `/proc`, `/sys`, `/var/log`
- Problem: Mount namespace isolation breaks access
- Solution: Bind-mount required paths into sandbox

**4. SSH Execution:**
- Current: `SSHExecutor` runs commands on remote hosts
- Problem: Can't sandbox remote execution
- Solution: Only apply sandboxing to `LocalExecutor`

#### Privilege Escalation Detection

**Current System Behavior:**
```go
// If command needs root but user is not root, it fails
result, err := executor.ExecuteWithContext(ctx, "pvesh get /cluster/resources")
// Exit code: 13 (permission denied)
```

**No Automatic Sudo:**
- System does NOT inject `sudo` prefix
- Commands run with current user privileges
- Failures logged but not escalated

#### Sandboxing Implementation Approach

**Option 1: Direct syscall.SysProcAttr (Lightweight)**
```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWPID,
    UidMappings: []syscall.SysProcIDMap{{...}},
}
```

**Pros:**
- No external dependencies
- Minimal overhead
- Works with user namespaces (no root needed)

**Cons:**
- Manual cgroup management
- Complex setup for full isolation
- Limited cross-platform support (Linux-only)

**Option 2: libcontainer (Heavier, more complete)**
```go
import "github.com/opencontainers/runc/libcontainer"

factory, _ := libcontainer.New("/var/run/lumo")
config := &configs.Config{
    Namespaces: []configs.Namespace{
        {Type: configs.NEWUSER},
        {Type: configs.NEWPID},
        {Type: configs.NEWNS},
    },
    Cgroups: &configs.Cgroup{
        Resources: &configs.Resources{
            Memory: 512 * 1024 * 1024,  // 512MB
            CpuShares: 512,
        },
    },
}
container, _ := factory.Create("lumo-sandbox", config)
```

**Pros:**
- Complete OCI runtime implementation
- Comprehensive resource limits
- Battle-tested (Docker, Podman)

**Cons:**
- Large dependency (~50MB)
- Complex API
- Still Linux-only

**Option 3: Hybrid Approach (Recommended)**
```go
type CommandExecutor interface {
    ExecuteWithContext(ctx context.Context, command string, opts CommandOptions) (CommandResult, error)
}

// Wraps existing executors
type SandboxedExecutor struct {
    inner CommandExecutor  // LocalExecutor or SSHExecutor
    sandbox SandboxConfig
}

func (e *SandboxedExecutor) ExecuteWithContext(ctx, cmd, opts) (CommandResult, error) {
    if !e.sandbox.Enabled {
        return e.inner.ExecuteWithContext(ctx, cmd, opts)  // No-op passthrough
    }

    // Only sandbox local execution
    if _, ok := e.inner.(*LocalExecutor); !ok {
        return e.inner.ExecuteWithContext(ctx, cmd, opts)  // SSH not sandboxed
    }

    // Apply sandboxing
    return e.runSandboxed(ctx, cmd, opts)
}
```

### Viability Assessment

**Overall Viability: ⚠️ MODERATE**

**Strengths:**
- Clean executor interface makes wrapping easy
- Context timeouts already implemented
- Stdout/stderr capture already structured
- Most checkers don't require privileged operations
- User namespace support eliminates need for root

**Challenges:**
- Shell dependency (`sh -c`) complicates direct exec replacement
- Network namespace isolation breaks connectivity checkers
- Mount namespace breaks `/proc`, `/sys` access
- Cross-platform support (Windows, macOS don't have namespaces)
- Increased complexity for minimal security gain (system already runs unprivileged)

**Risks:**
- 🟡 **Medium:** Breaking existing checker functionality (pipelines, network access)
- 🟢 **Low:** Performance overhead (namespaces are lightweight)
- 🔴 **High:** Cross-platform compatibility loss
- 🟡 **Medium:** Increased maintenance burden (cgroup v1 vs v2, kernel version dependencies)

### Refactor Effort: **HIGH**

**Breakdown:**
- Design sandboxing interface: 1-2 days
- Implement user/PID namespace support: 3-5 days
- Cgroup resource limit integration: 2-3 days
- Testing with all 12 checkers: 5-7 days (many edge cases)
- Handle bind mounts for `/proc`, `/sys`: 2-3 days
- Fallback logic for unsupported platforms: 2-3 days
- Documentation and security audit: 2 days

**Total Estimate:** 17-25 developer days

**Dependencies:**
```bash
# Option 1: Lightweight
# (No dependencies, use syscall package)

# Option 2: libcontainer
go get github.com/opencontainers/runc/libcontainer
go get github.com/opencontainers/runtime-spec
```

**Recommendation:** **Defer sandboxing to Phase 12 (Security Hardening)**

**Rationale:**
- System already runs unprivileged for 11/12 checkers
- High implementation complexity with limited security gain
- Risk of breaking existing functionality
- Cross-platform support loss
- Focus on higher-ROI modernizations first (gRPC, RAG, Restate)

---

## 3. State & Reliability Assessment: Restate

### Current State Management Architecture

**Hybrid Model:** Stateless API + Stateful Agents

```
┌─────────────────────────┐
│   API Server            │
│   (Stateless)           │
│   - Accepts requests    │
│   - Creates jobs in DB  │
│   - Returns job IDs     │
│   - Executes async      │
└─────────────────────────┘
          │
          ▼
┌─────────────────────────┐
│   PostgreSQL            │
│   - Jobs (JSONB)        │
│   - Agents              │
│   - API Keys            │
└─────────────────────────┘
          │
          ▼
┌─────────────────────────┐
│   Agents (Stateful)     │
│   - Local cache         │
│   - Cron scheduler      │
│   - Health status       │
│   - Metrics (Prometheus)│
└─────────────────────────┘
```

### In-Memory State Analysis

**Agent State Components:**

| Component | Storage | Thread-Safe | Serializable | Recoverable |
|-----------|---------|-------------|--------------|-------------|
| **Agent Config** | Struct | ❌ (read-only) | ✅ YAML | ✅ Config file |
| **Health Status** | Struct + RWMutex | ✅ | ✅ JSON | ❌ Resets on restart |
| **Cron Scheduler** | map[string]cron.EntryID | ✅ Mutex | ❌ Runtime-only | ❌ Rebuilds on restart |
| **Local Cache** | Disk (JSON files) | ✅ RWMutex | ✅ JSON | ✅ Persists across restarts |
| **Prometheus Metrics** | Prometheus client | ✅ (internal) | ❌ Time-series | ⚠️ Requires scraper |

**Key Finding:** Agent state is **mostly ephemeral** with persistent caching for offline resilience.

### State Persistence Patterns

**PostgreSQL Jobs Table:**
```sql
CREATE TABLE jobs (
    id UUID PRIMARY KEY,
    type VARCHAR(50) NOT NULL,  -- diagnostic, remediation
    status VARCHAR(20) NOT NULL,  -- pending, running, completed, failed, cancelled
    target VARCHAR(255),
    result JSONB,  -- Flexible storage
    metadata JSONB,  -- Additional context
    created_at TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP
);
```

**State Transitions:**
```
pending → running → completed
                 ↘ failed
                 ↘ cancelled
```

**Async Execution Pattern:**
```go
// Handler (internal/api/handlers/diagnostics.go)
func (h *Handler) RunDiagnostics(w http.ResponseWriter, r *http.Request) {
    job := &models.Job{
        ID:     uuid.New(),
        Type:   models.JobTypeDiagnostic,
        Status: models.JobStatusPending,  // Initial state
        Target: req.Target,
    }

    h.jobRepo.Create(r.Context(), job)  // Persist before execution

    go h.executeDiagnostics(context.Background(), job, req)  // ⚠️ Fire-and-forget

    response.Created(w, DiagnosticResponse{JobID: job.ID})  // Immediate return
}

func (h *Handler) executeDiagnostics(ctx context.Context, job *models.Job, req DiagnosticsRequest) {
    // Update status to running
    job.Status = models.JobStatusRunning
    h.jobRepo.Update(ctx, job)

    // Execute diagnostics
    result, err := h.diagnosticRunner.Run(ctx, req)

    // Update status and result
    if err != nil {
        job.Status = models.JobStatusFailed
        job.Error = err.Error()
    } else {
        job.Status = models.JobStatusCompleted
        job.Result = json.RawMessage(result)
    }
    h.jobRepo.Update(ctx, job)
}
```

**Key Pattern:** Database-backed job queue with status tracking, but **no durable execution guarantees**.

### Polling & Scheduling Patterns

**1. Cron-Based Scheduling** (`internal/agent/scheduler.go`):
```go
type Scheduler struct {
    cron  *cron.Cron
    tasks map[string]cron.EntryID  // ⚠️ In-memory task registry
    mu    sync.Mutex
}

func (s *Scheduler) AddTask(name, cronExpr string, cmd func()) error {
    entryID, err := s.cron.AddFunc(cronExpr, cmd)
    s.tasks[name] = entryID  // Store entry ID
    return err
}

func (s *Scheduler) Start() {
    s.cron.Start()  // ⚠️ State lost on restart
}
```

**Issues for Restate:**
- Task registry not persisted
- Scheduled tasks reset on agent restart
- No distributed coordination (each agent schedules independently)

**2. Heartbeat Loop** (`internal/agent/agent.go:208-229`):
```go
func (a *Agent) heartbeatLoop(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-a.stopCh:
            return
        case <-ticker.C:
            if err := a.reporter.SendHeartbeat(ctx); err != nil {
                log.WithError(err).Error("Failed to send heartbeat")
                a.healthStatus.SetUnhealthy("heartbeat_failed", err)
            }
            a.metrics.heartbeatTotal.Inc()
        }
    }
}
```

**Pattern:** Simple ticker with select statement - **stateless**, restarts cleanly.

**3. Continuous Mode** (`internal/agent/agent.go:273-294`):
```go
func (a *Agent) runContinuousMode(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-a.stopCh:
            return
        default:
            result, err := a.runDiagnostics(ctx)
            a.reporter.SendReport(ctx, result)
            time.Sleep(30 * time.Second)  // ⚠️ Fixed delay, no backpressure
        }
    }
}
```

**Issues for Restate:**
- No state checkpointing (restarts lose progress)
- Fixed sleep delay (no adaptive backoff)
- No distributed work distribution

### Job Queue Implementation

**Current Approach: No Explicit Queue**

```go
// API Handler
go h.executeDiagnostics(context.Background(), job, req)  // ⚠️ Unbounded goroutines
```

**Issues:**
- ❌ No worker pool (unbounded goroutine spawn)
- ❌ No rate limiting
- ❌ No priority queue
- ❌ No retry mechanism for failed jobs
- ❌ No dead-letter queue

**Job Tracking:**
- ✅ Jobs persisted in PostgreSQL before execution
- ✅ Status transitions tracked
- ⚠️ No automatic retry on failure
- ⚠️ No timeout enforcement (context cancellation only)

### Restate Migration Assessment

#### Current State Characteristics

**Stateless Components (Good for Restate):**
- ✅ API handlers (HTTP → gRPC)
- ✅ Diagnostic checkers (pure functions of command execution)
- ✅ AI analysis (stateless provider calls)
- ✅ SSH client (ephemeral connections)

**Stateful Components (Need Adaptation):**
- ⚠️ Cron scheduler (task registry)
- ⚠️ Agent health status (ephemeral, acceptable loss)
- ⚠️ Local cache (file-based, already persistent)
- ✅ Job state (already in PostgreSQL, easy to integrate)

#### Restate Fit Analysis

**What Restate Provides:**
1. **Durable Execution:** Functions resume from last checkpoint on failure
2. **State Management:** Persistent key-value state within handlers
3. **Timers & Delays:** Durable sleep/timers (better than cron)
4. **Workflow Orchestration:** Multi-step processes with retries
5. **Distributed Coordination:** Built-in leader election, locking

**Lumo Use Cases:**

**1. Diagnostic Workflow (Great Fit):**
```go
// Current (fire-and-forget)
go h.executeDiagnostics(context.Background(), job, req)

// With Restate
func DiagnosticsWorkflow(ctx restate.Context, req DiagnosticsRequest) error {
    // Step 1: Mark job as running (durable)
    jobID := ctx.Random().UUID()
    ctx.Set("job_status", "running")

    // Step 2: Execute checkers (resumes here if crashed)
    result, err := ctx.Run("execute_diagnostics", func() (DiagnosticResult, error) {
        return runCheckers(req)
    })
    if err != nil {
        ctx.Set("job_status", "failed")
        return err
    }

    // Step 3: AI analysis (separate durable step)
    analysis, err := ctx.Run("ai_analysis", func() (AIAnalysis, error) {
        return analyzeWithAI(result)
    })

    // Step 4: Store result (guaranteed to run if previous steps succeeded)
    ctx.Set("job_status", "completed")
    ctx.Set("job_result", result)
    return nil
}
```

**Benefits:**
- Automatic retry on crash
- Progress checkpointing
- Exactly-once execution guarantee

**2. Scheduled Diagnostics (Good Fit):**
```go
// Current (cron-based, lost on restart)
scheduler.AddTask("diagnostics", "*/5 * * * *", runDiagnostics)

// With Restate
func ScheduledDiagnosticsWorkflow(ctx restate.Context, target string) error {
    for {
        // Durable sleep (survives restarts)
        ctx.Sleep(5 * time.Minute)

        // Invoke diagnostic workflow
        ctx.Call(DiagnosticsWorkflow, DiagnosticsRequest{Target: target})
    }
}
```

**Benefits:**
- Schedule persists across restarts
- Exactly-once execution per interval
- Distributed coordination (only one agent runs)

**3. Agent Heartbeat (Moderate Fit):**
```go
// With Restate
func AgentHeartbeatWorkflow(ctx restate.Context, agentID string) error {
    for {
        ctx.Sleep(30 * time.Second)  // Durable timer

        // Update last_seen timestamp (survives crash)
        ctx.Set("last_heartbeat", time.Now())
        ctx.Call(UpdateAgentStatus, agentID, "online")
    }
}
```

**Benefits:**
- Heartbeat state survives agent restart
- Automatic recovery after crash

**4. Remediation with Approval (Excellent Fit):**
```go
// Current (synchronous, no durability)
approval, err := approver.RequestApproval(action)
if approval.Approved {
    executor.Execute(action)
}

// With Restate
func RemediationWorkflow(ctx restate.Context, action RemediationAction) error {
    // Step 1: Request approval (durable)
    ctx.Call(RequestApprovalService, action)

    // Step 2: Wait for human approval (durable wait, can be hours/days)
    approvalEvent := ctx.AwaitPromise("approval:" + action.ID)

    if !approvalEvent.Approved {
        return errors.New("approval denied")
    }

    // Step 3: Execute remediation (only runs if approved)
    result, err := ctx.Run("execute_remediation", func() error {
        return executeAction(action)
    })

    // Step 4: Verify result (separate checkpoint)
    ctx.Run("verify_remediation", func() error {
        return verifyAction(action, result)
    })

    return nil
}
```

**Benefits:**
- Approval can take hours/days without blocking resources
- Crash-safe: won't execute without approval, won't re-execute if already completed
- Full audit trail built-in

#### Migration Strategy

**Phase 1: API Handler Integration**
```go
// Wrap existing handlers as Restate services
type LumoService struct {
    diagnosticsRunner *diagnostics.Runner
    remediationRunner *remediation.Executor
}

func (s *LumoService) RunDiagnostics(ctx restate.Context, req DiagnosticsRequest) (DiagnosticsResult, error) {
    // Reuse existing business logic
    return s.diagnosticsRunner.Run(ctx, req)
}
```

**Phase 2: Replace Job Queue**
- Remove `go h.executeDiagnostics()` goroutines
- Replace with `restate.Call()` invocations
- Remove job status tracking code (Restate provides this)

**Phase 3: Replace Cron Scheduler**
- Remove `robfig/cron` dependency
- Replace with Restate durable timers
- Migrate scheduled tasks to Restate workflows

**Phase 4: Enhanced Workflows**
- Multi-step diagnostics (pre-check → execute → analyze → report)
- Human-in-the-loop remediation with durable approval
- Distributed agent coordination (leader election for cluster-wide checks)

#### Challenges

**1. State Serialization:**
- **Current:** Uses `map[string]interface{}` for flexible data
- **Restate:** Requires serializable types (JSON, protobuf)
- **Solution:** Already using JSON extensively (CheckResult, Report)

**2. Goroutine-Based Concurrency:**
- **Current:** Heavy use of goroutines and channels
- **Restate:** Virtual threads with durable scheduling
- **Solution:** Adapt patterns to use Restate's concurrency primitives

**3. Local Cache:**
- **Current:** File-based cache for offline mode
- **Restate:** Centralized state management
- **Solution:** Keep local cache as fallback, use Restate state as primary

**4. Cron Complexity:**
- **Current:** Robfig/cron supports complex expressions (`*/5 * * * *`)
- **Restate:** Simple timers (no cron expressions)
- **Solution:** Use external cron trigger → invoke Restate workflow, or implement cron logic in workflow

**5. Metrics and Observability:**
- **Current:** Prometheus metrics, structured logging
- **Restate:** Built-in tracing, logging
- **Solution:** Integrate Restate tracing with existing Prometheus/logrus setup

### Viability Assessment

**Overall Viability: ✅ HIGH**

**Strengths:**
- Business logic already stateless (pure functions)
- Job state already in PostgreSQL (easy to migrate to Restate state)
- Clear workflow boundaries (diagnostics, remediation, heartbeat)
- Async pattern maps naturally to Restate's durable execution
- Human-in-the-loop approval is a killer use case for Restate

**Challenges:**
- Rewriting job execution from goroutines to Restate workflows
- Replacing cron scheduler with durable timers
- Learning curve for Restate SDK
- Integration with existing PostgreSQL (dual state stores during transition)

**Risks:**
- 🟡 **Medium:** Complexity increase (new abstraction layer)
- 🟢 **Low:** Breaking existing functionality (business logic unchanged)
- 🟡 **Medium:** Operational overhead (new infrastructure component)

### Refactor Effort: **MEDIUM-HIGH**

**Breakdown:**
- Restate SDK integration and setup: 2-3 days
- Diagnostic workflow migration: 3-5 days
- Remediation workflow with approval: 3-4 days
- Scheduler replacement with durable timers: 2-3 days
- Agent heartbeat workflow: 1-2 days
- Job queue removal and cleanup: 2 days
- Testing and validation: 5-7 days
- Documentation: 2-3 days

**Total Estimate:** 20-29 developer days

**Dependencies:**
```bash
go get github.com/restatedev/sdk-go
```

**Recommendation:** **High value, defer to Phase 11-12**

**Rationale:**
- Significant reliability improvement (durable execution, retries)
- Excellent fit for human-in-the-loop workflows
- Simplifies agent coordination (built-in distributed primitives)
- But: High effort, new infrastructure dependency
- Prioritize gRPC and RAG first (lower effort, high value)

---

## 4. Intelligence Readiness Assessment: Local RAG

### Current AI Architecture

**Providers (5):** Anthropic (Claude), OpenAI (GPT), Ollama, Gemini, OpenRouter
**Pattern:** Prompt → Provider API → Stream/Parse → Analysis
**Token Optimization:** TOON format (30-60% reduction vs JSON)

**PromptBuilder** (`internal/ai/prompts.go`):
```go
type PromptBuilder struct {
    includeThinking bool
    focusAreas      []string
    useTOON         bool  // Token-efficient format
}

func BuildAnalysisPrompt(req *AnalysisRequest) (string, error) {
    // 1. System information context
    // 2. Selected checks (what data is available)
    // 3. Report summary statistics
    // 4. Detailed results in TOON format
    // 5. Focus areas (user-specified concerns)
    // 6. Analysis request

    return prompt, nil
}
```

**Current Token Budget:**
- System prompt: ~500 tokens
- Diagnostic data (TOON): 1-5K tokens
- Response: 1-2K tokens
- **Total: ~5-10K tokens per analysis**

### Log Parsing Capabilities

**Current State: Limited**

**Log Reading (Only 1 Checker):**
```go
// AuthFailuresChecker (internal/diagnostics/checkers/auth_failures.go)
command := "tail -n 1000 /var/log/auth.log"  // Last 1000 lines
output, _ := executor.ExecuteWithContext(ctx, command)

// Parse with simple string matching (NO regex)
lines := strings.Split(output.Stdout, "\n")
for _, line := range lines {
    if strings.Contains(line, "Failed password") {
        failedAttempts++
        // Extract username, IP with strings.Fields()
    }
}
```

**No General-Purpose Log Ingestion:**
- ❌ No `LogReader` interface
- ❌ No regex parsing library usage
- ❌ No structured log parsing (JSON logs, syslog format)
- ❌ No streaming log tail (only batch reads)
- ❌ No log file discovery (hardcoded paths)

### File Reading Patterns

**Minimal Direct File I/O:**
```go
// Found in:
internal/agent/cache.go:       os.ReadFile()      // Read cached JSON results
internal/ssh/auth.go:          os.ReadFile()      // Read SSH private key files
internal/remediation/audit.go: os.OpenFile()      // Append-only audit log
```

**Key Pattern:** System prefers command execution over direct file reading
- CPU: `cat /proc/stat` instead of `os.Open("/proc/stat")`
- Memory: `cat /proc/meminfo` instead of `os.ReadFile()`
- Logs: `tail -n 1000` instead of `bufio.Scanner`

**Rationale:** Unified interface for local and remote (SSH) execution

### Data Collection Standardization

**CheckResult Structure** (`internal/diagnostics/result.go`):
```go
type CheckResult struct {
    Name      string
    Category  CheckCategory   // core, security, specialized
    Status    CheckStatus     // pass, fail, warning
    Severity  Severity        // info, low, medium, high, critical
    Message   string          // Human-readable summary
    Data      map[string]interface{}  // ⚠️ Flexible, unstructured storage
    Metrics   []Metric        // Structured time-series data
    Timestamp time.Time
    Duration  time.Duration
    Error     string
}

type Metric struct {
    Name      string
    Value     interface{}  // float64, int, string
    Unit      string       // %, MB, count
    Timestamp time.Time
    Labels    map[string]string  // Additional dimensions
}

type Report struct {
    Timestamp time.Time
    Duration  time.Duration
    Results   []*CheckResult
    Summary   ReportSummary
    Metadata  map[string]interface{}  // Host info, platform, etc.
}
```

**Standardization Level:**
- ✅ Consistent `CheckResult` structure across all 12 checkers
- ✅ Metrics array for time-series data
- ⚠️ `Data` field is `map[string]interface{}` (flexible but untyped)
- ❌ No log entry structure (only AuthFailuresChecker, ad-hoc format)

**Data Flow:**
```
Command Execution → Parse stdout → Populate CheckResult.Data + Metrics
```

### Memory Footprint Analysis

**Agent Baseline Memory:**
```go
// Target from CLAUDE.md
// Memory: 64-128 MB baseline, 256 MB peak
// Disk: 100 MB binary, 1 GB cache
```

**Current Memory Usage Patterns:**

**1. Bounded Collections:**
```go
// CPU checker: Top 5 processes by CPU
command := "ps -eo pid,comm,%cpu --sort=-%cpu | head -6"

// Memory checker: Top 10 processes by memory
command := "ps aux --sort=-%mem | head -11 | tail -10"

// Auth logs: Last 1000 lines only
command := "tail -n 1000 /var/log/auth.log"
```

**2. Streaming Design:**
```go
// AI response streaming (internal/ai/providers/anthropic.go)
scanner := bufio.NewScanner(resp.Body)
for scanner.Scan() {
    line := scanner.Bytes()
    // Process line-by-line, not all in memory
}
```

**3. Local Cache (Disk-Based):**
```go
// internal/agent/cache.go
type Cache struct {
    dir      string      // File-based storage
    maxSize  int64       // 1 GB default
    mu       sync.RWMutex
}

// Enforces size limits
func (c *Cache) enforceMaxSize() error {
    // Removes oldest entries (LRU)
}
```

**4. No Large Buffers:**
```go
// Command output captured in bytes.Buffer, but bounded by command timeout (30s)
var stdout, stderr bytes.Buffer
cmd.Stdout = &stdout
cmd.Stderr = &stderr
```

**Memory Footprint Assessment:**
- ✅ Bounded collections (top N processes)
- ✅ Streaming I/O for AI responses
- ✅ Disk-based caching (not in-memory)
- ✅ Short-lived command output buffers
- ✅ No large in-memory data structures

**Chromem-go Overhead:**
- Typical: 50-100 MB for vector index + embeddings (1000-10000 documents)
- Additional: Embedding model (if local, e.g., all-MiniLM: ~80MB)

**Total Projected Memory:**
- Baseline: 64-128 MB (current)
- Chromem-go: 50-100 MB (vector store + embeddings)
- Embedding model: 0-80 MB (use API or local)
- **Total: 114-308 MB** (within 256 MB peak target)

### RAG Integration Points

#### A. Data Ingestion Layer (New Component)

**Proposed:** `internal/intelligence/ingestion/`

```go
type LogIngestionManager struct {
    readers []LogReader
    parser  LogParser
    embedder Embedder
    store   VectorStore
}

type LogReader interface {
    Read(ctx context.Context, source LogSource) ([]LogEntry, error)
    Stream(ctx context.Context, source LogSource) (<-chan LogEntry, error)
}

type LogEntry struct {
    Timestamp  time.Time
    Level      string  // INFO, WARN, ERROR, DEBUG
    Source     string  // /var/log/syslog, application.log
    Message    string
    Fields     map[string]interface{}  // Structured data
    Raw        string  // Original line
}

type LogParser interface {
    Parse(raw string) (*LogEntry, error)
    ParseFormat(raw string, format LogFormat) (*LogEntry, error)
}

type LogFormat int
const (
    LogFormatSyslog LogFormat = iota
    LogFormatJSON
    LogFormatApache
    LogFormatNginx
    LogFormatCustom
)
```

**Integration with Checkers:**
```go
// Extend AuthFailuresChecker
func (c *AuthFailuresChecker) Run(ctx context.Context, executor CommandExecutor) (*CheckResult, error) {
    // Current: Command execution + string parsing
    output, _ := executor.ExecuteWithContext(ctx, "tail -n 1000 /var/log/auth.log")

    // Enhanced: Use LogParser
    parser := ingestion.NewSyslogParser()
    entries := parser.ParseMulti(output.Stdout)

    // Store in CheckResult
    result := &CheckResult{
        Data: map[string]interface{}{
            "log_entries": entries,  // Structured log data
            "failed_attempts": countFailures(entries),
        },
    }

    // Feed to RAG system
    if c.enableRAG {
        c.ragManager.IngestLogs(ctx, entries)
    }

    return result, nil
}
```

#### B. Vector Store Layer (New Component)

**Proposed:** `internal/intelligence/vectorstore/`

```go
type VectorStore interface {
    Store(ctx context.Context, doc Document) error
    Query(ctx context.Context, query string, k int) ([]Match, error)
    Delete(ctx context.Context, id string) error
}

type Document struct {
    ID         string
    Content    string                 // Text content
    Embedding  []float32              // Vector representation
    Metadata   map[string]interface{} // Hostname, timestamp, severity
    Timestamp  time.Time
}

type Match struct {
    Document  Document
    Score     float32  // Similarity score
}

// Chromem-go implementation
type ChromemStore struct {
    db *chromem.DB
}

func (s *ChromemStore) Store(ctx context.Context, doc Document) error {
    return s.db.AddDocument(doc.ID, doc.Content, doc.Metadata)
}

func (s *ChromemStore) Query(ctx context.Context, query string, k int) ([]Match, error) {
    results, err := s.db.Query(query, k)
    // Convert chromem results to Match structs
    return matches, err
}
```

**Storage Location:**
```
/var/lib/lumo/rag/
├── embeddings.db       # Chromem-go vector store
├── documents/          # Original documents (optional)
└── config.yaml         # Embedding model, parameters
```

#### C. Prompt Enrichment Layer (Extend Existing)

**Extend:** `internal/ai/prompts.go`

```go
type PromptBuilder struct {
    includeThinking bool
    focusAreas      []string
    useTOON         bool
    ragEnabled      bool  // NEW
    vectorStore     VectorStore  // NEW
}

func (pb *PromptBuilder) BuildAnalysisPrompt(req *AnalysisRequest) (string, error) {
    var prompt strings.Builder

    // 1. System information context (existing)
    prompt.WriteString(buildSystemContext(req))

    // 2. RAG context (NEW)
    if pb.ragEnabled {
        similarIncidents, err := pb.vectorStore.Query(req.Context, req.Query, 5)
        if err == nil && len(similarIncidents) > 0 {
            prompt.WriteString("\n## Historical Context (Similar Incidents)\n")
            for _, incident := range similarIncidents {
                prompt.WriteString(fmt.Sprintf("- [%s] %s (similarity: %.2f)\n",
                    incident.Metadata["timestamp"],
                    incident.Content,
                    incident.Score))
            }
        }
    }

    // 3. Current diagnostic data (existing, TOON format)
    prompt.WriteString(buildDiagnosticData(req.Report))

    // 4. Analysis request (existing)
    prompt.WriteString(buildAnalysisRequest(req))

    return prompt.String(), nil
}
```

**RAG Query Examples:**
```go
// Query: Current CPU spike detected
vectorStore.Query(ctx, "High CPU usage, system slowdown, load average 8.5", k=5)

// Results:
// 1. [2024-11-15] CPU spike caused by runaway cron job (similarity: 0.92)
//    Resolution: Identified and killed process, fixed cron schedule
//
// 2. [2024-11-10] High load due to memory leak in application (similarity: 0.85)
//    Resolution: Restarted service, applied memory limit
```

**Enhanced Prompt:**
```
System Information: Ubuntu 22.04, 8 cores, 16GB RAM

Historical Context (Similar Incidents):
- [2024-11-15] CPU spike caused by runaway cron job (similarity: 0.92)
  Resolution: Identified and killed process, fixed cron schedule
- [2024-11-10] High load due to memory leak in application (similarity: 0.85)
  Resolution: Restarted service, applied memory limit

Current Diagnostic Data (TOON):
cpu_metrics[3]{name,value,unit}:
load_1min,8.5,
load_5min,7.2,
load_15min,5.8

top_processes[5]{pid,name,cpu}:
1234,python,45.2
5678,node,32.1
...

Analysis Request:
Investigate high CPU usage. System response is slow. Are there any runaway processes?
Consider historical patterns from similar incidents.
```

#### D. Historical Storage (Extend Database)

**Extend PostgreSQL Schema:**
```sql
-- Store historical diagnostic reports
CREATE TABLE diagnostic_history (
    id UUID PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    report JSONB NOT NULL,  -- Full CheckResult data
    summary TEXT,  -- Human-readable summary
    severity VARCHAR(20),
    embedding VECTOR(384),  -- pgvector extension for embeddings
    INDEX idx_timestamp (timestamp),
    INDEX idx_hostname (hostname),
    INDEX idx_embedding USING ivfflat (embedding vector_cosine_ops)  -- Vector similarity search
);

-- Store remediation outcomes
CREATE TABLE remediation_history (
    id UUID PRIMARY KEY,
    diagnostic_id UUID REFERENCES diagnostic_history(id),
    action VARCHAR(255) NOT NULL,
    success BOOLEAN NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    details JSONB
);
```

**Hybrid Approach:**
- PostgreSQL: Queryable structured data, pgvector for embeddings
- Chromem-go: Fast in-memory vector search
- Sync: Batch sync from PostgreSQL to Chromem-go on startup

### Data Ingestion Hook Points

**Option 1: Agent Reporter (Best for Real-Time)**
```go
// internal/agent/reporter.go
func (r *Reporter) SendReport(ctx context.Context, report *diagnostics.Report) error {
    // Existing: Send to API
    if err := r.apiClient.SubmitReport(ctx, report); err != nil {
        return err
    }

    // NEW: Ingest to local RAG
    if r.ragEnabled {
        go r.ragManager.IngestReport(ctx, report)  // Async, non-blocking
    }

    return nil
}
```

**Option 2: Diagnostic Runner (Best for Centralized)**
```go
// internal/diagnostics/diagnostics.go
func (r *Runner) Run(ctx context.Context, req *RunRequest) (*Report, error) {
    // Execute all checkers
    report := r.executeCheckers(ctx, req)

    // NEW: Ingest to RAG before returning
    if r.ragEnabled {
        r.ragManager.IngestReport(ctx, report)
    }

    return report, nil
}
```

**Option 3: Background Sync (Best for Low Overhead)**
```go
// internal/agent/agent.go
func (a *Agent) Start(ctx context.Context) error {
    // ... existing startup logic

    // NEW: Background RAG sync
    if a.config.RAG.Enabled {
        go a.ragSyncLoop(ctx)
    }
}

func (a *Agent) ragSyncLoop(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Fetch recent reports from cache/database
            reports := a.cache.GetRecentReports(100)

            // Ingest to RAG
            for _, report := range reports {
                a.ragManager.IngestReport(ctx, report)
            }
        }
    }
}
```

### Viability Assessment

**Overall Viability: ✅ HIGH**

**Strengths:**
- ✅ TOON format already optimized for LLM context windows
- ✅ PromptBuilder architecture easily extensible
- ✅ Structured data collection (CheckResult, Metrics)
- ✅ Memory footprint within acceptable limits (114-308 MB projected)
- ✅ Streaming design (AI responses, log parsing)
- ✅ Disk-based caching (no large in-memory buffers)
- ✅ Clear integration points (Reporter, Runner, PromptBuilder)

**Challenges:**
- ⚠️ Limited log parsing capabilities (need to build LogParser)
- ⚠️ No regex usage in current code (need to add)
- ⚠️ `CheckResult.Data` is unstructured (need to standardize log entries)
- ⚠️ Embedding model selection (API vs local)

**Risks:**
- 🟢 **Low:** Memory overhead (50-100 MB is acceptable)
- 🟢 **Low:** Performance impact (ingestion async, non-blocking)
- 🟢 **Low:** Breaking existing functionality (additive, no changes to core logic)
- 🟡 **Medium:** Data quality (embeddings only as good as input data)

### Refactor Effort: **LOW-MEDIUM**

**Breakdown:**
- Design VectorStore interface: 1 day
- Chromem-go integration: 2-3 days
- LogParser implementation (syslog, JSON formats): 2-3 days
- Extend CheckResult for log entries: 1 day
- PromptBuilder RAG enhancement: 2-3 days
- Background ingestion worker: 1-2 days
- PostgreSQL schema extension (optional): 1-2 days
- Testing and validation: 3-4 days
- Documentation: 1-2 days

**Total Estimate:** 14-21 developer days

**Dependencies:**
```bash
go get github.com/philippgille/chromem-go
# Optional: Local embedding model
go get github.com/daulet/tokenizers  # For tokenization
```

**Recommendation:** **High value, prioritize early (Phase 11 or sooner)**

**Rationale:**
- Low effort, high impact
- Enhances AI analysis with historical context
- No architectural changes required
- Additive feature (no breaking changes)
- Improves user experience (better root cause analysis)
- Low operational overhead (embedded, no new infrastructure)

---

## 5. Cross-Cutting Concerns

### Platform Compatibility

| Feature | Linux | macOS | Windows | Notes |
|---------|-------|-------|---------|-------|
| **Current Code** | ✅ Full | ✅ Full | ⚠️ Partial | Checkers have platform-specific fallbacks |
| **gRPC/mTLS** | ✅ Full | ✅ Full | ✅ Full | Cross-platform library |
| **Sandboxing** | ✅ Native | ❌ No | ❌ No | Linux-only (namespaces/cgroups) |
| **Restate** | ✅ Full | ✅ Full | ✅ Full | Cross-platform SDK |
| **Chromem-go** | ✅ Full | ✅ Full | ✅ Full | Pure Go, cross-platform |

**Key Concern:** Sandboxing breaks Windows/macOS support. Consider:
- Platform-specific execution backends (sandboxed on Linux, standard on others)
- Conditional compilation with build tags

### Dependency Management

**Current Dependencies (Simplified):**
```
Go 1.25.4
github.com/spf13/cobra (CLI)
github.com/spf13/viper (config)
github.com/sirupsen/logrus (logging)
github.com/go-chi/chi/v5 (HTTP router)
github.com/lib/pq (PostgreSQL)
github.com/redis/go-redis/v9 (Redis)
k8s.io/client-go (Kubernetes)
golang.org/x/crypto/ssh (SSH)
```

**Proposed Additions:**
```
# gRPC Stack
google.golang.org/grpc
google.golang.org/protobuf
github.com/grpc-ecosystem/grpc-gateway/v2  # Optional

# Sandboxing
github.com/opencontainers/runc/libcontainer  # If using Option 2

# Restate
github.com/restatedev/sdk-go

# RAG
github.com/philippgille/chromem-go
```

**Dependency Analysis:**
- ✅ No conflicting dependencies
- ⚠️ libcontainer adds ~50MB to binary (if used)
- ⚠️ Restate requires runtime infrastructure (Restate server)
- ✅ Chromem-go is lightweight (pure Go)

### Testing Strategy

**Current Coverage:** 66.7% (52 test files)

**Proposed Testing for Modernization:**

**gRPC:**
- Unit: gRPC handler tests (mock business logic)
- Integration: gRPC client/server roundtrip tests
- mTLS: Certificate validation tests

**Sandboxing:**
- Unit: Namespace creation, cgroup limits
- Integration: Full execution with sandboxing enabled
- Security: Escape attempt tests

**Restate:**
- Unit: Workflow logic tests (mock Restate context)
- Integration: End-to-end workflow tests with Restate runtime
- Reliability: Crash recovery tests

**RAG:**
- Unit: Vector store operations, embedding generation
- Integration: End-to-end RAG query tests
- Quality: Similarity matching accuracy tests

### Performance Implications

| Component | Latency Impact | Throughput Impact | Memory Impact |
|-----------|----------------|-------------------|---------------|
| **gRPC** | 🟢 -10 to -20% | 🟢 +20 to +50% | 🟢 Negligible |
| **Sandboxing** | 🟡 +5 to +15% | 🟡 -10 to -20% | 🟢 +10-20 MB |
| **Restate** | 🟡 +10 to +30% | 🟢 Better under load | 🟡 +50-100 MB |
| **Chromem-go** | 🟡 +50-200ms (query) | 🟢 Negligible | 🟡 +50-100 MB |

**Key Observations:**
- gRPC: Faster than HTTP/JSON (binary protocol, HTTP/2)
- Sandboxing: Adds overhead (namespace creation, context switches)
- Restate: Adds checkpointing overhead, but improves reliability
- RAG: Query latency acceptable for analysis use case

### Security Enhancements

**Current Security:**
- ✅ API key authentication
- ✅ JWT tokens for agents
- ✅ TLS support
- ✅ Command injection protection (shell quoting, path sanitization)
- ✅ SSH key-based auth

**Proposed Enhancements:**

**gRPC/mTLS:**
- ✅ Mutual authentication (both client and server certificates)
- ✅ Certificate-based authorization
- ✅ Automatic certificate rotation (with cert-manager)

**Sandboxing:**
- ✅ Reduced attack surface (namespace isolation)
- ✅ Resource limits (prevents DoS)
- ⚠️ Complexity (new attack vectors in namespace handling)

**Restate:**
- ✅ Built-in audit trail (all workflow steps logged)
- ✅ Idempotency (prevents duplicate execution)
- ⚠️ New infrastructure to secure (Restate server)

**RAG:**
- ⚠️ Data privacy (historical logs stored locally)
- ⚠️ Embedding data leakage (if using external API)
- ✅ Local deployment option (Chromem-go, no external calls)

---

## 6. Migration Roadmap

### Phased Approach (Recommended)

**Phase A: Transport Modernization (6-8 weeks)**
1. Implement gRPC service definitions (Protocol Buffers)
2. Create gRPC server implementation (reuse business logic)
3. Implement gRPC interceptors (auth, logging, recovery)
4. Set up mTLS with cert-manager
5. Deploy dual-protocol mode (HTTP + gRPC side-by-side)
6. Migrate agents to gRPC clients
7. Deprecate HTTP endpoints

**Deliverables:**
- gRPC API server
- mTLS authentication
- Updated agent communication
- Performance benchmarks (gRPC vs HTTP)

**Phase B: Intelligence Enhancement (4-6 weeks)**
1. Design VectorStore interface
2. Integrate Chromem-go
3. Implement LogParser (syslog, JSON formats)
4. Extend CheckResult for structured log entries
5. Enhance PromptBuilder with RAG context
6. Implement background ingestion worker
7. Add PostgreSQL schema for historical storage (optional)

**Deliverables:**
- Local RAG system with Chromem-go
- Enhanced AI analysis with historical context
- Log parsing capabilities
- Performance metrics (query latency, memory usage)

**Phase C: Reliability Upgrade (6-8 weeks)**
1. Integrate Restate SDK
2. Migrate diagnostic workflow to Restate
3. Migrate remediation workflow with durable approval
4. Replace cron scheduler with Restate timers
5. Implement agent heartbeat workflows
6. Remove unbounded goroutine spawning (job queue)
7. Migrate agents to Restate clients

**Deliverables:**
- Restate-based durable execution
- Crash-resistant workflows
- Human-in-the-loop approval with durability
- Distributed agent coordination

**Phase D: Execution Hardening (4-6 weeks) [Optional]**
1. Design SandboxedExecutor interface
2. Implement user/PID namespace support
3. Integrate cgroup resource limits
4. Test all 12 checkers with sandboxing
5. Handle bind mounts for /proc, /sys
6. Add fallback for unsupported platforms
7. Security audit and penetration testing

**Deliverables:**
- Sandboxed command execution (Linux)
- Resource limit enforcement
- Security audit report
- Performance impact analysis

### Incremental Adoption Strategy

**Dual-Protocol Pattern:**
```go
type DiagnosticsService struct {
    runner *diagnostics.Runner
}

// Shared business logic
func (s *DiagnosticsService) Execute(ctx context.Context, req DiagnosticsRequest) (*DiagnosticsResult, error) {
    return s.runner.Run(ctx, req)
}

// HTTP handler (existing)
func (h *HTTPHandler) RunDiagnostics(w http.ResponseWriter, r *http.Request) {
    result, err := h.service.Execute(r.Context(), parseRequest(r))
    json.NewEncoder(w).Encode(result)
}

// gRPC handler (new)
func (g *GRPCHandler) RunDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.JobResponse, error) {
    result, err := g.service.Execute(ctx, fromProto(req))
    return toProto(result), err
}
```

**Feature Flags:**
```yaml
# config.yaml
experimental:
  grpc_enabled: true
  grpc_port: 9443
  sandboxing_enabled: false  # Gradual rollout
  restate_enabled: false
  rag_enabled: true
```

### Rollback Strategy

**Per-Component Rollback:**
- **gRPC:** Keep HTTP endpoints active, switch agents back
- **Sandboxing:** Disable via config flag, fallback to standard executor
- **Restate:** Revert to PostgreSQL job queue, remove Restate calls
- **RAG:** Disable via config flag, PromptBuilder ignores RAG context

**Database Migrations:**
- Use reversible migrations (goose supports up/down)
- Test rollback in staging before production

### Success Metrics

**gRPC Migration:**
- ✅ All agents successfully connected via gRPC
- ✅ Latency reduced by 10-20%
- ✅ Throughput increased by 20-50%
- ✅ mTLS authentication working for all connections
- ✅ Zero HTTP-related errors after migration

**RAG Integration:**
- ✅ 100+ historical incidents ingested
- ✅ Query latency <500ms for similarity search
- ✅ Memory footprint <150 MB for RAG system
- ✅ AI analysis quality improved (user feedback)
- ✅ Relevant historical context retrieved in 80%+ of cases

**Restate Adoption:**
- ✅ All critical workflows migrated (diagnostics, remediation)
- ✅ Zero job loss after agent crash/restart
- ✅ Approval workflows survive multi-day waits
- ✅ Reduced operational alerts (automatic recovery)
- ✅ Workflow execution time reduced by 20-30% under load

**Sandboxing (Optional):**
- ✅ All 12 checkers work with sandboxing enabled
- ✅ Resource limit enforcement working (memory, CPU)
- ✅ No security vulnerabilities found in audit
- ✅ Performance overhead <20%
- ✅ Graceful fallback on unsupported platforms

---

## 7. Risk Assessment Matrix

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| **gRPC breaks existing clients** | Medium | High | Dual-protocol mode, gradual migration |
| **Sandboxing breaks checkers** | High | High | Extensive testing, platform-specific fallbacks |
| **Restate infrastructure failure** | Low | High | Keep PostgreSQL job queue as fallback |
| **RAG memory overhead** | Low | Medium | Configurable limits, monitoring |
| **Performance degradation** | Low | Medium | Benchmarking, gradual rollout with flags |
| **Increased operational complexity** | Medium | Medium | Comprehensive documentation, training |
| **Dependency conflicts** | Low | Low | Careful dependency management, go.mod checks |
| **Cross-platform compatibility loss** | Medium | Medium | Conditional compilation, platform checks |

---

## 8. Final Recommendations

### Priority Ranking

1. **🥇 gRPC + mTLS (High Priority)**
   - Effort: Medium (10-15 days)
   - Value: High (performance, security, scalability)
   - Risk: Low (well-isolated, proven technology)
   - Dependency: None (can start immediately)

2. **🥈 Local RAG (High Priority)**
   - Effort: Low-Medium (14-21 days)
   - Value: High (better AI analysis, user experience)
   - Risk: Low (additive, no breaking changes)
   - Dependency: None (can parallel with gRPC)

3. **🥉 Restate (Medium Priority)**
   - Effort: Medium-High (20-29 days)
   - Value: High (reliability, durability, simplified coordination)
   - Risk: Medium (new infrastructure, learning curve)
   - Dependency: Consider after gRPC (workflows via gRPC)

4. **4️⃣ Sandboxing (Low Priority)**
   - Effort: High (17-25 days)
   - Value: Medium (security hardening, limited benefit)
   - Risk: Medium-High (breaking changes, platform-specific)
   - Dependency: Defer to Phase 12+ (Security Hardening)

### Recommended Sequence

**Quarter 1:**
- gRPC migration (Weeks 1-4)
- RAG integration (Weeks 5-7)
- Testing and stabilization (Week 8)

**Quarter 2:**
- Restate integration (Weeks 1-5)
- Performance optimization (Weeks 6-7)
- Documentation and training (Week 8)

**Quarter 3+ (Optional):**
- Sandboxing (if required by security policy)
- Advanced features (predictive analysis, anomaly detection)

### Architecture Decision Records (ADRs)

**ADR-001: Adopt gRPC for Agent-API Communication**
- Status: ✅ Recommended
- Context: HTTP/JSON has overhead, lacks streaming, no native mTLS
- Decision: Migrate to gRPC with Protocol Buffers
- Consequences: Better performance, requires dual-protocol during transition

**ADR-002: Integrate Chromem-go for Local RAG**
- Status: ✅ Recommended
- Context: AI analysis lacks historical context
- Decision: Embed Chromem-go for local vector search
- Consequences: Enhanced analysis quality, +50-100MB memory

**ADR-003: Adopt Restate for Durable Execution**
- Status: ⚠️ Consider (after gRPC/RAG)
- Context: No crash recovery, unbounded goroutines, complex coordination
- Decision: Migrate critical workflows to Restate
- Consequences: Improved reliability, new infrastructure dependency

**ADR-004: Defer Sandboxing to Phase 12+**
- Status: ⏳ Deferred
- Context: High effort, platform-specific, limited security gain (already unprivileged)
- Decision: Defer to security hardening phase
- Consequences: Maintain current execution model, revisit if needed

---

## 9. Conclusion

Lumo's codebase is **well-architected for modernization**, with excellent separation of concerns and minimal coupling between layers. The proposed 2025 stack is **viable** with varying effort levels:

- **gRPC/mTLS:** High viability, medium effort, high value → **Prioritize**
- **Local RAG:** High viability, low-medium effort, high value → **Prioritize**
- **Restate:** High viability, medium-high effort, high value → **Consider after gRPC/RAG**
- **Sandboxing:** Moderate viability, high effort, medium value → **Defer**

**Key Strengths:**
- 85%+ of business logic is transport-agnostic
- Clean executor abstraction
- Structured data collection
- Memory-efficient design
- Comprehensive testing foundation

**Key Challenges:**
- ~2,200 LOC API layer rewrite for gRPC
- Limited log parsing capabilities for RAG
- Sandboxing breaks cross-platform support
- New infrastructure dependencies (Restate, mTLS cert management)

**Overall Assessment:** The modernization is **RECOMMENDED** with an incremental approach. Start with gRPC and RAG for quick wins, then evaluate Restate adoption based on operational needs. Defer sandboxing unless security requirements mandate it.

**Estimated Total Effort:**
- gRPC: 10-15 days
- RAG: 14-21 days
- Restate: 20-29 days (optional)
- Sandboxing: 17-25 days (defer)

**Total: 44-65 developer days** for gRPC + RAG (recommended minimum)
**Total: 64-94 developer days** for full stack (gRPC + RAG + Restate)
**Total: 81-119 developer days** for complete migration (including sandboxing)

---

**Report Generated:** 2025-11-20
**Codebase Version:** Phase 10 Complete
**Assessment Scope:** Transport, Execution, State, Intelligence
**Next Steps:** Review with team, prioritize based on business needs, create detailed implementation plans
