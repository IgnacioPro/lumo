package checkers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// ProcessChecker performs process-related checks
type ProcessChecker struct {
	thresholds diagnostics.ProcessThresholds
}

// NewProcessChecker creates a new process checker
func NewProcessChecker(thresholds diagnostics.ProcessThresholds) *ProcessChecker {
	return &ProcessChecker{
		thresholds: thresholds,
	}
}

// Name returns the checker name
func (p *ProcessChecker) Name() string {
	return "process_check"
}

// Category returns the checker category
func (p *ProcessChecker) Category() diagnostics.CheckCategory {
	return diagnostics.CategoryProcess
}

// Description returns the checker description
func (p *ProcessChecker) Description() string {
	return "Checks running processes, zombies, and resource consumption"
}

// RequiresRoot returns false as most process checks don't need root
func (p *ProcessChecker) RequiresRoot() bool {
	return false
}

// Run executes the process check
func (p *ProcessChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
	result := &diagnostics.CheckResult{
		Name:      p.Name(),
		Category:  p.Category(),
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
		Metrics:   []diagnostics.Metric{},
	}

	startTime := time.Now()

	// Get process counts
	processCounts, err := p.getProcessCounts(ctx, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to get process counts: %w", err)
	}

	// Get top CPU consumers
	topCPU, err := p.getTopCPUProcesses(ctx, executor, 5)
	if err != nil {
		// Non-fatal, just log
		result.SetData("cpu_warning", fmt.Sprintf("Could not retrieve CPU consumers: %v", err))
	} else {
		result.SetData("top_cpu_processes", topCPU)
	}

	// Get top memory consumers
	topMem, err := p.getTopMemoryProcesses(ctx, executor, 5)
	if err != nil {
		// Non-fatal, just log
		result.SetData("memory_warning", fmt.Sprintf("Could not retrieve memory consumers: %v", err))
	} else {
		result.SetData("top_memory_processes", topMem)
	}

	// Store process data
	result.SetData("total_processes", processCounts.Total)
	result.SetData("running_processes", processCounts.Running)
	result.SetData("sleeping_processes", processCounts.Sleeping)
	result.SetData("zombie_processes", processCounts.Zombie)

	// Add metrics for threshold evaluation
	result.AddMetric(diagnostics.Metric{
		Name:          "total_processes",
		Value:         float64(processCounts.Total),
		Unit:          "count",
		Threshold:     float64(p.thresholds.TotalWarn),
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.AddMetric(diagnostics.Metric{
		Name:          "zombie_processes",
		Value:         float64(processCounts.Zombie),
		Unit:          "count",
		Threshold:     float64(p.thresholds.ZombieWarn),
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.Duration = time.Since(startTime)

	// Generate message
	result.Message = p.formatMessage(processCounts, topCPU, topMem)

	return result, nil
}

// ProcessCounts holds process count statistics
type ProcessCounts struct {
	Total    int
	Running  int
	Sleeping int
	Zombie   int
	Stopped  int
}

// ProcessInfo holds information about a single process
type ProcessInfo struct {
	PID     int
	User    string
	CPU     float64
	Memory  float64
	Command string
}

// getProcessCounts retrieves process counts by state
func (p *ProcessChecker) getProcessCounts(ctx context.Context, executor diagnostics.CommandExecutor) (*ProcessCounts, error) {
	// Use ps to get process states
	// Linux/macOS compatible: ps -eo state --no-headers
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "ps -eo state --no-headers 2>/dev/null || ps -eo state")
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to execute ps command: %w", err)
	}

	return p.parseProcessStates(stdout)
}

// parseProcessStates parses ps state output
func (p *ProcessChecker) parseProcessStates(output string) (*ProcessCounts, error) {
	counts := &ProcessCounts{}
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		state := strings.TrimSpace(line)
		if state == "" {
			continue
		}

		// Get first character of state (main state indicator)
		stateChar := rune(state[0])

		counts.Total++

		switch stateChar {
		case 'R': // Running
			counts.Running++
		case 'S', 'I': // Sleeping (interruptible or idle)
			counts.Sleeping++
		case 'Z': // Zombie
			counts.Zombie++
		case 'T', 't': // Stopped
			counts.Stopped++
		case 'D': // Uninterruptible sleep (usually I/O)
			counts.Sleeping++
		}
	}

	return counts, nil
}

// getTopCPUProcesses retrieves top N CPU-consuming processes
func (p *ProcessChecker) getTopCPUProcesses(ctx context.Context, executor diagnostics.CommandExecutor, limit int) ([]ProcessInfo, error) {
	// Validate limit to prevent abuse (1-10000 is reasonable)
	if limit < 1 || limit > 10000 {
		return nil, fmt.Errorf("invalid limit: must be between 1 and 10000, got %d", limit)
	}

	// ps command to get CPU usage
	// Format: PID USER %CPU COMMAND
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx,
		"ps -eo pid,user,%cpu,comm --sort=-%cpu --no-headers 2>/dev/null | head -n "+strconv.Itoa(limit)+" || ps -eo pid,user,%cpu,comm -r | head -n "+strconv.Itoa(limit+1))
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to execute ps command: %w", err)
	}

	return p.parseProcessList(stdout, "cpu")
}

// getTopMemoryProcesses retrieves top N memory-consuming processes
func (p *ProcessChecker) getTopMemoryProcesses(ctx context.Context, executor diagnostics.CommandExecutor, limit int) ([]ProcessInfo, error) {
	// Validate limit to prevent abuse (1-10000 is reasonable)
	if limit < 1 || limit > 10000 {
		return nil, fmt.Errorf("invalid limit: must be between 1 and 10000, got %d", limit)
	}

	// ps command to get memory usage
	// Format: PID USER %MEM COMMAND
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx,
		"ps -eo pid,user,%mem,comm --sort=-%mem --no-headers 2>/dev/null | head -n "+strconv.Itoa(limit)+" || ps -eo pid,user,%mem,comm -m | head -n "+strconv.Itoa(limit+1))
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to execute ps command: %w", err)
	}

	return p.parseProcessList(stdout, "memory")
}

// parseProcessList parses ps output for process information
func (p *ProcessChecker) parseProcessList(output, metricType string) ([]ProcessInfo, error) {
	var processes []ProcessInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// Skip header line if present (macOS)
		if fields[0] == "PID" {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		user := fields[1]

		// Parse percentage (CPU or Memory)
		percent, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			continue
		}

		// Command is rest of fields joined
		command := strings.Join(fields[3:], " ")

		proc := ProcessInfo{
			PID:     pid,
			User:    user,
			Command: command,
		}

		if metricType == "cpu" {
			proc.CPU = percent
		} else {
			proc.Memory = percent
		}

		processes = append(processes, proc)
	}

	// Sort by the metric (descending)
	if metricType == "cpu" {
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].CPU > processes[j].CPU
		})
	} else {
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].Memory > processes[j].Memory
		})
	}

	return processes, nil
}

// formatMessage creates a human-readable message from process data
func (p *ProcessChecker) formatMessage(counts *ProcessCounts, topCPU, topMem []ProcessInfo) string {
	var parts []string

	// Total processes
	parts = append(parts, fmt.Sprintf("%d total processes (%d running, %d sleeping)",
		counts.Total, counts.Running, counts.Sleeping))

	// Zombie warning
	if counts.Zombie > 0 {
		parts = append(parts, fmt.Sprintf("%d zombie processes", counts.Zombie))
	}

	// Top CPU consumer
	if len(topCPU) > 0 && topCPU[0].CPU > 0 {
		parts = append(parts, fmt.Sprintf("Top CPU: %s (%.1f%%)",
			truncateCommand(topCPU[0].Command, 30), topCPU[0].CPU))
	}

	// Top memory consumer
	if len(topMem) > 0 && topMem[0].Memory > 0 {
		parts = append(parts, fmt.Sprintf("Top Memory: %s (%.1f%%)",
			truncateCommand(topMem[0].Command, 30), topMem[0].Memory))
	}

	return strings.Join(parts, ", ")
}

// truncateCommand truncates a command string to a maximum length
func truncateCommand(cmd string, maxLen int) string {
	if len(cmd) <= maxLen {
		return cmd
	}
	return cmd[:maxLen-3] + "..."
}
