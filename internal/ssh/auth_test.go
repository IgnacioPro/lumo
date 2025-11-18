package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/config"
)

func TestDetectKeyType(t *testing.T) {
	tests := []struct {
		name     string
		keyBytes []byte
		want     string
	}{
		{
			name:     "RSA key",
			keyBytes: []byte("-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----"),
			want:     "RSA",
		},
		{
			name:     "OpenSSH key",
			keyBytes: []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA...\n-----END OPENSSH PRIVATE KEY-----"),
			want:     "OpenSSH",
		},
		{
			name:     "ECDSA key",
			keyBytes: []byte("-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIIGc...\n-----END EC PRIVATE KEY-----"),
			want:     "ECDSA",
		},
		{
			name:     "DSA key",
			keyBytes: []byte("-----BEGIN DSA PRIVATE KEY-----\nMIIBuwIBAAKBgQC...\n-----END DSA PRIVATE KEY-----"),
			want:     "DSA",
		},
		{
			name:     "unknown key type",
			keyBytes: []byte("This is not a valid key"),
			want:     "Unknown",
		},
		{
			name:     "empty key",
			keyBytes: []byte(""),
			want:     "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectKeyType(tt.keyBytes)
			if got != tt.want {
				t.Errorf("detectKeyType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateKeyFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setupFile   func() string // returns file path
		wantErr     bool
		errContains string
	}{
		{
			name: "valid key file with 0600 permissions",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "key_600")
				if err := os.WriteFile(path, []byte("test key content"), 0600); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			wantErr: false,
		},
		{
			name: "valid key file with 0400 permissions",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "key_400")
				if err := os.WriteFile(path, []byte("test key content"), 0400); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			wantErr: false,
		},
		{
			name: "insecure permissions 0644",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "key_644")
				if err := os.WriteFile(path, []byte("test key content"), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			wantErr:     true,
			errContains: "insecure permissions",
		},
		{
			name: "insecure permissions 0777",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "key_777")
				if err := os.WriteFile(path, []byte("test key content"), 0777); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			wantErr:     true,
			errContains: "insecure permissions",
		},
		{
			name: "nonexistent file",
			setupFile: func() string {
				return filepath.Join(tmpDir, "nonexistent_key")
			},
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setupFile()

			err := validateKeyFile(filePath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateKeyFile() error = nil, want error containing %q", tt.errContains)
					return
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("validateKeyFile() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("validateKeyFile() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateKeyFile_PermissionChecks(t *testing.T) {
	// Additional tests for specific permission scenarios
	tmpDir := t.TempDir()

	testCases := []struct {
		perm        os.FileMode
		shouldPass  bool
		description string
	}{
		{0400, true, "read-only for owner"},
		{0600, true, "read-write for owner only"},
		{0500, false, "read-execute for owner"},
		{0700, false, "full permissions for owner"},
		{0640, false, "read for group"},
		{0604, false, "read for others"},
		{0666, false, "world readable/writable"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, "test_key")
			if err := os.WriteFile(filePath, []byte("test"), tc.perm); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			defer func() {
				_ = os.Remove(filePath)
			}()

			err := validateKeyFile(filePath)

			if tc.shouldPass && err != nil {
				t.Errorf("Permission %o: expected to pass but got error: %v", tc.perm, err)
			}
			if !tc.shouldPass && err == nil {
				t.Errorf("Permission %o: expected to fail but passed", tc.perm)
			}
		})
	}
}

func TestTrySSHAgent_NoAuthSock(t *testing.T) {
	// Save original SSH_AUTH_SOCK
	originalAuthSock := os.Getenv("SSH_AUTH_SOCK")
	defer func() {
		if originalAuthSock != "" {
			_ = os.Setenv("SSH_AUTH_SOCK", originalAuthSock)
		} else {
			_ = os.Unsetenv("SSH_AUTH_SOCK")
		}
	}()

	// Unset SSH_AUTH_SOCK
	_ = os.Unsetenv("SSH_AUTH_SOCK")

	authMethod, err := trySSHAgent()

	if err == nil {
		t.Error("trySSHAgent() should return error when SSH_AUTH_SOCK not set")
	}

	if authMethod != nil {
		t.Error("trySSHAgent() should return nil auth method on error")
	}

	if !strings.Contains(err.Error(), "SSH_AUTH_SOCK not set") {
		t.Errorf("Error should mention SSH_AUTH_SOCK, got: %v", err)
	}
}

func TestTrySSHAgent_InvalidSocket(t *testing.T) {
	// Save original SSH_AUTH_SOCK
	originalAuthSock := os.Getenv("SSH_AUTH_SOCK")
	defer func() {
		if originalAuthSock != "" {
			_ = os.Setenv("SSH_AUTH_SOCK", originalAuthSock)
		} else {
			_ = os.Unsetenv("SSH_AUTH_SOCK")
		}
	}()

	// Set SSH_AUTH_SOCK to nonexistent socket
	_ = os.Setenv("SSH_AUTH_SOCK", "/nonexistent/ssh-agent.sock")

	authMethod, err := trySSHAgent()

	if err == nil {
		t.Error("trySSHAgent() should return error for invalid socket")
	}

	if authMethod != nil {
		t.Error("trySSHAgent() should return nil auth method on error")
	}

	if !strings.Contains(err.Error(), "failed to connect to SSH agent") {
		t.Errorf("Error should mention connection failure, got: %v", err)
	}
}

func TestTryKeyFile_NonexistentFile(t *testing.T) {
	authMethod, err := tryKeyFile("/nonexistent/key/path", "", "testuser", "testhost")

	if err == nil {
		t.Error("tryKeyFile() should return error for nonexistent file")
	}

	if authMethod != nil {
		t.Error("tryKeyFile() should return nil auth method on error")
	}

	if !strings.Contains(err.Error(), "failed to read key file") {
		t.Errorf("Error should mention failed to read key file, got: %v", err)
	}
}

func TestTryKeyFile_InvalidKey(t *testing.T) {
	// Create a temporary file with invalid key content
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "invalid_key")
	if err := os.WriteFile(keyPath, []byte("this is not a valid SSH key"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	authMethod, err := tryKeyFile(keyPath, "", "testuser", "testhost")

	if err == nil {
		t.Error("tryKeyFile() should return error for invalid key format")
	}

	if authMethod != nil {
		t.Error("tryKeyFile() should return nil auth method on error")
	}

	if !strings.Contains(err.Error(), "failed to parse private key") {
		t.Errorf("Error should mention parse failure, got: %v", err)
	}
}

func TestGetHostKeyCallback_StrictCheckingDisabled(t *testing.T) {
	cfg := &ClientConfig{
		BaseConfig: config.SSHConfig{
			Port:          22,
			Timeout:       30 * time.Second,
			KeepAlive:     15 * time.Second,
			MaxRetries:    3,
			RetryInterval: 5 * time.Second,
		},
		StrictHostKeyChecking: false,
		KnownHostsPath:        "", // Empty known_hosts path
		PreferredAuthMethods:  []AuthMethod{AuthMethodAgent},
		CommandTimeout:        5 * time.Minute,
		OutputBufferSize:      1024 * 1024,
	}

	callback, err := getHostKeyCallback(cfg)

	if err != nil {
		t.Errorf("getHostKeyCallback() unexpected error: %v", err)
	}

	if callback == nil {
		t.Error("getHostKeyCallback() should return non-nil callback")
	}

	// The callback should be InsecureIgnoreHostKey when strict checking is disabled
	// We can't easily test the exact type, but we verified it returns without error
}

func TestGetHostKeyCallback_NoKnownHostsPath(t *testing.T) {
	cfg := &ClientConfig{
		BaseConfig: config.SSHConfig{
			Port:          22,
			Timeout:       30 * time.Second,
			KeepAlive:     15 * time.Second,
			MaxRetries:    3,
			RetryInterval: 5 * time.Second,
		},
		StrictHostKeyChecking: true,
		KnownHostsPath:        "", // Empty known_hosts path
		PreferredAuthMethods:  []AuthMethod{AuthMethodAgent},
		CommandTimeout:        5 * time.Minute,
		OutputBufferSize:      1024 * 1024,
	}

	callback, err := getHostKeyCallback(cfg)

	if err != nil {
		t.Errorf("getHostKeyCallback() unexpected error: %v", err)
	}

	if callback == nil {
		t.Error("getHostKeyCallback() should return non-nil callback (insecure fallback)")
	}
}

func TestGetHostKeyCallback_InvalidKnownHostsPath(t *testing.T) {
	cfg := &ClientConfig{
		BaseConfig: config.SSHConfig{
			Port:          22,
			Timeout:       30 * time.Second,
			KeepAlive:     15 * time.Second,
			MaxRetries:    3,
			RetryInterval: 5 * time.Second,
		},
		StrictHostKeyChecking: true,
		KnownHostsPath:        "/nonexistent/known_hosts",
		PreferredAuthMethods:  []AuthMethod{AuthMethodAgent},
		CommandTimeout:        5 * time.Minute,
		OutputBufferSize:      1024 * 1024,
	}

	callback, err := getHostKeyCallback(cfg)

	if err == nil {
		t.Error("getHostKeyCallback() should return error for nonexistent known_hosts file")
	}

	if callback != nil {
		t.Error("getHostKeyCallback() should return nil callback on error")
	}

	if !strings.Contains(err.Error(), "failed to load known_hosts file") {
		t.Errorf("Error should mention known_hosts loading failure, got: %v", err)
	}
}

func TestBuildAuthMethods_NoMethodsAvailable(t *testing.T) {
	cfg := &ClientConfig{
		BaseConfig: config.SSHConfig{
			Port:          22,
			Timeout:       30 * time.Second,
			KeepAlive:     15 * time.Second,
			MaxRetries:    3,
			RetryInterval: 5 * time.Second,
		},
		StrictHostKeyChecking: true,
		PreferredAuthMethods:  []AuthMethod{}, // No auth methods
		CommandTimeout:        5 * time.Minute,
		OutputBufferSize:      1024 * 1024,
	}

	authMethods, authTypes, err := buildAuthMethods(cfg, "testhost", "testuser")

	if err == nil {
		t.Error("buildAuthMethods() should return error when no auth methods configured")
	}

	if authMethods != nil {
		t.Error("buildAuthMethods() should return nil authMethods on error")
	}

	if authTypes != nil {
		t.Error("buildAuthMethods() should return nil authTypes on error")
	}

	if !strings.Contains(err.Error(), "no authentication methods available") {
		t.Errorf("Error should mention no methods available, got: %v", err)
	}
}

func TestBuildAuthMethods_PasswordMethod(t *testing.T) {
	cfg := &ClientConfig{
		BaseConfig: config.SSHConfig{
			Port:          22,
			Timeout:       30 * time.Second,
			KeepAlive:     15 * time.Second,
			MaxRetries:    3,
			RetryInterval: 5 * time.Second,
		},
		StrictHostKeyChecking: false,
		Password:              "testpassword", // Password provided
		PreferredAuthMethods:  []AuthMethod{AuthMethodPassword},
		CommandTimeout:        5 * time.Minute,
		OutputBufferSize:      1024 * 1024,
	}

	authMethods, authTypes, err := buildAuthMethods(cfg, "testhost", "testuser")

	if err != nil {
		t.Errorf("buildAuthMethods() unexpected error: %v", err)
	}

	if authMethods == nil {
		t.Fatal("buildAuthMethods() should return non-nil authMethods")
	}

	if len(authMethods) != 1 {
		t.Errorf("Expected 1 auth method, got %d", len(authMethods))
	}

	if len(authTypes) != 1 {
		t.Errorf("Expected 1 auth type, got %d", len(authTypes))
	}

	if authTypes[0] != AuthMethodPassword {
		t.Errorf("Expected auth type Password, got %v", authTypes[0])
	}
}

func TestBuildAuthMethods_AgentMethodFailover(t *testing.T) {
	// Unset SSH_AUTH_SOCK to ensure agent auth fails
	originalAuthSock := os.Getenv("SSH_AUTH_SOCK")
	defer func() {
		if originalAuthSock != "" {
			_ = os.Setenv("SSH_AUTH_SOCK", originalAuthSock)
		} else {
			_ = os.Unsetenv("SSH_AUTH_SOCK")
		}
	}()
	_ = os.Unsetenv("SSH_AUTH_SOCK")

	cfg := &ClientConfig{
		BaseConfig: config.SSHConfig{
			Port:          22,
			Timeout:       30 * time.Second,
			KeepAlive:     15 * time.Second,
			MaxRetries:    3,
			RetryInterval: 5 * time.Second,
		},
		StrictHostKeyChecking: false,
		PreferredAuthMethods:  []AuthMethod{AuthMethodAgent}, // Agent method will fail
		CommandTimeout:        5 * time.Minute,
		OutputBufferSize:      1024 * 1024,
	}

	authMethods, authTypes, err := buildAuthMethods(cfg, "testhost", "testuser")

	// Should return error because agent is unavailable and no other methods
	if err == nil {
		t.Error("buildAuthMethods() should return error when agent unavailable and no fallback")
	}

	if !strings.Contains(err.Error(), "no authentication methods available") {
		t.Errorf("Error should mention no methods available, got: %v", err)
	}

	if authMethods != nil || authTypes != nil {
		t.Error("buildAuthMethods() should return nil when all methods fail")
	}
}
