package remediation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// CleanTmpFilesAction deletes temporary files older than a specified duration.
type CleanTmpFilesAction struct {
	*BaseAction
	targetDir  string
	maxAgeDays int
	dryRun     bool // For validation/preview purposes
}

// NewCleanTmpFilesAction creates a new CleanTmpFilesAction.
func NewCleanTmpFilesAction(targetDir string, maxAgeDays int, dryRun bool, logger *logrus.Logger) *CleanTmpFilesAction {
	if targetDir == "" {
		targetDir = "/tmp" // Default to /tmp if not specified
	}
	if maxAgeDays <= 0 {
		maxAgeDays = 7 // Default to 7 days if not specified or invalid
	}

	actionID := fmt.Sprintf("disk.clean_tmp.%s.%d", strings.ReplaceAll(targetDir, "/", "_"), maxAgeDays)
	name := fmt.Sprintf("Clean temporary files in %s older than %d days", targetDir, maxAgeDays)
	description := fmt.Sprintf("Deletes files from '%s' that have not been accessed or modified in the last %d days. This helps free up disk space.", targetDir, maxAgeDays)
	impact := fmt.Sprintf("Frees up disk space in '%s'. May affect applications relying on long-lived temporary files. Files deleted: <count>", targetDir)
	if dryRun {
		impact = fmt.Sprintf("Preview of files that would be deleted in '%s': <list>", targetDir)
	}

	return &CleanTmpFilesAction{
		BaseAction: NewBaseAction(
			actionID,
			name,
			description,
			CategoryDisk,
			RiskSafe, // Low risk, but can cause issues if applications rely on old tmp files
			false,    // Not reversible
			impact,
			logger,
		),
		targetDir:  targetDir,
		maxAgeDays: maxAgeDays,
		dryRun:     dryRun,
	}
}

// NewCleanTmpFilesActionFactory returns a factory for creating CleanTmpFilesAction instances.
func NewCleanTmpFilesActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		targetDir, _ := params["target_dir"].(string)

		maxAgeDays := 7 // Default
		if ma, ok := params["max_age_days"].(int); ok {
			maxAgeDays = ma
		} else if maStr, ok := params["max_age_days"].(string); ok {
			if parsedMa, err := strconv.Atoi(maStr); err == nil {
				maxAgeDays = parsedMa
			}
		}

		dryRun := false
		if dr, ok := params["dry_run"].(bool); ok {
			dryRun = dr
		}

		return NewCleanTmpFilesAction(targetDir, maxAgeDays, dryRun, logger), nil
	}
}

// Validate checks if the target directory exists and is writable.
func (a *CleanTmpFilesAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	// Check if directory exists and is a directory
	cmdCheckDir := fmt.Sprintf("test -d %s", shellQuote(a.targetDir))
	_, _, exitCode, err := executor.ExecuteWithContext(ctx, cmdCheckDir)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("target directory '%s' does not exist or is not a directory: %v", a.targetDir, err)
	}

	// Check if directory is writable (by current user or root if executing as root)
	// This is a simple check, more robust checks might involve trying to create a temp file.
	cmdCheckWrite := fmt.Sprintf("test -w %s", shellQuote(a.targetDir))
	_, _, exitCode, err = executor.ExecuteWithContext(ctx, cmdCheckWrite)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("target directory '%s' is not writable: %v", a.targetDir, err)
	}

	if a.maxAgeDays <= 0 {
		return fmt.Errorf("max_age_days must be a positive integer")
	}

	return nil
}

// Execute performs the temporary file cleanup.
func (a *CleanTmpFilesAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	findArgs := []string{
		shellQuote(a.targetDir),
		"-mindepth 1",                           // Don't delete the directory itself
		"-type f",                               // Only consider files
		fmt.Sprintf("-atime +%d", a.maxAgeDays), // Access time
		fmt.Sprintf("-mtime +%d", a.maxAgeDays), // Modification time
	}

	// First, find files to be deleted to report them
	findCmd := fmt.Sprintf("find %s", strings.Join(findArgs, " "))
	filesToDeleteStr, _, exitCode, err := executor.ExecuteWithContext(ctx, findCmd)
	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to list files in '%s': %s", a.targetDir, err)
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("command failed: %w", err)
	}
	filesToDelete := strings.Split(strings.TrimSpace(filesToDeleteStr), "\n")
	if len(filesToDelete) == 1 && filesToDelete[0] == "" { // No files found
		filesToDelete = []string{}
	}

	if len(filesToDelete) == 0 {
		result.Status = StatusSkipped
		result.Message = fmt.Sprintf("No temporary files found in '%s' older than %d days to delete.", a.targetDir, a.maxAgeDays)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, nil
	}

	result.RollbackData = map[string]interface{}{
		"deleted_files": filesToDelete, // List files for audit, even if not reversible
	}
	result.ChangesApplied = []string{
		fmt.Sprintf("Identified %d files in '%s' older than %d days for deletion.", len(filesToDelete), a.targetDir, a.maxAgeDays),
	}

	if a.dryRun {
		result.Status = StatusSkipped
		result.Message = fmt.Sprintf("Dry run: %d files would be deleted from '%s'.", len(filesToDelete), a.targetDir)
		result.Output = strings.Join(filesToDelete, "\n")
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, nil
	}

	// Actual deletion command
	deleteCmd := fmt.Sprintf("%s -delete", findCmd)
	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, deleteCmd)

	if stderr != "" {
		result.Output = fmt.Sprintf("stdout: %s\nstderr: %s", stdout, stderr)
	} else {
		result.Output = stdout
	}

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to delete temporary files in '%s'", a.targetDir)
		errMsg := fmt.Sprintf("exit code %d", exitCode)
		if stderr != "" {
			errMsg = fmt.Sprintf("%s: %s", errMsg, stderr)
		}
		result.Error = errMsg
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("command failed: %w", err)
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully deleted %d temporary files from '%s' older than %d days.", len(filesToDelete), a.targetDir, a.maxAgeDays)
	result.ChangesApplied = append(result.ChangesApplied, fmt.Sprintf("Deleted %d files.", len(filesToDelete)))
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// RotateLogsAction compresses and truncates log files.
type RotateLogsAction struct {
	*BaseAction
	targetFile  string
	backupCount int
	compress    bool
	dryRun      bool
}

// NewRotateLogsAction creates a new RotateLogsAction.
func NewRotateLogsAction(targetFile string, backupCount int, compress bool, dryRun bool, logger *logrus.Logger) *RotateLogsAction {
	if targetFile == "" {
		targetFile = "/var/log/syslog" // Default log file
	}
	if backupCount <= 0 {
		backupCount = 1 // Default to 1 backup
	}

	actionID := fmt.Sprintf("disk.rotate_logs.%s", strings.ReplaceAll(targetFile, "/", "_"))
	name := fmt.Sprintf("Rotate log file '%s'", targetFile)
	description := fmt.Sprintf("Compresses and truncates '%s'. Keeps %d backups. Helps manage log growth.", targetFile, backupCount)
	impact := fmt.Sprintf("Reduces disk usage by rotating '%s'. Old logs are backed up. Disk space freed: <size>", targetFile)
	if dryRun {
		impact = fmt.Sprintf("Preview of log rotation for '%s': Would create backup '%s.1', truncate original.", targetFile, targetFile)
	}

	return &RotateLogsAction{
		BaseAction: NewBaseAction(
			actionID,
			name,
			description,
			CategoryDisk,
			RiskModerate, // Potential data loss if not handled correctly, but often safe
			false,        // Not reversible (though backups are kept)
			impact,
			logger,
		),
		targetFile:  targetFile,
		backupCount: backupCount,
		compress:    compress,
		dryRun:      dryRun,
	}
}

// NewRotateLogsActionFactory returns a factory for creating RotateLogsAction instances.
func NewRotateLogsActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		targetFile, ok := params["target_file"].(string)
		if !ok || targetFile == "" {
			return nil, fmt.Errorf("target_file parameter is required for RotateLogsAction")
		}

		backupCount := 1 // Default
		if bc, ok := params["backup_count"].(int); ok {
			backupCount = bc
		} else if bcStr, ok := params["backup_count"].(string); ok {
			if parsedBc, err := strconv.Atoi(bcStr); err == nil {
				backupCount = parsedBc
			}
		}

		compress := true // Default
		if c, ok := params["compress"].(bool); ok {
			compress = c
		}

		dryRun := false
		if dr, ok := params["dry_run"].(bool); ok {
			dryRun = dr
		}

		return NewRotateLogsAction(targetFile, backupCount, compress, dryRun, logger), nil
	}
}

// Validate checks if the target file exists and is writable.
func (a *RotateLogsAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	// Check if file exists and is a file
	cmdCheckFile := fmt.Sprintf("test -f %s", shellQuote(a.targetFile))
	_, _, exitCode, err := executor.ExecuteWithContext(ctx, cmdCheckFile)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("target file '%s' does not exist or is not a file: %v", a.targetFile, err)
	}

	// Check if file is writable
	cmdCheckWrite := fmt.Sprintf("test -w %s", shellQuote(a.targetFile))
	_, _, exitCode, err = executor.ExecuteWithContext(ctx, cmdCheckWrite)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("target file '%s' is not writable: %v", a.targetFile, err)
	}

	if a.backupCount <= 0 {
		return fmt.Errorf("backup_count must be a positive integer")
	}

	return nil
}

// Execute performs the log rotation.
// It renames the current log file and creates a new empty one.
// It also manages old backups by compressing and deleting oldest ones.
func (a *RotateLogsAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
	}

	// Get initial file size for impact estimation
	originalSize, err := getFileSize(ctx, executor, a.targetFile)
	if err != nil {
		a.logger.WithError(err).Warnf("Could not get original size for '%s'", a.targetFile)
		originalSize = 0 // Proceed without size if error
	}

	// Pre-check for dry run
	cmdPreCheck := fmt.Sprintf("ls -lah %s", shellQuote(a.targetFile))
	originalFileInfo, _, exitCode, err := executor.ExecuteWithContext(ctx, cmdPreCheck)
	if err != nil || exitCode != 0 {
		a.logger.WithError(err).Warnf("Failed to get pre-rotation file info for '%s'", a.targetFile)
	}

	if a.dryRun {
		result.Status = StatusSkipped
		result.Message = fmt.Sprintf("Dry run: Log rotation for '%s' would create backup, truncate original.", a.targetFile)
		result.Output = fmt.Sprintf("Original file info: %s\n", originalFileInfo)
		result.ChangesApplied = []string{
			fmt.Sprintf("Dry run: File '%s' would be moved to '%s.1' and truncated.", a.targetFile, a.targetFile),
		}
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, nil
	}

	// Shift existing backups: log.3 -> log.4, log.2 -> log.3, etc.
	for i := a.backupCount; i >= 1; i-- {
		oldBackup := fmt.Sprintf("%s.%d", a.targetFile, i)
		newBackup := fmt.Sprintf("%s.%d", a.targetFile, i+1)

		// If compressing, check for .gz suffix
		if a.compress {
			oldBackupGz := oldBackup + ".gz"
			newBackupGz := newBackup + ".gz"
			if _, _, exitCode, _ := executor.ExecuteWithContext(ctx, fmt.Sprintf("test -f %s", shellQuote(oldBackupGz))); exitCode == 0 {
				moveCmd := fmt.Sprintf("mv %s %s", shellQuote(oldBackupGz), shellQuote(newBackupGz))
				_, _, exitCode, err := executor.ExecuteWithContext(ctx, moveCmd) // Replaced stderr with _
				if err != nil || exitCode != 0 {
					a.logger.WithError(err).Warnf("Failed to move compressed backup '%s': exit code %d, err: %v", oldBackupGz, exitCode, err) // Adjusted error message
				} else {
					result.ChangesApplied = append(result.ChangesApplied, fmt.Sprintf("Moved compressed backup '%s' to '%s'", oldBackupGz, newBackupGz))
				}
			}
		}

		if _, _, exitCode, _ := executor.ExecuteWithContext(ctx, fmt.Sprintf("test -f %s", shellQuote(oldBackup))); exitCode == 0 {
			moveCmd := fmt.Sprintf("mv %s %s", shellQuote(oldBackup), shellQuote(newBackup))
			_, _, exitCode, err := executor.ExecuteWithContext(ctx, moveCmd) // Replaced stderr with _
			if err != nil || exitCode != 0 {
				a.logger.WithError(err).Warnf("Failed to move backup '%s': exit code %d, err: %v", oldBackup, exitCode, err) // Adjusted error message
			} else {
				result.ChangesApplied = append(result.ChangesApplied, fmt.Sprintf("Moved backup '%s' to '%s'", oldBackup, newBackup))
			}
		}
	}

	// Move current log file to .1
	backupFile := fmt.Sprintf("%s.1", a.targetFile)
	moveCurrentCmd := fmt.Sprintf("mv %s %s", shellQuote(a.targetFile), shellQuote(backupFile))
	_, _, exitCode, err = executor.ExecuteWithContext(ctx, moveCurrentCmd) // Replaced stderr with _
	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to move current log file '%s' to backup '%s'", a.targetFile, backupFile)
		result.Error = fmt.Errorf("exit code %d: %v", exitCode, err).Error() // Adjusted error message
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("command failed: %w", err)
	}
	result.ChangesApplied = append(result.ChangesApplied, fmt.Sprintf("Moved '%s' to '%s'", a.targetFile, backupFile))
	// Create a new empty log file with original permissions
	touchNewCmd := fmt.Sprintf("touch %s", shellQuote(a.targetFile))
	_, _, exitCode, err = executor.ExecuteWithContext(ctx, touchNewCmd)
	if err != nil || exitCode != 0 {
		a.logger.WithError(err).Warnf("Failed to create new log file '%s': %s", a.targetFile, err)
	} else {
		result.ChangesApplied = append(result.ChangesApplied, fmt.Sprintf("Created new empty log file '%s'", a.targetFile))
	}

	// If compress is true, compress the new backupFile
	if a.compress {
		compressCmd := fmt.Sprintf("gzip %s", shellQuote(backupFile))
		_, _, exitCode, err := executor.ExecuteWithContext(ctx, compressCmd)
		if err != nil || exitCode != 0 {
			a.logger.WithError(err).Warnf("Failed to compress backup file '%s': %s", backupFile, err)
		} else {
			result.ChangesApplied = append(result.ChangesApplied, fmt.Sprintf("Compressed backup file '%s' to '%s.gz'", backupFile, backupFile))
			backupFile += ".gz" // Update backupFile name
		}
	}

	// Calculate freed space
	freedSpace := originalSize - (getFileSizeSafe(ctx, executor, backupFile))

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully rotated log file '%s'. Freed %s.", a.targetFile, formatBytes(freedSpace))
	result.ChangesApplied = append(result.ChangesApplied, fmt.Sprintf("Freed approximately %s of disk space.", formatBytes(freedSpace)))
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// getFileSize returns the size of a file in bytes.
func getFileSize(ctx context.Context, executor diagnostics.CommandExecutor, filePath string) (int64, error) {
	cmd := fmt.Sprintf("stat -c %%s %s 2>/dev/null || stat -f %%z %s 2>/dev/null", shellQuote(filePath), shellQuote(filePath))
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	if err != nil || exitCode != 0 {
		return 0, fmt.Errorf("failed to get file size for '%s': %w", filePath, err)
	}
	sizeStr := strings.TrimSpace(stdout)
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse file size '%s': %w", sizeStr, err)
	}
	return size, nil
}

// getFileSizeSafe returns the size of a file in bytes, or 0 if an error occurs.
func getFileSizeSafe(ctx context.Context, executor diagnostics.CommandExecutor, filePath string) int64 {
	size, err := getFileSize(ctx, executor, filePath)
	if err != nil {
		return 0
	}
	return size
}

// formatBytes converts bytes to a human-readable format.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// Register all factories for disk actions
func init() {
	logger := logrus.StandardLogger() // Use standard logger for init functions
	registry := GetDefaultRegistry(logger)
	_ = registry.Register("disk.clean_tmp", NewCleanTmpFilesActionFactory())
	_ = registry.Register("disk.rotate_logs", NewRotateLogsActionFactory())
}
