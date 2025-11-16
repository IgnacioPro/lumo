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

	// Page fault tracking
	result.SetData("page_faults", memStats.PageFaults)
	result.SetData("major_page_faults", memStats.MajorPageFaults)
	result.SetData("minor_page_faults", memStats.MinorPageFaults)

	// Memory pressure indicators
	if memStats.PagesThrottled > 0 {
		result.SetData("pages_throttled", memStats.PagesThrottled)
	}
	if memStats.Compressions > 0 {
		result.SetData("compressions", memStats.Compressions)
		result.SetData("decompressions", memStats.Decompressions)
		result.SetData("compressed_pages", memStats.CompressedPages)
	}
	if memStats.Swapins > 0 || memStats.Swapouts > 0 {
		result.SetData("swapins", memStats.Swapins)
		result.SetData("swapouts", memStats.Swapouts)
	}
	if memStats.PagesPurged > 0 {
		result.SetData("pages_purged", memStats.PagesPurged)
	}

	// Additional metrics
	if memStats.FileBackedPages > 0 || memStats.AnonPages > 0 {
		result.SetData("file_backed_pages", memStats.FileBackedPages)
		result.SetData("anonymous_pages", memStats.AnonPages)
	}
	if memStats.ActiveBytes > 0 {
		result.SetData("active_bytes", memStats.ActiveBytes)
		result.SetData("inactive_bytes", memStats.InactiveBytes)
		result.SetData("wired_bytes", memStats.WiredBytes)
	}

	// Get top memory consumers
	topConsumers, err := m.getTopMemoryConsumers(ctx, executor, memStats.TotalBytes)
	if err != nil {
		// Log warning but don't fail the check
		result.SetData("top_consumers_warning", fmt.Sprintf("Failed to get top consumers: %v", err))
		result.SetData("top_consumers", []MemoryConsumer{})
	} else {
		memStats.TopConsumers = topConsumers
		result.SetData("top_consumers", topConsumers)
	}

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
	TopConsumers    []MemoryConsumer

	// Page fault tracking
	PageFaults      uint64 // Total page faults (Linux: pgfault, macOS: Translation faults)
	MajorPageFaults uint64 // Major page faults requiring disk I/O (Linux: pgmajfault, macOS: Pageins)
	MinorPageFaults uint64 // Minor page faults (calculated: PageFaults - MajorPageFaults)

	// Memory pressure indicators
	PagesThrottled     uint64 // Processes throttled due to memory pressure (macOS only)
	Compressions       uint64 // Memory compression events (macOS only)
	Decompressions     uint64 // Memory decompression events (macOS only)
	CompressedPages    uint64 // Pages stored in compressor (macOS only)
	Swapins            uint64 // Swap-in operations
	Swapouts           uint64 // Swap-out operations
	PagesPurged        uint64 // Pages purged from memory (macOS only)

	// Additional metrics
	FileBackedPages uint64 // File-backed pages (macOS only)
	AnonPages       uint64 // Anonymous (non-file-backed) pages (macOS only)
	ActiveBytes     uint64 // Active memory bytes
	InactiveBytes   uint64 // Inactive memory bytes
	WiredBytes      uint64 // Wired (kernel) memory bytes (macOS) or Unevictable (Linux)
}

// MemoryConsumer represents a process consuming memory
type MemoryConsumer struct {
	PID         int     `json:"pid"`
	Command     string  `json:"command"`
	MemoryBytes uint64  `json:"memory_bytes"`
	MemoryMB    float64 `json:"memory_mb"`
	Percent     float64 `json:"percent"`
}

// getMemoryStats retrieves memory statistics from the system
func (m *MemoryChecker) getMemoryStats(ctx context.Context, executor diagnostics.CommandExecutor) (*MemoryStats, error) {
	// Try Linux /proc/meminfo first, then fall back to free command, then macOS vm_stat
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "cat /proc/meminfo 2>/dev/null || free -b 2>/dev/null")
	if err != nil || exitCode != 0 {
		// Try macOS
		return m.getMemoryStatsMacOS(ctx, executor)
	}

	var stats *MemoryStats
	// Check if output looks like /proc/meminfo (contains colons)
	if strings.Contains(stdout, ":") {
		stats, err = m.parseMeminfo(stdout)
	} else {
		// Otherwise assume it's free command output
		stats, err = m.parseFreeCommand(stdout)
	}

	if err != nil {
		return nil, err
	}

	// On Linux, also try to get page fault statistics from /proc/vmstat
	vmstatOut, _, exitCode, _ := executor.ExecuteWithContext(ctx, "cat /proc/vmstat 2>/dev/null")
	if exitCode == 0 && vmstatOut != "" {
		m.parseVMStat(vmstatOut, stats)
	}

	return stats, nil
}

// parseVMStat parses /proc/vmstat output and adds page fault statistics to stats
func (m *MemoryChecker) parseVMStat(output string, stats *MemoryStats) {
	lines := strings.Split(output, "\n")
	values := make(map[string]uint64)

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := fields[0]
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}

		values[key] = value
	}

	// Page fault tracking
	stats.PageFaults = values["pgfault"]
	stats.MajorPageFaults = values["pgmajfault"]
	if stats.PageFaults > stats.MajorPageFaults {
		stats.MinorPageFaults = stats.PageFaults - stats.MajorPageFaults
	}

	// Memory pressure indicators (Linux-specific)
	stats.Swapins = values["pswpin"]
	stats.Swapouts = values["pswpout"]

	// Additional Linux memory stats
	// Note: These are cumulative counters, not current state
	// but useful for understanding memory activity patterns
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

	// Additional memory breakdown (Linux)
	stats.ActiveBytes = values["Active"]
	stats.InactiveBytes = values["Inactive"]
	stats.WiredBytes = values["Unevictable"] // Closest Linux equivalent to macOS wired memory
	stats.AnonPages = values["AnonPages"]
	stats.FileBackedPages = values["Mapped"] // Mapped pages are file-backed

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
	rawValues := make(map[string]uint64) // Store raw values (page counts) for some metrics

	for _, line := range lines {
		if !strings.Contains(line, ":") {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		// Remove quotes and periods from key
		key = strings.Trim(key, "\"")
		valueStr := strings.TrimSpace(strings.TrimSuffix(parts[1], "."))
		value, err := strconv.ParseUint(valueStr, 10, 64)
		if err != nil {
			continue
		}

		rawValues[key] = value          // Store raw page count
		values[key] = value * pageSize  // Store byte value
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

	// Page fault tracking
	stats.PageFaults = rawValues["Translation faults"]
	stats.MajorPageFaults = rawValues["Pageins"] // Pageins require disk I/O
	if stats.PageFaults > stats.MajorPageFaults {
		stats.MinorPageFaults = stats.PageFaults - stats.MajorPageFaults
	}

	// Memory pressure indicators
	stats.PagesThrottled = rawValues["Pages throttled"]
	stats.Compressions = rawValues["Compressions"]
	stats.Decompressions = rawValues["Decompressions"]
	stats.CompressedPages = rawValues["Pages stored in compressor"]
	stats.Swapins = rawValues["Swapins"]
	stats.Swapouts = rawValues["Swapouts"]
	stats.PagesPurged = rawValues["Pages purged"]

	// Additional metrics
	stats.FileBackedPages = rawValues["File-backed pages"]
	stats.AnonPages = rawValues["Anonymous pages"]
	stats.ActiveBytes = values["Pages active"]
	stats.InactiveBytes = values["Pages inactive"]
	stats.WiredBytes = values["Pages wired down"]

	return stats, nil
}

// Constants for ps output parsing
const (
	// PS field positions (0-indexed)
	psFieldPID     = 1  // Process ID
	psFieldCPU     = 2  // CPU percentage
	psFieldMem     = 3  // Memory percentage
	psFieldVSZ     = 4  // Virtual memory size
	psFieldRSS     = 5  // Resident set size
	psFieldCommand = 10 // Command line (fields 10+)

	// Parsing constraints
	psMinFields      = 11                // Minimum fields required for valid ps output
	maxCommandLength = 60                // Maximum command string length
	maxConsumers     = 10                // Maximum number of consumers to return
	rssKBThreshold   = 100 * 1024 * 1024 // 100GB threshold to detect KB vs bytes
)

// getTopMemoryConsumers retrieves top memory-consuming processes
func (m *MemoryChecker) getTopMemoryConsumers(ctx context.Context, executor diagnostics.CommandExecutor, totalBytes uint64) ([]MemoryConsumer, error) {
	// Try Linux-style sorting first, fall back to macOS if needed
	psOutput, err := m.executeProcessList(ctx, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to get process list: %w", err)
	}

	return m.parseProcessList(psOutput), nil
}

// executeProcessList executes platform-appropriate ps command
func (m *MemoryChecker) executeProcessList(ctx context.Context, executor diagnostics.CommandExecutor) (string, error) {
	// First try Linux-style sorting
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "ps aux --sort=-%mem")
	if exitCode == 0 && strings.TrimSpace(stdout) != "" {
		return stdout, nil
	}

	// Fall back to macOS-style sorting
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "ps aux -m")
	if err != nil {
		return "", fmt.Errorf("ps command failed: %w", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("ps command failed with exit code: %d", exitCode)
	}
	if strings.TrimSpace(stdout) == "" {
		return "", fmt.Errorf("ps command returned empty output")
	}

	return stdout, nil
}

// parseProcessList parses ps command output into MemoryConsumer structs
func (m *MemoryChecker) parseProcessList(output string) []MemoryConsumer {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	consumers := []MemoryConsumer{}
	processCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header lines
		if m.isHeaderLine(line) {
			continue
		}

		// Stop after collecting enough processes
		if processCount >= maxConsumers {
			break
		}

		consumer, ok := m.parseProcessLine(line)
		if ok {
			consumers = append(consumers, consumer)
			processCount++
		}
	}

	return consumers
}

// isHeaderLine checks if a line is a ps output header
func (m *MemoryChecker) isHeaderLine(line string) bool {
	return strings.Contains(line, "USER") && strings.Contains(line, "%CPU")
}

// parseProcessLine parses a single line of ps output
func (m *MemoryChecker) parseProcessLine(line string) (MemoryConsumer, bool) {
	fields := strings.Fields(line)
	if len(fields) < psMinFields {
		return MemoryConsumer{}, false
	}

	// Parse and validate PID
	pid, err := strconv.Atoi(fields[psFieldPID])
	if err != nil || pid <= 0 {
		return MemoryConsumer{}, false
	}

	// Parse and validate memory percentage
	percent, err := strconv.ParseFloat(fields[psFieldMem], 64)
	if err != nil || percent < 0 || percent > 100 {
		return MemoryConsumer{}, false
	}

	// Parse and validate RSS
	rssValue, err := strconv.ParseUint(fields[psFieldRSS], 10, 64)
	if err != nil {
		return MemoryConsumer{}, false
	}

	// Convert RSS to bytes (handle KB vs bytes detection)
	memoryBytes := m.convertRSSToBytes(rssValue)
	memoryMB := float64(memoryBytes) / (1024 * 1024)

	// Extract and sanitize command
	command := m.sanitizeCommand(strings.Join(fields[psFieldCommand:], " "))

	return MemoryConsumer{
		PID:         pid,
		Command:     command,
		MemoryBytes: memoryBytes,
		MemoryMB:    memoryMB,
		Percent:     percent,
	}, true
}

// convertRSSToBytes converts RSS value to bytes, handling KB vs bytes detection
func (m *MemoryChecker) convertRSSToBytes(rssValue uint64) uint64 {
	// If the value is very large (>100GB in KB), it's likely already in bytes
	if rssValue > rssKBThreshold {
		return rssValue
	}
	// Otherwise assume it's in KB and convert to bytes
	return rssValue * 1024
}

// sanitizeCommand sanitizes and truncates process command line
func (m *MemoryChecker) sanitizeCommand(command string) string {
	// Remove any potentially dangerous characters (though this is already safe since
	// we're not executing this string, just displaying it)
	command = strings.ReplaceAll(command, "\x00", "") // Remove null bytes
	command = strings.ReplaceAll(command, "\n", " ")  // Replace newlines with spaces
	command = strings.ReplaceAll(command, "\r", " ")  // Replace carriage returns with spaces

	// Truncate if too long
	if len(command) > maxCommandLength {
		return command[:maxCommandLength-3] + "..."
	}

	return command
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

	// Add top consumer info
	if len(stats.TopConsumers) > 0 {
		top := stats.TopConsumers[0]
		msg += fmt.Sprintf(", Top: %s (%.0f MB)", top.Command, top.MemoryMB)
	}

	// Add memory pressure indicators if significant
	pressureWarnings := []string{}

	if stats.PagesThrottled > 0 {
		pressureWarnings = append(pressureWarnings, fmt.Sprintf("%d pages throttled", stats.PagesThrottled))
	}

	if stats.CompressedPages > 1000 { // Only show if significant compression
		compressedMB := float64(stats.CompressedPages*16384) / 1024 / 1024 // Assuming 16KB page size
		pressureWarnings = append(pressureWarnings, fmt.Sprintf("%.0f MB compressed", compressedMB))
	}

	if stats.Swapouts > 10000 { // Only show if significant swap activity
		pressureWarnings = append(pressureWarnings, fmt.Sprintf("%d swapouts", stats.Swapouts))
	}

	if len(pressureWarnings) > 0 {
		msg += fmt.Sprintf(" [Pressure: %s]", strings.Join(pressureWarnings, ", "))
	}

	return msg
}
