package checkers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// ServiceChecker performs service status checks
type ServiceChecker struct {
	// Services to monitor (if empty, checks all enabled services)
	monitoredServices []string
}

// NewServiceChecker creates a new service checker
func NewServiceChecker(monitoredServices []string) *ServiceChecker {
	return &ServiceChecker{
		monitoredServices: monitoredServices,
	}
}

// Name returns the checker name
func (s *ServiceChecker) Name() string {
	return "service_check"
}

// Category returns the checker category
func (s *ServiceChecker) Category() diagnostics.CheckCategory {
	return diagnostics.CategoryService
}

// Description returns the checker description
func (s *ServiceChecker) Description() string {
	return "Checks status of system services (systemd/init)"
}

// RequiresRoot returns false as service status can usually be checked without root
func (s *ServiceChecker) RequiresRoot() bool {
	return false
}

// Run executes the service check
func (s *ServiceChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
	result := &diagnostics.CheckResult{
		Name:      s.Name(),
		Category:  s.Category(),
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
		Metrics:   []diagnostics.Metric{},
	}

	startTime := time.Now()

	// Detect service manager (systemd vs sysvinit)
	serviceManager, err := s.detectServiceManager(ctx, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to detect service manager: %w", err)
	}

	result.SetData("service_manager", serviceManager)

	// Get service statuses
	var services []ServiceInfo
	switch serviceManager {
	case "systemd":
		services, err = s.getSystemdServices(ctx, executor)
	case "sysvinit":
		services, err = s.getSysvinitServices(ctx, executor)
	case "launchd":
		services, err = s.getLaunchdServices(ctx, executor)
	default:
		return nil, fmt.Errorf("unsupported service manager: %s", serviceManager)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get service statuses: %w", err)
	}

	// Filter services if specific ones are monitored
	if len(s.monitoredServices) > 0 {
		services = s.filterServices(services, s.monitoredServices)
	}

	// Categorize services
	var running, failed, inactive int
	var failedServices []string

	for _, svc := range services {
		switch svc.State {
		case "running", "active":
			running++
		case "failed", "error":
			failed++
			failedServices = append(failedServices, svc.Name)
		default:
			inactive++
		}
	}

	// Store data
	result.SetData("total_services", len(services))
	result.SetData("running_services", running)
	result.SetData("failed_services", failed)
	result.SetData("inactive_services", inactive)
	result.SetData("services", services)

	if len(failedServices) > 0 {
		result.SetData("failed_service_names", failedServices)
	}

	// Add metrics for threshold evaluation
	result.AddMetric(diagnostics.Metric{
		Name:          "failed_services",
		Value:         float64(failed),
		Unit:          "count",
		Threshold:     1.0, // Any failed service is a warning
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.Duration = time.Since(startTime)

	// Generate message
	result.Message = s.formatMessage(len(services), running, failed, inactive, failedServices)

	return result, nil
}

// ServiceInfo holds information about a service
type ServiceInfo struct {
	Name        string `json:"name"`
	State       string `json:"state"`       // running, stopped, failed, etc.
	LoadState   string `json:"load_state"`  // loaded, not-found, etc.
	Description string `json:"description"` // Service description
}

// detectServiceManager detects which service manager is in use
func (s *ServiceChecker) detectServiceManager(ctx context.Context, executor diagnostics.CommandExecutor) (string, error) {
	// Check for systemd
	_, _, exitCode, _ := executor.ExecuteWithContext(ctx, "command -v systemctl >/dev/null 2>&1")
	if exitCode == 0 {
		// Verify systemd is actually running (not just installed)
		_, _, exitCode, _ := executor.ExecuteWithContext(ctx, "systemctl is-system-running >/dev/null 2>&1 || systemctl list-units >/dev/null 2>&1")
		if exitCode == 0 {
			return "systemd", nil
		}
	}

	// Check for launchd (macOS)
	_, _, exitCode, _ = executor.ExecuteWithContext(ctx, "command -v launchctl >/dev/null 2>&1")
	if exitCode == 0 {
		return "launchd", nil
	}

	// Check for sysvinit
	_, _, exitCode, _ = executor.ExecuteWithContext(ctx, "command -v service >/dev/null 2>&1")
	if exitCode == 0 {
		return "sysvinit", nil
	}

	return "unknown", fmt.Errorf("no supported service manager found")
}

// getSystemdServices retrieves systemd service statuses
func (s *ServiceChecker) getSystemdServices(ctx context.Context, executor diagnostics.CommandExecutor) ([]ServiceInfo, error) {
	// Get list of all systemd services
	// Format: UNIT LOAD ACTIVE SUB DESCRIPTION
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx,
		"systemctl list-units --type=service --all --no-pager --no-legend --plain")
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to list systemd services: %w", err)
	}

	return s.parseSystemdOutput(stdout)
}

// parseSystemdOutput parses systemctl list-units output
func (s *ServiceChecker) parseSystemdOutput(output string) ([]ServiceInfo, error) {
	var services []ServiceInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		name := strings.TrimSuffix(fields[0], ".service")
		loadState := fields[1]
		activeState := fields[2]
		// subState := fields[3]
		description := ""
		if len(fields) > 4 {
			description = strings.Join(fields[4:], " ")
		}

		// Map systemd states to our standard states
		var state string
		switch activeState {
		case "active":
			state = "running"
		case "failed":
			state = "failed"
		default:
			state = "inactive"
		}

		services = append(services, ServiceInfo{
			Name:        name,
			State:       state,
			LoadState:   loadState,
			Description: description,
		})
	}

	return services, nil
}

// getSysvinitServices retrieves sysvinit service statuses
func (s *ServiceChecker) getSysvinitServices(ctx context.Context, executor diagnostics.CommandExecutor) ([]ServiceInfo, error) {
	// List services in /etc/init.d
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "ls /etc/init.d/ 2>/dev/null")
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to list init.d services: %w", err)
	}

	serviceNames := strings.Fields(strings.TrimSpace(stdout))
	var services []ServiceInfo

	// Check status of each service
	for _, name := range serviceNames {
		// Skip common non-service files
		if name == "README" || name == "skeleton" || strings.HasPrefix(name, ".") {
			continue
		}

		// Validate service name to prevent command injection from malicious filenames
		if !isValidServiceName(name) {
			// Skip files with potentially dangerous names
			continue
		}

		// Check service status - use shell quoting for defense in depth
		stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx,
			fmt.Sprintf("service %s status 2>/dev/null || /etc/init.d/%s status 2>/dev/null",
				shellQuote(name), shellQuote(name)))

		state := "inactive"
		if exitCode == 0 {
			// Service is running
			state = "running"
		} else if strings.Contains(strings.ToLower(stdout), "running") {
			state = "running"
		} else if strings.Contains(strings.ToLower(stdout), "stopped") {
			state = "inactive"
		}

		services = append(services, ServiceInfo{
			Name:        name,
			State:       state,
			LoadState:   "loaded",
			Description: "",
		})
	}

	return services, nil
}

// getLaunchdServices retrieves launchd service statuses (macOS)
func (s *ServiceChecker) getLaunchdServices(ctx context.Context, executor diagnostics.CommandExecutor) ([]ServiceInfo, error) {
	// Get list of launchd services
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "launchctl list")
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to list launchd services: %w", err)
	}

	return s.parseLaunchdOutput(stdout)
}

// parseLaunchdOutput parses launchctl list output
func (s *ServiceChecker) parseLaunchdOutput(output string) ([]ServiceInfo, error) {
	var services []ServiceInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Skip header line
	for i, line := range lines {
		if i == 0 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		pid := fields[0]
		// status := fields[1]
		name := fields[2]

		// Determine state based on PID
		state := "inactive"
		if pid != "-" {
			state = "running"
		}

		services = append(services, ServiceInfo{
			Name:        name,
			State:       state,
			LoadState:   "loaded",
			Description: "",
		})
	}

	return services, nil
}

// filterServices filters services based on monitored list
func (s *ServiceChecker) filterServices(services []ServiceInfo, monitored []string) []ServiceInfo {
	var filtered []ServiceInfo

	for _, svc := range services {
		for _, monitored := range monitored {
			if svc.Name == monitored || strings.Contains(svc.Name, monitored) {
				filtered = append(filtered, svc)
				break
			}
		}
	}

	return filtered
}

// formatMessage creates a human-readable message from service data
func (s *ServiceChecker) formatMessage(total, running, failed, inactive int, failedServices []string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("%d services (%d running, %d inactive)",
		total, running, inactive))

	if failed > 0 {
		if len(failedServices) <= 3 {
			parts = append(parts, fmt.Sprintf("%d failed: %s",
				failed, strings.Join(failedServices, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("%d failed: %s and %d more",
				failed, strings.Join(failedServices[:3], ", "), len(failedServices)-3))
		}
	}

	return strings.Join(parts, ", ")
}

// isValidServiceName validates a service name to prevent command injection
// Returns true if the name contains only safe characters
func isValidServiceName(name string) bool {
	if name == "" {
		return false
	}

	// Check length (systemd has a 256 char limit for unit names)
	if len(name) > 256 {
		return false
	}

	// Only allow alphanumeric, dots, dashes, underscores
	// This matches standard systemd/sysvinit naming conventions
	for _, ch := range name {
		if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') &&
			(ch < '0' || ch > '9') && ch != '.' && ch != '-' && ch != '_' {
			return false
		}
	}

	return true
}

// shellQuote safely quotes a string for use in POSIX shell commands
// This prevents command injection by escaping all special characters.
func shellQuote(s string) string {
	// Handle empty string
	if s == "" {
		return "''"
	}

	// POSIX shell quoting: wrap in single quotes and escape existing single quotes
	s = strings.ReplaceAll(s, "'", `'\''`)
	return "'" + s + "'"
}
