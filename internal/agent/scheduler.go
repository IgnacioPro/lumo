package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

// TaskFunc is a function that runs on a schedule
type TaskFunc func(ctx context.Context) error

// Scheduler manages periodic task execution using cron expressions
type Scheduler struct {
	cron   *cron.Cron
	logger *logrus.Logger
	mu     sync.Mutex
	tasks  map[string]cron.EntryID
	ctx    context.Context
	cancel context.CancelFunc
}

// NewScheduler creates a new scheduler
func NewScheduler(logger *logrus.Logger) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		cron: cron.New(cron.WithParser(cron.NewParser(
			cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
		))),
		logger: logger,
		tasks:  make(map[string]cron.EntryID),
		ctx:    ctx,
		cancel: cancel,
	}
}

// AddTask adds a task with the given schedule (cron expression)
func (s *Scheduler) AddTask(name, schedule string, task TaskFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if task already exists
	if _, exists := s.tasks[name]; exists {
		return fmt.Errorf("task %s already exists", name)
	}

	// Wrap task function to handle context and logging
	wrappedTask := func() {
		// Create a timeout context for this task execution
		// Use 30 minutes as a reasonable default for long-running tasks
		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Minute)
		defer cancel()

		start := time.Now()

		s.logger.WithFields(logrus.Fields{
			"task": name,
		}).Debug("Running scheduled task")

		if err := task(ctx); err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"task":     name,
				"duration": time.Since(start),
			}).Error("Scheduled task failed")
		} else {
			s.logger.WithFields(logrus.Fields{
				"task":     name,
				"duration": time.Since(start),
			}).Debug("Scheduled task completed")
		}
	}

	// Add to cron scheduler
	entryID, err := s.cron.AddFunc(schedule, wrappedTask)
	if err != nil {
		return fmt.Errorf("failed to add task to scheduler: %w", err)
	}

	s.tasks[name] = entryID
	s.logger.WithFields(logrus.Fields{
		"task":     name,
		"schedule": schedule,
	}).Info("Scheduled task added")

	return nil
}

// RemoveTask removes a scheduled task
func (s *Scheduler) RemoveTask(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, exists := s.tasks[name]
	if !exists {
		return fmt.Errorf("task %s not found", name)
	}

	s.cron.Remove(entryID)
	delete(s.tasks, name)

	s.logger.WithField("task", name).Info("Scheduled task removed")
	return nil
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.logger.Info("Starting scheduler")
	s.cron.Start()
}

// Stop stops the scheduler and waits for running tasks to complete
func (s *Scheduler) Stop() {
	s.logger.Info("Stopping scheduler")
	// Cancel the scheduler context to signal tasks to stop
	s.cancel()
	// Stop the cron scheduler and wait for running tasks
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("Scheduler stopped")
}

// GetNextRun returns the next run time for a task
func (s *Scheduler) GetNextRun(name string) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, exists := s.tasks[name]
	if !exists {
		return time.Time{}, fmt.Errorf("task %s not found", name)
	}

	entry := s.cron.Entry(entryID)
	return entry.Next, nil
}

// ListTasks returns a list of all scheduled tasks
func (s *Scheduler) ListTasks() []TaskInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	var tasks []TaskInfo
	for name, entryID := range s.tasks {
		entry := s.cron.Entry(entryID)
		tasks = append(tasks, TaskInfo{
			Name:     name,
			Next:     entry.Next,
			Previous: entry.Prev,
		})
	}

	return tasks
}

// TaskInfo contains information about a scheduled task
type TaskInfo struct {
	Name     string
	Next     time.Time
	Previous time.Time
}
