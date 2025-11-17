package checkers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/diagnostics"
)

// NetworkChecker performs network-related checks
type NetworkChecker struct {
	thresholds diagnostics.NetworkThresholds
	targets    []config.NetworkTarget
}

// NewNetworkChecker creates a new network checker
func NewNetworkChecker(thresholds diagnostics.NetworkThresholds, targets []config.NetworkTarget) *NetworkChecker {
	// If no targets provided, use defaults
	if len(targets) == 0 {
		targets = []config.NetworkTarget{
			{Host: "8.8.8.8", Port: 0, Protocol: "icmp"},
			{Host: "google.com", Port: 0, Protocol: "icmp"},
		}
	}
	return &NetworkChecker{
		thresholds: thresholds,
		targets:    targets,
	}
}

// Name returns the checker name
func (n *NetworkChecker) Name() string {
	return "network_check"
}

// Category returns the checker category
func (n *NetworkChecker) Category() diagnostics.CheckCategory {
	return diagnostics.CategoryNetwork
}

// Description returns the checker description
func (n *NetworkChecker) Description() string {
	return "Checks network connectivity, interfaces, and DNS"
}

// RequiresRoot returns false as basic network checks don't need root
func (n *NetworkChecker) RequiresRoot() bool {
	return false
}

// Run executes the network check
func (n *NetworkChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
	result := &diagnostics.CheckResult{
		Name:      n.Name(),
		Category:  n.Category(),
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
		Metrics:   []diagnostics.Metric{},
	}

	startTime := time.Now()

	// Get network interfaces
	interfaces, err := n.getNetworkInterfaces(ctx, executor)
	if err != nil {
		result.SetData("interfaces_warning", fmt.Sprintf("Could not retrieve interfaces: %v", err))
	} else {
		result.SetData("interfaces", interfaces)
		result.SetData("interface_count", len(interfaces))
	}

	// Get connection count
	connectionCount, err := n.getConnectionCount(ctx, executor)
	if err != nil {
		result.SetData("connections_warning", fmt.Sprintf("Could not retrieve connections: %v", err))
	} else {
		result.SetData("active_connections", connectionCount)

		// Add metric
		result.AddMetric(diagnostics.Metric{
			Name:          "active_connections",
			Value:         float64(connectionCount),
			Unit:          "count",
			Threshold:     float64(n.thresholds.ConnectionsWarn),
			ThresholdType: diagnostics.ThresholdTypeMax,
		})
	}

	// Test configured network targets
	targetResults := []TargetResult{}
	for _, target := range n.targets {
		targetResult := n.testTarget(ctx, executor, target)
		targetResults = append(targetResults, targetResult)

		// Add metrics for this target
		if !targetResult.Reachable {
			result.AddMetric(diagnostics.Metric{
				Name:          fmt.Sprintf("%s_reachable", target.Host),
				Value:         0,
				Unit:          "boolean",
				Threshold:     1,
				ThresholdType: diagnostics.ThresholdTypeMin,
			})
		} else if targetResult.Latency > 0 {
			result.AddMetric(diagnostics.Metric{
				Name:          fmt.Sprintf("%s_latency", target.Host),
				Value:         targetResult.Latency,
				Unit:          "ms",
				Threshold:     n.thresholds.DNSLatencyWarn, // Reuse DNS latency threshold
				ThresholdType: diagnostics.ThresholdTypeMax,
			})
		}

		if targetResult.PacketLoss > 0 {
			result.AddMetric(diagnostics.Metric{
				Name:          fmt.Sprintf("%s_packet_loss", target.Host),
				Value:         targetResult.PacketLoss,
				Unit:          "percent",
				Threshold:     n.thresholds.PacketLossWarn,
				ThresholdType: diagnostics.ThresholdTypeMax,
			})
		}
	}
	result.SetData("target_results", targetResults)

	// Get network interface statistics
	stats, err := n.getInterfaceStats(ctx, executor)
	if err != nil {
		result.SetData("stats_warning", fmt.Sprintf("Could not retrieve interface stats: %v", err))
	} else {
		result.SetData("interface_stats", stats)

		// Calculate total errors
		totalErrors := stats.RxErrors + stats.TxErrors
		if totalErrors > 0 {
			result.SetData("total_errors", totalErrors)
		}
	}

	result.Duration = time.Since(startTime)

	// Generate message
	result.Message = n.formatMessage(interfaces, connectionCount, targetResults)

	return result, nil
}

// NetworkInterface holds information about a network interface
type NetworkInterface struct {
	Name       string `json:"name"`
	State      string `json:"state"`            // up, down, inactive, unknown
	IPAddress  string `json:"ip_address"`       // IPv4 address
	MACAddress string `json:"mac_address"`      // Hardware address
	Status     string `json:"status,omitempty"` // active, inactive, unknown (from status line)
}

// ConnectivityResult holds connectivity test results
type ConnectivityResult struct {
	Host        string  `json:"host"`
	Reachable   bool    `json:"reachable"`
	AvgLatency  float64 `json:"avg_latency_ms"`
	PacketLoss  float64 `json:"packet_loss_percent"`
	PacketsSent int     `json:"packets_sent"`
	PacketsRecv int     `json:"packets_received"`
}

// TargetResult holds the result of testing a single network target
type TargetResult struct {
	Host        string  `json:"host"`
	Port        int     `json:"port"`
	Protocol    string  `json:"protocol"`
	Reachable   bool    `json:"reachable"`
	Latency     float64 `json:"latency_ms"`
	PacketLoss  float64 `json:"packet_loss_percent,omitempty"`
	Error       string  `json:"error,omitempty"`
	PacketsSent int     `json:"packets_sent,omitempty"`
	PacketsRecv int     `json:"packets_received,omitempty"`
}

// InterfaceStats holds interface statistics
type InterfaceStats struct {
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	TxPackets uint64 `json:"tx_packets"`
	RxErrors  uint64 `json:"rx_errors"`
	TxErrors  uint64 `json:"tx_errors"`
}

// getNetworkInterfaces retrieves network interface information
func (n *NetworkChecker) getNetworkInterfaces(ctx context.Context, executor diagnostics.CommandExecutor) ([]NetworkInterface, error) {
	// Use ip command on Linux, ifconfig fallback for macOS/BSD
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "ip -br addr show 2>/dev/null || ifconfig -a")
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to get network interfaces: %w", err)
	}

	// Check if it's ip command output (brief format)
	// The 'ip -br addr show' format has specific patterns:
	// - No leading tabs/spaces
	// - Lines like: "eth0 UP 192.168.1.1/24"
	// - No "flags=" pattern (which is in ifconfig)
	if strings.Contains(stdout, "flags=") {
		// This is ifconfig output
		return n.parseIfconfig(stdout)
	}

	// Check if this looks like ip -br output (has proper state columns)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) > 0 {
		firstLine := strings.Fields(lines[0])
		if len(firstLine) >= 2 {
			state := strings.ToUpper(firstLine[1])
			if state == "UP" || state == "DOWN" || state == "UNKNOWN" {
				// Likely ip -br format
				return n.parseIpBrief(stdout)
			}
		}
	}

	// Default to ifconfig parsing
	return n.parseIfconfig(stdout)
}

// parseIpBrief parses 'ip -br addr show' output
func (n *NetworkChecker) parseIpBrief(output string) ([]NetworkInterface, error) {
	var interfaces []NetworkInterface
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		name := fields[0]
		state := strings.ToLower(fields[1])
		ipAddr := ""

		// Extract IP address (skip MAC addresses)
		for i := 2; i < len(fields); i++ {
			if strings.Contains(fields[i], ".") && !strings.Contains(fields[i], ":") {
				ipAddr = strings.Split(fields[i], "/")[0]
				break
			}
		}

		interfaces = append(interfaces, NetworkInterface{
			Name:      name,
			State:     state,
			IPAddress: ipAddr,
		})
	}

	return interfaces, nil
}

// parseIfconfig parses ifconfig output
func (n *NetworkChecker) parseIfconfig(output string) ([]NetworkInterface, error) {
	var interfaces []NetworkInterface
	lines := strings.Split(output, "\n")

	var currentIface *NetworkInterface
	for _, line := range lines {
		// Trim for checking, but preserve for parsing
		trimmedLine := strings.TrimSpace(line)

		// New interface starts at beginning of line (no leading whitespace)
		// Interface lines start with alphanumeric (e.g., "en0:", "lo0:", "eth0:")
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && trimmedLine != "" {
			// This is a new interface line
			if currentIface != nil {
				interfaces = append(interfaces, *currentIface)
			}

			fields := strings.Fields(line)
			if len(fields) > 0 {
				name := strings.TrimSuffix(fields[0], ":")
				currentIface = &NetworkInterface{
					Name:   name,
					State:  "unknown",
					Status: "unknown",
				}

				// Check flags on the interface line itself (macOS format)
				// flags=<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST>
				if strings.Contains(line, "flags=") {
					// Extract flags between < and >
					flagStart := strings.Index(line, "<")
					flagEnd := strings.Index(line, ">")
					if flagStart != -1 && flagEnd != -1 {
						flags := line[flagStart+1 : flagEnd]
						if strings.Contains(flags, "UP") {
							currentIface.State = "up"
						}
					}
				}
			}
		} else if currentIface != nil && trimmedLine != "" {
			// Parse interface details (indented lines)
			if strings.Contains(line, "inet ") && !strings.Contains(line, "inet6") {
				fields := strings.Fields(line)
				for i, field := range fields {
					if field == "inet" && i+1 < len(fields) {
						currentIface.IPAddress = fields[i+1]
						break
					}
				}
			}

			// Check for status line (macOS: "status: active" or "status: inactive")
			if strings.Contains(line, "status:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					statusVal := fields[len(fields)-1]
					currentIface.Status = statusVal
					// Override state based on actual status
					switch statusVal {
					case "active":
						currentIface.State = "up"
					case "inactive":
						currentIface.State = "down"
					}
				}
			}

			// Also check for UP in detail lines (Linux format - less common)
			if strings.Contains(strings.ToUpper(line), " UP") || strings.Contains(strings.ToUpper(line), "UP ") {
				currentIface.State = "up"
			}
		}
	}

	if currentIface != nil {
		interfaces = append(interfaces, *currentIface)
	}

	return interfaces, nil
}

// getConnectionCount gets active network connections count
func (n *NetworkChecker) getConnectionCount(ctx context.Context, executor diagnostics.CommandExecutor) (int, error) {
	// Count established connections
	// Use command -v to check if ss exists first, otherwise use netstat
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx,
		"if command -v ss >/dev/null 2>&1; then ss -tan state established 2>/dev/null | tail -n +2 | wc -l; else netstat -tan 2>/dev/null | grep -c ESTABLISHED; fi")
	if err != nil || exitCode != 0 {
		return 0, fmt.Errorf("failed to count connections: %w", err)
	}

	count, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return 0, fmt.Errorf("failed to parse connection count: %w", err)
	}

	return count, nil
}

// testDNS tests DNS resolution latency
func (n *NetworkChecker) testDNS(ctx context.Context, executor diagnostics.CommandExecutor) (float64, error) {
	// Use dig with timing if available, fallback to nslookup
	startTime := time.Now()
	_, _, exitCode, err := executor.ExecuteWithContext(ctx, "nslookup google.com >/dev/null 2>&1")
	latency := time.Since(startTime).Milliseconds()

	if err != nil || exitCode != 0 {
		return 0, fmt.Errorf("DNS resolution failed")
	}

	return float64(latency), nil
}

// testConnectivity tests connectivity to a common host
func (n *NetworkChecker) testConnectivity(ctx context.Context, executor diagnostics.CommandExecutor) (*ConnectivityResult, error) {
	host := "8.8.8.8" // Google DNS
	count := 4

	// Ping command (cross-platform)
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx,
		fmt.Sprintf("ping -c %d -W 2 %s 2>/dev/null || ping -n %d -w 2000 %s", count, host, count, host))

	result := &ConnectivityResult{
		Host:        host,
		Reachable:   exitCode == 0,
		PacketsSent: count,
	}

	if err != nil || exitCode != 0 {
		result.PacketLoss = 100.0
		return result, nil
	}

	// Parse ping output
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		// Look for packet loss line
		if strings.Contains(line, "packet loss") || strings.Contains(line, "loss") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if strings.HasSuffix(field, "%") {
					lossStr := strings.TrimSuffix(field, "%")
					if loss, err := strconv.ParseFloat(lossStr, 64); err == nil {
						result.PacketLoss = loss
					}
				}
				if field == "received" && i > 0 {
					if recv, err := strconv.Atoi(fields[i-1]); err == nil {
						result.PacketsRecv = recv
					}
				}
			}
		}

		// Look for avg latency (rtt min/avg/max format)
		if strings.Contains(line, "avg") || strings.Contains(line, "Average") {
			fields := strings.Split(line, "=")
			if len(fields) > 1 {
				values := strings.Split(strings.TrimSpace(fields[1]), "/")
				if len(values) >= 2 {
					if avg, err := strconv.ParseFloat(values[1], 64); err == nil {
						result.AvgLatency = avg
					}
				}
			}
		}
	}

	return result, nil
}

// testTarget tests connectivity to a specific network target
func (n *NetworkChecker) testTarget(ctx context.Context, executor diagnostics.CommandExecutor, target config.NetworkTarget) TargetResult {
	result := TargetResult{
		Host:     target.Host,
		Port:     target.Port,
		Protocol: target.Protocol,
	}

	// Default to ICMP if protocol is empty
	protocol := target.Protocol
	if protocol == "" {
		protocol = "icmp"
	}

	switch protocol {
	case "tcp":
		return n.testTCPTarget(ctx, executor, target)
	case "icmp":
		return n.testICMPTarget(ctx, executor, target)
	default:
		result.Error = fmt.Sprintf("unsupported protocol: %s", protocol)
		return result
	}
}

// testTCPTarget tests TCP connectivity to a target
func (n *NetworkChecker) testTCPTarget(ctx context.Context, executor diagnostics.CommandExecutor, target config.NetworkTarget) TargetResult {
	result := TargetResult{
		Host:     target.Host,
		Port:     target.Port,
		Protocol: "tcp",
	}

	startTime := time.Now()

	// Try different methods in order:
	// 1. nc with -w flag for timeout (cross-platform)
	// 2. Bash TCP redirection (works if bash supports it)
	// 3. Perl one-liner (usually available)
	cmd := fmt.Sprintf(
		"nc -w 5 -zv %s %d 2>&1 || bash -c 'cat < /dev/null > /dev/tcp/%s/%d' 2>&1 || perl -MIO::Socket -e 'IO::Socket::INET->new(\"%s:%d\") or exit 1' 2>&1",
		target.Host, target.Port, target.Host, target.Port, target.Host, target.Port,
	)

	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	latency := time.Since(startTime).Milliseconds()

	if err != nil || exitCode != 0 {
		result.Reachable = false
		result.Latency = float64(latency)
		// Try to determine specific error
		output := stdout + stderr
		if strings.Contains(output, "Connection refused") || strings.Contains(output, "refused") {
			result.Error = "connection_refused"
		} else if strings.Contains(output, "Name or service not known") || strings.Contains(output, "could not resolve") || strings.Contains(output, "nodename nor servname provided") {
			result.Error = "dns_failed"
		} else if strings.Contains(output, "timed out") || strings.Contains(output, "Timeout") || strings.Contains(output, "timeout") {
			result.Error = "timeout"
		} else {
			result.Error = "unreachable"
		}
		return result
	}

	result.Reachable = true
	result.Latency = float64(latency)
	return result
}

// testICMPTarget tests ICMP (ping) connectivity to a target
func (n *NetworkChecker) testICMPTarget(ctx context.Context, executor diagnostics.CommandExecutor, target config.NetworkTarget) TargetResult {
	result := TargetResult{
		Host:     target.Host,
		Port:     0,
		Protocol: "icmp",
	}

	count := 4

	// Ping command (cross-platform)
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx,
		fmt.Sprintf("ping -c %d -W 2 %s 2>/dev/null || ping -n %d -w 2000 %s", count, target.Host, count, target.Host))

	result.PacketsSent = count

	if err != nil || exitCode != 0 {
		result.Reachable = false
		result.PacketLoss = 100.0
		result.Error = "unreachable"
		return result
	}

	result.Reachable = true

	// Parse ping output
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		// Look for packet loss line
		if strings.Contains(line, "packet loss") || strings.Contains(line, "loss") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if strings.HasSuffix(field, "%") {
					lossStr := strings.TrimSuffix(field, "%")
					if loss, err := strconv.ParseFloat(lossStr, 64); err == nil {
						result.PacketLoss = loss
					}
				}
				if field == "received" && i > 0 {
					if recv, err := strconv.Atoi(fields[i-1]); err == nil {
						result.PacketsRecv = recv
					}
				}
			}
		}

		// Look for avg latency (rtt min/avg/max format)
		if strings.Contains(line, "avg") || strings.Contains(line, "Average") {
			fields := strings.Split(line, "=")
			if len(fields) > 1 {
				values := strings.Split(strings.TrimSpace(fields[1]), "/")
				if len(values) >= 2 {
					if avg, err := strconv.ParseFloat(values[1], 64); err == nil {
						result.Latency = avg
					}
				}
			}
		}
	}

	return result
}

// getInterfaceStats retrieves network interface statistics
func (n *NetworkChecker) getInterfaceStats(ctx context.Context, executor diagnostics.CommandExecutor) (*InterfaceStats, error) {
	// Try Linux /proc/net/dev first
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "cat /proc/net/dev 2>/dev/null")
	if err == nil && exitCode == 0 {
		return n.parseProcNetDev(stdout)
	}

	// Fallback: try netstat
	stdout, _, exitCode, err = executor.ExecuteWithContext(ctx, "netstat -ib 2>/dev/null | grep -v '^lo'")
	if err == nil && exitCode == 0 {
		return n.parseNetstatStats(stdout)
	}

	return nil, fmt.Errorf("could not retrieve interface statistics")
}

// parseProcNetDev parses /proc/net/dev output
func (n *NetworkChecker) parseProcNetDev(output string) (*InterfaceStats, error) {
	stats := &InterfaceStats{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		// Skip header lines and loopback
		if strings.Contains(line, "Inter-") || strings.Contains(line, "face") || strings.Contains(line, "lo:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 17 {
			continue
		}

		// Sum up all interfaces
		if rxBytes, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
			stats.RxBytes += rxBytes
		}
		if rxPackets, err := strconv.ParseUint(fields[2], 10, 64); err == nil {
			stats.RxPackets += rxPackets
		}
		if rxErrors, err := strconv.ParseUint(fields[3], 10, 64); err == nil {
			stats.RxErrors += rxErrors
		}
		if txBytes, err := strconv.ParseUint(fields[9], 10, 64); err == nil {
			stats.TxBytes += txBytes
		}
		if txPackets, err := strconv.ParseUint(fields[10], 10, 64); err == nil {
			stats.TxPackets += txPackets
		}
		if txErrors, err := strconv.ParseUint(fields[11], 10, 64); err == nil {
			stats.TxErrors += txErrors
		}
	}

	return stats, nil
}

// parseNetstatStats parses netstat statistics
func (n *NetworkChecker) parseNetstatStats(output string) (*InterfaceStats, error) {
	stats := &InterfaceStats{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		// Try to parse packets and errors
		if packets, err := strconv.ParseUint(fields[4], 10, 64); err == nil {
			stats.RxPackets += packets
		}
		if errors, err := strconv.ParseUint(fields[5], 10, 64); err == nil {
			stats.RxErrors += errors
		}
	}

	return stats, nil
}

// formatMessage creates a human-readable message from network data
func (n *NetworkChecker) formatMessage(interfaces []NetworkInterface, connections int, targetResults []TargetResult) string {
	var parts []string

	// Interface count
	upCount := 0
	for _, iface := range interfaces {
		if iface.State == "up" || iface.State == "UP" {
			upCount++
		}
	}
	parts = append(parts, fmt.Sprintf("%d interfaces (%d up)", len(interfaces), upCount))

	// Connections
	if connections >= 0 {
		parts = append(parts, fmt.Sprintf("%d active connections", connections))
	}

	// Target results summary
	reachableCount := 0
	unreachableCount := 0
	for _, target := range targetResults {
		if target.Reachable {
			reachableCount++
		} else {
			unreachableCount++
		}
	}

	if len(targetResults) > 0 {
		if unreachableCount == 0 {
			parts = append(parts, fmt.Sprintf("%d/%d targets reachable", reachableCount, len(targetResults)))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%d targets reachable (%d FAILED)", reachableCount, len(targetResults), unreachableCount))
		}
	}

	return strings.Join(parts, ", ")
}
