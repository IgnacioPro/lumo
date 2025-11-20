package ssh

import (
	"net"
	"strings"
	"testing"
)

func TestValidateSSHTarget(t *testing.T) {
	tests := []struct {
		name          string
		target        string
		expectError   bool
		errorContains string
		desc          string
	}{
		{
			name:        "valid public IP",
			target:      "8.8.8.8",
			expectError: false,
			desc:        "public IP should be allowed",
		},
		{
			name:        "valid hostname",
			target:      "example.com",
			expectError: false,
			desc:        "public hostname should be allowed",
		},
		{
			name:        "valid IP with port",
			target:      "8.8.8.8:22",
			expectError: false,
			desc:        "public IP with port should be allowed",
		},
		{
			name:          "private IP 10.x",
			target:        "10.0.0.1",
			expectError:   true,
			errorContains: "private network",
			desc:          "10.0.0.0/8 should be blocked",
		},
		{
			name:          "private IP 192.168.x",
			target:        "192.168.1.1",
			expectError:   true,
			errorContains: "private network",
			desc:          "192.168.0.0/16 should be blocked",
		},
		{
			name:          "private IP 172.16.x",
			target:        "172.16.0.1",
			expectError:   true,
			errorContains: "private network",
			desc:          "172.16.0.0/12 should be blocked",
		},
		{
			name:          "loopback 127.0.0.1",
			target:        "127.0.0.1",
			expectError:   true,
			errorContains: "loopback",
			desc:          "loopback should be blocked",
		},
		{
			name:          "localhost",
			target:        "localhost",
			expectError:   true,
			errorContains: "loopback",
			desc:          "localhost should resolve to 127.0.0.1 and be blocked",
		},
		{
			name:          "metadata service IP",
			target:        "169.254.169.254",
			expectError:   true,
			errorContains: "metadata service",
			desc:          "cloud metadata service should be blocked",
		},
		{
			name:          "link-local",
			target:        "169.254.1.1",
			expectError:   true,
			errorContains: "link-local",
			desc:          "link-local range should be blocked",
		},
		{
			name:          "multicast",
			target:        "224.0.0.1",
			expectError:   true,
			errorContains: "multicast",
			desc:          "multicast range should be blocked",
		},
		{
			name:          "broadcast",
			target:        "255.255.255.255",
			expectError:   true,
			errorContains: "broadcast",
			desc:          "broadcast address should be blocked",
		},
		{
			name:          "zero address",
			target:        "0.0.0.0",
			expectError:   true,
			errorContains: "zero",
			desc:          "zero address should be blocked",
		},
		{
			name:          "empty target",
			target:        "",
			expectError:   true,
			errorContains: "cannot be empty",
			desc:          "empty target should be rejected",
		},
		{
			name:          "IPv6 loopback",
			target:        "::1",
			expectError:   true,
			errorContains: "loopback",
			desc:          "IPv6 loopback should be blocked",
		},
		{
			name:          "IPv6 link-local",
			target:        "fe80::1",
			expectError:   true,
			errorContains: "link-local",
			desc:          "IPv6 link-local should be blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSSHTarget(tt.target)

			if tt.expectError && err == nil {
				t.Errorf("ValidateSSHTarget(%q) expected error but got none (%s)", tt.target, tt.desc)
			}

			if !tt.expectError && err != nil {
				t.Errorf("ValidateSSHTarget(%q) unexpected error: %v (%s)", tt.target, err, tt.desc)
			}

			if tt.expectError && err != nil && tt.errorContains != "" {
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errorContains)) {
					t.Errorf("ValidateSSHTarget(%q) error %q does not contain %q (%s)",
						tt.target, err.Error(), tt.errorContains, tt.desc)
				}
			}
		})
	}
}

func TestValidateSSHTargetWithAllowlist(t *testing.T) {
	tests := []struct {
		name          string
		target        string
		allowlist     []string
		expectError   bool
		errorContains string
		desc          string
	}{
		{
			name:        "target in allowlist",
			target:      "8.8.8.8",
			allowlist:   []string{"8.8.8.8", "1.1.1.1"},
			expectError: false,
			desc:        "target in allowlist should be allowed",
		},
		{
			name:          "target not in allowlist",
			target:        "1.1.1.1",
			allowlist:     []string{"8.8.8.8"},
			expectError:   true,
			errorContains: "not in allowlist",
			desc:          "target not in allowlist should be rejected",
		},
		{
			name:          "blocked IP even with allowlist",
			target:        "127.0.0.1",
			allowlist:     []string{"127.0.0.1"},
			expectError:   true,
			errorContains: "loopback",
			desc:          "blocked IPs should be rejected even if in allowlist",
		},
		{
			name:        "empty allowlist allows all",
			target:      "example.com",
			allowlist:   []string{},
			expectError: false,
			desc:        "empty allowlist should allow any valid target",
		},
		{
			name:        "CIDR in allowlist - match",
			target:      "203.0.114.5",
			allowlist:   []string{"203.0.114.0/24"},
			expectError: false,
			desc:        "IP in allowed CIDR should be allowed",
		},
		{
			name:          "CIDR in allowlist - no match",
			target:        "203.0.115.5",
			allowlist:     []string{"203.0.114.0/24"},
			expectError:   true,
			errorContains: "not in allowlist",
			desc:          "IP not in allowed CIDR should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSSHTargetWithAllowlist(tt.target, tt.allowlist)

			if tt.expectError && err == nil {
				t.Errorf("ValidateSSHTargetWithAllowlist(%q, %v) expected error but got none (%s)",
					tt.target, tt.allowlist, tt.desc)
			}

			if !tt.expectError && err != nil {
				t.Errorf("ValidateSSHTargetWithAllowlist(%q, %v) unexpected error: %v (%s)",
					tt.target, tt.allowlist, err, tt.desc)
			}

			if tt.expectError && err != nil && tt.errorContains != "" {
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errorContains)) {
					t.Errorf("ValidateSSHTargetWithAllowlist(%q, %v) error %q does not contain %q (%s)",
						tt.target, tt.allowlist, err.Error(), tt.errorContains, tt.desc)
				}
			}
		})
	}
}

func TestValidateIPNotBlocked(t *testing.T) {
	tests := []struct {
		name          string
		ip            string
		expectError   bool
		errorContains string
		desc          string
	}{
		{
			name:        "public IPv4",
			ip:          "8.8.8.8",
			expectError: false,
			desc:        "public IPv4 should be allowed",
		},
		{
			name:          "private 10.x",
			ip:            "10.1.2.3",
			expectError:   true,
			errorContains: "private network",
			desc:          "10.0.0.0/8 should be blocked",
		},
		{
			name:          "AWS metadata service",
			ip:            "169.254.169.254",
			expectError:   true,
			errorContains: "metadata service",
			desc:          "AWS metadata service IP should be specifically blocked",
		},
		{
			name:          "carrier-grade NAT",
			ip:            "100.64.1.1",
			expectError:   true,
			errorContains: "carrier-grade NAT",
			desc:          "100.64.0.0/10 should be blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := parseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}

			err := validateIPNotBlocked(ip)

			if tt.expectError && err == nil {
				t.Errorf("validateIPNotBlocked(%q) expected error but got none (%s)", tt.ip, tt.desc)
			}

			if !tt.expectError && err != nil {
				t.Errorf("validateIPNotBlocked(%q) unexpected error: %v (%s)", tt.ip, err, tt.desc)
			}

			if tt.expectError && err != nil && tt.errorContains != "" {
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errorContains)) {
					t.Errorf("validateIPNotBlocked(%q) error %q does not contain %q (%s)",
						tt.ip, err.Error(), tt.errorContains, tt.desc)
				}
			}
		})
	}
}

// Helper function to parse IP for tests
func parseIP(ipStr string) net.IP {
	return net.ParseIP(ipStr)
}
