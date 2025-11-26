package remediation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// KillProcessAction kills a process by PID.
type KillProcessAction struct {
	*BaseAction
	pid         int
	processName string
	signal      string
}

// NewKillProcessAction creates a new kill process action.
func NewKillProcessAction(pid int, processName string, signal string, logger *logrus.Logger) *KillProcessAction {
	if signal == "" {
		signal = "SIGKILL"
	}

	return &KillProcessAction{
		BaseAction: NewBaseAction(
			fmt.Sprintf("process.kill.%d", pid),
			fmt.Sprintf("Kill process %d (%s)", pid, processName),
			fmt.Sprintf("Sends %s signal to process %d (%s) to terminate it", signal, pid, processName),
			CategoryProcess,
			RiskCritical,
			false, // not reversible - process is killed
			fmt.Sprintf("Process %d (%s) will be forcefully terminated. This may cause data loss.", pid, processName),
			logger,
		),
		pid:         pid,
		processName: processName,
		signal:      signal,
	}
}

// NewKillProcessActionFactory returns a factory for creating KillProcessAction instances.
func NewKillProcessActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		pid, ok := params["pid"].(int)
		if !ok {
			return nil, fmt.Errorf("pid parameter is required and must be an integer")
		}
		processName, _ := params["process_name"].(string)
		if processName == "" {
			processName = "unknown"
		}
		signal, _ := params["signal"].(string)
		if signal == "" {
			signal = "SIGKILL"
		}
		return NewKillProcessAction(pid, processName, signal, logger), nil
	}
}

// Validate checks if the process exists and can be killed.
func (a *KillProcessAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	// Check if process exists
	cmd := fmt.Sprintf("ps -p %d -o pid= 2>/dev/null", a.pid)
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, cmd)
	if exitCode != 0 || strings.TrimSpace(stdout) == "" {
		return fmt.Errorf("process %d does not exist", a.pid)
	}

	// Check if we have permission to kill the process
	cmd = fmt.Sprintf("kill -0 %d 2>/dev/null", a.pid)
	_, _, exitCode, _ = executor.ExecuteWithContext(ctx, cmd)
	if exitCode != 0 {
		return fmt.Errorf("insufficient permissions to kill process %d", a.pid)
	}

	return nil
}

// Execute kills the process.
func (a *KillProcessAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	// Get process info before killing
	processInfo, _ := a.getProcessInfo(ctx, executor)

	// Kill the process
	cmd := fmt.Sprintf("kill -%s %d", a.signal, a.pid)
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	result.Output = fmt.Sprintf("stdout: %s\nstderr: %s", stdout, stderr)

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to kill process %d", a.pid)
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("kill command failed: %s", stderr)
	}

	// Wait a moment and verify process is dead
	time.Sleep(1 * time.Second)
	stillExists := a.processExists(ctx, executor)

	if stillExists {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Process %d was sent %s but is still running", a.pid, a.signal)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("process did not terminate")
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully killed process %d (%s)", a.pid, a.processName)
	result.ChangesApplied = []string{
		fmt.Sprintf("Process %d (%s) terminated with %s", a.pid, a.processName, a.signal),
		fmt.Sprintf("Process info: %s", processInfo),
	}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback is not supported for this action.
func (a *KillProcessAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	return fmt.Errorf("process termination cannot be rolled back")
}

// processExists checks if the process still exists.
func (a *KillProcessAction) processExists(ctx context.Context, executor diagnostics.CommandExecutor) bool {
	cmd := fmt.Sprintf("ps -p %d -o pid= 2>/dev/null", a.pid)
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, cmd)
	return exitCode == 0 && strings.TrimSpace(stdout) != ""
}

// getProcessInfo returns information about the process.
func (a *KillProcessAction) getProcessInfo(ctx context.Context, executor diagnostics.CommandExecutor) (string, error) {
	cmd := fmt.Sprintf("ps -p %d -o pid,ppid,user,%%cpu,%%mem,cmd 2>/dev/null", a.pid)
	stdout, _, _, _ := executor.ExecuteWithContext(ctx, cmd)
	lines := strings.Split(stdout, "\n")
	if len(lines) > 1 {
		return strings.TrimSpace(lines[1]), nil
	}
	return "", fmt.Errorf("process info not available")
}

// KillProcessGracefulAction kills a process gracefully (SIGTERM then SIGKILL).
type KillProcessGracefulAction struct {
	*BaseAction
	pid         int
	processName string
	timeout     time.Duration
}

// NewKillProcessGracefulAction creates a new graceful kill process action.
func NewKillProcessGracefulAction(pid int, processName string, timeout time.Duration, logger *logrus.Logger) *KillProcessGracefulAction {
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return &KillProcessGracefulAction{
		BaseAction: NewBaseAction(
			fmt.Sprintf("process.kill_graceful.%d", pid),
			fmt.Sprintf("Gracefully kill process %d (%s)", pid, processName),
			fmt.Sprintf("Sends SIGTERM to process %d (%s), waits %s, then SIGKILL if needed", pid, processName, timeout),
			CategoryProcess,
			RiskModerate,
			false,
			fmt.Sprintf("Process %d (%s) will be gracefully terminated (SIGTERM then SIGKILL)", pid, processName),
			logger,
		),
		pid:         pid,
		processName: processName,
		timeout:     timeout,
	}
}

// NewKillProcessGracefulActionFactory returns a factory for creating KillProcessGracefulAction instances.
func NewKillProcessGracefulActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		pid, ok := params["pid"].(int)
		if !ok {
			return nil, fmt.Errorf("pid parameter is required and must be an integer")
		}
		processName, _ := params["process_name"].(string)
		if processName == "" {
			processName = "unknown"
		}
		timeout := 10 * time.Second
		if t, ok := params["timeout"].(time.Duration); ok {
			timeout = t
		} else if seconds, ok := params["timeout_seconds"].(int); ok {
			timeout = time.Duration(seconds) * time.Second
		}
		return NewKillProcessGracefulAction(pid, processName, timeout, logger), nil
	}
}

// Validate checks if the process exists and can be killed.
func (a *KillProcessGracefulAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	// Check if process exists
	cmd := fmt.Sprintf("ps -p %d -o pid= 2>/dev/null", a.pid)
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, cmd)
	if exitCode != 0 || strings.TrimSpace(stdout) == "" {
		return fmt.Errorf("process %d does not exist", a.pid)
	}

	// Check if we have permission to kill the process
	cmd = fmt.Sprintf("kill -0 %d 2>/dev/null", a.pid)
	_, _, exitCode, _ = executor.ExecuteWithContext(ctx, cmd)
	if exitCode != 0 {
		return fmt.Errorf("insufficient permissions to kill process %d", a.pid)
	}

	return nil
}

// Execute gracefully kills the process.
func (a *KillProcessGracefulAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	// Get process info before killing
	processInfo, _ := a.getProcessInfo(ctx, executor)

	// Send SIGTERM
	cmd := fmt.Sprintf("kill -TERM %d", a.pid)
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	result.Output = fmt.Sprintf("SIGTERM sent:\nstdout: %s\nstderr: %s\n", stdout, stderr)

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to send SIGTERM to process %d", a.pid)
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("SIGTERM failed: %s", stderr)
	}

	// Wait for process to exit gracefully
	gracefullyExited := a.waitForProcessExit(ctx, executor, a.timeout)

	if gracefullyExited {
		result.Status = StatusSuccess
		result.Message = fmt.Sprintf("Process %d (%s) gracefully exited after SIGTERM", a.pid, a.processName)
		result.ChangesApplied = []string{
			fmt.Sprintf("Process %d (%s) terminated gracefully with SIGTERM", a.pid, a.processName),
			fmt.Sprintf("Process info: %s", processInfo),
		}
	} else {
		// Process didn't exit gracefully, send SIGKILL
		cmd = fmt.Sprintf("kill -KILL %d", a.pid)
		stdout, stderr, exitCode, err = executor.ExecuteWithContext(ctx, cmd)
		result.Output += fmt.Sprintf("\nSIGKILL sent (timeout after %s):\nstdout: %s\nstderr: %s", a.timeout, stdout, stderr)

		if err != nil || exitCode != 0 {
			result.Status = StatusFailed
			result.Message = fmt.Sprintf("Failed to send SIGKILL to process %d", a.pid)
			result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result, fmt.Errorf("SIGKILL failed: %s", stderr)
		}

		result.Status = StatusSuccess
		result.Message = fmt.Sprintf("Process %d (%s) forcefully killed after timeout", a.pid, a.processName)
		result.ChangesApplied = []string{
			fmt.Sprintf("Process %d (%s) terminated with SIGKILL after SIGTERM timeout", a.pid, a.processName),
			fmt.Sprintf("Process info: %s", processInfo),
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	return result, nil
}

// Rollback is not supported.
func (a *KillProcessGracefulAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	return fmt.Errorf("process termination cannot be rolled back")
}

// waitForProcessExit waits for a process to exit, returns true if it exited within timeout.
func (a *KillProcessGracefulAction) waitForProcessExit(ctx context.Context, executor diagnostics.CommandExecutor, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	checkInterval := 500 * time.Millisecond

	for time.Now().Before(deadline) {
		cmd := fmt.Sprintf("ps -p %d -o pid= 2>/dev/null", a.pid)
		stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, cmd)

		if exitCode != 0 || strings.TrimSpace(stdout) == "" {
			// Process has exited
			return true
		}

		time.Sleep(checkInterval)
	}

	return false
}

// getProcessInfo returns information about the process.
func (a *KillProcessGracefulAction) getProcessInfo(ctx context.Context, executor diagnostics.CommandExecutor) (string, error) {
	cmd := fmt.Sprintf("ps -p %d -o pid,ppid,user,%%cpu,%%mem,cmd 2>/dev/null", a.pid)
	stdout, _, _, _ := executor.ExecuteWithContext(ctx, cmd)
	lines := strings.Split(stdout, "\n")
	if len(lines) > 1 {
		return strings.TrimSpace(lines[1]), nil
	}
	return "", fmt.Errorf("process info not available")
}

// KillProcessByNameAction kills all processes matching a name pattern.
type KillProcessByNameAction struct {
	*BaseAction
	processPattern string
	signal         string
}

// NewKillProcessByNameAction creates a new kill by name action.
func NewKillProcessByNameAction(processPattern string, signal string, logger *logrus.Logger) *KillProcessByNameAction {
	if signal == "" {
		signal = "SIGTERM"
	}

	return &KillProcessByNameAction{
		BaseAction: NewBaseAction(
			fmt.Sprintf("process.kill_by_name.%s", processPattern),
			fmt.Sprintf("Kill processes matching '%s'", processPattern),
			fmt.Sprintf("Kills all processes with names matching '%s' using %s signal", processPattern, signal),
			CategoryProcess,
			RiskCritical,
			false,
			fmt.Sprintf("All processes matching '%s' will be terminated. This may cause data loss.", processPattern),
			logger,
		),
		processPattern: processPattern,
		signal:         signal,
	}
}

// Validate checks if any matching processes exist.
func (a *KillProcessByNameAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	// Validate process pattern to prevent command injection
	if err := validateProcessPattern(a.processPattern); err != nil {
		return err
	}

	// Find matching processes - use shellQuote for defense in depth
	cmd := fmt.Sprintf("pgrep -f %s", shellQuote(a.processPattern))
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, cmd)

	if exitCode != 0 || strings.TrimSpace(stdout) == "" {
		return fmt.Errorf("no processes found matching pattern '%s'", a.processPattern)
	}

	return nil
}

// Execute kills matching processes.
func (a *KillProcessByNameAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	// Get list of matching PIDs - use shellQuote to prevent command injection
	cmd := fmt.Sprintf("pgrep -f %s", shellQuote(a.processPattern))
	stdout, _, _, _ := executor.ExecuteWithContext(ctx, cmd)
	pids := strings.Fields(strings.TrimSpace(stdout))

	if len(pids) == 0 {
		result.Status = StatusSuccess
		result.Message = "No matching processes found"
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, nil
	}

	// Kill each process
	killedCount := 0
	var errors []string

	for _, pidStr := range pids {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			errors = append(errors, fmt.Sprintf("invalid PID: %s", pidStr))
			continue
		}

		cmd = fmt.Sprintf("kill -%s %d 2>&1", a.signal, pid)
		_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
		if err != nil || exitCode != 0 {
			errors = append(errors, fmt.Sprintf("PID %d: %s", pid, stderr))
		} else {
			killedCount++
		}
	}

	if killedCount == 0 {
		result.Status = StatusFailed
		result.Message = "Failed to kill any matching processes"
		result.Error = strings.Join(errors, "; ")
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("no processes were killed")
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Killed %d out of %d matching processes", killedCount, len(pids))
	result.ChangesApplied = []string{
		fmt.Sprintf("Terminated %d processes matching '%s' with %s", killedCount, a.processPattern, a.signal),
	}

	if len(errors) > 0 {
		result.Output = fmt.Sprintf("Errors: %s", strings.Join(errors, "; "))
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	return result, nil
}

// Rollback is not supported.
func (a *KillProcessByNameAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	return fmt.Errorf("process termination cannot be rolled back")
}
