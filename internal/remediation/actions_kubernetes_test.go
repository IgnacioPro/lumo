package remediation

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestK8sRolloutRestartAction(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(new(mockWriter))
	ctx := context.Background()

	ns := "default"
	resName := "my-app"
	resType := "deployment"

	t.Run("Validates successfully", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "which kubectl").Return("/usr/bin/kubectl", "", 0, nil).Once()
		// Expect quoted resource type and namespace as shellQuote is used in implementation
		// Use explicit string literals to avoid ambiguity
		mockExecutor.On("ExecuteWithContext", ctx, "kubectl get 'deployment' 'my-app' -n 'default'").Return("NAME...", "", 0, nil).Once()

		action := NewK8sRolloutRestartAction(ns, resType, resName, logger)
		err := action.Validate(ctx, mockExecutor)
		assert.NoError(t, err)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Validation fails if kubectl missing", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "which kubectl").Return("", "", 1, errors.New("not found")).Once()

		action := NewK8sRolloutRestartAction(ns, resType, resName, logger)
		err := action.Validate(ctx, mockExecutor)
		assert.Error(t, err)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Execute restarts successfully", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		// Explicit string literal
		cmd := "kubectl rollout restart 'deployment' 'my-app' -n 'default'"
		mockExecutor.On("ExecuteWithContext", ctx, cmd).Return("restarted", "", 0, nil).Once()

		action := NewK8sRolloutRestartAction(ns, resType, resName, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.NoError(t, err)
		assert.Equal(t, StatusSuccess, result.Status)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Rollback performs undo", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		cmd := "kubectl rollout undo 'deployment' 'my-app' -n 'default'"
		mockExecutor.On("ExecuteWithContext", ctx, cmd).Return("rolled back", "", 0, nil).Once()

		action := NewK8sRolloutRestartAction(ns, resType, resName, logger)
		err := action.Rollback(ctx, mockExecutor, nil)
		assert.NoError(t, err)
		mockExecutor.AssertExpectations(t)
	})
}

func TestK8sDeletePodAction(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(new(mockWriter))
	ctx := context.Background()

	ns := "kube-system"
	pod := "coredns-123"

	t.Run("Validates successfully", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "which kubectl").Return("/usr/bin/kubectl", "", 0, nil).Once()
		// Using explicit string literal
		mockExecutor.On("ExecuteWithContext", ctx, "kubectl get pod 'coredns-123' -n 'kube-system'").Return("NAME...", "", 0, nil).Once()

		action := NewK8sDeletePodAction(ns, pod, false, logger)
		err := action.Validate(ctx, mockExecutor)
		assert.NoError(t, err)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Execute deletes pod successfully", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		cmd := "kubectl delete pod 'coredns-123' -n 'kube-system'"

		mockExecutor.On("ExecuteWithContext", ctx, cmd).Return("deleted", "", 0, nil).Once()

		action := NewK8sDeletePodAction(ns, pod, false, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.NoError(t, err)
		assert.Equal(t, StatusSuccess, result.Status)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Execute deletes pod with force", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		cmd := "kubectl delete pod 'coredns-123' -n 'kube-system' --force --grace-period=0"

		mockExecutor.On("ExecuteWithContext", ctx, cmd).Return("deleted", "", 0, nil).Once()

		action := NewK8sDeletePodAction(ns, pod, true, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.NoError(t, err)
		assert.Equal(t, StatusSuccess, result.Status)
		mockExecutor.AssertExpectations(t)
	})
}
