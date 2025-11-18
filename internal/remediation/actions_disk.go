package remediation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/sirupsen/logrus"
)

// CleanLogsAction cleans old log files to free disk space.
type CleanLogsAction struct {
	*BaseAction
	olderThanDays int
	dryRun        bool
}

// NewCleanLogsAction creates a new clean logs action.
func NewCleanLogsAction(olderThanDays int, dryRun bool, logger *logrus.Logger) *CleanLogsAction {
	return &CleanLogsAction{
		BaseAction: NewBaseAction(
			"disk.clean_logs",
			"Clean old log files",
			fmt.Sprintf("Removes log files older than %d days from /var/log to free disk space", olderThanDays),
			CategoryDisk,
			RiskSafe,
			false, // not reversible - files are deleted
			fmt.Sprintf("Old log files (>%d days) will be permanently deleted from /var/log", olderThanDays),
			logger,
		),
		olderThanDays: olderThanDays,
		dryRun:        dryRun,
	}
}

// NewCleanLogsActionFactory returns a factory for creating CleanLogsAction instances.
func NewCleanLogsActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		olderThanDays := 30 // default
		if days, ok := params["older_than_days"].(int); ok {
			olderThanDays = days
		}
		dryRun := false
		if dr, ok := params["dry_run"].(bool); ok {
			dryRun = dr
		}
		return NewCleanLogsAction(olderThanDays, dryRun, logger), nil
	}
}

// Validate checks prerequisites for log cleanup.
func (a *CleanLogsAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	// Check if /var/log exists and is accessible
	cmd := "test -d /var/log && test -r /var/log"
	_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("/var/log not accessible: %s", stderr)
	}

	return nil
}

// Execute cleans old log files.
func (a *CleanLogsAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	// Find old log files
	findCmd := fmt.Sprintf("find /var/log -type f -name '*.log' -mtime +%d", a.olderThanDays)
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, findCmd)
	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = "Failed to find old log files"
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("find command failed: %s", stderr)
	}

	files := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(files) == 1 && files[0] == "" {
		result.Status = StatusSuccess
		result.Message = "No old log files found to clean"
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, nil
	}

	// Get disk space before cleanup
	dfBefore, _ := a.getDiskUsage(ctx, executor)

	// Delete files
	if !a.dryRun {
		deleteCmd := fmt.Sprintf("find /var/log -type f -name '*.log' -mtime +%d -delete", a.olderThanDays)
		_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, deleteCmd)
		if err != nil || exitCode != 0 {
			result.Status = StatusFailed
			result.Message = "Failed to delete old log files"
			result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result, fmt.Errorf("delete command failed: %s", stderr)
		}
	}

	// Get disk space after cleanup
	dfAfter, _ := a.getDiskUsage(ctx, executor)

	result.Status = StatusSuccess
	if a.dryRun {
		result.Message = fmt.Sprintf("Would delete %d old log files (dry-run mode)", len(files))
	} else {
		result.Message = fmt.Sprintf("Successfully deleted %d old log files", len(files))
	}
	result.ChangesApplied = []string{
		fmt.Sprintf("Deleted %d files older than %d days", len(files), a.olderThanDays),
		fmt.Sprintf("Disk usage: %s → %s", dfBefore, dfAfter),
	}
	result.Output = fmt.Sprintf("Files cleaned:\n%s", strings.Join(files, "\n"))
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback is not supported for this action.
func (a *CleanLogsAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	return fmt.Errorf("log cleanup cannot be rolled back - files are permanently deleted")
}

// getDiskUsage returns current disk usage of /var/log.
func (a *CleanLogsAction) getDiskUsage(ctx context.Context, executor diagnostics.CommandExecutor) (string, error) {
	cmd := "df -h /var/log | tail -1 | awk '{print $5}'"
	stdout, _, _, _ := executor.ExecuteWithContext(ctx, cmd)
	return strings.TrimSpace(stdout), nil
}

// CleanTempAction cleans temporary files to free disk space.
type CleanTempAction struct {
	*BaseAction
	olderThanDays int
}

// NewCleanTempAction creates a new clean temp action.
func NewCleanTempAction(olderThanDays int, logger *logrus.Logger) *CleanTempAction {
	return &CleanTempAction{
		BaseAction: NewBaseAction(
			"disk.clean_temp",
			"Clean temporary files",
			fmt.Sprintf("Removes temporary files older than %d days from /tmp to free disk space", olderThanDays),
			CategoryDisk,
			RiskSafe,
			false,
			fmt.Sprintf("Old files (>%d days) will be deleted from /tmp", olderThanDays),
			logger,
		),
		olderThanDays: olderThanDays,
	}
}

// NewCleanTempActionFactory returns a factory for creating CleanTempAction instances.
func NewCleanTempActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		olderThanDays := 7 // default
		if days, ok := params["older_than_days"].(int); ok {
			olderThanDays = days
		}
		return NewCleanTempAction(olderThanDays, logger), nil
	}
}

// Validate checks prerequisites.
func (a *CleanTempAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	cmd := "test -d /tmp && test -w /tmp"
	_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("/tmp not accessible or writable: %s", stderr)
	}

	return nil
}

// Execute cleans temporary files.
func (a *CleanTempAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	// Find and count old temp files
	findCmd := fmt.Sprintf("find /tmp -type f -mtime +%d 2>/dev/null | wc -l", a.olderThanDays)
	stdout, _, _, _ := executor.ExecuteWithContext(ctx, findCmd)
	fileCount := strings.TrimSpace(stdout)

	// Delete old temp files
	deleteCmd := fmt.Sprintf("find /tmp -type f -mtime +%d -delete 2>/dev/null", a.olderThanDays)
	_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, deleteCmd)
	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = "Failed to clean temp files"
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("cleanup failed: %s", stderr)
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully cleaned %s temporary files", fileCount)
	result.ChangesApplied = []string{
		fmt.Sprintf("Deleted %s files older than %d days from /tmp", fileCount, a.olderThanDays),
	}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback is not supported.
func (a *CleanTempAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	return fmt.Errorf("temp cleanup cannot be rolled back")
}

// CleanCacheAction cleans user cache directories.
type CleanCacheAction struct {
	*BaseAction
}

// NewCleanCacheAction creates a new clean cache action.
func NewCleanCacheAction(logger *logrus.Logger) *CleanCacheAction {
	return &CleanCacheAction{
		BaseAction: NewBaseAction(
			"disk.clean_cache",
			"Clean user cache directories",
			"Removes cache files from ~/.cache to free disk space",
			CategoryDisk,
			RiskSafe,
			false,
			"User cache directories will be cleaned",
			logger,
		),
	}
}

// NewCleanCacheActionFactory returns a factory for creating CleanCacheAction instances.
func NewCleanCacheActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		return NewCleanCacheAction(logger), nil
	}
}

// Validate checks prerequisites.
func (a *CleanCacheAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}
	return nil
}

// Execute cleans cache directories.
func (a *CleanCacheAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	// Find cache directories
	cmd := "find ~/.cache -type f 2>/dev/null | wc -l"
	stdout, _, _, _ := executor.ExecuteWithContext(ctx, cmd)
	fileCount := strings.TrimSpace(stdout)

	// Clean cache
	cleanCmd := "rm -rf ~/.cache/* 2>/dev/null"
	_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cleanCmd)
	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = "Failed to clean cache"
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("cache cleanup failed: %s", stderr)
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully cleaned %s cache files", fileCount)
	result.ChangesApplied = []string{fmt.Sprintf("Deleted %s cache files from ~/.cache", fileCount)}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback is not supported.
func (a *CleanCacheAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	return fmt.Errorf("cache cleanup cannot be rolled back")
}

// CleanAptCacheAction cleans apt package cache (Debian/Ubuntu).
type CleanAptCacheAction struct {
	*BaseAction
}

// NewCleanAptCacheAction creates a new clean apt cache action.
func NewCleanAptCacheAction(logger *logrus.Logger) *CleanAptCacheAction {
	return &CleanAptCacheAction{
		BaseAction: NewBaseAction(
			"disk.clean_apt_cache",
			"Clean APT package cache",
			"Runs 'apt-get clean' to remove cached package files",
			CategoryDisk,
			RiskSafe,
			false,
			"APT package cache will be cleaned (packages can be re-downloaded if needed)",
			logger,
		),
	}
}

// NewCleanAptCacheActionFactory returns a factory for creating CleanAptCacheAction instances.
func NewCleanAptCacheActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		return NewCleanAptCacheAction(logger), nil
	}
}

// Validate checks if apt is available.
func (a *CleanAptCacheAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	// Check if apt-get is available
	cmd := "which apt-get"
	_, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("apt-get not available (not a Debian/Ubuntu system?): %s", stderr)
	}

	return nil
}

// Execute cleans apt cache.
func (a *CleanAptCacheAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	// Get cache size before
	beforeCmd := "du -sh /var/cache/apt/archives 2>/dev/null | awk '{print $1}'"
	beforeSize, _, _, _ := executor.ExecuteWithContext(ctx, beforeCmd)
	beforeSize = strings.TrimSpace(beforeSize)

	// Clean apt cache
	cmd := "apt-get clean"
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	result.Output = fmt.Sprintf("stdout: %s\nstderr: %s", stdout, stderr)

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = "Failed to clean APT cache"
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("apt-get clean failed: %s", stderr)
	}

	// Get cache size after
	afterCmd := "du -sh /var/cache/apt/archives 2>/dev/null | awk '{print $1}'"
	afterSize, _, _, _ := executor.ExecuteWithContext(ctx, afterCmd)
	afterSize = strings.TrimSpace(afterSize)

	result.Status = StatusSuccess
	result.Message = "Successfully cleaned APT package cache"
	result.ChangesApplied = []string{
		fmt.Sprintf("APT cache cleaned: %s → %s", beforeSize, afterSize),
	}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback is not supported.
func (a *CleanAptCacheAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	return fmt.Errorf("apt cache cleanup cannot be rolled back")
}
