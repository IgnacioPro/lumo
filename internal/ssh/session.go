package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// Session wraps an SSH session with command execution capabilities
type Session struct {
	client *ssh.Client
	logger *logrus.Logger
	mu     sync.Mutex
}

// NewSession creates a new session from an SSH client
func NewSession(client *ssh.Client, logger *logrus.Logger) *Session {
	if logger == nil {
		logger = logrus.New()
	}

	return &Session{
		client: client,
		logger: logger,
	}
}

// Execute runs a command and returns the result
func (s *Session) Execute(command string, options *CommandOptions) (*CommandResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("SSH client is nil")
	}

	// Set defaults for options
	if options == nil {
		options = &CommandOptions{
			Timeout: DefaultCommandTimeout,
			Context: context.Background(),
		}
	}

	if options.Timeout == 0 {
		options.Timeout = DefaultCommandTimeout
	}

	if options.Context == nil {
		options.Context = context.Background()
	}

	s.logger.Debugf("Executing command: %s", command)

	result := &CommandResult{
		Command:   command,
		StartTime: time.Now(),
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(options.Context, options.Timeout)
	defer cancel()

	// Execute with timeout
	done := make(chan error, 1)

	go func() {
		err := s.executeCommand(command, options, result)
		done <- err
	}()

	select {
	case err := <-done:
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)

		if err != nil {
			result.Error = err
			s.logger.Warnf("Command failed: %s (exit code: %d)", command, result.ExitCode)
			return result, err
		}

		s.logger.Debugf("Command succeeded: %s (duration: %v)", command, result.Duration)
		return result, nil

	case <-ctx.Done():
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Error = NewTimeoutError("command execution", options.Timeout.String(), command)

		s.logger.Warnf("Command timed out: %s (after %v)", command, result.Duration)
		return result, result.Error
	}
}

// executeCommand performs the actual command execution
func (s *Session) executeCommand(command string, options *CommandOptions, result *CommandResult) error {
	// Create a new session
	session, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Set up output buffers
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Set up environment variables if specified
	if options.Env != nil && len(options.Env) > 0 {
		for key, value := range options.Env {
			if err := session.Setenv(key, value); err != nil {
				s.logger.Warnf("Failed to set environment variable %s: %v", key, err)
			}
		}
	}

	// Allocate PTY if requested
	if options.UsePTY {
		modes := ssh.TerminalModes{
			ssh.ECHO:          0,     // disable echoing
			ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
			ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
		}

		if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
			return fmt.Errorf("failed to request PTY: %w", err)
		}
	}

	// Change working directory if specified
	if options.WorkingDir != "" {
		command = fmt.Sprintf("cd %s && %s", options.WorkingDir, command)
	}

	// Execute the command
	err = session.Run(command)

	// Capture output
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	// Extract exit code
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			result.ExitCode = exitErr.ExitStatus()
		} else {
			// Non-zero exit but couldn't determine code
			result.ExitCode = 1
		}

		return NewCommandError(command, "command execution failed", result.ExitCode, result.Stdout, result.Stderr, err)
	}

	result.ExitCode = 0
	return nil
}

// ExecuteStream runs a command and streams output to the provided writers
func (s *Session) ExecuteStream(command string, stdout, stderr io.Writer, options *CommandOptions) error {
	if s.client == nil {
		return fmt.Errorf("SSH client is nil")
	}

	// Set defaults for options
	if options == nil {
		options = &CommandOptions{
			Timeout: DefaultCommandTimeout,
			Context: context.Background(),
		}
	}

	if options.Timeout == 0 {
		options.Timeout = DefaultCommandTimeout
	}

	if options.Context == nil {
		options.Context = context.Background()
	}

	s.logger.Debugf("Executing command with streaming: %s", command)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(options.Context, options.Timeout)
	defer cancel()

	// Execute with timeout
	done := make(chan error, 1)

	go func() {
		err := s.executeStreamCommand(command, stdout, stderr, options)
		done <- err
	}()

	select {
	case err := <-done:
		return err

	case <-ctx.Done():
		return NewTimeoutError("command execution", options.Timeout.String(), command)
	}
}

// executeStreamCommand performs the actual streaming command execution
func (s *Session) executeStreamCommand(command string, stdout, stderr io.Writer, options *CommandOptions) error {
	// Create a new session
	session, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Set up output streams
	session.Stdout = stdout
	session.Stderr = stderr

	// Set up environment variables if specified
	if options.Env != nil && len(options.Env) > 0 {
		for key, value := range options.Env {
			if err := session.Setenv(key, value); err != nil {
				s.logger.Warnf("Failed to set environment variable %s: %v", key, err)
			}
		}
	}

	// Allocate PTY if requested
	if options.UsePTY {
		modes := ssh.TerminalModes{
			ssh.ECHO:          0,     // disable echoing
			ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
			ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
		}

		if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
			return fmt.Errorf("failed to request PTY: %w", err)
		}
	}

	// Change working directory if specified
	if options.WorkingDir != "" {
		command = fmt.Sprintf("cd %s && %s", options.WorkingDir, command)
	}

	// Execute the command
	if err := session.Run(command); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			return NewCommandError(command, "command execution failed", exitErr.ExitStatus(), "", "", err)
		}
		return NewCommandError(command, "command execution failed", 1, "", "", err)
	}

	return nil
}

// Close closes the session (for interface compatibility)
func (s *Session) Close() error {
	// Session is created on-demand, nothing to close here
	return nil
}

// ExecuteMultiple executes multiple commands sequentially
func (s *Session) ExecuteMultiple(commands []string, options *CommandOptions) ([]*CommandResult, error) {
	results := make([]*CommandResult, 0, len(commands))

	for _, command := range commands {
		result, err := s.Execute(command, options)
		if result != nil {
			results = append(results, result)
		}

		if err != nil {
			s.logger.Warnf("Command in batch failed: %s", command)
			return results, fmt.Errorf("command '%s' failed: %w", command, err)
		}
	}

	return results, nil
}

// ExecuteScript executes a multi-line script
func (s *Session) ExecuteScript(script string, options *CommandOptions) (*CommandResult, error) {
	// Normalize line endings
	script = strings.ReplaceAll(script, "\r\n", "\n")

	// Execute as a single command with shell
	command := fmt.Sprintf("/bin/bash -c %s", shellQuote(script))
	return s.Execute(command, options)
}

// ExecuteWithInput executes a command with stdin input
func (s *Session) ExecuteWithInput(command string, input io.Reader, options *CommandOptions) (*CommandResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("SSH client is nil")
	}

	// Set defaults for options
	if options == nil {
		options = &CommandOptions{
			Timeout: DefaultCommandTimeout,
			Context: context.Background(),
		}
	}

	s.logger.Debugf("Executing command with input: %s", command)

	result := &CommandResult{
		Command:   command,
		StartTime: time.Now(),
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(options.Context, options.Timeout)
	defer cancel()

	// Execute with timeout
	done := make(chan error, 1)

	go func() {
		err := s.executeCommandWithInput(command, input, options, result)
		done <- err
	}()

	select {
	case err := <-done:
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Error = err
		return result, err

	case <-ctx.Done():
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Error = NewTimeoutError("command execution", options.Timeout.String(), command)
		return result, result.Error
	}
}

// executeCommandWithInput performs command execution with stdin
func (s *Session) executeCommandWithInput(command string, input io.Reader, options *CommandOptions, result *CommandResult) error {
	// Create a new session
	session, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Set up output buffers
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	session.Stdin = input

	// Execute the command
	err = session.Run(command)

	// Capture output
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	// Extract exit code
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			result.ExitCode = exitErr.ExitStatus()
		} else {
			result.ExitCode = 1
		}
		return NewCommandError(command, "command execution failed", result.ExitCode, result.Stdout, result.Stderr, err)
	}

	result.ExitCode = 0
	return nil
}

// Helper function to quote shell arguments
func shellQuote(s string) string {
	// Simple shell quoting - wrap in single quotes and escape existing single quotes
	s = strings.ReplaceAll(s, "'", "'\"'\"'")
	return "'" + s + "'"
}

// GetCommandOutput is a convenience function to just get command output as a string
func GetCommandOutput(client *ssh.Client, command string, timeout time.Duration) (string, error) {
	session := NewSession(client, nil)

	result, err := session.Execute(command, &CommandOptions{
		Timeout: timeout,
	})

	if err != nil {
		return "", err
	}

	return result.Stdout, nil
}

// RunSimpleCommand is a convenience function for simple command execution
func RunSimpleCommand(client *ssh.Client, command string) error {
	session := NewSession(client, nil)

	_, err := session.Execute(command, &CommandOptions{
		Timeout: DefaultCommandTimeout,
	})

	return err
}
