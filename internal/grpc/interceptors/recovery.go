package interceptors

import (
	"context"
	"runtime/debug"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoveryInterceptor recovers from panics in unary RPC handlers
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

// StreamRecoveryInterceptor recovers from panics in streaming RPC handlers
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
