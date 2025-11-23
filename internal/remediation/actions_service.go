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

// NewRestartServiceAction creates a new RestartServiceAction.
func NewRestartServiceAction(serviceName string, logger *logrus.Logger) *RestartServiceAction {
	return &RestartServiceAction{
		BaseAction: NewBaseAction(
			fmt.Sprintf("service.restart.%s", serviceName),
			fmt.Sprintf("Restart service %s", serviceName),
			fmt.Sprintf("Restarts the systemd service '%s'", serviceName),
			CategoryService,
			RiskModerate,
			true, // Reversible by stopping the service (if it was stopped) or restarting it again
			fmt.Sprintf("Service '%s' will be restarted. Short downtime may occur.", serviceName),
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

// Validate checks if the service exists and systemd is available.
func (a *RestartServiceAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	if err := validateServiceName(a.serviceName); err != nil {
		return err
	}

	// Check if systemctl is available
	_, _, exitCode, _ := executor.ExecuteWithContext(ctx, "which systemctl")
	if exitCode != 0 {
		return fmt.Errorf("systemctl command not found")
	}

	// Check if service exists
	// Normalize service name - add .service if not already present
	serviceName := a.serviceName
	if !strings.HasSuffix(serviceName, ".service") {
		serviceName += ".service"
	}

	cmd := fmt.Sprintf("systemctl list-unit-files %s", shellQuote(serviceName))
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, cmd)
	if exitCode != 0 || !strings.Contains(stdout, serviceName) {
		return fmt.Errorf("service '%s' not found", a.serviceName)
	}

	return nil
}

// Execute restarts the service.
func (a *RestartServiceAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	// Get current status for rollback
	status, _ := a.getServiceStatus(ctx, executor)
	result.RollbackData = map[string]interface{}{
		"previous_status": status,
	}

	cmd := fmt.Sprintf("systemctl restart %s", shellQuote(a.serviceName))
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	result.Output = fmt.Sprintf("stdout: %s\nstderr: %s", stdout, stderr)

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to restart service '%s'", a.serviceName)
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("systemctl restart failed: %s", stderr)
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully restarted service '%s'", a.serviceName)
	result.ChangesApplied = []string{fmt.Sprintf("Restarted service '%s'", a.serviceName)}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback attempts to restore the service to its previous state.
func (a *RestartServiceAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	prevStatus, ok := result.RollbackData["previous_status"].(string)
	if !ok {
		return fmt.Errorf("no rollback data available")
	}

	var cmd string
	if prevStatus == "inactive" || prevStatus == "failed" {
		cmd = fmt.Sprintf("systemctl stop %s", shellQuote(a.serviceName))
	} else {
		// If it was active, we might want to restart it again or just leave it running?
		// Usually rollback for restart means "if restart broke it, try to fix it"
		// But if restart succeeded, rollback might mean stopping it?
		// Let's assume rollback means reverting state. If it was stopped, stop it.
		// If it was running, restart implies it is running now, so no action needed unless we want to ensure config reload?
		return nil // Nothing to do if it was already active
	}

	_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("rollback failed: %s", stderr)
	}

	return nil
}

// getServiceStatus returns the ActiveState of the service
func (a *RestartServiceAction) getServiceStatus(ctx context.Context, executor diagnostics.CommandExecutor) (string, error) {
	cmd := fmt.Sprintf("systemctl show -p ActiveState --value %s", shellQuote(a.serviceName))
	stdout, _, _, err := executor.ExecuteWithContext(ctx, cmd)
	if err != nil {
		return "unknown", err
	}
	return strings.TrimSpace(stdout), nil
}

// StopServiceAction stops a systemd service.
type StopServiceAction struct {
	*BaseAction
	serviceName string
}

// NewStopServiceAction creates a new StopServiceAction.
func NewStopServiceAction(serviceName string, logger *logrus.Logger) *StopServiceAction {
	return &StopServiceAction{
		BaseAction: NewBaseAction(
			fmt.Sprintf("service.stop.%s", serviceName),
			fmt.Sprintf("Stop service %s", serviceName),
			fmt.Sprintf("Stops the systemd service '%s'", serviceName),
			CategoryService,
			RiskCritical, // Stopping a service is critical risk
			true,     // Reversible by starting it
			fmt.Sprintf("Service '%s' will be stopped. This will cause downtime.", serviceName),
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

// Validate checks if the service exists and systemd is available.
func (a *StopServiceAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	if err := validateServiceName(a.serviceName); err != nil {
		return err
	}

	_, _, exitCode, _ := executor.ExecuteWithContext(ctx, "which systemctl")
	if exitCode != 0 {
		return fmt.Errorf("systemctl command not found")
	}

	// Normalize service name - add .service if not already present
	serviceName := a.serviceName
	if !strings.HasSuffix(serviceName, ".service") {
		serviceName += ".service"
	}

	cmd := fmt.Sprintf("systemctl list-unit-files %s", shellQuote(serviceName))
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, cmd)
	if exitCode != 0 || !strings.Contains(stdout, serviceName) {
		return fmt.Errorf("service '%s' not found", a.serviceName)
	}

	return nil
}

// Execute stops the service.
func (a *StopServiceAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	cmd := fmt.Sprintf("systemctl stop %s", shellQuote(a.serviceName))
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	result.Output = fmt.Sprintf("stdout: %s\nstderr: %s", stdout, stderr)

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to stop service '%s'", a.serviceName)
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("systemctl stop failed: %s", stderr)
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully stopped service '%s'", a.serviceName)
	result.ChangesApplied = []string{fmt.Sprintf("Stopped service '%s'", a.serviceName)}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback starts the service.
func (a *StopServiceAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	cmd := fmt.Sprintf("systemctl start %s", shellQuote(a.serviceName))
	_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("rollback failed: %s", stderr)
	}
	return nil
}

// StartServiceAction starts a systemd service.
type StartServiceAction struct {
	*BaseAction
	serviceName string
}

// NewStartServiceAction creates a new StartServiceAction.
func NewStartServiceAction(serviceName string, logger *logrus.Logger) *StartServiceAction {
	return &StartServiceAction{
		BaseAction: NewBaseAction(
			fmt.Sprintf("service.start.%s", serviceName),
			fmt.Sprintf("Start service %s", serviceName),
			fmt.Sprintf("Starts the systemd service '%s'", serviceName),
			CategoryService,
			RiskModerate,
			true, // Reversible by stopping it
			fmt.Sprintf("Service '%s' will be started.", serviceName),
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

// Validate checks if the service exists and systemd is available.
func (a *StartServiceAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	if err := validateServiceName(a.serviceName); err != nil {
		return err
	}

	_, _, exitCode, _ := executor.ExecuteWithContext(ctx, "which systemctl")
	if exitCode != 0 {
		return fmt.Errorf("systemctl command not found")
	}

	// Normalize service name - add .service if not already present
	serviceName := a.serviceName
	if !strings.HasSuffix(serviceName, ".service") {
		serviceName += ".service"
	}

	cmd := fmt.Sprintf("systemctl list-unit-files %s", shellQuote(serviceName))
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, cmd)
	if exitCode != 0 || !strings.Contains(stdout, serviceName) {
		return fmt.Errorf("service '%s' not found", a.serviceName)
	}

	return nil
}

// Execute starts the service.
func (a *StartServiceAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	cmd := fmt.Sprintf("systemctl start %s", shellQuote(a.serviceName))
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	result.Output = fmt.Sprintf("stdout: %s\nstderr: %s", stdout, stderr)

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to start service '%s'", a.serviceName)
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("systemctl start failed: %s", stderr)
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully started service '%s'", a.serviceName)
	result.ChangesApplied = []string{fmt.Sprintf("Started service '%s'", a.serviceName)}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback stops the service.
func (a *StartServiceAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	cmd := fmt.Sprintf("systemctl stop %s", shellQuote(a.serviceName))
	_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("rollback failed: %s", stderr)
	}
	return nil
}

func init() {
	logger := logrus.StandardLogger()
	registry := GetDefaultRegistry(logger)
	_ = registry.Register("service.restart", NewRestartServiceActionFactory())
	_ = registry.Register("service.stop", NewStopServiceActionFactory())
	_ = registry.Register("service.start", NewStartServiceActionFactory())
}