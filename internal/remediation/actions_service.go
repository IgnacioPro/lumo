package remediation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/sirupsen/logrus"
)

// RestartServiceAction restarts a systemd service.
type RestartServiceAction struct {
	*BaseAction
	serviceName string
}

// NewRestartServiceAction creates a new restart service action.
func NewRestartServiceAction(serviceName string, logger *logrus.Logger) *RestartServiceAction {
	return &RestartServiceAction{
		BaseAction: NewBaseAction(
			fmt.Sprintf("service.restart.%s", serviceName),
			fmt.Sprintf("Restart %s service", serviceName),
			fmt.Sprintf("Restarts the %s systemd service to recover from a failed state", serviceName),
			CategoryService,
			RiskModerate,
			true, // reversible (can be stopped if restart causes issues)
			fmt.Sprintf("Service %s will be restarted. Dependent services may be briefly unavailable.", serviceName),
			logger,
		),
		serviceName: serviceName,
	}
}

// NewRestartServiceActionFactory returns a factory for creating RestartServiceAction instances.
func NewRestartServiceActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		serviceName, ok := params["service_name"].(string)
		if !ok || serviceName == "" {
			return nil, fmt.Errorf("service_name parameter is required")
		}
		return NewRestartServiceAction(serviceName, logger), nil
	}
}

// Validate checks if the service exists and can be restarted.
func (a *RestartServiceAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	// Check if systemctl is available
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, "which systemctl")
	if err != nil || exitCode != 0 {
		return fmt.Errorf("systemctl not available: %s %s", stdout, stderr)
	}

	// Check if service exists
	cmd := fmt.Sprintf("systemctl list-unit-files --type=service | grep -E '^%s\\.service'", a.serviceName)
	stdout, stderr, exitCode, err = executor.ExecuteWithContext(ctx, cmd)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("service %s not found: %s %s", a.serviceName, stdout, stderr)
	}

	return nil
}

// Execute restarts the service.
func (a *RestartServiceAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	// Get service status before restart
	statusBefore, _ := a.getServiceStatus(ctx, executor)

	// Restart the service
	cmd := fmt.Sprintf("systemctl restart %s", a.serviceName)
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)

	result.Output = fmt.Sprintf("stdout: %s\nstderr: %s", stdout, stderr)

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to restart service %s", a.serviceName)
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("systemctl restart failed: %s", stderr)
	}

	// Verify service is running
	time.Sleep(2 * time.Second) // Give service time to start
	statusAfter, err := a.getServiceStatus(ctx, executor)
	if err != nil {
		result.Status = StatusFailed
		result.Message = "Service restarted but unable to verify status"
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully restarted %s service", a.serviceName)
	result.ChangesApplied = []string{
		fmt.Sprintf("Service %s restarted", a.serviceName),
		fmt.Sprintf("Status: %s → %s", statusBefore, statusAfter),
	}
	result.RollbackData = map[string]interface{}{
		"previous_status": statusBefore,
		"service_name":    a.serviceName,
	}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback stops the service (reversing the restart).
func (a *RestartServiceAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	if result.RollbackData == nil {
		return fmt.Errorf("no rollback data available")
	}

	previousStatus, ok := result.RollbackData["previous_status"].(string)
	if !ok {
		previousStatus = "unknown"
	}

	// If service was stopped before, stop it again
	if strings.Contains(previousStatus, "inactive") || strings.Contains(previousStatus, "failed") {
		cmd := fmt.Sprintf("systemctl stop %s", a.serviceName)
		_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
		if err != nil || exitCode != 0 {
			return fmt.Errorf("failed to stop service during rollback: %s", stderr)
		}
	}

	return nil
}

// getServiceStatus returns the current status of the service.
func (a *RestartServiceAction) getServiceStatus(ctx context.Context, executor diagnostics.CommandExecutor) (string, error) {
	cmd := fmt.Sprintf("systemctl is-active %s", a.serviceName)
	stdout, _, _, _ := executor.ExecuteWithContext(ctx, cmd)
	return strings.TrimSpace(stdout), nil
}

// StartServiceAction starts a systemd service.
type StartServiceAction struct {
	*BaseAction
	serviceName string
}

// NewStartServiceAction creates a new start service action.
func NewStartServiceAction(serviceName string, logger *logrus.Logger) *StartServiceAction {
	return &StartServiceAction{
		BaseAction: NewBaseAction(
			fmt.Sprintf("service.start.%s", serviceName),
			fmt.Sprintf("Start %s service", serviceName),
			fmt.Sprintf("Starts the %s systemd service if it is currently stopped", serviceName),
			CategoryService,
			RiskSafe,
			true,
			fmt.Sprintf("Service %s will be started", serviceName),
			logger,
		),
		serviceName: serviceName,
	}
}

// NewStartServiceActionFactory returns a factory for creating StartServiceAction instances.
func NewStartServiceActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		serviceName, ok := params["service_name"].(string)
		if !ok || serviceName == "" {
			return nil, fmt.Errorf("service_name parameter is required")
		}
		return NewStartServiceAction(serviceName, logger), nil
	}
}

// Validate checks if the service exists.
func (a *StartServiceAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	// Check if systemctl is available
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, "which systemctl")
	if err != nil || exitCode != 0 {
		return fmt.Errorf("systemctl not available: %s %s", stdout, stderr)
	}

	// Check if service exists
	cmd := fmt.Sprintf("systemctl list-unit-files --type=service | grep -E '^%s\\.service'", a.serviceName)
	stdout, stderr, exitCode, err = executor.ExecuteWithContext(ctx, cmd)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("service %s not found: %s %s", a.serviceName, stdout, stderr)
	}

	return nil
}

// Execute starts the service.
func (a *StartServiceAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	cmd := fmt.Sprintf("systemctl start %s", a.serviceName)
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)

	result.Output = fmt.Sprintf("stdout: %s\nstderr: %s", stdout, stderr)

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to start service %s", a.serviceName)
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("systemctl start failed: %s", stderr)
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully started %s service", a.serviceName)
	result.ChangesApplied = []string{fmt.Sprintf("Service %s started", a.serviceName)}
	result.RollbackData = map[string]interface{}{
		"service_name": a.serviceName,
	}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback stops the service.
func (a *StartServiceAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	cmd := fmt.Sprintf("systemctl stop %s", a.serviceName)
	_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to stop service during rollback: %s", stderr)
	}
	return nil
}

// StopServiceAction stops a systemd service.
type StopServiceAction struct {
	*BaseAction
	serviceName string
}

// NewStopServiceAction creates a new stop service action.
func NewStopServiceAction(serviceName string, logger *logrus.Logger) *StopServiceAction {
	return &StopServiceAction{
		BaseAction: NewBaseAction(
			fmt.Sprintf("service.stop.%s", serviceName),
			fmt.Sprintf("Stop %s service", serviceName),
			fmt.Sprintf("Stops the %s systemd service if it is currently running", serviceName),
			CategoryService,
			RiskModerate,
			true,
			fmt.Sprintf("Service %s will be stopped. Dependent services may fail.", serviceName),
			logger,
		),
		serviceName: serviceName,
	}
}

// NewStopServiceActionFactory returns a factory for creating StopServiceAction instances.
func NewStopServiceActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		serviceName, ok := params["service_name"].(string)
		if !ok || serviceName == "" {
			return nil, fmt.Errorf("service_name parameter is required")
		}
		return NewStopServiceAction(serviceName, logger), nil
	}
}

// Validate checks if the service exists.
func (a *StopServiceAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	// Check if systemctl is available
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, "which systemctl")
	if err != nil || exitCode != 0 {
		return fmt.Errorf("systemctl not available: %s %s", stdout, stderr)
	}

	return nil
}

// Execute stops the service.
func (a *StopServiceAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	cmd := fmt.Sprintf("systemctl stop %s", a.serviceName)
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)

	result.Output = fmt.Sprintf("stdout: %s\nstderr: %s", stdout, stderr)

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to stop service %s", a.serviceName)
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("systemctl stop failed: %s", stderr)
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully stopped %s service", a.serviceName)
	result.ChangesApplied = []string{fmt.Sprintf("Service %s stopped", a.serviceName)}
	result.RollbackData = map[string]interface{}{
		"service_name": a.serviceName,
	}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback starts the service.
func (a *StopServiceAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	cmd := fmt.Sprintf("systemctl start %s", a.serviceName)
	_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to start service during rollback: %s", stderr)
	}
	return nil
}
