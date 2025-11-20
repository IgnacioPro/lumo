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

// Server wraps the gRPC server with Lumo-specific configuration
type Server struct {
	config     *config.Config
	grpcServer *grpc.Server
	listener   net.Listener
	log        *logrus.Logger
}

// ServerOptions contains configuration for the gRPC server
type ServerOptions struct {
	Port       int
	TLSEnabled bool
	CertFile   string
	KeyFile    string
	MaxMsgSize int // Max message size in bytes
}

// NewServer creates a new gRPC server with the specified options
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

// Start starts the gRPC server and blocks until it stops
func (s *Server) Start() error {
	s.log.WithField("addr", s.listener.Addr().String()).Info("Starting gRPC server")
	return s.grpcServer.Serve(s.listener)
}

// GracefulStop gracefully stops the gRPC server with a timeout
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

// GetGRPCServer returns the underlying gRPC server for registering services
func (s *Server) GetGRPCServer() *grpc.Server {
	return s.grpcServer
}
