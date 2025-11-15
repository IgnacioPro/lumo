package checkers

import (
	"context"
	"strings"
	"testing"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestNewMemoryChecker(t *testing.T) {
	thresholds := diagnostics.MemoryThresholds{
		UsageWarn:     85.0,
		UsageCritical: 95.0,
	}

	checker := NewMemoryChecker(thresholds)
	if checker == nil {
		t.Fatal("NewMemoryChecker returned nil")
	}
	if checker.thresholds.UsageWarn != 85.0 {
		t.Errorf("Expected UsageWarn 85.0, got %v", checker.thresholds.UsageWarn)
	}
}

func TestMemoryChecker_Name(t *testing.T) {
	checker := NewMemoryChecker(diagnostics.MemoryThresholds{})
	if got := checker.Name(); got != "memory_check" {
		t.Errorf("Name() = %v, want memory_check", got)
	}
}

func TestMemoryChecker_Category(t *testing.T) {
	checker := NewMemoryChecker(diagnostics.MemoryThresholds{})
	if got := checker.Category(); got != diagnostics.CategoryMemory {
		t.Errorf("Category() = %v, want %v", got, diagnostics.CategoryMemory)
	}
}

func TestMemoryChecker_Description(t *testing.T) {
	checker := NewMemoryChecker(diagnostics.MemoryThresholds{})
	desc := checker.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if !strings.Contains(strings.ToLower(desc), "memory") {
		t.Error("Description should mention memory")
	}
}

func TestMemoryChecker_RequiresRoot(t *testing.T) {
	checker := NewMemoryChecker(diagnostics.MemoryThresholds{})
	if checker.RequiresRoot() {
		t.Error("MemoryChecker should not require root")
	}
}

func TestMemoryChecker_ParseMeminfo(t *testing.T) {
	meminfoOutput := `MemTotal:       16384000 kB
MemFree:         8192000 kB
MemAvailable:   10240000 kB
Buffers:          512000 kB
Cached:          2048000 kB
SwapTotal:       8192000 kB
SwapFree:        6144000 kB`

	checker := NewMemoryChecker(diagnostics.MemoryThresholds{})
	stats, err := checker.parseMeminfo(meminfoOutput)

	if err != nil {
		t.Fatalf("parseMeminfo() error = %v", err)
	}

	// Check total bytes (16384000 KB = 16384000 * 1024 bytes)
	expectedTotal := uint64(16384000 * 1024)
	if stats.TotalBytes != expectedTotal {
		t.Errorf("TotalBytes = %v, want %v", stats.TotalBytes, expectedTotal)
	}

	// Check free bytes
	expectedFree := uint64(8192000 * 1024)
	if stats.FreeBytes != expectedFree {
		t.Errorf("FreeBytes = %v, want %v", stats.FreeBytes, expectedFree)
	}

	// Check available bytes
	expectedAvailable := uint64(10240000 * 1024)
	if stats.AvailableBytes != expectedAvailable {
		t.Errorf("AvailableBytes = %v, want %v", stats.AvailableBytes, expectedAvailable)
	}

	// Check used bytes (TotalBytes - AvailableBytes)
	expectedUsed := expectedTotal - expectedAvailable
	if stats.UsedBytes != expectedUsed {
		t.Errorf("UsedBytes = %v, want %v", stats.UsedBytes, expectedUsed)
	}

	// Check usage percent
	expectedPercent := float64(expectedUsed) / float64(expectedTotal) * 100.0
	if stats.UsedPercent != expectedPercent {
		t.Errorf("UsedPercent = %v, want %v", stats.UsedPercent, expectedPercent)
	}

	// Check swap
	expectedSwapTotal := uint64(8192000 * 1024)
	if stats.SwapTotalBytes != expectedSwapTotal {
		t.Errorf("SwapTotalBytes = %v, want %v", stats.SwapTotalBytes, expectedSwapTotal)
	}

	expectedSwapUsed := uint64(2048000 * 1024) // 8192000 - 6144000
	if stats.SwapUsedBytes != expectedSwapUsed {
		t.Errorf("SwapUsedBytes = %v, want %v", stats.SwapUsedBytes, expectedSwapUsed)
	}
}

func TestMemoryChecker_ParseMeminfo_OldKernel(t *testing.T) {
	// Old kernels without MemAvailable
	meminfoOutput := `MemTotal:       16384000 kB
MemFree:         8192000 kB
Buffers:          512000 kB
Cached:          2048000 kB
SwapTotal:       8192000 kB
SwapFree:        6144000 kB`

	checker := NewMemoryChecker(diagnostics.MemoryThresholds{})
	stats, err := checker.parseMeminfo(meminfoOutput)

	if err != nil {
		t.Fatalf("parseMeminfo() error = %v", err)
	}

	// MemAvailable should be estimated as MemFree + Buffers + Cached
	expectedAvailable := uint64((8192000 + 512000 + 2048000) * 1024)
	if stats.AvailableBytes != expectedAvailable {
		t.Errorf("AvailableBytes = %v, want %v (should estimate from MemFree+Buffers+Cached)",
			stats.AvailableBytes, expectedAvailable)
	}
}

func TestMemoryChecker_ParseFreeCommand(t *testing.T) {
	freeOutput := `              total        used        free      shared  buff/cache   available
Mem:       16777216     6291456     8388608      102400     2097152    10485760
Swap:       8388608     2097152     6291456`

	checker := NewMemoryChecker(diagnostics.MemoryThresholds{})
	stats, err := checker.parseFreeCommand(freeOutput)

	if err != nil {
		t.Fatalf("parseFreeCommand() error = %v", err)
	}

	if stats.TotalBytes != 16777216 {
		t.Errorf("TotalBytes = %v, want 16777216", stats.TotalBytes)
	}

	if stats.UsedBytes != 6291456 {
		t.Errorf("UsedBytes = %v, want 6291456", stats.UsedBytes)
	}

	if stats.FreeBytes != 8388608 {
		t.Errorf("FreeBytes = %v, want 8388608", stats.FreeBytes)
	}

	if stats.AvailableBytes != 10485760 {
		t.Errorf("AvailableBytes = %v, want 10485760", stats.AvailableBytes)
	}

	// Check swap
	if stats.SwapTotalBytes != 8388608 {
		t.Errorf("SwapTotalBytes = %v, want 8388608", stats.SwapTotalBytes)
	}

	if stats.SwapUsedBytes != 2097152 {
		t.Errorf("SwapUsedBytes = %v, want 2097152", stats.SwapUsedBytes)
	}

	// Check percentages are calculated
	if stats.UsedPercent == 0 {
		t.Error("UsedPercent should be calculated")
	}

	if stats.SwapUsedPercent == 0 {
		t.Error("SwapUsedPercent should be calculated")
	}
}

func TestMemoryChecker_Run(t *testing.T) {
	thresholds := diagnostics.MemoryThresholds{
		UsageWarn:     85.0,
		UsageCritical: 95.0,
		SwapWarn:      25.0,
		SwapCritical:  75.0,
	}

	meminfoOutput := `MemTotal:       16384000 kB
MemFree:         8192000 kB
MemAvailable:   10240000 kB
SwapTotal:       8192000 kB
SwapFree:        6144000 kB`

	executor := &mockExecutor{
		responses: map[string]mockResponse{
			"meminfo": {
				stdout:   meminfoOutput,
				exitCode: 0,
			},
		},
	}

	checker := NewMemoryChecker(thresholds)
	result, err := checker.Run(context.Background(), executor)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Name != "memory_check" {
		t.Errorf("Result.Name = %v, want memory_check", result.Name)
	}

	if result.Category != diagnostics.CategoryMemory {
		t.Errorf("Result.Category = %v, want %v", result.Category, diagnostics.CategoryMemory)
	}

	if result.Status != diagnostics.StatusCompleted {
		t.Errorf("Result.Status = %v, want %v", result.Status, diagnostics.StatusCompleted)
	}

	// Check that metrics are present
	if len(result.Metrics) == 0 {
		t.Error("Expected metrics to be present")
	}

	// Check message is generated
	if result.Message == "" {
		t.Error("Expected non-empty message")
	}
}

func TestMemoryChecker_FormatMessage(t *testing.T) {
	checker := NewMemoryChecker(diagnostics.MemoryThresholds{})

	stats := &MemoryStats{
		TotalBytes:      16 * 1024 * 1024 * 1024, // 16 GB
		UsedBytes:       8 * 1024 * 1024 * 1024,  // 8 GB
		AvailableBytes:  8 * 1024 * 1024 * 1024,  // 8 GB
		UsedPercent:     50.0,
		SwapTotalBytes:  4 * 1024 * 1024 * 1024, // 4 GB
		SwapUsedBytes:   1 * 1024 * 1024 * 1024, // 1 GB
		SwapUsedPercent: 25.0,
	}

	message := checker.formatMessage(stats)

	// Check message contains key information
	if !strings.Contains(message, "Memory:") {
		t.Error("Message should contain 'Memory:'")
	}

	if !strings.Contains(message, "50.0%") {
		t.Error("Message should contain usage percentage")
	}

	if !strings.Contains(message, "Swap:") {
		t.Error("Message should contain swap information")
	}

	if !strings.Contains(message, "25.0%") {
		t.Error("Message should contain swap percentage")
	}
}
