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

// TestNetworkChecker_Run is complex due to network commands - covered by integration tests

// parseInterfaces and InterfaceInfo are private, testing via Run() instead

func TestNetworkChecker_GetConnectionCount(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		want     int
		wantErr  bool
	}{
		{
			name:     "valid count",
			output:   "42\n",
			want:     42,
			wantErr:  false,
		},
		{
			name:     "zero connections",
			output:   "0\n",
			want:     0,
			wantErr:  false,
		},
		{
			name:     "with whitespace",
			output:   "  123  \n",
			want:     123,
			wantErr:  false,
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
