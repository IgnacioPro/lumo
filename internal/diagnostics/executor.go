package diagnostics

import (
	"context"
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
