package grpc_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/grpc/client"
	"github.com/ignacio/lumo/internal/grpc/handlers"
	grpcserver "github.com/ignacio/lumo/internal/grpc/server"
)

const bufSize = 1024 * 1024

// mockJobRepo is a minimal in-memory job repository for testing
type mockJobRepo struct{}

func (m *mockJobRepo) Create(ctx context.Context, job interface{}) error { return nil }
func (m *mockJobRepo) Get(ctx context.Context, id interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockJobRepo) List(ctx context.Context, limit, offset int) (interface{}, error) {
	return []interface{}{}, nil
}
func (m *mockJobRepo) Count(ctx context.Context) (int, error) { return 0, nil }
func (m *mockJobRepo) Update(ctx context.Context, job interface{}) error { return nil }
func (m *mockJobRepo) Delete(ctx context.Context, id interface{}) error  { return nil }

// mockAgentRepo is a minimal in-memory agent repository for testing
type mockAgentRepo struct{}

func (m *mockAgentRepo) Create(ctx context.Context, agent interface{}) error { return nil }
func (m *mockAgentRepo) Get(ctx context.Context, id interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockAgentRepo) List(ctx context.Context, limit, offset int) (interface{}, error) {
	return []interface{}{}, nil
}
func (m *mockAgentRepo) Count(ctx context.Context) (int, error) { return 5, nil }
func (m *mockAgentRepo) UpdateHeartbeat(ctx context.Context, id interface{}) error {
	return nil
}
func (m *mockAgentRepo) Delete(ctx context.Context, id interface{}) error { return nil }
func (m *mockAgentRepo) GetStats(ctx context.Context) (interface{}, error) {
	return struct {
		TotalAgents   int
		OnlineAgents  int
		OfflineAgents int
		ErrorAgents   int
		ByPlatform    map[string]int32
	}{
		TotalAgents:  5,
		OnlineAgents: 4,
		ByPlatform:   map[string]int32{"linux": 3, "darwin": 2},
	}, nil
}

// setupTestServerAndClient creates a test gRPC server and client using bufconn
func setupTestServerAndClient(t *testing.T) (*grpc.Server, *client.Client, func()) {
	// Create bufconn listener for in-memory connection
	lis := bufconn.Listen(bufSize)

	// Create gRPC server
	srv := grpc.NewServer()

	// Register services with mock repos
	cfg := &config.Config{}
	log := logrus.New()
	log.SetLevel(logrus.WarnLevel) // Reduce noise in tests

	// Note: These are simplified mocks. In real tests, use proper mock implementations
	// from internal/database/repository
	healthHandler := handlers.NewHealthHandler(cfg, nil)
	lumov1.RegisterHealthServiceServer(srv, healthHandler)

	// Start server
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Server exited: %v", err)
		}
	}()

	// Create bufconn dialer
	bufDialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	// Create client connection
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	require.NoError(t, err)

	// Create client
	grpcClient := &client.Client{}
	// Note: In a real scenario, you'd use client.NewClient() with proper options

	cleanup := func() {
		conn.Close()
		srv.Stop()
		lis.Close()
	}

	return srv, grpcClient, cleanup
}

func TestIntegration_HealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create bufconn listener
	lis := bufconn.Listen(bufSize)
	defer lis.Close()

	// Create and start server
	srv := grpc.NewServer()
	defer srv.Stop()

	cfg := &config.Config{}
	healthHandler := handlers.NewHealthHandler(cfg, nil)
	lumov1.RegisterHealthServiceServer(srv, healthHandler)

	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// Create client
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer conn.Close()

	// Test health check
	healthClient := lumov1.NewHealthServiceClient(conn)

	t.Run("Live", func(t *testing.T) {
		resp, err := healthClient.Live(ctx, &lumov1.LiveRequest{})
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.Alive)
		assert.NotNil(t, resp.Timestamp)
	})

	t.Run("Ready", func(t *testing.T) {
		resp, err := healthClient.Ready(ctx, &lumov1.ReadyRequest{})
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.Ready)
	})

	t.Run("Check", func(t *testing.T) {
		resp, err := healthClient.Check(ctx, &lumov1.HealthCheckRequest{})
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, lumov1.HealthCheckResponse_SERVING_STATUS_SERVING, resp.Status)
		assert.NotEmpty(t, resp.Version)
	})
}

func TestIntegration_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create bufconn listener
	lis := bufconn.Listen(bufSize)
	defer lis.Close()

	// Create and start server
	srv := grpc.NewServer()
	defer srv.Stop()

	cfg := &config.Config{}
	healthHandler := handlers.NewHealthHandler(cfg, nil)
	lumov1.RegisterHealthServiceServer(srv, healthHandler)

	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// Create client
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer conn.Close()

	healthClient := lumov1.NewHealthServiceClient(conn)

	// Test concurrent requests
	numRequests := 50
	errChan := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := healthClient.Live(ctx, &lumov1.LiveRequest{})
			if err != nil {
				errChan <- fmt.Errorf("request %d failed: %w", id, err)
				return
			}
			if !resp.Alive {
				errChan <- fmt.Errorf("request %d: expected alive=true", id)
				return
			}
			errChan <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}
}

func TestIntegration_Timeouts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create bufconn listener
	lis := bufconn.Listen(bufSize)
	defer lis.Close()

	// Create and start server
	srv := grpc.NewServer()
	defer srv.Stop()

	cfg := &config.Config{}
	healthHandler := handlers.NewHealthHandler(cfg, nil)
	lumov1.RegisterHealthServiceServer(srv, healthHandler)

	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// Create client
	baseCtx := context.Background()
	conn, err := grpc.DialContext(baseCtx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer conn.Close()

	healthClient := lumov1.NewHealthServiceClient(conn)

	t.Run("ShortTimeout", func(t *testing.T) {
		// Use a very short timeout that should still work
		ctx, cancel := context.WithTimeout(baseCtx, 100*time.Millisecond)
		defer cancel()

		resp, err := healthClient.Live(ctx, &lumov1.LiveRequest{})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("CancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(baseCtx)
		cancel() // Cancel immediately

		_, err := healthClient.Live(ctx, &lumov1.LiveRequest{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

func TestIntegration_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create bufconn listener
	lis := bufconn.Listen(bufSize)
	defer lis.Close()

	// Create and start server
	srv := grpc.NewServer()
	defer srv.Stop()

	cfg := &config.Config{}
	healthHandler := handlers.NewHealthHandler(cfg, nil)
	lumov1.RegisterHealthServiceServer(srv, healthHandler)

	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// Create client
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer conn.Close()

	healthClient := lumov1.NewHealthServiceClient(conn)

	// Test with valid request
	resp, err := healthClient.Live(ctx, &lumov1.LiveRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Alive)

	// Note: For testing invalid arguments, you'd need handlers that validate input
	// The health service doesn't do much validation, so this is just a smoke test
}

func TestIntegration_ConnectionManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create bufconn listener
	lis := bufconn.Listen(bufSize)

	// Create and start server
	srv := grpc.NewServer()

	cfg := &config.Config{}
	healthHandler := handlers.NewHealthHandler(cfg, nil)
	lumov1.RegisterHealthServiceServer(srv, healthHandler)

	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	t.Run("MultipleConnections", func(t *testing.T) {
		// Create multiple connections
		ctx := context.Background()

		for i := 0; i < 5; i++ {
			conn, err := grpc.DialContext(ctx, "bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
					return lis.Dial()
				}),
				grpc.WithInsecure(),
			)
			require.NoError(t, err)

			healthClient := lumov1.NewHealthServiceClient(conn)
			resp, err := healthClient.Live(ctx, &lumov1.LiveRequest{})
			require.NoError(t, err)
			assert.True(t, resp.Alive)

			conn.Close()
		}
	})

	t.Run("ReuseConnection", func(t *testing.T) {
		ctx := context.Background()
		conn, err := grpc.DialContext(ctx, "bufnet",
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
				return lis.Dial()
			}),
			grpc.WithInsecure(),
		)
		require.NoError(t, err)
		defer conn.Close()

		healthClient := lumov1.NewHealthServiceClient(conn)

		// Make multiple requests on the same connection
		for i := 0; i < 10; i++ {
			resp, err := healthClient.Live(ctx, &lumov1.LiveRequest{})
			require.NoError(t, err)
			assert.True(t, resp.Alive)
		}
	})

	// Cleanup
	srv.Stop()
	lis.Close()
}

// BenchmarkGRPCHealthCheck benchmarks the health check RPC
func BenchmarkGRPCHealthCheck(b *testing.B) {
	// Create bufconn listener
	lis := bufconn.Listen(bufSize)
	defer lis.Close()

	// Create and start server
	srv := grpc.NewServer()
	defer srv.Stop()

	cfg := &config.Config{}
	healthHandler := handlers.NewHealthHandler(cfg, nil)
	lumov1.RegisterHealthServiceServer(srv, healthHandler)

	go func() {
		if err := srv.Serve(lis); err != nil {
			b.Logf("Server error: %v", err)
		}
	}()

	// Create client
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithInsecure(),
	)
	require.NoError(b, err)
	defer conn.Close()

	healthClient := lumov1.NewHealthServiceClient(conn)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := healthClient.Live(ctx, &lumov1.LiveRequest{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGRPCConcurrentHealthCheck benchmarks concurrent health check RPCs
func BenchmarkGRPCConcurrentHealthCheck(b *testing.B) {
	// Create bufconn listener
	lis := bufconn.Listen(bufSize)
	defer lis.Close()

	// Create and start server
	srv := grpc.NewServer()
	defer srv.Stop()

	cfg := &config.Config{}
	healthHandler := handlers.NewHealthHandler(cfg, nil)
	lumov1.RegisterHealthServiceServer(srv, healthHandler)

	go func() {
		if err := srv.Serve(lis); err != nil {
			b.Logf("Server error: %v", err)
		}
	}()

	// Create client
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithInsecure(),
	)
	require.NoError(b, err)
	defer conn.Close()

	healthClient := lumov1.NewHealthServiceClient(conn)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := healthClient.Live(ctx, &lumov1.LiveRequest{})
			if err != nil {
				b.Error(err)
			}
		}
	})
}
