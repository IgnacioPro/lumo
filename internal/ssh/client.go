package ssh

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// Client wraps an SSH client connection with additional management features
type Client struct {
	// SSH client connection
	sshClient *ssh.Client

	// Configuration
	config *ClientConfig

	// Connection information
	connInfo *ConnectionInfo

	// Health checker
	healthChecker *HealthChecker

	// Logger
	logger *logrus.Logger

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Connection state
	status ConnectionStatus

	// Retry configuration
	retryConfig *RetryConfig
}

// NewClient creates a new SSH client with the given configuration
func NewClient(config *ClientConfig, logger *logrus.Logger) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if logger == nil {
		logger = logrus.New()
	}

	return &Client{
		config:      config,
		logger:      logger,
		status:      StatusDisconnected,
		retryConfig: DefaultRetryConfig(),
		connInfo: &ConnectionInfo{
			Status: StatusDisconnected,
		},
	}, nil
}

// Connect establishes an SSH connection to the specified host
func (c *Client) Connect(host string, port int, user string) error {
	c.mu.Lock()
	if c.status == StatusConnected {
		c.mu.Unlock()
		return fmt.Errorf("already connected")
	}
	c.status = StatusConnecting
	c.mu.Unlock()

	c.logger.Infof("Connecting to %s@%s:%d...", user, host, port)

	// Update connection info
	c.connInfo = &ConnectionInfo{
		Host:   host,
		Port:   port,
		User:   user,
		Status: StatusConnecting,
	}

	// Build authentication methods
	authMethods, authMethodTypes, err := buildAuthMethods(c.config, host, user)
	if err != nil {
		c.setStatus(StatusFailed)
		return NewAuthenticationError(host, user, AuthMethodAgent, "failed to build auth methods", err)
	}

	c.logger.Debugf("Attempting authentication with %d method(s)", len(authMethods))

	// Get host key callback
	hostKeyCallback, err := getHostKeyCallback(c.config)
	if err != nil {
		c.setStatus(StatusFailed)
		return NewConnectionError(host, port, user, "failed to get host key callback", err)
	}

	// Create SSH client config
	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         c.config.GetTimeout(),
	}

	// Attempt connection with retry
	var connectErr error
	var lastAuthMethod AuthMethod

	connectFunc := func() error {
		addr := fmt.Sprintf("%s:%d", host, port)
		client, err := ssh.Dial("tcp", addr, sshConfig)
		if err != nil {
			return err
		}

		c.mu.Lock()
		c.sshClient = client
		c.mu.Unlock()

		return nil
	}

	// Configure retry
	c.retryConfig.MaxAttempts = c.config.GetMaxRetries()
	c.retryConfig.InitialInterval = c.config.GetRetryInterval()
	c.retryConfig.Logger = c.logger

	// Attempt connection with retry
	connectErr = RetryConnection(host, port, user, connectFunc, c.retryConfig)

	if connectErr != nil {
		c.setStatus(StatusFailed)

		// Try to determine which auth method failed
		for i, authMethod := range authMethodTypes {
			if i < len(authMethods) {
				lastAuthMethod = authMethod
			}
		}

		return NewAuthenticationError(host, user, lastAuthMethod, "connection failed", connectErr)
	}

	// Connection successful
	c.connInfo.Status = StatusConnected
	c.connInfo.ConnectedAt = time.Now()
	c.connInfo.AuthMethodUsed = lastAuthMethod
	c.setStatus(StatusConnected)

	c.logger.Infof("Successfully connected to %s@%s:%d", user, host, port)

	// Start health checking
	if err := c.startHealthChecker(); err != nil {
		c.logger.Warnf("Failed to start health checker: %v", err)
	}

	return nil
}

// Disconnect closes the SSH connection
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == StatusDisconnected {
		return nil
	}

	c.logger.Infof("Disconnecting from %s@%s:%d", c.connInfo.User, c.connInfo.Host, c.connInfo.Port)

	// Stop health checker
	if c.healthChecker != nil {
		c.healthChecker.Stop()
		c.healthChecker = nil
	}

	// Close SSH client
	if c.sshClient != nil {
		if err := c.sshClient.Close(); err != nil {
			c.logger.Warnf("Error closing SSH client: %v", err)
		}
		c.sshClient = nil
	}

	c.status = StatusDisconnected
	c.connInfo.Status = StatusDisconnected

	c.logger.Info("Disconnected successfully")

	return nil
}

// Reconnect attempts to reconnect to the same host
func (c *Client) Reconnect() error {
	c.mu.RLock()
	host := c.connInfo.Host
	port := c.connInfo.Port
	user := c.connInfo.User
	c.mu.RUnlock()

	if host == "" || user == "" {
		return fmt.Errorf("no previous connection information available")
	}

	c.logger.Infof("Reconnecting to %s@%s:%d...", user, host, port)

	// Disconnect first
	if err := c.Disconnect(); err != nil {
		c.logger.Warnf("Error during disconnect before reconnect: %v", err)
	}

	c.setStatus(StatusReconnecting)
	c.connInfo.ReconnectAttempts++

	// Attempt to connect again
	return c.Connect(host, port, user)
}

// IsConnected returns true if the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == StatusConnected && c.sshClient != nil
}

// GetStatus returns the current connection status
func (c *Client) GetStatus() ConnectionStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// GetConnectionInfo returns the connection metadata
func (c *Client) GetConnectionInfo() *ConnectionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create a copy to avoid race conditions
	infoCopy := *c.connInfo
	if c.healthChecker != nil {
		infoCopy.LastHealthCheck = c.healthChecker.GetLastCheckTime()
	}

	return &infoCopy
}

// Execute runs a command on the remote server
func (c *Client) Execute(command string, options *CommandOptions) (*CommandResult, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	c.mu.RLock()
	client := c.sshClient
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("SSH client is nil")
	}

	// Create a new session for execution
	session := &Session{
		client: client,
		logger: c.logger,
	}

	return session.Execute(command, options)
}

// GetSSHClient returns the underlying SSH client (use with caution)
func (c *Client) GetSSHClient() *ssh.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sshClient
}

// startHealthChecker starts the health monitoring
func (c *Client) startHealthChecker() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sshClient == nil {
		return fmt.Errorf("SSH client is nil")
	}

	interval := c.config.GetKeepAlive()
	if interval == 0 {
		interval = DefaultKeepAliveInterval
	}

	c.healthChecker = NewHealthChecker(c.sshClient, interval, c.logger)

	// Set callback for unhealthy connection
	c.healthChecker.SetUnhealthyCallback(func() {
		c.logger.Warn("Connection became unhealthy, attempting to reconnect...")
		if err := c.Reconnect(); err != nil {
			c.logger.Errorf("Failed to reconnect: %v", err)
		}
	})

	return c.healthChecker.Start()
}

// setStatus updates the connection status (must be called with lock held or use mutex)
func (c *Client) setStatus(status ConnectionStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
	if c.connInfo != nil {
		c.connInfo.Status = status
	}
}

// SetLogger sets a custom logger
func (c *Client) SetLogger(logger *logrus.Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if logger != nil {
		c.logger = logger
	}
}

// SetRetryConfig sets a custom retry configuration
func (c *Client) SetRetryConfig(config *RetryConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if config != nil {
		c.retryConfig = config
	}
}

// CheckHealth manually checks the connection health
func (c *Client) CheckHealth() error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	c.mu.RLock()
	checker := c.healthChecker
	c.mu.RUnlock()

	if checker == nil {
		// No health checker, try a simple ping
		return PingServer(c.sshClient)
	}

	return checker.CheckHealth()
}

// WaitForHealthy waits for the connection to become healthy with a timeout
func (c *Client) WaitForHealthy(timeout time.Duration) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	c.mu.RLock()
	checker := c.healthChecker
	c.mu.RUnlock()

	if checker == nil {
		return fmt.Errorf("health checker not initialized")
	}

	return WaitForConnection(checker, timeout)
}
