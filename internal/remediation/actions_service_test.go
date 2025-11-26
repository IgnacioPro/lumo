package remediation

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestRestartServiceAction(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(new(mockWriter)) // Suppress logging during tests
	ctx := context.Background()
	serviceName := "nginx"
	serviceNameQuoted := "'nginx'"
	// After normalization, the service name becomes 'nginx.service'
	normalizedServiceNameQuoted := "'nginx.service'"

	t.Run("Validates successfully", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "which systemctl").Return("/bin/systemctl", "", 0, nil).Once()
		mockExecutor.On("ExecuteWithContext", ctx, fmt.Sprintf("systemctl list-unit-files %s", normalizedServiceNameQuoted)).Return("nginx.service enabled", "", 0, nil).Once()

		action := NewRestartServiceAction(serviceName, logger)
		err := action.Validate(ctx, mockExecutor)
		assert.NoError(t, err)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Validation fails if service does not exist", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "which systemctl").Return("/bin/systemctl", "", 0, nil).Once()
		mockExecutor.On("ExecuteWithContext", ctx, fmt.Sprintf("systemctl list-unit-files %s", normalizedServiceNameQuoted)).Return("", "", 1, errors.New("not found")).Once()

		action := NewRestartServiceAction(serviceName, logger)
		err := action.Validate(ctx, mockExecutor)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Execute restarts service successfully", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		// Get status for rollback
		mockExecutor.On("ExecuteWithContext", ctx, fmt.Sprintf("systemctl show -p ActiveState --value %s", serviceNameQuoted)).Return("active", "", 0, nil).Once()
		// Restart command
		mockExecutor.On("ExecuteWithContext", ctx, fmt.Sprintf("systemctl restart %s", serviceNameQuoted)).Return("", "", 0, nil).Once()

		action := NewRestartServiceAction(serviceName, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.NoError(t, err)
		assert.Equal(t, StatusSuccess, result.Status)
		assert.Contains(t, result.Message, "Successfully restarted service")
		assert.Equal(t, "active", result.RollbackData["previous_status"])
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Execute fails when restart command fails", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, fmt.Sprintf("systemctl show -p ActiveState --value %s", serviceNameQuoted)).Return("active", "", 0, nil).Once()
		mockExecutor.On("ExecuteWithContext", ctx, fmt.Sprintf("systemctl restart %s", serviceNameQuoted)).Return("", "failed to restart", 1, errors.New("failed")).Once()

		action := NewRestartServiceAction(serviceName, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.Error(t, err)
		assert.Equal(t, StatusFailed, result.Status)
		assert.Contains(t, result.Message, "Failed to restart service")
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Rollback stops service if it was inactive", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, fmt.Sprintf("systemctl stop %s", serviceNameQuoted)).Return("", "", 0, nil).Once()

		action := NewRestartServiceAction(serviceName, logger)
		result := &ActionResult{
			RollbackData: map[string]interface{}{
				"previous_status": "inactive",
			},
		}
		err := action.Rollback(ctx, mockExecutor, result)
		assert.NoError(t, err)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Rollback does nothing if service was active", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		// No calls expected

		action := NewRestartServiceAction(serviceName, logger)
		result := &ActionResult{
			RollbackData: map[string]interface{}{
				"previous_status": "active",
			},
		}
		err := action.Rollback(ctx, mockExecutor, result)
		assert.NoError(t, err)
		mockExecutor.AssertExpectations(t)
	})
}

func TestStopServiceAction(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(new(mockWriter)) // Suppress logging during tests
	ctx := context.Background()
	serviceName := "apache2"
	serviceNameQuoted := "'apache2'"

	t.Run("Execute stops service successfully", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, fmt.Sprintf("systemctl stop %s", serviceNameQuoted)).Return("", "", 0, nil).Once()

		action := NewStopServiceAction(serviceName, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.NoError(t, err)
		assert.Equal(t, StatusSuccess, result.Status)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Rollback starts service", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, fmt.Sprintf("systemctl start %s", serviceNameQuoted)).Return("", "", 0, nil).Once()

		action := NewStopServiceAction(serviceName, logger)
		result := &ActionResult{} // Data not needed for simple stop rollback
		err := action.Rollback(ctx, mockExecutor, result)
		assert.NoError(t, err)
		mockExecutor.AssertExpectations(t)
	})
}

func TestStartServiceAction(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(new(mockWriter)) // Suppress logging during tests
	ctx := context.Background()
	serviceName := "postgresql"
	serviceNameQuoted := "'postgresql'"

	t.Run("Execute starts service successfully", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, fmt.Sprintf("systemctl start %s", serviceNameQuoted)).Return("", "", 0, nil).Once()

		action := NewStartServiceAction(serviceName, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.NoError(t, err)
		assert.Equal(t, StatusSuccess, result.Status)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Rollback stops service", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, fmt.Sprintf("systemctl stop %s", serviceNameQuoted)).Return("", "", 0, nil).Once()

		action := NewStartServiceAction(serviceName, logger)
		result := &ActionResult{}
		err := action.Rollback(ctx, mockExecutor, result)
		assert.NoError(t, err)
		mockExecutor.AssertExpectations(t)
	})
}
