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

// Context key types to avoid collisions
type contextKey string

const (
	contextKeyClaims contextKey = "claims"
	contextKeyUserID contextKey = "user_id"
)

// AuthInterceptor validates JWT tokens from gRPC metadata
func AuthInterceptor(cfg *config.Config) grpc.UnaryServerInterceptor {
	// Initialize JWT manager
	jwtManager, err := auth.NewJWTManager(
		cfg.API.JWTSecret,
		cfg.API.JWTExpiration,
		cfg.API.JWTIssuer,
	)
	if err != nil {
		// If JWT manager initialization fails, return a no-op interceptor
		// This allows the server to start even if JWT is not configured
		return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			if !isPublicMethod(info.FullMethod) {
				return nil, status.Error(codes.Internal, "authentication not configured")
			}
			return handler(ctx, req)
		}
	}

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
		claims, err := jwtManager.ValidateToken(token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		// Inject claims into context
		ctx = context.WithValue(ctx, contextKeyClaims, claims)
		ctx = context.WithValue(ctx, contextKeyUserID, claims.UserID)

		return handler(ctx, req)
	}
}

// StreamAuthInterceptor validates JWT tokens for streaming RPCs
func StreamAuthInterceptor(cfg *config.Config) grpc.StreamServerInterceptor {
	// Initialize JWT manager
	jwtManager, err := auth.NewJWTManager(
		cfg.API.JWTSecret,
		cfg.API.JWTExpiration,
		cfg.API.JWTIssuer,
	)
	if err != nil {
		// If JWT manager initialization fails, return a no-op interceptor
		return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			if !isPublicMethod(info.FullMethod) {
				return status.Error(codes.Internal, "authentication not configured")
			}
			return handler(srv, ss)
		}
	}

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
		claims, err := jwtManager.ValidateToken(token)
		if err != nil {
			return status.Error(codes.Unauthenticated, "invalid token")
		}

		// Wrap stream with authenticated context
		wrappedStream := &authenticatedStream{
			ServerStream: ss,
			ctx:          context.WithValue(ss.Context(), contextKeyClaims, claims),
		}

		return handler(srv, wrappedStream)
	}
}

// authenticatedStream wraps a ServerStream with an authenticated context
type authenticatedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedStream) Context() context.Context {
	return s.ctx
}

// isPublicMethod checks if a gRPC method should skip authentication
func isPublicMethod(method string) bool {
	publicMethods := []string{
		"/lumo.v1.HealthService/Check",
		"/lumo.v1.HealthService/Ready",
		"/lumo.v1.HealthService/Live",
		"/grpc.health.v1.Health/Check",
		"/grpc.health.v1.Health/Watch",
	}

	for _, public := range publicMethods {
		if method == public {
			return true
		}
	}
	return false
}
