package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ignacio/lumo/internal/config"
)

// ClientConfig extends the base SSH configuration with client-specific settings
type ClientConfig struct {
	// Base SSH configuration from global config
	BaseConfig config.SSHConfig

	// Authentication method preferences (order matters)
	PreferredAuthMethods []AuthMethod

	// Path to known_hosts file (empty to skip host key verification)
	KnownHostsPath string

	// Path to SSH private key file
	KeyPath string

	// Passphrase for encrypted private keys
	Passphrase string

	// Password for password authentication
	Password string

	// Default timeout for command execution
	CommandTimeout time.Duration

	// Buffer size for output capture
	OutputBufferSize int

	// Enable strict host key checking
	StrictHostKeyChecking bool

	// Maximum number of connection attempts in the pool (for future)
	MaxConnectionsPerHost int
}

// NewClientConfig creates a new ClientConfig with defaults
func NewClientConfig(baseConfig config.SSHConfig) *ClientConfig {
	return &ClientConfig{
		BaseConfig: baseConfig,
		PreferredAuthMethods: []AuthMethod{
			AuthMethodAgent,
			AuthMethodKey,
			AuthMethodPassword,
			AuthMethodInteractive,
		},
		KnownHostsPath:        getDefaultKnownHostsPath(),
		CommandTimeout:        DefaultCommandTimeout,
		OutputBufferSize:      MaxOutputBufferSize,
		StrictHostKeyChecking: false, // Default to false for easier development
		MaxConnectionsPerHost: 10,
	}
}

// Validate validates the client configuration
func (c *ClientConfig) Validate() error {
	// Validate base config
	if c.BaseConfig.Port < 1 || c.BaseConfig.Port > 65535 {
		return fmt.Errorf("invalid SSH port: %d", c.BaseConfig.Port)
	}

	if c.BaseConfig.Timeout < 0 {
		return fmt.Errorf("SSH timeout must be positive")
	}

	if c.BaseConfig.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative")
	}

	// Validate auth methods
	if len(c.PreferredAuthMethods) == 0 {
		return fmt.Errorf("at least one authentication method must be specified")
	}

	// Validate key path if specified
	if c.KeyPath != "" {
		if _, err := os.Stat(c.KeyPath); os.IsNotExist(err) {
			return fmt.Errorf("SSH key file not found: %s", c.KeyPath)
		}

		// Check key file permissions (should be 600 or 400)
		info, err := os.Stat(c.KeyPath)
		if err != nil {
			return fmt.Errorf("failed to check key file permissions: %w", err)
		}

		perm := info.Mode().Perm()
		if perm&0077 != 0 {
			return fmt.Errorf("key file %s has insecure permissions %o (should be 600 or 400)", c.KeyPath, perm)
		}
	}

	// Validate known_hosts path if specified and strict checking is enabled
	if c.StrictHostKeyChecking && c.KnownHostsPath != "" {
		if _, err := os.Stat(c.KnownHostsPath); os.IsNotExist(err) {
			return fmt.Errorf("known_hosts file not found: %s", c.KnownHostsPath)
		}
	}

	// Validate timeouts
	if c.CommandTimeout < 0 {
		return fmt.Errorf("command timeout must be non-negative")
	}

	// Validate buffer size
	if c.OutputBufferSize < 1024 {
		return fmt.Errorf("output buffer size must be at least 1KB")
	}

	if c.OutputBufferSize > MaxOutputBufferSize {
		return fmt.Errorf("output buffer size exceeds maximum of %d bytes", MaxOutputBufferSize)
	}

	return nil
}

// SetKeyPath sets the SSH private key path and validates it
func (c *ClientConfig) SetKeyPath(path string) error {
	// Expand home directory if needed
	if path != "" && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("SSH key file not found: %s", path)
	}

	c.KeyPath = path
	return nil
}

// SetPassword sets the password for authentication
func (c *ClientConfig) SetPassword(password string) {
	c.Password = password
}

// SetPassphrase sets the passphrase for encrypted keys
func (c *ClientConfig) SetPassphrase(passphrase string) {
	c.Passphrase = passphrase
}

// EnableStrictHostKeyChecking enables or disables strict host key checking
func (c *ClientConfig) EnableStrictHostKeyChecking(enable bool) {
	c.StrictHostKeyChecking = enable
}

// SetAuthMethodPriority sets the order of authentication methods to try
func (c *ClientConfig) SetAuthMethodPriority(methods []AuthMethod) {
	c.PreferredAuthMethods = methods
}

// GetTimeout returns the connection timeout
func (c *ClientConfig) GetTimeout() time.Duration {
	return c.BaseConfig.Timeout
}

// GetKeepAlive returns the keep-alive interval
func (c *ClientConfig) GetKeepAlive() time.Duration {
	return c.BaseConfig.KeepAlive
}

// GetMaxRetries returns the maximum number of retry attempts
func (c *ClientConfig) GetMaxRetries() int {
	return c.BaseConfig.MaxRetries
}

// GetRetryInterval returns the interval between retry attempts
func (c *ClientConfig) GetRetryInterval() time.Duration {
	return c.BaseConfig.RetryInterval
}

// getDefaultKnownHostsPath returns the default known_hosts file path
func getDefaultKnownHostsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", "known_hosts")
}

// GetDefaultKeyPaths returns a list of default SSH key paths to try
func GetDefaultKeyPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{}
	}

	sshDir := filepath.Join(home, ".ssh")
	return []string{
		filepath.Join(sshDir, "id_ed25519"),
		filepath.Join(sshDir, "id_ecdsa"),
		filepath.Join(sshDir, "id_rsa"),
		filepath.Join(sshDir, "id_dsa"),
	}
}

// FindAvailableKey searches for an available SSH key in default locations
func FindAvailableKey() (string, error) {
	for _, keyPath := range GetDefaultKeyPaths() {
		if _, err := os.Stat(keyPath); err == nil {
			return keyPath, nil
		}
	}
	return "", fmt.Errorf("no SSH key found in default locations")
}
