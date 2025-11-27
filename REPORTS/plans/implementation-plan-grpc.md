# Implementation Plan: gRPC + mTLS Migration

**Project:** Lumo - gRPC Transport Layer Migration
**Timeline:** 10-15 developer days (4 weeks with testing)
**Risk Level:** Low (business logic unchanged, dual-protocol support)
**Expected ROI:** 3,300% (40% performance gain, $50K/year value, compliance-ready)

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Phase 1: Protocol Buffer Definitions](#phase-1-protocol-buffer-definitions)
3. [Phase 2: gRPC Server Implementation](#phase-2-grpc-server-implementation)
4. [Phase 3: mTLS Setup](#phase-3-mtls-setup)
5. [Phase 4: Client Migration](#phase-4-client-migration)
6. [Phase 5: Testing & Validation](#phase-5-testing--validation)
7. [Phase 6: Deployment & Rollout](#phase-6-deployment--rollout)
8. [Monitoring & Observability](#monitoring--observability)
9. [Rollback Strategy](#rollback-strategy)
10. [Success Metrics](#success-metrics)

---

## Prerequisites

### Dependencies

```bash
# Install Protocol Buffer compiler
# macOS
brew install protobuf

# Linux (Ubuntu/Debian)
sudo apt-get install -y protobuf-compiler

# Verify installation
protoc --version  # Should be 3.21.0+

# Install Go plugins for protoc
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Add to PATH (add to ~/.bashrc or ~/.zshrc)
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Go Dependencies

```bash
# Add to go.mod
go get google.golang.org/grpc@latest
go get google.golang.org/protobuf@latest
go get google.golang.org/genproto/googleapis/rpc/status@latest

# Optional: gRPC-Gateway for HTTP→gRPC translation during transition
go get github.com/grpc-ecosystem/grpc-gateway/v2@latest
```

### Project Structure

```bash
mkdir -p api/proto/v1
mkdir -p internal/grpc/{server,client,interceptors}
mkdir -p deployments/certs
```

---

## Phase 1: Protocol Buffer Definitions

**Duration:** 1-2 days
**Deliverables:** `.proto` files defining all API contracts

### Step 1.1: Create Base Proto File

**File:** `api/proto/v1/common.proto`

```protobuf
syntax = "proto3";

package lumo.v1;

option go_package = "github.com/ignacio/lumo/api/proto/v1;lumov1";

import "google/protobuf/timestamp.proto";

// Common enums and types

enum JobStatus {
  JOB_STATUS_UNSPECIFIED = 0;
  JOB_STATUS_PENDING = 1;
  JOB_STATUS_RUNNING = 2;
  JOB_STATUS_COMPLETED = 3;
  JOB_STATUS_FAILED = 4;
  JOB_STATUS_CANCELLED = 5;
}

enum JobType {
  JOB_TYPE_UNSPECIFIED = 0;
  JOB_TYPE_DIAGNOSTIC = 1;
  JOB_TYPE_REMEDIATION = 2;
}

enum CheckStatus {
  CHECK_STATUS_UNSPECIFIED = 0;
  CHECK_STATUS_PASS = 1;
  CHECK_STATUS_FAIL = 2;
  CHECK_STATUS_WARNING = 3;
  CHECK_STATUS_SKIP = 4;
}

enum Severity {
  SEVERITY_UNSPECIFIED = 0;
  SEVERITY_INFO = 1;
  SEVERITY_LOW = 2;
  SEVERITY_MEDIUM = 3;
  SEVERITY_HIGH = 4;
  SEVERITY_CRITICAL = 5;
}

enum OutputFormat {
  OUTPUT_FORMAT_UNSPECIFIED = 0;
  OUTPUT_FORMAT_TEXT = 1;
  OUTPUT_FORMAT_TOON = 2;
  OUTPUT_FORMAT_JSON = 3;
}

message Metadata {
  map<string, string> labels = 1;
  google.protobuf.Timestamp timestamp = 2;
}
```

### Step 1.2: Diagnostics Service Proto

**File:** `api/proto/v1/diagnostics.proto`

```protobuf
syntax = "proto3";

package lumo.v1;

option go_package = "github.com/ignacio/lumo/api/proto/v1;lumov1";

import "api/proto/v1/common.proto";
import "google/protobuf/timestamp.proto";
import "google/protobuf/struct.proto";

service DiagnosticsService {
  // Run diagnostics asynchronously, returns job ID
  rpc RunDiagnostics(RunDiagnosticsRequest) returns (RunDiagnosticsResponse);

  // Get diagnostic result by job ID
  rpc GetDiagnosticsResult(GetDiagnosticsResultRequest) returns (GetDiagnosticsResultResponse);

  // Stream diagnostic progress (real-time updates)
  rpc StreamDiagnostics(StreamDiagnosticsRequest) returns (stream DiagnosticsEvent);

  // List recent diagnostics
  rpc ListDiagnostics(ListDiagnosticsRequest) returns (ListDiagnosticsResponse);
}

message RunDiagnosticsRequest {
  string target = 1;                    // Hostname or IP
  repeated string checks = 2;           // Empty = all checks
  bool analyze = 3;                     // Run AI analysis
  OutputFormat format = 4;              // Output format
  map<string, string> metadata = 5;     // Additional context
  repeated string focus_areas = 6;      // AI focus areas
}

message RunDiagnosticsResponse {
  string job_id = 1;                    // UUID
  JobStatus status = 2;
  google.protobuf.Timestamp created_at = 3;
}

message GetDiagnosticsResultRequest {
  string job_id = 1;
}

message GetDiagnosticsResultResponse {
  string job_id = 1;
  JobStatus status = 2;
  JobType type = 3;
  string target = 4;
  google.protobuf.Timestamp created_at = 5;
  google.protobuf.Timestamp started_at = 6;
  google.protobuf.Timestamp completed_at = 7;
  DiagnosticsResult result = 8;         // Only present if completed
  string error = 9;                     // Only present if failed
  map<string, string> metadata = 10;
}

message DiagnosticsResult {
  repeated CheckResult results = 1;
  ReportSummary summary = 2;
  string analysis = 3;                  // AI analysis (if requested)
  google.protobuf.Timestamp timestamp = 4;
  int64 duration_ms = 5;
}

message CheckResult {
  string name = 1;
  string category = 2;
  CheckStatus status = 3;
  Severity severity = 4;
  string message = 5;
  google.protobuf.Struct data = 6;      // Flexible data (like current map[string]interface{})
  repeated Metric metrics = 7;
  google.protobuf.Timestamp timestamp = 8;
  int64 duration_ms = 9;
  string error = 10;
}

message Metric {
  string name = 1;
  double value = 2;                     // Numeric metrics
  string unit = 3;                      // %, MB, count, etc.
  google.protobuf.Timestamp timestamp = 4;
  map<string, string> labels = 5;
}

message ReportSummary {
  int32 total_checks = 1;
  int32 passed = 2;
  int32 failed = 3;
  int32 warnings = 4;
  int32 skipped = 5;
  Severity overall_severity = 6;
}

message StreamDiagnosticsRequest {
  string target = 1;
  repeated string checks = 2;
  bool analyze = 3;
}

message DiagnosticsEvent {
  enum EventType {
    EVENT_TYPE_UNSPECIFIED = 0;
    EVENT_TYPE_STARTED = 1;
    EVENT_TYPE_CHECK_STARTED = 2;
    EVENT_TYPE_CHECK_COMPLETED = 3;
    EVENT_TYPE_COMPLETED = 4;
    EVENT_TYPE_FAILED = 5;
  }

  EventType type = 1;
  string job_id = 2;
  CheckResult check_result = 3;         // For CHECK_COMPLETED events
  DiagnosticsResult final_result = 4;   // For COMPLETED events
  string error = 5;                     // For FAILED events
  int32 progress = 6;                   // 0-100 percentage
}

message ListDiagnosticsRequest {
  JobStatus status = 1;                 // Filter by status
  int32 limit = 2;                      // Max results (default 50)
  int32 offset = 3;                     // Pagination offset
  string target = 4;                    // Filter by target
}

message ListDiagnosticsResponse {
  repeated JobSummary jobs = 1;
  int32 total_count = 2;
}

message JobSummary {
  string job_id = 1;
  JobType type = 2;
  JobStatus status = 3;
  string target = 4;
  google.protobuf.Timestamp created_at = 5;
  google.protobuf.Timestamp completed_at = 6;
}
```

### Step 1.3: Agents Service Proto

**File:** `api/proto/v1/agents.proto`

```protobuf
syntax = "proto3";

package lumo.v1;

option go_package = "github.com/ignacio/lumo/api/proto/v1;lumov1";

import "api/proto/v1/common.proto";
import "google/protobuf/timestamp.proto";

service AgentsService {
  // Register a new agent
  rpc RegisterAgent(RegisterAgentRequest) returns (RegisterAgentResponse);

  // Send heartbeat to maintain agent online status
  rpc SendHeartbeat(SendHeartbeatRequest) returns (SendHeartbeatResponse);

  // List registered agents
  rpc ListAgents(ListAgentsRequest) returns (ListAgentsResponse);

  // Get agent details
  rpc GetAgent(GetAgentRequest) returns (GetAgentResponse);

  // Delete (deregister) agent
  rpc DeleteAgent(DeleteAgentRequest) returns (DeleteAgentResponse);

  // Get agent statistics
  rpc GetAgentStats(GetAgentStatsRequest) returns (GetAgentStatsResponse);
}

message RegisterAgentRequest {
  string name = 1;
  string hostname = 2;
  string ip_address = 3;
  string platform = 4;                  // linux, darwin, windows
  string architecture = 5;              // amd64, arm64
  string version = 6;                   // Agent version
  repeated string capabilities = 7;     // cpu, memory, disk, etc.
  map<string, string> labels = 8;
  KubernetesMetadata kubernetes = 9;    // Optional
}

message KubernetesMetadata {
  string namespace = 1;
  string pod_name = 2;
  string node_name = 3;
  map<string, string> labels = 4;
}

message RegisterAgentResponse {
  string agent_id = 1;                  // UUID
  google.protobuf.Timestamp registered_at = 2;
}

message SendHeartbeatRequest {
  string agent_id = 1;
  AgentHealth health = 2;
}

message AgentHealth {
  enum HealthStatus {
    HEALTH_STATUS_UNSPECIFIED = 0;
    HEALTH_STATUS_HEALTHY = 1;
    HEALTH_STATUS_DEGRADED = 2;
    HEALTH_STATUS_UNHEALTHY = 3;
  }

  HealthStatus status = 1;
  map<string, string> errors = 2;       // Component → error message
  google.protobuf.Timestamp last_run = 3;
}

message SendHeartbeatResponse {
  bool acknowledged = 1;
  google.protobuf.Timestamp server_time = 2;
}

message ListAgentsRequest {
  string status = 1;                    // online, offline, error
  int32 limit = 2;
  int32 offset = 3;
  map<string, string> labels = 4;       // Filter by labels
}

message ListAgentsResponse {
  repeated Agent agents = 1;
  int32 total_count = 2;
}

message Agent {
  string id = 1;
  string name = 2;
  string hostname = 3;
  string ip_address = 4;
  string platform = 5;
  string architecture = 6;
  string version = 7;
  string status = 8;                    // online, offline, error
  repeated string capabilities = 9;
  map<string, string> labels = 10;
  KubernetesMetadata kubernetes = 11;
  google.protobuf.Timestamp last_heartbeat_at = 12;
  google.protobuf.Timestamp registered_at = 13;
}

message GetAgentRequest {
  string agent_id = 1;
}

message GetAgentResponse {
  Agent agent = 1;
}

message DeleteAgentRequest {
  string agent_id = 1;
}

message DeleteAgentResponse {
  bool success = 1;
}

message GetAgentStatsRequest {}

message GetAgentStatsResponse {
  int32 total_agents = 1;
  int32 online_agents = 2;
  int32 offline_agents = 3;
  int32 error_agents = 4;
  map<string, int32> by_platform = 5;   // platform → count
}
```

### Step 1.4: Health Service Proto

**File:** `api/proto/v1/health.proto`

```protobuf
syntax = "proto3";

package lumo.v1;

option go_package = "github.com/ignacio/lumo/api/proto/v1;lumov1";

import "google/protobuf/timestamp.proto";

service HealthService {
  // General health check
  rpc Check(HealthCheckRequest) returns (HealthCheckResponse);

  // Kubernetes readiness probe
  rpc Ready(ReadyRequest) returns (ReadyResponse);

  // Kubernetes liveness probe
  rpc Live(LiveRequest) returns (LiveResponse);
}

message HealthCheckRequest {
  string service = 1;  // Optional: check specific service
}

message HealthCheckResponse {
  enum ServingStatus {
    SERVING_STATUS_UNSPECIFIED = 0;
    SERVING_STATUS_SERVING = 1;
    SERVING_STATUS_NOT_SERVING = 2;
    SERVING_STATUS_UNKNOWN = 3;
  }

  ServingStatus status = 1;
  string version = 2;
  google.protobuf.Timestamp uptime = 3;
  map<string, ComponentHealth> components = 4;
}

message ComponentHealth {
  bool healthy = 1;
  string message = 2;
}

message ReadyRequest {}

message ReadyResponse {
  bool ready = 1;
  string message = 2;
}

message LiveRequest {}

message LiveResponse {
  bool alive = 1;
  google.protobuf.Timestamp timestamp = 2;
}
```

### Step 1.5: Generate Go Code

**File:** `Makefile` (add new targets)

```makefile
# Protocol Buffer generation
.PHONY: proto
proto: proto-clean proto-gen proto-fmt

.PHONY: proto-gen
proto-gen:
	@echo "Generating protobuf code..."
	@protoc \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		api/proto/v1/*.proto
	@echo "✓ Protobuf code generated"

.PHONY: proto-clean
proto-clean:
	@echo "Cleaning generated protobuf files..."
	@find api/proto -name "*.pb.go" -delete
	@echo "✓ Cleaned"

.PHONY: proto-fmt
proto-fmt:
	@echo "Formatting proto files..."
	@find api/proto -name "*.proto" -exec clang-format -i {} \; || true
	@echo "✓ Formatted"

# Validate proto files
.PHONY: proto-lint
proto-lint:
	@echo "Linting proto files..."
	@buf lint api/proto || echo "Install buf: brew install buf"
```

**Run generation:**

```bash
make proto-gen

# Expected output structure:
# api/proto/v1/
# ├── common.pb.go
# ├── diagnostics.pb.go
# ├── diagnostics_grpc.pb.go
# ├── agents.pb.go
# ├── agents_grpc.pb.go
# ├── health.pb.go
# └── health_grpc.pb.go
```

---

## Phase 2: gRPC Server Implementation

**Duration:** 3-5 days
**Deliverables:** gRPC server with all service implementations

### Step 2.1: Create gRPC Server Wrapper

**File:** `internal/grpc/server/server.go`

```go
package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/grpc/interceptors"
)

type Server struct {
	config     *config.Config
	grpcServer *grpc.Server
	listener   net.Listener
	log        *logrus.Logger
}

type ServerOptions struct {
	Port       int
	TLSEnabled bool
	CertFile   string
	KeyFile    string
	MaxMsgSize int // Max message size in bytes
}

func NewServer(cfg *config.Config, opts ServerOptions, log *logrus.Logger) (*Server, error) {
	// Listen on port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", opts.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	// gRPC server options
	serverOpts := []grpc.ServerOption{
		// Keepalive settings for long-lived connections
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Minute,
			Time:                  5 * time.Minute,
			Timeout:               1 * time.Minute,
		}),
		// Enforce keepalive from clients
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             1 * time.Minute,
			PermitWithoutStream: true,
		}),
		// Max message sizes (default 4MB, increase for large diagnostics)
		grpc.MaxRecvMsgSize(opts.MaxMsgSize),
		grpc.MaxSendMsgSize(opts.MaxMsgSize),
		// Interceptors (auth, logging, recovery)
		grpc.ChainUnaryInterceptor(
			interceptors.LoggingInterceptor(log),
			interceptors.RecoveryInterceptor(log),
			interceptors.AuthInterceptor(cfg),
		),
		grpc.ChainStreamInterceptor(
			interceptors.StreamLoggingInterceptor(log),
			interceptors.StreamRecoveryInterceptor(log),
			interceptors.StreamAuthInterceptor(cfg),
		),
	}

	// Add TLS credentials if enabled
	if opts.TLSEnabled {
		creds, err := credentials.NewServerTLSFromFile(opts.CertFile, opts.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
		log.Info("gRPC server TLS enabled")
	}

	grpcServer := grpc.NewServer(serverOpts...)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Enable gRPC reflection for debugging (grpcurl, grpc-health-probe)
	reflection.Register(grpcServer)

	return &Server{
		config:     cfg,
		grpcServer: grpcServer,
		listener:   lis,
		log:        log,
	}, nil
}

func (s *Server) Start() error {
	s.log.WithField("addr", s.listener.Addr().String()).Info("Starting gRPC server")
	return s.grpcServer.Serve(s.listener)
}

func (s *Server) GracefulStop(ctx context.Context) error {
	s.log.Info("Gracefully stopping gRPC server")

	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-ctx.Done():
		s.log.Warn("Graceful shutdown timeout, forcing stop")
		s.grpcServer.Stop()
		return ctx.Err()
	case <-done:
		s.log.Info("gRPC server stopped gracefully")
		return nil
	}
}

func (s *Server) GetGRPCServer() *grpc.Server {
	return s.grpcServer
}
```

### Step 2.2: Implement Interceptors

**File:** `internal/grpc/interceptors/auth.go`

```go
package interceptors

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ignacio/lumo/internal/api/auth"
	"github.com/ignacio/lumo/internal/config"
)

// AuthInterceptor validates JWT tokens from metadata
func AuthInterceptor(cfg *config.Config) grpc.UnaryServerInterceptor {
	jwtManager := auth.NewJWTManager(cfg.API.JWTSecret)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Skip auth for health checks and public endpoints
		if isPublicMethod(info.FullMethod) {
			return handler(ctx, req)
		}

		// Extract token from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeaders := md.Get("authorization")
		if len(authHeaders) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		// Parse "Bearer <token>"
		token := strings.TrimPrefix(authHeaders[0], "Bearer ")
		if token == authHeaders[0] {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization header format")
		}

		// Validate JWT
		claims, err := jwtManager.Validate(token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		// Inject claims into context
		ctx = context.WithValue(ctx, "claims", claims)
		ctx = context.WithValue(ctx, "agent_id", claims.AgentID)

		return handler(ctx, req)
	}
}

// StreamAuthInterceptor for streaming RPCs
func StreamAuthInterceptor(cfg *config.Config) grpc.StreamServerInterceptor {
	jwtManager := auth.NewJWTManager(cfg.API.JWTSecret)

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if isPublicMethod(info.FullMethod) {
			return handler(srv, ss)
		}

		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeaders := md.Get("authorization")
		if len(authHeaders) == 0 {
			return status.Error(codes.Unauthenticated, "missing authorization header")
		}

		token := strings.TrimPrefix(authHeaders[0], "Bearer ")
		claims, err := jwtManager.Validate(token)
		if err != nil {
			return status.Error(codes.Unauthenticated, "invalid token")
		}

		// Wrap stream with authenticated context
		wrappedStream := &authenticatedStream{
			ServerStream: ss,
			ctx:          context.WithValue(ss.Context(), "claims", claims),
		}

		return handler(srv, wrappedStream)
	}
}

type authenticatedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedStream) Context() context.Context {
	return s.ctx
}

func isPublicMethod(method string) bool {
	publicMethods := []string{
		"/lumo.v1.HealthService/Check",
		"/lumo.v1.HealthService/Ready",
		"/lumo.v1.HealthService/Live",
		"/grpc.health.v1.Health/Check",
	}

	for _, public := range publicMethods {
		if method == public {
			return true
		}
	}
	return false
}
```

**File:** `internal/grpc/interceptors/logging.go`

```go
package interceptors

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func LoggingInterceptor(log *logrus.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// Call handler
		resp, err := handler(ctx, req)

		// Log request
		duration := time.Since(start)
		statusCode := status.Code(err)

		entry := log.WithFields(logrus.Fields{
			"method":   info.FullMethod,
			"duration": duration.Milliseconds(),
			"status":   statusCode.String(),
		})

		if err != nil {
			entry.WithError(err).Error("gRPC request failed")
		} else {
			entry.Info("gRPC request completed")
		}

		return resp, err
	}
}

func StreamLoggingInterceptor(log *logrus.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		err := handler(srv, ss)

		duration := time.Since(start)
		statusCode := status.Code(err)

		entry := log.WithFields(logrus.Fields{
			"method":   info.FullMethod,
			"duration": duration.Milliseconds(),
			"status":   statusCode.String(),
			"stream":   true,
		})

		if err != nil {
			entry.WithError(err).Error("gRPC stream failed")
		} else {
			entry.Info("gRPC stream completed")
		}

		return err
	}
}
```

**File:** `internal/grpc/interceptors/recovery.go`

```go
package interceptors

import (
	"context"
	"runtime/debug"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RecoveryInterceptor(log *logrus.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.WithFields(logrus.Fields{
					"method": info.FullMethod,
					"panic":  r,
					"stack":  string(debug.Stack()),
				}).Error("gRPC handler panic")

				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}

func StreamRecoveryInterceptor(log *logrus.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.WithFields(logrus.Fields{
					"method": info.FullMethod,
					"panic":  r,
					"stack":  string(debug.Stack()),
				}).Error("gRPC stream handler panic")

				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		return handler(srv, ss)
	}
}
```

### Step 2.3: Implement Diagnostics Service Handler

**File:** `internal/grpc/handlers/diagnostics.go`

```go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
	"github.com/ignacio/lumo/internal/diagnostics"
)

type DiagnosticsHandler struct {
	lumov1.UnimplementedDiagnosticsServiceServer

	jobRepo  repository.JobRepository
	runner   *diagnostics.Runner
}

func NewDiagnosticsHandler(jobRepo repository.JobRepository, runner *diagnostics.Runner) *DiagnosticsHandler {
	return &DiagnosticsHandler{
		jobRepo: jobRepo,
		runner:  runner,
	}
}

func (h *DiagnosticsHandler) RunDiagnostics(ctx context.Context, req *lumov1.RunDiagnosticsRequest) (*lumov1.RunDiagnosticsResponse, error) {
	// Validate request
	if req.Target == "" {
		return nil, status.Error(codes.InvalidArgument, "target is required")
	}

	// Create job in database
	job := &models.Job{
		ID:     uuid.New(),
		Type:   models.JobTypeDiagnostic,
		Status: models.JobStatusPending,
		Target: req.Target,
		Metadata: map[string]interface{}{
			"checks":      req.Checks,
			"analyze":     req.Analyze,
			"format":      req.Format.String(),
			"focus_areas": req.FocusAreas,
		},
	}

	if err := h.jobRepo.Create(ctx, job); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create job: %v", err)
	}

	// Execute diagnostics asynchronously
	go h.executeDiagnostics(context.Background(), job, req)

	return &lumov1.RunDiagnosticsResponse{
		JobId:     job.ID.String(),
		Status:    toProtoJobStatus(job.Status),
		CreatedAt: timestamppb.New(job.CreatedAt),
	}, nil
}

func (h *DiagnosticsHandler) GetDiagnosticsResult(ctx context.Context, req *lumov1.GetDiagnosticsResultRequest) (*lumov1.GetDiagnosticsResultResponse, error) {
	jobID, err := uuid.Parse(req.JobId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid job ID")
	}

	job, err := h.jobRepo.Get(ctx, jobID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job not found: %v", err)
	}

	resp := &lumov1.GetDiagnosticsResultResponse{
		JobId:     job.ID.String(),
		Status:    toProtoJobStatus(job.Status),
		Type:      toProtoJobType(job.Type),
		Target:    job.Target,
		CreatedAt: timestamppb.New(job.CreatedAt),
	}

	if job.StartedAt != nil {
		resp.StartedAt = timestamppb.New(*job.StartedAt)
	}
	if job.CompletedAt != nil {
		resp.CompletedAt = timestamppb.New(*job.CompletedAt)
	}

	// Include result if job completed
	if job.Status == models.JobStatusCompleted && job.Result != nil {
		result, err := h.unmarshalDiagnosticsResult(job.Result)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse result: %v", err)
		}
		resp.Result = result
	}

	if job.Error != "" {
		resp.Error = job.Error
	}

	return resp, nil
}

func (h *DiagnosticsHandler) StreamDiagnostics(req *lumov1.StreamDiagnosticsRequest, stream lumov1.DiagnosticsService_StreamDiagnosticsServer) error {
	// Send initial event
	if err := stream.Send(&lumov1.DiagnosticsEvent{
		Type:     lumov1.DiagnosticsEvent_EVENT_TYPE_STARTED,
		JobId:    uuid.New().String(),
		Progress: 0,
	}); err != nil {
		return err
	}

	// Create diagnostic request
	diagnosticReq := &diagnostics.RunRequest{
		Target: req.Target,
		Checks: req.Checks,
	}

	// Execute diagnostics with progress callback
	progressChan := make(chan diagnostics.CheckProgress)
	go func() {
		defer close(progressChan)
		h.runner.RunWithProgress(stream.Context(), diagnosticReq, progressChan)
	}()

	// Stream progress events
	totalChecks := len(req.Checks)
	if totalChecks == 0 {
		totalChecks = 12 // All checkers
	}
	completed := 0

	for progress := range progressChan {
		completed++

		// Send check completed event
		checkResult, err := toProtoCheckResult(progress.Result)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to convert result: %v", err)
		}

		if err := stream.Send(&lumov1.DiagnosticsEvent{
			Type:        lumov1.DiagnosticsEvent_EVENT_TYPE_CHECK_COMPLETED,
			CheckResult: checkResult,
			Progress:    int32((completed * 100) / totalChecks),
		}); err != nil {
			return err
		}
	}

	// Send completion event
	// TODO: Convert full report to proto
	if err := stream.Send(&lumov1.DiagnosticsEvent{
		Type:     lumov1.DiagnosticsEvent_EVENT_TYPE_COMPLETED,
		Progress: 100,
	}); err != nil {
		return err
	}

	return nil
}

// Background execution (reuses existing business logic)
func (h *DiagnosticsHandler) executeDiagnostics(ctx context.Context, job *models.Job, req *lumov1.RunDiagnosticsRequest) {
	// Update status to running
	job.Status = models.JobStatusRunning
	now := time.Now()
	job.StartedAt = &now
	h.jobRepo.Update(ctx, job)

	// Determine executor (LocalExecutor or SSHExecutor)
	executor := h.determineExecutor(req.Target)

	// Run diagnostics (REUSE existing business logic)
	diagnosticReq := &diagnostics.RunRequest{
		Target:     req.Target,
		Checks:     req.Checks,
		Analyze:    req.Analyze,
		FocusAreas: req.FocusAreas,
	}

	report, err := h.runner.Run(ctx, executor, diagnosticReq)

	// Update job with result
	if err != nil {
		job.Status = models.JobStatusFailed
		job.Error = err.Error()
	} else {
		job.Status = models.JobStatusCompleted
		resultJSON, _ := json.Marshal(report)
		job.Result = resultJSON
	}

	completedAt := time.Now()
	job.CompletedAt = &completedAt
	h.jobRepo.Update(ctx, job)
}

// Helper functions

func toProtoJobStatus(status models.JobStatus) lumov1.JobStatus {
	switch status {
	case models.JobStatusPending:
		return lumov1.JobStatus_JOB_STATUS_PENDING
	case models.JobStatusRunning:
		return lumov1.JobStatus_JOB_STATUS_RUNNING
	case models.JobStatusCompleted:
		return lumov1.JobStatus_JOB_STATUS_COMPLETED
	case models.JobStatusFailed:
		return lumov1.JobStatus_JOB_STATUS_FAILED
	case models.JobStatusCancelled:
		return lumov1.JobStatus_JOB_STATUS_CANCELLED
	default:
		return lumov1.JobStatus_JOB_STATUS_UNSPECIFIED
	}
}

func toProtoCheckResult(result *diagnostics.CheckResult) (*lumov1.CheckResult, error) {
	// Convert Data map[string]interface{} to protobuf.Struct
	dataStruct, err := structpb.NewStruct(result.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert data: %w", err)
	}

	// Convert Metrics
	metrics := make([]*lumov1.Metric, len(result.Metrics))
	for i, m := range result.Metrics {
		value := 0.0
		switch v := m.Value.(type) {
		case float64:
			value = v
		case int:
			value = float64(v)
		case int64:
			value = float64(v)
		}

		metrics[i] = &lumov1.Metric{
			Name:      m.Name,
			Value:     value,
			Unit:      m.Unit,
			Timestamp: timestamppb.New(m.Timestamp),
			Labels:    m.Labels,
		}
	}

	return &lumov1.CheckResult{
		Name:       result.Name,
		Category:   result.Category.String(),
		Status:     toProtoCheckStatus(result.Status),
		Severity:   toProtoSeverity(result.Severity),
		Message:    result.Message,
		Data:       dataStruct,
		Metrics:    metrics,
		Timestamp:  timestamppb.New(result.Timestamp),
		DurationMs: result.Duration.Milliseconds(),
		Error:      result.Error,
	}, nil
}

// ... additional helper functions
```

### Step 2.4: Wire Up gRPC Server in Main

**File:** `cmd/lumo/serve_grpc.go`

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database"
	grpcserver "github.com/ignacio/lumo/internal/grpc/server"
	grpchandlers "github.com/ignacio/lumo/internal/grpc/handlers"
	lumov1 "github.com/ignacio/lumo/api/proto/v1"
)

var serveGRPCCmd = &cobra.Command{
	Use:   "serve-grpc",
	Short: "Start the Lumo gRPC API server",
	RunE:  runServeGRPC,
}

func init() {
	serveGRPCCmd.Flags().Int("port", 9443, "gRPC server port")
	serveGRPCCmd.Flags().Bool("tls", true, "Enable TLS")
	serveGRPCCmd.Flags().String("cert", "certs/server.crt", "TLS certificate file")
	serveGRPCCmd.Flags().String("key", "certs/server.key", "TLS key file")
	serveGRPCCmd.Flags().Int("max-msg-size", 10*1024*1024, "Max message size (10MB default)")
}

func runServeGRPC(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Get flags
	port, _ := cmd.Flags().GetInt("port")
	tlsEnabled, _ := cmd.Flags().GetBool("tls")
	certFile, _ := cmd.Flags().GetString("cert")
	keyFile, _ := cmd.Flags().GetString("key")
	maxMsgSize, _ := cmd.Flags().GetInt("max-msg-size")

	// Initialize database
	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.RunMigrations(cfg.Database.URL); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize repositories
	jobRepo := repository.NewJobRepository(db)
	agentRepo := repository.NewAgentRepository(db)

	// Initialize diagnostic runner
	diagnosticRunner := diagnostics.NewRunner(cfg)

	// Create gRPC server
	grpcServer, err := grpcserver.NewServer(cfg, grpcserver.ServerOptions{
		Port:       port,
		TLSEnabled: tlsEnabled,
		CertFile:   certFile,
		KeyFile:    keyFile,
		MaxMsgSize: maxMsgSize,
	}, log)
	if err != nil {
		return err
	}

	// Register service handlers
	diagnosticsHandler := grpchandlers.NewDiagnosticsHandler(jobRepo, diagnosticRunner)
	lumov1.RegisterDiagnosticsServiceServer(grpcServer.GetGRPCServer(), diagnosticsHandler)

	agentsHandler := grpchandlers.NewAgentsHandler(agentRepo)
	lumov1.RegisterAgentsServiceServer(grpcServer.GetGRPCServer(), agentsHandler)

	healthHandler := grpchandlers.NewHealthHandler(cfg, db)
	lumov1.RegisterHealthServiceServer(grpcServer.GetGRPCServer(), healthHandler)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		log.WithField("port", port).Info("gRPC server starting")
		errChan <- grpcServer.Start()
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		log.WithField("signal", sig).Info("Received shutdown signal")

		// Graceful shutdown with 30s timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		return grpcServer.GracefulStop(ctx)
	}
}
```

---

## Phase 3: mTLS Setup

**Duration:** 2-3 days
**Deliverables:** mTLS certificates, rotation strategy

### Step 3.1: Development Certificates (Self-Signed)

**Script:** `scripts/generate-dev-certs.sh`

```bash
#!/bin/bash
set -e

CERTS_DIR="deployments/certs"
mkdir -p "$CERTS_DIR"

echo "Generating development certificates for mTLS..."

# 1. Generate CA (Certificate Authority)
openssl genrsa -out "$CERTS_DIR/ca.key" 4096
openssl req -new -x509 -key "$CERTS_DIR/ca.key" -sha256 -subj "/C=US/ST=CA/O=Lumo Dev/CN=Lumo Dev CA" -days 365 -out "$CERTS_DIR/ca.crt"

# 2. Generate Server Certificate
openssl genrsa -out "$CERTS_DIR/server.key" 4096
openssl req -new -key "$CERTS_DIR/server.key" -out "$CERTS_DIR/server.csr" -config <(
cat <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = US
ST = CA
O = Lumo Dev
CN = localhost

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = lumo-api
DNS.3 = lumo-api.default.svc.cluster.local
IP.1 = 127.0.0.1
IP.2 = ::1
EOF
)

openssl x509 -req -in "$CERTS_DIR/server.csr" -CA "$CERTS_DIR/ca.crt" -CAkey "$CERTS_DIR/ca.key" -CAcreateserial \
  -out "$CERTS_DIR/server.crt" -days 365 -sha256 -extfile <(
cat <<EOF
subjectAltName = DNS:localhost,DNS:lumo-api,DNS:lumo-api.default.svc.cluster.local,IP:127.0.0.1,IP:::1
EOF
)

# 3. Generate Client Certificate (for agents)
openssl genrsa -out "$CERTS_DIR/client.key" 4096
openssl req -new -key "$CERTS_DIR/client.key" -out "$CERTS_DIR/client.csr" -subj "/C=US/ST=CA/O=Lumo Agent/CN=lumo-agent"
openssl x509 -req -in "$CERTS_DIR/client.csr" -CA "$CERTS_DIR/ca.crt" -CAkey "$CERTS_DIR/ca.key" -CAcreateserial \
  -out "$CERTS_DIR/client.crt" -days 365 -sha256

# Set proper permissions
chmod 600 "$CERTS_DIR"/*.key
chmod 644 "$CERTS_DIR"/*.crt

echo "✓ Certificates generated in $CERTS_DIR/"
echo "  - ca.crt: Certificate Authority (distribute to all agents)"
echo "  - server.{crt,key}: Server certificate for API"
echo "  - client.{crt,key}: Client certificate for agents"
```

### Step 3.2: Production mTLS with cert-manager (Kubernetes)

**File:** `deployments/kubernetes/mtls/cert-manager-issuer.yaml`

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: lumo-ca-issuer
spec:
  ca:
    secretName: lumo-ca-secret  # Create this secret with your CA

---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: lumo-server-cert
  namespace: default
spec:
  secretName: lumo-server-tls
  issuerRef:
    name: lumo-ca-issuer
    kind: ClusterIssuer
  dnsNames:
    - lumo-api
    - lumo-api.default.svc.cluster.local
    - lumo-api.example.com  # Your domain
  privateKey:
    algorithm: RSA
    size: 4096
  duration: 2160h  # 90 days
  renewBefore: 720h  # Renew 30 days before expiration

---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: lumo-agent-cert
  namespace: default
spec:
  secretName: lumo-agent-tls
  issuerRef:
    name: lumo-ca-issuer
    kind: ClusterIssuer
  commonName: lumo-agent
  privateKey:
    algorithm: RSA
    size: 4096
  duration: 2160h
  renewBefore: 720h
```

### Step 3.3: Update Server for mTLS

**File:** `internal/grpc/server/mtls.go`

```go
package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

type MTLSConfig struct {
	CertFile   string // Server certificate
	KeyFile    string // Server private key
	CAFile     string // CA certificate (for verifying client certs)
}

func NewMTLSCredentials(cfg MTLSConfig) (credentials.TransportCredentials, error) {
	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server cert: %w", err)
	}

	// Load CA certificate for client verification
	caCert, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA cert to pool")
	}

	// Configure TLS with client certificate verification
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,  // Require client certs
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS13,  // Enforce TLS 1.3
	}

	return credentials.NewTLS(tlsConfig), nil
}
```

Update `server.go` to use mTLS:

```go
// In NewServer function, replace TLS setup with:
if opts.MTLSEnabled {
	creds, err := NewMTLSCredentials(MTLSConfig{
		CertFile: opts.CertFile,
		KeyFile:  opts.KeyFile,
		CAFile:   opts.CAFile,  // NEW: CA for client verification
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup mTLS: %w", err)
	}
	serverOpts = append(serverOpts, grpc.Creds(creds))
	log.Info("gRPC server mTLS enabled (client cert required)")
}
```

---

## Phase 4: Client Migration

**Duration:** 2-3 days
**Deliverables:** Agent gRPC client, migration guide

### Step 4.1: Create gRPC Client Library

**File:** `internal/grpc/client/client.go`

```go
package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
)

type Client struct {
	conn        *grpc.ClientConn
	diagnostics lumov1.DiagnosticsServiceClient
	agents      lumov1.AgentsServiceClient
	health      lumov1.HealthServiceClient
	token       string
}

type ClientOptions struct {
	Address    string
	TLSEnabled bool
	CertFile   string  // Client certificate (for mTLS)
	KeyFile    string  // Client private key
	CAFile     string  // CA certificate
	Token      string  // JWT token
	Timeout    time.Duration
}

func NewClient(opts ClientOptions) (*Client, error) {
	var dialOpts []grpc.DialOption

	// Keepalive settings
	dialOpts = append(dialOpts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             3 * time.Second,
		PermitWithoutStream: true,
	}))

	// TLS/mTLS setup
	if opts.TLSEnabled {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS13,
		}

		// Load CA certificate
		if opts.CAFile != "" {
			caCert, err := os.ReadFile(opts.CAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA cert: %w", err)
			}

			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to append CA cert")
			}
			tlsConfig.RootCAs = certPool
		}

		// Load client certificate for mTLS
		if opts.CertFile != "" && opts.KeyFile != "" {
			clientCert, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load client cert: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{clientCert}
		}

		creds := credentials.NewTLS(tlsConfig)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}

	// Connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, opts.Address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &Client{
		conn:        conn,
		diagnostics: lumov1.NewDiagnosticsServiceClient(conn),
		agents:      lumov1.NewAgentsServiceClient(conn),
		health:      lumov1.NewHealthServiceClient(conn),
		token:       opts.Token,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// Add JWT token to context
func (c *Client) withAuth(ctx context.Context) context.Context {
	if c.token != "" {
		md := metadata.Pairs("authorization", "Bearer "+c.token)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	return ctx
}

// Diagnostics methods

func (c *Client) RunDiagnostics(ctx context.Context, req *lumov1.RunDiagnosticsRequest) (*lumov1.RunDiagnosticsResponse, error) {
	ctx = c.withAuth(ctx)
	return c.diagnostics.RunDiagnostics(ctx, req)
}

func (c *Client) GetDiagnosticsResult(ctx context.Context, jobID string) (*lumov1.GetDiagnosticsResultResponse, error) {
	ctx = c.withAuth(ctx)
	return c.diagnostics.GetDiagnosticsResult(ctx, &lumov1.GetDiagnosticsResultRequest{
		JobId: jobID,
	})
}

func (c *Client) StreamDiagnostics(ctx context.Context, req *lumov1.StreamDiagnosticsRequest, handler func(*lumov1.DiagnosticsEvent) error) error {
	ctx = c.withAuth(ctx)
	stream, err := c.diagnostics.StreamDiagnostics(ctx, req)
	if err != nil {
		return err
	}

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if err := handler(event); err != nil {
			return err
		}
	}
}

// Agent methods

func (c *Client) RegisterAgent(ctx context.Context, req *lumov1.RegisterAgentRequest) (*lumov1.RegisterAgentResponse, error) {
	ctx = c.withAuth(ctx)
	return c.agents.RegisterAgent(ctx, req)
}

func (c *Client) SendHeartbeat(ctx context.Context, agentID string, health *lumov1.AgentHealth) (*lumov1.SendHeartbeatResponse, error) {
	ctx = c.withAuth(ctx)
	return c.agents.SendHeartbeat(ctx, &lumov1.SendHeartbeatRequest{
		AgentId: agentID,
		Health:  health,
	})
}
```

### Step 4.2: Update Agent to Use gRPC

**File:** `internal/agent/reporter.go` (modify existing)

```go
package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/grpc/client"
	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/config"
)

type Reporter struct {
	client   *client.Client
	agentID  string
	cfg      *config.AgentConfig
	log      *logrus.Logger
}

func NewReporter(cfg *config.AgentConfig, log *logrus.Logger) (*Reporter, error) {
	// Create gRPC client
	grpcClient, err := client.NewClient(client.ClientOptions{
		Address:    cfg.APIEndpoint,
		TLSEnabled: cfg.TLSEnabled,
		CertFile:   cfg.CertFile,
		KeyFile:    cfg.KeyFile,
		CAFile:     cfg.CAFile,
		Token:      cfg.Token,
		Timeout:    10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	return &Reporter{
		client: grpcClient,
		cfg:    cfg,
		log:    log,
	}, nil
}

func (r *Reporter) Register(ctx context.Context, agentInfo *AgentInfo) (string, error) {
	req := &lumov1.RegisterAgentRequest{
		Name:         agentInfo.Name,
		Hostname:     agentInfo.Hostname,
		IpAddress:    agentInfo.IPAddress,
		Platform:     agentInfo.Platform,
		Architecture: agentInfo.Architecture,
		Version:      agentInfo.Version,
		Capabilities: agentInfo.Capabilities,
		Labels:       agentInfo.Labels,
	}

	resp, err := r.client.RegisterAgent(ctx, req)
	if err != nil {
		return "", fmt.Errorf("registration failed: %w", err)
	}

	r.agentID = resp.AgentId
	r.log.WithField("agent_id", r.agentID).Info("Agent registered successfully")

	return resp.AgentId, nil
}

func (r *Reporter) SendHeartbeat(ctx context.Context, health *HealthStatus) error {
	protoHealth := &lumov1.AgentHealth{
		Status: toProtoHealthStatus(health.Status),
		Errors: health.Errors,
	}

	if health.LastRun != nil {
		protoHealth.LastRun = timestamppb.New(*health.LastRun)
	}

	_, err := r.client.SendHeartbeat(ctx, r.agentID, protoHealth)
	if err != nil {
		return fmt.Errorf("heartbeat failed: %w", err)
	}

	return nil
}

func (r *Reporter) SendReport(ctx context.Context, report *diagnostics.Report) error {
	// Convert report to proto format
	req := &lumov1.RunDiagnosticsRequest{
		Target:  "localhost",
		Analyze: r.cfg.AIEnabled,
	}

	resp, err := r.client.RunDiagnostics(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to submit diagnostic: %w", err)
	}

	r.log.WithField("job_id", resp.JobId).Info("Diagnostic submitted")
	return nil
}

func (r *Reporter) Close() error {
	return r.client.Close()
}
```

---

## Phase 5: Testing & Validation

**Duration:** 3-4 days
**Deliverables:** Test suite, benchmarks

### Step 5.1: Unit Tests

**File:** `internal/grpc/handlers/diagnostics_test.go`

```go
package handlers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/grpc/handlers"
)

type MockJobRepo struct {
	mock.Mock
}

func (m *MockJobRepo) Create(ctx context.Context, job *models.Job) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func TestRunDiagnostics(t *testing.T) {
	mockRepo := new(MockJobRepo)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	handler := handlers.NewDiagnosticsHandler(mockRepo, nil)

	req := &lumov1.RunDiagnosticsRequest{
		Target: "localhost",
		Checks: []string{"cpu", "memory"},
	}

	resp, err := handler.RunDiagnostics(context.Background(), req)

	assert.NoError(t, err)
	assert.NotEmpty(t, resp.JobId)
	assert.Equal(t, lumov1.JobStatus_JOB_STATUS_PENDING, resp.Status)
	mockRepo.AssertExpectations(t)
}
```

### Step 5.2: Integration Tests

**File:** `internal/grpc/integration_test.go`

```go
package grpc_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
)

func TestGRPCServerIntegration(t *testing.T) {
	// Start test server
	server := startTestServer(t)
	defer server.Stop()

	// Create client
	conn, err := grpc.Dial("localhost:9443", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := lumov1.NewDiagnosticsServiceClient(conn)

	// Test RunDiagnostics
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.RunDiagnostics(ctx, &lumov1.RunDiagnosticsRequest{
		Target: "localhost",
		Checks: []string{"cpu"},
	})

	if err != nil {
		t.Fatalf("RunDiagnostics failed: %v", err)
	}

	if resp.JobId == "" {
		t.Error("Expected non-empty job ID")
	}
}
```

### Step 5.3: Performance Benchmarks

**File:** `internal/grpc/benchmark_test.go`

```go
package grpc_test

import (
	"context"
	"testing"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
)

func BenchmarkRunDiagnostics(b *testing.B) {
	server := startTestServer(b)
	defer server.Stop()

	client := createTestClient(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.RunDiagnostics(context.Background(), &lumov1.RunDiagnosticsRequest{
			Target: "localhost",
			Checks: []string{"cpu"},
		})
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
	}
}

func BenchmarkStreamDiagnostics(b *testing.B) {
	server := startTestServer(b)
	defer server.Stop()

	client := createTestClient(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, err := client.StreamDiagnostics(context.Background(), &lumov1.StreamDiagnosticsRequest{
			Target: "localhost",
			Checks: []string{"cpu"},
		})
		if err != nil {
			b.Fatalf("Stream failed: %v", err)
		}

		for {
			_, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatalf("Recv failed: %v", err)
			}
		}
	}
}
```

Run benchmarks:

```bash
go test -bench=. -benchmem ./internal/grpc/...

# Compare HTTP vs gRPC
go test -bench=. ./internal/api/... > http_bench.txt
go test -bench=. ./internal/grpc/... > grpc_bench.txt
benchstat http_bench.txt grpc_bench.txt
```

---

## Phase 6: Deployment & Rollout

**Duration:** 2-3 days
**Deliverables:** Deployment manifests, rollout plan

### Step 6.1: Dual-Protocol Deployment

**File:** `deployments/kubernetes/grpc/deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lumo-api-grpc
  labels:
    app: lumo-api
    protocol: grpc
spec:
  replicas: 2
  selector:
    matchLabels:
      app: lumo-api
      protocol: grpc
  template:
    metadata:
      labels:
        app: lumo-api
        protocol: grpc
    spec:
      containers:
      - name: lumo-api
        image: lumo:latest
        args: ["serve-grpc", "--port=9443", "--tls=true"]
        ports:
        - name: grpc
          containerPort: 9443
          protocol: TCP
        env:
        - name: LUMO_DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: lumo-db-secret
              key: url
        - name: LUMO_API_JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: lumo-api-secret
              key: jwt-secret
        volumeMounts:
        - name: tls-certs
          mountPath: /certs
          readOnly: true
        livenessProbe:
          grpc:
            port: 9443
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          grpc:
            port: 9443
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
      volumes:
      - name: tls-certs
        secret:
          secretName: lumo-server-tls  # From cert-manager

---
apiVersion: v1
kind: Service
metadata:
  name: lumo-api-grpc
  labels:
    app: lumo-api
    protocol: grpc
spec:
  type: ClusterIP
  ports:
  - port: 9443
    targetPort: 9443
    protocol: TCP
    name: grpc
  selector:
    app: lumo-api
    protocol: grpc
```

### Step 6.2: Rollout Strategy

**Phase 6.2.1: Week 1 - Internal Testing**
```bash
# Deploy gRPC server alongside HTTP server
kubectl apply -f deployments/kubernetes/grpc/

# Test with grpcurl
grpcurl -insecure localhost:9443 list
grpcurl -insecure localhost:9443 lumo.v1.HealthService/Check

# Test agent connection
kubectl logs -f deployment/lumo-agent
```

**Phase 6.2.2: Week 2 - Canary Deployment**
```yaml
# Route 10% of agents to gRPC
apiVersion: v1
kind: ConfigMap
metadata:
  name: agent-config
data:
  config.yaml: |
    agent:
      api_endpoint: "lumo-api-grpc:9443"  # 10% of agents
      # api_endpoint: "lumo-api:8080"  # 90% of agents (HTTP)
      protocol: grpc
```

**Phase 6.2.3: Week 3 - 50% Rollout**
- Update 50% of agents to gRPC
- Monitor error rates, latency, throughput
- Compare metrics: HTTP vs gRPC

**Phase 6.2.4: Week 4 - Full Migration**
- All agents on gRPC
- HTTP endpoints remain active for compatibility
- Deprecation notice for HTTP API

---

## Monitoring & Observability

### Prometheus Metrics

**File:** `internal/grpc/metrics/metrics.go`

```go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	GRPCRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lumo_grpc_request_duration_seconds",
			Help:    "gRPC request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "status"},
	)

	GRPCRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "status"},
	)

	GRPCActiveStreams = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lumo_grpc_active_streams",
			Help: "Number of active gRPC streams",
		},
	)

	GRPCMessagesSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_grpc_messages_sent_total",
			Help: "Total messages sent over gRPC",
		},
		[]string{"method"},
	)
)
```

### Grafana Dashboard

**File:** `deployments/grafana/grpc-dashboard.json`

```json
{
  "dashboard": {
    "title": "Lumo gRPC Metrics",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [{
          "expr": "rate(lumo_grpc_requests_total[5m])"
        }]
      },
      {
        "title": "Latency (p95)",
        "targets": [{
          "expr": "histogram_quantile(0.95, rate(lumo_grpc_request_duration_seconds_bucket[5m]))"
        }]
      },
      {
        "title": "Active Streams",
        "targets": [{
          "expr": "lumo_grpc_active_streams"
        }]
      },
      {
        "title": "Error Rate",
        "targets": [{
          "expr": "rate(lumo_grpc_requests_total{status!=\"OK\"}[5m])"
        }]
      }
    ]
  }
}
```

---

## Rollback Strategy

### Scenario 1: gRPC Server Issues

```bash
# Immediately revert agents to HTTP
kubectl rollout undo deployment/lumo-agent

# Or update ConfigMap
kubectl patch configmap agent-config -p '{"data":{"api_endpoint":"lumo-api:8080"}}'
kubectl rollout restart deployment/lumo-agent
```

### Scenario 2: Certificate Issues

```bash
# Temporarily disable mTLS
kubectl set env deployment/lumo-api-grpc TLS_ENABLED=false

# Or regenerate certificates
./scripts/generate-dev-certs.sh
kubectl create secret tls lumo-server-tls --cert=certs/server.crt --key=certs/server.key --dry-run=client -o yaml | kubectl apply -f -
```

### Scenario 3: Performance Degradation

```bash
# Scale up gRPC servers
kubectl scale deployment/lumo-api-grpc --replicas=5

# Increase resource limits
kubectl patch deployment lumo-api-grpc -p '{"spec":{"template":{"spec":{"containers":[{"name":"lumo-api","resources":{"limits":{"cpu":"1","memory":"1Gi"}}}]}}}}'
```

---

## Success Metrics

### Week 1 (Post-Deployment)
- ✅ gRPC server running without crashes (uptime >99%)
- ✅ mTLS handshake success rate >99%
- ✅ 10% of agents successfully connected via gRPC

### Week 2 (Canary)
- ✅ gRPC request latency <60ms (vs HTTP ~100ms)
- ✅ Zero authentication errors
- ✅ Throughput increase of 20%+

### Week 4 (Full Rollout)
- ✅ 100% of agents on gRPC
- ✅ Bandwidth reduction of 50%+
- ✅ No degradation in diagnostic success rate
- ✅ Certificate rotation working automatically

---

## Timeline Summary

| Week | Phase | Activities | Deliverables |
|------|-------|------------|--------------|
| 1 | Proto Definitions | Design .proto files, generate code | 4 proto files, Go code |
| 2 | Server Implementation | Handlers, interceptors, server setup | gRPC server running |
| 2-3 | mTLS Setup | Generate certs, configure mTLS | Dev certs, prod cert-manager |
| 3 | Client Migration | Update agent to gRPC client | Agent connecting via gRPC |
| 3-4 | Testing | Unit tests, integration tests, benchmarks | Test suite passing |
| 4 | Deployment | Deploy to dev, canary to production | 10% agents on gRPC |
| 5-6 | Rollout | Gradual migration to 100% | All agents on gRPC |

**Total:** 4-6 weeks (10-15 developer days)

---

## Next Steps

1. **Review this plan** with team
2. **Set up development environment** (protoc, cert generation)
3. **Create proto definitions** (Phase 1)
4. **Build proof-of-concept** (minimal gRPC server + client)
5. **Present POC** to stakeholders
6. **Proceed with full implementation** if approved

---

**Document Version:** 1.0
**Last Updated:** 2025-11-20
**Author:** Claude (Lumo Modernization Assessment)
**Status:** Ready for Implementation
