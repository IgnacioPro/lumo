package agent

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestScheduler(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	scheduler := NewScheduler(logger)

	t.Run("AddTask", func(t *testing.T) {
		task := func(ctx context.Context) error {
			return nil
		}

		// Every second
		err := scheduler.AddTask("test-task", "*/1 * * * * *", task)
		assert.NoError(t, err)

		// Try to add same task again (should fail)
		err = scheduler.AddTask("test-task", "*/1 * * * * *", task)
		assert.Error(t, err)
	})

	t.Run("RemoveTask", func(t *testing.T) {
		task := func(ctx context.Context) error {
			return nil
		}

		err := scheduler.AddTask("remove-test", "*/1 * * * * *", task)
		assert.NoError(t, err)

		err = scheduler.RemoveTask("remove-test")
		assert.NoError(t, err)

		// Try to remove non-existent task
		err = scheduler.RemoveTask("nonexistent")
		assert.Error(t, err)
	})

	t.Run("ListTasks", func(t *testing.T) {
		s := NewScheduler(logger)
		task := func(ctx context.Context) error {
			return nil
		}

		err := s.AddTask("list-test-1", "*/1 * * * * *", task)
		assert.NoError(t, err)
		err = s.AddTask("list-test-2", "*/2 * * * * *", task)
		assert.NoError(t, err)

		tasks := s.ListTasks()
		assert.Len(t, tasks, 2)
	})

	t.Run("StartStop", func(t *testing.T) {
		s := NewScheduler(logger)
		executed := make(chan bool, 1)

		task := func(ctx context.Context) error {
			select {
			case executed <- true:
			default:
			}
			return nil
		}

		// Run every second
		err := s.AddTask("start-stop-test", "*/1 * * * * *", task)
		assert.NoError(t, err)

		s.Start()

		// Wait for task to execute
		select {
		case <-executed:
			// Task executed successfully
		case <-time.After(2 * time.Second):
			t.Fatal("Task did not execute within timeout")
		}

		s.Stop()
	})
}
