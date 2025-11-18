package remediation

import (
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// Test action factory functions

func TestNewCleanLogsActionFactory(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	factory := NewCleanLogsActionFactory()

	t.Run("creates action with valid parameters", func(t *testing.T) {
		params := map[string]interface{}{
			"older_than_days": 30,
			"compress":        true,
		}

		action, err := factory(params, logger)

		if err != nil {
			t.Fatalf("Factory() unexpected error: %v", err)
		}

		if action == nil {
			t.Fatal("Factory() returned nil action")
		}

		if action.Category() != CategoryDisk {
			t.Errorf("Action category = %v, want %v", action.Category(), CategoryDisk)
		}
	})

	t.Run("uses defaults when parameters missing", func(t *testing.T) {
		params := map[string]interface{}{}

		action, err := factory(params, logger)

		if err != nil {
			t.Fatalf("Factory() unexpected error: %v", err)
		}

		if action == nil {
			t.Fatal("Factory() returned nil action")
		}
	})

	t.Run("handles invalid older_than_days type", func(t *testing.T) {
		params := map[string]interface{}{
			"older_than_days": "invalid",
		}

		action, err := factory(params, logger)

		// Should still create action with defaults
		if err != nil {
			t.Fatalf("Factory() unexpected error: %v", err)
		}

		if action == nil {
			t.Fatal("Factory() returned nil action")
		}
	})
}

func TestNewKillProcessActionFactory(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	factory := NewKillProcessActionFactory()

	t.Run("creates action with valid parameters", func(t *testing.T) {
		params := map[string]interface{}{
			"pid":          1234,
			"process_name": "test-process",
			"signal":       "SIGTERM",
		}

		action, err := factory(params, logger)

		if err != nil {
			t.Fatalf("Factory() unexpected error: %v", err)
		}

		if action == nil {
			t.Fatal("Factory() returned nil action")
		}

		if action.Category() != CategoryProcess {
			t.Errorf("Action category = %v, want %v", action.Category(), CategoryProcess)
		}
	})

	t.Run("fails when pid parameter missing", func(t *testing.T) {
		params := map[string]interface{}{
			"process_name": "test-process",
		}

		action, err := factory(params, logger)

		if err == nil {
			t.Fatal("Factory() expected error when pid missing, got nil")
		}

		if action != nil {
			t.Error("Factory() should return nil action on error")
		}

		if !strings.Contains(err.Error(), "pid parameter is required") {
			t.Errorf("Error message = %q, want to mention pid required", err.Error())
		}
	})

	t.Run("uses defaults for optional parameters", func(t *testing.T) {
		params := map[string]interface{}{
			"pid": 5678,
		}

		action, err := factory(params, logger)

		if err != nil {
			t.Fatalf("Factory() unexpected error: %v", err)
		}

		if action == nil {
			t.Fatal("Factory() returned nil action")
		}
	})
}

func TestNewKillProcessGracefulActionFactory(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	factory := NewKillProcessGracefulActionFactory()

	t.Run("creates action with valid parameters", func(t *testing.T) {
		params := map[string]interface{}{
			"pid":             1234,
			"process_name":    "test-process",
			"timeout_seconds": 15,
		}

		action, err := factory(params, logger)

		if err != nil {
			t.Fatalf("Factory() unexpected error: %v", err)
		}

		if action == nil {
			t.Fatal("Factory() returned nil action")
		}
	})

	t.Run("fails when pid parameter missing", func(t *testing.T) {
		params := map[string]interface{}{
			"process_name": "test-process",
		}

		_, err := factory(params, logger)

		if err == nil {
			t.Fatal("Factory() expected error when pid missing, got nil")
		}

		if !strings.Contains(err.Error(), "pid parameter is required") {
			t.Errorf("Error message = %q, want to mention pid required", err.Error())
		}
	})

	t.Run("accepts timeout as Duration", func(t *testing.T) {
		params := map[string]interface{}{
			"pid":     1234,
			"timeout": 20 * time.Second,
		}

		action, err := factory(params, logger)

		if err != nil {
			t.Fatalf("Factory() unexpected error: %v", err)
		}

		if action == nil {
			t.Fatal("Factory() returned nil action")
		}
	})
}

func TestNewRestartServiceActionFactory(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	factory := NewRestartServiceActionFactory()

	t.Run("creates action with valid parameters", func(t *testing.T) {
		params := map[string]interface{}{
			"service_name": "nginx",
		}

		action, err := factory(params, logger)

		if err != nil {
			t.Fatalf("Factory() unexpected error: %v", err)
		}

		if action == nil {
			t.Fatal("Factory() returned nil action")
		}

		if action.Category() != CategoryService {
			t.Errorf("Action category = %v, want %v", action.Category(), CategoryService)
		}
	})

	t.Run("fails when service_name parameter missing", func(t *testing.T) {
		params := map[string]interface{}{}

		_, err := factory(params, logger)

		if err == nil {
			t.Fatal("Factory() expected error when service_name missing, got nil")
		}

		if !strings.Contains(err.Error(), "service_name parameter is required") {
			t.Errorf("Error message = %q, want to mention service_name required", err.Error())
		}
	})

	t.Run("fails when service_name is empty", func(t *testing.T) {
		params := map[string]interface{}{
			"service_name": "",
		}

		_, err := factory(params, logger)

		if err == nil {
			t.Fatal("Factory() expected error when service_name empty, got nil")
		}
	})
}

func TestNewStartServiceActionFactory(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	factory := NewStartServiceActionFactory()

	t.Run("creates action with valid parameters", func(t *testing.T) {
		params := map[string]interface{}{
			"service_name": "postgresql",
		}

		action, err := factory(params, logger)

		if err != nil {
			t.Fatalf("Factory() unexpected error: %v", err)
		}

		if action == nil {
			t.Fatal("Factory() returned nil action")
		}
	})

	t.Run("fails when service_name parameter missing", func(t *testing.T) {
		params := map[string]interface{}{}

		_, err := factory(params, logger)

		if err == nil {
			t.Fatal("Factory() expected error when service_name missing, got nil")
		}

		if !strings.Contains(err.Error(), "service_name parameter is required") {
			t.Errorf("Error message = %q, want to mention service_name required", err.Error())
		}
	})
}

func TestNewStopServiceActionFactory(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	factory := NewStopServiceActionFactory()

	t.Run("creates action with valid parameters", func(t *testing.T) {
		params := map[string]interface{}{
			"service_name": "apache2",
		}

		action, err := factory(params, logger)

		if err != nil {
			t.Fatalf("Factory() unexpected error: %v", err)
		}

		if action == nil {
			t.Fatal("Factory() returned nil action")
		}
	})

	t.Run("fails when service_name parameter missing", func(t *testing.T) {
		params := map[string]interface{}{}

		_, err := factory(params, logger)

		if err == nil {
			t.Fatal("Factory() expected error when service_name missing, got nil")
		}

		if !strings.Contains(err.Error(), "service_name parameter is required") {
			t.Errorf("Error message = %q, want to mention service_name required", err.Error())
		}
	})
}
