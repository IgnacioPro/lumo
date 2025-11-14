package checkers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// CPUChecker performs CPU and load average checks
type CPUChecker struct {
	thresholds diagnostics.CPUThresholds
}

// NewCPUChecker creates a new CPU checker
func NewCPUChecker(thresholds diagnostics.CPUThresholds) *CPUChecker {
	return &CPUChecker{
		thresholds: thresholds,
	}
}

// Name returns the checker name
func (c *CPUChecker) Name() string {
	return "cpu_check"
}

// Category returns the checker category
func (c *CPUChecker) Category() diagnostics.CheckCategory {
	return diagnostics.CategoryCPU
}

// Description returns the checker description
func (c *CPUChecker) Description() string {
	return "Checks CPU usage and load average"
}

// RequiresRoot returns false as CPU checks don't need root
func (c *CPUChecker) RequiresRoot() bool {
	return false
}

// Run executes the CPU check
func (c *CPUChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
	result := &diagnostics.CheckResult{
		Name:      c.Name(),
		Category:  c.Category(),
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
		Metrics:   []diagnostics.Metric{},
	}

	startTime := time.Now()

	// Get CPU count
	cpuCount, err := c.getCPUCount(ctx, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU count: %w", err)
	}
	result.SetData("cpu_count", cpuCount)

	// Get load average
	load1, load5, load15, err := c.getLoadAverage(ctx, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to get load average: %w", err)
	}

	result.SetData("load_1min", load1)
	result.SetData("load_5min", load5)
	result.SetData("load_15min", load15)

	// Calculate load per CPU
	loadPerCPU := load1 / float64(cpuCount)
	result.SetData("load_per_cpu_1min", loadPerCPU)

	// Get CPU usage percentage
	cpuUsage, err := c.getCPUUsage(ctx, executor)
	if err != nil {
		// CPU usage might fail, but we can still report load
		cpuUsage = 0
	}
	result.SetData("cpu_percent", cpuUsage)

	// Add metrics for threshold evaluation
	if cpuUsage > 0 {
		result.AddMetric(diagnostics.Metric{
			Name:          "cpu_usage",
			Value:         cpuUsage,
			Unit:          "percent",
			Threshold:     c.thresholds.UsageWarn,
			ThresholdType: diagnostics.ThresholdTypeMax,
		})
	}

	result.AddMetric(diagnostics.Metric{
		Name:          "load_per_cpu_1min",
		Value:         loadPerCPU,
		Unit:          "load",
		Threshold:     c.thresholds.LoadWarn,
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.Duration = time.Since(startTime)

	// Generate message
	if cpuUsage > 0 {
		result.Message = fmt.Sprintf("CPU: %.1f%%, Load: %.2f (%.2f per CPU), %d cores",
			cpuUsage, load1, loadPerCPU, cpuCount)
	} else {
		result.Message = fmt.Sprintf("Load: %.2f (%.2f per CPU), %d cores",
			load1, loadPerCPU, cpuCount)
	}

	return result, nil
}

// getCPUCount returns the number of CPU cores
func (c *CPUChecker) getCPUCount(ctx context.Context, executor diagnostics.CommandExecutor) (int, error) {
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "nproc --all 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 1")
	if err != nil || exitCode != 0 {
		return 1, fmt.Errorf("failed to get CPU count: %w", err)
	}

	count, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return 1, fmt.Errorf("failed to parse CPU count: %w", err)
	}

	return count, nil
}

// getLoadAverage returns 1, 5, and 15 minute load averages
func (c *CPUChecker) getLoadAverage(ctx context.Context, executor diagnostics.CommandExecutor) (float64, float64, float64, error) {
	// Try Linux /proc/loadavg first
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "cat /proc/loadavg 2>/dev/null || uptime | awk -F'load average:' '{ print $2 }' | awk '{ print $1, $2, $3 }' | tr -d ','")
	if err != nil || exitCode != 0 {
		return 0, 0, 0, fmt.Errorf("failed to get load average: %w", err)
	}

	fields := strings.Fields(strings.TrimSpace(stdout))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected load average format: %s", stdout)
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse 1min load: %w", err)
	}

	load5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse 5min load: %w", err)
	}

	load15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse 15min load: %w", err)
	}

	return load1, load5, load15, nil
}

// getCPUUsage returns current CPU usage percentage
func (c *CPUChecker) getCPUUsage(ctx context.Context, executor diagnostics.CommandExecutor) (float64, error) {
	// Use top command to get CPU usage
	// Linux: top -bn2 gives two iterations, we take the second one
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "top -bn2 -d 0.5 | grep '^%Cpu' | tail -1 | awk '{print $2}' | tr -d 'us,' 2>/dev/null || top -l 2 -n 0 -s 0.5 | grep 'CPU usage' | tail -1 | awk '{print $3}' | tr -d '%'")
	if err != nil || exitCode != 0 {
		// CPU usage extraction failed, return 0 (not critical)
		return 0, nil
	}

	usage, err := strconv.ParseFloat(strings.TrimSpace(stdout), 64)
	if err != nil {
		return 0, nil
	}

	return usage, nil
}
