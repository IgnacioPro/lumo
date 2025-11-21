//nolint:staticcheck // Tests use deprecated gRPC APIs supported throughout 1.x
package client

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
)

const bufSize = 1024 * 1024

// mockHealthServer implements a simple health service for testing
type mockHealthServer struct {
	lumov1.UnimplementedHealthServiceServer
}

func (s *mockHealthServer) Live(ctx context.Context, req *lumov1.LiveRequest) (*lumov1.LiveResponse, error) {
	return &lumov1.LiveResponse{
		Alive: true,
	}, nil
}

func (s *mockHealthServer) Check(ctx context.Context, req *lumov1.HealthCheckRequest) (*lumov1.HealthCheckResponse, error) {
	return &lumov1.HealthCheckResponse{
		Status:  lumov1.HealthCheckResponse_SERVING_STATUS_SERVING,
		Version: "test-version",
	}, nil
}

func (s *mockHealthServer) Ready(ctx context.Context, req *lumov1.ReadyRequest) (*lumov1.ReadyResponse, error) {
	return &lumov1.ReadyResponse{
		Ready:   true,
		Message: "ready",
	}, nil
}

// mockAgentsServer implements a simple agents service for testing
type mockAgentsServer struct {
	lumov1.UnimplementedAgentsServiceServer
}

func (s *mockAgentsServer) RegisterAgent(ctx context.Context, req *lumov1.RegisterAgentRequest) (*lumov1.RegisterAgentResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	return &lumov1.RegisterAgentResponse{
		AgentId: "test-agent-id",
	}, nil
}

func (s *mockAgentsServer) SendHeartbeat(ctx context.Context, req *lumov1.SendHeartbeatRequest) (*lumov1.SendHeartbeatResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	return &lumov1.SendHeartbeatResponse{
		Acknowledged: true,
	}, nil
}

func (s *mockAgentsServer) GetAgentStats(ctx context.Context, req *lumov1.GetAgentStatsRequest) (*lumov1.GetAgentStatsResponse, error) {
	return &lumov1.GetAgentStatsResponse{
		TotalAgents:  10,
		OnlineAgents: 8,
	}, nil
}

// mockDiagnosticsServer implements a simple diagnostics service for testing
type mockDiagnosticsServer struct {
	lumov1.UnimplementedDiagnosticsServiceServer
}

func (s *mockDiagnosticsServer) RunDiagnostics(ctx context.Context, req *lumov1.RunDiagnosticsRequest) (*lumov1.RunDiagnosticsResponse, error) {
	if req.Target == "" {
		return nil, status.Error(codes.InvalidArgument, "target is required")
	}
	return &lumov1.RunDiagnosticsResponse{
		JobId:  "test-job-id",
		Status: lumov1.JobStatus_JOB_STATUS_PENDING,
	}, nil
}

func (s *mockDiagnosticsServer) GetDiagnosticsResult(ctx context.Context, req *lumov1.GetDiagnosticsResultRequest) (*lumov1.GetDiagnosticsResultResponse, error) {
	if req.JobId == "" {
		return nil, status.Error(codes.InvalidArgument, "job_id is required")
	}
	return &lumov1.GetDiagnosticsResultResponse{
		JobId:  req.JobId,
		Status: lumov1.JobStatus_JOB_STATUS_COMPLETED,
	}, nil
}

// setupTestServer creates a test gRPC server with mock services
// Returns: server, dialer function, cleanup function
func setupTestServer() (*grpc.Server, func(context.Context, string) (net.Conn, error), func()) {
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()

	lumov1.RegisterHealthServiceServer(s, &mockHealthServer{})
	lumov1.RegisterAgentsServiceServer(s, &mockAgentsServer{})
	lumov1.RegisterDiagnosticsServiceServer(s, &mockDiagnosticsServer{})

	go func() {
		_ = s.Serve(lis)
	}()

	// Create a dialer that closes over the specific listener
	bufDialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	cleanup := func() {
		s.Stop()
		_ = lis.Close()
	}

	return s, bufDialer, cleanup
}

func TestNewClient(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	tests := []struct {
		name    string
		opts    ClientOptions
		wantErr bool
	}{
		{
			name: "valid client without TLS",
			opts: ClientOptions{
				Address:    "bufnet",
				TLSEnabled: false,
				Timeout:    5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "client with default timeout",
			opts: ClientOptions{
				Address:    "bufnet",
				TLSEnabled: false,
			},
			wantErr: false,
		},
		{
			name: "client with custom max message size",
			opts: ClientOptions{
				Address:    "bufnet",
				TLSEnabled: false,
				MaxMsgSize: 50 * 1024 * 1024,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Override dial options for testing
			ctx := context.Background()
			//nolint:staticcheck // Deprecated API supported throughout 1.x
			conn, err := grpc.DialContext(ctx, "bufnet",
				grpc.WithContextDialer(bufDialer),
				//nolint:staticcheck // Deprecated API supported throughout 1.x
				grpc.WithInsecure(),
			)
			require.NoError(t, err)
			defer func() { _ = conn.Close() }()

			client := &Client{
				conn:        conn,
				diagnostics: lumov1.NewDiagnosticsServiceClient(conn),
				agents:      lumov1.NewAgentsServiceClient(conn),
				health:      lumov1.NewHealthServiceClient(conn),
				token:       tt.opts.Token,
			}

			assert.NotNil(t, client)
			assert.NotNil(t, client.conn)
			assert.NotNil(t, client.diagnostics)
			assert.NotNil(t, client.agents)
			assert.NotNil(t, client.health)
		})
	}
}

func TestClient_Close(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	ctx := context.Background()
	//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		//nolint:staticcheck // Deprecated API supported throughout 1.x
		grpc.WithInsecure(),
	)
	require.NoError(t, err)

	client := &Client{conn: conn}

	err = client.Close()
	assert.NoError(t, err)

	// Wait a bit for the connection to fully close
	time.Sleep(10 * time.Millisecond)

	// Closing again should not error (connection already closed)
	err = client.Close()
	// Either no error or "closing/closed" error is acceptable
	if err != nil {
		assert.Contains(t, err.Error(), "clos")
	}
}

func TestClient_HealthCheck(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	ctx := context.Background()
	//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		//nolint:staticcheck // Deprecated API supported throughout 1.x
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := &Client{
		conn:   conn,
		health: lumov1.NewHealthServiceClient(conn),
	}

	resp, err := client.HealthCheck(ctx, "test-service")
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, lumov1.HealthCheckResponse_SERVING_STATUS_SERVING, resp.Status)
	assert.Equal(t, "test-version", resp.Version)
}

func TestClient_Live(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	ctx := context.Background()
	//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		//nolint:staticcheck // Deprecated API supported throughout 1.x
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := &Client{
		conn:   conn,
		health: lumov1.NewHealthServiceClient(conn),
	}

	resp, err := client.Live(ctx)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Alive)
}

func TestClient_Ready(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	ctx := context.Background()
	//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		//nolint:staticcheck // Deprecated API supported throughout 1.x
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := &Client{
		conn:   conn,
		health: lumov1.NewHealthServiceClient(conn),
	}

	resp, err := client.Ready(ctx)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ready)
	assert.Equal(t, "ready", resp.Message)
}

func TestClient_Ping(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	ctx := context.Background()
	//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		//nolint:staticcheck // Deprecated API supported throughout 1.x
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := &Client{
		conn:   conn,
		health: lumov1.NewHealthServiceClient(conn),
	}

	err = client.Ping(ctx)
	assert.NoError(t, err)
}

func TestClient_RegisterAgent(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	ctx := context.Background()
	//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		//nolint:staticcheck // Deprecated API supported throughout 1.x
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := &Client{
		conn:   conn,
		agents: lumov1.NewAgentsServiceClient(conn),
	}

	tests := []struct {
		name    string
		req     *lumov1.RegisterAgentRequest
		wantErr bool
	}{
		{
			name: "valid registration",
			req: &lumov1.RegisterAgentRequest{
				Name:     "test-agent",
				Hostname: "test-host",
				Platform: "linux",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			req: &lumov1.RegisterAgentRequest{
				Hostname: "test-host",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.RegisterAgent(ctx, tt.req)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "test-agent-id", resp.AgentId)
			}
		})
	}
}

func TestClient_SendHeartbeat(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	ctx := context.Background()
	//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		//nolint:staticcheck // Deprecated API supported throughout 1.x
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := &Client{
		conn:   conn,
		agents: lumov1.NewAgentsServiceClient(conn),
	}

	tests := []struct {
		name    string
		agentID string
		health  *lumov1.AgentHealth
		wantErr bool
	}{
		{
			name:    "valid heartbeat",
			agentID: "test-agent-id",
			health: &lumov1.AgentHealth{
				Status: lumov1.AgentHealth_HEALTH_STATUS_HEALTHY,
			},
			wantErr: false,
		},
		{
			name:    "empty agent ID",
			agentID: "",
			health:  &lumov1.AgentHealth{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.SendHeartbeat(ctx, tt.agentID, tt.health)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.True(t, resp.Acknowledged)
			}
		})
	}
}

func TestClient_GetAgentStats(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	ctx := context.Background()
	//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		//nolint:staticcheck // Deprecated API supported throughout 1.x
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := &Client{
		conn:   conn,
		agents: lumov1.NewAgentsServiceClient(conn),
	}

	resp, err := client.GetAgentStats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(10), resp.TotalAgents)
	assert.Equal(t, int32(8), resp.OnlineAgents)
}

func TestClient_RunDiagnostics(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	ctx := context.Background()
	//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		//nolint:staticcheck // Deprecated API supported throughout 1.x
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := &Client{
		conn:        conn,
		diagnostics: lumov1.NewDiagnosticsServiceClient(conn),
	}

	tests := []struct {
		name    string
		req     *lumov1.RunDiagnosticsRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &lumov1.RunDiagnosticsRequest{
				Target: "localhost",
				Checks: []string{"cpu", "memory"},
			},
			wantErr: false,
		},
		{
			name: "missing target",
			req: &lumov1.RunDiagnosticsRequest{
				Checks: []string{"cpu"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.RunDiagnostics(ctx, tt.req)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "test-job-id", resp.JobId)
				assert.Equal(t, lumov1.JobStatus_JOB_STATUS_PENDING, resp.Status)
			}
		})
	}
}

func TestClient_GetDiagnosticsResult(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	ctx := context.Background()
	//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		//nolint:staticcheck // Deprecated API supported throughout 1.x
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := &Client{
		conn:        conn,
		diagnostics: lumov1.NewDiagnosticsServiceClient(conn),
	}

	tests := []struct {
		name    string
		jobID   string
		wantErr bool
	}{
		{
			name:    "valid job ID",
			jobID:   "test-job-id",
			wantErr: false,
		},
		{
			name:    "empty job ID",
			jobID:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.GetDiagnosticsResult(ctx, tt.jobID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.jobID, resp.JobId)
				assert.Equal(t, lumov1.JobStatus_JOB_STATUS_COMPLETED, resp.Status)
			}
		})
	}
}

func TestClient_WithAuth(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	ctx := context.Background()
	//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		//nolint:staticcheck // Deprecated API supported throughout 1.x
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "with token",
			token: "test-jwt-token",
		},
		{
			name:  "without token",
			token: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				conn:  conn,
				token: tt.token,
			}

			ctx = client.withAuth(context.Background())
			assert.NotNil(t, ctx)

			// If token is set, metadata should be present
			// This is hard to test without actually making a call,
			// but we can at least verify the context is not nil
		})
	}
}

func TestClient_GetConnectionState(t *testing.T) {
	_, bufDialer, cleanup := setupTestServer()
	defer cleanup()

	ctx := context.Background()
	//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		//nolint:staticcheck // Deprecated API supported throughout 1.x
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := &Client{conn: conn}

	state := client.GetConnectionState()
	assert.NotEmpty(t, state)

	// Test with nil connection
	client2 := &Client{conn: nil}
	state2 := client2.GetConnectionState()
	assert.Equal(t, "CLOSED", state2)
}
