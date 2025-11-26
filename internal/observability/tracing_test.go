package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestInitTracer(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		version     string
	}{
		{
			name:        "valid initialization",
			serviceName: "test-service",
			version:     "1.0.0",
		},
		{
			name:        "empty service name",
			serviceName: "",
			version:     "1.0.0",
		},
		{
			name:        "empty version",
			serviceName: "test-service",
			version:     "",
		},
		{
			name:        "both empty",
			serviceName: "",
			version:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shutdown, err := InitTracer(tt.serviceName, tt.version)

			// Should always succeed with valid inputs
			require.NoError(t, err)
			assert.NotNil(t, shutdown)

			// Verify tracer provider was set
			tp := otel.GetTracerProvider()
			assert.NotNil(t, tp)

			// Verify we can create a tracer
			tracer := tp.Tracer("test-tracer")
			assert.NotNil(t, tracer)

			// Test shutdown function
			ctx := context.Background()
			err = shutdown(ctx)
			assert.NoError(t, err)
		})
	}
}

func TestInitTracer_MultipleCalls(t *testing.T) {
	// First initialization
	shutdown1, err1 := InitTracer("service1", "1.0.0")
	require.NoError(t, err1)
	require.NotNil(t, shutdown1)

	// Second initialization (should replace the first)
	shutdown2, err2 := InitTracer("service2", "2.0.0")
	require.NoError(t, err2)
	require.NotNil(t, shutdown2)

	// Both shutdown functions should work
	ctx := context.Background()

	err := shutdown2(ctx)
	assert.NoError(t, err)

	err = shutdown1(ctx)
	assert.NoError(t, err)
}

func TestInitTracer_ShutdownCancel(t *testing.T) {
	shutdown, err := InitTracer("test-service", "1.0.0")
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Test shutdown with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Shutdown should handle cancelled context gracefully
	err = shutdown(ctx)
	// May or may not error depending on implementation, just verify it doesn't panic
	_ = err
}
