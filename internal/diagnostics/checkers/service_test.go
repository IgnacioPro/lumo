package checkers

import (
	"context"
	"strings"
	"testing"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestNewServiceChecker(t *testing.T) {
	services := []string{"nginx", "mysql"}
	checker := NewServiceChecker(services)

	if checker == nil {
		t.Fatal("NewServiceChecker returned nil")
	}

	if len(checker.monitoredServices) != 2 {
		t.Errorf("Expected 2 monitored services, got %d", len(checker.monitoredServices))
	}
}

func TestServiceChecker_Name(t *testing.T) {
	checker := NewServiceChecker(nil)
	if got := checker.Name(); got != "service_check" {
		t.Errorf("Name() = %v, want service_check", got)
	}
}

func TestServiceChecker_Category(t *testing.T) {
	checker := NewServiceChecker(nil)
	if got := checker.Category(); got != diagnostics.CategoryService {
		t.Errorf("Category() = %v, want %v", got, diagnostics.CategoryService)
	}
}

func TestServiceChecker_Description(t *testing.T) {
	checker := NewServiceChecker(nil)
	desc := checker.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if !strings.Contains(strings.ToLower(desc), "service") {
		t.Error("Description should mention service")
	}
}

func TestServiceChecker_RequiresRoot(t *testing.T) {
	checker := NewServiceChecker(nil)
	if checker.RequiresRoot() {
		t.Error("ServiceChecker should not require root")
	}
}

func TestServiceChecker_Run_Systemd(t *testing.T) {
	executor := &mockExecutor{
		responses: map[string]mockResponse{
			"command -v systemctl": {
				stdout:   "/usr/bin/systemctl\n",
				exitCode: 0,
			},
			"systemctl is-system-running": {
				stdout:   "running\n",
				exitCode: 0,
			},
			"systemctl list-units": {
				stdout: `UNIT                  LOAD   ACTIVE SUB     DESCRIPTION
nginx.service        loaded active running A high performance web server
mysql.service        loaded failed failed  MySQL Database Server
ssh.service          loaded active running OpenBSD Secure Shell server
`,
				exitCode: 0,
			},
		},
	}

	checker := NewServiceChecker([]string{"nginx", "mysql"})
	result, err := checker.Run(context.Background(), executor)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	if result.Name != "service_check" {
		t.Errorf("Name = %v, want service_check", result.Name)
	}

	if result.Category != diagnostics.CategoryService {
		t.Errorf("Category = %v, want %v", result.Category, diagnostics.CategoryService)
	}
}

func TestServiceChecker_Run_NoServiceManager(t *testing.T) {
	executor := &mockExecutor{
		responses: map[string]mockResponse{
			"command -v systemctl": {
				stderr:   "not found",
				exitCode: 1,
			},
			"command -v service": {
				stderr:   "not found",
				exitCode: 1,
			},
			"command -v launchctl": {
				stderr:   "not found",
				exitCode: 1,
			},
		},
	}

	checker := NewServiceChecker(nil)
	result, err := checker.Run(context.Background(), executor)

	// Should return error when no service manager is found
	if err == nil {
		t.Fatal("Run() should return error when no service manager found")
	}

	// Result should be nil when error is returned
	if result != nil {
		t.Error("Run() should return nil result when error occurs")
	}
}

func TestServiceChecker_Run_WithMonitoredServices(t *testing.T) {
	executor := &mockExecutor{
		responses: map[string]mockResponse{
			"command -v systemctl": {
				stdout:   "/usr/bin/systemctl\n",
				exitCode: 0,
			},
			"systemctl is-system-running": {
				stdout:   "running\n",
				exitCode: 0,
			},
			"systemctl list-units": {
				stdout: `UNIT                  LOAD   ACTIVE SUB     DESCRIPTION
nginx.service        loaded active running Nginx
mysql.service        loaded active running MySQL
redis.service        loaded failed failed  Redis
postgres.service     loaded active running PostgreSQL
`,
				exitCode: 0,
			},
		},
	}

	// Only monitor nginx and mysql
	checker := NewServiceChecker([]string{"nginx", "mysql"})
	result, err := checker.Run(context.Background(), executor)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// Result should have data about monitored services
	if result.Data == nil {
		t.Error("Result.Data is nil")
	}
}

func TestServiceChecker_FilterServices(t *testing.T) {
	services := []ServiceInfo{
		{Name: "nginx", State: "running"},
		{Name: "mysql", State: "running"},
		{Name: "redis", State: "failed"},
		{Name: "postgres", State: "inactive"},
	}

	checker := NewServiceChecker(nil)

	tests := []struct {
		name      string
		filter    []string
		wantCount int
	}{
		{
			name:      "filter specific services",
			filter:    []string{"nginx", "mysql"},
			wantCount: 2,
		},
		{
			name:      "filter single service",
			filter:    []string{"redis"},
			wantCount: 1,
		},
		{
			name:      "filter non-existent service",
			filter:    []string{"nonexistent"},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := checker.filterServices(services, tt.filter)
			if len(filtered) != tt.wantCount {
				t.Errorf("filterServices() returned %d services, want %d", len(filtered), tt.wantCount)
			}
		})
	}
}
