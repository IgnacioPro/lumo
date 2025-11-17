package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
