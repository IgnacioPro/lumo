package checkers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// PortsChecker checks for open ports and listening services
type PortsChecker struct {
	whitelistedPorts map[int]bool
}

// NewPortsChecker creates a new open ports checker
func NewPortsChecker(whitelistedPorts []int) *PortsChecker {
	whitelist := make(map[int]bool)
	for _, port := range whitelistedPorts {
		whitelist[port] = true
	}
	return &PortsChecker{
		whitelistedPorts: whitelist,
	}
}

// Name returns the checker name
func (p *PortsChecker) Name() string {
	return "open_ports"
}

// Category returns the checker category
func (p *PortsChecker) Category() diagnostics.CheckCategory {
	return diagnostics.CategorySecurity
}

// Description returns the checker description
func (p *PortsChecker) Description() string {
	return "Checks for open ports and listening services"
}

// RequiresRoot returns false as basic port listing doesn't need root
func (p *PortsChecker) RequiresRoot() bool {
	return false
}

// ListeningPort represents a listening port and its associated process
type ListeningPort struct {
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"` // tcp, tcp6, udp, udp6
	Address     string `json:"address"`  // Listen address (0.0.0.0, 127.0.0.1, ::, etc.)
	Process     string `json:"process"`  // Process name/command
	PID         int    `json:"pid"`
	IsPublic    bool   `json:"is_public"`    // True if listening on 0.0.0.0 or ::
	IsLocalhost bool   `json:"is_localhost"` // True if listening on 127.0.0.1 or ::1
}

// Run executes the open ports check
func (p *PortsChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
	result := diagnostics.NewCheckResult(p)
	startTime := time.Now()

	// Get listening ports
	ports, err := p.getListeningPorts(ctx, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to get listening ports: %w", err)
	}

	// Categorize ports
	publicPorts := []ListeningPort{}
	localhostPorts := []ListeningPort{}
	unexpectedPorts := []ListeningPort{}
	highNumberedPorts := []ListeningPort{}

	for _, port := range ports {
		if port.IsPublic {
			publicPorts = append(publicPorts, port)
		}
		if port.IsLocalhost {
			localhostPorts = append(localhostPorts, port)
		}
		if !p.whitelistedPorts[port.Port] {
			unexpectedPorts = append(unexpectedPorts, port)
		}
		if port.Port > 10000 {
			highNumberedPorts = append(highNumberedPorts, port)
		}
	}

	// Store data
	result.SetData("total_listening_ports", len(ports))
	result.SetData("public_ports", len(publicPorts))
	result.SetData("localhost_ports", len(localhostPorts))
	result.SetData("unexpected_ports", len(unexpectedPorts))
	result.SetData("high_numbered_ports", len(highNumberedPorts))
	result.SetData("all_ports", ports)

	if len(publicPorts) > 0 {
		result.SetData("public_ports_list", publicPorts)
	}
	if len(unexpectedPorts) > 0 {
		result.SetData("unexpected_ports_list", unexpectedPorts)
	}

	// Add metrics
	result.AddMetric(diagnostics.Metric{
		Name:          "public_listening_ports",
		Value:         float64(len(publicPorts)),
		Unit:          "count",
		Threshold:     5, // Warning if more than 5 public ports
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.AddMetric(diagnostics.Metric{
		Name:          "unexpected_ports",
		Value:         float64(len(unexpectedPorts)),
		Unit:          "count",
		Threshold:     3, // Warning if more than 3 unexpected ports
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.Duration = time.Since(startTime)
	result.Message = p.formatMessage(len(ports), len(publicPorts), len(unexpectedPorts))

	return result, nil
}

// getListeningPorts retrieves listening ports using ss or netstat
func (p *PortsChecker) getListeningPorts(ctx context.Context, executor diagnostics.CommandExecutor) ([]ListeningPort, error) {
	// Try ss first (modern Linux)
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "ss -tlnp 2>/dev/null")
	if exitCode == 0 && stdout != "" {
		ports, err := p.parseSSOutput(stdout)
		if err == nil && len(ports) > 0 {
			return ports, nil
		}
	}

	// Fallback to netstat
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "netstat -tlnp 2>/dev/null")
	if err != nil || exitCode != 0 {
		// Try macOS/BSD netstat (no -p flag)
		stdout, _, exitCode, err = executor.ExecuteWithContext(ctx, "netstat -tln 2>/dev/null")
		if err != nil || exitCode != 0 {
			return nil, fmt.Errorf("failed to execute netstat: %w", err)
		}
	}

	return p.parseNetstatOutput(stdout)
}

// parseSSOutput parses ss command output
func (p *PortsChecker) parseSSOutput(output string) ([]ListeningPort, error) {
	lines := strings.Split(output, "\n")
	ports := []ListeningPort{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Netid") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// ss output: State Recv-Q Send-Q Local_Address:Port Peer_Address:Port Process
		// Example: LISTEN 0 128 0.0.0.0:22 0.0.0.0:* users:(("sshd",pid=1234,fd=3))

		localAddr := fields[3]
		port, err := p.extractPort(localAddr)
		if err != nil {
			continue
		}

		address := p.extractAddress(localAddr)
		protocol := strings.ToLower(fields[0])

		// Extract process info (last field, may not exist)
		processInfo := ""
		pid := 0
		if len(fields) >= 6 {
			processInfo = fields[5]
			pid = p.extractPID(processInfo)
		}

		ports = append(ports, ListeningPort{
			Port:        port,
			Protocol:    protocol,
			Address:     address,
			Process:     p.extractProcessName(processInfo),
			PID:         pid,
			IsPublic:    p.isPublicAddress(address),
			IsLocalhost: p.isLocalhostAddress(address),
		})
	}

	return ports, nil
}

// parseNetstatOutput parses netstat command output
func (p *PortsChecker) parseNetstatOutput(output string) ([]ListeningPort, error) {
	lines := strings.Split(output, "\n")
	ports := []ListeningPort{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Active") || strings.HasPrefix(line, "Proto") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// netstat output: Proto Recv-Q Send-Q Local_Address Foreign_Address State [PID/Program]
		// Example: tcp 0 0 0.0.0.0:22 0.0.0.0:* LISTEN 1234/sshd

		protocol := fields[0]
		localAddr := fields[3]
		state := ""
		if len(fields) > 5 {
			state = fields[5]
		}

		// Only process LISTEN state
		if state != "" && !strings.Contains(strings.ToUpper(state), "LISTEN") {
			continue
		}

		port, err := p.extractPort(localAddr)
		if err != nil {
			continue
		}

		address := p.extractAddress(localAddr)

		// Extract process info if available
		processInfo := ""
		pid := 0
		if len(fields) >= 7 {
			processInfo = fields[6]
			pid = p.extractPID(processInfo)
		}

		ports = append(ports, ListeningPort{
			Port:        port,
			Protocol:    protocol,
			Address:     address,
			Process:     p.extractProcessName(processInfo),
			PID:         pid,
			IsPublic:    p.isPublicAddress(address),
			IsLocalhost: p.isLocalhostAddress(address),
		})
	}

	return ports, nil
}

// extractPort extracts port number from address:port string
func (p *PortsChecker) extractPort(addrPort string) (int, error) {
	// Handle IPv6 format [::]:port or IPv4 format 0.0.0.0:port
	lastColon := strings.LastIndex(addrPort, ":")
	if lastColon == -1 {
		return 0, fmt.Errorf("no port found in %s", addrPort)
	}

	portStr := addrPort[lastColon+1:]
	// Remove any trailing wildcards
	portStr = strings.TrimSuffix(portStr, "*")

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port: %w", err)
	}

	return port, nil
}

// extractAddress extracts IP address from address:port string
func (p *PortsChecker) extractAddress(addrPort string) string {
	// Handle IPv6 format [::]:port
	if strings.HasPrefix(addrPort, "[") {
		endBracket := strings.Index(addrPort, "]")
		if endBracket != -1 {
			return addrPort[1:endBracket]
		}
	}

	// IPv4 format 0.0.0.0:port
	lastColon := strings.LastIndex(addrPort, ":")
	if lastColon != -1 {
		return addrPort[:lastColon]
	}

	return addrPort
}

// extractPID extracts PID from process info string
func (p *PortsChecker) extractPID(processInfo string) int {
	// Format can be: "1234/sshd" or "users:(("sshd",pid=1234,fd=3))"
	if strings.Contains(processInfo, "pid=") {
		// ss format
		parts := strings.Split(processInfo, "pid=")
		if len(parts) > 1 {
			pidStr := strings.Split(parts[1], ",")[0]
			if pid, err := strconv.Atoi(pidStr); err == nil {
				return pid
			}
		}
	} else if strings.Contains(processInfo, "/") {
		// netstat format
		pidStr := strings.Split(processInfo, "/")[0]
		if pid, err := strconv.Atoi(pidStr); err == nil {
			return pid
		}
	}

	return 0
}

// extractProcessName extracts process name from process info string
func (p *PortsChecker) extractProcessName(processInfo string) string {
	if processInfo == "" || processInfo == "-" {
		return "unknown"
	}

	// ss format: users:(("sshd",pid=1234,fd=3))
	if strings.Contains(processInfo, "((") {
		start := strings.Index(processInfo, "((\"")
		if start != -1 {
			start += 3
			end := strings.Index(processInfo[start:], "\"")
			if end != -1 {
				return processInfo[start : start+end]
			}
		}
	}

	// netstat format: 1234/sshd
	if strings.Contains(processInfo, "/") {
		parts := strings.Split(processInfo, "/")
		if len(parts) > 1 {
			return parts[1]
		}
	}

	return processInfo
}

// isPublicAddress checks if an address is publicly accessible
func (p *PortsChecker) isPublicAddress(addr string) bool {
	return addr == "0.0.0.0" || addr == "::" || addr == "*"
}

// isLocalhostAddress checks if an address is localhost-only
func (p *PortsChecker) isLocalhostAddress(addr string) bool {
	return addr == "127.0.0.1" || addr == "::1" || addr == "localhost"
}

// formatMessage creates a human-readable message from port scan results
func (p *PortsChecker) formatMessage(totalPorts, publicPorts, unexpectedPorts int) string {
	msg := fmt.Sprintf("%d listening ports detected", totalPorts)

	warnings := []string{}
	if publicPorts > 0 {
		warnings = append(warnings, fmt.Sprintf("%d public", publicPorts))
	}
	if unexpectedPorts > 0 {
		warnings = append(warnings, fmt.Sprintf("%d unexpected", unexpectedPorts))
	}

	if len(warnings) > 0 {
		msg += fmt.Sprintf(" (%s)", strings.Join(warnings, ", "))
	}

	return msg
}
