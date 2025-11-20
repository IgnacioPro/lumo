//nolint:staticcheck // Tests use deprecated gRPC APIs supported throughout 1.x
package grpc_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/grpc/handlers"
)

const bufSize = 1024 * 1024

func TestIntegration_HealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create bufconn listener
	lis := bufconn.Listen(bufSize)
	defer func() { _ = lis.Close() }()

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
//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.Dial("bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	// Test health check
	healthClient := lumov1.NewHealthServiceClient(conn)
	ctx := context.Background()

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
	defer func() { _ = lis.Close() }()

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
//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.Dial("bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

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
	defer func() { _ = lis.Close() }()

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
//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.Dial("bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

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
	defer func() { _ = lis.Close() }()

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
//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.Dial("bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	healthClient := lumov1.NewHealthServiceClient(conn)
	ctx := context.Background()

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

		for i := 0; i < 5; i++ {
//nolint:staticcheck // Deprecated API supported throughout 1.x
			conn, err := grpc.Dial("bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
					return lis.Dial()
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			healthClient := lumov1.NewHealthServiceClient(conn)
	ctx := context.Background()
			resp, err := healthClient.Live(ctx, &lumov1.LiveRequest{})
			require.NoError(t, err)
			assert.True(t, resp.Alive)

			_ = conn.Close()
		}
	})

	t.Run("ReuseConnection", func(t *testing.T) {
//nolint:staticcheck // Deprecated API supported throughout 1.x
		conn, err := grpc.Dial("bufnet",
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
				return lis.Dial()
			}),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		healthClient := lumov1.NewHealthServiceClient(conn)
	ctx := context.Background()

		// Make multiple requests on the same connection
		for i := 0; i < 10; i++ {
			resp, err := healthClient.Live(ctx, &lumov1.LiveRequest{})
			require.NoError(t, err)
			assert.True(t, resp.Alive)
		}
	})

	// Cleanup
	srv.Stop()
	_ = lis.Close()
}

// BenchmarkGRPCHealthCheck benchmarks the health check RPC
func BenchmarkGRPCHealthCheck(b *testing.B) {
	// Create bufconn listener
	lis := bufconn.Listen(bufSize)
	defer func() { _ = lis.Close() }()

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
//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.Dial("bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(b, err)
	defer func() { _ = conn.Close() }()

	healthClient := lumov1.NewHealthServiceClient(conn)
	ctx := context.Background()

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
	defer func() { _ = lis.Close() }()

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
//nolint:staticcheck // Deprecated API supported throughout 1.x
	conn, err := grpc.Dial("bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(b, err)
	defer func() { _ = conn.Close() }()

	healthClient := lumov1.NewHealthServiceClient(conn)
	ctx := context.Background()

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
