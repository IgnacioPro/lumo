package checkers

import (
	"context"
	"strings"
	"testing"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestNewDiskChecker(t *testing.T) {
	thresholds := diagnostics.DiskThresholds{
		UsageWarn:     85.0,
		UsageCritical: 95.0,
	}

	checker := NewDiskChecker(thresholds)
	if checker == nil {
		t.Fatal("NewDiskChecker returned nil")
	}
	if checker.thresholds.UsageWarn != 85.0 {
		t.Errorf("Expected UsageWarn 85.0, got %v", checker.thresholds.UsageWarn)
	}
}

func TestDiskChecker_Name(t *testing.T) {
	checker := NewDiskChecker(diagnostics.DiskThresholds{})
	if got := checker.Name(); got != "disk_check" {
		t.Errorf("Name() = %v, want disk_check", got)
	}
}

func TestDiskChecker_Category(t *testing.T) {
	checker := NewDiskChecker(diagnostics.DiskThresholds{})
	if got := checker.Category(); got != diagnostics.CategoryDisk {
		t.Errorf("Category() = %v, want %v", got, diagnostics.CategoryDisk)
	}
}

func TestDiskChecker_Description(t *testing.T) {
	checker := NewDiskChecker(diagnostics.DiskThresholds{})
	desc := checker.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if !strings.Contains(strings.ToLower(desc), "disk") {
		t.Error("Description should mention disk")
	}
}

func TestDiskChecker_RequiresRoot(t *testing.T) {
	checker := NewDiskChecker(diagnostics.DiskThresholds{})
	if checker.RequiresRoot() {
		t.Error("DiskChecker should not require root")
	}
}

func TestDiskChecker_Run(t *testing.T) {
	thresholds := diagnostics.DiskThresholds{
		UsageWarn:     85.0,
		UsageCritical: 95.0,
		InodeWarn:     85.0,
		InodeCritical: 95.0,
	}

	dfOutput := `Filesystem     1K-blocks      Used Available Use% Mounted on
/dev/sda1      104806400  52403200  47252224  53% /`

	dfInodeOutput := `Filesystem     Inodes  IUsed  IFree IUse% Mounted on
/dev/sda1      6553600 655360 5898240   10% /`

	executor := &mockExecutor{
		responses: map[string]mockResponse{
			"df -k": {
				stdout:   dfOutput,
				exitCode: 0,
			},
			"df -i": {
				stdout:   dfInodeOutput,
				exitCode: 0,
			},
		},
	}

	checker := NewDiskChecker(thresholds)
	result, err := checker.Run(context.Background(), executor)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Name != "disk_check" {
		t.Errorf("Result.Name = %v, want disk_check", result.Name)
	}

	if result.Category != diagnostics.CategoryDisk {
		t.Errorf("Result.Category = %v, want %v", result.Category, diagnostics.CategoryDisk)
	}

	if result.Status != diagnostics.StatusCompleted {
		t.Errorf("Result.Status = %v, want %v", result.Status, diagnostics.StatusCompleted)
	}

	// Check that filesystems data is present
	if _, ok := result.Data["filesystems"]; !ok {
		t.Error("Expected filesystems data to be present")
	}

	// Check message is generated
	if result.Message == "" {
		t.Error("Expected non-empty message")
	}
}

func TestDiskChecker_ShouldMonitorFilesystem(t *testing.T) {
	tests := []struct {
		name       string
		mountPoint string
		want       bool
	}{
		{
			name:       "root filesystem",
			mountPoint: "/",
			want:       true,
		},
		{
			name:       "home directory",
			mountPoint: "/home",
			want:       true,
		},
		{
			name:       "var directory",
			mountPoint: "/var",
			want:       true,
		},
		{
			name:       "tmp directory",
			mountPoint: "/tmp",
			want:       true,
		},
		{
			name:       "skip /dev (not exact)",
			mountPoint: "/dev/shm",
			want:       false,
		},
		{
			name:       "skip /sys",
			mountPoint: "/sys/kernel",
			want:       false,
		},
		{
			name:       "skip /proc",
			mountPoint: "/proc/sys",
			want:       false,
		},
		{
			name:       "skip /run",
			mountPoint: "/run/user",
			want:       false,
		},
		{
			name:       "skip /snap",
			mountPoint: "/snap/core",
			want:       false,
		},
		{
			name:       "skip /boot/efi",
			mountPoint: "/boot/efi",
			want:       false,
		},
		{
			name:       "allow /boot (not /boot/efi)",
			mountPoint: "/boot",
			want:       true,
		},
		{
			name:       "skip loop devices",
			mountPoint: "/var/lib/snapd/loop1",
			want:       false,
		},
		{
			name:       "skip tmpfs (non-root)",
			mountPoint: "/run/tmpfs",
			want:       false,
		},
		{
			name:       "custom mount point",
			mountPoint: "/mnt/data",
			want:       true,
		},
		{
			name:       "opt directory",
			mountPoint: "/opt",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewDiskChecker(diagnostics.DiskThresholds{})
			got := checker.shouldMonitorFilesystem(tt.mountPoint)
			if got != tt.want {
				t.Errorf("shouldMonitorFilesystem(%q) = %v, want %v", tt.mountPoint, got, tt.want)
			}
		})
	}
}

func TestDiskChecker_SanitizeMountPoint(t *testing.T) {
	tests := []struct {
		name       string
		mountPoint string
		want       string
	}{
		{
			name:       "root filesystem",
			mountPoint: "/",
			want:       "root",
		},
		{
			name:       "home directory",
			mountPoint: "/home",
			want:       "home",
		},
		{
			name:       "nested path",
			mountPoint: "/var/lib/docker",
			want:       "var_lib_docker",
		},
		{
			name:       "mnt path",
			mountPoint: "/mnt/data",
			want:       "mnt_data",
		},
		{
			name:       "multiple levels",
			mountPoint: "/mnt/backup/daily",
			want:       "mnt_backup_daily",
		},
		{
			name:       "trailing slash",
			mountPoint: "/opt/",
			want:       "opt",
		},
		{
			name:       "multiple trailing slashes",
			mountPoint: "/usr/local//",
			want:       "usr_local",
		},
		{
			name:       "single level",
			mountPoint: "/tmp",
			want:       "tmp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeMountPoint(tt.mountPoint)
			if got != tt.want {
				t.Errorf("sanitizeMountPoint(%q) = %q, want %q", tt.mountPoint, got, tt.want)
			}
		})
	}
}
