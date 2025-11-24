package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/reliability"
)

// Client wraps the gRPC client connections for Lumo services
type Client struct {
	conn           *grpc.ClientConn
	diagnostics    lumov1.DiagnosticsServiceClient
	agents         lumov1.AgentsServiceClient
	health         lumov1.HealthServiceClient
	token          string
	circuitBreaker *reliability.CircuitBreaker
}

// ClientOptions contains configuration for the gRPC client
type ClientOptions struct {
	Address    string        // Server address (host:port)
	TLSEnabled bool          // Enable TLS
	CertFile   string        // Client certificate (for mTLS)
	KeyFile    string        // Client private key
	CAFile     string        // CA certificate for server verification
	Token      string        // JWT token for authentication
	Timeout    time.Duration // Connection timeout
	MaxMsgSize int           // Max message size (default 10MB)
}

// NewClient creates a new gRPC client with the specified options
func NewClient(opts ClientOptions) (*Client, error) {
	var dialOpts []grpc.DialOption

	// Set default timeout if not specified
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}

	// Set default max message size if not specified
	if opts.MaxMsgSize == 0 {
		opts.MaxMsgSize = 10 * 1024 * 1024 // 10MB
	}

	// Keepalive settings for long-lived connections
	dialOpts = append(dialOpts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                10 * time.Second, // Send keepalive ping every 10s
		Timeout:             3 * time.Second,  // Wait 3s for ping ack
		PermitWithoutStream: true,             // Send pings even without active streams
	}))

	// Max message size
	dialOpts = append(dialOpts,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(opts.MaxMsgSize),
			grpc.MaxCallSendMsgSize(opts.MaxMsgSize),
		),
	)

	// TLS/mTLS setup
	if opts.TLSEnabled {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS13,
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
			},
		}

		// Load CA certificate for server verification
		if opts.CAFile != "" {
			caCert, err := os.ReadFile(opts.CAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA certificate: %w", err)
			}

			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to append CA certificate to pool")
			}
			tlsConfig.RootCAs = certPool
		}

		// Load client certificate for mTLS
		if opts.CertFile != "" && opts.KeyFile != "" {
			clientCert, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load client certificate: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{clientCert}
		}

		creds := credentials.NewTLS(tlsConfig)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		// Insecure connection (development only)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	//nolint:staticcheck // SA1019: grpc.DialContext is deprecated but supported throughout 1.x
	conn, err := grpc.DialContext(ctx, opts.Address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", opts.Address, err)
	}

	return &Client{
		conn:           conn,
		diagnostics:    lumov1.NewDiagnosticsServiceClient(conn),
		agents:         lumov1.NewAgentsServiceClient(conn),
		health:         lumov1.NewHealthServiceClient(conn),
		token:          opts.Token,
		circuitBreaker: reliability.NewCircuitBreaker(fmt.Sprintf("grpc-%s", opts.Address)),
	}, nil
}

// Close closes the underlying gRPC connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// withAuth adds JWT token to context metadata if available
func (c *Client) withAuth(ctx context.Context) context.Context {
	if c.token != "" {
		md := metadata.Pairs("authorization", "Bearer "+c.token)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	return ctx
}

// ===== Diagnostics Service Methods =====

// RunDiagnostics initiates a new diagnostic run
func (c *Client) RunDiagnostics(ctx context.Context, req *lumov1.RunDiagnosticsRequest) (*lumov1.RunDiagnosticsResponse, error) {
	result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
		authCtx := c.withAuth(ctx)
		return c.diagnostics.RunDiagnostics(authCtx, req)
	})
	if err != nil {
		return nil, err
	}
	return result.(*lumov1.RunDiagnosticsResponse), nil
}

// GetDiagnosticsResult retrieves the result of a diagnostic job
func (c *Client) GetDiagnosticsResult(ctx context.Context, jobID string) (*lumov1.GetDiagnosticsResultResponse, error) {
	result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
		authCtx := c.withAuth(ctx)
		return c.diagnostics.GetDiagnosticsResult(authCtx, &lumov1.GetDiagnosticsResultRequest{
			JobId: jobID,
		})
	})
	if err != nil {
		return nil, err
	}
	return result.(*lumov1.GetDiagnosticsResultResponse), nil
}

// StreamDiagnostics streams diagnostic progress in real-time
// The handler function is called for each event received
func (c *Client) StreamDiagnostics(ctx context.Context, req *lumov1.StreamDiagnosticsRequest, handler func(*lumov1.DiagnosticsEvent) error) error {
	ctx = c.withAuth(ctx)
	stream, err := c.diagnostics.StreamDiagnostics(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			// Stream completed normally
			return nil
		}
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		// Call handler with received event
		if err := handler(event); err != nil {
			return fmt.Errorf("handler error: %w", err)
		}
	}
}

// ListDiagnostics lists diagnostic jobs with optional filtering
func (c *Client) ListDiagnostics(ctx context.Context, req *lumov1.ListDiagnosticsRequest) (*lumov1.ListDiagnosticsResponse, error) {
	ctx = c.withAuth(ctx)
	return c.diagnostics.ListDiagnostics(ctx, req)
}

// ===== Agent Service Methods =====

// RegisterAgent registers a new agent with the server
func (c *Client) RegisterAgent(ctx context.Context, req *lumov1.RegisterAgentRequest) (*lumov1.RegisterAgentResponse, error) {
	result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
		authCtx := c.withAuth(ctx)
		return c.agents.RegisterAgent(authCtx, req)
	})
	if err != nil {
		return nil, err
	}
	return result.(*lumov1.RegisterAgentResponse), nil
}

// SendHeartbeat updates agent heartbeat and health status
func (c *Client) SendHeartbeat(ctx context.Context, agentID string, health *lumov1.AgentHealth) (*lumov1.SendHeartbeatResponse, error) {
	result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
		authCtx := c.withAuth(ctx)
		return c.agents.SendHeartbeat(authCtx, &lumov1.SendHeartbeatRequest{
			AgentId: agentID,
			Health:  health,
		})
	})
	if err != nil {
		return nil, err
	}
	return result.(*lumov1.SendHeartbeatResponse), nil
}

// ListAgents lists registered agents with optional filtering
func (c *Client) ListAgents(ctx context.Context, req *lumov1.ListAgentsRequest) (*lumov1.ListAgentsResponse, error) {
	ctx = c.withAuth(ctx)
	return c.agents.ListAgents(ctx, req)
}

// GetAgent retrieves details of a specific agent
func (c *Client) GetAgent(ctx context.Context, agentID string) (*lumov1.GetAgentResponse, error) {
	ctx = c.withAuth(ctx)
	return c.agents.GetAgent(ctx, &lumov1.GetAgentRequest{
		AgentId: agentID,
	})
}

// DeleteAgent removes an agent registration
func (c *Client) DeleteAgent(ctx context.Context, agentID string) (*lumov1.DeleteAgentResponse, error) {
	ctx = c.withAuth(ctx)
	return c.agents.DeleteAgent(ctx, &lumov1.DeleteAgentRequest{
		AgentId: agentID,
	})
}

// GetAgentStats returns aggregate agent statistics
func (c *Client) GetAgentStats(ctx context.Context) (*lumov1.GetAgentStatsResponse, error) {
	ctx = c.withAuth(ctx)
	return c.agents.GetAgentStats(ctx, &lumov1.GetAgentStatsRequest{})
}

// ===== Health Service Methods =====

// HealthCheck performs a comprehensive health check
func (c *Client) HealthCheck(ctx context.Context, service string) (*lumov1.HealthCheckResponse, error) {
	// Health checks don't require authentication
	return c.health.Check(ctx, &lumov1.HealthCheckRequest{
		Service: service,
	})
}

// Ready checks if the server is ready to serve requests
func (c *Client) Ready(ctx context.Context) (*lumov1.ReadyResponse, error) {
	// Readiness checks don't require authentication
	return c.health.Ready(ctx, &lumov1.ReadyRequest{})
}

// Live checks if the server is alive
func (c *Client) Live(ctx context.Context) (*lumov1.LiveResponse, error) {
	// Liveness checks don't require authentication
	return c.health.Live(ctx, &lumov1.LiveRequest{})
}

// ===== Connection Management =====

// Ping performs a simple health check to verify connectivity
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Live(ctx)
	return err
}

// GetConnectionState returns the current connection state
func (c *Client) GetConnectionState() string {
	if c.conn == nil {
		return "CLOSED"
	}
	return c.conn.GetState().String()
}

// WaitForReady blocks until the connection is ready or context is cancelled
func (c *Client) WaitForReady(ctx context.Context) error {
	if c.conn == nil {
		return fmt.Errorf("connection is closed")
	}

	// Wait for state change from current state
	currentState := c.conn.GetState()
	if !c.conn.WaitForStateChange(ctx, currentState) {
		// Context was cancelled or deadline exceeded
		return ctx.Err()
	}

	return nil
}
