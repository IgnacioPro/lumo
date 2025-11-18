package main

import (
	"context"
	"testing"
)

func TestGetRootContext(t *testing.T) {
	// Save original values
	originalCtx := rootCtx
	originalCancel := rootCancel
	defer func() {
		rootCtx = originalCtx
		rootCancel = originalCancel
	}()

	t.Run("returns background when rootCtx is nil", func(t *testing.T) {
		rootCtx = nil
		rootCancel = nil

		ctx := getRootContext()
		if ctx == nil {
			t.Error("getRootContext() returned nil, expected context.Background()")
		}

		// Verify it's a valid context
		select {
		case <-ctx.Done():
			t.Error("context should not be done")
		default:
			// Expected behavior
		}
	})

	t.Run("returns rootCtx when initialized", func(t *testing.T) {
		// Initialize a test context
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		rootCtx = ctx
		rootCancel = cancel

		result := getRootContext()
		if result != ctx {
			t.Error("getRootContext() did not return the initialized rootCtx")
		}
	})
}

// Note: Execute function is tested via integration tests
// Direct unit testing would require mocking os.Exit which is complex
