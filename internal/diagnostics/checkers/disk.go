package checkers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// DiskChecker performs disk usage and inode checks
type DiskChecker struct {
	thresholds diagnostics.DiskThresholds
}

// NewDiskChecker creates a new disk checker
func NewDiskChecker(thresholds diagnostics.DiskThresholds) *DiskChecker {
	return &DiskChecker{
		thresholds: thresholds,
	}
}

// Name returns the checker name
func (d *DiskChecker) Name() string {
	return "disk_check"
}

// Category returns the checker category
func (d *DiskChecker) Category() diagnostics.CheckCategory {
	return diagnostics.CategoryDisk
}

// Description returns the checker description
func (d *DiskChecker) Description() string {
	return "Checks disk space and inode usage for all mounted filesystems"
}

// RequiresRoot returns false as disk checks don't need root
func (d *DiskChecker) RequiresRoot() bool {
	return false
}

// Run executes the disk check
func (d *DiskChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
	result := &diagnostics.CheckResult{
		Name:      d.Name(),
		Category:  d.Category(),
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
		Metrics:   []diagnostics.Metric{},
	}

	startTime := time.Now()

	// Get disk usage for all filesystems
	diskUsage, err := d.getDiskUsage(ctx, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk usage: %w", err)
	}

	// Get inode usage
	inodeUsage, err := d.getInodeUsage(ctx, executor)
	if err != nil {
		// Inode info might not be available on all systems, just warn
		result.SetData("inode_warning", fmt.Sprintf("Could not retrieve inode information: %v", err))
	}

	// Store filesystem data
	filesystems := make([]map[string]interface{}, 0, len(diskUsage))
	for _, fs := range diskUsage {
		fsData := map[string]interface{}{
			"filesystem":   fs.Filesystem,
			"mount_point":  fs.MountPoint,
			"total_bytes":  fs.TotalBytes,
			"used_bytes":   fs.UsedBytes,
			"avail_bytes":  fs.AvailBytes,
			"used_percent": fs.UsedPercent,
		}

		// Add inode data if available
		if inode, ok := inodeUsage[fs.MountPoint]; ok {
			fsData["inodes_total"] = inode.Total
			fsData["inodes_used"] = inode.Used
			fsData["inodes_free"] = inode.Free
			fsData["inodes_used_percent"] = inode.UsedPercent
		}

		filesystems = append(filesystems, fsData)

		// Add metrics for threshold evaluation (only for important filesystems)
		if d.shouldMonitorFilesystem(fs.MountPoint) {
			result.AddMetric(diagnostics.Metric{
				Name:          fmt.Sprintf("disk_usage_%s", sanitizeMountPoint(fs.MountPoint)),
				Value:         fs.UsedPercent,
				Unit:          "percent",
				Threshold:     d.thresholds.UsageWarn,
				ThresholdType: diagnostics.ThresholdTypeMax,
			})

			// Add inode metric if available
			if inode, ok := inodeUsage[fs.MountPoint]; ok {
				result.AddMetric(diagnostics.Metric{
					Name:          fmt.Sprintf("inode_usage_%s", sanitizeMountPoint(fs.MountPoint)),
					Value:         inode.UsedPercent,
					Unit:          "percent",
					Threshold:     d.thresholds.InodeWarn,
					ThresholdType: diagnostics.ThresholdTypeMax,
				})
			}
		}
	}

	result.SetData("filesystems", filesystems)
	result.SetData("filesystem_count", len(filesystems))
	result.Duration = time.Since(startTime)

	// Generate message
	result.Message = d.formatMessage(diskUsage, inodeUsage)

	return result, nil
}

// FilesystemUsage holds disk usage statistics for a filesystem
type FilesystemUsage struct {
	Filesystem  string
	MountPoint  string
	TotalBytes  uint64
	UsedBytes   uint64
	AvailBytes  uint64
	UsedPercent float64
}

// InodeUsage holds inode statistics for a filesystem
type InodeUsage struct {
	MountPoint  string
	Total       uint64
	Used        uint64
	Free        uint64
	UsedPercent float64
}

// getDiskUsage retrieves disk usage for all filesystems
func (d *DiskChecker) getDiskUsage(ctx context.Context, executor diagnostics.CommandExecutor) ([]FilesystemUsage, error) {
	// Use df -B1 for Linux (bytes), df -k for macOS (will convert from KB)
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "df -B1 2>/dev/null || df -k")
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to execute df command: %w", err)
	}

	return d.parseDfOutput(stdout)
}

// parseDfOutput parses df command output
func (d *DiskChecker) parseDfOutput(output string) ([]FilesystemUsage, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected df output: %s", output)
	}

	var filesystems []FilesystemUsage

	// Skip header line
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		filesystem := fields[0]
		mountPoint := fields[len(fields)-1]

		// Skip special filesystems
		if !d.shouldMonitorFilesystem(mountPoint) {
			continue
		}

		// Parse sizes (handle both byte and KB output)
		totalStr := fields[1]
		usedStr := fields[2]
		availStr := fields[3]
		usedPercentStr := strings.TrimSuffix(fields[4], "%")

		total, err := strconv.ParseUint(totalStr, 10, 64)
		if err != nil {
			continue
		}

		used, err := strconv.ParseUint(usedStr, 10, 64)
		if err != nil {
			continue
		}

		avail, err := strconv.ParseUint(availStr, 10, 64)
		if err != nil {
			continue
		}

		usedPercent, err := strconv.ParseFloat(usedPercentStr, 64)
		if err != nil {
			continue
		}

		// If values look small, they're probably in KB (macOS), convert to bytes
		if total < 1000000 {
			total *= 1024
			used *= 1024
			avail *= 1024
		}

		filesystems = append(filesystems, FilesystemUsage{
			Filesystem:  filesystem,
			MountPoint:  mountPoint,
			TotalBytes:  total,
			UsedBytes:   used,
			AvailBytes:  avail,
			UsedPercent: usedPercent,
		})
	}

	return filesystems, nil
}

// getInodeUsage retrieves inode usage for all filesystems
func (d *DiskChecker) getInodeUsage(ctx context.Context, executor diagnostics.CommandExecutor) (map[string]InodeUsage, error) {
	// df -i for inode information (not available on all systems)
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "df -i")
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to execute df -i command: %w", err)
	}

	return d.parseDfInodeOutput(stdout)
}

// parseDfInodeOutput parses df -i command output
func (d *DiskChecker) parseDfInodeOutput(output string) (map[string]InodeUsage, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected df -i output: %s", output)
	}

	inodeUsage := make(map[string]InodeUsage)

	// Skip header line
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		mountPoint := fields[len(fields)-1]

		// Skip special filesystems
		if !d.shouldMonitorFilesystem(mountPoint) {
			continue
		}

		// Parse inode counts
		totalStr := fields[1]
		usedStr := fields[2]
		freeStr := fields[3]
		usedPercentStr := strings.TrimSuffix(fields[4], "%")

		total, err := strconv.ParseUint(totalStr, 10, 64)
		if err != nil {
			continue
		}

		used, err := strconv.ParseUint(usedStr, 10, 64)
		if err != nil {
			continue
		}

		free, err := strconv.ParseUint(freeStr, 10, 64)
		if err != nil {
			continue
		}

		usedPercent, err := strconv.ParseFloat(usedPercentStr, 64)
		if err != nil {
			continue
		}

		inodeUsage[mountPoint] = InodeUsage{
			MountPoint:  mountPoint,
			Total:       total,
			Used:        used,
			Free:        free,
			UsedPercent: usedPercent,
		}
	}

	return inodeUsage, nil
}

// shouldMonitorFilesystem determines if a filesystem should be monitored
func (d *DiskChecker) shouldMonitorFilesystem(mountPoint string) bool {
	// Skip special/virtual filesystems
	skipPrefixes := []string{
		"/dev",
		"/sys",
		"/proc",
		"/run",
		"/snap",
		"/boot/efi",
	}

	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(mountPoint, prefix) && mountPoint != "/dev" {
			return false
		}
	}

	// Skip loop devices and tmpfs on non-root mounts
	if strings.Contains(mountPoint, "/loop") ||
		(strings.Contains(mountPoint, "tmpfs") && mountPoint != "/") {
		return false
	}

	return true
}

// sanitizeMountPoint converts a mount point to a metric-safe name
func sanitizeMountPoint(mountPoint string) string {
	// Replace / with root, and other special chars with underscores
	if mountPoint == "/" {
		return "root"
	}

	sanitized := strings.ReplaceAll(mountPoint, "/", "_")
	sanitized = strings.Trim(sanitized, "_")
	return sanitized
}

// formatMessage creates a human-readable message from disk usage
func (d *DiskChecker) formatMessage(diskUsage []FilesystemUsage, inodeUsage map[string]InodeUsage) string {
	if len(diskUsage) == 0 {
		return "No filesystems found"
	}

	var parts []string

	for _, fs := range diskUsage {
		totalGB := float64(fs.TotalBytes) / 1024 / 1024 / 1024
		availGB := float64(fs.AvailBytes) / 1024 / 1024 / 1024

		msg := fmt.Sprintf("%s: %.1f%% used (%.1f/%.1f GB free)",
			fs.MountPoint, fs.UsedPercent, availGB, totalGB)

		// Add inode info if available and concerning
		if inode, ok := inodeUsage[fs.MountPoint]; ok && inode.UsedPercent > 50 {
			msg += fmt.Sprintf(", inodes: %.1f%%", inode.UsedPercent)
		}

		parts = append(parts, msg)
	}

	// Limit to first 3 filesystems in message, add count if more
	if len(parts) > 3 {
		return fmt.Sprintf("%s (and %d more)", strings.Join(parts[:3], "; "), len(parts)-3)
	}

	return strings.Join(parts, "; ")
}
