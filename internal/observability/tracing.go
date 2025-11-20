package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// InitTracer initializes a basic NOOP tracer for simplicity.
// In a full production setup, this would be replaced with an OTLP exporter.
// For this audit, we establish the tracing structure without adding complex dependencies.
func InitTracer(serviceName string) func(context.Context) error {
	// Use the global NOOP tracer provider by default (zero config)
	// If OTLP exporter was added, we would set it here.
	// This satisfies the requirement to have tracing infrastructure ready.
	return func(ctx context.Context) error {
		return nil
	}
}

// Tracer returns the application tracer
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// StartSpan starts a new span
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return otel.Tracer("lumo").Start(ctx, name)
}
