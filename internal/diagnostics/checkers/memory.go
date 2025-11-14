package checkers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// MemoryChecker performs memory and swap checks
type MemoryChecker struct {
	thresholds diagnostics.MemoryThresholds
}

// NewMemoryChecker creates a new memory checker
func NewMemoryChecker(thresholds diagnostics.MemoryThresholds) *MemoryChecker {
	return &MemoryChecker{
		thresholds: thresholds,
	}
}

// Name returns the checker name
func (m *MemoryChecker) Name() string {
	return "memory_check"
}

// Category returns the checker category
func (m *MemoryChecker) Category() diagnostics.CheckCategory {
	return diagnostics.CategoryMemory
}

// Description returns the checker description
func (m *MemoryChecker) Description() string {
	return "Checks memory and swap usage"
}

// RequiresRoot returns false as memory checks don't need root
func (m *MemoryChecker) RequiresRoot() bool {
	return false
}

// Run executes the memory check
func (m *MemoryChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
	result := &diagnostics.CheckResult{
		Name:      m.Name(),
		Category:  m.Category(),
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
		Metrics:   []diagnostics.Metric{},
	}

	startTime := time.Now()

	// Get memory statistics
	memStats, err := m.getMemoryStats(ctx, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}

	// Store raw data
	result.SetData("total_bytes", memStats.TotalBytes)
	result.SetData("used_bytes", memStats.UsedBytes)
	result.SetData("free_bytes", memStats.FreeBytes)
	result.SetData("available_bytes", memStats.AvailableBytes)
	result.SetData("used_percent", memStats.UsedPercent)

	// Swap data
	result.SetData("swap_total_bytes", memStats.SwapTotalBytes)
	result.SetData("swap_used_bytes", memStats.SwapUsedBytes)
	result.SetData("swap_used_percent", memStats.SwapUsedPercent)

	// Add metrics for threshold evaluation
	result.AddMetric(diagnostics.Metric{
		Name:          "memory_used_percent",
		Value:         memStats.UsedPercent,
		Unit:          "percent",
		Threshold:     m.thresholds.UsageWarn,
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	if memStats.SwapTotalBytes > 0 {
		result.AddMetric(diagnostics.Metric{
			Name:          "swap_used_percent",
			Value:         memStats.SwapUsedPercent,
			Unit:          "percent",
			Threshold:     m.thresholds.SwapWarn,
			ThresholdType: diagnostics.ThresholdTypeMax,
		})
	}

	result.Duration = time.Since(startTime)

	// Generate message
	result.Message = m.formatMessage(memStats)

	return result, nil
}

// MemoryStats holds parsed memory statistics
type MemoryStats struct {
	TotalBytes      uint64
	UsedBytes       uint64
	FreeBytes       uint64
	AvailableBytes  uint64
	UsedPercent     float64
	SwapTotalBytes  uint64
	SwapUsedBytes   uint64
	SwapUsedPercent float64
}

// getMemoryStats retrieves memory statistics from the system
func (m *MemoryChecker) getMemoryStats(ctx context.Context, executor diagnostics.CommandExecutor) (*MemoryStats, error) {
	// Try Linux /proc/meminfo first, then fall back to free command, then macOS vm_stat
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "cat /proc/meminfo 2>/dev/null || free -b 2>/dev/null")
	if err != nil || exitCode != 0 {
		// Try macOS
		return m.getMemoryStatsMacOS(ctx, executor)
	}

	// Check if output looks like /proc/meminfo (contains colons)
	if strings.Contains(stdout, ":") {
		return m.parseMeminfo(stdout)
	}

	// Otherwise assume it's free command output
	return m.parseFreeCommand(stdout)
}

// parseMeminfo parses /proc/meminfo output
func (m *MemoryChecker) parseMeminfo(output string) (*MemoryStats, error) {
	stats := &MemoryStats{}
	lines := strings.Split(output, "\n")

	values := make(map[string]uint64)
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		valueStr := fields[1]
		value, err := strconv.ParseUint(valueStr, 10, 64)
		if err != nil {
			continue
		}

		// Convert from KB to bytes (meminfo is in KB)
		values[key] = value * 1024
	}

	// Extract values
	stats.TotalBytes = values["MemTotal"]
	stats.FreeBytes = values["MemFree"]
	stats.AvailableBytes = values["MemAvailable"]

	// If MemAvailable is not present (old kernels), estimate it
	if stats.AvailableBytes == 0 {
		stats.AvailableBytes = stats.FreeBytes + values["Buffers"] + values["Cached"]
	}

	stats.UsedBytes = stats.TotalBytes - stats.AvailableBytes
	if stats.TotalBytes > 0 {
		stats.UsedPercent = float64(stats.UsedBytes) / float64(stats.TotalBytes) * 100.0
	}

	// Swap
	stats.SwapTotalBytes = values["SwapTotal"]
	stats.SwapUsedBytes = stats.SwapTotalBytes - values["SwapFree"]
	if stats.SwapTotalBytes > 0 {
		stats.SwapUsedPercent = float64(stats.SwapUsedBytes) / float64(stats.SwapTotalBytes) * 100.0
	}

	return stats, nil
}

// parseFreeCommand parses free command output
func (m *MemoryChecker) parseFreeCommand(output string) (*MemoryStats, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected free command output: %s", output)
	}

	stats := &MemoryStats{}

	// Parse memory line (second line, first is header)
	memFields := strings.Fields(lines[1])
	if len(memFields) < 7 {
		return nil, fmt.Errorf("unexpected free memory line format: %s", lines[1])
	}

	var err error
	stats.TotalBytes, err = strconv.ParseUint(memFields[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse total memory: %w", err)
	}

	stats.UsedBytes, err = strconv.ParseUint(memFields[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse used memory: %w", err)
	}

	stats.FreeBytes, err = strconv.ParseUint(memFields[3], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse free memory: %w", err)
	}

	// Available is typically the 7th field (index 6)
	if len(memFields) >= 7 {
		stats.AvailableBytes, err = strconv.ParseUint(memFields[6], 10, 64)
		if err != nil {
			stats.AvailableBytes = stats.FreeBytes
		}
	} else {
		stats.AvailableBytes = stats.FreeBytes
	}

	if stats.TotalBytes > 0 {
		stats.UsedPercent = float64(stats.UsedBytes) / float64(stats.TotalBytes) * 100.0
	}

	// Parse swap line if present (third line)
	if len(lines) >= 3 {
		swapFields := strings.Fields(lines[2])
		if len(swapFields) >= 4 && strings.HasPrefix(swapFields[0], "Swap") {
			stats.SwapTotalBytes, _ = strconv.ParseUint(swapFields[1], 10, 64)
			stats.SwapUsedBytes, _ = strconv.ParseUint(swapFields[2], 10, 64)

			if stats.SwapTotalBytes > 0 {
				stats.SwapUsedPercent = float64(stats.SwapUsedBytes) / float64(stats.SwapTotalBytes) * 100.0
			}
		}
	}

	return stats, nil
}

// getMemoryStatsMacOS retrieves memory stats on macOS using vm_stat
func (m *MemoryChecker) getMemoryStatsMacOS(ctx context.Context, executor diagnostics.CommandExecutor) (*MemoryStats, error) {
	// Get page size
	pageSizeOut, _, exitCode, err := executor.ExecuteWithContext(ctx, "pagesize 2>/dev/null || sysctl -n hw.pagesize")
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to get page size: %w", err)
	}

	pageSize, err := strconv.ParseUint(strings.TrimSpace(pageSizeOut), 10, 64)
	if err != nil {
		pageSize = 4096 // Default page size
	}

	// Get total memory
	totalOut, _, exitCode, err := executor.ExecuteWithContext(ctx, "sysctl -n hw.memsize")
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to get total memory: %w", err)
	}

	totalBytes, err := strconv.ParseUint(strings.TrimSpace(totalOut), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse total memory: %w", err)
	}

	// Get vm_stat output
	vmStatOut, _, exitCode, err := executor.ExecuteWithContext(ctx, "vm_stat")
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to get vm_stat: %w", err)
	}

	stats := &MemoryStats{
		TotalBytes: totalBytes,
	}

	// Parse vm_stat output
	lines := strings.Split(vmStatOut, "\n")
	values := make(map[string]uint64)

	for _, line := range lines {
		if !strings.Contains(line, ":") {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		valueStr := strings.TrimSpace(strings.TrimSuffix(parts[1], "."))
		value, err := strconv.ParseUint(valueStr, 10, 64)
		if err != nil {
			continue
		}

		values[key] = value * pageSize
	}

	// Calculate used memory (active + wired + compressed)
	active := values["Pages active"]
	wired := values["Pages wired down"]
	compressed := values["Pages occupied by compressor"]

	stats.UsedBytes = active + wired + compressed
	stats.FreeBytes = totalBytes - stats.UsedBytes
	stats.AvailableBytes = stats.FreeBytes

	if totalBytes > 0 {
		stats.UsedPercent = float64(stats.UsedBytes) / float64(totalBytes) * 100.0
	}

	// macOS swap info (approximate from vm_stat)
	swapUsed := values["Swapouts"]
	swapFree := values["Swapins"]

	stats.SwapUsedBytes = swapUsed
	stats.SwapTotalBytes = swapUsed + swapFree

	if stats.SwapTotalBytes > 0 {
		stats.SwapUsedPercent = float64(stats.SwapUsedBytes) / float64(stats.SwapTotalBytes) * 100.0
	}

	return stats, nil
}

// formatMessage creates a human-readable message from memory stats
func (m *MemoryChecker) formatMessage(stats *MemoryStats) string {
	totalGB := float64(stats.TotalBytes) / 1024 / 1024 / 1024
	usedGB := float64(stats.UsedBytes) / 1024 / 1024 / 1024
	availGB := float64(stats.AvailableBytes) / 1024 / 1024 / 1024

	msg := fmt.Sprintf("Memory: %.1f%% used (%.1f/%.1f GB), %.1f GB available",
		stats.UsedPercent, usedGB, totalGB, availGB)

	if stats.SwapTotalBytes > 0 {
		swapGB := float64(stats.SwapUsedBytes) / 1024 / 1024 / 1024
		msg += fmt.Sprintf(", Swap: %.1f%% (%.1f GB)", stats.SwapUsedPercent, swapGB)
	}

	return msg
}
