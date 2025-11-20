# Lumo gRPC API

This directory contains the gRPC server implementation for Lumo, providing high-performance RPC communication with mutual TLS (mTLS) support.

## Architecture

```
internal/grpc/
├── server/
│   ├── server.go          # Main gRPC server wrapper
│   └── mtls.go            # TLS/mTLS credential management
├── interceptors/
│   ├── auth.go            # JWT authentication interceptor
│   ├── logging.go         # Request/response logging
│   └── recovery.go        # Panic recovery
└── handlers/
    ├── diagnostics.go     # DiagnosticsService implementation
    ├── agents.go          # AgentsService implementation
    └── health.go          # HealthService implementation
```

## Services

### DiagnosticsService

Manages diagnostic job lifecycle:
- `RunDiagnostics` - Create and queue diagnostic jobs
- `GetDiagnosticsResult` - Retrieve job results
- `StreamDiagnostics` - Real-time progress streaming
- `ListDiagnostics` - Query diagnostic history

### AgentsService

Agent registration and management:
- `RegisterAgent` - Register new agent
- `SendHeartbeat` - Update agent heartbeat
- `ListAgents` - List registered agents
- `GetAgent` - Get agent details
- `DeleteAgent` - Remove agent registration
- `GetAgentStats` - Aggregate agent statistics

### HealthService

Kubernetes health probes:
- `Check` - Comprehensive health check with component status
- `Ready` - Readiness probe (database connectivity)
- `Live` - Liveness probe (simple response)

## Security

### Authentication

**JWT Authentication** via gRPC metadata:
```go
// Client-side
md := metadata.Pairs("authorization", "Bearer "+token)
ctx := metadata.NewOutgoingContext(context.Background(), md)
```

**Public Endpoints** (no auth required):
- `HealthService/*` - All health check endpoints

### TLS/mTLS

#### Standard TLS (Development)

Server authenticates to clients with certificate:

```bash
# Start server with TLS
lumo serve-grpc --tls --cert deployments/certs/server.crt --key deployments/certs/server.key
```

#### Mutual TLS (Production)

Both server and client authenticate with certificates:

```bash
# Start server with mTLS
lumo serve-grpc --mtls --cert deployments/certs/server.crt \
  --key deployments/certs/server.key --ca-file deployments/certs/ca.crt
```

**mTLS Requirements:**
- Server certificate and private key
- CA certificate for client verification
- Clients must present valid certificates signed by the CA

#### Certificate Generation

**Development Certificates:**

```bash
# Generate CA, server cert, and client cert
./scripts/generate-dev-certs.sh

# Certificates created in deployments/certs/:
# - ca.crt, ca.key           # Certificate Authority
# - server.crt, server.key   # Server certificate
# - client.crt, client.key   # Client certificate
```

**Production Certificates:**

Use a proper Certificate Authority or cert-manager:

```bash
# Kubernetes with cert-manager
kubectl apply -f deployments/kubernetes/cert-manager/
```

**Certificate Details:**
- Algorithm: RSA 4096-bit
- Validity: 365 days (adjust for production)
- TLS Version: TLS 1.3 only
- Server SAN: localhost, 127.0.0.1, ::1, lumo-api, *.lumo-api.svc.cluster.local

### TLS Configuration

**Server TLS Config:**
```go
&tls.Config{
    Certificates: []tls.Certificate{serverCert},
    ClientAuth:   tls.RequireAndVerifyClientCert,  // mTLS
    ClientCAs:    certPool,
    MinVersion:   tls.VersionTLS13,
    CipherSuites: []uint16{
        tls.TLS_AES_128_GCM_SHA256,
        tls.TLS_AES_256_GCM_SHA384,
        tls.TLS_CHACHA20_POLY1305_SHA256,
    },
}
```

## Configuration

### Server Options

```go
grpcserver.ServerOptions{
    Port:        9443,                              // gRPC port
    TLSEnabled:  true,                              // Enable TLS
    MTLSEnabled: true,                              // Enable mTLS (requires TLS)
    CertFile:    "deployments/certs/server.crt",    // Server certificate
    KeyFile:     "deployments/certs/server.key",    // Server private key
    CAFile:      "deployments/certs/ca.crt",        // CA for client verification
    MaxMsgSize:  10 * 1024 * 1024,                  // 10MB max message size
}
```

### Keepalive Settings

Long-lived connection management:

```go
grpc.KeepaliveParams(keepalive.ServerParameters{
    MaxConnectionIdle:     15 * time.Minute,  // Close idle connections
    MaxConnectionAge:      30 * time.Minute,  // Max connection lifetime
    MaxConnectionAgeGrace: 5 * time.Minute,   // Grace period for active RPCs
    Time:                  5 * time.Minute,   // Keepalive ping interval
    Timeout:               1 * time.Minute,   // Keepalive ping timeout
})
```

### Interceptors

**Execution Order:**
1. Logging (request start)
2. Recovery (panic handling)
3. Authentication (JWT validation)
4. Handler execution
5. Logging (request complete)

## Usage

### Starting the Server

```bash
# Without TLS (development only)
lumo serve-grpc --port 9443

# With TLS
lumo serve-grpc --tls --cert server.crt --key server.key

# With mTLS (recommended for production)
lumo serve-grpc --mtls --cert server.crt --key server.key --ca-file ca.crt

# Custom message size (default 10MB)
lumo serve-grpc --max-msg-size 52428800  # 50MB
```

### Client Examples

#### Go Client

```go
package main

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "os"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/metadata"

    lumov1 "github.com/ignacio/lumo/api/proto/v1"
)

func main() {
    // Load client certificate (for mTLS)
    clientCert, _ := tls.LoadX509KeyPair("client.crt", "client.key")

    // Load CA certificate
    caCert, _ := os.ReadFile("ca.crt")
    certPool := x509.NewCertPool()
    certPool.AppendCertsFromPEM(caCert)

    // Create TLS credentials
    creds := credentials.NewTLS(&tls.Config{
        Certificates: []tls.Certificate{clientCert},  // Client cert for mTLS
        RootCAs:      certPool,                        // Trust server CA
        MinVersion:   tls.VersionTLS13,
    })

    // Connect
    conn, _ := grpc.Dial("localhost:9443",
        grpc.WithTransportCredentials(creds),
        grpc.WithMaxMsgSize(10*1024*1024),
    )
    defer conn.Close()

    // Create client
    client := lumov1.NewDiagnosticsServiceClient(conn)

    // Add JWT token to metadata
    md := metadata.Pairs("authorization", "Bearer "+jwtToken)
    ctx := metadata.NewOutgoingContext(context.Background(), md)

    // Call RPC
    resp, _ := client.RunDiagnostics(ctx, &lumov1.RunDiagnosticsRequest{
        Target: "localhost",
        Checks: []string{"cpu", "memory", "disk"},
    })
}
```

#### grpcurl (Testing)

```bash
# Health check (no auth)
grpcurl -insecure localhost:9443 lumo.v1.HealthService/Live

# With mTLS
grpcurl -cacert ca.crt -cert client.crt -key client.key \
  localhost:9443 lumo.v1.HealthService/Check

# With JWT authentication
grpcurl -insecure -H "authorization: Bearer $JWT_TOKEN" \
  localhost:9443 lumo.v1.AgentsService/ListAgents

# List available services (reflection enabled)
grpcurl -insecure localhost:9443 list

# Describe a service
grpcurl -insecure localhost:9443 describe lumo.v1.DiagnosticsService
```

#### Python Client

```python
import grpc
from lumo.v1 import diagnostics_pb2, diagnostics_pb2_grpc

# Load certificates
with open('ca.crt', 'rb') as f:
    ca_cert = f.read()
with open('client.crt', 'rb') as f:
    client_cert = f.read()
with open('client.key', 'rb') as f:
    client_key = f.read()

# Create credentials
credentials = grpc.ssl_channel_credentials(
    root_certificates=ca_cert,
    private_key=client_key,
    certificate_chain=client_cert
)

# Connect
channel = grpc.secure_channel('localhost:9443', credentials)
client = diagnostics_pb2_grpc.DiagnosticsServiceStub(channel)

# Add JWT token to metadata
metadata = [('authorization', f'Bearer {jwt_token}')]

# Call RPC
response = client.RunDiagnostics(
    diagnostics_pb2.RunDiagnosticsRequest(
        target='localhost',
        checks=['cpu', 'memory', 'disk']
    ),
    metadata=metadata
)
```

## Graceful Shutdown

The server handles `SIGINT` and `SIGTERM` signals:

1. Stops accepting new connections
2. Waits up to 30 seconds for active RPCs to complete
3. Forces shutdown if timeout exceeded

```go
// In application code
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := grpcServer.GracefulStop(ctx); err != nil {
    log.Warn("Graceful shutdown timeout, forcing stop")
}
```

## Debugging

### Enable gRPC Logging

```bash
# Set environment variable
export GRPC_GO_LOG_VERBOSITY_LEVEL=99
export GRPC_GO_LOG_SEVERITY_LEVEL=info

lumo serve-grpc --tls
```

### Test with grpcurl

```bash
# Install grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# List services (reflection enabled)
grpcurl -insecure localhost:9443 list

# Call health check
grpcurl -insecure localhost:9443 lumo.v1.HealthService/Live

# Describe request format
grpcurl -insecure localhost:9443 describe lumo.v1.RunDiagnosticsRequest
```

### Monitor with Prometheus

The gRPC server exposes metrics via the lumo-agent metrics endpoint:

```bash
# If running lumo-agent with gRPC
curl http://localhost:9090/metrics | grep grpc
```

## Deployment

### Kubernetes

See `deployments/kubernetes/` for manifests:

```bash
# Deploy with Helm
helm install lumo-api deployments/kubernetes/helm/lumo-api \
  --set grpc.enabled=true \
  --set grpc.mtls.enabled=true

# Deploy with kubectl
kubectl apply -f deployments/kubernetes/grpc/
```

### Docker

```dockerfile
FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY lumo /usr/local/bin/
COPY deployments/certs/ /etc/lumo/certs/
ENTRYPOINT ["lumo", "serve-grpc"]
CMD ["--mtls", "--cert", "/etc/lumo/certs/server.crt", \
     "--key", "/etc/lumo/certs/server.key", \
     "--ca-file", "/etc/lumo/certs/ca.crt"]
```

## Troubleshooting

### Connection Refused

**Problem:** `connection refused`

**Solutions:**
- Check server is running: `netstat -tuln | grep 9443`
- Verify firewall rules allow port 9443
- Check server logs for startup errors

### Certificate Verification Failed

**Problem:** `x509: certificate signed by unknown authority`

**Solutions:**
- Ensure CA certificate is correct: `openssl x509 -in ca.crt -text -noout`
- Verify client trusts the CA
- Check certificate expiration: `openssl x509 -in server.crt -noout -dates`
- Verify SAN includes correct hostname/IP

### Authentication Failed

**Problem:** `rpc error: code = Unauthenticated`

**Solutions:**
- Check JWT token is valid: `echo $JWT_TOKEN | base64 -d`
- Verify token is in metadata: `authorization: Bearer <token>`
- Check token expiration
- Verify JWT secret matches server config

### TLS Handshake Failed

**Problem:** `tls: handshake failure`

**Solutions:**
- Check TLS version compatibility (server requires TLS 1.3)
- Verify certificate and key match: `openssl x509 -noout -modulus -in server.crt | openssl md5`
- Check client certificate (if mTLS): `openssl verify -CAfile ca.crt client.crt`

## Performance

### Recommended Settings

**Production:**
- mTLS enabled
- Max message size: 10-50 MB (depending on diagnostic data)
- Keepalive: 5 min intervals
- Connection pool: 10-50 connections per client
- Max concurrent streams: 100 per connection

**High Throughput:**
- Increase max message size for large diagnostics
- Use streaming RPCs for real-time updates
- Enable HTTP/2 flow control tuning
- Connection pooling in clients

### Benchmarks

Coming soon in Phase 5 (Testing & Validation).

## References

- [gRPC Documentation](https://grpc.io/docs/)
- [Protocol Buffers](https://protobuf.dev/)
- [gRPC Best Practices](https://grpc.io/docs/guides/performance/)
- [TLS 1.3 Specification](https://datatracker.ietf.org/doc/html/rfc8446)
- [Kubernetes cert-manager](https://cert-manager.io/)

## Related Files

- Protocol Buffer definitions: `api/proto/v1/`
- Generated code: `api/proto/v1/*.pb.go`
- CLI command: `cmd/lumo/serve_grpc.go`
- Certificate script: `scripts/generate-dev-certs.sh`
