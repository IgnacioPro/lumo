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

func TestServiceChecker_GetSysvinitServices(t *testing.T) {
	checker := NewServiceChecker(nil)
	ctx := context.Background()

	t.Run("lists and checks sysvinit services", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"ls /etc/init.d/": {
					stdout:   "nginx\nmysql\nREADME\nskeleton\n.hidden\nnetworking",
					exitCode: 0,
				},
				"service 'nginx' status": {
					stdout:   "nginx is running",
					exitCode: 0,
				},
				"service 'mysql' status": {
					stdout:   "mysql is not running",
					exitCode: 3,
				},
				"service 'networking' status": {
					stdout:   "networking service is stopped",
					exitCode: 1,
				},
			},
		}

		services, err := checker.getSysvinitServices(ctx, executor)

		if err != nil {
			t.Fatalf("getSysvinitServices() unexpected error: %v", err)
		}

		// Should skip README, skeleton, and .hidden
		if len(services) != 3 {
			t.Errorf("getSysvinitServices() found %d services, want 3 (nginx, mysql, networking)", len(services))
		}

		// Verify nginx is marked as running (exit code 0)
		nginxFound := false
		for _, svc := range services {
			if svc.Name == "nginx" {
				nginxFound = true
				if svc.State != "running" {
					t.Errorf("nginx state = %q, want running", svc.State)
				}
			}
		}
		if !nginxFound {
			t.Error("nginx service not found in results")
		}
	})

	t.Run("handles service status check with 'running' text", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"ls /etc/init.d/": {
					stdout:   "apache2",
					exitCode: 0,
				},
				"service 'apache2' status": {
					stdout:   "Apache is currently running",
					exitCode: 1, // Non-zero exit but stdout says running
				},
			},
		}

		services, err := checker.getSysvinitServices(ctx, executor)

		if err != nil {
			t.Fatalf("getSysvinitServices() unexpected error: %v", err)
		}

		if len(services) != 1 {
			t.Fatalf("getSysvinitServices() found %d services, want 1", len(services))
		}

		// Should detect "running" in stdout even with non-zero exit
		if services[0].State != "running" {
			t.Errorf("apache2 state = %q, want running (based on stdout text)", services[0].State)
		}
	})

	t.Run("handles service status check with 'stopped' text", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"ls /etc/init.d/": {
					stdout:   "postfix",
					exitCode: 0,
				},
				"service 'postfix' status": {
					stdout:   "postfix is stopped",
					exitCode: 3,
				},
			},
		}

		services, err := checker.getSysvinitServices(ctx, executor)

		if err != nil {
			t.Fatalf("getSysvinitServices() unexpected error: %v", err)
		}

		if len(services) != 1 {
			t.Fatalf("getSysvinitServices() found %d services, want 1", len(services))
		}

		if services[0].State != "inactive" {
			t.Errorf("postfix state = %q, want inactive (based on stopped text)", services[0].State)
		}
	})

	t.Run("handles empty service list", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"ls /etc/init.d/": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		services, err := checker.getSysvinitServices(ctx, executor)

		if err != nil {
			t.Fatalf("getSysvinitServices() unexpected error: %v", err)
		}

		if len(services) != 0 {
			t.Errorf("getSysvinitServices() found %d services, want 0", len(services))
		}
	})

	t.Run("returns error when ls fails", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"ls /etc/init.d/": {
					stdout:   "",
					exitCode: 2,
				},
			},
		}

		_, err := checker.getSysvinitServices(ctx, executor)

		if err == nil {
			t.Error("getSysvinitServices() expected error when ls fails, got nil")
		}

		if !strings.Contains(err.Error(), "failed to list init.d services") {
			t.Errorf("error message = %q, want to contain 'failed to list init.d services'", err.Error())
		}
	})

	t.Run("skips common non-service files", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"ls /etc/init.d/": {
					stdout:   "README\nskeleton\n.dotfile\n..parent\nnginx",
					exitCode: 0,
				},
				"service 'nginx' status": {
					stdout:   "running",
					exitCode: 0,
				},
			},
		}

		services, err := checker.getSysvinitServices(ctx, executor)

		if err != nil {
			t.Fatalf("getSysvinitServices() unexpected error: %v", err)
		}

		// Should only include nginx, skip README, skeleton, and dotfiles
		if len(services) != 1 {
			t.Errorf("getSysvinitServices() found %d services, want 1 (only nginx)", len(services))
		}

		if services[0].Name != "nginx" {
			t.Errorf("service name = %q, want nginx", services[0].Name)
		}
	})

	t.Run("sets LoadState to loaded for all services", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"ls /etc/init.d/": {
					stdout:   "nginx",
					exitCode: 0,
				},
				"service 'nginx' status": {
					stdout:   "",
					exitCode: 0,
				},
			},
		}

		services, err := checker.getSysvinitServices(ctx, executor)

		if err != nil {
			t.Fatalf("getSysvinitServices() unexpected error: %v", err)
		}

		if services[0].LoadState != "loaded" {
			t.Errorf("LoadState = %q, want loaded", services[0].LoadState)
		}
	})
}

func TestServiceChecker_GetLaunchdServices(t *testing.T) {
	checker := NewServiceChecker(nil)
	ctx := context.Background()

	t.Run("calls launchctl and parses output", func(t *testing.T) {
		launchctlOutput := `PID	Status	Label
123	0	com.apple.nginx
-	0	com.apple.mysql
456	0	com.apple.apache`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"launchctl list": {
					stdout:   launchctlOutput,
					exitCode: 0,
				},
			},
		}

		services, err := checker.getLaunchdServices(ctx, executor)

		if err != nil {
			t.Fatalf("getLaunchdServices() unexpected error: %v", err)
		}

		// Should return 3 services (skipping header)
		if len(services) != 3 {
			t.Errorf("getLaunchdServices() found %d services, want 3", len(services))
		}

		// Verify first service has PID (running)
		if services[0].Name != "com.apple.nginx" {
			t.Errorf("services[0].Name = %q, want com.apple.nginx", services[0].Name)
		}
		if services[0].State != "running" {
			t.Errorf("services[0].State = %q, want running (has PID)", services[0].State)
		}

		// Verify second service has no PID (inactive)
		if services[1].Name != "com.apple.mysql" {
			t.Errorf("services[1].Name = %q, want com.apple.mysql", services[1].Name)
		}
		if services[1].State != "inactive" {
			t.Errorf("services[1].State = %q, want inactive (PID is -)", services[1].State)
		}
	})

	t.Run("returns error when launchctl fails", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"launchctl list": {
					stdout:   "",
					exitCode: 1,
				},
			},
		}

		_, err := checker.getLaunchdServices(ctx, executor)

		if err == nil {
			t.Error("getLaunchdServices() expected error when launchctl fails, got nil")
		}

		if !strings.Contains(err.Error(), "failed to list launchd services") {
			t.Errorf("error message = %q, want to contain 'failed to list launchd services'", err.Error())
		}
	})

	t.Run("handles empty launchctl output", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"launchctl list": {
					stdout:   "PID	Status	Label\n",
					exitCode: 0,
				},
			},
		}

		services, err := checker.getLaunchdServices(ctx, executor)

		if err != nil {
			t.Fatalf("getLaunchdServices() unexpected error: %v", err)
		}

		// Only header line, no services
		if len(services) != 0 {
			t.Errorf("getLaunchdServices() found %d services, want 0", len(services))
		}
	})
}

func TestServiceChecker_ParseLaunchdOutput(t *testing.T) {
	checker := NewServiceChecker(nil)

	t.Run("parses launchctl output correctly", func(t *testing.T) {
		output := `PID	Status	Label
123	0	com.apple.nginx
-	0	com.apple.mysql
456	0	com.apple.apache
789	1	com.apple.postgres`

		services, err := checker.parseLaunchdOutput(output)

		if err != nil {
			t.Fatalf("parseLaunchdOutput() unexpected error: %v", err)
		}

		if len(services) != 4 {
			t.Errorf("parseLaunchdOutput() found %d services, want 4", len(services))
		}

		// Test first service with PID
		if services[0].Name != "com.apple.nginx" {
			t.Errorf("services[0].Name = %q, want com.apple.nginx", services[0].Name)
		}
		if services[0].State != "running" {
			t.Errorf("services[0].State = %q, want running", services[0].State)
		}
		if services[0].LoadState != "loaded" {
			t.Errorf("services[0].LoadState = %q, want loaded", services[0].LoadState)
		}

		// Test second service without PID (-)
		if services[1].Name != "com.apple.mysql" {
			t.Errorf("services[1].Name = %q, want com.apple.mysql", services[1].Name)
		}
		if services[1].State != "inactive" {
			t.Errorf("services[1].State = %q, want inactive", services[1].State)
		}
	})

	t.Run("skips header line", func(t *testing.T) {
		output := `PID	Status	Label`

		services, err := checker.parseLaunchdOutput(output)

		if err != nil {
			t.Fatalf("parseLaunchdOutput() unexpected error: %v", err)
		}

		// Header only, no services
		if len(services) != 0 {
			t.Errorf("parseLaunchdOutput() found %d services, want 0 (header only)", len(services))
		}
	})

	t.Run("handles empty output", func(t *testing.T) {
		output := ``

		services, err := checker.parseLaunchdOutput(output)

		if err != nil {
			t.Fatalf("parseLaunchdOutput() unexpected error: %v", err)
		}

		if len(services) != 0 {
			t.Errorf("parseLaunchdOutput() found %d services, want 0", len(services))
		}
	})

	t.Run("skips lines with insufficient fields", func(t *testing.T) {
		output := `PID	Status	Label
123	0	com.apple.nginx
invalid-line
456
-	0	com.apple.mysql`

		services, err := checker.parseLaunchdOutput(output)

		if err != nil {
			t.Fatalf("parseLaunchdOutput() unexpected error: %v", err)
		}

		// Should only include valid lines (nginx and mysql)
		if len(services) != 2 {
			t.Errorf("parseLaunchdOutput() found %d services, want 2 (skipping invalid lines)", len(services))
		}
	})

	t.Run("handles PID variations", func(t *testing.T) {
		output := `PID	Status	Label
123	0	service.with.pid
-	0	service.without.pid
0	0	service.with.zero.pid`

		services, err := checker.parseLaunchdOutput(output)

		if err != nil {
			t.Fatalf("parseLaunchdOutput() unexpected error: %v", err)
		}

		if len(services) != 3 {
			t.Fatalf("parseLaunchdOutput() found %d services, want 3", len(services))
		}

		// PID 123 should be running
		if services[0].State != "running" {
			t.Errorf("services[0].State = %q, want running (PID=123)", services[0].State)
		}

		// PID - should be inactive
		if services[1].State != "inactive" {
			t.Errorf("services[1].State = %q, want inactive (PID=-)", services[1].State)
		}

		// PID 0 is treated as running (implementation only checks != "-")
		if services[2].State != "running" {
			t.Errorf("services[2].State = %q, want running (PID=0, treated as running)", services[2].State)
		}
	})

	t.Run("preserves full service label names", func(t *testing.T) {
		output := `PID	Status	Label
123	0	com.apple.very.long.service.name.with.multiple.dots`

		services, err := checker.parseLaunchdOutput(output)

		if err != nil {
			t.Fatalf("parseLaunchdOutput() unexpected error: %v", err)
		}

		expectedName := "com.apple.very.long.service.name.with.multiple.dots"
		if services[0].Name != expectedName {
			t.Errorf("services[0].Name = %q, want %q", services[0].Name, expectedName)
		}
	})

	t.Run("handles whitespace variations", func(t *testing.T) {
		output := `PID	Status	Label
  123  	  0  	  com.apple.nginx
    -    	    0    	    com.apple.mysql    `

		services, err := checker.parseLaunchdOutput(output)

		if err != nil {
			t.Fatalf("parseLaunchdOutput() unexpected error: %v", err)
		}

		if len(services) != 2 {
			t.Errorf("parseLaunchdOutput() found %d services, want 2", len(services))
		}

		// Verify names are trimmed correctly
		if services[0].Name != "com.apple.nginx" {
			t.Errorf("services[0].Name = %q, want com.apple.nginx (whitespace trimmed)", services[0].Name)
		}
	})
}
