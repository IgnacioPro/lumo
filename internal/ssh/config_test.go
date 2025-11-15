package ssh

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/config"
)

func TestNewClientConfig(t *testing.T) {
	baseConfig := config.SSHConfig{
		Port:          22,
		Timeout:       30 * time.Second,
		KeepAlive:     15 * time.Second,
		MaxRetries:    3,
		RetryInterval: 5 * time.Second,
	}

	clientConfig := NewClientConfig(baseConfig)

	if clientConfig == nil {
		t.Fatal("NewClientConfig returned nil")
	}

	if clientConfig.BaseConfig.Port != 22 {
		t.Errorf("Port = %d, want 22", clientConfig.BaseConfig.Port)
	}

	if len(clientConfig.PreferredAuthMethods) != 4 {
		t.Errorf("PreferredAuthMethods length = %d, want 4", len(clientConfig.PreferredAuthMethods))
	}

	if clientConfig.CommandTimeout != DefaultCommandTimeout {
		t.Errorf("CommandTimeout = %v, want %v", clientConfig.CommandTimeout, DefaultCommandTimeout)
	}

	if clientConfig.OutputBufferSize != MaxOutputBufferSize {
		t.Errorf("OutputBufferSize = %d, want %d", clientConfig.OutputBufferSize, MaxOutputBufferSize)
	}

	if clientConfig.StrictHostKeyChecking {
		t.Error("StrictHostKeyChecking = true, want false")
	}
}

func TestClientConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *ClientConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &ClientConfig{
				BaseConfig: config.SSHConfig{
					Port:          22,
					Timeout:       30 * time.Second,
					MaxRetries:    3,
					RetryInterval: 5 * time.Second,
				},
				PreferredAuthMethods: []AuthMethod{AuthMethodAgent},
				CommandTimeout:       5 * time.Minute,
				OutputBufferSize:     1024 * 1024,
			},
			wantErr: false,
		},
		{
			name: "invalid port - too low",
			config: &ClientConfig{
				BaseConfig: config.SSHConfig{
					Port:          0,
					Timeout:       30 * time.Second,
					MaxRetries:    3,
					RetryInterval: 5 * time.Second,
				},
				PreferredAuthMethods: []AuthMethod{AuthMethodAgent},
				CommandTimeout:       5 * time.Minute,
				OutputBufferSize:     1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: &ClientConfig{
				BaseConfig: config.SSHConfig{
					Port:          99999,
					Timeout:       30 * time.Second,
					MaxRetries:    3,
					RetryInterval: 5 * time.Second,
				},
				PreferredAuthMethods: []AuthMethod{AuthMethodAgent},
				CommandTimeout:       5 * time.Minute,
				OutputBufferSize:     1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			config: &ClientConfig{
				BaseConfig: config.SSHConfig{
					Port:          22,
					Timeout:       -1 * time.Second,
					MaxRetries:    3,
					RetryInterval: 5 * time.Second,
				},
				PreferredAuthMethods: []AuthMethod{AuthMethodAgent},
				CommandTimeout:       5 * time.Minute,
				OutputBufferSize:     1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "negative max retries",
			config: &ClientConfig{
				BaseConfig: config.SSHConfig{
					Port:          22,
					Timeout:       30 * time.Second,
					MaxRetries:    -1,
					RetryInterval: 5 * time.Second,
				},
				PreferredAuthMethods: []AuthMethod{AuthMethodAgent},
				CommandTimeout:       5 * time.Minute,
				OutputBufferSize:     1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "no auth methods",
			config: &ClientConfig{
				BaseConfig: config.SSHConfig{
					Port:          22,
					Timeout:       30 * time.Second,
					MaxRetries:    3,
					RetryInterval: 5 * time.Second,
				},
				PreferredAuthMethods: []AuthMethod{},
				CommandTimeout:       5 * time.Minute,
				OutputBufferSize:     1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "negative command timeout",
			config: &ClientConfig{
				BaseConfig: config.SSHConfig{
					Port:          22,
					Timeout:       30 * time.Second,
					MaxRetries:    3,
					RetryInterval: 5 * time.Second,
				},
				PreferredAuthMethods: []AuthMethod{AuthMethodAgent},
				CommandTimeout:       -1 * time.Minute,
				OutputBufferSize:     1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "buffer size too small",
			config: &ClientConfig{
				BaseConfig: config.SSHConfig{
					Port:          22,
					Timeout:       30 * time.Second,
					MaxRetries:    3,
					RetryInterval: 5 * time.Second,
				},
				PreferredAuthMethods: []AuthMethod{AuthMethodAgent},
				CommandTimeout:       5 * time.Minute,
				OutputBufferSize:     512,
			},
			wantErr: true,
		},
		{
			name: "buffer size too large",
			config: &ClientConfig{
				BaseConfig: config.SSHConfig{
					Port:          22,
					Timeout:       30 * time.Second,
					MaxRetries:    3,
					RetryInterval: 5 * time.Second,
				},
				PreferredAuthMethods: []AuthMethod{AuthMethodAgent},
				CommandTimeout:       5 * time.Minute,
				OutputBufferSize:     MaxOutputBufferSize + 1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClientConfig_SetKeyPath(t *testing.T) {
	// Create a temporary key file for testing
	tmpDir := t.TempDir()
	validKeyPath := filepath.Join(tmpDir, "id_rsa")
	if err := os.WriteFile(validKeyPath, []byte("test key"), 0600); err != nil {
		t.Fatalf("Failed to create test key file: %v", err)
	}

	baseConfig := config.SSHConfig{Port: 22, Timeout: 30 * time.Second, MaxRetries: 3, RetryInterval: 5 * time.Second}
	clientConfig := NewClientConfig(baseConfig)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid key path",
			path:    validKeyPath,
			wantErr: false,
		},
		{
			name:    "non-existent path",
			path:    "/nonexistent/path/to/key",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := clientConfig.SetKeyPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetKeyPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && clientConfig.KeyPath != tt.path {
				t.Errorf("KeyPath = %s, want %s", clientConfig.KeyPath, tt.path)
			}
		})
	}
}

func TestClientConfig_Setters(t *testing.T) {
	baseConfig := config.SSHConfig{Port: 22, Timeout: 30 * time.Second, MaxRetries: 3, RetryInterval: 5 * time.Second}
	clientConfig := NewClientConfig(baseConfig)

	// Test SetPassword
	clientConfig.SetPassword("testpass")
	if clientConfig.Password != "testpass" {
		t.Errorf("Password = %s, want testpass", clientConfig.Password)
	}

	// Test SetPassphrase
	clientConfig.SetPassphrase("testphrase")
	if clientConfig.Passphrase != "testphrase" {
		t.Errorf("Passphrase = %s, want testphrase", clientConfig.Passphrase)
	}

	// Test EnableStrictHostKeyChecking
	clientConfig.EnableStrictHostKeyChecking(true)
	if !clientConfig.StrictHostKeyChecking {
		t.Error("StrictHostKeyChecking = false, want true")
	}

	clientConfig.EnableStrictHostKeyChecking(false)
	if clientConfig.StrictHostKeyChecking {
		t.Error("StrictHostKeyChecking = true, want false")
	}

	// Test SetAuthMethodPriority
	methods := []AuthMethod{AuthMethodKey, AuthMethodPassword}
	clientConfig.SetAuthMethodPriority(methods)
	if len(clientConfig.PreferredAuthMethods) != 2 {
		t.Errorf("PreferredAuthMethods length = %d, want 2", len(clientConfig.PreferredAuthMethods))
	}
	if clientConfig.PreferredAuthMethods[0] != AuthMethodKey {
		t.Errorf("PreferredAuthMethods[0] = %v, want %v", clientConfig.PreferredAuthMethods[0], AuthMethodKey)
	}
}

func TestClientConfig_Getters(t *testing.T) {
	baseConfig := config.SSHConfig{
		Port:          2222,
		Timeout:       60 * time.Second,
		KeepAlive:     45 * time.Second,
		MaxRetries:    5,
		RetryInterval: 10 * time.Second,
	}
	clientConfig := NewClientConfig(baseConfig)

	if got := clientConfig.GetTimeout(); got != 60*time.Second {
		t.Errorf("GetTimeout() = %v, want 60s", got)
	}

	if got := clientConfig.GetKeepAlive(); got != 45*time.Second {
		t.Errorf("GetKeepAlive() = %v, want 45s", got)
	}

	if got := clientConfig.GetMaxRetries(); got != 5 {
		t.Errorf("GetMaxRetries() = %d, want 5", got)
	}

	if got := clientConfig.GetRetryInterval(); got != 10*time.Second {
		t.Errorf("GetRetryInterval() = %v, want 10s", got)
	}
}

func TestGetDefaultKeyPaths(t *testing.T) {
	paths := GetDefaultKeyPaths()

	if len(paths) == 0 {
		t.Error("GetDefaultKeyPaths returned empty slice")
	}

	// Check that all paths contain expected key names
	expectedKeys := []string{"id_ed25519", "id_ecdsa", "id_rsa", "id_dsa"}
	for i, path := range paths {
		if i < len(expectedKeys) {
			if !containsKey(path, expectedKeys[i]) {
				t.Errorf("Path %s does not contain %s", path, expectedKeys[i])
			}
		}
	}
}

func TestFindAvailableKey(t *testing.T) {
	// This test will likely return an error in CI environments without SSH keys
	// We just verify it doesn't panic and returns the expected type
	key, err := FindAvailableKey()

	if err == nil && key == "" {
		t.Error("FindAvailableKey returned empty string with nil error")
	}

	// If a key is found, verify it's one of the default paths
	if err == nil && key != "" {
		found := false
		for _, defaultPath := range GetDefaultKeyPaths() {
			if key == defaultPath {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("FindAvailableKey returned unexpected path: %s", key)
		}
	}
}

func TestClientConfig_ValidateKeyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a key file with insecure permissions
	insecureKeyPath := filepath.Join(tmpDir, "insecure_key")
	if err := os.WriteFile(insecureKeyPath, []byte("test key"), 0644); err != nil {
		t.Fatalf("Failed to create insecure key file: %v", err)
	}

	// Create a key file with secure permissions
	secureKeyPath := filepath.Join(tmpDir, "secure_key")
	if err := os.WriteFile(secureKeyPath, []byte("test key"), 0600); err != nil {
		t.Fatalf("Failed to create secure key file: %v", err)
	}

	baseConfig := config.SSHConfig{
		Port:          22,
		Timeout:       30 * time.Second,
		MaxRetries:    3,
		RetryInterval: 5 * time.Second,
	}

	tests := []struct {
		name    string
		keyPath string
		wantErr bool
	}{
		{
			name:    "secure key file",
			keyPath: secureKeyPath,
			wantErr: false,
		},
		{
			name:    "insecure key file",
			keyPath: insecureKeyPath,
			wantErr: true,
		},
		{
			name:    "non-existent key file",
			keyPath: "/nonexistent/key",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientConfig := NewClientConfig(baseConfig)
			clientConfig.KeyPath = tt.keyPath
			clientConfig.PreferredAuthMethods = []AuthMethod{AuthMethodKey}
			clientConfig.CommandTimeout = 5 * time.Minute
			clientConfig.OutputBufferSize = 1024 * 1024

			err := clientConfig.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function
func containsKey(path, key string) bool {
	return filepath.Base(path) == key
}
