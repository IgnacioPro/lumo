package checkers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestNewPortsChecker(t *testing.T) {
	t.Run("with whitelisted ports", func(t *testing.T) {
		whitelisted := []int{22, 80, 443}
		checker := NewPortsChecker(whitelisted)

		if checker == nil {
			t.Fatal("NewPortsChecker() returned nil")
		}

		// Verify whitelisted ports are in the map
		for _, port := range whitelisted {
			if !checker.whitelistedPorts[port] {
				t.Errorf("Port %d should be whitelisted", port)
			}
		}

		// Verify non-whitelisted port is not in the map
		if checker.whitelistedPorts[8080] {
			t.Error("Port 8080 should not be whitelisted")
		}
	})

	t.Run("with empty whitelist", func(t *testing.T) {
		checker := NewPortsChecker([]int{})

		if checker == nil {
			t.Fatal("NewPortsChecker() returned nil")
		}

		if len(checker.whitelistedPorts) != 0 {
			t.Errorf("Empty whitelist should result in empty map, got %d entries", len(checker.whitelistedPorts))
		}
	})

	t.Run("with nil whitelist", func(t *testing.T) {
		checker := NewPortsChecker(nil)

		if checker == nil {
			t.Fatal("NewPortsChecker() returned nil")
		}

		if len(checker.whitelistedPorts) != 0 {
			t.Errorf("Nil whitelist should result in empty map, got %d entries", len(checker.whitelistedPorts))
		}
	})
}

func TestPortsChecker_Getters(t *testing.T) {
	checker := NewPortsChecker([]int{22})

	t.Run("Name", func(t *testing.T) {
		if name := checker.Name(); name != "open_ports" {
			t.Errorf("Name() = %q, want 'open_ports'", name)
		}
	})

	t.Run("Category", func(t *testing.T) {
		if cat := checker.Category(); cat != diagnostics.CategorySecurity {
			t.Errorf("Category() = %v, want %v", cat, diagnostics.CategorySecurity)
		}
	})

	t.Run("Description", func(t *testing.T) {
		desc := checker.Description()
		if desc == "" {
			t.Error("Description() should return non-empty string")
		}
		if !strings.Contains(strings.ToLower(desc), "port") {
			t.Errorf("Description() should mention 'port', got %q", desc)
		}
	})

	t.Run("RequiresRoot", func(t *testing.T) {
		if checker.RequiresRoot() {
			t.Error("RequiresRoot() = true, want false")
		}
	})
}

func TestPortsChecker_ExtractPort(t *testing.T) {
	checker := NewPortsChecker(nil)

	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"IPv4 standard", "0.0.0.0:22", 22, false},
		{"IPv4 high port", "192.168.1.1:8080", 8080, false},
		{"IPv6 standard", "[::]:443", 443, false},
		{"IPv6 localhost", "[::1]:3000", 3000, false},
		{"with wildcard", "0.0.0.0:80*", 80, false},
		{"localhost", "127.0.0.1:5432", 5432, false},
		{"no port", "0.0.0.0", 0, true},
		{"invalid port", "0.0.0.0:abc", 0, true},
		{"empty string", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checker.extractPort(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("extractPort(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("extractPort(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got != tt.want {
				t.Errorf("extractPort(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestPortsChecker_ExtractAddress(t *testing.T) {
	checker := NewPortsChecker(nil)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"IPv4 any", "0.0.0.0:22", "0.0.0.0"},
		{"IPv4 localhost", "127.0.0.1:80", "127.0.0.1"},
		{"IPv4 specific", "192.168.1.1:443", "192.168.1.1"},
		{"IPv6 any", "[::]:22", "::"},
		{"IPv6 localhost", "[::1]:80", "::1"},
		{"IPv6 full", "[2001:db8::1]:443", "2001:db8::1"},
		{"no port", "0.0.0.0", "0.0.0.0"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checker.extractAddress(tt.input)
			if got != tt.want {
				t.Errorf("extractAddress(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPortsChecker_ExtractPID(t *testing.T) {
	checker := NewPortsChecker(nil)

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"ss format", `users:(("sshd",pid=1234,fd=3))`, 1234},
		{"netstat format", "1234/sshd", 1234},
		{"ss with multiple", `users:(("nginx",pid=5678,fd=5))`, 5678},
		{"empty", "", 0},
		{"no pid", "sshd", 0},
		{"invalid pid", "abc/sshd", 0},
		{"dash", "-", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checker.extractPID(tt.input)
			if got != tt.want {
				t.Errorf("extractPID(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestPortsChecker_ExtractProcessName(t *testing.T) {
	checker := NewPortsChecker(nil)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ss format", `users:(("sshd",pid=1234,fd=3))`, "sshd"},
		{"netstat format", "1234/sshd", "sshd"},
		{"ss nginx", `users:(("nginx",pid=5678,fd=5))`, "nginx"},
		{"netstat apache", "9999/apache2", "apache2"},
		{"empty", "", "unknown"},
		{"dash", "-", "unknown"},
		{"no process", "1234", "1234"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checker.extractProcessName(tt.input)
			if got != tt.want {
				t.Errorf("extractProcessName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPortsChecker_IsPublicAddress(t *testing.T) {
	checker := NewPortsChecker(nil)

	tests := []struct {
		addr     string
		isPublic bool
	}{
		{"0.0.0.0", true},
		{"::", true},
		{"*", true},
		{"127.0.0.1", false},
		{"::1", false},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"localhost", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got := checker.isPublicAddress(tt.addr)
			if got != tt.isPublic {
				t.Errorf("isPublicAddress(%q) = %v, want %v", tt.addr, got, tt.isPublic)
			}
		})
	}
}

func TestPortsChecker_IsLocalhostAddress(t *testing.T) {
	checker := NewPortsChecker(nil)

	tests := []struct {
		addr        string
		isLocalhost bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"localhost", true},
		{"0.0.0.0", false},
		{"::", false},
		{"*", false},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got := checker.isLocalhostAddress(tt.addr)
			if got != tt.isLocalhost {
				t.Errorf("isLocalhostAddress(%q) = %v, want %v", tt.addr, got, tt.isLocalhost)
			}
		})
	}
}

func TestPortsChecker_FormatMessage(t *testing.T) {
	checker := NewPortsChecker(nil)

	tests := []struct {
		name            string
		totalPorts      int
		publicPorts     int
		unexpectedPorts int
		wantContains    []string
	}{
		{
			name:            "no warnings",
			totalPorts:      3,
			publicPorts:     0,
			unexpectedPorts: 0,
			wantContains:    []string{"3 listening ports"},
		},
		{
			name:            "with public ports",
			totalPorts:      5,
			publicPorts:     2,
			unexpectedPorts: 0,
			wantContains:    []string{"5 listening ports", "2 public"},
		},
		{
			name:            "with unexpected ports",
			totalPorts:      4,
			publicPorts:     0,
			unexpectedPorts: 1,
			wantContains:    []string{"4 listening ports", "1 unexpected"},
		},
		{
			name:            "with both warnings",
			totalPorts:      10,
			publicPorts:     3,
			unexpectedPorts: 2,
			wantContains:    []string{"10 listening ports", "3 public", "2 unexpected"},
		},
		{
			name:            "zero ports",
			totalPorts:      0,
			publicPorts:     0,
			unexpectedPorts: 0,
			wantContains:    []string{"0 listening ports"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checker.formatMessage(tt.totalPorts, tt.publicPorts, tt.unexpectedPorts)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatMessage() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}

func TestPortsChecker_ParseSSOutput(t *testing.T) {
	checker := NewPortsChecker(nil)

	ssOutput := `State      Recv-Q Send-Q Local Address:Port  Peer Address:Port Process
LISTEN     0      128    0.0.0.0:22           0.0.0.0:*     users:(("sshd",pid=1234,fd=3))
LISTEN     0      128    127.0.0.1:631        0.0.0.0:*     users:(("cupsd",pid=5678,fd=7))
LISTEN     0      100    [::]:25              [::]:*        users:(("master",pid=9999,fd=13))`

	ports, err := checker.parseSSOutput(ssOutput)

	if err != nil {
		t.Fatalf("parseSSOutput() unexpected error: %v", err)
	}

	if len(ports) != 3 {
		t.Fatalf("parseSSOutput() returned %d ports, want 3", len(ports))
	}

	// Check first port (SSH)
	if ports[0].Port != 22 {
		t.Errorf("Port 0: Port = %d, want 22", ports[0].Port)
	}
	if ports[0].Address != "0.0.0.0" {
		t.Errorf("Port 0: Address = %q, want '0.0.0.0'", ports[0].Address)
	}
	if !ports[0].IsPublic {
		t.Error("Port 0: IsPublic should be true for 0.0.0.0")
	}
	if ports[0].Process != "sshd" {
		t.Errorf("Port 0: Process = %q, want 'sshd'", ports[0].Process)
	}
	if ports[0].PID != 1234 {
		t.Errorf("Port 0: PID = %d, want 1234", ports[0].PID)
	}

	// Check second port (CUPS - localhost)
	if ports[1].Port != 631 {
		t.Errorf("Port 1: Port = %d, want 631", ports[1].Port)
	}
	if ports[1].Address != "127.0.0.1" {
		t.Errorf("Port 1: Address = %q, want '127.0.0.1'", ports[1].Address)
	}
	if !ports[1].IsLocalhost {
		t.Error("Port 1: IsLocalhost should be true for 127.0.0.1")
	}
	if ports[1].Process != "cupsd" {
		t.Errorf("Port 1: Process = %q, want 'cupsd'", ports[1].Process)
	}

	// Check third port (SMTP - IPv6)
	if ports[2].Port != 25 {
		t.Errorf("Port 2: Port = %d, want 25", ports[2].Port)
	}
	if ports[2].Address != "::" {
		t.Errorf("Port 2: Address = %q, want '::'", ports[2].Address)
	}
	if !ports[2].IsPublic {
		t.Error("Port 2: IsPublic should be true for ::")
	}
	if ports[2].Process != "master" {
		t.Errorf("Port 2: Process = %q, want 'master'", ports[2].Process)
	}
}

func TestPortsChecker_ParseNetstatOutput(t *testing.T) {
	checker := NewPortsChecker(nil)

	netstatOutput := `Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
tcp        0      0 0.0.0.0:22              0.0.0.0:*               LISTEN      1234/sshd
tcp        0      0 127.0.0.1:631           0.0.0.0:*               LISTEN      5678/cupsd
tcp6       0      0 :::80                   :::*                    LISTEN      9999/nginx`

	ports, err := checker.parseNetstatOutput(netstatOutput)

	if err != nil {
		t.Fatalf("parseNetstatOutput() unexpected error: %v", err)
	}

	if len(ports) != 3 {
		t.Fatalf("parseNetstatOutput() returned %d ports, want 3", len(ports))
	}

	// Check first port (SSH)
	if ports[0].Port != 22 {
		t.Errorf("Port 0: Port = %d, want 22", ports[0].Port)
	}
	if ports[0].Address != "0.0.0.0" {
		t.Errorf("Port 0: Address = %q, want '0.0.0.0'", ports[0].Address)
	}
	if ports[0].Protocol != "tcp" {
		t.Errorf("Port 0: Protocol = %q, want 'tcp'", ports[0].Protocol)
	}
	if ports[0].Process != "sshd" {
		t.Errorf("Port 0: Process = %q, want 'sshd'", ports[0].Process)
	}
	if ports[0].PID != 1234 {
		t.Errorf("Port 0: PID = %d, want 1234", ports[0].PID)
	}

	// Check second port (CUPS - localhost)
	if ports[1].Port != 631 {
		t.Errorf("Port 1: Port = %d, want 631", ports[1].Port)
	}
	if !ports[1].IsLocalhost {
		t.Error("Port 1: IsLocalhost should be true")
	}

	// Check third port (nginx - IPv6)
	if ports[2].Port != 80 {
		t.Errorf("Port 2: Port = %d, want 80", ports[2].Port)
	}
	if ports[2].Protocol != "tcp6" {
		t.Errorf("Port 2: Protocol = %q, want 'tcp6'", ports[2].Protocol)
	}
}

func TestPortsChecker_Run(t *testing.T) {
	t.Run("with ss output", func(t *testing.T) {
		whitelisted := []int{22, 80}
		checker := NewPortsChecker(whitelisted)

		ssOutput := `State      Recv-Q Send-Q Local Address:Port  Peer Address:Port Process
LISTEN     0      128    0.0.0.0:22           0.0.0.0:*     users:(("sshd",pid=1234,fd=3))
LISTEN     0      128    0.0.0.0:80           0.0.0.0:*     users:(("nginx",pid=5678,fd=7))
LISTEN     0      100    127.0.0.1:8080       0.0.0.0:*     users:(("app",pid=9999,fd=13))
LISTEN     0      50     0.0.0.0:12345        0.0.0.0:*     users:(("unknown",pid=1111,fd=5))`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"ss -tlnp": {stdout: ssOutput, exitCode: 0},
			},
		}

		ctx := context.Background()
		result, err := checker.Run(ctx, executor)

		if err != nil {
			t.Fatalf("Run() unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("Run() returned nil result")
		}

		// Check basic fields
		if result.Name != "open_ports" {
			t.Errorf("Name = %q, want 'open_ports'", result.Name)
		}

		if result.Status != diagnostics.StatusCompleted {
			t.Errorf("Status = %v, want %v", result.Status, diagnostics.StatusCompleted)
		}

		// Check data
		totalPorts, _ := result.GetDataValue("total_listening_ports")
		if totalPorts != 4 {
			t.Errorf("total_listening_ports = %v, want 4", totalPorts)
		}

		publicPorts, _ := result.GetDataValue("public_ports")
		if publicPorts != 3 {
			t.Errorf("public_ports = %v, want 3 (ssh, nginx, unknown)", publicPorts)
		}

		localhostPorts, _ := result.GetDataValue("localhost_ports")
		if localhostPorts != 1 {
			t.Errorf("localhost_ports = %v, want 1 (app on 127.0.0.1)", localhostPorts)
		}

		unexpectedPorts, _ := result.GetDataValue("unexpected_ports")
		if unexpectedPorts != 2 {
			t.Errorf("unexpected_ports = %v, want 2 (8080 and 12345 not whitelisted)", unexpectedPorts)
		}

		highNumberedPorts, _ := result.GetDataValue("high_numbered_ports")
		if highNumberedPorts != 1 {
			t.Errorf("high_numbered_ports = %v, want 1 (port 12345)", highNumberedPorts)
		}

		// Check metrics
		if len(result.Metrics) != 2 {
			t.Errorf("Metrics count = %d, want 2", len(result.Metrics))
		}
	})

	t.Run("fallback to netstat", func(t *testing.T) {
		checker := NewPortsChecker([]int{})

		netstatOutput := `Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
tcp        0      0 0.0.0.0:22              0.0.0.0:*               LISTEN      1234/sshd`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"ss -tlnp":      {stdout: "", exitCode: 1}, // ss fails
				"netstat -tlnp": {stdout: netstatOutput, exitCode: 0},
			},
		}

		ctx := context.Background()
		result, err := checker.Run(ctx, executor)

		if err != nil {
			t.Fatalf("Run() unexpected error: %v", err)
		}

		totalPorts, _ := result.GetDataValue("total_listening_ports")
		if totalPorts != 1 {
			t.Errorf("total_listening_ports = %v, want 1", totalPorts)
		}
	})

	t.Run("execution error", func(t *testing.T) {
		checker := NewPortsChecker([]int{})

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"ss -tlnp":      {stdout: "", exitCode: 1},
				"netstat -tlnp": {stdout: "", exitCode: 1},
				"netstat -tln":  {stdout: "", exitCode: 1},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := checker.Run(ctx, executor)

		if err == nil {
			t.Error("Run() expected error when all commands fail")
		}

		if result != nil {
			t.Errorf("Run() should return nil result on error, got %+v", result)
		}
	})
}

func TestPortsChecker_ParseSSOutput_EdgeCases(t *testing.T) {
	checker := NewPortsChecker(nil)

	t.Run("empty output", func(t *testing.T) {
		ports, err := checker.parseSSOutput("")
		if err != nil {
			t.Errorf("parseSSOutput('') unexpected error: %v", err)
		}
		if len(ports) != 0 {
			t.Errorf("parseSSOutput('') = %d ports, want 0", len(ports))
		}
	})

	t.Run("header only", func(t *testing.T) {
		output := "State      Recv-Q Send-Q Local Address:Port  Peer Address:Port Process"
		ports, err := checker.parseSSOutput(output)
		if err != nil {
			t.Errorf("parseSSOutput(header only) unexpected error: %v", err)
		}
		if len(ports) != 0 {
			t.Errorf("parseSSOutput(header only) = %d ports, want 0", len(ports))
		}
	})

	t.Run("malformed line", func(t *testing.T) {
		output := "LISTEN 0 128" // Too few fields
		ports, err := checker.parseSSOutput(output)
		if err != nil {
			t.Errorf("parseSSOutput(malformed) unexpected error: %v", err)
		}
		if len(ports) != 0 {
			t.Errorf("parseSSOutput(malformed) = %d ports, want 0 (should skip)", len(ports))
		}
	})
}

func TestPortsChecker_ParseNetstatOutput_EdgeCases(t *testing.T) {
	checker := NewPortsChecker(nil)

	t.Run("empty output", func(t *testing.T) {
		ports, err := checker.parseNetstatOutput("")
		if err != nil {
			t.Errorf("parseNetstatOutput('') unexpected error: %v", err)
		}
		if len(ports) != 0 {
			t.Errorf("parseNetstatOutput('') = %d ports, want 0", len(ports))
		}
	})

	t.Run("non-LISTEN state", func(t *testing.T) {
		output := "tcp        0      0 192.168.1.1:22          192.168.1.2:54321       ESTABLISHED"
		ports, err := checker.parseNetstatOutput(output)
		if err != nil {
			t.Errorf("parseNetstatOutput(ESTABLISHED) unexpected error: %v", err)
		}
		if len(ports) != 0 {
			t.Errorf("parseNetstatOutput(ESTABLISHED) = %d ports, want 0 (should skip)", len(ports))
		}
	})

	t.Run("macOS format without process info", func(t *testing.T) {
		// macOS netstat format: Proto Recv-Q Send-Q  Local-Address  Foreign-Address  (state)
		// Note: State field position varies, and port format is different
		output := "tcp4       0      0  127.0.0.1.80           *.*                    LISTEN"
		ports, err := checker.parseNetstatOutput(output)
		if err != nil {
			t.Errorf("parseNetstatOutput(macOS) unexpected error: %v", err)
		}
		// macOS format uses dot separator (127.0.0.1.80) which extractPort may not handle
		// This test documents the limitation - may return 0 ports
		if len(ports) > 0 {
			// If it does parse, verify process info is unknown
			if ports[0].Process != "unknown" {
				t.Errorf("Process = %q, want 'unknown'", ports[0].Process)
			}
			if ports[0].PID != 0 {
				t.Errorf("PID = %d, want 0", ports[0].PID)
			}
		}
	})
}
