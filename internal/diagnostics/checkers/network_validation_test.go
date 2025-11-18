package checkers

import (
	"strings"
	"testing"

	"github.com/ignacio/lumo/internal/config"
)

func TestValidateNetworkTarget(t *testing.T) {
	tests := []struct {
		name    string
		target  config.NetworkTarget
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid TCP target",
			target: config.NetworkTarget{
				Host:     "example.com",
				Port:     443,
				Protocol: "tcp",
			},
			wantErr: false,
		},
		{
			name: "valid ICMP target",
			target: config.NetworkTarget{
				Host:     "8.8.8.8",
				Port:     0,
				Protocol: "icmp",
			},
			wantErr: false,
		},
		{
			name: "valid IPv6 host",
			target: config.NetworkTarget{
				Host:     "2001:4860:4860::8888",
				Port:     80,
				Protocol: "tcp",
			},
			wantErr: false,
		},
		{
			name: "valid hostname with hyphen",
			target: config.NetworkTarget{
				Host:     "my-server.example.com",
				Port:     22,
				Protocol: "tcp",
			},
			wantErr: false,
		},
		{
			name: "empty host",
			target: config.NetworkTarget{
				Host:     "",
				Port:     80,
				Protocol: "tcp",
			},
			wantErr: true,
			errMsg:  "host cannot be empty",
		},
		{
			name: "host with semicolon (command injection attempt)",
			target: config.NetworkTarget{
				Host:     "example.com; rm -rf /",
				Port:     80,
				Protocol: "tcp",
			},
			wantErr: true,
			errMsg:  "contains shell metacharacters",
		},
		{
			name: "host with pipe (command injection attempt)",
			target: config.NetworkTarget{
				Host:     "example.com | cat /etc/passwd",
				Port:     80,
				Protocol: "tcp",
			},
			wantErr: true,
			errMsg:  "contains shell metacharacters",
		},
		{
			name: "host with backticks (command injection attempt)",
			target: config.NetworkTarget{
				Host:     "example.com`whoami`",
				Port:     80,
				Protocol: "tcp",
			},
			wantErr: true,
			errMsg:  "contains shell metacharacters",
		},
		{
			name: "host with dollar sign (variable expansion attempt)",
			target: config.NetworkTarget{
				Host:     "example.com$PATH",
				Port:     80,
				Protocol: "tcp",
			},
			wantErr: true,
			errMsg:  "contains shell metacharacters",
		},
		{
			name: "host with ampersand (background execution attempt)",
			target: config.NetworkTarget{
				Host:     "example.com & sleep 10",
				Port:     80,
				Protocol: "tcp",
			},
			wantErr: true,
			errMsg:  "contains shell metacharacters",
		},
		{
			name: "host with newline (command injection attempt)",
			target: config.NetworkTarget{
				Host:     "example.com\nrm -rf /",
				Port:     80,
				Protocol: "tcp",
			},
			wantErr: true,
			errMsg:  "contains shell metacharacters",
		},
		{
			name: "port too low",
			target: config.NetworkTarget{
				Host:     "example.com",
				Port:     -1,
				Protocol: "tcp",
			},
			wantErr: true,
			errMsg:  "invalid port",
		},
		{
			name: "port too high",
			target: config.NetworkTarget{
				Host:     "example.com",
				Port:     65536,
				Protocol: "tcp",
			},
			wantErr: true,
			errMsg:  "invalid port",
		},
		{
			name: "invalid protocol",
			target: config.NetworkTarget{
				Host:     "example.com",
				Port:     80,
				Protocol: "udp",
			},
			wantErr: true,
			errMsg:  "invalid protocol",
		},
		{
			name: "empty protocol (defaults to icmp)",
			target: config.NetworkTarget{
				Host:     "example.com",
				Port:     0,
				Protocol: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNetworkTarget(tt.target)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateNetworkTarget() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateNetworkTarget() error = %v, should contain %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateNetworkTarget() unexpected error = %v", err)
				}
			}
		})
	}
}
