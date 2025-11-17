package remediation

import (
	"context"
	"fmt"
	"time"
)

// Executor interface defines how remediation actions are executed
type Executor interface {
	// Execute runs a remediation action
	Execute(ctx context.Context, action *Action) (*Result, error)

	// Verify checks if an action was successful
	Verify(ctx context.Context, action *Action, result *Result) (*Result, error)

	// Rollback reverts an action if possible
	Rollback(ctx context.Context, action *Action, result *Result) error
}

// LocalExecutor executes remediation actions locally (no SSH)
type LocalExecutor struct {
	commandExecutor CommandExecutor
	dryRun          bool
}

// CommandExecutor interface abstracts command execution
type CommandExecutor interface {
	// Execute runs a shell command and returns stdout, stderr, and exit code
	ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error)
}

// NewLocalExecutor creates a new local remediation executor
func NewLocalExecutor(cmdExecutor CommandExecutor, dryRun bool) *LocalExecutor {
	return &LocalExecutor{
		commandExecutor: cmdExecutor,
		dryRun:          dryRun,
	}
}

// Execute runs a remediation action
func (e *LocalExecutor) Execute(ctx context.Context, action *Action) (*Result, error) {
	result := &Result{
		ActionID:   action.ID,
		Status:     StatusExecuting,
		ExecutedAt: time.Now(),
	}

	if e.dryRun {
		result.Status = StatusApproved
		result.Output = fmt.Sprintf("[DRY-RUN] Would execute: %v\n", action.Commands)
		return result, nil
	}

	// Execute each command in sequence
	var output string
	for i, command := range action.Commands {
		startTime := time.Now()

		stdout, stderr, exitCode, err := e.commandExecutor.ExecuteWithContext(ctx, command)
		duration := time.Since(startTime)
		result.Duration += duration

		output += fmt.Sprintf("Command %d: %s\n", i+1, command)
		output += fmt.Sprintf("Exit Code: %d\n", exitCode)
		if stdout != "" {
			output += fmt.Sprintf("Output:\n%s\n", stdout)
		}
		if stderr != "" {
			output += fmt.Sprintf("Error:\n%s\n", stderr)
		}

		if exitCode != 0 || err != nil {
			result.Status = StatusFailed
			result.Error = fmt.Sprintf("Command failed: %v", err)
			result.Output = output
			return result, err
		}
	}

	result.Status = StatusSuccess
	result.Output = output
	return result, nil
}

// Verify checks if an action was successful
func (e *LocalExecutor) Verify(ctx context.Context, action *Action, result *Result) (*Result, error) {
	if len(action.VerifyCommands) == 0 {
		return result, nil
	}

	var verifyOutput string
	for i, command := range action.VerifyCommands {
		stdout, stderr, exitCode, err := e.commandExecutor.ExecuteWithContext(ctx, command)

		verifyOutput += fmt.Sprintf("Verify Command %d: %s\n", i+1, command)
		verifyOutput += fmt.Sprintf("Exit Code: %d\n", exitCode)
		if stdout != "" {
			verifyOutput += fmt.Sprintf("Output:\n%s\n", stdout)
		}
		if stderr != "" {
			verifyOutput += fmt.Sprintf("Error:\n%s\n", stderr)
		}

		if exitCode != 0 || err != nil {
			result.Status = StatusFailed
			result.VerificationOutput = verifyOutput
			return result, fmt.Errorf("verification failed: %w", err)
		}
	}

	result.VerificationOutput = verifyOutput
	return result, nil
}

// Rollback attempts to revert an action
func (e *LocalExecutor) Rollback(ctx context.Context, action *Action, result *Result) error {
	if len(action.RollbackCommands) == 0 {
		return nil
	}

	now := time.Now()
	result.RolledBackAt = &now

	if e.dryRun {
		result.RollbackError = fmt.Sprintf("[DRY-RUN] Would rollback: %v", action.RollbackCommands)
		return nil
	}

	var rollbackError string
	for _, command := range action.RollbackCommands {
		_, stderr, exitCode, err := e.commandExecutor.ExecuteWithContext(ctx, command)

		if exitCode != 0 || err != nil {
			rollbackError += fmt.Sprintf("Rollback command failed: %s\nError: %v\n", command, err)
			if stderr != "" {
				rollbackError += fmt.Sprintf("stderr: %s\n", stderr)
			}
		}
	}

	if rollbackError != "" {
		result.RollbackError = rollbackError
		result.Status = StatusRolledBack
		return fmt.Errorf("rollback had errors: %s", rollbackError)
	}

	result.Status = StatusRolledBack
	return nil
}

// SSHExecutor executes remediation actions over SSH
type SSHExecutor struct {
	sshClient SSHClient
	dryRun    bool
}

// SSHClient interface for SSH operations (should be satisfied by diagnostics executors)
type SSHClient interface {
	ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error)
}

// NewSSHExecutor creates a new SSH remediation executor
func NewSSHExecutor(sshClient SSHClient, dryRun bool) *SSHExecutor {
	return &SSHExecutor{
		sshClient: sshClient,
		dryRun:    dryRun,
	}
}

// Execute runs a remediation action over SSH
func (e *SSHExecutor) Execute(ctx context.Context, action *Action) (*Result, error) {
	result := &Result{
		ActionID:   action.ID,
		Status:     StatusExecuting,
		ExecutedAt: time.Now(),
	}

	if e.dryRun {
		result.Status = StatusApproved
		result.Output = fmt.Sprintf("[DRY-RUN] Would execute over SSH: %v\n", action.Commands)
		return result, nil
	}

	var output string
	for i, command := range action.Commands {
		startTime := time.Now()

		stdout, stderr, exitCode, err := e.sshClient.ExecuteWithContext(ctx, command)
		duration := time.Since(startTime)
		result.Duration += duration

		output += fmt.Sprintf("Command %d: %s\n", i+1, command)
		output += fmt.Sprintf("Exit Code: %d\n", exitCode)
		if stdout != "" {
			output += fmt.Sprintf("Output:\n%s\n", stdout)
		}
		if stderr != "" {
			output += fmt.Sprintf("Error:\n%s\n", stderr)
		}

		if exitCode != 0 || err != nil {
			result.Status = StatusFailed
			result.Error = fmt.Sprintf("Command failed: %v", err)
			result.Output = output
			return result, err
		}
	}

	result.Status = StatusSuccess
	result.Output = output
	return result, nil
}

// Verify checks if an SSH-executed action was successful
func (e *SSHExecutor) Verify(ctx context.Context, action *Action, result *Result) (*Result, error) {
	if len(action.VerifyCommands) == 0 {
		return result, nil
	}

	var verifyOutput string
	for i, command := range action.VerifyCommands {
		stdout, stderr, exitCode, err := e.sshClient.ExecuteWithContext(ctx, command)

		verifyOutput += fmt.Sprintf("Verify Command %d: %s\n", i+1, command)
		verifyOutput += fmt.Sprintf("Exit Code: %d\n", exitCode)
		if stdout != "" {
			verifyOutput += fmt.Sprintf("Output:\n%s\n", stdout)
		}
		if stderr != "" {
			verifyOutput += fmt.Sprintf("Error:\n%s\n", stderr)
		}

		if exitCode != 0 || err != nil {
			result.Status = StatusFailed
			result.VerificationOutput = verifyOutput
			return result, fmt.Errorf("verification failed: %w", err)
		}
	}

	result.VerificationOutput = verifyOutput
	return result, nil
}

// Rollback attempts to revert an SSH-executed action
func (e *SSHExecutor) Rollback(ctx context.Context, action *Action, result *Result) error {
	if len(action.RollbackCommands) == 0 {
		return nil
	}

	now := time.Now()
	result.RolledBackAt = &now

	if e.dryRun {
		result.RollbackError = fmt.Sprintf("[DRY-RUN] Would rollback over SSH: %v", action.RollbackCommands)
		return nil
	}

	var rollbackError string
	for _, command := range action.RollbackCommands {
		_, stderr, exitCode, err := e.sshClient.ExecuteWithContext(ctx, command)

		if exitCode != 0 || err != nil {
			rollbackError += fmt.Sprintf("Rollback command failed: %s\nError: %v\n", command, err)
			if stderr != "" {
				rollbackError += fmt.Sprintf("stderr: %s\n", stderr)
			}
		}
	}

	if rollbackError != "" {
		result.RollbackError = rollbackError
		result.Status = StatusRolledBack
		return fmt.Errorf("rollback had errors: %s", rollbackError)
	}

	result.Status = StatusRolledBack
	return nil
}
