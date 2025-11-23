package remediation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCommandExecutor is a mock implementation of diagnostics.CommandExecutor
type MockCommandExecutor struct {
	mock.Mock
}

func (m *MockCommandExecutor) ExecuteWithContext(ctx context.Context, command string) (string, string, int, error) {
	args := m.Called(ctx, command)
	return args.String(0), args.String(1), args.Int(2), args.Error(3)
}

func (m *MockCommandExecutor) Execute(command string, timeout time.Duration) (string, string, int, error) {
	args := m.Called(context.Background(), command, timeout)
	return args.String(0), args.String(1), args.Int(2), args.Error(3)
}

func TestCleanTmpFilesAction(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(new(mockWriter))
	ctx := context.Background()

	t.Run("Validates successfully", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "test -d '/tmp'").Return("", "", 0, nil).Once()
		mockExecutor.On("ExecuteWithContext", ctx, "test -w '/tmp'").Return("", "", 0, nil).Once()

		action := NewCleanTmpFilesAction("/tmp", 7, false, logger)
		err := action.Validate(ctx, mockExecutor)
		assert.NoError(t, err)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Validation fails if targetDir does not exist", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "test -d '/nonexistent'").Return("", "", 1, errors.New("not a directory")).Once()

		action := NewCleanTmpFilesAction("/nonexistent", 7, false, logger)
		err := action.Validate(ctx, mockExecutor)
		assert.Error(t, err)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Validation fails if targetDir not writable", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "test -d '/tmp'").Return("", "", 0, nil).Once()
		mockExecutor.On("ExecuteWithContext", ctx, "test -w '/tmp'").Return("", "", 1, errors.New("not writable")).Once()

		action := NewCleanTmpFilesAction("/tmp", 7, false, logger)
		err := action.Validate(ctx, mockExecutor)
		assert.Error(t, err)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Execute deletes files successfully", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		// The find command uses single quotes for the directory
		mockExecutor.On("ExecuteWithContext", ctx, "find '/tmp' -mindepth 1 -type f -atime +7 -mtime +7").Return("file1\nfile2", "", 0, nil).Once()
		mockExecutor.On("ExecuteWithContext", ctx, "find '/tmp' -mindepth 1 -type f -atime +7 -mtime +7 -delete").Return("", "", 0, nil).Once()

		action := NewCleanTmpFilesAction("/tmp", 7, false, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.NoError(t, err)
		assert.Equal(t, StatusSuccess, result.Status)
		assert.Len(t, result.ChangesApplied, 2)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Execute handles no files found", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "find '/tmp' -mindepth 1 -type f -atime +7 -mtime +7").Return("", "", 0, nil).Once()

		action := NewCleanTmpFilesAction("/tmp", 7, false, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.NoError(t, err)
		assert.Equal(t, StatusSkipped, result.Status)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Execute dry run returns skipped status", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "find '/tmp' -mindepth 1 -type f -atime +7 -mtime +7").Return("file1\nfile2", "", 0, nil).Once()

		action := NewCleanTmpFilesAction("/tmp", 7, true, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.NoError(t, err)
		assert.Equal(t, StatusSkipped, result.Status)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Factory creates action correctly", func(t *testing.T) {
		registry := GetDefaultRegistry(logger)
		params := map[string]interface{}{
			"target_dir":   "/var/tmp",
			"max_age_days": 10,
			"dry_run":      true,
		}
		action, err := registry.Create("disk.clean_tmp", params)
		assert.NoError(t, err)
		cleanAction, ok := action.(*CleanTmpFilesAction)
		assert.True(t, ok)
		assert.Equal(t, "/var/tmp", cleanAction.targetDir)
		assert.Equal(t, 10, cleanAction.maxAgeDays)
		assert.True(t, cleanAction.dryRun)
	})
}

func TestRotateLogsAction(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(new(mockWriter))
	ctx := context.Background()
	testLogFile := "/var/log/test.log"

	// Explicitly define the quoted string to match shellQuote behavior
	testLogFileQuoted := "'/var/log/test.log'"

	t.Run("Validates successfully", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "test -f "+testLogFileQuoted).Return("", "", 0, nil).Once()
		mockExecutor.On("ExecuteWithContext", ctx, "test -w "+testLogFileQuoted).Return("", "", 0, nil).Once()

		action := NewRotateLogsAction(testLogFile, 1, false, false, logger)
		err := action.Validate(ctx, mockExecutor)
		assert.NoError(t, err)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Validation fails if targetFile does not exist", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "test -f "+testLogFileQuoted).Return("", "", 1, errors.New("not a file")).Once()

		action := NewRotateLogsAction(testLogFile, 1, false, false, logger)
		err := action.Validate(ctx, mockExecutor)
		assert.Error(t, err)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Validation fails if targetFile not writable", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "test -f "+testLogFileQuoted).Return("", "", 0, nil).Once()
		mockExecutor.On("ExecuteWithContext", ctx, "test -w "+testLogFileQuoted).Return("", "", 1, errors.New("not writable")).Once()

		action := NewRotateLogsAction(testLogFile, 1, false, false, logger)
		err := action.Validate(ctx, mockExecutor)
		assert.Error(t, err)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Execute rotates logs successfully (no compression)", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		
		// 1. Initial file size check
		mockExecutor.On("ExecuteWithContext", ctx, "stat -c %s "+testLogFileQuoted+" 2>/dev/null || stat -f %z "+testLogFileQuoted+" 2>/dev/null").Return("1024", "", 0, nil).Once()
		
		// 2. Pre-check for dry run info (ALWAYS called)
		mockExecutor.On("ExecuteWithContext", ctx, "ls -lah "+testLogFileQuoted).Return("-rw-r--r-- 1 root root 1.0K Nov 23 12:00 test.log", "", 0, nil).Once()

		// 3. Backup shifting
		// test -f '/var/log/test.log.1.gz'
		mockExecutor.On("ExecuteWithContext", ctx, "test -f '/var/log/test.log.1.gz'").Return("", "", 1, nil).Once()
		// test -f '/var/log/test.log.1'
		mockExecutor.On("ExecuteWithContext", ctx, "test -f '/var/log/test.log.1'").Return("", "", 1, nil).Once()

		// 4. Move current log to .1
		mockExecutor.On("ExecuteWithContext", ctx, "mv "+testLogFileQuoted+" '/var/log/test.log.1'").Return("", "", 0, nil).Once()
		
		// 5. Create new empty log file
		mockExecutor.On("ExecuteWithContext", ctx, "touch "+testLogFileQuoted).Return("", "", 0, nil).Once()
		
		// 6. Get new backup size
		mockExecutor.On("ExecuteWithContext", ctx, "stat -c %s '/var/log/test.log.1' 2>/dev/null || stat -f %z '/var/log/test.log.1' 2>/dev/null").Return("1024", "", 0, nil).Once()

		action := NewRotateLogsAction(testLogFile, 1, false, false, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.NoError(t, err)
		assert.Equal(t, StatusSuccess, result.Status)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Execute rotates logs successfully (with compression)", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		
		// 1. Initial size
		mockExecutor.On("ExecuteWithContext", ctx, "stat -c %s "+testLogFileQuoted+" 2>/dev/null || stat -f %z "+testLogFileQuoted+" 2>/dev/null").Return("10240", "", 0, nil).Once()
		
		// 2. Info check
		mockExecutor.On("ExecuteWithContext", ctx, "ls -lah "+testLogFileQuoted).Return("-rw-r--r-- 1 root root 10K Nov 23 12:00 test.log", "", 0, nil).Once()

		// 3. Backup shifting
		mockExecutor.On("ExecuteWithContext", ctx, "test -f '/var/log/test.log.1.gz'").Return("", "", 1, nil).Once()
		mockExecutor.On("ExecuteWithContext", ctx, "test -f '/var/log/test.log.1'").Return("", "", 1, nil).Once()

		// 4. Move current
		mockExecutor.On("ExecuteWithContext", ctx, "mv "+testLogFileQuoted+" '/var/log/test.log.1'").Return("", "", 0, nil).Once()
		
		// 5. Create new
		mockExecutor.On("ExecuteWithContext", ctx, "touch "+testLogFileQuoted).Return("", "", 0, nil).Once()
		
		// 6. Compress backup
		mockExecutor.On("ExecuteWithContext", ctx, "gzip '/var/log/test.log.1'").Return("", "", 0, nil).Once()
		
		// 7. Get size of compressed backup
		mockExecutor.On("ExecuteWithContext", ctx, "stat -c %s '/var/log/test.log.1.gz' 2>/dev/null || stat -f %z '/var/log/test.log.1.gz' 2>/dev/null").Return("1024", "", 0, nil).Once()

		action := NewRotateLogsAction(testLogFile, 1, true, false, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.NoError(t, err)
		assert.Equal(t, StatusSuccess, result.Status)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Execute dry run for RotateLogs returns skipped status", func(t *testing.T) {
		mockExecutor := new(MockCommandExecutor)
		mockExecutor.On("ExecuteWithContext", ctx, "stat -c %s "+testLogFileQuoted+" 2>/dev/null || stat -f %z "+testLogFileQuoted+" 2>/dev/null").Return("1024", "", 0, nil).Once()
		mockExecutor.On("ExecuteWithContext", ctx, "ls -lah "+testLogFileQuoted).Return("-rw-r--r-- 1 root root 1.0K Nov 23 12:00 test.log", "", 0, nil).Once()

		action := NewRotateLogsAction(testLogFile, 1, false, true, logger)
		result, err := action.Execute(ctx, mockExecutor)

		assert.NoError(t, err)
		assert.Equal(t, StatusSkipped, result.Status)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("Factory creates action correctly", func(t *testing.T) {
		registry := GetDefaultRegistry(logger)
		params := map[string]interface{}{
			"target_file":  "/var/log/nginx/access.log",
			"backup_count": 5,
			"compress":     false,
		}
		action, err := registry.Create("disk.rotate_logs", params)
		assert.NoError(t, err)
		rotateAction, ok := action.(*RotateLogsAction)
		assert.True(t, ok)
		assert.Equal(t, "/var/log/nginx/access.log", rotateAction.targetFile)
		assert.Equal(t, 5, rotateAction.backupCount)
		assert.False(t, rotateAction.compress)
	})
}

// mockWriter is a simple writer to suppress logrus output during tests.
type mockWriter struct{}

func (m *mockWriter) Write(p []byte) (int, error) {
	return len(p), nil
}