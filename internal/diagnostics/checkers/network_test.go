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

func TestNetworkChecker_ParseProcNetDev(t *testing.T) {
	checker := NewNetworkChecker(diagnostics.NetworkThresholds{}, nil)

	t.Run("parses /proc/net/dev output", func(t *testing.T) {
		output := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 123456789  654321    5    0    0     0          0         0  987654321  456789    2    0    0     0       0          0
wlan0: 987654321  123456    0    0    0     0          0         0  123456789  654321    0    0    0     0       0          0
    lo: 111111111   22222    0    0    0     0          0         0  111111111   22222    0    0    0     0       0          0`

		stats, err := checker.parseProcNetDev(output)

		if err != nil {
			t.Errorf("parseProcNetDev() unexpected error: %v", err)
		}

		// Should sum eth0 + wlan0 (lo is skipped)
		expectedRxBytes := uint64(123456789 + 987654321)
		if stats.RxBytes != expectedRxBytes {
			t.Errorf("RxBytes = %d, want %d", stats.RxBytes, expectedRxBytes)
		}

		expectedRxPackets := uint64(654321 + 123456)
		if stats.RxPackets != expectedRxPackets {
			t.Errorf("RxPackets = %d, want %d", stats.RxPackets, expectedRxPackets)
		}

		expectedRxErrors := uint64(5 + 0)
		if stats.RxErrors != expectedRxErrors {
			t.Errorf("RxErrors = %d, want %d", stats.RxErrors, expectedRxErrors)
		}

		expectedTxBytes := uint64(987654321 + 123456789)
		if stats.TxBytes != expectedTxBytes {
			t.Errorf("TxBytes = %d, want %d", stats.TxBytes, expectedTxBytes)
		}

		expectedTxPackets := uint64(456789 + 654321)
		if stats.TxPackets != expectedTxPackets {
			t.Errorf("TxPackets = %d, want %d", stats.TxPackets, expectedTxPackets)
		}

		expectedTxErrors := uint64(2 + 0)
		if stats.TxErrors != expectedTxErrors {
			t.Errorf("TxErrors = %d, want %d", stats.TxErrors, expectedTxErrors)
		}
	})

	t.Run("skips loopback interface", func(t *testing.T) {
		output := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 111111111   22222    0    0    0     0          0         0  111111111   22222    0    0    0     0       0          0`

		stats, err := checker.parseProcNetDev(output)

		if err != nil {
			t.Errorf("parseProcNetDev() unexpected error: %v", err)
		}

		// Should skip loopback, all stats should be 0
		if stats.RxBytes != 0 || stats.RxPackets != 0 || stats.TxBytes != 0 || stats.TxPackets != 0 {
			t.Errorf("parseProcNetDev() should skip loopback interface, got non-zero stats: %+v", stats)
		}
	})

	t.Run("handles empty output", func(t *testing.T) {
		stats, err := checker.parseProcNetDev("")

		if err != nil {
			t.Errorf("parseProcNetDev() unexpected error on empty output: %v", err)
		}

		// Should return zero stats
		if stats.RxBytes != 0 || stats.RxPackets != 0 {
			t.Errorf("parseProcNetDev() should return zero stats for empty output, got: %+v", stats)
		}
	})

	t.Run("handles malformed lines", func(t *testing.T) {
		output := `Inter-|   Receive                                                |  Transmit
  eth0: 123456789  654321    5
wlan0: invalid data here`

		stats, err := checker.parseProcNetDev(output)

		// Should not error, just skip malformed lines
		if err != nil {
			t.Errorf("parseProcNetDev() unexpected error: %v", err)
		}

		// Should have zero stats because lines have < 17 fields
		if stats.RxBytes != 0 {
			t.Errorf("parseProcNetDev() should skip malformed lines, got RxBytes=%d", stats.RxBytes)
		}
	})
}

func TestNetworkChecker_ParseNetstatStats(t *testing.T) {
	checker := NewNetworkChecker(diagnostics.NetworkThresholds{}, nil)

	t.Run("parses netstat output", func(t *testing.T) {
		// netstat -i format with all fields (need at least 10 fields per line)
		output := `Name  Mtu   Network       Address            Ipkts Ierrs    Opkts Oerrs  Coll  Drop
lo0   16384 <Link#1>      127.0.0.1         10000     0     9999     0     0     0
en0   1500  <Link#4>      aa:bb:cc:dd:ee:ff 50000     5    45000     2     0     0
en1   1500  <Link#5>      11:22:33:44:55:66 30000     1    28000     0     0     0`

		stats, err := checker.parseNetstatStats(output)

		if err != nil {
			t.Errorf("parseNetstatStats() unexpected error: %v", err)
		}

		// Should sum all interfaces (fields[4] = Ipkts, fields[5] = Ierrs)
		expectedRxPackets := uint64(10000 + 50000 + 30000)
		if stats.RxPackets != expectedRxPackets {
			t.Errorf("RxPackets = %d, want %d", stats.RxPackets, expectedRxPackets)
		}

		expectedRxErrors := uint64(0 + 5 + 1)
		if stats.RxErrors != expectedRxErrors {
			t.Errorf("RxErrors = %d, want %d", stats.RxErrors, expectedRxErrors)
		}
	})

	t.Run("handles empty output", func(t *testing.T) {
		stats, err := checker.parseNetstatStats("")

		if err != nil {
			t.Errorf("parseNetstatStats() unexpected error on empty output: %v", err)
		}

		// Should return zero stats
		if stats.RxPackets != 0 || stats.RxErrors != 0 {
			t.Errorf("parseNetstatStats() should return zero stats for empty output, got: %+v", stats)
		}
	})

	t.Run("skips short lines", func(t *testing.T) {
		output := `Name  Mtu   Network
en0   1500  <Link#4>`

		stats, err := checker.parseNetstatStats(output)

		if err != nil {
			t.Errorf("parseNetstatStats() unexpected error: %v", err)
		}

		// Should skip lines with < 10 fields
		if stats.RxPackets != 0 {
			t.Errorf("parseNetstatStats() should skip short lines, got RxPackets=%d", stats.RxPackets)
		}
	})

	t.Run("handles non-numeric fields", func(t *testing.T) {
		output := `Name  Mtu   Network       Address            Ipkts Ierrs    Opkts Oerrs  Coll
en0   1500  <Link#4>      aa:bb:cc:dd:ee:ff INVALID ERROR 45000     2     0`

		stats, err := checker.parseNetstatStats(output)

		// Should not error, just skip non-numeric fields
		if err != nil {
			t.Errorf("parseNetstatStats() unexpected error: %v", err)
		}

		// Should have zero stats because fields can't be parsed as numbers
		if stats.RxPackets != 0 || stats.RxErrors != 0 {
			t.Errorf("parseNetstatStats() should skip non-numeric fields, got: %+v", stats)
		}
	})
}
