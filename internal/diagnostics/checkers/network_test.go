package checkers

import (
	"context"
	"strings"
	"testing"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestNewNetworkChecker(t *testing.T) {
	targets := []config.NetworkTarget{
		{Host: "8.8.8.8", Port: 0, Protocol: "icmp"},
		{Host: "example.com", Port: 80, Protocol: "tcp"},
	}

	thresholds := diagnostics.NetworkThresholds{
		ConnectionsWarn: 1000,
	}

	checker := NewNetworkChecker(thresholds, targets)

	if checker == nil {
		t.Fatal("NewNetworkChecker returned nil")
	}

	if len(checker.targets) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(checker.targets))
	}
}

func TestNetworkChecker_Name(t *testing.T) {
	checker := NewNetworkChecker(diagnostics.NetworkThresholds{}, nil)
	if got := checker.Name(); got != "network_check" {
		t.Errorf("Name() = %v, want network_check", got)
	}
}

func TestNetworkChecker_Category(t *testing.T) {
	checker := NewNetworkChecker(diagnostics.NetworkThresholds{}, nil)
	if got := checker.Category(); got != diagnostics.CategoryNetwork {
		t.Errorf("Category() = %v, want %v", got, diagnostics.CategoryNetwork)
	}
}

func TestNetworkChecker_Description(t *testing.T) {
	checker := NewNetworkChecker(diagnostics.NetworkThresholds{}, nil)
	desc := checker.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if !strings.Contains(strings.ToLower(desc), "network") {
		t.Error("Description should mention network")
	}
}

func TestNetworkChecker_RequiresRoot(t *testing.T) {
	checker := NewNetworkChecker(diagnostics.NetworkThresholds{}, nil)
	if checker.RequiresRoot() {
		t.Error("NetworkChecker should not require root")
	}
}

func TestNetworkChecker_Run(t *testing.T) {
	targets := []config.NetworkTarget{
		{Host: "8.8.8.8", Port: 0, Protocol: "icmp"},
		{Host: "example.com", Port: 80, Protocol: "tcp"},
	}

	thresholds := diagnostics.NetworkThresholds{
		ConnectionsWarn: 1000,
	}

	executor := &mockExecutor{
		responses: map[string]mockResponse{
			"ip -br addr": {
				stdout:   "eth0             UP             192.168.1.100/24\nlo               UP             127.0.0.1/8\n",
				exitCode: 0,
			},
			"ss -tan": {
				stdout:   "250\n",
				exitCode: 0,
			},
			"ping -c 1 -W 2 8.8.8.8": {
				stdout:   "time=15.2 ms\n",
				exitCode: 0,
			},
			"nc -zv -w 2 example.com 80": {
				stdout:   "Connection to example.com 80 port [tcp/http] succeeded!\n",
				exitCode: 0,
			},
		},
	}

	checker := NewNetworkChecker(thresholds, targets)
	result, err := checker.Run(context.Background(), executor)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	if result.Name != "network_check" {
		t.Errorf("Name = %v, want network_check", result.Name)
	}

	if result.Category != diagnostics.CategoryNetwork {
		t.Errorf("Category = %v, want %v", result.Category, diagnostics.CategoryNetwork)
	}
}

func TestNetworkChecker_RunWithErrors(t *testing.T) {
	executor := &mockExecutor{
		responses: map[string]mockResponse{
			"ip -br addr": {
				stderr:   "command not found",
				exitCode: 127,
			},
			"ss -tan": {
				stderr:   "ss: command not found",
				exitCode: 127,
			},
		},
	}

	checker := NewNetworkChecker(diagnostics.NetworkThresholds{}, nil)
	result, err := checker.Run(context.Background(), executor)

	// Should still complete even with errors
	if err != nil {
		t.Fatalf("Run() should not error even with command failures, got: %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// Check that warnings were recorded
	if _, exists := result.Data["interfaces_warning"]; !exists {
		t.Error("Expected interfaces_warning in result data")
	}
}

func TestNetworkChecker_GetConnectionCount(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    int
		wantErr bool
	}{
		{
			name:    "valid count",
			output:  "42\n",
			want:    42,
			wantErr: false,
		},
		{
			name:    "zero connections",
			output:  "0\n",
			want:    0,
			wantErr: false,
		},
		{
			name:    "with whitespace",
			output:  "  123  \n",
			want:    123,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				responses: map[string]mockResponse{
					"ss": {
						stdout:   tt.output,
						exitCode: 0,
					},
				},
			}

			checker := NewNetworkChecker(diagnostics.NetworkThresholds{}, nil)
			got, err := checker.getConnectionCount(context.Background(), executor)

			if (err != nil) != tt.wantErr {
				t.Errorf("getConnectionCount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("getConnectionCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

// classifyConnectivityError is private, testing behavior via Run() instead

func TestNetworkChecker_ParseIfconfig(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantCount int
		wantIface map[string]struct {
			ipAddr string
			state  string
			status string
		}
	}{
		{
			name: "macOS format - active interface",
			output: `en0: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST> mtu 1500
	inet 192.168.1.100 netmask 0xffffff00 broadcast 192.168.1.255
	status: active`,
			wantCount: 1,
			wantIface: map[string]struct {
				ipAddr string
				state  string
				status string
			}{
				"en0": {ipAddr: "192.168.1.100", state: "up", status: "active"},
			},
		},
		{
			name: "macOS format - inactive interface",
			output: `en1: flags=8822<BROADCAST,SMART,SIMPLEX,MULTICAST> mtu 1500
	status: inactive`,
			wantCount: 1,
			wantIface: map[string]struct {
				ipAddr string
				state  string
				status string
			}{
				"en1": {ipAddr: "", state: "down", status: "inactive"},
			},
		},
		{
			name: "multiple interfaces",
			output: `lo0: flags=8049<UP,LOOPBACK,RUNNING,MULTICAST> mtu 16384
	inet 127.0.0.1 netmask 0xff000000
	status: active
en0: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST> mtu 1500
	inet 192.168.1.100 netmask 0xffffff00
	status: active`,
			wantCount: 2,
			wantIface: map[string]struct {
				ipAddr string
				state  string
				status string
			}{
				"lo0": {ipAddr: "127.0.0.1", state: "up", status: "active"},
				"en0": {ipAddr: "192.168.1.100", state: "up", status: "active"},
			},
		},
		{
			name: "interface without IP",
			output: `en1: flags=8822<BROADCAST,SMART,SIMPLEX,MULTICAST> mtu 1500
	status: inactive`,
			wantCount: 1,
			wantIface: map[string]struct {
				ipAddr string
				state  string
				status string
			}{
				"en1": {ipAddr: "", state: "down", status: "inactive"},
			},
		},
		{
			name: "interface with status but no flags",
			output: `eth0: mtu 1500
	inet 10.0.0.1 netmask 255.255.255.0
	status: active`,
			wantCount: 1,
			wantIface: map[string]struct {
				ipAddr string
				state  string
				status string
			}{
				"eth0": {ipAddr: "10.0.0.1", state: "up", status: "active"},
			},
		},
		{
			name:      "empty output",
			output:    "",
			wantCount: 0,
			wantIface: map[string]struct {
				ipAddr string
				state  string
				status string
			}{},
		},
		{
			name: "only whitespace",
			output: `

`,
			wantCount: 0,
			wantIface: map[string]struct {
				ipAddr string
				state  string
				status string
			}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewNetworkChecker(diagnostics.NetworkThresholds{}, nil)
			interfaces, err := checker.parseIfconfig(tt.output)

			if err != nil {
				t.Fatalf("parseIfconfig() error = %v", err)
			}

			if len(interfaces) != tt.wantCount {
				t.Errorf("parseIfconfig() returned %d interfaces, want %d", len(interfaces), tt.wantCount)
			}

			for _, iface := range interfaces {
				want, ok := tt.wantIface[iface.Name]
				if !ok {
					t.Errorf("Unexpected interface: %s", iface.Name)
					continue
				}

				if iface.IPAddress != want.ipAddr {
					t.Errorf("Interface %s: IPAddress = %q, want %q", iface.Name, iface.IPAddress, want.ipAddr)
				}

				if iface.State != want.state {
					t.Errorf("Interface %s: State = %q, want %q", iface.Name, iface.State, want.state)
				}

				if iface.Status != want.status {
					t.Errorf("Interface %s: Status = %q, want %q", iface.Name, iface.Status, want.status)
				}
			}
		})
	}
}
