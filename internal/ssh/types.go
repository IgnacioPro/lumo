package ssh

import (
	"context"
	"io"
	"time"
)

// ConnectionStatus represents the current state of an SSH connection
type ConnectionStatus int

const (
	// StatusDisconnected indicates no active connection
	StatusDisconnected ConnectionStatus = iota
	// StatusConnecting indicates connection is in progress
	StatusConnecting
	// StatusConnected indicates active connection
	StatusConnected
	// StatusReconnecting indicates reconnection attempt in progress
	StatusReconnecting
	// StatusFailed indicates connection failed
	StatusFailed
)

// String returns the string representation of ConnectionStatus
func (s ConnectionStatus) String() string {
	switch s {
	case StatusDisconnected:
		return "disconnected"
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	case StatusReconnecting:
		return "reconnecting"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// AuthMethod represents different SSH authentication methods
type AuthMethod int

const (
	// AuthMethodAgent uses SSH agent for authentication
	AuthMethodAgent AuthMethod = iota
	// AuthMethodKey uses private key file for authentication
	AuthMethodKey
	// AuthMethodPassword uses password for authentication
	AuthMethodPassword
	// AuthMethodInteractive uses keyboard-interactive authentication
	AuthMethodInteractive
)

// String returns the string representation of AuthMethod
func (a AuthMethod) String() string {
	switch a {
	case AuthMethodAgent:
		return "ssh-agent"
	case AuthMethodKey:
		return "private-key"
	case AuthMethodPassword:
		return "password"
	case AuthMethodInteractive:
		return "keyboard-interactive"
	default:
		return "unknown"
	}
}

// ConnectionInfo contains metadata about an SSH connection
type ConnectionInfo struct {
	Host              string
	Port              int
	User              string
	Status            ConnectionStatus
	AuthMethodUsed    AuthMethod
	ConnectedAt       time.Time
	LastHealthCheck   time.Time
	ReconnectAttempts int
}

// CommandOptions contains options for command execution
type CommandOptions struct {
	// Timeout for command execution (0 means no timeout)
	Timeout time.Duration

	// Environment variables to set for the command
	Env map[string]string

	// Working directory for command execution
	WorkingDir string

	// Whether to allocate a PTY for interactive commands
	UsePTY bool

	// Context for cancellation
	Context context.Context
}

// CommandResult contains the result of command execution
type CommandResult struct {
	Command   string
	Stdout    string
	Stderr    string
	ExitCode  int
	Duration  time.Duration
	StartTime time.Time
	EndTime   time.Time
	Error     error
}

// Success returns true if the command executed successfully (exit code 0)
func (r *CommandResult) Success() bool {
	return r.ExitCode == 0 && r.Error == nil
}

// Output returns combined stdout and stderr
func (r *CommandResult) Output() string {
	if r.Stderr != "" {
		return r.Stdout + "\n" + r.Stderr
	}
	return r.Stdout
}

// SSHClient defines the interface for SSH client operations
type SSHClient interface {
	// Connect establishes an SSH connection
	Connect(host string, port int, user string) error

	// Disconnect closes the SSH connection
	Disconnect() error

	// IsConnected returns the current connection status
	IsConnected() bool

	// Execute runs a command and returns the result
	Execute(command string, options *CommandOptions) (*CommandResult, error)

	// GetConnectionInfo returns connection metadata
	GetConnectionInfo() *ConnectionInfo
}

// SessionExecutor defines the interface for command execution
type SessionExecutor interface {
	// Execute runs a command with the given options
	Execute(command string, options *CommandOptions) (*CommandResult, error)

	// ExecuteStream runs a command and streams output to the provided writers
	ExecuteStream(command string, stdout, stderr io.Writer, options *CommandOptions) error

	// Close closes the session
	Close() error
}

// Constants for default values
const (
	// DefaultCommandTimeout is the default timeout for command execution
	DefaultCommandTimeout = 5 * time.Minute

	// DefaultConnectionTimeout is the default timeout for establishing connections
	DefaultConnectionTimeout = 30 * time.Second

	// DefaultKeepAliveInterval is the default interval for keep-alive checks
	DefaultKeepAliveInterval = 30 * time.Second

	// DefaultMaxRetries is the default maximum number of retry attempts
	DefaultMaxRetries = 3

	// DefaultRetryInterval is the default interval between retries
	DefaultRetryInterval = 5 * time.Second

	// MaxOutputBufferSize is the maximum size of output buffers
	MaxOutputBufferSize = 10 * 1024 * 1024 // 10MB
)
