package interceptors

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggingInterceptor logs all unary RPC calls
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

// StreamLoggingInterceptor logs all streaming RPC calls
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
