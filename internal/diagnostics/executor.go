package diagnostics

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/ignacio/lumo/internal/ssh"
)

// SSHExecutor adapts an SSH client to the CommandExecutor interface
type SSHExecutor struct {
	client *ssh.Client
}

// NewSSHExecutor creates a new SSH command executor
func NewSSHExecutor(client *ssh.Client) *SSHExecutor {
	return &SSHExecutor{
		client: client,
	}
}

// Execute runs a command and returns the result
func (e *SSHExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	result, err := e.client.Execute(command, &ssh.CommandOptions{
		Timeout: timeout,
	})

	if err != nil {
		return "", "", 1, err
	}

	return result.Stdout, result.Stderr, result.ExitCode, nil
}

// ExecuteWithContext runs a command with context support
func (e *SSHExecutor) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	result, err := e.client.Execute(command, &ssh.CommandOptions{
		Context: ctx,
		Timeout: 30 * time.Second,
	})

	if err != nil {
		return "", "", 1, err
	}

	return result.Stdout, result.Stderr, result.ExitCode, nil
}

// LocalExecutor executes commands locally without SSH
type LocalExecutor struct{}

// NewLocalExecutor creates a new local command executor
func NewLocalExecutor() *LocalExecutor {
	return &LocalExecutor{}
}

// Execute runs a command locally and returns the result
func (e *LocalExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return e.ExecuteWithContext(ctx, command)
}

// ExecuteWithContext runs a command locally with context support
func (e *LocalExecutor) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	// Use sh -c to execute the command (similar to SSH behavior)
	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()

	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	// Determine exit code
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			// Don't return error for non-zero exit codes
			err = nil
		} else {
			// This is a real error (command not found, etc.)
			exitCode = 1
			err = fmt.Errorf("failed to execute command: %w", err)
		}
	} else {
		exitCode = 0
	}

	return stdout, stderr, exitCode, err
}
