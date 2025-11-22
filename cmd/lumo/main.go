package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ignacio/lumo/internal/observability"
	"github.com/ignacio/lumo/internal/version"
)

func main() {
	// Initialize distributed tracing
	shutdown, err := observability.InitTracer("lumo", version.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize tracer: %v\n", err)
	} else {
		defer func() {
			if err := shutdown(context.Background()); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to shutdown tracer: %v\n", err)
			}
		}()
	}

	Execute()
}
