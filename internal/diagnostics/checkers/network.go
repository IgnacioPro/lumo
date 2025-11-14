package checkers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// NetworkChecker performs network-related checks
type NetworkChecker struct {
	thresholds diagnostics.NetworkThresholds
}

// NewNetworkChecker creates a new network checker
func NewNetworkChecker(thresholds diagnostics.NetworkThresholds) *NetworkChecker {
	return &NetworkChecker{
		thresholds: thresholds,
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

	// Test DNS resolution
	dnsLatency, err := n.testDNS(ctx, executor)
	if err != nil {
		result.SetData("dns_warning", fmt.Sprintf("DNS test failed: %v", err))
	} else {
		result.SetData("dns_latency_ms", dnsLatency)

		// Add metric
		result.AddMetric(diagnostics.Metric{
			Name:          "dns_latency",
			Value:         dnsLatency,
			Unit:          "ms",
			Threshold:     n.thresholds.DNSLatencyWarn,
			ThresholdType: diagnostics.ThresholdTypeMax,
		})
	}

	// Test connectivity (ping to common hosts)
	connectivity, err := n.testConnectivity(ctx, executor)
	if err != nil {
		result.SetData("connectivity_warning", fmt.Sprintf("Connectivity test failed: %v", err))
	} else {
		result.SetData("connectivity", connectivity)
		if connectivity.PacketLoss > 0 {
			result.AddMetric(diagnostics.Metric{
				Name:          "packet_loss",
				Value:         connectivity.PacketLoss,
				Unit:          "percent",
				Threshold:     n.thresholds.PacketLossWarn,
				ThresholdType: diagnostics.ThresholdTypeMax,
			})
		}
	}

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
	result.Message = n.formatMessage(interfaces, connectionCount, dnsLatency, connectivity)

	return result, nil
}

// NetworkInterface holds information about a network interface
type NetworkInterface struct {
	Name       string `json:"name"`
	State      string `json:"state"`       // up, down, unknown
	IPAddress  string `json:"ip_address"`  // IPv4 address
	MACAddress string `json:"mac_address"` // Hardware address
}

// ConnectivityResult holds connectivity test results
type ConnectivityResult struct {
	Host         string  `json:"host"`
	Reachable    bool    `json:"reachable"`
	AvgLatency   float64 `json:"avg_latency_ms"`
	PacketLoss   float64 `json:"packet_loss_percent"`
	PacketsSent  int     `json:"packets_sent"`
	PacketsRecv  int     `json:"packets_received"`
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
	if strings.Contains(stdout, "UP") || strings.Contains(stdout, "DOWN") {
		return n.parseIpBrief(stdout)
	}

	// Otherwise parse as ifconfig
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
		// New interface starts at beginning of line (no leading whitespace)
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			if currentIface != nil {
				interfaces = append(interfaces, *currentIface)
			}

			fields := strings.Fields(line)
			if len(fields) > 0 {
				name := strings.TrimSuffix(fields[0], ":")
				currentIface = &NetworkInterface{
					Name:  name,
					State: "unknown",
				}
			}
		} else if currentIface != nil {
			// Parse interface details
			if strings.Contains(line, "inet ") && !strings.Contains(line, "inet6") {
				fields := strings.Fields(line)
				for i, field := range fields {
					if field == "inet" && i+1 < len(fields) {
						currentIface.IPAddress = fields[i+1]
						break
					}
				}
			}

			if strings.Contains(strings.ToUpper(line), "UP") {
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
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx,
		"ss -tan state established 2>/dev/null | wc -l || netstat -tan | grep ESTABLISHED | wc -l")
	if err != nil || exitCode != 0 {
		return 0, fmt.Errorf("failed to count connections: %w", err)
	}

	count, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return 0, fmt.Errorf("failed to parse connection count: %w", err)
	}

	// Subtract header line if present
	if count > 0 {
		count--
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
func (n *NetworkChecker) formatMessage(interfaces []NetworkInterface, connections int, dnsLatency float64, connectivity *ConnectivityResult) string {
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

	// DNS latency
	if dnsLatency > 0 {
		parts = append(parts, fmt.Sprintf("DNS: %.0fms", dnsLatency))
	}

	// Connectivity
	if connectivity != nil {
		if connectivity.Reachable {
			parts = append(parts, fmt.Sprintf("Ping: %.1fms", connectivity.AvgLatency))
			if connectivity.PacketLoss > 0 {
				parts = append(parts, fmt.Sprintf("%.0f%% loss", connectivity.PacketLoss))
			}
		} else {
			parts = append(parts, "Connectivity: FAILED")
		}
	}

	return strings.Join(parts, ", ")
}
